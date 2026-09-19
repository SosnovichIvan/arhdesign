package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
)

type telegramRetentionStoreStub struct {
	due       []repository.TelegramNotification
	markedIDs []int64
	dueErr    error
	markErr   error
}

func (store *telegramRetentionStoreStub) RecordTelegramNotification(context.Context, int64, int64, time.Time) error {
	return nil
}

func (store *telegramRetentionStoreStub) DueTelegramNotifications(context.Context, time.Time, int) ([]repository.TelegramNotification, error) {
	return store.due, store.dueErr
}

func (store *telegramRetentionStoreStub) MarkTelegramNotificationDeleted(_ context.Context, id int64, _ time.Time) error {
	store.markedIDs = append(store.markedIDs, id)
	return store.markErr
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

func TestTelegramRetentionReportsStoreAndDeleteErrors(t *testing.T) {
	store := &telegramRetentionStoreStub{dueErr: errors.New("database unavailable")}
	retention := NewTelegramRetention(Telegram{}, store)
	if deleted, err := retention.DeleteDue(context.Background(), time.Now()); err == nil || deleted != 0 {
		t.Fatalf("deleted = %d, error = %v; want 0 and error", deleted, err)
	}

	store = &telegramRetentionStoreStub{due: []repository.TelegramNotification{{ID: 7, ChatID: 123, MessageID: 42}}}
	retention = NewTelegramRetention(Telegram{client: failingHTTPClient{}, url: "https://example.test/sendMessage"}, store)
	if deleted, err := retention.DeleteDue(context.Background(), time.Now()); err == nil || deleted != 0 || len(store.markedIDs) != 0 {
		t.Fatalf("deleted = %d, marked = %#v, error = %v; want delete error", deleted, store.markedIDs, err)
	}
}

func TestTelegramRetentionReportsMarkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()
	store := &telegramRetentionStoreStub{due: []repository.TelegramNotification{{ID: 7, ChatID: 123, MessageID: 42}}, markErr: errors.New("database unavailable")}
	retention := NewTelegramRetention(Telegram{client: server.Client(), url: server.URL + "/sendMessage"}, store)
	if deleted, err := retention.DeleteDue(context.Background(), time.Now()); err == nil || deleted != 0 || len(store.markedIDs) != 1 {
		t.Fatalf("deleted = %d, marked = %#v, error = %v; want mark error", deleted, store.markedIDs, err)
	}
}
