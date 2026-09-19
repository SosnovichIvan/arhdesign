//go:build integration

package repository

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStoreIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for the isolated PostgreSQL integration test")
	}
	if !strings.Contains(strings.ToLower(databaseURL), "test") {
		t.Fatal("TEST_DATABASE_URL must point to an isolated database whose name contains 'test'")
	}

	context := context.Background()
	pool, err := pgxpool.New(context, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	migrationEntries, err := os.ReadDir(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(migrationEntries, func(left, right int) bool { return migrationEntries[left].Name() < migrationEntries[right].Name() })
	for _, entry := range migrationEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		migration, readErr := os.ReadFile(filepath.Join("..", "..", "migrations", entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := pool.Exec(context, string(migration)); execErr != nil {
			t.Fatal(execErr)
		}
	}
	if _, err := pool.Exec(context, "TRUNCATE telegram_notification_receipts, contact_consents, contact_submissions, contact_cooldowns, telegram_subscribers"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context, "TRUNCATE telegram_notification_receipts, contact_consents, contact_submissions, contact_cooldowns, telegram_subscribers")
	})

	store := NewPostgres(pool)
	now := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	if err := store.ActivateTelegramSubscriber(context, 123, "admin"); err != nil {
		t.Fatal(err)
	}
	if active, err := store.TelegramSubscriberActive(context, 123); err != nil || !active {
		t.Fatalf("active subscriber = %v, error = %v; want true and nil", active, err)
	}
	chatIDs, err := store.ActiveTelegramChatIDs(context)
	if err != nil || len(chatIDs) != 1 || chatIDs[0] != 123 {
		t.Fatalf("active chat IDs = %#v, error = %v", chatIDs, err)
	}
	if err := store.DeactivateTelegramSubscriber(context, 123); err != nil {
		t.Fatal(err)
	}
	if active, err := store.TelegramSubscriberActive(context, 123); err != nil || active {
		t.Fatalf("active subscriber after deactivation = %v, error = %v; want false and nil", active, err)
	}
	chatIDs, err = store.ActiveTelegramChatIDs(context)
	if err != nil || len(chatIDs) != 0 {
		t.Fatalf("active chat IDs after deactivation = %#v, error = %v", chatIDs, err)
	}
	tokenHash := []byte("test-token-hash")
	if err := store.CreateSubmissionAndSetCooldown(context, ContactSubmission{
		Name:                  "Анна Иванова",
		Contact:               "anna@example.com",
		ProjectType:           "Квартира",
		ProjectDetails:        "Нужен проект квартиры",
		ConsentGranted:        true,
		ConsentMethod:         ConsentMethod,
		ConsentSourceURL:      "https://designer-svetlana.ru/#contact",
		ConsentText:           ConsentText,
		ConsentVersion:        ConsentVersion,
		ConsentDocumentPath:   ConsentDocumentPath,
		ConsentDocumentSHA256: ConsentDocumentSHA256,
	}, tokenHash, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	var submissionCount int
	if err := pool.QueryRow(context, "SELECT count(*) FROM contact_submissions").Scan(&submissionCount); err != nil {
		t.Fatal(err)
	}
	if submissionCount != 1 {
		t.Fatalf("submission count = %d, want 1", submissionCount)
	}
	var consentVersion, documentPath, documentSHA256, consentMethod, consentText, sourceURL string
	var consentGranted bool
	if err := pool.QueryRow(context, "SELECT consent_version, document_path, document_sha256, consent_granted, consent_method, consent_text, source_url FROM contact_consents").Scan(&consentVersion, &documentPath, &documentSHA256, &consentGranted, &consentMethod, &consentText, &sourceURL); err != nil {
		t.Fatal(err)
	}
	if consentVersion != ConsentVersion || documentPath != ConsentDocumentPath || documentSHA256 != ConsentDocumentSHA256 || !consentGranted || consentMethod != ConsentMethod || consentText != ConsentText || sourceURL == "" {
		t.Fatalf("unexpected consent audit: version=%q path=%q sha=%q granted=%v method=%q text=%q source=%q", consentVersion, documentPath, documentSHA256, consentGranted, consentMethod, consentText, sourceURL)
	}
	if retryAfter, err := store.CooldownRetryAfter(context, tokenHash, now); err != nil || retryAfter != time.Hour {
		t.Fatalf("retry after = %v, error = %v; want 1h and nil", retryAfter, err)
	}
	if retryAfter, err := store.CooldownRetryAfter(context, tokenHash, now.Add(time.Hour)); err != nil || retryAfter != 0 {
		t.Fatalf("expired cooldown retry after = %v, error = %v; want 0 and nil", retryAfter, err)
	}
	if _, err := pool.Exec(context, "INSERT INTO contact_cooldowns (token_hash, expires_at) VALUES ($1, $2)", []byte("expired-token"), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := store.DeleteExpiredCooldowns(context, now); err != nil || deleted != 1 {
		t.Fatalf("deleted cooldowns = %d, error = %v; want 1 and nil", deleted, err)
	}
	if err := store.RecordTelegramNotification(context, 123, 42, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	due, err := store.DueTelegramNotifications(context, now.Add(2*time.Hour), 100)
	if err != nil || len(due) != 1 || due[0].ChatID != 123 || due[0].MessageID != 42 {
		t.Fatalf("due notifications = %#v, error = %v", due, err)
	}
	if err := store.MarkTelegramNotificationDeleted(context, due[0].ID, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	due, err = store.DueTelegramNotifications(context, now.Add(3*time.Hour), 100)
	if err != nil || len(due) != 0 {
		t.Fatalf("deleted notification remained due: %#v, error = %v", due, err)
	}
	if _, err := pool.Exec(context, "UPDATE contact_submissions SET created_at = $1", now.AddDate(0, 0, -366)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := store.DeleteSubmissionsOlderThan(context, now.AddDate(0, 0, -365)); err != nil || deleted != 1 {
		t.Fatalf("deleted = %d, error = %v; want 1 and nil", deleted, err)
	}
	var remainingConsents int
	if err := pool.QueryRow(context, "SELECT count(*) FROM contact_consents").Scan(&remainingConsents); err != nil {
		t.Fatal(err)
	}
	if remainingConsents != 0 {
		t.Fatalf("remaining consent records = %d, want 0 after submission retention cleanup", remainingConsents)
	}
}

func TestPostgresStoreReturnsErrorsFromClosedPool(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for the isolated PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	store := NewPostgres(pool)
	ctx := context.Background()
	if _, err := store.DeleteSubmissionsOlderThan(ctx, time.Now()); err == nil {
		t.Fatal("delete must report a closed-pool error")
	}
	if _, err := store.DeleteExpiredCooldowns(ctx, time.Now()); err == nil {
		t.Fatal("cooldown deletion must report a closed-pool error")
	}
	if err := store.CreateSubmissionAndSetCooldown(ctx, ContactSubmission{}, []byte("token"), time.Now()); err == nil {
		t.Fatal("create must report a closed-pool error")
	}
	if _, err := store.CooldownRetryAfter(ctx, []byte("token"), time.Now()); err == nil {
		t.Fatal("cooldown lookup must report a closed-pool error")
	}
	if _, err := store.TelegramSubscriberActive(ctx, 123); err == nil {
		t.Fatal("subscriber lookup must report a closed-pool error")
	}
	if err := store.ActivateTelegramSubscriber(ctx, 123, "admin"); err == nil {
		t.Fatal("subscriber activation must report a closed-pool error")
	}
	if err := store.DeactivateTelegramSubscriber(ctx, 123); err == nil {
		t.Fatal("subscriber deactivation must report a closed-pool error")
	}
	if _, err := store.ActiveTelegramChatIDs(ctx); err == nil {
		t.Fatal("subscriber listing must report a closed-pool error")
	}
	if err := store.RecordTelegramNotification(ctx, 123, 42, time.Now()); err == nil {
		t.Fatal("notification recording must report a closed-pool error")
	}
	if _, err := store.DueTelegramNotifications(ctx, time.Now(), 100); err == nil {
		t.Fatal("notification listing must report a closed-pool error")
	}
	if err := store.MarkTelegramNotificationDeleted(ctx, 1, time.Now()); err == nil {
		t.Fatal("notification marking must report a closed-pool error")
	}
}
