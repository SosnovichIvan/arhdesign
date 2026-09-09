package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	generated "github.com/SosnovichIvan/arhdesign/apps/api/internal/api/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/notification"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/service"
)

type ContactRequest = generated.CreateContactSubmissionRequest

type ContactEndpoint struct {
	submit http.HandlerFunc
}

func NewContactEndpoint(cooldown *service.Cooldown, store repository.ContactStore, hmacSecret []byte, limiter *service.RateLimiter, notifier interface {
	Notify(context.Context, notification.Submission)
}) ContactEndpoint {
	return ContactEndpoint{submit: newContactWithStoreAndRateLimit(cooldown, store, hmacSecret, limiter, notifier)}
}

func (endpoint ContactEndpoint) CreateContactSubmission(w http.ResponseWriter, r *http.Request) {
	endpoint.submit(w, r)
}

func NewContact(cooldown *service.Cooldown) http.HandlerFunc {
	return newContactWithStoreAndRateLimit(cooldown, nil, nil, service.NewRateLimiter(time.Now, time.Minute), notification.NoopDispatcher{})
}

func NewContactWithStore(cooldown *service.Cooldown, store repository.ContactStore, hmacSecret []byte) http.HandlerFunc {
	return newContactWithStoreAndRateLimit(cooldown, store, hmacSecret, service.NewRateLimiter(time.Now, time.Minute), notification.NoopDispatcher{})
}

func NewContactWithStoreAndRateLimit(cooldown *service.Cooldown, store repository.ContactStore, hmacSecret []byte, limiter *service.RateLimiter) http.HandlerFunc {
	return newContactWithStoreAndRateLimit(cooldown, store, hmacSecret, limiter, notification.NoopDispatcher{})
}

func newContactWithStoreAndRateLimit(cooldown *service.Cooldown, store repository.ContactStore, hmacSecret []byte, limiter *service.RateLimiter, notifier interface {
	Notify(context.Context, notification.Submission)
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cooldownKey, err := cooldownKey(r)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "Не удалось обработать форму")
			return
		}

		if retryAfter := cooldown.RetryAfter(cooldownKey); retryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			writeJSONError(w, http.StatusTooManyRequests, "contact_cooldown", "Повторная отправка будет доступна позже")
			return
		}
		if store != nil {
			retryAfter, storeErr := store.CooldownRetryAfter(r.Context(), service.CooldownTokenHash(hmacSecret, cooldownKey), time.Now())
			if storeErr != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "Не удалось обработать форму")
				return
			}
			if retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
				writeJSONError(w, http.StatusTooManyRequests, "contact_cooldown", "Повторная отправка будет доступна позже")
				return
			}
		}

		var request ContactRequest
		if json.NewDecoder(r.Body).Decode(&request) != nil || !isValidRequest(request) {
			writeJSONError(w, http.StatusBadRequest, "validation_error", "Проверьте поля формы")
			return
		}

		ip, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil {
			ip = r.RemoteAddr
		}
		if retryAfter := limiter.RetryAfter(ip); retryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			writeJSONError(w, http.StatusTooManyRequests, "rate_limited", "Повторите попытку позже")
			return
		}

		projectDetails := optionalString(request.ProjectDetails)
		if store != nil {
			if err := store.CreateSubmissionAndSetCooldown(r.Context(), repository.ContactSubmission{Name: request.Name, Contact: request.Contact, ProjectType: request.ProjectType, ProjectDetails: projectDetails}, service.CooldownTokenHash(hmacSecret, cooldownKey), time.Now().Add(time.Hour)); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "Не удалось обработать форму")
				return
			}
		}
		cooldown.Set(cooldownKey)
		notifier.Notify(r.Context(), notification.Submission{Name: request.Name, Contact: request.Contact, ProjectType: request.ProjectType, ProjectDetails: projectDetails})
		http.SetCookie(w, &http.Cookie{
			Name:     "contact_cooldown",
			Value:    cooldownKey,
			Path:     "/",
			MaxAge:   int(time.Hour.Seconds()),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   r.TLS != nil,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}
}

func isValidRequest(request ContactRequest) bool {
	return between(strings.TrimSpace(request.Name), 2, 100) &&
		between(strings.TrimSpace(request.Contact), 5, 254) &&
		between(strings.TrimSpace(request.ProjectType), 2, 120) &&
		between(strings.TrimSpace(optionalString(request.ProjectDetails)), 0, 5000) &&
		bool(request.Consent) &&
		(request.Website == nil || *request.Website == "")
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func between(value string, minimum int, maximum int) bool {
	length := len([]rune(value))
	return length >= minimum && length <= maximum
}

func cooldownKey(r *http.Request) (string, error) {
	if cookie, err := r.Cookie("contact_cooldown"); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}`))
}
