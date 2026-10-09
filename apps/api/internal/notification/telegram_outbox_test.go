package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
)

type fakeTelegramAccountOutboxStore struct {
	entry       repository.TelegramAccountOutboxEntry
	found       bool
	claimErr    error
	deliveredID string
	failedID    string
	failureCode string
	receiptChat int64
	receiptMsg  int64
	recoverErr  error
	receiptErr  error
	deliverErr  error
	failureErr  error
}

func (store *fakeTelegramAccountOutboxStore) RecoverStaleTelegramAccountOutbox(context.Context, time.Time, time.Time) (int64, error) {
	return 0, store.recoverErr
}
func (store *fakeTelegramAccountOutboxStore) TerminalizeInactiveTelegramAccountOutbox(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (store *fakeTelegramAccountOutboxStore) ClaimTelegramAccountOutbox(context.Context, time.Time) (repository.TelegramAccountOutboxEntry, bool, error) {
	return store.entry, store.found, store.claimErr
}
func (store *fakeTelegramAccountOutboxStore) MarkTelegramAccountOutboxDelivered(_ context.Context, id string, _ time.Time) error {
	store.deliveredID = id
	return store.deliverErr
}
func (store *fakeTelegramAccountOutboxStore) MarkTelegramAccountOutboxFailed(_ context.Context, id string, _, _ int, _, _ time.Time, code string) error {
	store.failedID, store.failureCode = id, code
	return store.failureErr
}
func (store *fakeTelegramAccountOutboxStore) RecordTelegramNotification(_ context.Context, chatID, messageID int64, _ time.Time) error {
	store.receiptChat, store.receiptMsg = chatID, messageID
	return store.receiptErr
}

type telegramOutboxHTTPClient struct {
	request *http.Request
	err     error
}

func (client *telegramOutboxHTTPClient) Do(request *http.Request) (*http.Response, error) {
	client.request = request
	if client.err != nil {
		return nil, client.err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":55}}`))}, nil
}

func TestTelegramRetryScheduleExtendsThroughTwentyFourHours(t *testing.T) {
	want := []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}
	for index, expected := range want {
		if actual := telegramRetryDelay(index + 1); actual != expected {
			t.Fatalf("attempt %d delay=%s want=%s", index+1, actual, expected)
		}
	}
	if telegramAccountOutboxMaximumAttempts < 10 {
		t.Fatal("retry budget must cover at least 24 hours")
	}
}

func telegramAccountOutboxFixture(t *testing.T) (*fakeTelegramAccountOutboxStore, account.PayloadCipher, time.Time) {
	t.Helper()
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{7}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(account.TelegramAuthMessage{Text: "Ссылка восстановления"})
	ciphertext, version, err := cipher.Encrypt(payload, "account.password_reset")
	if err != nil {
		t.Fatal(err)
	}
	entry := repository.TelegramAccountOutboxEntry{ID: "telegram-outbox-1", ChatID: 701, MessageType: "account.password_reset", PayloadCiphertext: ciphertext, PayloadKeyVersion: version, Attempts: 1}
	return &fakeTelegramAccountOutboxStore{entry: entry, found: true}, cipher, time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
}

func TestTelegramAccountOutboxDeliversAndRecordsRetention(t *testing.T) {
	store, cipher, now := telegramAccountOutboxFixture(t)
	client := &telegramOutboxHTTPClient{}
	worker, err := NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", client), slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	processed, err := worker.ProcessOne(context.Background())
	if err != nil || !processed || store.deliveredID != "telegram-outbox-1" || store.receiptChat != 701 || store.receiptMsg != 55 {
		t.Fatalf("processed=%v delivered=%q receipt=%d/%d error=%v", processed, store.deliveredID, store.receiptChat, store.receiptMsg, err)
	}
	if client.request == nil || !strings.HasSuffix(client.request.URL.Path, "/sendMessage") {
		t.Fatalf("Telegram request=%v", client.request)
	}
}

func TestTelegramAccountOutboxDeliversProjectEventPayload(t *testing.T) {
	for _, testCase := range []struct {
		messageType, text, fragment string
	}{
		{"project.context_chat.message_created", "Иван Петров написал в обсуждении задачи: Проверьте чертежи", "Проверьте чертежи"},
		{"project.expense.created", "Новый расход по проекту: Керамогранит, 1250.50", "Керамогранит"},
		{"project.material.updated", "Материал проекта изменён: Керамогранит серый", "Керамогранит серый"},
	} {
		t.Run(testCase.messageType, func(t *testing.T) {
			store, cipher, now := telegramAccountOutboxFixture(t)
			store.entry.MessageType = testCase.messageType
			payload, err := json.Marshal(account.TelegramAuthMessage{Text: testCase.text})
			if err != nil {
				t.Fatal(err)
			}
			store.entry.PayloadCiphertext, store.entry.PayloadKeyVersion, err = cipher.Encrypt(payload, store.entry.MessageType)
			if err != nil {
				t.Fatal(err)
			}
			client := &telegramOutboxHTTPClient{}
			worker, err := NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", client), nil, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			processed, err := worker.ProcessOne(context.Background())
			if err != nil || !processed || store.deliveredID != store.entry.ID {
				t.Fatalf("processed=%v delivered=%q error=%v", processed, store.deliveredID, err)
			}
			if client.request == nil {
				t.Fatal("Telegram request was not sent")
			}
			body, err := io.ReadAll(client.request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), testCase.fragment) {
				t.Fatalf("Telegram request body=%s", body)
			}
		})
	}
}

