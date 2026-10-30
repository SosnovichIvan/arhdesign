package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
)

type fakeEmailOutboxStore struct {
	entry       repository.EmailOutboxEntry
	found       bool
	claimErr    error
	recoverErr  error
	recovered   int64
	deliverErr  error
	failureErr  error
	deliveredID string
	failedID    string
	failedState struct {
		attempts, maximum int
		retryAt           time.Time
		code              string
	}
}

func (store *fakeEmailOutboxStore) RecoverStaleEmailOutbox(context.Context, time.Time, time.Time) (int64, error) {
	return store.recovered, store.recoverErr
}

func (store *fakeEmailOutboxStore) ClaimEmailOutbox(context.Context, time.Time) (repository.EmailOutboxEntry, bool, error) {
	return store.entry, store.found, store.claimErr
}
func (store *fakeEmailOutboxStore) MarkEmailOutboxDelivered(_ context.Context, id string, _ time.Time) error {
	store.deliveredID = id
	return store.deliverErr
}
func (store *fakeEmailOutboxStore) MarkEmailOutboxFailed(_ context.Context, id string, attempts, maximum int, _ time.Time, retryAt time.Time, code string) error {
	store.failedID = id
	store.failedState.attempts, store.failedState.maximum, store.failedState.retryAt, store.failedState.code = attempts, maximum, retryAt, code
	return store.failureErr
}

type fakeOutboxEmailTransport struct {
	message EmailMessage
	err     error
}

func (transport *fakeOutboxEmailTransport) Send(_ context.Context, message EmailMessage) error {
	transport.message = message
	return transport.err
}

func emailOutboxFixture(t *testing.T, attempts int) (*fakeEmailOutboxStore, account.PayloadCipher, time.Time) {
	t.Helper()
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(account.VerificationEmail{From: "from@example.com", To: "to@example.com", Subject: "Подтвердите почту", Text: "Текст"})
	ciphertext, version, err := cipher.Encrypt(payload, "account.email_verification")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeEmailOutboxStore{found: true, entry: repository.EmailOutboxEntry{ID: "outbox-1", MessageType: "account.email_verification", PayloadCiphertext: ciphertext, PayloadKeyVersion: version, Attempts: attempts}}
	return store, cipher, time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
}

func TestEmailOutboxSuccessAndEmptyQueue(t *testing.T) {
	store, cipher, now := emailOutboxFixture(t, 1)
	transport := &fakeOutboxEmailTransport{}
	worker, err := NewEmailOutbox(store, cipher, transport, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	processed, err := worker.ProcessOne(context.Background())
	if err != nil || !processed || store.deliveredID != "outbox-1" || transport.message.To != "to@example.com" {
		t.Fatalf("processed = %v, delivered = %q, message = %#v, error = %v", processed, store.deliveredID, transport.message, err)
	}
	store.found = false
	processed, err = worker.ProcessOne(context.Background())
	if err != nil || processed {
		t.Fatalf("empty processed = %v, error = %v", processed, err)
	}
}

func TestEmailOutboxRetriesAndTerminates(t *testing.T) {
	for _, attempts := range []int{1, 5} {
		t.Run(time.Duration(attempts).String(), func(t *testing.T) {
			store, cipher, now := emailOutboxFixture(t, attempts)
			transport := &fakeOutboxEmailTransport{err: errors.New("SMTP unavailable")}
			worker, err := NewEmailOutbox(store, cipher, transport, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			processed, err := worker.ProcessOne(context.Background())
			if err != nil || !processed || store.failedID != "outbox-1" || store.failedState.code != "smtp_unavailable" || store.failedState.attempts != attempts || store.failedState.maximum != 5 {
				t.Fatalf("processed=%v failed=%q state=%#v err=%v", processed, store.failedID, store.failedState, err)
			}
			if attempts == 1 && !store.failedState.retryAt.Equal(now.Add(time.Minute)) {
				t.Fatalf("retryAt = %v", store.failedState.retryAt)
			}
		})
	}
}

func TestEmailOutboxRejectsTamperedPayload(t *testing.T) {
	store, cipher, now := emailOutboxFixture(t, 1)
	store.entry.PayloadCiphertext[len(store.entry.PayloadCiphertext)-1] ^= 1
	worker, err := NewEmailOutbox(store, cipher, &fakeOutboxEmailTransport{}, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.ProcessOne(context.Background()); err != nil || !processed || store.failedState.code != "payload_invalid" {
		t.Fatalf("processed=%v code=%q error=%v", processed, store.failedState.code, err)
	}
}

func TestEmailOutboxConstructorAndClaimError(t *testing.T) {
	_, cipher, now := emailOutboxFixture(t, 1)
	if _, err := NewEmailOutbox(nil, cipher, &fakeOutboxEmailTransport{}, nil, nil); err == nil {
		t.Fatal("nil store must fail")
	}
	if _, err := NewEmailOutbox(&fakeEmailOutboxStore{}, cipher, nil, nil, nil); err == nil {
		t.Fatal("nil transport must fail")
	}
	store := &fakeEmailOutboxStore{claimErr: context.DeadlineExceeded}
	worker, err := NewEmailOutbox(store, cipher, &fakeOutboxEmailTransport{}, nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.ProcessOne(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("claim error = %v", err)
	}
}

func TestEmailOutboxStoreMarkErrors(t *testing.T) {
	store, cipher, now := emailOutboxFixture(t, 1)
	store.deliverErr = context.DeadlineExceeded
	worker, err := NewEmailOutbox(store, cipher, &fakeOutboxEmailTransport{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.ProcessOne(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("delivery mark error = %v", err)
	}
	store, cipher, _ = emailOutboxFixture(t, 1)
	store.entry.PayloadCiphertext[len(store.entry.PayloadCiphertext)-1] ^= 1
	store.failureErr = context.DeadlineExceeded
	worker, err = NewEmailOutbox(store, cipher, &fakeOutboxEmailTransport{}, nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.ProcessOne(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("failure mark error = %v", err)
	}
}

func TestEmailOutboxRecoveryError(t *testing.T) {
	_, cipher, _ := emailOutboxFixture(t, 1)
	worker, err := NewEmailOutbox(&fakeEmailOutboxStore{recoverErr: context.DeadlineExceeded}, cipher, &fakeOutboxEmailTransport{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.ProcessOne(context.Background()); processed || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("processed=%v recovery error=%v", processed, err)
	}
}

func TestEmailOutboxRunStopsOnCancellationAndRetrySchedule(t *testing.T) {
	_, cipher, now := emailOutboxFixture(t, 1)
	store := &fakeEmailOutboxStore{}
	worker, err := NewEmailOutbox(store, cipher, &fakeOutboxEmailTransport{}, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		worker.Run(ctx, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	for attempt, expected := range map[int]time.Duration{2: 5 * time.Minute, 3: 15 * time.Minute, 4: time.Hour} {
		if got := emailRetryDelay(attempt); got != expected {
			t.Fatalf("attempt %d delay = %v, want %v", attempt, got, expected)
		}
	}
}
