//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationsBuildR1SchemaFromEmptyDatabase(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_empty")
	paths := migrationPaths(t)
	applyMigrationPaths(t, pool, paths)
	applyMigrationPaths(t, pool, paths)

	requiredTables := []string{
		"contact_submissions",
		"telegram_subscribers",
		"professional_roles",
		"users",
		"profiles",
		"credentials",
		"sessions",
		"email_verification_tokens",
		"password_reset_tokens",
		"auth_rate_limit_buckets",
		"projects",
		"project_memberships",
		"project_tasks",
		"project_meetings",
		"user_settings",
		"project_user_settings",
		"audit_events",
		"notification_outbox",
		"monitoring_samples",
		"monitoring_incidents",
		"report_deliveries",
		"telegram_bot_auth_states",
		"telegram_account_bindings",
		"account_notification_preferences",
		"chats",
		"chat_memberships",
		"chat_messages",
		"project_documents",
		"global_chats",
		"global_chat_members",
		"global_chat_messages",
		"technical_reports",
	}
	for _, table := range requiredTables {
		var relation *string
		if err := pool.QueryRow(context.Background(), "SELECT to_regclass($1)::text", table).Scan(&relation); err != nil {
			t.Fatal(err)
		}
		if relation == nil {
			t.Fatalf("required table %q was not created", table)
		}
	}

	var roleCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM professional_roles WHERE code IN ('customer', 'designer', 'foreman', 'architect')").Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != 4 {
		t.Fatalf("seeded professional roles = %d, want 4", roleCount)
	}
}

