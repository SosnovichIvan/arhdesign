package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
)

const emailOutboxMaximumAttempts = 5

type EmailOutboxStore interface {
	RecoverStaleEmailOutbox(context.Context, time.Time, time.Time) (int64, error)
	ClaimEmailOutbox(context.Context, time.Time) (repository.EmailOutboxEntry, bool, error)
	MarkEmailOutboxDelivered(context.Context, string, time.Time) error
	MarkEmailOutboxFailed(context.Context, string, int, int, time.Time, time.Time, string) error
}

type EmailOutbox struct {
	store     EmailOutboxStore
	cipher    account.PayloadCipher
	transport EmailTransport
	logger    *slog.Logger
	now       func() time.Time
	timeout   time.Duration
}

func NewEmailOutbox(store EmailOutboxStore, cipher account.PayloadCipher, transport EmailTransport, logger *slog.Logger, now func() time.Time) (*EmailOutbox, error) {
	if store == nil || transport == nil {
		return nil, fmt.Errorf("email outbox requires store and transport")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &EmailOutbox{store: store, cipher: cipher, transport: transport, logger: logger, now: now, timeout: 10 * time.Second}, nil
}

func (worker *EmailOutbox) ProcessOne(ctx context.Context) (bool, error) {
	now := worker.now().UTC()
	if _, err := worker.store.RecoverStaleEmailOutbox(ctx, now.Add(-5*time.Minute), now); err != nil {
		return false, err
	}
	entry, found, err := worker.store.ClaimEmailOutbox(ctx, now)
	if err != nil || !found {
		return found, err
	}
	payload, err := worker.cipher.Decrypt(entry.PayloadCiphertext, entry.MessageType, entry.PayloadKeyVersion)
	if err != nil {
		return true, worker.fail(ctx, entry, now, "payload_invalid", err)
	}
	var email account.VerificationEmail
	if err := json.Unmarshal(payload, &email); err != nil || email.From == "" || email.To == "" || email.Subject == "" || email.Text == "" {
		return true, worker.fail(ctx, entry, now, "payload_invalid", fmt.Errorf("invalid email payload"))
	}
	sendContext, cancel := context.WithTimeout(ctx, worker.timeout)
	err = worker.transport.Send(sendContext, EmailMessage{From: email.From, To: email.To, Subject: email.Subject, Text: email.Text})
	cancel()
	if err != nil {
		return true, worker.fail(ctx, entry, now, "smtp_unavailable", err)
	}
	if err := worker.store.MarkEmailOutboxDelivered(ctx, entry.ID, worker.now().UTC()); err != nil {
		return true, err
	}
	return true, nil
}

func (worker *EmailOutbox) fail(ctx context.Context, entry repository.EmailOutboxEntry, now time.Time, code string, cause error) error {
	retryAt := now.Add(emailRetryDelay(entry.Attempts))
	if err := worker.store.MarkEmailOutboxFailed(ctx, entry.ID, entry.Attempts, emailOutboxMaximumAttempts, now, retryAt, code); err != nil {
		return err
	}
	worker.logger.Warn("email outbox delivery failed", "error_class", code, "attempt", entry.Attempts)
	return nil
}

func (worker *EmailOutbox) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	process := func() {
		for {
			processed, err := worker.ProcessOne(ctx)
			if err != nil {
				worker.logger.Warn("email outbox worker failed", "error_class", fmt.Sprintf("%T", err))
				return
			}
			if !processed {
				return
			}
		}
	}
	process()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}

func emailRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 15 * time.Minute
	default:
		return time.Hour
	}
}
