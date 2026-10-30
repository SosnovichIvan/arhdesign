package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type limitedEntropyReader struct{ remaining int }

func (reader *limitedEntropyReader) Read(buffer []byte) (int, error) {
	if reader.remaining <= 0 {
		return 0, io.ErrUnexpectedEOF
	}
	count := len(buffer)
	if count > reader.remaining {
		count = reader.remaining
	}
	for index := 0; index < count; index++ {
		buffer[index] = byte(index + 1)
	}
	reader.remaining -= count
	return count, nil
}

type fakeAccountStore struct {
	pending           PendingAccount
	createErr         error
	consumeHash       []byte
	consumeErr        error
	pendingEmail      string
	pendingFound      bool
	pendingErr        error
	replacement       ReplacementVerification
	replaceCalled     bool
	replaceErr        error
	channelSelection  VerificationChannelSelection
	channelFound      bool
	channelErr        error
	loginAccount      LoginAccount
	loginFound        bool
	loginErr          error
	accessAccount     AccountView
	accessFound       bool
	accessErr         error
	accessHash        []byte
	newSession        NewSession
	createSessionErr  error
	bootstrap         BootstrapSession
	bootstrapAccount  LoginAccount
	bootstrapOK       bool
	bootstrapErr      error
	rotation          SessionRotation
	rotationAccount   LoginAccount
	rotationErr       error
	revocation        SessionRevocation
	revocationErr     error
	loginFailure      string
	loginFailureErr   error
	rateLimitRequests []RateLimitRequest
	rateLimitRetry    time.Duration
	rateLimitErr      error
	activeEmail       string
	activeFound       bool
	activeEmailErr    error
	resetIssue        PasswordResetIssue
	resetIssueErr     error
	resetAccount      PasswordResetAccount
	resetContextErr   error
	resetCompletion   PasswordResetCompletion
	resetCompleteErr  error
	resetRequestAudit string
	resetFailureAudit bool
	resetAuditErr     error
	telegramStart     TelegramAuthStart
	telegramAdvance   TelegramAuthAdvance
	telegramStep      string
	telegramAccount   LoginAccount
	telegramFound     bool
	telegramAuthErr   error
	telegramComplete  TelegramAuthCompletion
	telegramReject    TelegramAuthRejection
	adminUsers        []AdminUserView
	adminUsersErr     error
	adminQuery        AdminUserQuery
	adminStatus       AdminUserView
	adminStatusErr    error
	adminCommand      AdminUserStatusCommand
}

func (store *fakeAccountStore) ListAdminUsers(_ context.Context, query AdminUserQuery) ([]AdminUserView, error) {
	store.adminQuery = query
	return store.adminUsers, store.adminUsersErr
}
func (store *fakeAccountStore) SetAdminUserStatus(_ context.Context, command AdminUserStatusCommand) (AdminUserView, error) {
	store.adminCommand = command
	return store.adminStatus, store.adminStatusErr
}

func (store *fakeAccountStore) FindLoginAccount(context.Context, string) (LoginAccount, bool, error) {
	return store.loginAccount, store.loginFound, store.loginErr
}
func (store *fakeAccountStore) ResolveAccessSession(_ context.Context, hash []byte, _ time.Time) (AccountView, bool, error) {
	store.accessHash = append([]byte(nil), hash...)
	return store.accessAccount, store.accessFound, store.accessErr
}
func (store *fakeAccountStore) CreateSession(_ context.Context, session NewSession) error {
	store.newSession = session
	return store.createSessionErr
}
func (store *fakeAccountStore) BootstrapSuperAdminSession(_ context.Context, bootstrap BootstrapSession) (LoginAccount, bool, error) {
	store.bootstrap = bootstrap
	return store.bootstrapAccount, store.bootstrapOK, store.bootstrapErr
}

