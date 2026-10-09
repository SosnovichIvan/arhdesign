package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
)

const telegramAccountOutboxMaximumAttempts = 10

type TelegramAccountOutboxStore interface {
	RecoverStaleTelegramAccountOutbox(context.Context, time.Time, time.Time) (int64, error)
	TerminalizeInactiveTelegramAccountOutbox(context.Context, time.Time) (int64, error)
	ClaimTelegramAccountOutbox(context.Context, time.Time) (repository.TelegramAccountOutboxEntry, bool, error)
	MarkTelegramAccountOutboxDelivered(context.Context, string, time.Time) error
	MarkTelegramAccountOutboxFailed(context.Context, string, int, int, time.Time, time.Time, string) error
	RecordTelegramNotification(context.Context, int64, int64, time.Time) error
}

type TelegramAccountOutbox struct {
	store     TelegramAccountOutboxStore
	cipher    account.PayloadCipher
	telegram  Telegram
	logger    *slog.Logger
	now       func() time.Time
	timeout   time.Duration
	retention time.Duration
}

func NewTelegramAccountOutbox(store TelegramAccountOutboxStore, cipher account.PayloadCipher, telegram Telegram, logger *slog.Logger, now func() time.Time) (*TelegramAccountOutbox, error) {
	if store == nil || telegram.client == nil || telegram.url == "" {
		return nil, fmt.Errorf("Telegram account outbox requires store and client")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &TelegramAccountOutbox{store: store, cipher: cipher, telegram: telegram, logger: logger, now: now, timeout: 10 * time.Second, retention: 30 * time.Minute}, nil
}

func (worker *TelegramAccountOutbox) ProcessOne(ctx context.Context) (bool, error) {
	now := worker.now().UTC()
	if _, err := worker.store.RecoverStaleTelegramAccountOutbox(ctx, now.Add(-5*time.Minute), now); err != nil {
		return false, err
	}
	if _, err := worker.store.TerminalizeInactiveTelegramAccountOutbox(ctx, now); err != nil {
		return false, err
	}
	entry, found, err := worker.store.ClaimTelegramAccountOutbox(ctx, now)
	if err != nil || !found {
		return found, err
	}
	payload, err := worker.cipher.Decrypt(entry.PayloadCiphertext, entry.MessageType, entry.PayloadKeyVersion)
	if err != nil {
		return true, worker.fail(ctx, entry, now, "payload_invalid")
	}
	var message account.TelegramAuthMessage
	if err := json.Unmarshal(payload, &message); err != nil || strings.TrimSpace(message.Text) == "" {
		return true, worker.fail(ctx, entry, now, "payload_invalid")
	}
	sendContext, cancel := context.WithTimeout(ctx, worker.timeout)
	messageID, err := worker.telegram.send(sendContext, fmt.Sprint(entry.ChatID), message.Text, false, false)
	cancel()
	if err != nil {
		return true, worker.fail(ctx, entry, now, "telegram_unavailable")
	}
	if err := worker.store.RecordTelegramNotification(ctx, entry.ChatID, messageID, worker.now().UTC().Add(worker.retention)); err != nil {
		_ = worker.telegram.DeleteMessage(ctx, entry.ChatID, messageID)
		return true, err
	}
	if err := worker.store.MarkTelegramAccountOutboxDelivered(ctx, entry.ID, worker.now().UTC()); err != nil {
		return true, err
	}
	return true, nil
}

func (worker *TelegramAccountOutbox) fail(ctx context.Context, entry repository.TelegramAccountOutboxEntry, now time.Time, code string) error {
	if err := worker.store.MarkTelegramAccountOutboxFailed(ctx, entry.ID, entry.Attempts, telegramAccountOutboxMaximumAttempts, now, now.Add(telegramRetryDelay(entry.Attempts)), code); err != nil {
		return err
	}
	worker.logger.Warn("Telegram account outbox delivery failed", "error_class", code, "attempt", entry.Attempts)
	return nil
}

func telegramRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 30 * time.Second
	case 2:
		return 2 * time.Minute
	case 3:
		return 10 * time.Minute
	case 4:
		return 30 * time.Minute
	case 5:
		return 2 * time.Hour
	default:
		return 6 * time.Hour
	}
}

func (worker *TelegramAccountOutbox) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	process := func() {
		for {
			processed, err := worker.ProcessOne(ctx)
			if err != nil {
				worker.logger.Warn("Telegram account outbox worker failed", "error_class", fmt.Sprintf("%T", err))
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
