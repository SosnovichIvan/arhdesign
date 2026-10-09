package technicalsupport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
)

var (
	ErrInvalidInput = errors.New("invalid technical support input")
	ErrRateLimited  = errors.New("technical support rate limited")
)

var (
	urlPattern       = regexp.MustCompile(`https?://\S+`)
	emailPattern     = regexp.MustCompile(`(?i)[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}`)
	bearerPattern    = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)
	longTokenPattern = regexp.MustCompile(`[A-Za-z0-9_-]{24,}`)
)

type Actor struct {
	UserID, Login, FirstName string
}

type FrontendErrorRequest struct {
	Message, Path, Fingerprint, Digest, RequestID string
}

type FeedbackRequest struct {
	Category, Message, Path, RequestID string
}

type BackendError struct {
	Method, Route, RequestID, ErrorCode string
	StatusCode                          int
	OccurredAt                          time.Time
}

type Report struct {
	Kind, Category, ActorUserID, RequestID string
	Method, Route, ErrorCode               string
	StatusCode                             int
	Fingerprint, Summary, UserMessage      string
	PayloadCiphertext                      []byte
	PayloadKeyVersion                      int
	Now                                    time.Time
}

type Store interface {
	ConsumeRateLimit(context.Context, account.RateLimitRequest) (time.Duration, error)
	CreateTechnicalReport(context.Context, Report) (bool, error)
}

type PayloadEncryptor interface {
	Encrypt([]byte, string) ([]byte, int, error)
}

type Service struct {
	store      Store
	cipher     PayloadEncryptor
	hmacKey    []byte
	keyVersion int
	now        func() time.Time
}

func NewService(store Store, cipher PayloadEncryptor, hmacKey []byte, keyVersion int, now func() time.Time) (*Service, error) {
	if store == nil || cipher == nil || len(hmacKey) < 32 || keyVersion < 1 {
		return nil, errors.New("technical support service requires store, cipher and HMAC key")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, cipher: cipher, hmacKey: append([]byte(nil), hmacKey...), keyVersion: keyVersion, now: now}, nil
}

func (service *Service) ReportFrontend(ctx context.Context, actor Actor, request FrontendErrorRequest) error {
	path := sanitizePath(request.Path)
	message := sanitizeErrorMessage(request.Message)
	if actor.UserID == "" || path == "" || len(request.RequestID) < 8 || !safeFingerprint(request.Fingerprint) || message == "" || len([]rune(request.Digest)) > 128 {
		return ErrInvalidInput
	}
	if err := service.limit(ctx, actor.UserID, "technical.frontend", 10, time.Hour); err != nil {
		return err
	}
	now := service.now().UTC()
	fingerprint := service.fingerprint("frontend_error", path, request.Fingerprint, request.Digest)
	summary := "Ошибка интерфейса: " + message
	text := "Ошибка frontend\nСтраница: " + path + "\nКод: " + shortFingerprint(fingerprint) + "\nОписание: " + message
	ciphertext, keyVersion := service.encrypt(text, "technical.frontend_error")
	return service.create(ctx, Report{Kind: "frontend_error", ActorUserID: actor.UserID, RequestID: request.RequestID, Route: path, ErrorCode: "frontend_runtime", Fingerprint: fingerprint, Summary: summary, PayloadCiphertext: ciphertext, PayloadKeyVersion: keyVersion, Now: now})
}

func (service *Service) CreateFeedback(ctx context.Context, actor Actor, request FeedbackRequest) error {
	path := sanitizePath(request.Path)
	message := strings.TrimSpace(request.Message)
	if actor.UserID == "" || path == "" || len(request.RequestID) < 8 || (request.Category != "complaint" && request.Category != "suggestion") || len([]rune(message)) < 5 || len([]rune(message)) > 2000 {
		return ErrInvalidInput
	}
	if err := service.limit(ctx, actor.UserID, "technical.feedback", 3, time.Hour); err != nil {
		return err
	}
	now := service.now().UTC()
	fingerprint := service.fingerprint("feedback", actor.UserID, request.RequestID)
	categoryLabel := map[string]string{"complaint": "Жалоба", "suggestion": "Пожелание"}[request.Category]
	text := categoryLabel + " разработчику\nОт: " + actor.FirstName + " (@" + actor.Login + ")\nСтраница: " + path + "\n\n" + message
	ciphertext, keyVersion := service.encrypt(text, "technical.feedback")
	return service.create(ctx, Report{Kind: "feedback", Category: request.Category, ActorUserID: actor.UserID, RequestID: request.RequestID, Route: path, Fingerprint: fingerprint, Summary: categoryLabel + " пользователя", UserMessage: message, PayloadCiphertext: ciphertext, PayloadKeyVersion: keyVersion, Now: now})
}