func (store *fakeAccountStore) EnsureTechnicalAdmin(_ context.Context, bootstrap TechnicalAdminBootstrap) (AccountView, bool, error) {
	role := "technical_admin"
	return AccountView{ID: "00000000-0000-0000-0000-000000000099", Login: bootstrap.Login, Email: bootstrap.Email, FirstName: bootstrap.FirstName, ProfessionalRoleCode: bootstrap.ProfessionalRoleCode, ProfessionalRoleName: "Дизайнер", Status: "active", GlobalRole: &role, Version: 1}, true, nil
}
func (store *fakeAccountStore) RotateSession(_ context.Context, rotation SessionRotation) (LoginAccount, error) {
	store.rotation = rotation
	return store.rotationAccount, store.rotationErr
}
func (store *fakeAccountStore) RevokeSessionFamily(_ context.Context, revocation SessionRevocation) error {
	store.revocation = revocation
	return store.revocationErr
}
func (store *fakeAccountStore) RecordLoginFailure(_ context.Context, result, _ string) error {
	store.loginFailure = result
	return store.loginFailureErr
}
func (store *fakeAccountStore) ConsumeRateLimit(_ context.Context, request RateLimitRequest) (time.Duration, error) {
	store.rateLimitRequests = append(store.rateLimitRequests, request)
	return store.rateLimitRetry, store.rateLimitErr
}
func (store *fakeAccountStore) ActiveAccountEmail(context.Context, string) (string, bool, error) {
	return store.activeEmail, store.activeFound, store.activeEmailErr
}
func (store *fakeAccountStore) CreatePasswordReset(_ context.Context, issue PasswordResetIssue) (bool, error) {
	store.resetIssue = issue
	return store.activeFound, store.resetIssueErr
}
func (store *fakeAccountStore) CreateTelegramPasswordReset(_ context.Context, issue PasswordResetIssue) (bool, error) {
	store.resetIssue = issue
	return store.activeFound, store.resetIssueErr
}
func (store *fakeAccountStore) PasswordResetContext(context.Context, []byte, int, time.Time, string) (PasswordResetAccount, error) {
	return store.resetAccount, store.resetContextErr
}
func (store *fakeAccountStore) CompletePasswordReset(_ context.Context, completion PasswordResetCompletion) error {
	store.resetCompletion = completion
	return store.resetCompleteErr
}
func (store *fakeAccountStore) RecordPasswordResetRequest(_ context.Context, result, _ string) error {
	store.resetRequestAudit = result
	return store.resetAuditErr
}
func (store *fakeAccountStore) RecordPasswordResetFailure(context.Context, string) error {
	store.resetFailureAudit = true
	return store.resetAuditErr
}
func (store *fakeAccountStore) BeginTelegramAuth(_ context.Context, start TelegramAuthStart) error {
	store.telegramStart = start
	return store.telegramAuthErr
}
func (store *fakeAccountStore) AdvanceTelegramAuth(_ context.Context, advance TelegramAuthAdvance) error {
	store.telegramAdvance = advance
	return store.telegramAuthErr
}
func (store *fakeAccountStore) TelegramAuthStep(context.Context, int64, time.Time) (string, error) {
	return store.telegramStep, store.telegramAuthErr
}
func (store *fakeAccountStore) TelegramAuthAccount(context.Context, int64, time.Time) (LoginAccount, bool, error) {
	return store.telegramAccount, store.telegramFound, store.telegramAuthErr
}
func (store *fakeAccountStore) CompleteTelegramAuth(_ context.Context, completion TelegramAuthCompletion) error {
	store.telegramComplete = completion
	return store.telegramAuthErr
}
func (store *fakeAccountStore) RejectTelegramAuth(_ context.Context, rejection TelegramAuthRejection) error {
	store.telegramReject = rejection
	return nil
}

func (store *fakeAccountStore) CreatePendingAccount(_ context.Context, pending PendingAccount) error {
	store.pending = pending
	return store.createErr
}
func (store *fakeAccountStore) ConsumeVerificationToken(_ context.Context, hash []byte, _ int, _ time.Time, _ string) error {
	store.consumeHash = hash
	return store.consumeErr
}
func (store *fakeAccountStore) PendingAccountEmail(context.Context, string) (string, bool, error) {
	return store.pendingEmail, store.pendingFound, store.pendingErr
}
func (store *fakeAccountStore) ReplaceVerificationToken(_ context.Context, replacement ReplacementVerification) (bool, error) {
	store.replaceCalled, store.replacement = true, replacement
	return store.replaceErr == nil, store.replaceErr
}
func (store *fakeAccountStore) SetVerificationChannel(_ context.Context, selection VerificationChannelSelection) (bool, error) {
	store.channelSelection = selection
	return store.channelFound, store.channelErr
}

