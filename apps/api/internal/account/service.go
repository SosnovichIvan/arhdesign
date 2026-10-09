package account

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

var (
	ErrInvalidInput          = errors.New("invalid registration input")
	ErrIdentifierUnavailable = errors.New("identifier unavailable")
	ErrInvalidToken          = errors.New("invalid or expired token")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrAccountUnverified     = errors.New("account unverified")
	ErrAccountUnavailable    = errors.New("account unavailable")
	ErrUnauthenticated       = errors.New("unauthenticated")
	ErrServiceUnavailable    = errors.New("service unavailable")
	ErrForbidden             = errors.New("forbidden")
	ErrAdminUserNotFound     = errors.New("admin user not found")
	ErrAdminUserConflict     = errors.New("admin user version conflict")
	ErrLastSuperAdmin        = errors.New("last active super administrator")
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

const verificationMessageType = "account.email_verification"
const passwordResetMessageType = "account.password_reset"
const constantWorkPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type Registration struct {
	Login, Email, FirstName, LastName, MiddleName, ProfessionalRoleCode string
	Password, PasswordConfirmation                                      string
	RequestID                                                           string
}

type RegistrationValidationError struct {
	Cause  error
	Fields map[string]string
}

func (err RegistrationValidationError) Error() string { return "invalid registration fields" }
func (err RegistrationValidationError) Unwrap() error { return err.Cause }

type PendingAccount struct {
	Login, LoginNormalized, Email, EmailNormalized        string
	FirstName, LastName, MiddleName, ProfessionalRoleCode string
	PasswordHash, PasswordAlgorithm                       string
	PasswordParameters                                    map[string]any
	PasswordHashVersion                                   int
	TokenHash                                             []byte
	TokenKeyVersion                                       int
	TokenExpiresAt                                        time.Time
	OutboxCiphertext                                      []byte
	OutboxKeyVersion                                      int
	OutboxIdempotencyKey                                  string
	RequestID                                             string
}

type ReplacementVerification struct {
	IdentifierNormalized string
	TokenHash            []byte
	TokenKeyVersion      int
	TokenExpiresAt       time.Time
	OutboxCiphertext     []byte
	OutboxKeyVersion     int
	OutboxIdempotencyKey string
	RequestID            string
}

type Store interface {
	CreatePendingAccount(context.Context, PendingAccount) error
	ConsumeVerificationToken(context.Context, []byte, int, time.Time, string) error
	PendingAccountEmail(context.Context, string) (string, bool, error)
	ReplaceVerificationToken(context.Context, ReplacementVerification) (bool, error)
	SetVerificationChannel(context.Context, VerificationChannelSelection) (bool, error)
	FindLoginAccount(context.Context, string) (LoginAccount, bool, error)
	ResolveAccessSession(context.Context, []byte, time.Time) (AccountView, bool, error)
	CreateSession(context.Context, NewSession) error
	BootstrapSuperAdminSession(context.Context, BootstrapSession) (LoginAccount, bool, error)
	EnsureTechnicalAdmin(context.Context, TechnicalAdminBootstrap) (AccountView, bool, error)
	RotateSession(context.Context, SessionRotation) (LoginAccount, error)
	RevokeSessionFamily(context.Context, SessionRevocation) error
	RecordLoginFailure(context.Context, string, string) error
	ConsumeRateLimit(context.Context, RateLimitRequest) (time.Duration, error)
	ActiveAccountEmail(context.Context, string) (string, bool, error)
	CreatePasswordReset(context.Context, PasswordResetIssue) (bool, error)
	CreateTelegramPasswordReset(context.Context, PasswordResetIssue) (bool, error)
	PasswordResetContext(context.Context, []byte, int, time.Time, string) (PasswordResetAccount, error)
	CompletePasswordReset(context.Context, PasswordResetCompletion) error
	RecordPasswordResetRequest(context.Context, string, string) error
	RecordPasswordResetFailure(context.Context, string) error
	BeginTelegramAuth(context.Context, TelegramAuthStart) error
	AdvanceTelegramAuth(context.Context, TelegramAuthAdvance) error
	TelegramAuthStep(context.Context, int64, time.Time) (string, error)
	TelegramAuthAccount(context.Context, int64, time.Time) (LoginAccount, bool, error)
	CompleteTelegramAuth(context.Context, TelegramAuthCompletion) error
	RejectTelegramAuth(context.Context, TelegramAuthRejection) error
	ListAdminUsers(context.Context, AdminUserQuery) ([]AdminUserView, error)
	SetAdminUserStatus(context.Context, AdminUserStatusCommand) (AdminUserView, error)
}

type Service struct {
	store             Store
	hasher            PasswordHasher
	tokens            TokenManager
	cipher            PayloadCipher
	now               func() time.Time
	publicOrigin      string
	mailFrom          string
	bootstrapLogin    string
	bootstrapPassword string
	telegramBotName   string
	dummyPasswordHash string
}

type LoginRequest struct {
	Identifier, Password, RequestID, AnonymousMarker string
	RememberMe                                       bool
}

type RateLimitRequest struct {
	PolicyCode               string
	SubjectHash              []byte
	HashKeyVersion, Capacity int
	FullRefill, Retention    time.Duration
	Now                      time.Time
}

type RateLimitError struct{ RetryAfter time.Duration }

func (err RateLimitError) Error() string { return "rate limited" }

type PasswordResetIssue struct {
	IdentifierNormalized string
	TokenHash            []byte
	TokenKeyVersion      int
	TokenExpiresAt       time.Time
	OutboxCiphertext     []byte
	OutboxKeyVersion     int
	OutboxIdempotencyKey string
	RequestID            string
}

type VerificationChannelSelection struct {
	IdentifierNormalized string
	Channel              string
	TokenHash            []byte
	TokenKeyVersion      int
	TokenExpiresAt       time.Time
	OutboxCiphertext     []byte
	OutboxKeyVersion     int
	OutboxIdempotencyKey string
	RequestID            string
	Now                  time.Time
}

type PasswordResetAccount struct{ Login, Email string }

type PasswordResetCompletion struct {
	TokenHash                       []byte
	TokenKeyVersion                 int
	PasswordHash, PasswordAlgorithm string
	PasswordParameters              map[string]any
	PasswordHashVersion             int
	Now                             time.Time
	RequestID                       string
}

type AccountView struct {
	ID, Login, Email, FirstName, ProfessionalRoleCode, ProfessionalRoleName, Status string
	LastName, MiddleName, GlobalRole                                                *string
	Version                                                                         int64
}

func (view AccountView) IsGlobalAdministrator() bool {
	return view.GlobalRole != nil && (*view.GlobalRole == "super_admin" || *view.GlobalRole == "technical_admin")
}

type LoginAccount struct {
	AccountView
	PasswordHash     string
	SecurityVersion  int64
	RememberMe       bool
	RefreshExpiresAt time.Time
}

type Session struct {
	Account                              AccountView
	AccessToken, RefreshToken, CSRFToken string
	AccessExpiresAt, RefreshExpiresAt    time.Time
	RememberMe                           bool
}

type NewSession struct {
	UserID, RequestID                      string
	AccessHash, RefreshHash, CSRFHash      []byte
	HashKeyVersion                         int
	SecurityVersion                        int64
	Now, AccessExpiresAt, RefreshExpiresAt time.Time
	RememberMe                             bool
	LoginAccountLimitHash                  []byte
}

type BootstrapSession struct {
	Login, Email, FirstName, ProfessionalRoleCode string
	PasswordHash, PasswordAlgorithm               string
	PasswordParameters                            map[string]any
	PasswordHashVersion                           int
	Session                                       NewSession
}

type TechnicalAdminBootstrap struct {
	Login, Email, FirstName, ProfessionalRoleCode string
	PasswordHash, PasswordAlgorithm               string
	PasswordParameters                            map[string]any
	PasswordHashVersion                           int
	Now                                           time.Time
}

type SessionRotation struct {
	RefreshHash, CSRFHash                      []byte
	NewAccessHash, NewRefreshHash, NewCSRFHash []byte
	HashKeyVersion                             int
	Now, AccessExpiresAt                       time.Time
	RequestID                                  string
}

type SessionRevocation struct {
	AccessHash, RefreshHash, CSRFHash []byte
	Now                               time.Time
	RequestID                         string
}

type VerificationEmail struct {
	From, To, Subject, Text string
}

type TelegramAuthMessage struct {
	Text string `json:"text"`
}

type TelegramAuthStart struct {
	ChatID               int64
	Flow, RequestID      string
	StartedAt, ExpiresAt time.Time
}

type TelegramAuthAdvance struct {
	ChatID          int64
	CandidateUserID *string
	Now             time.Time
}

type TelegramAuthCompletion struct {
	ChatID, MessageID int64
	UserID            string
	ChatUsername      string
	Now               time.Time
	RequestID         string
}

type TelegramAuthRejection struct {
	ChatID    int64
	Now       time.Time
	RequestID string
	Reason    string
}

func NewService(store Store, tokens TokenManager, cipher PayloadCipher, publicOrigin, mailFrom string, now func() time.Time) (*Service, error) {
	if store == nil || strings.TrimSpace(publicOrigin) == "" || strings.TrimSpace(mailFrom) == "" {
		return nil, fmt.Errorf("account service requires store, public origin and mail sender")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, tokens: tokens, cipher: cipher, publicOrigin: strings.TrimRight(publicOrigin, "/"), mailFrom: mailFrom, now: now, dummyPasswordHash: constantWorkPasswordHash}, nil
}

func (service *Service) ConfigureBootstrap(login, password string) {
	service.bootstrapLogin = strings.ToLower(strings.TrimSpace(norm.NFC.String(login)))
	service.bootstrapPassword = norm.NFC.String(password)
}

func (service *Service) ConfigureTelegramBot(username string) {
	service.telegramBotName = strings.TrimPrefix(strings.TrimSpace(username), "@")
}

// EnsureTechnicalAdmin creates the configured system account before the HTTP
// server starts. Repeated starts preserve an existing technical administrator's
// credential; password rotation remains an explicit account operation.
func (service *Service) EnsureTechnicalAdmin(ctx context.Context, login, email, password string) (AccountView, bool, error) {
	login = strings.ToLower(strings.TrimSpace(norm.NFC.String(login)))
	email = strings.ToLower(strings.TrimSpace(norm.NFC.String(email)))
	password = norm.NFC.String(password)
	// The single environment-managed technical administrator has a documented
	// 14-character exception. Self-service registration continues to require 15.
	if !loginPattern.MatchString(login) || len([]rune(password)) < 14 || len([]rune(password)) > 128 {
		return AccountView{}, false, ErrInvalidInput
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || strings.ToLower(parsed.Address) != email {
		return AccountView{}, false, ErrInvalidInput
	}
	passwordHash, err := service.hasher.HashBootstrap(password)
	if err != nil {
		return AccountView{}, false, err
	}
	return service.store.EnsureTechnicalAdmin(ctx, TechnicalAdminBootstrap{
		Login: login, Email: email, FirstName: "Технический администратор", ProfessionalRoleCode: "designer",
		PasswordHash: passwordHash.PHC, PasswordAlgorithm: passwordHash.Algorithm,
		PasswordParameters: passwordHash.Parameters, PasswordHashVersion: passwordHash.Version, Now: service.now().UTC(),
	})
}

func (service *Service) PublicOrigin() string { return service.publicOrigin }

// Authenticate resolves the opaque access cookie on every protected request.
// The repository owns expiry, revocation, account status and security-version
// checks so a disabled account loses access without waiting for cookie expiry.
func (service *Service) Authenticate(ctx context.Context, accessToken string) (AccountView, error) {
	if len(accessToken) != 43 {
		return AccountView{}, ErrUnauthenticated
	}
	view, found, err := service.store.ResolveAccessSession(ctx, service.tokens.HashForPurpose(accessToken, accessPurpose), service.now().UTC())
	if err != nil {
		return AccountView{}, err
	}
	if !found {
		return AccountView{}, ErrUnauthenticated
	}
	return view, nil
}

func (service *Service) Login(ctx context.Context, request LoginRequest) (Session, error) {
	request.Identifier = strings.ToLower(strings.TrimSpace(norm.NFC.String(request.Identifier)))
	request.Password = norm.NFC.String(request.Password)
	if len([]rune(request.Identifier)) < 3 || len([]rune(request.Identifier)) > 254 || len([]rune(request.Password)) < 1 || len([]rune(request.Password)) > 128 || len(request.RequestID) < 8 {
		return Session{}, ErrInvalidInput
	}
	accountLimitHash := service.tokens.HashForPurpose(request.Identifier, "arhdesign/rate-limit/login-account/v1")
	marker := strings.TrimSpace(request.AnonymousMarker)
	if marker == "" {
		marker = "unknown"
	}
	markerLimitHash := service.tokens.HashForPurpose(marker, "arhdesign/rate-limit/login-marker/v1")
	if err := service.consumeLoginLimit(ctx, RateLimitRequest{PolicyCode: "login.marker", SubjectHash: markerLimitHash, HashKeyVersion: service.tokens.Version(), Capacity: 30, FullRefill: 15 * time.Minute, Retention: 48 * time.Hour, Now: service.now().UTC()}, request.RequestID); err != nil {
		return Session{}, err
	}
	if err := service.consumeLoginLimit(ctx, RateLimitRequest{PolicyCode: "login.account", SubjectHash: accountLimitHash, HashKeyVersion: service.tokens.Version(), Capacity: 5, FullRefill: 15 * time.Minute, Retention: 48 * time.Hour, Now: service.now().UTC()}, request.RequestID); err != nil {
		return Session{}, err
	}
	accountRecord, found, err := service.store.FindLoginAccount(ctx, request.Identifier)
	if err != nil {
		return Session{}, err
	}
	if !found {
		_, _ = service.hasher.Compare(service.dummyPasswordHash, request.Password)
		session, bootstrapErr := service.bootstrap(ctx, request, accountLimitHash)
		if errors.Is(bootstrapErr, ErrInvalidCredentials) {
			if auditErr := service.store.RecordLoginFailure(ctx, "invalid_credentials", request.RequestID); auditErr != nil {
				return Session{}, auditErr
			}
		}
		return session, bootstrapErr
	}
	matched, compareErr := service.hasher.Compare(accountRecord.PasswordHash, request.Password)
	if compareErr != nil || !matched {
		if auditErr := service.store.RecordLoginFailure(ctx, "invalid_credentials", request.RequestID); auditErr != nil {
			return Session{}, auditErr
		}
		return Session{}, ErrInvalidCredentials
	}
	if accountRecord.Status != "active" {
		result := "unverified"
		if accountRecord.Status == "disabled" {
			result = "disabled"
		}
		if auditErr := service.store.RecordLoginFailure(ctx, result, request.RequestID); auditErr != nil {
			return Session{}, auditErr
		}
		if result == "unverified" {
			return Session{}, ErrAccountUnverified
		}
		return Session{}, ErrAccountUnavailable
	}
	session, prepared, err := service.prepareSession(accountRecord.AccountView, accountRecord.SecurityVersion, request.RememberMe, request.RequestID)
	if err != nil {
		return Session{}, err
	}
	prepared.LoginAccountLimitHash = accountLimitHash
	if err := service.store.CreateSession(ctx, prepared); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (service *Service) consumeLoginLimit(ctx context.Context, request RateLimitRequest, requestID string) error {
	retryAfter, err := service.store.ConsumeRateLimit(ctx, request)
	if err != nil {
		return fmt.Errorf("%w: login rate limiter", ErrServiceUnavailable)
	}
	if retryAfter <= 0 {
		return nil
	}
	if auditErr := service.store.RecordLoginFailure(ctx, "rate_limited", requestID); auditErr != nil {
		return fmt.Errorf("%w: rate-limit audit", ErrServiceUnavailable)
	}
	return RateLimitError{RetryAfter: retryAfter}
}

func (service *Service) bootstrap(ctx context.Context, request LoginRequest, accountLimitHash []byte) (Session, error) {
	if service.bootstrapLogin == "" || request.Identifier != service.bootstrapLogin || subtle.ConstantTimeCompare([]byte(request.Password), []byte(service.bootstrapPassword)) != 1 {
		return Session{}, ErrInvalidCredentials
	}
	parsedFrom, err := mail.ParseAddress(service.mailFrom)
	if err != nil {
		return Session{}, fmt.Errorf("bootstrap mail sender must be an email address: %w", err)
	}
	bootstrapEmail := strings.ToLower(parsedFrom.Address)
	passwordHash, err := service.hasher.HashBootstrap(request.Password)
	if err != nil {
		return Session{}, err
	}
	placeholder := AccountView{Login: service.bootstrapLogin, Email: bootstrapEmail, FirstName: "Администратор", ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер", Status: "active", Version: 1}
	session, prepared, err := service.prepareSession(placeholder, 1, request.RememberMe, request.RequestID)
	if err != nil {
		return Session{}, err
	}
	prepared.LoginAccountLimitHash = accountLimitHash
	created, bootstrapped, err := service.store.BootstrapSuperAdminSession(ctx, BootstrapSession{
		Login: service.bootstrapLogin, Email: bootstrapEmail, FirstName: "Администратор", ProfessionalRoleCode: "designer",
		PasswordHash: passwordHash.PHC, PasswordAlgorithm: passwordHash.Algorithm, PasswordParameters: passwordHash.Parameters, PasswordHashVersion: passwordHash.Version,
		Session: prepared,
	})
	if err != nil {
		return Session{}, err
	}
	if !bootstrapped {
		return Session{}, ErrInvalidCredentials
	}
	session.Account = created.AccountView
	return session, nil
}

func (service *Service) prepareSession(view AccountView, securityVersion int64, remember bool, requestID string) (Session, NewSession, error) {
	access, accessHash, version, err := service.tokens.NewForPurpose(accessPurpose)
	if err != nil {
		return Session{}, NewSession{}, err
	}
	refresh, refreshHash, _, err := service.tokens.NewForPurpose(refreshPurpose)
	if err != nil {
		return Session{}, NewSession{}, err
	}
	csrf, csrfHash, _, err := service.tokens.NewForPurpose(csrfPurpose)
	if err != nil {
		return Session{}, NewSession{}, err
	}
	now := service.now().UTC()
	accessExpires := now.Add(30 * time.Minute)
	refreshExpires := now.Add(90 * 24 * time.Hour)
	return Session{Account: view, AccessToken: access, RefreshToken: refresh, CSRFToken: csrf, AccessExpiresAt: accessExpires, RefreshExpiresAt: refreshExpires, RememberMe: remember}, NewSession{
		UserID: view.ID, RequestID: requestID, AccessHash: accessHash, RefreshHash: refreshHash, CSRFHash: csrfHash, HashKeyVersion: version,
		SecurityVersion: securityVersion, Now: now, AccessExpiresAt: accessExpires, RefreshExpiresAt: refreshExpires, RememberMe: remember,
	}, nil
}

func (service *Service) Refresh(ctx context.Context, refreshToken, csrfToken, requestID string) (Session, error) {
	if len(refreshToken) < 32 || len(csrfToken) < 32 {
		return Session{}, ErrUnauthenticated
	}
	access, accessHash, version, err := service.tokens.NewForPurpose(accessPurpose)
	if err != nil {
		return Session{}, err
	}
	refresh, refreshHash, _, err := service.tokens.NewForPurpose(refreshPurpose)
	if err != nil {
		return Session{}, err
	}
	csrf, csrfHash, _, err := service.tokens.NewForPurpose(csrfPurpose)
	if err != nil {
		return Session{}, err
	}
	now := service.now().UTC()
	rotation := SessionRotation{RefreshHash: service.tokens.HashForPurpose(refreshToken, refreshPurpose), CSRFHash: service.tokens.HashForPurpose(csrfToken, csrfPurpose), NewAccessHash: accessHash, NewRefreshHash: refreshHash, NewCSRFHash: csrfHash, HashKeyVersion: version, Now: now, AccessExpiresAt: now.Add(30 * time.Minute), RequestID: requestID}
	view, err := service.store.RotateSession(ctx, rotation)
	if err != nil {
		return Session{}, err
	}
	return Session{Account: view.AccountView, AccessToken: access, RefreshToken: refresh, CSRFToken: csrf, AccessExpiresAt: rotation.AccessExpiresAt, RefreshExpiresAt: view.RefreshExpiresAt, RememberMe: view.RememberMe}, nil
}

func (service *Service) Logout(ctx context.Context, accessToken, refreshToken, csrfToken, requestID string) error {
	if len(csrfToken) < 32 || (len(accessToken) < 32 && len(refreshToken) < 32) {
		return ErrUnauthenticated
	}
	return service.store.RevokeSessionFamily(ctx, SessionRevocation{
		AccessHash: service.tokens.HashForPurpose(accessToken, accessPurpose), RefreshHash: service.tokens.HashForPurpose(refreshToken, refreshPurpose), CSRFHash: service.tokens.HashForPurpose(csrfToken, csrfPurpose), Now: service.now().UTC(), RequestID: requestID,
	})
}

func (service *Service) Register(ctx context.Context, request Registration) (time.Time, error) {
	request = normalizeRegistration(request)
	if err := validateRegistration(request); err != nil {
		return time.Time{}, err
	}
	passwordHash, err := service.hasher.Hash(request.Password, request.Login, request.Email)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			return time.Time{}, RegistrationValidationError{Cause: ErrInvalidPassword, Fields: map[string]string{
				"password": "Выберите другой пароль длиной от 15 до 128 символов",
			}}
		}
		return time.Time{}, err
	}
	now := service.now().UTC()
	err = service.store.CreatePendingAccount(ctx, PendingAccount{
		Login: request.Login, LoginNormalized: strings.ToLower(request.Login), Email: request.Email, EmailNormalized: strings.ToLower(request.Email),
		FirstName: request.FirstName, LastName: request.LastName, MiddleName: request.MiddleName, ProfessionalRoleCode: request.ProfessionalRoleCode,
		PasswordHash: passwordHash.PHC, PasswordAlgorithm: passwordHash.Algorithm, PasswordParameters: passwordHash.Parameters, PasswordHashVersion: passwordHash.Version,
		RequestID: request.RequestID,
	})
	if err != nil {
		return time.Time{}, err
	}
	return now, nil
}

func (service *Service) SelectVerificationChannel(ctx context.Context, identifier, channel, requestID string) (string, error) {
	identifier = strings.ToLower(strings.TrimSpace(norm.NFC.String(identifier)))
	channel = strings.ToLower(strings.TrimSpace(channel))
	if len([]rune(identifier)) < 3 || len([]rune(identifier)) > 254 || (channel != "email" && channel != "telegram") || len(requestID) < 8 {
		return "", ErrInvalidInput
	}

	selection := VerificationChannelSelection{IdentifierNormalized: identifier, Channel: channel, RequestID: requestID, Now: service.now().UTC()}
	if channel == "email" {
		recipient, found, err := service.store.PendingAccountEmail(ctx, identifier)
		if err != nil {
			return "", err
		}
		if !found {
			recipient = "pending-account@example.invalid"
		}
		token, tokenHash, tokenVersion, err := service.tokens.New()
		if err != nil {
			return "", err
		}
		ciphertext, cipherVersion, idempotencyKey, err := service.verificationOutbox(recipient, token, tokenHash)
		if err != nil {
			return "", err
		}
		selection.TokenHash = tokenHash
		selection.TokenKeyVersion = tokenVersion
		selection.TokenExpiresAt = selection.Now.Add(24 * time.Hour)
		selection.OutboxCiphertext = ciphertext
		selection.OutboxKeyVersion = cipherVersion
		selection.OutboxIdempotencyKey = idempotencyKey
	}
	if _, err := service.store.SetVerificationChannel(ctx, selection); err != nil {
		return "", err
	}
	if channel != "telegram" {
		return "", nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`).MatchString(service.telegramBotName) {
		return "", fmt.Errorf("%w: Telegram bot is not configured", ErrServiceUnavailable)
	}
	return "https://t.me/" + service.telegramBotName + "?start=register", nil
}

func (service *Service) BeginTelegramConfirmation(ctx context.Context, chatID int64, requestID string) error {
	if chatID == 0 || len(requestID) < 8 {
		return ErrInvalidInput
	}
	now := service.now().UTC()
	return service.store.BeginTelegramAuth(ctx, TelegramAuthStart{ChatID: chatID, Flow: "registration_confirmation", RequestID: requestID, StartedAt: now, ExpiresAt: now.Add(5 * time.Minute)})
}

func (service *Service) SubmitTelegramLogin(ctx context.Context, chatID int64, identifier string) error {
	identifier = strings.ToLower(strings.TrimSpace(norm.NFC.String(identifier)))
	if chatID == 0 || len([]rune(identifier)) < 3 || len([]rune(identifier)) > 254 {
		return ErrInvalidInput
	}
	loginAccount, found, err := service.store.FindLoginAccount(ctx, identifier)
	if err != nil {
		return err
	}
	var candidate *string
	if found {
		candidate = &loginAccount.ID
	}
	return service.store.AdvanceTelegramAuth(ctx, TelegramAuthAdvance{ChatID: chatID, CandidateUserID: candidate, Now: service.now().UTC()})
}

func (service *Service) TelegramConfirmationStep(ctx context.Context, chatID int64) (string, error) {
	if chatID == 0 {
		return "", ErrInvalidInput
	}
	return service.store.TelegramAuthStep(ctx, chatID, service.now().UTC())
}

func (service *Service) CompleteTelegramConfirmation(ctx context.Context, chatID, messageID int64, username, password, requestID string) error {
	password = norm.NFC.String(password)
	if chatID == 0 || messageID == 0 || len([]rune(password)) < 1 || len([]rune(password)) > 128 || len(requestID) < 8 {
		return ErrInvalidCredentials
	}
	now := service.now().UTC()
	chatLimit := RateLimitRequest{PolicyCode: "telegram_auth.chat", SubjectHash: service.tokens.HashForPurpose(fmt.Sprint(chatID), "arhdesign/rate-limit/telegram-chat/v1"), HashKeyVersion: service.tokens.Version(), Capacity: 5, FullRefill: 15 * time.Minute, Retention: 48 * time.Hour, Now: now}
	if retryAfter, err := service.store.ConsumeRateLimit(ctx, chatLimit); err != nil {
		return fmt.Errorf("%w: Telegram auth rate limiter", ErrServiceUnavailable)
	} else if retryAfter > 0 {
		return RateLimitError{RetryAfter: retryAfter}
	}
	loginAccount, found, err := service.store.TelegramAuthAccount(ctx, chatID, now)
	if err != nil {
		return err
	}
	hash := service.dummyPasswordHash
	if found {
		hash = loginAccount.PasswordHash
	}
	matched, compareErr := service.hasher.Compare(hash, password)
	if compareErr != nil || !matched || !found || loginAccount.Status == "disabled" {
		_ = service.store.RejectTelegramAuth(ctx, TelegramAuthRejection{ChatID: chatID, Now: now, RequestID: requestID, Reason: "invalid_credentials"})
		return ErrInvalidCredentials
	}
	return service.store.CompleteTelegramAuth(ctx, TelegramAuthCompletion{ChatID: chatID, MessageID: messageID, UserID: loginAccount.ID, ChatUsername: strings.TrimPrefix(strings.TrimSpace(username), "@"), Now: now, RequestID: requestID})
}

func (service *Service) Verify(ctx context.Context, token, requestID string) error {
	if len(token) < 32 || len(token) > 512 {
		return ErrInvalidToken
	}
	return service.store.ConsumeVerificationToken(ctx, service.tokens.Hash(token), service.tokens.Version(), service.now().UTC(), requestID)
}

func (service *Service) Resend(ctx context.Context, identifier, requestID string) error {
	identifier = strings.ToLower(strings.TrimSpace(norm.NFC.String(identifier)))
	if len([]rune(identifier)) < 3 || len([]rune(identifier)) > 254 {
		return ErrInvalidInput
	}
	token, tokenHash, tokenVersion, err := service.tokens.New()
	if err != nil {
		return err
	}
	recipient, found, err := service.store.PendingAccountEmail(ctx, identifier)
	if err != nil {
		return err
	}
	if !found {
		recipient = "pending-account@example.invalid"
	}
	ciphertext, cipherVersion, idempotencyKey, err := service.verificationOutbox(recipient, token, tokenHash)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	_, err = service.store.ReplaceVerificationToken(ctx, ReplacementVerification{
		IdentifierNormalized: identifier, TokenHash: tokenHash, TokenKeyVersion: tokenVersion,
		TokenExpiresAt: service.now().UTC().Add(24 * time.Hour), OutboxCiphertext: ciphertext,
		OutboxKeyVersion: cipherVersion, OutboxIdempotencyKey: idempotencyKey, RequestID: requestID,
	})
	return err
}

func (service *Service) ForgotPassword(ctx context.Context, identifier, channel, anonymousMarker, requestID string) error {
	identifier = strings.ToLower(strings.TrimSpace(norm.NFC.String(identifier)))
	channel = strings.ToLower(strings.TrimSpace(channel))
	if len([]rune(identifier)) < 3 || len([]rune(identifier)) > 254 || (channel != "email" && channel != "telegram") || len(requestID) < 8 {
		return ErrInvalidInput
	}
	marker := strings.TrimSpace(anonymousMarker)
	if marker == "" {
		marker = "unknown"
	}
	now := service.now().UTC()
	limits := []RateLimitRequest{
		{PolicyCode: "forgot.marker", SubjectHash: service.tokens.HashForPurpose(marker, "arhdesign/rate-limit/forgot-marker/v1"), HashKeyVersion: service.tokens.Version(), Capacity: 10, FullRefill: time.Hour, Retention: 48 * time.Hour, Now: now},
		{PolicyCode: "forgot.account", SubjectHash: service.tokens.HashForPurpose(identifier, "arhdesign/rate-limit/forgot-account/v1"), HashKeyVersion: service.tokens.Version(), Capacity: 3, FullRefill: time.Hour, Retention: 48 * time.Hour, Now: now},
	}
	for _, limit := range limits {
		retryAfter, err := service.store.ConsumeRateLimit(ctx, limit)
		if err != nil {
			return fmt.Errorf("%w: forgot-password rate limiter", ErrServiceUnavailable)
		}
		if retryAfter > 0 {
			if auditErr := service.store.RecordPasswordResetRequest(ctx, "rate_limited", requestID); auditErr != nil {
				return fmt.Errorf("%w: forgot-password rate-limit audit", ErrServiceUnavailable)
			}
			return RateLimitError{RetryAfter: retryAfter}
		}
	}
	token, tokenHash, tokenVersion, err := service.tokens.NewForPurpose(passwordResetPurpose)
	if err != nil {
		return err
	}
	issue := PasswordResetIssue{
		IdentifierNormalized: identifier, TokenHash: tokenHash, TokenKeyVersion: tokenVersion,
		TokenExpiresAt: now.Add(30 * time.Minute), RequestID: requestID,
	}
	if channel == "telegram" {
		ciphertext, cipherVersion, idempotencyKey, err := service.telegramPasswordResetOutbox(token, tokenHash)
		if err != nil {
			return err
		}
		issue.OutboxCiphertext, issue.OutboxKeyVersion, issue.OutboxIdempotencyKey = ciphertext, cipherVersion, idempotencyKey
		_, err = service.store.CreateTelegramPasswordReset(ctx, issue)
		return err
	}
	recipient, found, err := service.store.ActiveAccountEmail(ctx, identifier)
	if err != nil {
		return err
	}
	if !found {
		recipient = "active-account@example.invalid"
	}
	ciphertext, cipherVersion, idempotencyKey, err := service.passwordResetOutbox(recipient, token, tokenHash)
	if err != nil {
		return err
	}
	issue.OutboxCiphertext, issue.OutboxKeyVersion, issue.OutboxIdempotencyKey = ciphertext, cipherVersion, idempotencyKey
	_, err = service.store.CreatePasswordReset(ctx, issue)
	return err
}

func (service *Service) ResetPassword(ctx context.Context, token, password, confirmation, anonymousMarker, requestID string) error {
	password = norm.NFC.String(password)
	confirmation = norm.NFC.String(confirmation)
	if password != confirmation || len(requestID) < 8 {
		return ErrInvalidInput
	}
	marker := strings.TrimSpace(anonymousMarker)
	if marker == "" {
		marker = "unknown"
	}
	now := service.now().UTC()
	retryAfter, err := service.store.ConsumeRateLimit(ctx, RateLimitRequest{PolicyCode: "reset.marker", SubjectHash: service.tokens.HashForPurpose(marker, "arhdesign/rate-limit/reset-marker/v1"), HashKeyVersion: service.tokens.Version(), Capacity: 10, FullRefill: 15 * time.Minute, Retention: 48 * time.Hour, Now: now})
	if err != nil {
		return fmt.Errorf("%w: reset-password rate limiter", ErrServiceUnavailable)
	}
	if retryAfter > 0 {
		return RateLimitError{RetryAfter: retryAfter}
	}
	if len(token) < 32 || len(token) > 512 {
		if err := service.store.RecordPasswordResetFailure(ctx, requestID); err != nil {
			return err
		}
		return ErrInvalidToken
	}
	tokenHash := service.tokens.HashForPurpose(token, passwordResetPurpose)
	resetAccount, err := service.store.PasswordResetContext(ctx, tokenHash, service.tokens.Version(), now, requestID)
	if err != nil {
		return err
	}
	passwordHash, err := service.hasher.Hash(password, resetAccount.Login, resetAccount.Email)
	if err != nil {
		return err
	}
	return service.store.CompletePasswordReset(ctx, PasswordResetCompletion{
		TokenHash: tokenHash, TokenKeyVersion: service.tokens.Version(), PasswordHash: passwordHash.PHC,
		PasswordAlgorithm: passwordHash.Algorithm, PasswordParameters: passwordHash.Parameters,
		PasswordHashVersion: passwordHash.Version, Now: now, RequestID: requestID,
	})
}

func (service *Service) verificationOutbox(recipient, token string, tokenHash []byte) ([]byte, int, string, error) {
	message := VerificationEmail{
		From: service.mailFrom, To: recipient, Subject: "Подтвердите электронную почту — arhDesign",
		Text: "Здравствуйте!\n\nПодтвердите электронную почту, чтобы завершить регистрацию в личном кабинете arhDesign:\n" + service.publicOrigin + "/verify-email#token=" + token + "\n\nСсылка действует 24 часа. Если вы не регистрировались, просто проигнорируйте это письмо.",
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, 0, "", fmt.Errorf("encode verification email: %w", err)
	}
	ciphertext, version, err := service.cipher.Encrypt(payload, verificationMessageType)
	return ciphertext, version, "email-verification:" + hex.EncodeToString(tokenHash), err
}

func (service *Service) passwordResetOutbox(recipient, token string, tokenHash []byte) ([]byte, int, string, error) {
	message := VerificationEmail{
		From: service.mailFrom, To: recipient, Subject: "Восстановление доступа — arhDesign",
		Text: "Здравствуйте!\n\nЧтобы установить новый пароль личного кабинета arhDesign, откройте ссылку:\n" + service.publicOrigin + "/reset-password#token=" + token + "\n\nСсылка действует 30 минут и применяется только один раз. Если вы не запрашивали восстановление, просто проигнорируйте это письмо.",
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, 0, "", fmt.Errorf("encode password-reset email: %w", err)
	}
	ciphertext, version, err := service.cipher.Encrypt(payload, passwordResetMessageType)
	return ciphertext, version, "password-reset:" + hex.EncodeToString(tokenHash), err
}

func (service *Service) telegramPasswordResetOutbox(token string, tokenHash []byte) ([]byte, int, string, error) {
	message := TelegramAuthMessage{Text: "Восстановление доступа к arhDesign\n\nОткройте ссылку, чтобы установить новый пароль:\n" + service.publicOrigin + "/reset-password#token=" + token + "\n\nСсылка действует 30 минут и применяется один раз."}
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, 0, "", fmt.Errorf("encode Telegram password reset: %w", err)
	}
	ciphertext, version, err := service.cipher.Encrypt(payload, passwordResetMessageType)
	return ciphertext, version, "telegram-password-reset:" + hex.EncodeToString(tokenHash), err
}

func normalizeRegistration(request Registration) Registration {
	request.Login = norm.NFC.String(strings.TrimSpace(request.Login))
	request.Email = strings.ToLower(norm.NFC.String(strings.TrimSpace(request.Email)))
	request.FirstName = norm.NFC.String(strings.TrimSpace(request.FirstName))
	request.LastName = norm.NFC.String(strings.TrimSpace(request.LastName))
	request.MiddleName = norm.NFC.String(strings.TrimSpace(request.MiddleName))
	request.ProfessionalRoleCode = strings.ToLower(strings.TrimSpace(request.ProfessionalRoleCode))
	request.Password = norm.NFC.String(request.Password)
	request.PasswordConfirmation = norm.NFC.String(request.PasswordConfirmation)
	return request
}

func validateRegistration(request Registration) error {
	fields := make(map[string]string)
	if !loginPattern.MatchString(request.Login) {
		fields["login"] = "Используйте 3–64 латинских символа, цифры, точку, дефис или подчёркивание"
	}
	parsedEmail, err := mail.ParseAddress(request.Email)
	if err != nil || strings.ToLower(parsedEmail.Address) != request.Email || len([]rune(request.Email)) > 254 {
		fields["email"] = "Введите корректный адрес электронной почты"
	}
	if !lengthBetween(request.FirstName, 1, 100) {
		fields["firstName"] = "Укажите имя длиной до 100 символов"
	}
	if !optionalLength(request.LastName, 100) {
		fields["lastName"] = "Фамилия не должна быть длиннее 100 символов"
	}
	if !optionalLength(request.MiddleName, 100) {
		fields["middleName"] = "Отчество не должно быть длиннее 100 символов"
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`).MatchString(request.ProfessionalRoleCode) {
		fields["professionalRoleCode"] = "Выберите роль из списка"
	}
	if request.Password != request.PasswordConfirmation {
		fields["passwordConfirmation"] = "Пароли не совпадают"
	}
	if len(request.RequestID) < 8 {
		return ErrInvalidInput
	}
	if len(fields) > 0 {
		return RegistrationValidationError{Cause: ErrInvalidInput, Fields: fields}
	}
	return nil
}

func lengthBetween(value string, minimum, maximum int) bool {
	length := len([]rune(value))
	return length >= minimum && length <= maximum
}
func optionalLength(value string, maximum int) bool {
	return value == "" || lengthBetween(value, 1, maximum)
}
