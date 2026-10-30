//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountRegistrationVerificationResendAndOutboxIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_account")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	tokens, err := account.NewTokenManager(bytes.Repeat([]byte{1}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	accountService, err := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "from@example.com", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	registration := account.Registration{
		Login: "sveta.design", Email: "sveta@example.com", FirstName: "Светлана",
		ProfessionalRoleCode: "designer", Password: "Надёжный пароль 2026!",
		PasswordConfirmation: "Надёжный пароль 2026!", RequestID: "request-register",
	}
	if _, err := accountService.Register(context.Background(), registration); err != nil {
		t.Fatal(err)
	}
	assertAccountTransactionRows(t, pool, 1, 1, 1, 0, 1, 0)
	if _, err := accountService.SelectVerificationChannel(context.Background(), "sveta.design", "email", "request-select-email"); err != nil {
		t.Fatal(err)
	}
	assertAccountTransactionRows(t, pool, 1, 1, 1, 1, 2, 1)
	if _, err := accountService.Register(context.Background(), registration); !errors.Is(err, account.ErrIdentifierUnavailable) {
		t.Fatalf("duplicate registration error = %v", err)
	}
	assertAccountTransactionRows(t, pool, 1, 1, 1, 1, 2, 1)

	var ciphertext []byte
	var keyVersion int
	if err := pool.QueryRow(context.Background(), `SELECT payload_ciphertext, payload_key_version FROM notification_outbox ORDER BY created_at LIMIT 1`).Scan(&ciphertext, &keyVersion); err != nil {
		t.Fatal(err)
	}
	payload, err := cipher.Decrypt(ciphertext, verificationMessageType, keyVersion)
	if err != nil {
		t.Fatal(err)
	}
	var email account.VerificationEmail
	if err := json.Unmarshal(payload, &email); err != nil {
		t.Fatal(err)
	}
	token := strings.SplitN(email.Text, "/verify-email#token=", 2)
	if len(token) != 2 {
		t.Fatalf("verification token missing from email: %q", email.Text)
	}
	tokenValue := strings.SplitN(token[1], "\n", 2)[0]
	if strings.Contains(string(ciphertext), tokenValue) {
		t.Fatal("raw verification token is visible in outbox ciphertext")
	}
	if err := accountService.Verify(context.Background(), tokenValue, "request-verify"); err != nil {
		t.Fatal(err)
	}
	if err := accountService.Verify(context.Background(), tokenValue, "request-repeat"); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("reused token error = %v", err)
	}
	var status string
	var verifiedAt *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT status, email_verified_at FROM users WHERE login_normalized = 'sveta.design'`).Scan(&status, &verifiedAt); err != nil || status != "active" || verifiedAt == nil {
		t.Fatalf("status=%q verifiedAt=%v error=%v", status, verifiedAt, err)
	}

	registration.Login, registration.Email, registration.RequestID = "pending.user", "pending@example.com", "request-pending"
	if _, err := accountService.Register(context.Background(), registration); err != nil {
		t.Fatal(err)
	}
	if _, err := accountService.SelectVerificationChannel(context.Background(), "pending.user", "email", "request-pending-email"); err != nil {
		t.Fatal(err)
	}
	if err := accountService.Resend(context.Background(), "PENDING.USER", "request-resend"); err != nil {
		t.Fatal(err)
	}
	var activeTokens, revokedTokens, pendingOutbox int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			count(*) FILTER (WHERE token.consumed_at IS NULL AND token.revoked_at IS NULL),
			count(*) FILTER (WHERE token.revoke_reason = 'resend')
		FROM email_verification_tokens token
		JOIN users ON users.id = token.user_id
		WHERE users.login_normalized = 'pending.user'
	`).Scan(&activeTokens, &revokedTokens); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM notification_outbox WHERE state = 'pending'`).Scan(&pendingOutbox); err != nil {
		t.Fatal(err)
	}
	if activeTokens != 1 || revokedTokens != 1 || pendingOutbox != 3 {
		t.Fatalf("active=%d revoked=%d pending outbox=%d", activeTokens, revokedTokens, pendingOutbox)
	}

	claimAt := now.Add(24 * time.Hour)
	entry, found, err := store.ClaimEmailOutbox(context.Background(), claimAt)
	if err != nil || !found || entry.Attempts != 1 {
		t.Fatalf("entry=%#v found=%v error=%v", entry, found, err)
	}
	if err := store.MarkEmailOutboxFailed(context.Background(), entry.ID, entry.Attempts, 5, claimAt, claimAt.Add(time.Minute), "smtp_unavailable"); err != nil {
		t.Fatal(err)
	}
	var outboxState string
	if err := pool.QueryRow(context.Background(), `SELECT state FROM notification_outbox WHERE id = $1`, entry.ID).Scan(&outboxState); err != nil || outboxState != "retry" {
		t.Fatalf("outbox state=%q error=%v", outboxState, err)
	}
	entry, found, err = store.ClaimEmailOutbox(context.Background(), claimAt.Add(2*time.Minute))
	if err != nil || !found {
		t.Fatalf("retry claim found=%v error=%v", found, err)
	}
	if err := store.MarkEmailOutboxDelivered(context.Background(), entry.ID, claimAt.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkEmailOutboxDelivered(context.Background(), entry.ID, claimAt.Add(2*time.Minute)); err == nil {
		t.Fatal("a delivered outbox row must not be marked from a lost claim")
	}
	entry, found, err = store.ClaimEmailOutbox(context.Background(), claimAt.Add(3*time.Minute))
	if err != nil || !found {
		t.Fatalf("terminal claim found=%v error=%v", found, err)
	}
	if err := store.MarkEmailOutboxFailed(context.Background(), entry.ID, 5, 5, claimAt, claimAt.Add(time.Hour), "payload_invalid"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkEmailOutboxFailed(context.Background(), entry.ID, 5, 5, claimAt, claimAt.Add(time.Hour), "payload_invalid"); err == nil {
		t.Fatal("a terminal outbox row must not be marked from a lost claim")
	}
	if _, err := pool.Exec(context.Background(), `
		UPDATE notification_outbox
		   SET state = 'delivering', locked_at = $1
		 WHERE id = (SELECT id FROM notification_outbox WHERE state = 'retry' LIMIT 1)
	`, claimAt.Add(-10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if recovered, err := store.RecoverStaleEmailOutbox(context.Background(), claimAt.Add(-5*time.Minute), claimAt); err != nil || recovered != 1 {
		t.Fatalf("recovered=%d error=%v", recovered, err)
	}
	if email, found, err := store.PendingAccountEmail(context.Background(), "absent@example.com"); err != nil || found || email != "" {
		t.Fatalf("absent email=%q found=%v error=%v", email, found, err)
	}
	queued, err := store.ReplaceVerificationToken(context.Background(), account.ReplacementVerification{IdentifierNormalized: "absent@example.com"})
	if err != nil || queued {
		t.Fatalf("absent replacement queued=%v error=%v", queued, err)
	}
}

func TestTelegramAccountConfirmationActivatesAndBindsAtomically(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_telegram_confirm")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	tokens, _ := account.NewTokenManager(bytes.Repeat([]byte{21}, 32), 1)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{22}, 32), 1)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "from@example.com", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureTelegramBot("polismakovaSvetlanaBot")
	password := "Telegram подтверждение 2026!"
	registration := account.Registration{Login: "telegram.user", Email: "telegram@example.com", FirstName: "Telegram", ProfessionalRoleCode: "customer", Password: password, PasswordConfirmation: password, RequestID: "request-telegram-register"}
	if _, err = service.Register(context.Background(), registration); err != nil {
		t.Fatal(err)
	}
	if handoff, err := service.SelectVerificationChannel(context.Background(), registration.Login, "telegram", "request-telegram-channel"); err != nil || handoff != "https://t.me/polismakovaSvetlanaBot?start=register" {
		t.Fatalf("handoff=%q error=%v", handoff, err)
	}
	if err = service.BeginTelegramConfirmation(context.Background(), 9001, "telegram-9001-1"); err != nil {
		t.Fatal(err)
	}
	if step, stepErr := service.TelegramConfirmationStep(context.Background(), 9001); stepErr != nil || step != "awaiting_login" {
		t.Fatalf("initial Telegram step=%q error=%v", step, stepErr)
	}
	if err = service.SubmitTelegramLogin(context.Background(), 9001, registration.Login); err != nil {
		t.Fatal(err)
	}
	if step, stepErr := service.TelegramConfirmationStep(context.Background(), 9001); stepErr != nil || step != "awaiting_password" {
		t.Fatalf("password Telegram step=%q error=%v", step, stepErr)
	}
	if err = service.CompleteTelegramConfirmation(context.Background(), 9001, 3, "telegram_user", password, "telegram-9001-3"); err != nil {
		t.Fatal(err)
	}
	var status string
	var emailVerifiedAt *time.Time
	var bindings, states, activeTokens int
	if err = pool.QueryRow(context.Background(), `SELECT status,email_verified_at FROM users WHERE login_normalized='telegram.user'`).Scan(&status, &emailVerifiedAt); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM telegram_account_bindings WHERE chat_id=9001 AND revoked_at IS NULL`).Scan(&bindings); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM telegram_bot_auth_states WHERE chat_id=9001`).Scan(&states); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM email_verification_tokens token JOIN users ON users.id=token.user_id WHERE users.login_normalized='telegram.user' AND token.consumed_at IS NULL AND token.revoked_at IS NULL`).Scan(&activeTokens); err != nil {
		t.Fatal(err)
	}
	if status != "active" || emailVerifiedAt != nil || bindings != 1 || states != 0 || activeTokens != 0 {
		t.Fatalf("status=%s emailVerified=%v bindings=%d states=%d activeTokens=%d", status, emailVerifiedAt, bindings, states, activeTokens)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: registration.Login, Password: password, RequestID: "request-after-telegram"}); err != nil {
		t.Fatalf("login after Telegram confirmation: %v", err)
	}
	if _, err = service.TelegramConfirmationStep(context.Background(), 9001); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("consumed Telegram dialog error=%v", err)
	}

	var confirmedUserID string
	if err = pool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE login_normalized='telegram.user'`).Scan(&confirmedUserID); err != nil {
		t.Fatal(err)
	}
	if err = store.BeginTelegramAuth(context.Background(), account.TelegramAuthStart{ChatID: 9003, Flow: "registration_confirmation", RequestID: "telegram-race-start", StartedAt: now, ExpiresAt: now.Add(5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err = store.AdvanceTelegramAuth(context.Background(), account.TelegramAuthAdvance{ChatID: 9003, CandidateUserID: &confirmedUserID, Now: now}); err != nil {
		t.Fatal(err)
	}
	raceErrors := make(chan error, 2)
	var raceGroup sync.WaitGroup
	for index := 0; index < 2; index++ {
		raceGroup.Add(1)
		go func(attempt int) {
			defer raceGroup.Done()
			raceErrors <- store.CompleteTelegramAuth(context.Background(), account.TelegramAuthCompletion{ChatID: 9003, MessageID: int64(80 + attempt), UserID: confirmedUserID, ChatUsername: "telegram_race", Now: now, RequestID: fmt.Sprintf("telegram-race-%d", attempt)})
		}(index)
	}
	raceGroup.Wait()
	close(raceErrors)
	successes, consumed := 0, 0
	for raceErr := range raceErrors {
		if raceErr == nil {
			successes++
		} else if errors.Is(raceErr, account.ErrInvalidToken) {
			consumed++
		} else {
			t.Fatalf("unexpected concurrent completion error: %v", raceErr)
		}
	}
	if successes != 1 || consumed != 1 {
		t.Fatalf("concurrent Telegram completion successes=%d consumed=%d, want 1/1", successes, consumed)
	}

	if err = store.BeginTelegramAuth(context.Background(), account.TelegramAuthStart{ChatID: 9004, Flow: "registration_confirmation", RequestID: "telegram-expired-start", StartedAt: now.Add(-6 * time.Minute), ExpiresAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.TelegramAuthStep(context.Background(), 9004, now); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("expired Telegram dialog error=%v", err)
	}

	if err = store.BeginTelegramAuth(context.Background(), account.TelegramAuthStart{ChatID: 9002, Flow: "registration_confirmation", RequestID: "telegram-9002-1", StartedAt: now, ExpiresAt: now.Add(5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err = store.RejectTelegramAuth(context.Background(), account.TelegramAuthRejection{ChatID: 9002, Now: now, RequestID: "telegram-9002-2", Reason: "invalid_credentials"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.TelegramAuthStep(context.Background(), 9002, now); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("rejected Telegram dialog error=%v", err)
	}

	claimAt := time.Now().UTC().Add(time.Minute)
	if err = service.ForgotPassword(context.Background(), registration.Login, "telegram", "192.0.2.10", "request-telegram-reset-1"); err != nil {
		t.Fatal(err)
	}
	entry, found, err := store.ClaimTelegramAccountOutbox(context.Background(), claimAt)
	if err != nil || !found || entry.ChatID != 9003 || entry.MessageType != passwordResetMessageType {
		t.Fatalf("claimed Telegram reset=%#v found=%v error=%v", entry, found, err)
	}
	if err = store.MarkTelegramAccountOutboxDelivered(context.Background(), entry.ID, claimAt); err != nil {
		t.Fatal(err)
	}
	if err = store.MarkTelegramAccountOutboxDelivered(context.Background(), entry.ID, claimAt); err == nil {
		t.Fatal("delivered Telegram outbox claim must not be reusable")
	}

	if err = service.ForgotPassword(context.Background(), registration.Email, "telegram", "192.0.2.11", "request-telegram-reset-2"); err != nil {
		t.Fatal(err)
	}
	entry, found, err = store.ClaimTelegramAccountOutbox(context.Background(), claimAt)
	if err != nil || !found {
		t.Fatalf("claim retry candidate found=%v error=%v", found, err)
	}
	retryAt := claimAt.Add(time.Minute)
	if err = store.MarkTelegramAccountOutboxFailed(context.Background(), entry.ID, entry.Attempts, 5, claimAt, retryAt, "telegram_unavailable"); err != nil {
		t.Fatal(err)
	}
	if _, found, err = store.ClaimTelegramAccountOutbox(context.Background(), claimAt); err != nil || found {
		t.Fatalf("future retry found=%v error=%v", found, err)
	}
	entry, found, err = store.ClaimTelegramAccountOutbox(context.Background(), retryAt)
	if err != nil || !found {
		t.Fatalf("due retry found=%v error=%v", found, err)
	}
	if err = store.MarkTelegramAccountOutboxFailed(context.Background(), entry.ID, 5, 5, retryAt, retryAt.Add(time.Minute), "payload_invalid"); err != nil {
		t.Fatal(err)
	}
	if err = store.MarkTelegramAccountOutboxFailed(context.Background(), entry.ID, 5, 5, retryAt, retryAt.Add(time.Minute), "payload_invalid"); err == nil {
		t.Fatal("terminal Telegram outbox claim must not be reusable")
	}

	if err = service.ForgotPassword(context.Background(), registration.Login, "telegram", "192.0.2.12", "request-telegram-reset-3"); err != nil {
		t.Fatal(err)
	}
	entry, found, err = store.ClaimTelegramAccountOutbox(context.Background(), retryAt.Add(time.Minute))
	if err != nil || !found {
		t.Fatalf("claim stale candidate found=%v error=%v", found, err)
	}
	if recovered, recoverErr := store.RecoverStaleTelegramAccountOutbox(context.Background(), retryAt.Add(2*time.Minute), retryAt.Add(3*time.Minute)); recoverErr != nil || recovered != 1 {
		t.Fatalf("recovered=%d error=%v", recovered, recoverErr)
	}
	if err = service.ForgotPassword(context.Background(), "unlinked@example.com", "telegram", "192.0.2.13", "request-telegram-unlinked"); err != nil {
		t.Fatal(err)
	}

	emailRegistration := account.Registration{Login: "email.channel", Email: "email-channel@example.com", FirstName: "Email", ProfessionalRoleCode: "customer", Password: password, PasswordConfirmation: password, RequestID: "request-email-channel-register"}
	if _, err = service.Register(context.Background(), emailRegistration); err != nil {
		t.Fatal(err)
	}
	if handoff, selectErr := service.SelectVerificationChannel(context.Background(), emailRegistration.Login, "email", "request-email-channel-select"); selectErr != nil || handoff != "" {
		t.Fatalf("email handoff=%q error=%v", handoff, selectErr)
	}
	var emailOutbox int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM notification_outbox outbox JOIN users ON users.id=outbox.recipient_user_id WHERE users.login_normalized='email.channel' AND outbox.channel='email'`).Scan(&emailOutbox); err != nil || emailOutbox != 1 {
		t.Fatalf("email outbox=%d error=%v", emailOutbox, err)
	}

	superRegistration := account.Registration{Login: "telegram.admin", Email: "telegram-admin@example.com", FirstName: "Admin", ProfessionalRoleCode: "designer", Password: password, PasswordConfirmation: password, RequestID: "request-telegram-admin-register"}
	if _, err = service.Register(context.Background(), superRegistration); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE users SET global_role='super_admin' WHERE login_normalized='telegram.admin'`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SelectVerificationChannel(context.Background(), superRegistration.Login, "telegram", "request-telegram-admin-channel"); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		requestSuffix := fmt.Sprint(index)
		if err = service.BeginTelegramConfirmation(context.Background(), 9010, "telegram-admin-start-"+requestSuffix); err != nil {
			t.Fatal(err)
		}
		if err = service.SubmitTelegramLogin(context.Background(), 9010, superRegistration.Login); err != nil {
			t.Fatal(err)
		}
		if err = service.CompleteTelegramConfirmation(context.Background(), 9010, int64(70+index), "admin_chat", password, "telegram-admin-done-"+requestSuffix); err != nil {
			t.Fatal(err)
		}
	}
	var adminSubscriptions, activeAdminBindings int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM telegram_subscribers WHERE chat_id=9010 AND unsubscribed_at IS NULL`).Scan(&adminSubscriptions); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM telegram_account_bindings WHERE chat_id=9010 AND revoked_at IS NULL`).Scan(&activeAdminBindings); err != nil {
		t.Fatal(err)
	}
	if adminSubscriptions != 1 || activeAdminBindings != 1 {
		t.Fatalf("admin subscriptions=%d active bindings=%d", adminSubscriptions, activeAdminBindings)
	}
	if err = store.CompleteTelegramAuth(context.Background(), account.TelegramAuthCompletion{ChatID: 9010, MessageID: 99, UserID: "00000000-0000-0000-0000-000000000000", Now: now, RequestID: "telegram-admin-invalid"}); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("invalid completion error=%v", err)
	}
	if found, selectErr := store.SetVerificationChannel(context.Background(), account.VerificationChannelSelection{IdentifierNormalized: "absent@example.com", Channel: "email", RequestID: "request-absent-selection", Now: now}); selectErr != nil || found {
		t.Fatalf("absent selection found=%v error=%v", found, selectErr)
	}
	if err = store.AdvanceTelegramAuth(context.Background(), account.TelegramAuthAdvance{ChatID: 9999, Now: now}); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("missing Telegram dialog advance error=%v", err)
	}
}

func TestTelegramRepositoryReportsClosedDatabaseErrors(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_telegram_closed_pool")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	pool.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	checks := []struct {
		name string
		run  func() error
	}{
		{"create session", func() error { return store.CreateSession(ctx, account.NewSession{}) }},
		{"bootstrap admin", func() error {
			_, _, err := store.BootstrapSuperAdminSession(ctx, account.BootstrapSession{})
			return err
		}},
		{"rotate session", func() error { _, err := store.RotateSession(ctx, account.SessionRotation{}); return err }},
		{"revoke session", func() error { return store.RevokeSessionFamily(ctx, account.SessionRevocation{}) }},
		{"consume rate limit", func() error { _, err := store.ConsumeRateLimit(ctx, account.RateLimitRequest{}); return err }},
		{"create account", func() error { return store.CreatePendingAccount(ctx, account.PendingAccount{}) }},
		{"set channel", func() error {
			_, err := store.SetVerificationChannel(ctx, account.VerificationChannelSelection{IdentifierNormalized: "user", Channel: "telegram", RequestID: "request-closed", Now: now})
			return err
		}},
		{"begin auth", func() error {
			return store.BeginTelegramAuth(ctx, account.TelegramAuthStart{ChatID: 1, Flow: "registration_confirmation", RequestID: "request-closed", StartedAt: now, ExpiresAt: now.Add(5 * time.Minute)})
		}},
		{"advance auth", func() error { return store.AdvanceTelegramAuth(ctx, account.TelegramAuthAdvance{ChatID: 1, Now: now}) }},
		{"load step", func() error { _, err := store.TelegramAuthStep(ctx, 1, now); return err }},
		{"load account", func() error { _, _, err := store.TelegramAuthAccount(ctx, 1, now); return err }},
		{"complete auth", func() error {
			return store.CompleteTelegramAuth(ctx, account.TelegramAuthCompletion{ChatID: 1, MessageID: 1, UserID: "user", Now: now, RequestID: "request-closed"})
		}},
		{"reject auth", func() error {
			return store.RejectTelegramAuth(ctx, account.TelegramAuthRejection{ChatID: 1, Now: now, RequestID: "request-closed", Reason: "invalid_credentials"})
		}},
		{"create reset", func() error {
			_, err := store.CreateTelegramPasswordReset(ctx, account.PasswordResetIssue{IdentifierNormalized: "user", RequestID: "request-closed"})
			return err
		}},
		{"recover outbox", func() error { _, err := store.RecoverStaleTelegramAccountOutbox(ctx, now, now); return err }},
		{"claim outbox", func() error { _, _, err := store.ClaimTelegramAccountOutbox(ctx, now); return err }},
		{"deliver outbox", func() error { return store.MarkTelegramAccountOutboxDelivered(ctx, "missing", now) }},
		{"fail outbox", func() error { return store.MarkTelegramAccountOutboxFailed(ctx, "missing", 1, 5, now, now, "closed") }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("closed PostgreSQL pool must return an error")
			}
		})
	}
}

func TestAccountRegistrationRollsBackForUnknownRole(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_account_rollback")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	err := store.CreatePendingAccount(context.Background(), account.PendingAccount{
		Login: "invalid.role", LoginNormalized: "invalid.role", Email: "invalid@example.com", EmailNormalized: "invalid@example.com",
		FirstName: "Invalid", ProfessionalRoleCode: "missing", PasswordHash: "$argon2id$test", PasswordAlgorithm: "argon2id", PasswordParameters: map[string]any{}, PasswordHashVersion: 1,
		TokenHash: []byte("hash"), TokenKeyVersion: 1, TokenExpiresAt: time.Now().Add(time.Hour), OutboxCiphertext: []byte("cipher"), OutboxKeyVersion: 1,
		OutboxIdempotencyKey: "rollback-test", RequestID: "request-rollback",
	})
	if !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("error=%v, want invalid role", err)
	}
	assertAccountTransactionRows(t, pool, 0, 0, 0, 0, 0, 0)
}

func TestAccountSessionRotationReuseAndLogoutIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_sessions")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	tokens, _ := account.NewTokenManager(bytes.Repeat([]byte{3}, 32), 1)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{4}, 32), 1)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "from@example.com", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	registration := account.Registration{Login: "session.user", Email: "session@example.com", FirstName: "Сессия", ProfessionalRoleCode: "customer", Password: "Сессионный пароль 2026!", PasswordConfirmation: "Сессионный пароль 2026!", RequestID: "request-session-register"}
	if _, err = service.Register(context.Background(), registration); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE email_verification_tokens SET consumed_at=$1`, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE users SET status='active',email_verified_at=$1 WHERE login_normalized='session.user'`, now); err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(context.Background(), account.LoginRequest{Identifier: "session.user", Password: "Сессионный пароль 2026!", RememberMe: true, RequestID: "request-session-login"})
	if err != nil {
		t.Fatal(err)
	}
	var accountBuckets, markerBuckets int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE policy_code='login.account'),count(*) FILTER (WHERE policy_code='login.marker') FROM auth_rate_limit_buckets`).Scan(&accountBuckets, &markerBuckets); err != nil || accountBuckets != 0 || markerBuckets != 1 {
		t.Fatalf("account buckets=%d marker buckets=%d error=%v", accountBuckets, markerBuckets, err)
	}
	var storedAccess, storedRefresh, storedCSRF []byte
	var sessionUserID string
	var securityVersion int64
	if err = pool.QueryRow(context.Background(), `SELECT access_token_hash,refresh_token_hash,csrf_token_hash,user_id::text,captured_security_version FROM sessions WHERE revoked_at IS NULL`).Scan(&storedAccess, &storedRefresh, &storedCSRF, &sessionUserID, &securityVersion); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(storedAccess, []byte(session.AccessToken)) || bytes.Contains(storedRefresh, []byte(session.RefreshToken)) {
		t.Fatal("raw session token stored in database")
	}
	duplicateSession := account.NewSession{UserID: sessionUserID, RequestID: "request-duplicate", AccessHash: storedAccess, RefreshHash: storedRefresh, CSRFHash: storedCSRF, HashKeyVersion: 1, SecurityVersion: securityVersion, Now: now, AccessExpiresAt: now.Add(30 * time.Minute), RefreshExpiresAt: now.Add(90 * 24 * time.Hour)}
	if err = store.CreateSession(context.Background(), duplicateSession); err == nil {
		t.Fatal("duplicate session hashes must fail")
	}
	invalidAuditSession := duplicateSession
	invalidAuditSession.AccessHash = []byte("unique-access-hash-00000000000001")
	invalidAuditSession.RefreshHash = []byte("unique-refresh-hash-000000000001")
	invalidAuditSession.CSRFHash = []byte("unique-csrf-hash-00000000000001")
	invalidAuditSession.RequestID = "short"
	if err = store.CreateSession(context.Background(), invalidAuditSession); err == nil {
		t.Fatal("invalid login audit must roll back")
	}
	badRotation := account.SessionRotation{RefreshHash: storedRefresh, CSRFHash: storedCSRF, NewAccessHash: []byte("rotation-access-00000000000000001"), NewRefreshHash: []byte("rotation-refresh-000000000000001"), NewCSRFHash: []byte("rotation-csrf-00000000000000001"), HashKeyVersion: 1, Now: now, AccessExpiresAt: now.Add(30 * time.Minute), RequestID: "short"}
	if _, err = store.RotateSession(context.Background(), badRotation); err == nil {
		t.Fatal("invalid refresh audit must roll back")
	}
	if err = store.RevokeSessionFamily(context.Background(), account.SessionRevocation{AccessHash: storedAccess, RefreshHash: []byte("none"), CSRFHash: storedCSRF, Now: now, RequestID: "short"}); err == nil {
		t.Fatal("invalid logout audit must roll back")
	}
	if _, err = service.Refresh(context.Background(), session.RefreshToken, strings.Repeat("w", 43), "request-session-wrong-csrf"); !errors.Is(err, account.ErrUnauthenticated) {
		t.Fatalf("wrong csrf refresh error=%v", err)
	}
	if _, err = service.Refresh(context.Background(), session.RefreshToken, session.CSRFToken, "request-session-refresh"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Refresh(context.Background(), strings.Repeat("x", 43), strings.Repeat("c", 43), "request-session-missing"); !errors.Is(err, account.ErrUnauthenticated) {
		t.Fatalf("missing refresh error=%v", err)
	}
	if _, err = service.Refresh(context.Background(), session.RefreshToken, session.CSRFToken, "request-session-reuse"); !errors.Is(err, account.ErrUnauthenticated) {
		t.Fatalf("reuse error=%v", err)
	}
	var activeFamily int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE revoked_at IS NULL`).Scan(&activeFamily); err != nil || activeFamily != 0 {
		t.Fatalf("active sessions=%d error=%v", activeFamily, err)
	}
	second, err := service.Login(context.Background(), account.LoginRequest{Identifier: "session@example.com", Password: "Сессионный пароль 2026!", RequestID: "request-session-login-2"})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Logout(context.Background(), second.AccessToken, "", second.CSRFToken, "request-session-logout"); err != nil {
		t.Fatal(err)
	}
	if err = service.Logout(context.Background(), second.AccessToken, "", strings.Repeat("w", 43), "request-session-bad-csrf"); !errors.Is(err, account.ErrUnauthenticated) {
		t.Fatalf("bad csrf logout error=%v", err)
	}
	if err = service.Logout(context.Background(), second.AccessToken, "", second.CSRFToken, "request-session-logout-repeat"); err != nil {
		t.Fatal(err)
	}
	if err = service.Logout(context.Background(), strings.Repeat("z", 43), "", strings.Repeat("c", 43), "request-session-missing-logout"); err != nil {
		t.Fatalf("missing logout error=%v", err)
	}
	if _, found, err := store.FindLoginAccount(context.Background(), "missing@example.com"); err != nil || found {
		t.Fatalf("missing account found=%v error=%v", found, err)
	}
	if err = store.CreateSession(context.Background(), account.NewSession{UserID: "00000000-0000-0000-0000-000000000099", SecurityVersion: 1}); !errors.Is(err, account.ErrAccountUnavailable) {
		t.Fatalf("missing user session error=%v", err)
	}
	if err = store.RecordLoginFailure(context.Background(), "unsupported", "request-unsupported"); err == nil {
		t.Fatal("unsupported audit result must fail")
	}
}

func TestPasswordResetExpiresIsSingleUseAndRevokesSessions(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_password_reset")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	tokens, _ := account.NewTokenManager(bytes.Repeat([]byte{12}, 32), 1)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{13}, 32), 1)
	now := time.Now().UTC().Truncate(time.Second)
	currentTime := now
	service, err := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "from@example.com", func() time.Time { return currentTime })
	if err != nil {
		t.Fatal(err)
	}
	registration := account.Registration{Login: "reset.user", Email: "reset@example.com", FirstName: "Reset", ProfessionalRoleCode: "customer", Password: "Исходный пароль 2026!", PasswordConfirmation: "Исходный пароль 2026!", RequestID: "request-reset-register"}
	if _, err = service.Register(context.Background(), registration); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE email_verification_tokens SET consumed_at=$1`, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE users SET status='active',email_verified_at=$1 WHERE login_normalized='reset.user'`, now); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: "reset.user", Password: registration.Password, RequestID: "request-reset-login"}); err != nil {
		t.Fatal(err)
	}
	if err = service.ForgotPassword(context.Background(), "reset.user", "email", "192.0.2.1", "request-reset-expired"); err != nil {
		t.Fatal(err)
	}
	expiredToken := passwordResetTokenFromLatestOutbox(t, pool, cipher)
	currentTime = now.Add(31 * time.Minute)
	if err = service.ResetPassword(context.Background(), expiredToken, "Новый пароль 2026 надёжный!", "Новый пароль 2026 надёжный!", "192.0.2.1", "request-reset-expired-use"); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("expired token error=%v", err)
	}
	currentTime = now
	if err = service.ForgotPassword(context.Background(), "RESET@EXAMPLE.COM", "email", "192.0.2.1", "request-reset-valid"); err != nil {
		t.Fatal(err)
	}
	validToken := passwordResetTokenFromLatestOutbox(t, pool, cipher)
	newPassword := "Новый пароль 2026 надёжный!"
	if err = service.ResetPassword(context.Background(), validToken, newPassword, newPassword, "192.0.2.1", "request-reset-complete"); err != nil {
		t.Fatal(err)
	}
	if err = service.ResetPassword(context.Background(), validToken, "Другой новый пароль 2026!", "Другой новый пароль 2026!", "192.0.2.1", "request-reset-reuse"); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("reused token error=%v", err)
	}
	var activeSessions, securityVersion, completedAudits int
	if err = pool.QueryRow(context.Background(), `
		SELECT count(*) FILTER (WHERE sessions.revoked_at IS NULL),max(users.security_version),
		       count(*) FILTER (WHERE audit_events.event_type='auth.password_reset_completed' AND audit_events.result='success')
		  FROM users
		  LEFT JOIN sessions ON sessions.user_id=users.id
		  LEFT JOIN audit_events ON audit_events.subject_user_id=users.id
		 WHERE users.login_normalized='reset.user'
	`).Scan(&activeSessions, &securityVersion, &completedAudits); err != nil {
		t.Fatal(err)
	}
	if activeSessions != 0 || securityVersion != 2 || completedAudits < 1 {
		t.Fatalf("active sessions=%d security version=%d completed audits=%d", activeSessions, securityVersion, completedAudits)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: "reset.user", Password: registration.Password, RequestID: "request-reset-old-password"}); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatalf("old password error=%v", err)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: "reset.user", Password: newPassword, RequestID: "request-reset-new-password"}); err != nil {
		t.Fatal(err)
	}
	if err = service.ForgotPassword(context.Background(), "absent@example.com", "email", "192.0.2.2", "request-reset-absent"); err != nil {
		t.Fatal(err)
	}
	var absentOutbox int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM notification_outbox WHERE message_type='account.password_reset' AND recipient_user_id IS NULL`).Scan(&absentOutbox); err != nil || absentOutbox != 0 {
		t.Fatalf("absent outbox=%d error=%v", absentOutbox, err)
	}
}

