package notification

import (
	"context"
	"errors"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
)

type TelegramRetention struct {
	client Telegram
	store  repository.TelegramNotificationStore
}

func NewTelegramRetention(client Telegram, store repository.TelegramNotificationStore) TelegramRetention {
	return TelegramRetention{client: client, store: store}
}

func (retention TelegramRetention) DeleteDue(ctx context.Context, now time.Time) (int, error) {
	notifications, err := retention.store.DueTelegramNotifications(ctx, now, 100)
	if err != nil {
		return 0, err
	}

	deleted := 0
	var deletionErrors []error
	for _, item := range notifications {
		if err := retention.client.DeleteMessage(ctx, item.ChatID, item.MessageID); err != nil {
			deletionErrors = append(deletionErrors, err)
			continue
		}
		if err := retention.store.MarkTelegramNotificationDeleted(ctx, item.ID, now); err != nil {
			deletionErrors = append(deletionErrors, err)
			continue
		}
		deleted++
	}
	return deleted, errors.Join(deletionErrors...)
}
