//go:build integration

package repository

import (
	"context"
	"os"
	"path/filepath"
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

	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_create_contact_submissions.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context, "TRUNCATE contact_submissions, contact_cooldowns"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context, "TRUNCATE contact_submissions, contact_cooldowns")
	})

	repository := NewPostgres(pool)
	now := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	tokenHash := []byte("test-token-hash")
	if err := repository.CreateSubmissionAndSetCooldown(context, ContactSubmission{
		Name:           "Анна Иванова",
		Contact:        "anna@example.com",
		ProjectType:    "Квартира",
		ProjectDetails: "Нужен проект квартиры",
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
	if retryAfter, err := repository.CooldownRetryAfter(context, tokenHash, now); err != nil || retryAfter != time.Hour {
		t.Fatalf("retry after = %v, error = %v; want 1h and nil", retryAfter, err)
	}
	if retryAfter, err := repository.CooldownRetryAfter(context, tokenHash, now.Add(time.Hour)); err != nil || retryAfter != 0 {
		t.Fatalf("expired cooldown retry after = %v, error = %v; want 0 and nil", retryAfter, err)
	}
	if _, err := pool.Exec(context, "UPDATE contact_submissions SET created_at = $1", now.AddDate(0, 0, -366)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := repository.DeleteSubmissionsOlderThan(context, now.AddDate(0, 0, -365)); err != nil || deleted != 1 {
		t.Fatalf("deleted = %d, error = %v; want 1 and nil", deleted, err)
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
	if err := store.CreateSubmissionAndSetCooldown(ctx, ContactSubmission{}, []byte("token"), time.Now()); err == nil {
		t.Fatal("create must report a closed-pool error")
	}
	if _, err := store.CooldownRetryAfter(ctx, []byte("token"), time.Now()); err == nil {
		t.Fatal("cooldown lookup must report a closed-pool error")
	}
}