func newAccountServiceForTest(t *testing.T, store Store) (*Service, PayloadCipher, time.Time) {
	t.Helper()
	tokens, err := NewTokenManager(bytes.Repeat([]byte{1}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	service, err := NewService(store, tokens, cipher, "https://designer-svetlana.ru", "cabinet@example.com", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return service, cipher, now
}

func TestTechnicalAdministratorAllowsDocumentedFourteenCharacterPassword(t *testing.T) {
	service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{})
	view, created, err := service.EnsureTechnicalAdmin(context.Background(), "isosnovich", "isosnovich@yandex.ru", "Semen15052018!")
	if err != nil || !created || view.Login != "isosnovich" {
		t.Fatalf("view=%#v created=%v error=%v", view, created, err)
	}
	if _, _, err = service.EnsureTechnicalAdmin(context.Background(), "isosnovich", "isosnovich@yandex.ru", "shorter-pass!"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("short technical administrator password error=%v", err)
	}
}

func TestAuthenticateResolvesOnlyValidAccessSession(t *testing.T) {
	validToken := strings.Repeat("a", 43)
	view := AccountView{ID: "00000000-0000-0000-0000-000000000001", Login: "svetlana", Status: "active"}
	store := &fakeAccountStore{accessAccount: view, accessFound: true}
	service, _, _ := newAccountServiceForTest(t, store)

	resolved, err := service.Authenticate(context.Background(), validToken)
	if err != nil || resolved.ID != view.ID || len(store.accessHash) != 32 {
		t.Fatalf("resolved=%#v hash=%x error=%v", resolved, store.accessHash, err)
	}
	if _, err = service.Authenticate(context.Background(), "short"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("short token error=%v", err)
	}
	store.accessFound = false
	if _, err = service.Authenticate(context.Background(), validToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("missing session error=%v", err)
	}
	store.accessErr = context.DeadlineExceeded
	if _, err = service.Authenticate(context.Background(), validToken); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("store error=%v", err)
	}
}

func validRegistration() Registration {
	return Registration{Login: "Sveta.Design", Email: "Sveta@example.com", FirstName: " Светлана ", LastName: " Полисмакова ", ProfessionalRoleCode: "Designer", Password: "Надёжный пароль 2026!", PasswordConfirmation: "Надёжный пароль 2026!", RequestID: "request-123"}
}

func TestRegisterBuildsAtomicPendingAccountWithoutSelectingDelivery(t *testing.T) {
	store := &fakeAccountStore{}
	service, _, now := newAccountServiceForTest(t, store)
	resendAt, err := service.Register(context.Background(), validRegistration())
	if err != nil || !resendAt.Equal(now) {
		t.Fatalf("resendAt = %v, error = %v", resendAt, err)
	}
	pending := store.pending
	if pending.Login != "Sveta.Design" || pending.LoginNormalized != "sveta.design" || pending.Email != "sveta@example.com" || pending.FirstName != "Светлана" || pending.ProfessionalRoleCode != "designer" {
		t.Fatalf("unexpected normalized account: %#v", pending)
	}
	if strings.Contains(pending.PasswordHash, validRegistration().Password) || len(pending.TokenHash) != 0 || len(pending.OutboxCiphertext) != 0 {
		t.Fatal("registration must store only a password hash and wait for explicit channel selection")
	}
}

func TestSelectVerificationChannelBuildsEmailOrTelegramHandoff(t *testing.T) {
	store := &fakeAccountStore{pendingEmail: "sveta@example.com", pendingFound: true, channelFound: true}
	service, cipher, now := newAccountServiceForTest(t, store)
	service.ConfigureTelegramBot("@polismakovaSvetlanaBot")

	if handoff, err := service.SelectVerificationChannel(context.Background(), " Sveta.Design ", "email", "request-channel-email"); err != nil || handoff != "" {
		t.Fatalf("email handoff=%q error=%v", handoff, err)
	}
	selection := store.channelSelection
	if selection.IdentifierNormalized != "sveta.design" || selection.Channel != "email" || len(selection.TokenHash) != 32 || !selection.TokenExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("email selection=%#v", selection)
	}
	payload, err := cipher.Decrypt(selection.OutboxCiphertext, verificationMessageType, selection.OutboxKeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	var email VerificationEmail
	if err = json.Unmarshal(payload, &email); err != nil || email.To != "sveta@example.com" || !strings.Contains(email.Text, "/verify-email#token=") {
		t.Fatalf("email=%#v error=%v", email, err)
	}

	handoff, err := service.SelectVerificationChannel(context.Background(), "sveta.design", "telegram", "request-channel-telegram")
	if err != nil || handoff != "https://t.me/polismakovaSvetlanaBot?start=register" {
		t.Fatalf("telegram handoff=%q error=%v", handoff, err)
	}
	if store.channelSelection.Channel != "telegram" || len(store.channelSelection.TokenHash) != 0 || len(store.channelSelection.OutboxCiphertext) != 0 {
		t.Fatalf("Telegram selection contains email delivery data: %#v", store.channelSelection)
	}
}

func TestTelegramConfirmationKeepsCredentialsOutOfPersistentState(t *testing.T) {
	password := "Надёжный пароль для Telegram 2026!"
	hash, err := (PasswordHasher{}).Hash(password, "sveta.design", "sveta@example.com")
	if err != nil {
		t.Fatal(err)
	}
	accountRecord := LoginAccount{AccountView: AccountView{ID: "user-telegram", Login: "sveta.design", Status: "pending_verification"}, PasswordHash: hash.PHC}
	store := &fakeAccountStore{loginAccount: accountRecord, loginFound: true, telegramAccount: accountRecord, telegramFound: true}
	service, _, now := newAccountServiceForTest(t, store)

	if err := service.BeginTelegramConfirmation(context.Background(), 701, "telegram-request-start"); err != nil {
		t.Fatal(err)
	}
	if store.telegramStart.ChatID != 701 || store.telegramStart.Flow != "registration_confirmation" || !store.telegramStart.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("start=%#v", store.telegramStart)
	}
	if err := service.SubmitTelegramLogin(context.Background(), 701, " Sveta.Design "); err != nil {
		t.Fatal(err)
	}
	if store.telegramAdvance.CandidateUserID == nil || *store.telegramAdvance.CandidateUserID != "user-telegram" {
		t.Fatalf("advance=%#v", store.telegramAdvance)
	}
	if err := service.CompleteTelegramConfirmation(context.Background(), 701, 33, "sveta", password, "telegram-request-finish"); err != nil {
		t.Fatal(err)
	}
	if store.telegramComplete.UserID != "user-telegram" || store.telegramComplete.ChatID != 701 || store.telegramComplete.ChatUsername != "sveta" {
		t.Fatalf("completion=%#v", store.telegramComplete)
	}
	if strings.Contains(store.telegramComplete.RequestID, password) {
		t.Fatal("password leaked into completion state")
	}
}

func TestTelegramConfirmationUsesNeutralFailureAndConsumesState(t *testing.T) {
	store := &fakeAccountStore{telegramFound: false}
	service, _, _ := newAccountServiceForTest(t, store)
	if err := service.CompleteTelegramConfirmation(context.Background(), 702, 34, "", "wrong", "telegram-request-failed"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("error=%v", err)
	}
	if store.telegramReject.ChatID != 702 || store.telegramReject.Reason != "invalid_credentials" {
		t.Fatalf("rejection=%#v", store.telegramReject)
	}
}

