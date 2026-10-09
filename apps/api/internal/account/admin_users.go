package account

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

const adminUsersCursorPurpose = "arhdesign/admin-users-cursor/v1"

type AdminUserView struct {
	AccountView
	RegisteredAt           time.Time
	LastInteractiveLoginAt *time.Time
}

type AdminUserQuery struct {
	Limit              int
	Status             string
	Identifier         string
	BeforeRegisteredAt *time.Time
	BeforeID           *string
}

type AdminUserStatusCommand struct {
	ActorUserID, TargetUserID, RequestID string
	Disable                              bool
	ExpectedVersion                      int64
	Now                                  time.Time
}

type AdminUserListRequest struct {
	Actor                      AccountView
	PageSize                   int
	Cursor, Status, Identifier string
}

type AdminUserPage struct {
	Items      []AdminUserView
	NextCursor *string
	HasMore    bool
}

type adminUsersCursor struct {
	RegisteredAt time.Time `json:"registeredAt"`
	ID           string    `json:"id"`
}

func (service *Service) ListUsers(ctx context.Context, request AdminUserListRequest) (AdminUserPage, error) {
	if !request.Actor.IsGlobalAdministrator() {
		return AdminUserPage{}, ErrForbidden
	}
	if request.PageSize == 0 {
		request.PageSize = 20
	}
	if request.PageSize < 1 || request.PageSize > 50 {
		return AdminUserPage{}, ErrInvalidInput
	}
	status := strings.TrimSpace(request.Status)
	if status != "" && status != "pending_verification" && status != "active" && status != "disabled" {
		return AdminUserPage{}, ErrInvalidInput
	}
	identifier := strings.ToLower(norm.NFC.String(strings.TrimSpace(request.Identifier)))
	if identifier != "" && (len([]rune(identifier)) < 3 || len(identifier) > 254) {
		return AdminUserPage{}, ErrInvalidInput
	}
	query := AdminUserQuery{Limit: request.PageSize + 1, Status: status, Identifier: identifier}
	if request.Cursor != "" {
		cursor, ok := service.decodeAdminUsersCursor(request.Cursor)
		if !ok {
			return AdminUserPage{}, ErrInvalidInput
		}
		query.BeforeRegisteredAt, query.BeforeID = &cursor.RegisteredAt, &cursor.ID
	}
	items, err := service.store.ListAdminUsers(ctx, query)
	if err != nil {
		return AdminUserPage{}, err
	}
	page := AdminUserPage{Items: items}
	if len(items) > request.PageSize {
		page.HasMore = true
		page.Items = items[:request.PageSize]
		last := page.Items[len(page.Items)-1]
		cursor := service.encodeAdminUsersCursor(adminUsersCursor{RegisteredAt: last.RegisteredAt, ID: last.ID})
		page.NextCursor = &cursor
	}
	return page, nil
}

func (service *Service) DisableUser(ctx context.Context, actor AccountView, targetUserID string, expectedVersion int64, requestID string) (AdminUserView, error) {
	return service.setAdminUserStatus(ctx, actor, targetUserID, expectedVersion, requestID, true)
}

func (service *Service) RestoreUser(ctx context.Context, actor AccountView, targetUserID string, expectedVersion int64, requestID string) (AdminUserView, error) {
	return service.setAdminUserStatus(ctx, actor, targetUserID, expectedVersion, requestID, false)
}

func (service *Service) setAdminUserStatus(ctx context.Context, actor AccountView, targetUserID string, expectedVersion int64, requestID string, disable bool) (AdminUserView, error) {
	if !actor.IsGlobalAdministrator() {
		return AdminUserView{}, ErrForbidden
	}
	if _, err := uuid.Parse(targetUserID); err != nil || expectedVersion < 1 || len(requestID) < 8 {
		return AdminUserView{}, ErrInvalidInput
	}
	now := service.now().UTC()
	retry, err := service.store.ConsumeRateLimit(ctx, RateLimitRequest{
		PolicyCode: "admin.user-status", SubjectHash: service.tokens.HashForPurpose(actor.ID, "arhdesign/admin-user-status/v1"),
		HashKeyVersion: service.tokens.Version(), Capacity: 30, FullRefill: time.Minute, Retention: 24 * time.Hour, Now: now,
	})
	if err != nil {
		return AdminUserView{}, ErrServiceUnavailable
	}
	if retry > 0 {
		return AdminUserView{}, RateLimitError{RetryAfter: retry}
	}
	return service.store.SetAdminUserStatus(ctx, AdminUserStatusCommand{
		ActorUserID: actor.ID, TargetUserID: targetUserID, RequestID: requestID,
		Disable: disable, ExpectedVersion: expectedVersion, Now: now,
	})
}

func (service *Service) encodeAdminUsersCursor(cursor adminUsersCursor) string {
	payload, _ := json.Marshal(cursor)
	signature := service.tokens.HashForPurpose(string(payload), adminUsersCursorPurpose)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (service *Service) decodeAdminUsersCursor(value string) (adminUsersCursor, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return adminUsersCursor{}, false
	}
	payload, payloadErr := base64.RawURLEncoding.DecodeString(parts[0])
	signature, signatureErr := base64.RawURLEncoding.DecodeString(parts[1])
	expected := service.tokens.HashForPurpose(string(payload), adminUsersCursorPurpose)
	if payloadErr != nil || signatureErr != nil || subtle.ConstantTimeCompare(signature, expected) != 1 {
		return adminUsersCursor{}, false
	}
	var cursor adminUsersCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.RegisteredAt.IsZero() {
		return adminUsersCursor{}, false
	}
	if _, err := uuid.Parse(cursor.ID); err != nil {
		return adminUsersCursor{}, false
	}
	return cursor, true
}
