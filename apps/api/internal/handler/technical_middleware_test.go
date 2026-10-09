package handler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
)

type middlewareTechnicalStore struct {
	reports []technicalsupport.Report
	err     error
}

func (store *middlewareTechnicalStore) ConsumeRateLimit(context.Context, account.RateLimitRequest) (time.Duration, error) {
	return 0, nil
}

func (store *middlewareTechnicalStore) CreateTechnicalReport(_ context.Context, report technicalsupport.Report) (bool, error) {
	if store.err != nil {
		return false, store.err
	}
	store.reports = append(store.reports, report)
	return true, nil
}

func technicalMiddlewareService(t *testing.T, store *middlewareTechnicalStore) *technicalsupport.Service {
	t.Helper()
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{91}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := technicalsupport.NewService(store, cipher, bytes.Repeat([]byte{92}, 32), 1, func() time.Time { return time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestTechnicalIncidentMiddlewareReportsOnlyServerFailures(t *testing.T) {
	store := &middlewareTechnicalStore{}
	service := technicalMiddlewareService(t, store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC) }

	clientFailure := TechnicalIncidentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}), service, logger, now)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chats?token=secret", nil)
	clientFailure.ServeHTTP(httptest.NewRecorder(), request)
	if len(store.reports) != 0 {
		t.Fatalf("expected 4xx not to page technical administrator, reports=%d", len(store.reports))
	}

	serverFailure := TechnicalIncidentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "request-server-failure")
		http.Error(w, "database URL must never be reported", http.StatusServiceUnavailable)
	}), service, logger, now)
	request = httptest.NewRequest(http.MethodPost, "/api/v1/chats?token=secret", nil)
	request.Pattern = "POST /api/v1/chats"
	response := httptest.NewRecorder()
	serverFailure.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || len(store.reports) != 1 {
		t.Fatalf("status=%d reports=%d", response.Code, len(store.reports))
	}
	report := store.reports[0]
	if report.Route != "/api/v1/chats" || report.Method != http.MethodPost || report.StatusCode != http.StatusServiceUnavailable || report.RequestID != "request-server-failure" {
		t.Fatalf("report=%#v", report)
	}
	if bytes.Contains(report.PayloadCiphertext, []byte("database URL")) || bytes.Contains(report.PayloadCiphertext, []byte("secret")) {
		t.Fatal("response body or query leaked into the encrypted incident envelope")
	}
}

func TestTechnicalIncidentMiddlewareRecoversPanic(t *testing.T) {
	store := &middlewareTechnicalStore{}
	service := technicalMiddlewareService(t, store)
	handler := TechnicalIncidentMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("credential value") }), service, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))
	if response.Code != http.StatusInternalServerError || len(store.reports) != 1 || store.reports[0].ErrorCode != "panic" {
		t.Fatalf("status=%d reports=%#v", response.Code, store.reports)
	}
	if bytes.Contains(store.reports[0].PayloadCiphertext, []byte("credential value")) {
		t.Fatal("panic value leaked into incident payload")
	}
}

func TestTechnicalIncidentMiddlewareFallbacksAndRecorderSemantics(t *testing.T) {
	plain := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusNoContent)
		_, _ = w.Write([]byte("created"))
	})
	response := httptest.NewRecorder()
	TechnicalIncidentMiddleware(plain, nil, nil, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plain", nil))
	if response.Code != http.StatusCreated || response.Body.String() != "created" {
		t.Fatalf("plain middleware status=%d body=%q", response.Code, response.Body.String())
	}

	store := &middlewareTechnicalStore{err: context.DeadlineExceeded}
	service := technicalMiddlewareService(t, store)
	failure := TechnicalIncidentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial"))
		panic("after write")
	}), service, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/fallback-route?secret=value", nil)
	request.Header.Set("X-Request-ID", "request-from-header")
	response = httptest.NewRecorder()
	failure.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "partial" {
		t.Fatalf("written panic status=%d body=%q", response.Code, response.Body.String())
	}
	if middlewareRequestID(&statusRecorder{ResponseWriter: httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/", nil)) != "middleware-error" {
		t.Fatal("middleware request id fallback changed")
	}
}