func TestTelegramConfirmationValidationAndDependencyFailures(t *testing.T) {
	service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{})
	if _, err := service.SelectVerificationChannel(context.Background(), "x", "telegram", "request-valid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid channel selection error=%v", err)
	}
	if _, err := service.SelectVerificationChannel(context.Background(), "sveta.design", "sms", "request-valid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unsupported channel error=%v", err)
	}
	if _, err := service.SelectVerificationChannel(context.Background(), "sveta.design", "telegram", "request-valid"); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("unconfigured bot error=%v", err)
	}

	pendingFailure := &fakeAccountStore{pendingErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, pendingFailure)
	if _, err := service.SelectVerificationChannel(context.Background(), "sveta.design", "email", "request-valid"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending lookup error=%v", err)
	}
	channelFailure := &fakeAccountStore{channelErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, channelFailure)
	service.ConfigureTelegramBot("polismakovaSvetlanaBot")
	if _, err := service.SelectVerificationChannel(context.Background(), "sveta.design", "telegram", "request-valid"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("channel persistence error=%v", err)
	}

	if err := service.BeginTelegramConfirmation(context.Background(), 0, "request-valid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid begin error=%v", err)
	}
	if err := service.SubmitTelegramLogin(context.Background(), 0, "sveta.design"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid login step error=%v", err)
	}
	if _, err := service.TelegramConfirmationStep(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid step error=%v", err)
	}
	if err := service.CompleteTelegramConfirmation(context.Background(), 0, 0, "", "", "request-valid"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("invalid completion error=%v", err)
	}

	lookupFailure := &fakeAccountStore{loginErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, lookupFailure)
	if err := service.SubmitTelegramLogin(context.Background(), 707, "sveta.design"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("login lookup error=%v", err)
	}
	missing := &fakeAccountStore{}
	service, _, _ = newAccountServiceForTest(t, missing)
	if err := service.SubmitTelegramLogin(context.Background(), 708, "missing.user"); err != nil || missing.telegramAdvance.CandidateUserID != nil {
		t.Fatalf("missing candidate=%v error=%v", missing.telegramAdvance.CandidateUserID, err)
	}

	limited := &fakeAccountStore{rateLimitRetry: time.Minute}
	service, _, _ = newAccountServiceForTest(t, limited)
	if err := service.CompleteTelegramConfirmation(context.Background(), 709, 1, "", "password", "request-valid"); !errors.As(err, &RateLimitError{}) {
		t.Fatalf("rate limit error=%v", err)
	}
	limiterFailure := &fakeAccountStore{rateLimitErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, limiterFailure)
	if err := service.CompleteTelegramConfirmation(context.Background(), 710, 1, "", "password", "request-valid"); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("limiter failure=%v", err)
	}
	authFailure := &fakeAccountStore{telegramAuthErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, authFailure)
	if err := service.BeginTelegramConfirmation(context.Background(), 711, "request-valid"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("begin dependency error=%v", err)
	}
	if _, err := service.TelegramConfirmationStep(context.Background(), 711); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("step dependency error=%v", err)
	}
	if err := service.CompleteTelegramConfirmation(context.Background(), 711, 1, "", "password", "request-valid"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("auth account dependency error=%v", err)
	}
}

func TestRegisterValidationAndStoreErrors(t *testing.T) {
	for name, mutate := range map[string]func(*Registration){
		"login":        func(request *Registration) { request.Login = "?" },
		"email":        func(request *Registration) { request.Email = "invalid" },
		"first name":   func(request *Registration) { request.FirstName = "" },
		"last name":    func(request *Registration) { request.LastName = strings.Repeat("л", 101) },
		"middle name":  func(request *Registration) { request.MiddleName = strings.Repeat("о", 101) },
		"role":         func(request *Registration) { request.ProfessionalRoleCode = "?" },
		"confirmation": func(request *Registration) { request.PasswordConfirmation = "different password!" },
	} {
		t.Run(name, func(t *testing.T) {
			service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{})
			request := validRegistration()
			mutate(&request)
			if _, err := service.Register(context.Background(), request); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want invalid input", err)
			} else {
				var validation RegistrationValidationError
				if !errors.As(err, &validation) || len(validation.Fields) == 0 {
					t.Fatalf("error = %#v, want field validation details", err)
				}
			}
		})
	}
	validation := RegistrationValidationError{Cause: ErrInvalidInput, Fields: map[string]string{"login": "invalid"}}
	if validation.Error() != "invalid registration fields" || !errors.Is(validation, ErrInvalidInput) {
		t.Fatalf("validation error=%v", validation)
	}
	invalidRequestID := validRegistration()
	invalidRequestID.RequestID = "short"
	service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{})
	if _, err := service.Register(context.Background(), invalidRequestID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("request id error=%v", err)
	}
	store := &fakeAccountStore{createErr: ErrIdentifierUnavailable}
	service, _, _ = newAccountServiceForTest(t, store)
	if _, err := service.Register(context.Background(), validRegistration()); !errors.Is(err, ErrIdentifierUnavailable) {
		t.Fatalf("error = %v, want identifier unavailable", err)
	}
}

