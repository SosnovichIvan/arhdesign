package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContactSubmission struct {
	Name           string
	Contact        string
	ProjectType    string
	ProjectDetails string
}

type Postgres struct {
	pool *pgxpool.Pool
}

type ContactStore interface {
	CooldownRetryAfter(context.Context, []byte, time.Time) (time.Duration, error)
	CreateSubmissionAndSetCooldown(context.Context, ContactSubmission, []byte, time.Time) error
	DeleteSubmissionsOlderThan(context.Context, time.Time) (int64, error)
}

type TelegramSubscriberStore interface {
	ActivateTelegramSubscriber(context.Context, int64, string) error
	DeactivateTelegramSubscriber(context.Context, int64) error
	ActiveTelegramChatIDs(context.Context) ([]int64, error)
	TelegramSubscriberActive(context.Context, int64) (bool, error)
}

func (repository *Postgres) TelegramSubscriberActive(ctx context.Context, chatID int64) (bool, error) {
	var active bool
	err := repository.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM telegram_subscribers WHERE chat_id = $1 AND unsubscribed_at IS NULL)`, chatID).Scan(&active)
	return active, err
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (repository *Postgres) DeleteSubmissionsOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result, err := repository.pool.Exec(ctx, `DELETE FROM contact_submissions WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (repository *Postgres) CreateSubmissionAndSetCooldown(ctx context.Context, submission ContactSubmission, tokenHash []byte, expiresAt time.Time) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err = transaction.Exec(ctx, `
		INSERT INTO contact_submissions (name, contact, project_type, project_details)
		VALUES ($1, $2, $3, $4)
	`, submission.Name, submission.Contact, submission.ProjectType, submission.ProjectDetails); err != nil {
		return err
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO contact_cooldowns (token_hash, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (token_hash) DO UPDATE SET expires_at = EXCLUDED.expires_at
	`, tokenHash, expiresAt); err != nil {
		return err
	}
	return transaction.Commit(ctx)
}

func (repository *Postgres) CooldownRetryAfter(ctx context.Context, tokenHash []byte, now time.Time) (time.Duration, error) {
	var expiresAt time.Time
	err := repository.pool.QueryRow(ctx, `SELECT expires_at FROM contact_cooldowns WHERE token_hash = $1`, tokenHash).Scan(&expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !expiresAt.After(now) {
		_, deleteErr := repository.pool.Exec(ctx, `DELETE FROM contact_cooldowns WHERE token_hash = $1`, tokenHash)
		return 0, deleteErr
	}
	return expiresAt.Sub(now), nil
}

func (repository *Postgres) ActivateTelegramSubscriber(ctx context.Context, chatID int64, username string) error {
	_, err := repository.pool.Exec(ctx, `INSERT INTO telegram_subscribers (chat_id, username, subscribed_at, unsubscribed_at, updated_at) VALUES ($1, $2, NOW(), NULL, NOW()) ON CONFLICT (chat_id) DO UPDATE SET username = EXCLUDED.username, unsubscribed_at = NULL, updated_at = NOW()`, chatID, username)
	return err
}

func (repository *Postgres) DeactivateTelegramSubscriber(ctx context.Context, chatID int64) error {
	_, err := repository.pool.Exec(ctx, `UPDATE telegram_subscribers SET unsubscribed_at = NOW(), updated_at = NOW() WHERE chat_id = $1`, chatID)
	return err
}

func (repository *Postgres) ActiveTelegramChatIDs(ctx context.Context) ([]int64, error) {
	rows, err := repository.pool.Query(ctx, `SELECT chat_id FROM telegram_subscribers WHERE unsubscribed_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chatIDs := make([]int64, 0)
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, chatID)
	}
	return chatIDs, rows.Err()
}
