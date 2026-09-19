package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContactSubmission struct {
	Name                  string
	Contact               string
	ProjectType           string
	ProjectDetails        string
	ConsentGranted        bool
	ConsentMethod         string
	ConsentSourceURL      string
	ConsentText           string
	ConsentVersion        string
	ConsentDocumentPath   string
	ConsentDocumentSHA256 string
}

const (
	ConsentVersion        = "2026-09-19-v4"
	ConsentDocumentPath   = "/documents/personal-data-consent-2026-09-19-v4.pdf"
	ConsentDocumentSHA256 = "b8bd07e0d4402acfa6b65da227008948adf7fc8dd9da00a7065fed5eaeb83f89"
	ConsentMethod         = "checkbox-and-submit"
	ConsentText           = "Я даю ИП Полисмаковой Светлане Александровне согласие на обработку персональных данных для рассмотрения обращения и связи со мной."
)

type TelegramNotification struct {
	ID        int64
	ChatID    int64
	MessageID int64
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

type TelegramNotificationStore interface {
	RecordTelegramNotification(context.Context, int64, int64, time.Time) error
	DueTelegramNotifications(context.Context, time.Time, int) ([]TelegramNotification, error)
	MarkTelegramNotificationDeleted(context.Context, int64, time.Time) error
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

func (repository *Postgres) DeleteExpiredCooldowns(ctx context.Context, before time.Time) (int64, error) {
	result, err := repository.pool.Exec(ctx, `DELETE FROM contact_cooldowns WHERE expires_at <= $1`, before)
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

	consentVersion := submission.ConsentVersion
	if consentVersion == "" {
		consentVersion = ConsentVersion
	}
	consentDocumentPath := submission.ConsentDocumentPath
	if consentDocumentPath == "" {
		consentDocumentPath = ConsentDocumentPath
	}
	consentDocumentSHA256 := submission.ConsentDocumentSHA256
	if consentDocumentSHA256 == "" {
		consentDocumentSHA256 = ConsentDocumentSHA256
	}
	consentMethod := submission.ConsentMethod
	if consentMethod == "" {
		consentMethod = ConsentMethod
	}
	consentText := submission.ConsentText
	if consentText == "" {
		consentText = ConsentText
	}

	var submissionID string
	if err = transaction.QueryRow(ctx, `
		INSERT INTO contact_submissions (name, contact, project_type, project_details)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text
	`, submission.Name, submission.Contact, submission.ProjectType, submission.ProjectDetails).Scan(&submissionID); err != nil {
		return err
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO contact_consents (
			submission_id, consent_version, document_path, document_sha256,
			consent_granted, consent_method, consent_text, source_url
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, submissionID, consentVersion, consentDocumentPath, consentDocumentSHA256, submission.ConsentGranted, consentMethod, consentText, submission.ConsentSourceURL); err != nil {
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

func (repository *Postgres) RecordTelegramNotification(ctx context.Context, chatID, messageID int64, deleteAfter time.Time) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO telegram_notification_receipts (chat_id, message_id, delete_after)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, message_id) DO UPDATE SET delete_after = EXCLUDED.delete_after
	`, chatID, messageID, deleteAfter)
	return err
}

func (repository *Postgres) DueTelegramNotifications(ctx context.Context, before time.Time, limit int) ([]TelegramNotification, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id, chat_id, message_id
		FROM telegram_notification_receipts
		WHERE deleted_at IS NULL AND delete_after <= $1
		ORDER BY delete_after
		LIMIT $2
	`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := make([]TelegramNotification, 0)
	for rows.Next() {
		var notification TelegramNotification
		if err := rows.Scan(&notification.ID, &notification.ChatID, &notification.MessageID); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (repository *Postgres) MarkTelegramNotificationDeleted(ctx context.Context, id int64, deletedAt time.Time) error {
	_, err := repository.pool.Exec(ctx, `UPDATE telegram_notification_receipts SET deleted_at = $2 WHERE id = $1`, id, deletedAt)
	return err
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