func TestVerifyAndResend(t *testing.T) {
	store := &fakeAccountStore{pendingEmail: "sveta@example.com", pendingFound: true}
	service, _, _ := newAccountServiceForTest(t, store)
	if err := service.Verify(context.Background(), strings.Repeat("a", 43), "request-123"); err != nil || len(store.consumeHash) != 32 {
		t.Fatalf("verify error = %v, hash length = %d", err, len(store.consumeHash))
	}
	if err := service.Verify(context.Background(), "short", "request-123"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("short token error = %v", err)
	}
	if err := service.Resend(context.Background(), " Sveta.Design ", "request-123"); err != nil || !store.replaceCalled || store.replacement.IdentifierNormalized != "sveta.design" {
		t.Fatalf("replacement = %#v, called = %v, error = %v", store.replacement, store.replaceCalled, err)
	}
	absent := &fakeAccountStore{}
	service, _, _ = newAccountServiceForTest(t, absent)
	if err := service.Resend(context.Background(), "absent@example.com", "request-123"); err != nil || absent.replaceCalled {
		t.Fatalf("absent resend called = %v, error = %v", absent.replaceCalled, err)
	}
}

func TestForgotAndResetPassword(t *testing.T) {
	store := &fakeAccountStore{activeEmail: "sveta@example.com", activeFound: true, resetAccount: PasswordResetAccount{Login: "sveta.design", Email: "sveta@example.com"}}
	service, cipher, now := newAccountServiceForTest(t, store)
	if err := service.ForgotPassword(context.Background(), " SVETA@EXAMPLE.COM ", "email", "192.0.2.4", "request-forgot"); err != nil {
		t.Fatal(err)
	}
	if store.resetIssue.IdentifierNormalized != "sveta@example.com" || !store.resetIssue.TokenExpiresAt.Equal(now.Add(30*time.Minute)) || len(store.resetIssue.TokenHash) != 32 {
		t.Fatalf("reset issue=%#v", store.resetIssue)
	}
	payload, err := cipher.Decrypt(store.resetIssue.OutboxCiphertext, passwordResetMessageType, store.resetIssue.OutboxKeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	var email VerificationEmail
	if err = json.Unmarshal(payload, &email); err != nil || email.To != "sveta@example.com" || !strings.Contains(email.Text, "/reset-password#token=") || !strings.Contains(email.Text, "30 минут") {
		t.Fatalf("email=%#v error=%v", email, err)
	}
	if len(store.rateLimitRequests) != 2 || store.rateLimitRequests[0].PolicyCode != "forgot.marker" || store.rateLimitRequests[1].PolicyCode != "forgot.account" {
		t.Fatalf("limits=%#v", store.rateLimitRequests)
	}
	store.rateLimitRequests = nil
	newPassword := "Новый пароль 2026 надёжный!"
	if err = service.ResetPassword(context.Background(), strings.Repeat("r", 43), newPassword, newPassword, "192.0.2.4", "request-reset"); err != nil {
		t.Fatal(err)
	}
	if store.resetCompletion.PasswordHash == "" || store.resetCompletion.PasswordHash == newPassword || !store.resetCompletion.Now.Equal(now) || len(store.resetCompletion.TokenHash) != 32 {
		t.Fatalf("completion=%#v", store.resetCompletion)
	}
	if len(store.rateLimitRequests) != 1 || store.rateLimitRequests[0].PolicyCode != "reset.marker" {
		t.Fatalf("reset limits=%#v", store.rateLimitRequests)
	}

	absent := &fakeAccountStore{}
	absentService, _, _ := newAccountServiceForTest(t, absent)
	if err = absentService.ForgotPassword(context.Background(), "absent@example.com", "email", "", "request-absent"); err != nil || absent.resetIssue.IdentifierNormalized != "absent@example.com" {
		t.Fatalf("absent issue=%#v error=%v", absent.resetIssue, err)
	}
}

func TestForgotPasswordThroughLinkedTelegram(t *testing.T) {
	store := &fakeAccountStore{activeFound: true}
	service, cipher, _ := newAccountServiceForTest(t, store)
	if err := service.ForgotPassword(context.Background(), " SVETA.DESIGN ", "telegram", "192.0.2.8", "request-forgot-telegram"); err != nil {
		t.Fatal(err)
	}
	if store.resetIssue.IdentifierNormalized != "sveta.design" || len(store.resetIssue.TokenHash) != 32 {
		t.Fatalf("reset issue=%#v", store.resetIssue)
	}
	payload, err := cipher.Decrypt(store.resetIssue.OutboxCiphertext, passwordResetMessageType, store.resetIssue.OutboxKeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	var message TelegramAuthMessage
	if err = json.Unmarshal(payload, &message); err != nil || !strings.Contains(message.Text, "/reset-password#token=") || !strings.Contains(message.Text, "30 минут") {
		t.Fatalf("message=%#v error=%v", message, err)
	}
}

func TestForgotAndResetPasswordErrors(t *testing.T) {
	if (RateLimitError{RetryAfter: time.Minute}).Error() != "rate limited" {
		t.Fatal("unexpected rate-limit error text")
	}
	service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{})
	if err := service.ForgotPassword(context.Background(), "x", "email", "marker", "request-invalid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid forgot error=%v", err)
	}
	if err := service.ResetPassword(context.Background(), "short", "password", "password", "marker", "request-invalid"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("invalid token error=%v", err)
	}
	if err := service.ResetPassword(context.Background(), strings.Repeat("r", 43), "Новый пароль 2026 надёжный!", "different password", "marker", "request-invalid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("confirmation error=%v", err)
	}
	limited := &fakeAccountStore{rateLimitRetry: 90 * time.Second}
	service, _, _ = newAccountServiceForTest(t, limited)
	if err := service.ForgotPassword(context.Background(), "user@example.com", "email", "marker", "request-limited"); !errors.As(err, &RateLimitError{}) {
		t.Fatalf("forgot limit error=%v", err)
	}
	if err := service.ResetPassword(context.Background(), strings.Repeat("r", 43), "Новый пароль 2026 надёжный!", "Новый пароль 2026 надёжный!", "marker", "request-limited"); !errors.As(err, &RateLimitError{}) {
		t.Fatalf("reset limit error=%v", err)
	}
	unavailable := &fakeAccountStore{rateLimitErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, unavailable)
	if err := service.ForgotPassword(context.Background(), "user@example.com", "email", "marker", "request-unavailable"); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("forgot unavailable error=%v", err)
	}
	if err := service.ResetPassword(context.Background(), strings.Repeat("r", 43), "Новый пароль 2026 надёжный!", "Новый пароль 2026 надёжный!", "marker", "request-unavailable"); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("reset unavailable error=%v", err)
	}
	issueFailure := &fakeAccountStore{activeEmail: "user@example.com", activeFound: true, resetIssueErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, issueFailure)
	if err := service.ForgotPassword(context.Background(), "user@example.com", "email", "marker", "request-issue-fail"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("issue error=%v", err)
	}
	auditFailure := &fakeAccountStore{rateLimitRetry: time.Minute, resetAuditErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, auditFailure)
	if err := service.ForgotPassword(context.Background(), "user@example.com", "email", "marker", "request-audit-fail"); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("forgot audit error=%v", err)
	}
	invalidAuditFailure := &fakeAccountStore{resetAuditErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, invalidAuditFailure)
	if err := service.ResetPassword(context.Background(), "short", "Новый пароль 2026 надёжный!", "Новый пароль 2026 надёжный!", "marker", "request-audit-fail"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reset audit error=%v", err)
	}
}

