package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
)

type telegramRetentionStoreStub struct {
	due       []repository.TelegramNotification
	markedIDs []int64
}

func (store *telegramRetentionStoreStub) RecordTelegramNotification(context.Context, int64, int64, time.Time) error {
	return nil
}

func (store *telegramRetentionStoreStub) DueTelegramNotifications(context.Context, time.Time, int) ([]repository.TelegramNotification, error) {
	return store.due, nil
}

func (store *telegramRetentionStoreStub) MarkTelegramNotificationDeleted(_ context.Context, id int64, _ time.Time) error {
	store.markedIDs = append(store.markedIDs, id)
	return nil
}

func TestTelegramRetentionDeletesAndMarksDueMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()

	store := &telegramRetentionStoreStub{due: []repository.TelegramNotification{{ID: 7, ChatID: 123, MessageID: 42}}}
	retention := NewTelegramRetention(Telegram{client: server.Client(), url: server.URL + "/sendMessage"}, store)
	deleted, err := retention.DeleteDue(context.Background(), time.Now())
	if err != nil || deleted != 1 || len(store.markedIDs) != 1 || store.markedIDs[0] != 7 {
		t.Fatalf("deleted = %d, marked = %#v, err = %v", deleted, store.markedIDs, err)
	}
}
