package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
)

type endpointAccountStore struct {
	createErr        error
	consumeErr       error
	pendingEmail     string
	found            bool
	pendingErr       error
	replaceErr       error
	loginAccount     account.LoginAccount
	loginFound       bool
	loginErr         error
	accessAccount    account.AccountView
	accessFound      bool
	accessErr        error
	newSession       account.NewSession
	rotationAccount  account.LoginAccount
	rotation         account.SessionRotation
	rotationErr      error
	revocation       account.SessionRevocation
	revocationErr    error
	rateLimitRetry   time.Duration
	rateLimitErr     error
	activeEmail      string
	activeFound      bool
	activeEmailErr   error
	resetAccount     account.PasswordResetAccount
	resetContextErr  error
	resetCompleteErr error
	channelSelection account.VerificationChannelSelection
	channelFound     bool
	channelErr       error
	adminUsers       []account.AdminUserView
	adminListErr     error
	adminStatus      account.AdminUserView
	adminStatusErr   error
	adminQuery       account.AdminUserQuery
	adminCommand     account.AdminUserStatusCommand
}

func (store *endpointAccountStore) ListAdminUsers(_ context.Context, query account.AdminUserQuery) ([]account.AdminUserView, error) {
	store.adminQuery = query
	return store.adminUsers, store.adminListErr
}
func (store *endpointAccountStore) SetAdminUserStatus(_ context.Context, command account.AdminUserStatusCommand) (account.AdminUserView, error) {
	store.adminCommand = command
	return store.adminStatus, store.adminStatusErr
}

func (store *endpointAccountStore) FindLoginAccount(context.Context, string) (account.LoginAccount, bool, error) {
	return store.loginAccount, store.loginFound, store.loginErr
}

func (store *endpointAccountStore) ResolveAccessSession(context.Context, []byte, time.Time) (account.AccountView, bool, error) {
	return store.accessAccount, store.accessFound, store.accessErr
}

func (store *endpointAccountStore) CreateSession(_ context.Context, session account.NewSession) error {
	store.newSession = session
	return nil
}
func (store *endpointAccountStore) BootstrapSuperAdminSession(context.Context, account.BootstrapSession) (account.LoginAccount, bool, error) {
	return account.LoginAccount{}, false, nil
}

func (store *endpointAccountStore) EnsureTechnicalAdmin(context.Context, account.TechnicalAdminBootstrap) (account.AccountView, bool, error) {
	return account.AccountView{}, false, nil
}
func (store *endpointAccountStore) RotateSession(_ context.Context, rotation account.SessionRotation) (account.LoginAccount, error) {
	store.rotation = rotation
	return store.rotationAccount, store.rotationErr
}
func (store *endpointAccountStore) RevokeSessionFamily(_ context.Context, revocation account.SessionRevocation) error {
	store.revocation = revocation
	return store.revocationErr
}
func (store *endpointAccountStore) RecordLoginFailure(context.Context, string, string) error {
	return nil
}
func (store *endpointAccountStore) ConsumeRateLimit(context.Context, account.RateLimitRequest) (time.Duration, error) {
	return store.rateLimitRetry, store.rateLimitErr
}
func (store *endpointAccountStore) ActiveAccountEmail(context.Context, string) (string, bool, error) {
	return store.activeEmail, store.activeFound, store.activeEmailErr
}
func (store *endpointAccountStore) CreatePasswordReset(context.Context, account.PasswordResetIssue) (bool, error) {
	return store.activeFound, nil
}
func (store *endpointAccountStore) CreateTelegramPasswordReset(context.Context, account.PasswordResetIssue) (bool, error) {
	return store.activeFound, nil
}
func (store *endpointAccountStore) PasswordResetContext(context.Context, []byte, int, time.Time, string) (account.PasswordResetAccount, error) {
	return store.resetAccount, store.resetContextErr
}
func (store *endpointAccountStore) CompletePasswordReset(context.Context, account.PasswordResetCompletion) error {
	return store.resetCompleteErr
}
func (store *endpointAccountStore) RecordPasswordResetRequest(context.Context, string, string) error {
	return nil
}
func (store *endpointAccountStore) RecordPasswordResetFailure(context.Context, string) error {
	return nil
}
func (store *endpointAccountStore) BeginTelegramAuth(context.Context, account.TelegramAuthStart) error {
	return nil
}
func (store *endpointAccountStore) AdvanceTelegramAuth(context.Context, account.TelegramAuthAdvance) error {
	return nil
}
func (store *endpointAccountStore) TelegramAuthStep(context.Context, int64, time.Time) (string, error) {
	return "", account.ErrInvalidToken
}
func (store *endpointAccountStore) TelegramAuthAccount(context.Context, int64, time.Time) (account.LoginAccount, bool, error) {
	return account.LoginAccount{}, false, nil
}
func (store *endpointAccountStore) CompleteTelegramAuth(context.Context, account.TelegramAuthCompletion) error {
	return nil
}
func (store *endpointAccountStore) RejectTelegramAuth(context.Context, account.TelegramAuthRejection) error {
	return nil
}