func TestServiceConstructorAndResendFailures(t *testing.T) {
	tokens, _ := NewTokenManager(bytes.Repeat([]byte{1}, 32), 1)
	cipher, _ := NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	if _, err := NewService(nil, tokens, cipher, "https://designer-svetlana.ru", "from@example.com", nil); err == nil {
		t.Fatal("nil store must fail")
	}
	if _, err := NewService(&fakeAccountStore{}, tokens, cipher, "", "from@example.com", nil); err == nil {
		t.Fatal("empty public origin must fail")
	}
	if _, err := NewService(&fakeAccountStore{}, tokens, cipher, "https://designer-svetlana.ru", "", nil); err == nil {
		t.Fatal("empty sender must fail")
	}
	defaultClockService, err := NewService(&fakeAccountStore{}, tokens, cipher, "https://designer-svetlana.ru/", "from@example.com", nil)
	if err != nil || defaultClockService.PublicOrigin() != "https://designer-svetlana.ru" || defaultClockService.now == nil {
		t.Fatalf("default clock service=%#v error=%v", defaultClockService, err)
	}
	absentChannelStore := &fakeAccountStore{}
	absentChannelService, _, _ := newAccountServiceForTest(t, absentChannelStore)
	if _, err := absentChannelService.SelectVerificationChannel(context.Background(), "absent@example.com", "email", "request-absent-channel"); err != nil || absentChannelStore.channelSelection.OutboxIdempotencyKey == "" {
		t.Fatalf("absent channel selection=%#v error=%v", absentChannelStore.channelSelection, err)
	}
	store := &fakeAccountStore{pendingErr: context.DeadlineExceeded}
	service, _, _ := newAccountServiceForTest(t, store)
	if err := service.Resend(context.Background(), "sveta@example.com", "request-123"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending lookup error = %v", err)
	}
	store = &fakeAccountStore{pendingEmail: "sveta@example.com", pendingFound: true, replaceErr: context.DeadlineExceeded}
	service, _, _ = newAccountServiceForTest(t, store)
	if err := service.Resend(context.Background(), "sveta@example.com", "request-123"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("replace error = %v", err)
	}
}

