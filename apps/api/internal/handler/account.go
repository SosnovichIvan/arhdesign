package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/preferences"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type AccountEndpoint struct {
	service       *account.Service
	projects      *project.Service
	preferences   *preferences.Service
	globalChats   *globalchat.Service
	technical     *technicalsupport.Service
	publicOrigin  string
	secureCookies bool
}

func (endpoint *AccountEndpoint) ConfigureProjects(service *project.Service) {
	endpoint.projects = service
}

func (endpoint *AccountEndpoint) ConfigurePreferences(service *preferences.Service) {
	endpoint.preferences = service
}

func (endpoint *AccountEndpoint) ConfigureGlobalChats(service *globalchat.Service) {
	endpoint.globalChats = service
}

func (endpoint *AccountEndpoint) ConfigureTechnicalSupport(service *technicalsupport.Service) {
	endpoint.technical = service
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,100}$`)

func NewAccountEndpoint(service *account.Service) (*AccountEndpoint, error) {
	if service == nil {
		return nil, errors.New("account endpoint requires service")
	}
	origin := service.PublicOrigin()
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil, errors.New("account endpoint requires valid public origin")
	}
	return &AccountEndpoint{service: service, publicOrigin: origin, secureCookies: parsed.Scheme == "https"}, nil
}

func (endpoint *AccountEndpoint) Login(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if !endpoint.validBrowserMutation(r) {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	var request accountgenerated.LoginRequest
	if !decodeAccountJSON(r, &request) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте логин и пароль", requestID)
		return
	}
	remember := request.RememberMe != nil && *request.RememberMe
	session, err := endpoint.service.Login(r.Context(), account.LoginRequest{Identifier: request.Identifier, Password: request.Password, RememberMe: remember, RequestID: requestID, AnonymousMarker: accountAnonymousMarker(r)})
	if err != nil {
		var rateLimit account.RateLimitError
		switch {
		case errors.Is(err, account.ErrInvalidInput):
			writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте логин и пароль", requestID)
		case errors.Is(err, account.ErrInvalidCredentials):
			writeAccountError(w, http.StatusUnauthorized, "invalid_credentials", "Неверный логин или пароль", requestID)
		case errors.Is(err, account.ErrAccountUnverified):
			writeAccountError(w, http.StatusForbidden, "account_unverified", "Учётная запись не подтверждена", requestID)
		case errors.Is(err, account.ErrAccountUnavailable):
			writeAccountError(w, http.StatusForbidden, "account_unavailable", "Учётная запись недоступна", requestID)
		case errors.As(err, &rateLimit):
			seconds := int64(rateLimit.RetryAfter / time.Second)
			if rateLimit.RetryAfter%time.Second != 0 {
				seconds++
			}
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
			writeAccountError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много попыток. Повторите позже", requestID)
		case errors.Is(err, account.ErrServiceUnavailable):
			writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис временно недоступен", requestID)
		default:
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось выполнить вход", requestID)
		}
		return
	}
	endpoint.setSessionCookies(w, session)
	writeAccountJSON(w, http.StatusOK, sessionResponse(session))
}

func accountAnonymousMarker(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Arhdesign-Client-IP")); net.ParseIP(forwarded) != nil {
		return forwarded
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && net.ParseIP(host) != nil {
		return host
	}
	if direct := strings.TrimSpace(r.RemoteAddr); net.ParseIP(direct) != nil {
		return direct
	}
	return "unknown"
}

func (endpoint *AccountEndpoint) RefreshSession(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	csrf, ok := endpoint.validAuthenticatedMutation(r)
	if !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	refreshCookie, err := r.Cookie(endpoint.cookieName("refresh"))
	if err != nil {
		endpoint.clearSessionCookies(w)
		writeAccountError(w, http.StatusUnauthorized, "unauthenticated", "Требуется повторный вход", requestID)
		return
	}
	session, err := endpoint.service.Refresh(r.Context(), refreshCookie.Value, csrf, requestID)
	if err != nil {
		endpoint.clearSessionCookies(w)
		if errors.Is(err, account.ErrUnauthenticated) {
			writeAccountError(w, http.StatusUnauthorized, "unauthenticated", "Требуется повторный вход", requestID)
		} else {
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось обновить сессию", requestID)
		}
		return
	}
	endpoint.setSessionCookies(w, session)
	writeAccountJSON(w, http.StatusOK, sessionResponse(session))
}

func (endpoint *AccountEndpoint) Logout(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	csrf, ok := endpoint.validAuthenticatedMutation(r)
	if !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	access, refresh := "", ""
	if cookie, err := r.Cookie(endpoint.cookieName("session")); err == nil {
		access = cookie.Value
	}
	if cookie, err := r.Cookie(endpoint.cookieName("refresh")); err == nil {
		refresh = cookie.Value
	}
	err := endpoint.service.Logout(r.Context(), access, refresh, csrf, requestID)
	endpoint.clearSessionCookies(w)
	if err != nil && !errors.Is(err, account.ErrUnauthenticated) {
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось завершить сеанс", requestID)
		return
	}
	if errors.Is(err, account.ErrUnauthenticated) {
		writeAccountError(w, http.StatusUnauthorized, "unauthenticated", "Требуется повторный вход", requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) validBrowserMutation(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		referer := strings.TrimSpace(r.Header.Get("Referer"))
		parsed, err := url.Parse(referer)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return false
		}
		origin = parsed.Scheme + "://" + parsed.Host
	}
	if origin != endpoint.publicOrigin {
		return false
	}
	return r.Header.Get("Sec-Fetch-Site") == "" || r.Header.Get("Sec-Fetch-Site") == "same-origin"
}

func (endpoint *AccountEndpoint) validAuthenticatedMutation(r *http.Request) (string, bool) {
	if !endpoint.validBrowserMutation(r) {
		return "", false
	}
	csrfCookie, err := r.Cookie(endpoint.cookieName("csrf"))
	if err != nil {
		return "", false
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(csrfCookie.Value)) != 1 {
		return "", false
	}
	return header, true
}

func (endpoint *AccountEndpoint) cookieName(kind string) string {
	if !endpoint.secureCookies {
		return "arhdesign_" + kind
	}
	if kind == "refresh" {
		return "__Secure-arhdesign_refresh"
	}
	return "__Host-arhdesign_" + kind
}

func (endpoint *AccountEndpoint) setSessionCookies(w http.ResponseWriter, session account.Session) {
	now := time.Now()
	http.SetCookie(w, &http.Cookie{Name: endpoint.cookieName("session"), Value: session.AccessToken, Path: "/", HttpOnly: true, Secure: endpoint.secureCookies, SameSite: http.SameSiteStrictMode, Expires: session.AccessExpiresAt, MaxAge: maxAge(now, session.AccessExpiresAt)})
	refresh := &http.Cookie{Name: endpoint.cookieName("refresh"), Value: session.RefreshToken, Path: "/api/v1/auth", HttpOnly: true, Secure: endpoint.secureCookies, SameSite: http.SameSiteStrictMode}
	if session.RememberMe {
		refresh.Expires = session.RefreshExpiresAt
		refresh.MaxAge = maxAge(now, session.RefreshExpiresAt)
	}
	http.SetCookie(w, refresh)
	http.SetCookie(w, &http.Cookie{Name: endpoint.cookieName("csrf"), Value: session.CSRFToken, Path: "/", Secure: endpoint.secureCookies, SameSite: http.SameSiteStrictMode, Expires: session.AccessExpiresAt, MaxAge: maxAge(now, session.AccessExpiresAt)})
}

func (endpoint *AccountEndpoint) clearSessionCookies(w http.ResponseWriter) {
	for _, kind := range []string{"session", "refresh", "csrf"} {
		path := "/"
		if kind == "refresh" {
			path = "/api/v1/auth"
		}
		http.SetCookie(w, &http.Cookie{Name: endpoint.cookieName(kind), Value: "", Path: path, HttpOnly: kind != "csrf", Secure: endpoint.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	}
}

func maxAge(now, expires time.Time) int {
	seconds := int(expires.Sub(now).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func sessionResponse(session account.Session) accountgenerated.SessionView {
	return accountgenerated.SessionView{ExpiresAt: session.AccessExpiresAt, Account: accountViewResponse(session.Account)}
}

func accountViewResponse(view account.AccountView) accountgenerated.AccountView {
	id, _ := uuid.Parse(view.ID)
	var globalRole *accountgenerated.AccountViewGlobalRole
	if view.GlobalRole != nil {
		role := accountgenerated.AccountViewGlobalRole(*view.GlobalRole)
		globalRole = &role
	}
	return accountgenerated.AccountView{
		Id: id, Login: view.Login, Email: openapi_types.Email(view.Email), FirstName: view.FirstName, LastName: view.LastName, MiddleName: view.MiddleName,
		ProfessionalRole: accountgenerated.ProfessionalRole{Code: view.ProfessionalRoleCode, Name: view.ProfessionalRoleName}, Status: accountgenerated.AccountStatus(view.Status), GlobalRole: globalRole, Version: view.Version,
	}
}

func (endpoint *AccountEndpoint) GetCurrentAccount(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	view, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return
	}
	writeAccountJSON(w, http.StatusOK, accountViewResponse(view))
}

func (endpoint *AccountEndpoint) authenticatedAccount(w http.ResponseWriter, r *http.Request, requestID string) (account.AccountView, bool) {
	cookie, err := r.Cookie(endpoint.cookieName("session"))
	if err != nil {
		writeAccountError(w, http.StatusUnauthorized, "unauthenticated", "Требуется вход", requestID)
		return account.AccountView{}, false
	}
	view, err := endpoint.service.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, account.ErrUnauthenticated) {
			writeAccountError(w, http.StatusUnauthorized, "unauthenticated", "Требуется повторный вход", requestID)
		} else {
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось проверить сессию", requestID)
		}
		return account.AccountView{}, false
	}
	return view, true
}

func (endpoint *AccountEndpoint) RegisterAccount(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	var request accountgenerated.RegisterRequest
	if !decodeAccountJSON(r, &request) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля формы", requestID)
		return
	}
	_, err := endpoint.service.Register(r.Context(), account.Registration{
		Login: request.Login, Email: string(request.Email), FirstName: request.FirstName,
		LastName: optionalAccountString(request.LastName), MiddleName: optionalAccountString(request.MiddleName),
		ProfessionalRoleCode: request.ProfessionalRoleCode, Password: request.Password,
		PasswordConfirmation: request.PasswordConfirmation, RequestID: requestID,
	})
	if err != nil {
		var validation account.RegistrationValidationError
		switch {
		case errors.As(err, &validation):
			writeAccountFieldError(w, http.StatusBadRequest, "validation_error", "Исправьте выделенные поля", requestID, validation.Fields)
		case errors.Is(err, account.ErrInvalidInput), errors.Is(err, account.ErrInvalidPassword):
			writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля формы", requestID)
		case errors.Is(err, account.ErrIdentifierUnavailable):
			writeAccountError(w, http.StatusConflict, "identifier_unavailable", "Логин или почта недоступны", requestID)
		default:
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось завершить регистрацию", requestID)
		}
		return
	}
	writeAccountJSON(w, http.StatusAccepted, accountgenerated.PendingVerification{Status: accountgenerated.VerificationPending, ChannelSelectionRequired: accountgenerated.True})
}

func (endpoint *AccountEndpoint) SelectVerificationChannel(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if !endpoint.validBrowserMutation(r) {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	var request accountgenerated.VerificationChannelRequest
	if !decodeAccountJSON(r, &request) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте способ подтверждения", requestID)
		return
	}
	handoff, err := endpoint.service.SelectVerificationChannel(r.Context(), request.Identifier, string(request.Channel), requestID)
	if err != nil {
		if errors.Is(err, account.ErrInvalidInput) {
			writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте способ подтверждения", requestID)
		} else if errors.Is(err, account.ErrServiceUnavailable) {
			writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис временно недоступен", requestID)
		} else {
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось выбрать способ подтверждения", requestID)
		}
		return
	}
	var telegramURL *string
	if handoff != "" {
		telegramURL = &handoff
	}
	writeAccountJSON(w, http.StatusAccepted, accountgenerated.ChannelHandoff{Status: accountgenerated.ChannelHandoffStatusAccepted, Channel: request.Channel, TelegramBotUrl: telegramURL})
}

func (endpoint *AccountEndpoint) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	var request accountgenerated.OneTimeTokenRequest
	if !decodeAccountJSON(r, &request) || request.Token == nil {
		writeAccountError(w, http.StatusBadRequest, "invalid_or_expired_token", "Ссылка недействительна или устарела", requestID)
		return
	}
	if err := endpoint.service.Verify(r.Context(), *request.Token, requestID); err != nil {
		if errors.Is(err, account.ErrInvalidToken) {
			writeAccountError(w, http.StatusBadRequest, "invalid_or_expired_token", "Ссылка недействительна или устарела", requestID)
		} else {
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось подтвердить почту", requestID)
		}
		return
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.SimpleStatus{Status: "email_verified"})
}

func (endpoint *AccountEndpoint) ResendVerification(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	var request accountgenerated.IdentifierRequest
	if !decodeAccountJSON(r, &request) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте идентификатор", requestID)
		return
	}
	if err := endpoint.service.Resend(r.Context(), request.Identifier, requestID); err != nil {
		if errors.Is(err, account.ErrInvalidInput) {
			writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте идентификатор", requestID)
		} else {
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось обработать запрос", requestID)
		}
		return
	}
	writeAccountJSON(w, http.StatusAccepted, accountgenerated.SimpleStatus{Status: "mail_accepted"})
}

func (endpoint *AccountEndpoint) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if !endpoint.validBrowserMutation(r) {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	var request accountgenerated.PasswordRecoveryRequest
	if !decodeAccountJSON(r, &request) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте идентификатор", requestID)
		return
	}
	err := endpoint.service.ForgotPassword(r.Context(), request.Identifier, string(request.Channel), accountAnonymousMarker(r), requestID)
	if err != nil {
		var rateLimit account.RateLimitError
		switch {
		case errors.Is(err, account.ErrInvalidInput):
			writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте идентификатор", requestID)
		case errors.As(err, &rateLimit):
			setRetryAfter(w, rateLimit.RetryAfter)
			writeAccountError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много попыток. Повторите позже", requestID)
		case errors.Is(err, account.ErrServiceUnavailable):
			writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис временно недоступен", requestID)
		default:
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось обработать запрос", requestID)
		}
		return
	}
	writeAccountJSON(w, http.StatusAccepted, accountgenerated.SimpleStatus{Status: "recovery_accepted"})
}

func (endpoint *AccountEndpoint) ResetPassword(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if !endpoint.validBrowserMutation(r) {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	var request accountgenerated.ResetPasswordRequest
	if !decodeAccountJSON(r, &request) || request.Token == nil || request.Password == nil || request.PasswordConfirmation == nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля формы", requestID)
		return
	}
	err := endpoint.service.ResetPassword(r.Context(), *request.Token, *request.Password, *request.PasswordConfirmation, accountAnonymousMarker(r), requestID)
	if err != nil {
		var rateLimit account.RateLimitError
		switch {
		case errors.Is(err, account.ErrInvalidToken):
			writeAccountError(w, http.StatusBadRequest, "invalid_or_expired_token", "Ссылка недействительна или устарела", requestID)
		case errors.Is(err, account.ErrInvalidInput), errors.Is(err, account.ErrInvalidPassword):
			writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте новый пароль", requestID)
		case errors.As(err, &rateLimit):
			setRetryAfter(w, rateLimit.RetryAfter)
			writeAccountError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много попыток. Повторите позже", requestID)
		case errors.Is(err, account.ErrServiceUnavailable):
			writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис временно недоступен", requestID)
		default:
			writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось изменить пароль", requestID)
		}
		return
	}
	endpoint.clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func setRetryAfter(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int64(retryAfter / time.Second)
	if retryAfter%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
}

func decodeAccountJSON(r *http.Request, target any) bool {
	if mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); mediaType != "application/json" {
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func accountRequestID(w http.ResponseWriter, r *http.Request) string {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if !requestIDPattern.MatchString(requestID) {
		bytes := make([]byte, 12)
		if _, err := rand.Read(bytes); err != nil {
			requestID = "request-unknown"
		} else {
			requestID = base64.RawURLEncoding.EncodeToString(bytes)
		}
	}
	w.Header().Set("X-Request-ID", requestID)
	return requestID
}

func optionalAccountString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func writeAccountError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeAccountJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "requestId": requestID}})
}

func writeAccountFieldError(w http.ResponseWriter, status int, code, message, requestID string, fields map[string]string) {
	writeAccountJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "requestId": requestID, "fields": fields}})
}

func writeAccountJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