func (store *endpointAccountStore) CreatePendingAccount(context.Context, account.PendingAccount) error {
	return store.createErr
}
func (store *endpointAccountStore) ConsumeVerificationToken(context.Context, []byte, int, time.Time, string) error {
	return store.consumeErr
}
func (store *endpointAccountStore) PendingAccountEmail(context.Context, string) (string, bool, error) {
	return store.pendingEmail, store.found, store.pendingErr
}
func (store *endpointAccountStore) ReplaceVerificationToken(context.Context, account.ReplacementVerification) (bool, error) {
	return store.found, store.replaceErr
}
func (store *endpointAccountStore) SetVerificationChannel(_ context.Context, selection account.VerificationChannelSelection) (bool, error) {
	store.channelSelection = selection
	return store.channelFound, store.channelErr
}

func accountEndpointForTest(t *testing.T, store account.Store) *AccountEndpoint {
	t.Helper()
	tokens, err := account.NewTokenManager(bytes.Repeat([]byte{1}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "from@example.com", func() time.Time { return time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureTelegramBot("polismakovaSvetlanaBot")
	endpoint, err := NewAccountEndpoint(service)
	if err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func TestSelectVerificationChannelResponsesAreGenericAndOpenTelegram(t *testing.T) {
	for _, found := range []bool{true, false} {
		store := &endpointAccountStore{pendingEmail: "sveta@example.com", found: found, channelFound: found}
		endpoint := accountEndpointForTest(t, store)
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-channel", strings.NewReader(`{"identifier":"sveta.design","channel":"telegram"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://designer-svetlana.ru")
		request.Header.Set("X-Request-ID", "request-channel")
		endpoint.SelectVerificationChannel(response, request)
		if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"status":"accepted"`) || !strings.Contains(response.Body.String(), "https://t.me/polismakovaSvetlanaBot?start=register") {
			t.Fatalf("found=%v status=%d body=%s", found, response.Code, response.Body.String())
		}
	}

	email := httptest.NewRecorder()
	emailRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-channel", strings.NewReader(`{"identifier":"sveta.design","channel":"email"}`))
	emailRequest.Header.Set("Content-Type", "application/json")
	emailRequest.Header.Set("Origin", "https://designer-svetlana.ru")
	accountEndpointForTest(t, &endpointAccountStore{pendingEmail: "sveta@example.com", found: true}).SelectVerificationChannel(email, emailRequest)
	if email.Code != http.StatusAccepted || strings.Contains(email.Body.String(), "t.me/") {
		t.Fatalf("email status=%d body=%s", email.Code, email.Body.String())
	}

	for _, test := range []struct {
		name, body, origin string
		store              *endpointAccountStore
		status             int
	}{
		{"cross-site", `{"identifier":"sveta.design","channel":"email"}`, "https://evil.example", &endpointAccountStore{}, http.StatusForbidden},
		{"malformed", `{}`, "https://designer-svetlana.ru", &endpointAccountStore{}, http.StatusBadRequest},
		{"invalid", `{"identifier":"x","channel":"email"}`, "https://designer-svetlana.ru", &endpointAccountStore{}, http.StatusBadRequest},
		{"unavailable", `{"identifier":"sveta.design","channel":"email"}`, "https://designer-svetlana.ru", &endpointAccountStore{pendingErr: account.ErrServiceUnavailable}, http.StatusServiceUnavailable},
		{"internal", `{"identifier":"sveta.design","channel":"telegram"}`, "https://designer-svetlana.ru", &endpointAccountStore{channelErr: errors.New("database unavailable")}, http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-channel", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", test.origin)
			accountEndpointForTest(t, test.store).SelectVerificationChannel(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRegisterAccountResponses(t *testing.T) {
	valid := `{"login":"sveta.design","email":"sveta@example.com","firstName":"Светлана","lastName":"Полисмакова","professionalRoleCode":"designer","password":"Надёжный пароль 2026!","passwordConfirmation":"Надёжный пароль 2026!"}`
	tests := []struct {
		name        string
		store       *endpointAccountStore
		body        string
		contentType string
		status      int
		code        string
	}{
		{"accepted", &endpointAccountStore{}, valid, "application/json", http.StatusAccepted, "verification_pending"},
		{"duplicate", &endpointAccountStore{createErr: account.ErrIdentifierUnavailable}, valid, "application/json", http.StatusConflict, "identifier_unavailable"},
		{"invalid", &endpointAccountStore{}, `{"login":"?"}`, "application/json", http.StatusBadRequest, "validation_error"},
		{"unknown field", &endpointAccountStore{}, strings.TrimSuffix(valid, "}") + `,"extra":true}`, "application/json", http.StatusBadRequest, "validation_error"},
		{"wrong content type", &endpointAccountStore{}, valid, "text/plain", http.StatusBadRequest, "validation_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := accountEndpointForTest(t, test.store)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set("X-Request-ID", "request-123")
			endpoint.RegisterAccount(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) || response.Header().Get("X-Request-ID") != "request-123" {
				t.Fatalf("status=%d body=%s requestID=%q", response.Code, response.Body.String(), response.Header().Get("X-Request-ID"))
			}
			if test.name == "invalid" && (!strings.Contains(response.Body.String(), `"fields"`) || !strings.Contains(response.Body.String(), `"login"`)) {
				t.Fatalf("field validation details missing: %s", response.Body.String())
			}
		})
	}
}

func TestVerifyAndResendResponses(t *testing.T) {
	store := &endpointAccountStore{pendingEmail: "sveta@example.com", found: true}
	endpoint := accountEndpointForTest(t, store)
	verify := httptest.NewRecorder()
	verifyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":"`+strings.Repeat("a", 43)+`"}`))
	verifyRequest.Header.Set("Content-Type", "application/json")
	endpoint.VerifyEmail(verify, verifyRequest)
	if verify.Code != http.StatusOK || !strings.Contains(verify.Body.String(), "email_verified") {
		t.Fatalf("verify status=%d body=%s", verify.Code, verify.Body.String())
	}

	store.consumeErr = account.ErrInvalidToken
	invalid := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":"`+strings.Repeat("b", 43)+`"}`))
	invalidRequest.Header.Set("Content-Type", "application/json")
	endpoint.VerifyEmail(invalid, invalidRequest)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "invalid_or_expired_token") {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}

	resend := httptest.NewRecorder()
	resendRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification", strings.NewReader(`{"identifier":"sveta@example.com"}`))
	resendRequest.Header.Set("Content-Type", "application/json")
	endpoint.ResendVerification(resend, resendRequest)
	if resend.Code != http.StatusAccepted || !strings.Contains(resend.Body.String(), "mail_accepted") {
		t.Fatalf("resend status=%d body=%s", resend.Code, resend.Body.String())
	}

	absent := accountEndpointForTest(t, &endpointAccountStore{})
	absentResponse := httptest.NewRecorder()
	absentRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification", strings.NewReader(`{"identifier":"absent@example.com"}`))
	absentRequest.Header.Set("Content-Type", "application/json")
	absent.ResendVerification(absentResponse, absentRequest)
	if absentResponse.Code != resend.Code || absentResponse.Body.String() != resend.Body.String() {
		t.Fatalf("existing response %d/%q differs from absent %d/%q", resend.Code, resend.Body.String(), absentResponse.Code, absentResponse.Body.String())
	}
}