func (service *Service) ReportBackend(ctx context.Context, event BackendError) error {
	route := sanitizePath(event.Route)
	if route == "" {
		route = "/unknown"
	}
	if event.StatusCode < 500 || event.StatusCode > 599 || len(event.RequestID) < 8 {
		return ErrInvalidInput
	}
	method := strings.ToUpper(strings.TrimSpace(event.Method))
	if method == "" || len(method) > 10 {
		method = "UNKNOWN"
	}
	code := strings.TrimSpace(event.ErrorCode)
	if code == "" {
		code = "http_5xx"
	}
	now := event.OccurredAt.UTC()
	if now.IsZero() {
		now = service.now().UTC()
	}
	fingerprint := service.fingerprint("backend_error", method, route, fmt.Sprint(event.StatusCode), code)
	text := fmt.Sprintf("Ошибка backend\n%s %s\nСтатус: %d\nRequest ID: %s\nКод: %s", method, route, event.StatusCode, event.RequestID, shortFingerprint(fingerprint))
	ciphertext, keyVersion := service.encrypt(text, "technical.backend_error")
	return service.create(ctx, Report{Kind: "backend_error", RequestID: event.RequestID, Method: method, Route: route, StatusCode: event.StatusCode, ErrorCode: code, Fingerprint: fingerprint, Summary: fmt.Sprintf("%s %s returned %d", method, route, event.StatusCode), PayloadCiphertext: ciphertext, PayloadKeyVersion: keyVersion, Now: now})
}

func (service *Service) create(ctx context.Context, report Report) error {
	if len(report.PayloadCiphertext) == 0 || report.PayloadKeyVersion == 0 {
		return errors.New("technical report payload encryption failed")
	}
	_, err := service.store.CreateTechnicalReport(ctx, report)
	return err
}

func (service *Service) encrypt(text, messageType string) ([]byte, int) {
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, 0
	}
	ciphertext, version, err := service.cipher.Encrypt(payload, messageType)
	if err != nil {
		return nil, 0
	}
	return ciphertext, version
}

func (service *Service) limit(ctx context.Context, subject, policy string, capacity int, refill time.Duration) error {
	retryAfter, err := service.store.ConsumeRateLimit(ctx, account.RateLimitRequest{PolicyCode: policy, SubjectHash: service.hash(subject + "|" + policy), HashKeyVersion: service.keyVersion, Capacity: capacity, FullRefill: refill, Retention: 48 * time.Hour, Now: service.now().UTC()})
	if err != nil {
		return err
	}
	if retryAfter > 0 {
		return ErrRateLimited
	}
	return nil
}

func (service *Service) fingerprint(parts ...string) string {
	return base64.RawURLEncoding.EncodeToString(service.hash(strings.Join(parts, "|")))
}
func (service *Service) hash(value string) []byte {
	mac := hmac.New(sha256.New, service.hmacKey)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func sanitizePath(value string) string {
	value = strings.TrimSpace(value)
	if query := strings.IndexAny(value, "?#"); query >= 0 {
		value = value[:query]
	}
	if !strings.HasPrefix(value, "/") || len([]rune(value)) > 300 {
		return ""
	}
	return value
}

func sanitizeErrorMessage(value string) string {
	value = strings.TrimSpace(value)
	value = urlPattern.ReplaceAllString(value, "[url]")
	value = emailPattern.ReplaceAllString(value, "[email]")
	value = bearerPattern.ReplaceAllString(value, "[credential]")
	value = longTokenPattern.ReplaceAllString(value, "[token]")
	runes := []rune(value)
	if len(runes) > 300 {
		value = string(runes[:300])
	}
	return value
}

func safeFingerprint(value string) bool {
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func shortFingerprint(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