func passwordResetTokenFromLatestOutbox(t *testing.T, pool *pgxpool.Pool, cipher account.PayloadCipher) string {
	t.Helper()
	var ciphertext []byte
	var keyVersion int
	if err := pool.QueryRow(context.Background(), `SELECT payload_ciphertext,payload_key_version FROM notification_outbox WHERE message_type='account.password_reset' ORDER BY created_at DESC LIMIT 1`).Scan(&ciphertext, &keyVersion); err != nil {
		t.Fatal(err)
	}
	payload, err := cipher.Decrypt(ciphertext, passwordResetMessageType, keyVersion)
	if err != nil {
		t.Fatal(err)
	}
	var email account.VerificationEmail
	if err = json.Unmarshal(payload, &email); err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(email.Text, "/reset-password#token=", 2)
	if len(parts) != 2 {
		t.Fatalf("reset token missing from email: %q", email.Text)
	}
	return strings.SplitN(parts[1], "\n", 2)[0]
}

func TestSuperAdminBootstrapLinksExistingPendingAccount(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_bootstrap_link")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	tokens, _ := account.NewTokenManager(bytes.Repeat([]byte{10}, 32), 1)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{11}, 32), 1)
	now := time.Now().UTC().Truncate(time.Second)
	service, _ := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "no-reply@designer-svetlana.ru", func() time.Time { return now })
	registration := account.Registration{Login: "root.link", Email: "root.link@example.com", FirstName: "Root", ProfessionalRoleCode: "designer", Password: "Старый надёжный пароль!", PasswordConfirmation: "Старый надёжный пароль!", RequestID: "request-link-register"}
	if _, err := service.Register(context.Background(), registration); err != nil {
		t.Fatal(err)
	}
	newHash, err := (account.PasswordHasher{}).Hash("Bootstrap пароль 2026!", "root.link", "root.link@example.com")
	if err != nil {
		t.Fatal(err)
	}
	access, accessHash, version, _ := tokens.NewForPurpose("arhdesign/access-session/v1")
	_, refreshHash, _, _ := tokens.NewForPurpose("arhdesign/refresh-session/v1")
	_, csrfHash, _, _ := tokens.NewForPurpose("arhdesign/csrf-session/v1")
	_ = access
	linked, created, err := store.BootstrapSuperAdminSession(context.Background(), account.BootstrapSession{Login: "root.link", Email: "root.link@example.com", FirstName: "Root", ProfessionalRoleCode: "designer", PasswordHash: newHash.PHC, PasswordAlgorithm: newHash.Algorithm, PasswordParameters: newHash.Parameters, PasswordHashVersion: newHash.Version, Session: account.NewSession{RequestID: "request-link-bootstrap", AccessHash: accessHash, RefreshHash: refreshHash, CSRFHash: csrfHash, HashKeyVersion: version, Now: now, AccessExpiresAt: now.Add(30 * time.Minute), RefreshExpiresAt: now.Add(90 * 24 * time.Hour)}})
	if err != nil || !created || linked.GlobalRole == nil || *linked.GlobalRole != "super_admin" {
		t.Fatalf("linked=%#v created=%v error=%v", linked, created, err)
	}
	found, ok, err := store.FindLoginAccount(context.Background(), "root.link")
	if err != nil || !ok {
		t.Fatal(err)
	}
	matched, err := (account.PasswordHasher{}).Compare(found.PasswordHash, "Bootstrap пароль 2026!")
	if err != nil || !matched {
		t.Fatalf("bootstrap credential match=%v error=%v", matched, err)
	}
}

