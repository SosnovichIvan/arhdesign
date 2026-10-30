package technicalsupport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
)

type fakeStore struct {
	retry  time.Duration
	err    error
	report Report
	limit  account.RateLimitRequest
}

func (store *fakeStore) ConsumeRateLimit(_ context.Context, request account.RateLimitRequest) (time.Duration, error) {
	store.limit = request
	return store.retry, store.err
}
func (store *fakeStore) CreateTechnicalReport(_ context.Context, report Report) (bool, error) {
	store.report = report
	return true, store.err
}

type fakeCipher struct {
	plaintext   []byte
	messageType string
	err         error
}

func (cipher *fakeCipher) Encrypt(payload []byte, messageType string) ([]byte, int, error) {
	cipher.plaintext, cipher.messageType = append([]byte(nil), payload...), messageType
	if cipher.err != nil {
		return nil, 0, cipher.err
	}
	return []byte("encrypted"), 4, nil
}

func newService(t *testing.T, store *fakeStore) (*Service, *fakeCipher) {
	t.Helper()
	cipher := &fakeCipher{}
	service, err := NewService(store, cipher, []byte("technical-support-test-hmac-key-32--"), 3, func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	return service, cipher
}
func actor() Actor {
	return Actor{UserID: "00000000-0000-0000-0000-000000000001", Login: "anna", FirstName: "Анна"}
}

func TestFrontendReportRedactsSensitiveText(t *testing.T) {
	store := &fakeStore{}
	service, cipher := newService(t, store)
	err := service.ReportFrontend(context.Background(), actor(), FrontendErrorRequest{Message: "failed https://example.com?a=secret for test@example.com bearer verysecrettokenvalue1234567890", Path: "/account?token=secret", Fingerprint: "abcdefghijklmnop", Digest: "digest-1", RequestID: "request-frontend"})
	if err != nil {
		t.Fatal(err)
	}
	if store.report.Route != "/account" || store.report.Kind != "frontend_error" || store.report.PayloadKeyVersion != 4 {
		t.Fatalf("report=%#v", store.report)
	}
	var payload map[string]string
	if err = json.Unmarshal(cipher.plaintext, &payload); err != nil {
		t.Fatal(err)
	}
	text := payload["text"]
	for _, forbidden := range []string{"example.com", "test@example.com", "verysecrettoken"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("plaintext contains %q: %q", forbidden, text)
		}
	}
}

func TestFeedbackAndRateLimit(t *testing.T) {
	store := &fakeStore{}
	service, cipher := newService(t, store)
	if err := service.CreateFeedback(context.Background(), actor(), FeedbackRequest{Category: "suggestion", Message: "Добавьте экспорт календаря", Path: "/account/calendar", RequestID: "request-feedback"}); err != nil {
		t.Fatal(err)
	}
	if store.report.Kind != "feedback" || store.report.UserMessage == "" || cipher.messageType != "technical.feedback" {
		t.Fatalf("report=%#v type=%q", store.report, cipher.messageType)
	}
	store.retry = time.Minute
	if err := service.CreateFeedback(context.Background(), actor(), FeedbackRequest{Category: "complaint", Message: "Не открывается календарь", Path: "/account/calendar", RequestID: "request-feedback-2"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v", err)
	}
	store.retry = 0
	for _, request := range []FeedbackRequest{{Category: "other", Message: "достаточно текста", Path: "/account", RequestID: "request-invalid"}, {Category: "complaint", Message: "x", Path: "/account", RequestID: "request-invalid"}, {Category: "complaint", Message: "достаточно текста", Path: "https://external", RequestID: "request-invalid"}} {
		if err := service.CreateFeedback(context.Background(), actor(), request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("request=%#v err=%v", request, err)
		}
	}
}

func TestBackendReportAcceptsOnlyServerErrors(t *testing.T) {
	store := &fakeStore{}
	service, cipher := newService(t, store)
	if err := service.ReportBackend(context.Background(), BackendError{Method: "get", Route: "/api/v1/projects/123?secret=yes", RequestID: "request-backend", StatusCode: 503}); err != nil {
		t.Fatal(err)
	}
	if store.report.Method != "GET" || store.report.Route != "/api/v1/projects/123" || store.report.Kind != "backend_error" || cipher.messageType != "technical.backend_error" {
		t.Fatalf("report=%#v", store.report)
	}
	if err := service.ReportBackend(context.Background(), BackendError{Method: "GET", Route: "/api", RequestID: "request-client", StatusCode: 404}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestNewServiceAndInvalidFrontend(t *testing.T) {
	if _, err := NewService(nil, nil, nil, 0, nil); err == nil {
		t.Fatal("missing dependencies accepted")
	}
	service, _ := newService(t, &fakeStore{})
	if err := service.ReportFrontend(context.Background(), actor(), FrontendErrorRequest{Message: "error", Path: "external", Fingerprint: "short", RequestID: "request-invalid"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
	defaultClock, err := NewService(&fakeStore{}, &fakeCipher{}, []byte("technical-support-test-hmac-key-32--"), 1, nil)
	if err != nil || defaultClock.now == nil {
		t.Fatalf("default clock err=%v", err)
	}
}

func TestDependencyFailuresAndBackendDefaults(t *testing.T) {
	store := &fakeStore{}
	service, cipher := newService(t, store)
	store.err = errors.New("rate limit unavailable")
	if err := service.ReportFrontend(context.Background(), actor(), FrontendErrorRequest{Message: "runtime error", Path: "/account", Fingerprint: "abcdefghijklmnop", RequestID: "request-frontend"}); !errors.Is(err, store.err) {
		t.Fatalf("limit err=%v", err)
	}
	store.err = nil
	cipher.err = errors.New("cipher unavailable")
	if err := service.CreateFeedback(context.Background(), actor(), FeedbackRequest{Category: "complaint", Message: "Не открывается список", Path: "/account", RequestID: "request-feedback"}); err == nil || err.Error() != "technical report payload encryption failed" {
		t.Fatalf("cipher err=%v", err)
	}
	cipher.err = nil
	store.err = errors.New("database unavailable")
	if err := service.ReportBackend(context.Background(), BackendError{Route: "invalid", Method: "", RequestID: "request-backend", StatusCode: 500}); !errors.Is(err, store.err) {
		t.Fatalf("store err=%v", err)
	}
	if store.report.Route != "/unknown" || store.report.Method != "UNKNOWN" || store.report.ErrorCode != "http_5xx" || store.report.Now.IsZero() {
		t.Fatalf("report=%#v", store.report)
	}
}

func TestSanitizersAndFingerprintBounds(t *testing.T) {
	if got := sanitizePath(" /account?q=secret#part "); got != "/account" {
		t.Fatalf("path=%q", got)
	}
	if got := sanitizePath("/" + strings.Repeat("a", 301)); got != "" {
		t.Fatalf("long path=%q", got)
	}
	message := sanitizeErrorMessage(strings.Repeat("д", 350))
	if len([]rune(message)) != 300 {
		t.Fatalf("message runes=%d", len([]rune(message)))
	}
	for _, value := range []string{"short", strings.Repeat("a", 129), "abcdefghijklmn!p"} {
		if safeFingerprint(value) {
			t.Fatalf("fingerprint %q accepted", value)
		}
	}
	if shortFingerprint("short") != "short" {
		t.Fatal("short fingerprint changed")
	}
}