func TestForgotAndResetPasswordResponses(t *testing.T) {
	store := &endpointAccountStore{activeEmail: "sveta@example.com", activeFound: true, resetAccount: account.PasswordResetAccount{Login: "sveta.design", Email: "sveta@example.com"}}
	endpoint := accountEndpointForTest(t, store)
	request := func(path, body string) *http.Request {
		result := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		result.Header.Set("Content-Type", "application/json")
		result.Header.Set("Origin", "https://designer-svetlana.ru")
		return result
	}
	forgot := httptest.NewRecorder()
	endpoint.ForgotPassword(forgot, request("/api/v1/auth/forgot-password", `{"identifier":"sveta@example.com","channel":"email"}`))
	if forgot.Code != http.StatusAccepted || !strings.Contains(forgot.Body.String(), "recovery_accepted") {
		t.Fatalf("forgot status=%d body=%s", forgot.Code, forgot.Body.String())
	}
	reset := httptest.NewRecorder()
	endpoint.ResetPassword(reset, request("/api/v1/auth/reset-password", `{"token":"`+strings.Repeat("r", 43)+`","password":"Новый пароль 2026 надёжный!","passwordConfirmation":"Новый пароль 2026 надёжный!"}`))
	if reset.Code != http.StatusNoContent || len(reset.Result().Cookies()) != 3 {
		t.Fatalf("reset status=%d cookies=%d body=%s", reset.Code, len(reset.Result().Cookies()), reset.Body.String())
	}

	limited := accountEndpointForTest(t, &endpointAccountStore{rateLimitRetry: 500 * time.Millisecond})
	limitedResponse := httptest.NewRecorder()
	limited.ForgotPassword(limitedResponse, request("/api/v1/auth/forgot-password", `{"identifier":"sveta@example.com","channel":"email"}`))
	if limitedResponse.Code != http.StatusTooManyRequests || limitedResponse.Header().Get("Retry-After") != "1" {
		t.Fatalf("limited status=%d retry=%q", limitedResponse.Code, limitedResponse.Header().Get("Retry-After"))
	}
	invalid := accountEndpointForTest(t, &endpointAccountStore{resetContextErr: account.ErrInvalidToken})
	invalidResponse := httptest.NewRecorder()
	invalid.ResetPassword(invalidResponse, request("/api/v1/auth/reset-password", `{"token":"`+strings.Repeat("r", 43)+`","password":"Новый пароль 2026 надёжный!","passwordConfirmation":"Новый пароль 2026 надёжный!"}`))
	if invalidResponse.Code != http.StatusBadRequest || !strings.Contains(invalidResponse.Body.String(), "invalid_or_expired_token") {
		t.Fatalf("invalid status=%d body=%s", invalidResponse.Code, invalidResponse.Body.String())
	}
}