func TestAdminUsersDisableRestoreRevokesSessionsAndProtectsLastSuperAdmin(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_admin_users")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	insertUser := func(login, globalRole string) string {
		t.Helper()
		var id string
		err := pool.QueryRow(ctx, `
			WITH created AS (
				INSERT INTO users (login, login_normalized, email, email_normalized, status, global_role, email_verified_at)
				VALUES ($1, $1, $1 || '@example.com', $1 || '@example.com', 'active', NULLIF($2, ''), $3)
				RETURNING id
			), profile AS (
				INSERT INTO profiles (user_id, professional_role_id, first_name)
				SELECT created.id, role.id, $1 FROM created CROSS JOIN professional_roles role WHERE role.code='designer'
			)
			SELECT id::text FROM created`, login, globalRole, now).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	adminOne := insertUser("admin.one", "super_admin")
	adminTwo := insertUser("admin.two", "super_admin")
	userID := insertUser("client.one", "")
	accessHash := []byte("admin-users-access-hash")
	if _, err := pool.Exec(ctx, `
		INSERT INTO sessions (user_id, family_id, access_token_hash, refresh_token_hash, csrf_token_hash,
			hash_key_version, captured_security_version, idle_expires_at, absolute_expires_at)
		VALUES ($1, gen_random_uuid(), $2, 'refresh'::bytea, 'csrf'::bytea, 1, 1, $3, $4)`,
		userID, accessHash, now.Add(time.Hour), now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	items, err := store.ListAdminUsers(ctx, account.AdminUserQuery{Limit: 10, Identifier: "client.one"})
	if err != nil || len(items) != 1 || items[0].ID != userID {
		t.Fatalf("items=%#v error=%v", items, err)
	}
	disabled, err := store.SetAdminUserStatus(ctx, account.AdminUserStatusCommand{
		ActorUserID: adminOne, TargetUserID: userID, RequestID: "request-disable-user", Disable: true, ExpectedVersion: 1, Now: now,
	})
	if err != nil || disabled.Status != "disabled" || disabled.Version != 2 {
		t.Fatalf("disabled=%#v error=%v", disabled, err)
	}
	if _, ok, err := store.ResolveAccessSession(ctx, accessHash, now); err != nil || ok {
		t.Fatalf("disabled session ok=%v error=%v", ok, err)
	}
	if _, err := store.SetAdminUserStatus(ctx, account.AdminUserStatusCommand{
		ActorUserID: adminOne, TargetUserID: userID, RequestID: "request-disable-repeat", Disable: true, ExpectedVersion: 1, Now: now,
	}); err != nil {
		t.Fatalf("idempotent disable: %v", err)
	}
	restored, err := store.SetAdminUserStatus(ctx, account.AdminUserStatusCommand{
		ActorUserID: adminOne, TargetUserID: userID, RequestID: "request-restore-user", ExpectedVersion: 2, Now: now.Add(time.Second),
	})
	if err != nil || restored.Status != "active" || restored.Version != 3 {
		t.Fatalf("restored=%#v error=%v", restored, err)
	}
	if _, err := store.SetAdminUserStatus(ctx, account.AdminUserStatusCommand{
		ActorUserID: adminTwo, TargetUserID: adminOne, RequestID: "request-disable-admin-one", Disable: true, ExpectedVersion: 1, Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetAdminUserStatus(ctx, account.AdminUserStatusCommand{
		ActorUserID: adminOne, TargetUserID: adminTwo, RequestID: "request-disable-last-admin", Disable: true, ExpectedVersion: 1, Now: now,
	}); !errors.Is(err, account.ErrLastSuperAdmin) {
		t.Fatalf("last super-admin error=%v", err)
	}
}

func TestAccountRepositoryPropagatesCancelledContext(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_cancelled")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := store.FindLoginAccount(ctx, "user"); err == nil {
		t.Fatal("find must propagate cancellation")
	}
	if err := store.CreateSession(ctx, account.NewSession{}); err == nil {
		t.Fatal("create session must propagate cancellation")
	}
	if _, _, err := store.BootstrapSuperAdminSession(ctx, account.BootstrapSession{}); err == nil {
		t.Fatal("bootstrap must propagate cancellation")
	}
	if _, err := store.RotateSession(ctx, account.SessionRotation{}); err == nil {
		t.Fatal("rotation must propagate cancellation")
	}
	if err := store.RevokeSessionFamily(ctx, account.SessionRevocation{}); err == nil {
		t.Fatal("logout must propagate cancellation")
	}
	if err := store.RecordLoginFailure(ctx, "invalid_credentials", "request-cancelled"); err == nil {
		t.Fatal("audit must propagate cancellation")
	}
	if _, err := store.ConsumeRateLimit(ctx, account.RateLimitRequest{PolicyCode: "login.marker", SubjectHash: []byte("hash"), HashKeyVersion: 1, Capacity: 1, FullRefill: time.Minute, Retention: time.Hour, Now: time.Now()}); err == nil {
		t.Fatal("limiter must propagate cancellation")
	}
	if _, _, err := store.ActiveAccountEmail(ctx, "user@example.com"); err == nil {
		t.Fatal("reset email lookup must propagate cancellation")
	}
	if _, err := store.CreatePasswordReset(ctx, account.PasswordResetIssue{}); err == nil {
		t.Fatal("reset issue must propagate cancellation")
	}
	if _, err := store.PasswordResetContext(ctx, []byte("hash"), 1, time.Now(), "request-cancelled"); err == nil {
		t.Fatal("reset context must propagate cancellation")
	}
	if err := store.CompletePasswordReset(ctx, account.PasswordResetCompletion{}); err == nil {
		t.Fatal("reset completion must propagate cancellation")
	}
	if err := store.RecordPasswordResetRequest(ctx, "rate_limited", "request-cancelled"); err == nil {
		t.Fatal("reset request audit must propagate cancellation")
	}
	if err := store.RecordPasswordResetFailure(ctx, "request-cancelled"); err == nil {
		t.Fatal("reset failure audit must propagate cancellation")
	}
	if err := store.RecordPasswordResetRequest(context.Background(), "unsupported", "request-unsupported"); err == nil {
		t.Fatal("unsupported reset request audit must fail")
	}
	if err := store.RecordPasswordResetRequest(context.Background(), "rate_limited", "request-reset-limited"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePasswordReset(context.Background(), account.PasswordResetIssue{IdentifierNormalized: "absent@example.com", RequestID: "short"}); err == nil {
		t.Fatal("invalid password-reset request audit must roll back")
	}
	if err := store.CompletePasswordReset(context.Background(), account.PasswordResetCompletion{PasswordParameters: map[string]any{"invalid": make(chan int)}}); err == nil {
		t.Fatal("unencodable reset password parameters must fail")
	}
	if err := store.CompletePasswordReset(context.Background(), account.PasswordResetCompletion{TokenHash: []byte("missing-reset-token"), TokenKeyVersion: 1, PasswordParameters: map[string]any{}, Now: time.Now(), RequestID: "request-missing-reset"}); !errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("missing reset completion error=%v", err)
	}
	if err := store.CompletePasswordReset(context.Background(), account.PasswordResetCompletion{TokenHash: []byte("missing-reset-token"), TokenKeyVersion: 1, PasswordParameters: map[string]any{}, Now: time.Now(), RequestID: "short"}); err == nil || errors.Is(err, account.ErrInvalidToken) {
		t.Fatalf("invalid rejected-reset audit error=%v", err)
	}
}

func TestRateLimitBucketConcurrencyAndRefill(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_rate_limit")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	request := account.RateLimitRequest{PolicyCode: "login.account", SubjectHash: []byte("same-subject-hash"), HashKeyVersion: 1, Capacity: 5, FullRefill: 15 * time.Minute, Retention: 48 * time.Hour, Now: now}
	results := make(chan time.Duration, 20)
	failures := make(chan error, 20)
	var group sync.WaitGroup
	for index := 0; index < 20; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			retry, err := store.ConsumeRateLimit(context.Background(), request)
			if err != nil {
				failures <- err
				return
			}
			results <- retry
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	accepted := 0
	for retry := range results {
		if retry == 0 {
			accepted++
		} else if retry != 3*time.Minute {
			t.Fatalf("retry=%v want 3m", retry)
		}
	}
	if accepted != 5 {
		t.Fatalf("accepted=%d want 5", accepted)
	}
	refilled := request
	refilled.Now = now.Add(3 * time.Minute)
	if retry, err := store.ConsumeRateLimit(context.Background(), refilled); err != nil || retry != 0 {
		t.Fatalf("refill retry=%v error=%v", retry, err)
	}
	blockedHash := []byte("blocked-subject-hash")
	blockedUntil := now.Add(2 * time.Minute)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO auth_rate_limit_buckets (policy_code,subject_hash,hash_key_version,tokens,last_refill_at,blocked_until,expires_at)
		VALUES ('login.account',$1,1,0,$2::timestamptz,$3::timestamptz,$3::timestamptz + interval '1 hour')
	`, blockedHash, now, blockedUntil); err != nil {
		t.Fatal(err)
	}
	blocked := request
	blocked.SubjectHash = blockedHash
	if retry, err := store.ConsumeRateLimit(context.Background(), blocked); err != nil || retry != 2*time.Minute {
		t.Fatalf("blocked retry=%v error=%v", retry, err)
	}
	if _, err := store.ConsumeRateLimit(context.Background(), account.RateLimitRequest{}); err == nil {
		t.Fatal("invalid policy must fail")
	}
}

func TestSuperAdminBootstrapIsOneTimeAndUsesNormalCredential(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_bootstrap")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	tokens, _ := account.NewTokenManager(bytes.Repeat([]byte{8}, 32), 1)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{9}, 32), 1)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "no-reply@designer-svetlana.ru", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureBootstrap("root.admin", "Bootstrap пароль 2026!")
	first, err := service.Login(context.Background(), account.LoginRequest{Identifier: "root.admin", Password: "Bootstrap пароль 2026!", RequestID: "request-bootstrap-first"})
	if err != nil || first.Account.GlobalRole == nil || *first.Account.GlobalRole != "super_admin" {
		t.Fatalf("session=%#v error=%v", first, err)
	}
	var passwordHash string
	if err = pool.QueryRow(context.Background(), `SELECT password_hash FROM credentials JOIN users ON users.id=credentials.user_id WHERE users.login_normalized='root.admin'`).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if passwordHash == "Bootstrap пароль 2026!" || !strings.HasPrefix(passwordHash, "$argon2id$") {
		t.Fatalf("bootstrap credential was not hashed: %q", passwordHash)
	}
	second, err := service.Login(context.Background(), account.LoginRequest{Identifier: "root.admin", Password: "Bootstrap пароль 2026!", RequestID: "request-bootstrap-second"})
	if err != nil || second.Account.ID != first.Account.ID {
		t.Fatalf("second session=%#v error=%v", second, err)
	}
	var users, admins int
	if err = pool.QueryRow(context.Background(), `SELECT count(*),count(*) FILTER (WHERE global_role='super_admin') FROM users`).Scan(&users, &admins); err != nil || users != 1 || admins != 1 {
		t.Fatalf("users=%d admins=%d error=%v", users, admins, err)
	}
	if err = store.RecordLoginFailure(context.Background(), "invalid_credentials", "request-bootstrap-failed"); err != nil {
		t.Fatal(err)
	}
	if _, created, err := store.BootstrapSuperAdminSession(context.Background(), account.BootstrapSession{}); err != nil || created {
		t.Fatalf("repeat bootstrap created=%v error=%v", created, err)
	}
}

func assertAccountTransactionRows(t *testing.T, pool *pgxpool.Pool, users, profiles, credentials, tokens, audits, outbox int) {
	t.Helper()
	queries := []struct {
		name string
		want int
	}{
		{"users", users}, {"profiles", profiles}, {"credentials", credentials}, {"email_verification_tokens", tokens}, {"audit_events", audits}, {"notification_outbox", outbox},
	}
	for _, query := range queries {
		var got int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+query.name).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != query.want {
			t.Fatalf("%s rows=%d, want %d", query.name, got, query.want)
		}
	}
}