func TestTelegramAccountOutboxRetriesInvalidPayloadAndTransportFailure(t *testing.T) {
	store, cipher, now := telegramAccountOutboxFixture(t)
	store.entry.PayloadCiphertext[len(store.entry.PayloadCiphertext)-1] ^= 1
	worker, err := NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.ProcessOne(context.Background()); err != nil || !processed || store.failureCode != "payload_invalid" {
		t.Fatalf("processed=%v code=%q error=%v", processed, store.failureCode, err)
	}

	store, cipher, now = telegramAccountOutboxFixture(t)
	worker, err = NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{err: errors.New("network unavailable")}), nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.ProcessOne(context.Background()); err != nil || !processed || store.failureCode != "telegram_unavailable" {
		t.Fatalf("processed=%v code=%q error=%v", processed, store.failureCode, err)
	}
}

func TestTelegramAccountOutboxEmptyAndFailureBoundaries(t *testing.T) {
	store, cipher, now := telegramAccountOutboxFixture(t)
	store.found = false
	worker, err := NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if processed, processErr := worker.ProcessOne(context.Background()); processErr != nil || processed {
		t.Fatalf("empty processed=%v error=%v", processed, processErr)
	}

	store.recoverErr = errors.New("recover failed")
	if _, processErr := worker.ProcessOne(context.Background()); processErr == nil {
		t.Fatal("recover error must be returned")
	}
	store.recoverErr = nil
	store.claimErr = errors.New("claim failed")
	if _, processErr := worker.ProcessOne(context.Background()); processErr == nil {
		t.Fatal("claim error must be returned")
	}

	store, cipher, now = telegramAccountOutboxFixture(t)
	payload, _ := json.Marshal(map[string]string{"text": ""})
	store.entry.PayloadCiphertext, store.entry.PayloadKeyVersion, _ = cipher.Encrypt(payload, store.entry.MessageType)
	worker, _ = NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, func() time.Time { return now })
	if processed, processErr := worker.ProcessOne(context.Background()); processErr != nil || !processed || store.failureCode != "payload_invalid" {
		t.Fatalf("invalid body processed=%v code=%s error=%v", processed, store.failureCode, processErr)
	}

	store, cipher, now = telegramAccountOutboxFixture(t)
	store.receiptErr = errors.New("receipt failed")
	worker, _ = NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, func() time.Time { return now })
	if processed, processErr := worker.ProcessOne(context.Background()); processErr == nil || !processed {
		t.Fatalf("receipt failure processed=%v error=%v", processed, processErr)
	}

	store, cipher, now = telegramAccountOutboxFixture(t)
	store.deliverErr = errors.New("delivery state failed")
	worker, _ = NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, func() time.Time { return now })
	if processed, processErr := worker.ProcessOne(context.Background()); processErr == nil || !processed {
		t.Fatalf("delivery state failure processed=%v error=%v", processed, processErr)
	}

	store, cipher, now = telegramAccountOutboxFixture(t)
	store.entry.PayloadCiphertext[len(store.entry.PayloadCiphertext)-1] ^= 1
	store.failureErr = errors.New("failure state failed")
	worker, _ = NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, func() time.Time { return now })
	if _, processErr := worker.ProcessOne(context.Background()); processErr == nil {
		t.Fatal("outbox failure persistence error must be returned")
	}
}

func TestTelegramAccountOutboxConstructorAndRun(t *testing.T) {
	store, cipher, now := telegramAccountOutboxFixture(t)
	if _, err := NewTelegramAccountOutbox(nil, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, nil); err == nil {
		t.Fatal("nil store must fail")
	}
	if _, err := NewTelegramAccountOutbox(store, cipher, Telegram{}, nil, nil); err == nil {
		t.Fatal("empty Telegram client must fail")
	}
	store.found = false
	worker, err := NewTelegramAccountOutbox(store, cipher, NewTelegram("token", "", &telegramOutboxHTTPClient{}), nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker.Run(ctx, 0)
}