func TestForgotAndResetPasswordErrorResponses(t *testing.T) {
	request := func(path, body string, sameOrigin bool) *http.Request {
		result := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		result.Header.Set("Content-Type", "application/json")
		if sameOrigin {
			result.Header.Set("Origin", "https://designer-svetlana.ru")
		}
		return result
	}
	forgotCases := []struct {
		name       string
		store      *endpointAccountStore
		body       string
		sameOrigin bool
		status     int
	}{
		{"origin", &endpointAccountStore{}, `{"identifier":"sveta@example.com","channel":"email"}`, false, http.StatusForbidden},
		{"malformed", &endpointAccountStore{}, `{}`, true, http.StatusBadRequest},
		{"invalid identifier", &endpointAccountStore{}, `{"identifier":"x","channel":"email"}`, true, http.StatusBadRequest},
		{"limiter unavailable", &endpointAccountStore{rateLimitErr: context.DeadlineExceeded}, `{"identifier":"sveta@example.com","channel":"email"}`, true, http.StatusServiceUnavailable},
		{"internal", &endpointAccountStore{activeEmailErr: errors.New("database unavailable")}, `{"identifier":"sveta@example.com","channel":"email"}`, true, http.StatusInternalServerError},
	}
	for _, test := range forgotCases {
		t.Run("forgot "+test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			accountEndpointForTest(t, test.store).ForgotPassword(response, request("/api/v1/auth/forgot-password", test.body, test.sameOrigin))
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	validBody := `{"token":"` + strings.Repeat("r", 43) + `","password":"Новый пароль 2026 надёжный!","passwordConfirmation":"Новый пароль 2026 надёжный!"}`
	resetAccount := account.PasswordResetAccount{Login: "sveta.design", Email: "sveta@example.com"}
	resetCases := []struct {
		name       string
		store      *endpointAccountStore
		body       string
		sameOrigin bool
		status     int
	}{
		{"origin", &endpointAccountStore{}, validBody, false, http.StatusForbidden},
		{"malformed", &endpointAccountStore{}, `{}`, true, http.StatusBadRequest},
		{"short token", &endpointAccountStore{}, `{"token":"short","password":"Новый пароль 2026 надёжный!","passwordConfirmation":"Новый пароль 2026 надёжный!"}`, true, http.StatusBadRequest},
		{"invalid password", &endpointAccountStore{resetAccount: resetAccount}, `{"token":"` + strings.Repeat("r", 43) + `","password":"passwordpassword","passwordConfirmation":"passwordpassword"}`, true, http.StatusBadRequest},
		{"rate limited", &endpointAccountStore{rateLimitRetry: time.Minute}, validBody, true, http.StatusTooManyRequests},
		{"limiter unavailable", &endpointAccountStore{rateLimitErr: context.DeadlineExceeded}, validBody, true, http.StatusServiceUnavailable},
		{"internal", &endpointAccountStore{resetAccount: resetAccount, resetCompleteErr: errors.New("database unavailable")}, validBody, true, http.StatusInternalServerError},
	}
	for _, test := range resetCases {
		t.Run("reset "+test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			accountEndpointForTest(t, test.store).ResetPassword(response, request("/api/v1/auth/reset-password", test.body, test.sameOrigin))
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAccountEndpointConstructorAndInternalFailure(t *testing.T) {
	if _, err := NewAccountEndpoint(nil); err == nil {
		t.Fatal("nil service must fail")
	}
	endpoint := accountEndpointForTest(t, &endpointAccountStore{createErr: errors.New("database unavailable")})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"login":"sveta.design","email":"sveta@example.com","firstName":"Светлана","professionalRoleCode":"designer","password":"Надёжный пароль 2026!","passwordConfirmation":"Надёжный пароль 2026!"}`))
	request.Header.Set("Content-Type", "application/json")
	endpoint.RegisterAccount(response, request)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestVerifyAndResendErrorBranches(t *testing.T) {
	tests := []struct {
		name   string
		store  *endpointAccountStore
		method string
		body   string
		status int
		code   string
	}{
		{"verify malformed", &endpointAccountStore{}, "verify", `{}`, http.StatusBadRequest, "invalid_or_expired_token"},
		{"verify internal", &endpointAccountStore{consumeErr: errors.New("database unavailable")}, "verify", `{"token":"` + strings.Repeat("a", 43) + `"}`, http.StatusInternalServerError, "internal_error"},
		{"resend invalid", &endpointAccountStore{}, "resend", `{"identifier":"x"}`, http.StatusBadRequest, "validation_error"},
		{"resend internal", &endpointAccountStore{pendingErr: errors.New("database unavailable")}, "resend", `{"identifier":"sveta@example.com"}`, http.StatusInternalServerError, "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := accountEndpointForTest(t, test.store)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/"+test.method, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			if test.method == "verify" {
				endpoint.VerifyEmail(response, request)
			} else {
				endpoint.ResendVerification(response, request)
			}
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestLoginSetsSecureCookiesAndRejectsCrossSiteOrigin(t *testing.T) {
	hash, err := (account.PasswordHasher{}).Hash("Надёжный пароль 2026!", "sveta.design", "sveta@example.com")
	if err != nil {
		t.Fatal(err)
	}
	store := &endpointAccountStore{loginFound: true, loginAccount: account.LoginAccount{AccountView: account.AccountView{ID: "00000000-0000-0000-0000-000000000001", Login: "sveta.design", Email: "sveta@example.com", FirstName: "Светлана", ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер", Status: "active", Version: 1}, PasswordHash: hash.PHC, SecurityVersion: 1}}
	endpoint := accountEndpointForTest(t, store)
	body := `{"identifier":"sveta.design","password":"Надёжный пароль 2026!","rememberMe":true}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://designer-svetlana.ru")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	endpoint.Login(response, request)
	if response.Code != http.StatusOK || len(response.Result().Cookies()) != 3 || !strings.Contains(response.Header().Get("Set-Cookie"), "Secure") || strings.Contains(response.Body.String(), store.loginAccount.PasswordHash) {
		t.Fatalf("status=%d cookies=%v body=%s", response.Code, response.Result().Cookies(), response.Body.String())
	}

	crossSite := httptest.NewRecorder()
	crossRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	crossRequest.Header.Set("Content-Type", "application/json")
	crossRequest.Header.Set("Origin", "https://designer-svetlana.ru.evil.example")
	endpoint.Login(crossSite, crossRequest)
	if crossSite.Code != http.StatusForbidden || !strings.Contains(crossSite.Body.String(), "csrf_rejected") {
		t.Fatalf("status=%d body=%s", crossSite.Code, crossSite.Body.String())
	}
}

func TestLoginDistinguishesUnverifiedAndDisabledAccounts(t *testing.T) {
	hash, err := (account.PasswordHasher{}).Hash("Надёжный пароль 2026!", "sveta.design", "sveta@example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, status, code string
	}{
		{name: "unverified", status: "pending_verification", code: "account_unverified"},
		{name: "disabled", status: "disabled", code: "account_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &endpointAccountStore{loginFound: true, loginAccount: account.LoginAccount{AccountView: account.AccountView{ID: "00000000-0000-0000-0000-000000000001", Login: "sveta.design", Email: "sveta@example.com", FirstName: "Светлана", ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер", Status: test.status, Version: 1}, PasswordHash: hash.PHC, SecurityVersion: 1}}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"sveta.design","password":"Надёжный пароль 2026!"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://designer-svetlana.ru")
			accountEndpointForTest(t, store).Login(response, request)
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRefreshAndLogoutRotateAndClearCookies(t *testing.T) {
	hash, err := (account.PasswordHasher{}).Hash("Надёжный пароль 2026!", "sveta.design", "sveta@example.com")
	if err != nil {
		t.Fatal(err)
	}
	view := account.AccountView{ID: "00000000-0000-0000-0000-000000000001", Login: "sveta.design", Email: "sveta@example.com", FirstName: "Светлана", ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер", Status: "active", Version: 1}
	store := &endpointAccountStore{loginFound: true, loginAccount: account.LoginAccount{AccountView: view, PasswordHash: hash.PHC, SecurityVersion: 1}, rotationAccount: account.LoginAccount{AccountView: view, SecurityVersion: 1, RememberMe: true, RefreshExpiresAt: time.Now().Add(90 * 24 * time.Hour)}}
	endpoint := accountEndpointForTest(t, store)
	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"sveta.design","password":"Надёжный пароль 2026!","rememberMe":true}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("Origin", "https://designer-svetlana.ru")
	endpoint.Login(login, loginRequest)
	cookies := map[string]*http.Cookie{}
	for _, cookie := range login.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
	refreshCookie, csrfCookie := cookies["__Secure-arhdesign_refresh"], cookies["__Host-arhdesign_csrf"]
	if refreshCookie == nil || csrfCookie == nil {
		t.Fatalf("login cookies=%v", cookies)
	}

	refresh := httptest.NewRecorder()
	refreshRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshRequest.Header.Set("Origin", "https://designer-svetlana.ru")
	refreshRequest.Header.Set("X-CSRF-Token", csrfCookie.Value)
	refreshRequest.AddCookie(refreshCookie)
	refreshRequest.AddCookie(csrfCookie)
	endpoint.RefreshSession(refresh, refreshRequest)
	if refresh.Code != http.StatusOK || len(store.rotation.RefreshHash) != 32 {
		t.Fatalf("status=%d body=%s rotation=%#v", refresh.Code, refresh.Body.String(), store.rotation)
	}
	rotatedCookies := map[string]*http.Cookie{}
	for _, cookie := range refresh.Result().Cookies() {
		rotatedCookies[cookie.Name] = cookie
	}

	logout := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.Header.Set("Origin", "https://designer-svetlana.ru")
	logoutRequest.Header.Set("X-CSRF-Token", rotatedCookies["__Host-arhdesign_csrf"].Value)
	logoutRequest.AddCookie(rotatedCookies["__Host-arhdesign_session"])
	logoutRequest.AddCookie(rotatedCookies["__Host-arhdesign_csrf"])
	endpoint.Logout(logout, logoutRequest)
	if logout.Code != http.StatusNoContent || len(store.revocation.AccessHash) != 32 || len(logout.Result().Cookies()) != 3 {
		t.Fatalf("status=%d revocation=%#v cookies=%v", logout.Code, store.revocation, logout.Result().Cookies())
	}

	rejected := httptest.NewRecorder()
	rejectedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	rejectedRequest.Header.Set("Origin", "https://designer-svetlana.ru")
	rejectedRequest.Header.Set("X-CSRF-Token", "wrong")
	rejectedRequest.AddCookie(refreshCookie)
	rejectedRequest.AddCookie(csrfCookie)
	endpoint.RefreshSession(rejected, rejectedRequest)
	if rejected.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rejected.Code, rejected.Body.String())
	}
}

func TestSessionEndpointErrorMapping(t *testing.T) {
	loginRequest := func(body string) *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://designer-svetlana.ru")
		return request
	}
	t.Run("malformed login", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{})
		response := httptest.NewRecorder()
		endpoint.Login(response, loginRequest(`{}`))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", response.Code)
		}
	})
	t.Run("service validation login", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{})
		response := httptest.NewRecorder()
		endpoint.Login(response, loginRequest(`{"identifier":"x","password":"p"}`))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", response.Code)
		}
	})
	t.Run("invalid credentials", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{})
		response := httptest.NewRecorder()
		endpoint.Login(response, loginRequest(`{"identifier":"missing.user","password":"wrong"}`))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})
	t.Run("login store failure", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{loginErr: context.DeadlineExceeded})
		response := httptest.NewRecorder()
		endpoint.Login(response, loginRequest(`{"identifier":"valid.user","password":"password"}`))
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", response.Code)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{rateLimitRetry: 1500 * time.Millisecond})
		response := httptest.NewRecorder()
		endpoint.Login(response, loginRequest(`{"identifier":"valid.user","password":"password"}`))
		if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "2" || !strings.Contains(response.Body.String(), "rate_limited") {
			t.Fatalf("status=%d retry=%q body=%s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
		}
	})
	t.Run("limiter unavailable", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{rateLimitErr: context.DeadlineExceeded})
		response := httptest.NewRecorder()
		endpoint.Login(response, loginRequest(`{"identifier":"valid.user","password":"password"}`))
		if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "service_unavailable") {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})
	t.Run("disabled account", func(t *testing.T) {
		hash, _ := (account.PasswordHasher{}).Hash("Надёжный пароль 2026!", "valid.user", "valid@example.com")
		store := &endpointAccountStore{loginFound: true, loginAccount: account.LoginAccount{AccountView: account.AccountView{Status: "disabled"}, PasswordHash: hash.PHC}}
		endpoint := accountEndpointForTest(t, store)
		response := httptest.NewRecorder()
		endpoint.Login(response, loginRequest(`{"identifier":"valid.user","password":"Надёжный пароль 2026!"}`))
		if response.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})
	t.Run("refresh missing cookie", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{})
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		request.Header.Set("Origin", "https://designer-svetlana.ru")
		request.Header.Set("X-CSRF-Token", strings.Repeat("c", 43))
		request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: strings.Repeat("c", 43)})
		endpoint.RefreshSession(response, request)
		if response.Code != http.StatusUnauthorized || len(response.Result().Cookies()) != 3 {
			t.Fatalf("status=%d cookies=%v", response.Code, response.Result().Cookies())
		}
	})
	t.Run("refresh store failure", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{rotationErr: context.DeadlineExceeded})
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		request.Header.Set("Origin", "https://designer-svetlana.ru")
		request.Header.Set("X-CSRF-Token", strings.Repeat("c", 43))
		request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: strings.Repeat("c", 43)})
		request.AddCookie(&http.Cookie{Name: "__Secure-arhdesign_refresh", Value: strings.Repeat("r", 43)})
		endpoint.RefreshSession(response, request)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", response.Code)
		}
	})
	t.Run("refresh unauthenticated", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{rotationErr: account.ErrUnauthenticated})
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		request.Header.Set("Origin", "https://designer-svetlana.ru")
		request.Header.Set("X-CSRF-Token", strings.Repeat("c", 43))
		request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: strings.Repeat("c", 43)})
		request.AddCookie(&http.Cookie{Name: "__Secure-arhdesign_refresh", Value: strings.Repeat("r", 43)})
		endpoint.RefreshSession(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", response.Code)
		}
	})
	t.Run("logout store failure", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{revocationErr: context.DeadlineExceeded})
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		request.Header.Set("Origin", "https://designer-svetlana.ru")
		request.Header.Set("X-CSRF-Token", strings.Repeat("c", 43))
		request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: strings.Repeat("c", 43)})
		request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_session", Value: strings.Repeat("a", 43)})
		request.AddCookie(&http.Cookie{Name: "__Secure-arhdesign_refresh", Value: strings.Repeat("r", 43)})
		endpoint.Logout(response, request)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", response.Code)
		}
	})
	t.Run("logout unauthenticated", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{revocationErr: account.ErrUnauthenticated})
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		request.Header.Set("Origin", "https://designer-svetlana.ru")
		request.Header.Set("X-CSRF-Token", strings.Repeat("c", 43))
		request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: strings.Repeat("c", 43)})
		request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_session", Value: strings.Repeat("a", 43)})
		endpoint.Logout(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", response.Code)
		}
	})
	t.Run("referer fallback", func(t *testing.T) {
		endpoint := accountEndpointForTest(t, &endpointAccountStore{})
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.Header.Set("Referer", "https://designer-svetlana.ru/login")
		if !endpoint.validBrowserMutation(request) {
			t.Fatal("same-origin referer must be accepted")
		}
	})
}