func TestTelegramAccountAuthSchemaEnforcesTTLSecretsAndActiveBindings(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_telegram_auth")
	applyMigrationPaths(t, pool, migrationPaths(t))

	ctx := context.Background()
	var userOne, userTwo string
	for index, destination := range []*string{&userOne, &userTwo} {
		login := fmt.Sprintf("telegram_user_%d_%d", time.Now().UnixNano(), index)
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (login, login_normalized, email, email_normalized, status)
			VALUES ($1, $1, $1 || '@example.test', $1 || '@example.test', 'active')
			RETURNING id::text
		`, login).Scan(destination); err != nil {
			t.Fatalf("insert Telegram-confirmed active user without verified email: %v", err)
		}
	}

	var forbiddenSecretColumns int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'telegram_bot_auth_states'
		  AND column_name IN ('login', 'password', 'password_hash', 'credential')
	`).Scan(&forbiddenSecretColumns); err != nil {
		t.Fatal(err)
	}
	if forbiddenSecretColumns != 0 {
		t.Fatalf("telegram bot auth state contains %d forbidden credential columns", forbiddenSecretColumns)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := pool.Exec(ctx, `
		INSERT INTO telegram_bot_auth_states
			(chat_id, flow, step, request_id, started_at, expires_at)
		VALUES (701, 'registration_confirmation', 'awaiting_login', 'request-telegram-701', $1::timestamptz, $1::timestamptz + INTERVAL '5 minutes')
	`, now); err != nil {
		t.Fatalf("insert valid five-minute auth state: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO telegram_bot_auth_states
			(chat_id, flow, step, request_id, started_at, expires_at)
		VALUES (702, 'registration_confirmation', 'awaiting_login', 'request-telegram-702', $1::timestamptz, $1::timestamptz + INTERVAL '5 minutes 1 second')
	`, now); err == nil {
		t.Fatal("bot auth state longer than five minutes was accepted")
	}
	if _, err := pool.Exec(ctx, `
		UPDATE telegram_bot_auth_states
		SET step = 'awaiting_password', candidate_user_id = $2
		WHERE chat_id = $1
	`, 701, userOne); err != nil {
		t.Fatalf("advance bot auth state without storing login: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO telegram_account_bindings (user_id, chat_id, chat_username, verified_at)
		VALUES ($1, 701, 'first_user', $3), ($2, 702, 'second_user', $3)
	`, userOne, userTwo, now); err != nil {
		t.Fatalf("insert independent Telegram bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO telegram_account_bindings (user_id, chat_id, verified_at)
		VALUES ($1, 703, $2)
	`, userOne, now); err == nil {
		t.Fatal("second active binding for one user was accepted")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO telegram_account_bindings (user_id, chat_id, verified_at)
		VALUES ($1, 701, $2)
	`, userTwo, now); err == nil {
		t.Fatal("one private chat was bound to two active users")
	}

	if _, err := pool.Exec(ctx, `
		UPDATE telegram_account_bindings
		SET revoked_at = $2, revoke_reason = 'user_unlinked', version = version + 1
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userOne, now.Add(time.Second)); err != nil {
		t.Fatalf("revoke Telegram binding: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO telegram_account_bindings (user_id, chat_id, verified_at)
		VALUES ($1, 703, $2)
	`, userOne, now.Add(2*time.Second)); err != nil {
		t.Fatalf("create new binding after revoke: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO account_notification_preferences
			(user_id, verification_channel, recovery_channel, telegram_events_enabled)
		VALUES ($1, 'telegram', 'telegram', TRUE)
	`, userOne); err != nil {
		t.Fatalf("save Telegram delivery preferences: %v", err)
	}
}

func TestReportDeliveryLogicalKeyHasOneConcurrentWinner(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_delivery")
	applyMigrationPaths(t, pool, migrationPaths(t))

	context := context.Background()
	var recipientID string
	if err := pool.QueryRow(context, `
		INSERT INTO users (
			login, login_normalized, email, email_normalized,
			status, global_role, email_verified_at
		)
		VALUES ('report_admin', 'report_admin', 'report@example.test', 'report@example.test', 'active', 'super_admin', NOW())
		RETURNING id::text
	`).Scan(&recipientID); err != nil {
		t.Fatal(err)
	}

	periodStart := time.Date(2026, time.September, 21, 21, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(24 * time.Hour)
	start := make(chan struct{})
	errors := make(chan error, 20)
	var winners int64
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, err := pool.Exec(context, `
				INSERT INTO report_deliveries (
					report_type, period_start, period_end, recipient_id, aggregate_payload
				)
				VALUES ('daily_platform', $1, $2, $3, '{}'::jsonb)
				ON CONFLICT (report_type, period_start, period_end, recipient_id) DO NOTHING
			`, periodStart, periodEnd, recipientID)
			if err != nil {
				errors <- err
				return
			}
			if result.RowsAffected() == 1 {
				atomic.AddInt64(&winners, 1)
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}

	var rows int
	if err := pool.QueryRow(context, `
		SELECT count(*)
		FROM report_deliveries
		WHERE report_type = 'daily_platform'
		  AND period_start = $1
		  AND period_end = $2
		  AND recipient_id = $3
	`, periodStart, periodEnd, recipientID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if winners != 1 || rows != 1 {
		t.Fatalf("logical delivery winners=%d rows=%d, want 1 and 1", winners, rows)
	}
}

func TestAuditEventsAreImmutableUntilRetentionExpires(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_audit")
	applyMigrationPaths(t, pool, migrationPaths(t))

	context := context.Background()
	var retainedID int64
	if err := pool.QueryRow(context, `
		INSERT INTO audit_events (event_type, result, request_id)
		VALUES ('auth.login', 'success', 'request-retained')
		RETURNING id
	`).Scan(&retainedID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context, "UPDATE audit_events SET result = 'invalid_credentials' WHERE id = $1", retainedID); err == nil {
		t.Fatal("audit event content update succeeded")
	}
	if _, err := pool.Exec(context, "DELETE FROM audit_events WHERE id = $1", retainedID); err == nil {
		t.Fatal("audit event was deleted before retention expired")
	}

	var expiredID int64
	if err := pool.QueryRow(context, `
		INSERT INTO audit_events (
			event_type, result, request_id, occurred_at, retention_until
		)
		VALUES (
			'auth.login', 'invalid_credentials', 'request-expired',
			NOW() - INTERVAL '366 days', NOW() - INTERVAL '1 day'
		)
		RETURNING id
	`).Scan(&expiredID); err != nil {
		t.Fatal(err)
	}
	if result, err := pool.Exec(context, "DELETE FROM audit_events WHERE id = $1", expiredID); err != nil || result.RowsAffected() != 1 {
		t.Fatalf("expired audit event delete rows=%d error=%v, want 1 and nil", result.RowsAffected(), err)
	}
}

func TestR1MigrationUpgradesCurrentSchemaWithoutLosingRows(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_upgrade")
	paths := migrationPaths(t)
	if len(paths) < 5 {
		t.Fatalf("migration count = %d, want at least 5", len(paths))
	}
	applyMigrationPaths(t, pool, paths[:4])

	context := context.Background()
	var submissionID string
	if err := pool.QueryRow(context, `
		INSERT INTO contact_submissions (name, contact, project_type, project_details)
		VALUES ('Legacy', 'legacy@example.com', 'Квартира', '')
		RETURNING id::text
	`).Scan(&submissionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context, `
		INSERT INTO contact_consents (submission_id, consent_version, document_path)
		VALUES ($1, 'legacy', '/legacy.pdf')
	`, submissionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context, "INSERT INTO telegram_subscribers (chat_id, username) VALUES (101, 'legacy_admin')"); err != nil {
		t.Fatal(err)
	}

	applyMigrationPaths(t, pool, paths[4:])

	var submissionCount, consentCount, subscriberCount int
	if err := pool.QueryRow(context, "SELECT count(*) FROM contact_submissions").Scan(&submissionCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context, "SELECT count(*) FROM contact_consents").Scan(&consentCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context, "SELECT count(*) FROM telegram_subscribers WHERE chat_id = 101 AND user_id IS NULL AND verified_at IS NULL AND version = 1").Scan(&subscriberCount); err != nil {
		t.Fatal(err)
	}
	if submissionCount != 1 || consentCount != 1 || subscriberCount != 1 {
		t.Fatalf("upgrade changed legacy rows: submissions=%d consents=%d subscribers=%d", submissionCount, consentCount, subscriberCount)
	}
}

func TestR1ProjectCustomerConstraintIsDeferredAndConsistent(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_customer")
	applyMigrationPaths(t, pool, migrationPaths(t))

	context := context.Background()
	var customerID, creatorID string
	for index, destination := range []*string{&customerID, &creatorID} {
		if err := pool.QueryRow(context, `
			INSERT INTO users (
				login, login_normalized, email, email_normalized,
				status, email_verified_at
			)
			VALUES ($1, $1, $1 || '@example.test', $1 || '@example.test', 'active', NOW())
			RETURNING id::text
		`, fmt.Sprintf("user_%d_%d", time.Now().UnixNano(), index)).Scan(destination); err != nil {
			t.Fatal(err)
		}
	}

	tx, err := pool.Begin(context)
	if err != nil {
		t.Fatal(err)
	}
	var projectID string
	if err := tx.QueryRow(context, `
		INSERT INTO projects (created_by_user_id, customer_user_id, name, type)
		VALUES ($1, $2, 'R1 project', 'Interior')
		RETURNING id::text
	`, creatorID, customerID).Scan(&projectID); err != nil {
		_ = tx.Rollback(context)
		t.Fatal(err)
	}
	if _, err := tx.Exec(context, `
		INSERT INTO project_memberships (project_id, user_id, project_role)
		VALUES ($1, $2, 'customer'), ($1, $3, 'project_admin')
	`, projectID, customerID, creatorID); err != nil {
		_ = tx.Rollback(context)
		t.Fatal(err)
	}
	if err := tx.Commit(context); err != nil {
		t.Fatalf("consistent project/customer transaction failed: %v", err)
	}

	invalidTx, err := pool.Begin(context)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := invalidTx.Exec(context, `
		INSERT INTO projects (created_by_user_id, customer_user_id, name, type)
		VALUES ($1, $2, 'Invalid project', 'Interior')
	`, creatorID, customerID); err != nil {
		_ = invalidTx.Rollback(context)
		t.Fatal(err)
	}
	if err := invalidTx.Commit(context); err == nil {
		t.Fatal("project with customer but without active customer membership committed")
	}

	var noCustomerProjectID string
	if err := pool.QueryRow(context, `
		INSERT INTO projects (created_by_user_id, name, type)
		VALUES ($1, 'No customer project', 'Interior')
		RETURNING id::text
	`, creatorID).Scan(&noCustomerProjectID); err != nil {
		t.Fatal(err)
	}
	moveTx, err := pool.Begin(context)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moveTx.Exec(context, `
		UPDATE project_memberships
		SET project_id = $1
		WHERE project_id = $2 AND project_role = 'customer' AND revoked_at IS NULL
	`, noCustomerProjectID, projectID); err != nil {
		_ = moveTx.Rollback(context)
		t.Fatal(err)
	}
	if err := moveTx.Commit(context); err == nil {
		t.Fatal("moving a customer membership without updating both project pointers committed")
	}
}

func newMigrationSchemaPool(t *testing.T, prefix string) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for migration integration tests")
	}
	if !strings.Contains(strings.ToLower(databaseURL), "test") {
		t.Fatal("TEST_DATABASE_URL must point to an isolated database whose name contains 'test'")
	}

	context := context.Background()
	adminPool, err := pgxpool.New(context, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(context, "CREATE SCHEMA "+identifier); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		_, _ = adminPool.Exec(context, "DROP SCHEMA "+identifier+" CASCADE")
		adminPool.Close()
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(context, config)
	if err != nil {
		_, _ = adminPool.Exec(context, "DROP SCHEMA "+identifier+" CASCADE")
		adminPool.Close()
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context, "DROP SCHEMA "+identifier+" CASCADE")
		adminPool.Close()
	})
	return pool
}

func migrationPaths(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		paths = append(paths, filepath.Join("..", "..", "migrations", entry.Name()))
	}
	sort.Strings(paths)
	return paths
}

func applyMigrationPaths(t *testing.T, pool *pgxpool.Pool, paths []string) {
	t.Helper()
	for _, path := range paths {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), string(migration)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), err)
		}
	}
}