func TestLoginRefreshLogoutAndBootstrap(t *testing.T) {
	hash, err := (PasswordHasher{}).Hash("Надёжный пароль 2026!", "sveta.design", "sveta@example.com")
	if err != nil {
		t.Fatal(err)
	}
	role := "super_admin"
	store := &fakeAccountStore{loginFound: true, loginAccount: LoginAccount{AccountView: AccountView{ID: "00000000-0000-0000-0000-000000000001", Login: "sveta.design", Email: "sveta@example.com", FirstName: "Светлана", ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер", Status: "active", GlobalRole: &role, Version: 1}, PasswordHash: hash.PHC, SecurityVersion: 2}}
	service, _, now := newAccountServiceForTest(t, store)
	session, err := service.Login(context.Background(), LoginRequest{Identifier: "SVETA.DESIGN", Password: "Надёжный пароль 2026!", RememberMe: true, RequestID: "request-login"})
	if err != nil || len(session.AccessToken) != 43 || len(session.RefreshToken) != 43 || len(session.CSRFToken) != 43 {
		t.Fatalf("session=%#v error=%v", session, err)
	}
	if bytes.Equal(store.newSession.AccessHash, store.newSession.RefreshHash) || bytes.Contains(store.newSession.AccessHash, []byte(session.AccessToken)) || !store.newSession.RememberMe || !store.newSession.AccessExpiresAt.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("new session=%#v", store.newSession)
	}
	if len(store.rateLimitRequests) != 2 || store.rateLimitRequests[0].PolicyCode != "login.marker" || store.rateLimitRequests[1].PolicyCode != "login.account" || bytes.Equal(store.rateLimitRequests[0].SubjectHash, store.rateLimitRequests[1].SubjectHash) || len(store.newSession.LoginAccountLimitHash) != 32 {
		t.Fatalf("rate limits=%#v session=%#v", store.rateLimitRequests, store.newSession)
	}
	if _, err = service.Login(context.Background(), LoginRequest{Identifier: "sveta.design", Password: "wrong", RequestID: "request-wrong"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error=%v", err)
	}

	store.rotationAccount = store.loginAccount
	store.rotationAccount.RememberMe = true
	store.rotationAccount.RefreshExpiresAt = now.Add(90 * 24 * time.Hour)
	rotated, err := service.Refresh(context.Background(), session.RefreshToken, session.CSRFToken, "request-refresh")
	if err != nil || rotated.RefreshToken == session.RefreshToken || !rotated.RememberMe || !rotated.RefreshExpiresAt.Equal(store.rotationAccount.RefreshExpiresAt) {
		t.Fatalf("rotated=%#v error=%v", rotated, err)
	}
	if bytes.Equal(store.rotation.RefreshHash, store.rotation.NewRefreshHash) || len(store.rotation.CSRFHash) != 32 {
		t.Fatalf("rotation=%#v", store.rotation)
	}
	if err = service.Logout(context.Background(), session.AccessToken, "", session.CSRFToken, "request-logout"); err != nil || len(store.revocation.AccessHash) != 32 || len(store.revocation.CSRFHash) != 32 {
		t.Fatalf("revocation=%#v error=%v", store.revocation, err)
	}

	bootstrapStore := &fakeAccountStore{bootstrapOK: true, bootstrapAccount: store.loginAccount}
	bootstrapService, _, _ := newAccountServiceForTest(t, bootstrapStore)
	bootstrapService.ConfigureBootstrap("root.admin", "Bootstrap пароль 2026!")
	if _, err = bootstrapService.Login(context.Background(), LoginRequest{Identifier: "root.admin", Password: "Bootstrap пароль 2026!", RequestID: "request-bootstrap"}); err != nil || bootstrapStore.bootstrap.PasswordHash == "" || bootstrapStore.bootstrap.PasswordHash == "Bootstrap пароль 2026!" {
		t.Fatalf("bootstrap=%#v error=%v", bootstrapStore.bootstrap, err)
	}
	if _, err = bootstrapService.Login(context.Background(), LoginRequest{Identifier: "root.admin", Password: "wrong bootstrap", RequestID: "request-bootstrap-wrong"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong bootstrap error=%v", err)
	}
}

func TestSessionServiceFailureBranches(t *testing.T) {
	t.Run("invalid login input", func(t *testing.T) {
		service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{})
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "x", Password: "p", RequestID: "request-invalid"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("lookup failure", func(t *testing.T) {
		service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{loginErr: context.DeadlineExceeded})
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "password", RequestID: "request-lookup"}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		store := &fakeAccountStore{rateLimitRetry: 90 * time.Second}
		service, _, _ := newAccountServiceForTest(t, store)
		_, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "password", RequestID: "request-limited", AnonymousMarker: "198.51.100.1"})
		var limited RateLimitError
		if !errors.As(err, &limited) || limited.RetryAfter != 90*time.Second || store.loginFailure != "rate_limited" {
			t.Fatalf("error=%v audit=%q", err, store.loginFailure)
		}
	})
	t.Run("limiter unavailable", func(t *testing.T) {
		store := &fakeAccountStore{rateLimitErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "password", RequestID: "request-limit-db"}); !errors.Is(err, ErrServiceUnavailable) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("rate limit audit unavailable", func(t *testing.T) {
		store := &fakeAccountStore{rateLimitRetry: time.Minute, loginFailureErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "password", RequestID: "request-limit-audit"}); !errors.Is(err, ErrServiceUnavailable) {
			t.Fatalf("error=%v", err)
		}
	})
	hash, _ := (PasswordHasher{}).Hash("Надёжный пароль 2026!", "valid.user", "valid@example.com")
	base := LoginAccount{AccountView: AccountView{ID: "00000000-0000-0000-0000-000000000001", Login: "valid.user", Email: "valid@example.com", FirstName: "Valid", ProfessionalRoleCode: "customer", ProfessionalRoleName: "Заказчик", Status: "active", Version: 1}, PasswordHash: hash.PHC, SecurityVersion: 1}
	t.Run("unverified audit", func(t *testing.T) {
		unverified := base
		unverified.Status = "pending_verification"
		store := &fakeAccountStore{loginFound: true, loginAccount: unverified}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "Надёжный пароль 2026!", RequestID: "request-unverified"}); !errors.Is(err, ErrAccountUnverified) || store.loginFailure != "unverified" {
			t.Fatalf("error=%v audit=%q", err, store.loginFailure)
		}
	})
	t.Run("disabled audit", func(t *testing.T) {
		disabled := base
		disabled.Status = "disabled"
		store := &fakeAccountStore{loginFound: true, loginAccount: disabled}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "Надёжный пароль 2026!", RequestID: "request-disabled"}); !errors.Is(err, ErrAccountUnavailable) || store.loginFailure != "disabled" {
			t.Fatalf("error=%v audit=%q", err, store.loginFailure)
		}
	})
	t.Run("login audit failure", func(t *testing.T) {
		store := &fakeAccountStore{loginFound: true, loginAccount: base, loginFailureErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "wrong", RequestID: "request-audit"}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("missing account audit failure", func(t *testing.T) {
		store := &fakeAccountStore{loginFailureErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "missing.user", Password: "wrong", RequestID: "request-missing-audit"}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("session persistence failure", func(t *testing.T) {
		store := &fakeAccountStore{loginFound: true, loginAccount: base, createSessionErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "Надёжный пароль 2026!", RequestID: "request-session"}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("refresh validation and store failure", func(t *testing.T) {
		store := &fakeAccountStore{rotationErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		if _, err := service.Refresh(context.Background(), "short", "short", "request-refresh"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("error=%v", err)
		}
		if _, err := service.Refresh(context.Background(), strings.Repeat("r", 43), strings.Repeat("c", 43), "request-refresh"); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("logout validation and store failure", func(t *testing.T) {
		store := &fakeAccountStore{revocationErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		if err := service.Logout(context.Background(), "", "", "short", "request-logout"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("error=%v", err)
		}
		if err := service.Logout(context.Background(), strings.Repeat("a", 43), "", strings.Repeat("c", 43), "request-logout"); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("configured bootstrap password may use the legacy system secret", func(t *testing.T) {
		store := &fakeAccountStore{bootstrapOK: true}
		service, _, _ := newAccountServiceForTest(t, store)
		service.ConfigureBootstrap("root.admin", "too-short")
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "root.admin", Password: "too-short", RequestID: "request-bootstrap"}); err != nil || store.bootstrap.PasswordHash == "" {
			t.Fatalf("bootstrap=%#v error=%v", store.bootstrap, err)
		}
	})
	t.Run("session token entropy failure", func(t *testing.T) {
		store := &fakeAccountStore{loginFound: true, loginAccount: base}
		service, _, _ := newAccountServiceForTest(t, store)
		service.tokens.random = failingReader{}
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "Надёжный пароль 2026!", RequestID: "request-entropy"}); err == nil {
			t.Fatal("entropy failure must propagate")
		}
	})
	for name, available := range map[string]int{"refresh entropy": 32, "csrf entropy": 64} {
		t.Run(name, func(t *testing.T) {
			store := &fakeAccountStore{loginFound: true, loginAccount: base}
			service, _, _ := newAccountServiceForTest(t, store)
			service.tokens.random = &limitedEntropyReader{remaining: available}
			if _, err := service.Login(context.Background(), LoginRequest{Identifier: "valid.user", Password: "Надёжный пароль 2026!", RequestID: "request-staged-entropy"}); err == nil {
				t.Fatal("staged entropy failure must propagate")
			}
		})
	}
	for name, available := range map[string]int{"refresh access entropy": 0, "refresh refresh entropy": 32, "refresh csrf entropy": 64} {
		t.Run(name, func(t *testing.T) {
			service, _, _ := newAccountServiceForTest(t, &fakeAccountStore{})
			service.tokens.random = &limitedEntropyReader{remaining: available}
			if _, err := service.Refresh(context.Background(), strings.Repeat("r", 43), strings.Repeat("c", 43), "request-refresh-entropy"); err == nil {
				t.Fatal("refresh entropy failure must propagate")
			}
		})
	}
	t.Run("bootstrap invalid sender", func(t *testing.T) {
		tokens, _ := NewTokenManager(bytes.Repeat([]byte{1}, 32), 1)
		cipher, _ := NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
		service, err := NewService(&fakeAccountStore{}, tokens, cipher, "https://designer-svetlana.ru", "not-an-email", time.Now)
		if err != nil {
			t.Fatal(err)
		}
		service.ConfigureBootstrap("root.admin", "Bootstrap пароль 2026!")
		if _, err = service.Login(context.Background(), LoginRequest{Identifier: "root.admin", Password: "Bootstrap пароль 2026!", RequestID: "request-bad-sender"}); err == nil {
			t.Fatal("invalid bootstrap sender must fail")
		}
	})
	t.Run("bootstrap already completed", func(t *testing.T) {
		store := &fakeAccountStore{}
		service, _, _ := newAccountServiceForTest(t, store)
		service.ConfigureBootstrap("root.admin", "Bootstrap пароль 2026!")
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "root.admin", Password: "Bootstrap пароль 2026!", RequestID: "request-bootstrap-done"}); !errors.Is(err, ErrInvalidCredentials) || store.loginFailure != "invalid_credentials" {
			t.Fatalf("error=%v audit=%q", err, store.loginFailure)
		}
	})
	t.Run("bootstrap store failure", func(t *testing.T) {
		store := &fakeAccountStore{bootstrapErr: context.DeadlineExceeded}
		service, _, _ := newAccountServiceForTest(t, store)
		service.ConfigureBootstrap("root.admin", "Bootstrap пароль 2026!")
		if _, err := service.Login(context.Background(), LoginRequest{Identifier: "root.admin", Password: "Bootstrap пароль 2026!", RequestID: "request-bootstrap-store"}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
}