func TestAccountEndpointRejectsMalformedJSONAcrossPublicMutations(t *testing.T) {
	endpoint := accountEndpointForTest(t, &endpointAccountStore{})
	request := func(path string) *http.Request {
		result := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{`))
		result.Header.Set("Content-Type", "application/json")
		result.Header.Set("Origin", "https://designer-svetlana.ru")
		return result
	}
	tests := []struct {
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"/api/v1/auth/login", endpoint.Login},
		{"/api/v1/auth/verification-channel", endpoint.SelectVerificationChannel},
		{"/api/v1/auth/resend-verification", endpoint.ResendVerification},
		{"/api/v1/auth/forgot-password", endpoint.ForgotPassword},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		test.call(response, request(test.path))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}

	registration := `{"login":"sveta.design","email":"sveta@example.com","firstName":"Светлана","professionalRoleCode":"designer","password":"Надёжный пароль 2026!","passwordConfirmation":"Надёжный пароль 2026!"}`
	registrationResponse := httptest.NewRecorder()
	accountEndpointForTest(t, &endpointAccountStore{createErr: account.ErrInvalidInput}).RegisterAccount(registrationResponse, func() *http.Request {
		result := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(registration))
		result.Header.Set("Content-Type", "application/json")
		result.Header.Set("Origin", "https://designer-svetlana.ru")
		return result
	}())
	if registrationResponse.Code != http.StatusBadRequest {
		t.Fatalf("registration status=%d body=%s", registrationResponse.Code, registrationResponse.Body.String())
	}

	retryResponse := httptest.NewRecorder()
	setRetryAfter(retryResponse, -time.Second)
	if retryResponse.Header().Get("Retry-After") != "1" {
		t.Fatalf("retry-after=%q", retryResponse.Header().Get("Retry-After"))
	}
}

func TestAccountBrowserBoundaryHelpers(t *testing.T) {
	endpoint := accountEndpointForTest(t, &endpointAccountStore{})
	invalidReferer := httptest.NewRequest(http.MethodPost, "/", nil)
	invalidReferer.Header.Set("Referer", "not a url")
	if endpoint.validBrowserMutation(invalidReferer) {
		t.Fatal("invalid referer must fail")
	}
	crossFetch := httptest.NewRequest(http.MethodPost, "/", nil)
	crossFetch.Header.Set("Origin", "https://designer-svetlana.ru")
	crossFetch.Header.Set("Sec-Fetch-Site", "cross-site")
	if endpoint.validBrowserMutation(crossFetch) {
		t.Fatal("cross-site fetch metadata must fail")
	}
	missingCSRF := httptest.NewRequest(http.MethodPost, "/", nil)
	missingCSRF.Header.Set("Origin", "https://designer-svetlana.ru")
	if _, ok := endpoint.validAuthenticatedMutation(missingCSRF); ok {
		t.Fatal("missing CSRF cookie must fail")
	}
	logoutRejected := httptest.NewRecorder()
	endpoint.Logout(logoutRejected, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if logoutRejected.Code != http.StatusForbidden {
		t.Fatalf("logout status=%d", logoutRejected.Code)
	}
	if maxAge(time.Now(), time.Now().Add(-time.Second)) != 1 {
		t.Fatal("expired cookie max-age must clamp")
	}
	role := "super_admin"
	response := sessionResponse(account.Session{Account: account.AccountView{ID: "00000000-0000-0000-0000-000000000001", Email: "admin@example.com", Status: "active", GlobalRole: &role, Version: 1}})
	if response.Account.GlobalRole == nil || *response.Account.GlobalRole != "super_admin" {
		t.Fatalf("response=%#v", response)
	}

	tokens, _ := account.NewTokenManager(bytes.Repeat([]byte{1}, 32), 1)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	service, err := account.NewService(&endpointAccountStore{}, tokens, cipher, "http://localhost", "from@example.com", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	local, err := NewAccountEndpoint(service)
	if err != nil {
		t.Fatal(err)
	}
	if local.cookieName("session") != "arhdesign_session" || local.cookieName("refresh") != "arhdesign_refresh" {
		t.Fatal("local cookies must not use secure prefixes")
	}
	markerRequest := httptest.NewRequest(http.MethodPost, "/", nil)
	markerRequest.RemoteAddr = "192.0.2.10:4321"
	if marker := accountAnonymousMarker(markerRequest); marker != "192.0.2.10" {
		t.Fatalf("marker=%q", marker)
	}
	markerRequest.Header.Set("X-Arhdesign-Client-IP", "198.51.100.12")
	if marker := accountAnonymousMarker(markerRequest); marker != "198.51.100.12" {
		t.Fatalf("forwarded marker=%q", marker)
	}
	markerRequest.Header.Del("X-Arhdesign-Client-IP")
	markerRequest.RemoteAddr = "203.0.113.7"
	if marker := accountAnonymousMarker(markerRequest); marker != "203.0.113.7" {
		t.Fatalf("direct marker=%q", marker)
	}
	markerRequest.RemoteAddr = "not-an-address"
	if marker := accountAnonymousMarker(markerRequest); marker != "unknown" {
		t.Fatalf("unknown marker=%q", marker)
	}
}
