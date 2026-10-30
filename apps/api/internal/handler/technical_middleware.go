package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	if recorder.status != 0 {
		return
	}
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Write(payload []byte) (int, error) {
	if recorder.status == 0 {
		recorder.WriteHeader(http.StatusOK)
	}
	return recorder.ResponseWriter.Write(payload)
}

func TechnicalIncidentMiddleware(next http.Handler, service *technicalsupport.Service, logger *slog.Logger, now func() time.Time) http.Handler {
	if service == nil {
		return next
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				if recorder.status == 0 {
					writeAccountError(recorder, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервиса", middlewareRequestID(recorder, r))
				}
				logger.Error("HTTP panic recovered", "error_class", "panic", "request_id", middlewareRequestID(recorder, r), "stack_size", len(debug.Stack()))
				reportTechnicalHTTPFailure(service, logger, recorder, r, "panic", now())
				return
			}
			if recorder.status >= 500 {
				reportTechnicalHTTPFailure(service, logger, recorder, r, "http_5xx", now())
			}
		}()
		next.ServeHTTP(recorder, r)
	})
}

func reportTechnicalHTTPFailure(service *technicalsupport.Service, logger *slog.Logger, recorder *statusRecorder, r *http.Request, code string, occurredAt time.Time) {
	status := recorder.status
	if status == 0 {
		status = http.StatusInternalServerError
	}
	route := strings.TrimSpace(r.Pattern)
	if fields := strings.Fields(route); len(fields) == 2 && strings.HasPrefix(fields[1], "/") {
		route = fields[1]
	}
	if route == "" {
		route = r.URL.Path
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := service.ReportBackend(ctx, technicalsupport.BackendError{Method: r.Method, Route: route, RequestID: middlewareRequestID(recorder, r), StatusCode: status, ErrorCode: code, OccurredAt: occurredAt}); err != nil {
		logger.Warn("technical incident could not be queued", "error_class", fmt.Sprintf("%T", err), "request_id", middlewareRequestID(recorder, r))
	}
}

func middlewareRequestID(recorder *statusRecorder, r *http.Request) string {
	if value := strings.TrimSpace(recorder.Header().Get("X-Request-ID")); value != "" {
		return value
	}
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	return "middleware-error"
}
