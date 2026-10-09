package account

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func adminActor(role string) AccountView {
	return AccountView{ID: "00000000-0000-0000-0000-000000000001", Login: "admin", GlobalRole: &role, Status: "active"}
}

func TestAdminUsersRejectInvalidFiltersCursorsAndStoreFailures(t *testing.T) {
	role := "super_admin"
	actor := adminActor(role)
	store := &fakeAccountStore{}
	service, _, _ := newAccountServiceForTest(t, store)
	for _, request := range []AdminUserListRequest{
		{Actor: actor, PageSize: 51},
		{Actor: actor, Status: "unknown"},
		{Actor: actor, Identifier: "x"},
		{Actor: actor, Identifier: strings.Repeat("я", 255)},
		{Actor: actor, Cursor: "single-part"},
		{Actor: actor, Cursor: "%%%.%%%"},
	} {
		if _, err := service.ListUsers(context.Background(), request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("request=%#v error=%v", request, err)
		}
	}
	encodeCursor := func(payload string) string {
		signature := service.tokens.HashForPurpose(payload, adminUsersCursorPurpose)
		return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(signature)
	}
	for _, cursor := range []string{
		encodeCursor(`{"registeredAt":"bad","id":"00000000-0000-0000-0000-000000000001"}`),
		encodeCursor(`{"registeredAt":"2026-09-01T00:00:00Z","id":"bad"}`),
	} {
		if _, err := service.ListUsers(context.Background(), AdminUserListRequest{Actor: actor, Cursor: cursor}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("cursor=%q error=%v", cursor, err)
		}
	}
	store.adminUsersErr = context.DeadlineExceeded
	if _, err := service.ListUsers(context.Background(), AdminUserListRequest{Actor: actor}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("list store error=%v", err)
	}

	store.adminUsersErr = nil
	for _, testCase := range []struct {
		actor   AccountView
		target  string
		version int64
		request string
		want    error
	}{
		{actor: AccountView{}, target: actor.ID, version: 1, request: "request-valid", want: ErrForbidden},
		{actor: actor, target: "bad", version: 1, request: "request-valid", want: ErrInvalidInput},
		{actor: actor, target: actor.ID, version: 0, request: "request-valid", want: ErrInvalidInput},
		{actor: actor, target: actor.ID, version: 1, request: "short", want: ErrInvalidInput},
	} {
		if _, err := service.DisableUser(context.Background(), testCase.actor, testCase.target, testCase.version, testCase.request); !errors.Is(err, testCase.want) {
			t.Fatalf("status case=%#v error=%v", testCase, err)
		}
	}
	store.rateLimitErr = context.DeadlineExceeded
	if _, err := service.DisableUser(context.Background(), actor, "00000000-0000-0000-0000-000000000002", 1, "request-valid"); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("rate limiter error=%v", err)
	}
	store.rateLimitErr = nil
	store.adminStatusErr = context.DeadlineExceeded
	if _, err := service.DisableUser(context.Background(), actor, "00000000-0000-0000-0000-000000000002", 1, "request-valid"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("status store error=%v", err)
	}
}

func TestListUsersRequiresGlobalAdministratorAndBuildsSignedCursor(t *testing.T) {
	store := &fakeAccountStore{}
	service, _, _ := newAccountServiceForTest(t, store)
	if _, err := service.ListUsers(context.Background(), AdminUserListRequest{Actor: AccountView{}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin error=%v", err)
	}
	role := "super_admin"
	first := AdminUserView{AccountView: AccountView{ID: "00000000-0000-0000-0000-000000000010"}, RegisteredAt: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)}
	second := AdminUserView{AccountView: AccountView{ID: "00000000-0000-0000-0000-000000000009"}, RegisteredAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	store.adminUsers = []AdminUserView{first, second}
	page, err := service.ListUsers(context.Background(), AdminUserListRequest{Actor: adminActor(role), PageSize: 1, Identifier: " USER@Example.COM "})
	if err != nil || !page.HasMore || page.NextCursor == nil || len(page.Items) != 1 {
		t.Fatalf("page=%#v error=%v", page, err)
	}
	if store.adminQuery.Identifier != "user@example.com" || store.adminQuery.Limit != 2 {
		t.Fatalf("query=%#v", store.adminQuery)
	}
	if _, err := service.ListUsers(context.Background(), AdminUserListRequest{Actor: adminActor(role), Cursor: *page.NextCursor + "x"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("tampered cursor error=%v", err)
	}
}

func TestAdminUserStatusChecksAuthorizationRateLimitAndCommand(t *testing.T) {
	role := "technical_admin"
	store := &fakeAccountStore{adminStatus: AdminUserView{AccountView: AccountView{ID: "00000000-0000-0000-0000-000000000002", Version: 4}}}
	service, _, now := newAccountServiceForTest(t, store)
	view, err := service.DisableUser(context.Background(), adminActor(role), store.adminStatus.ID, 3, "request-admin-disable")
	if err != nil || view.Version != 4 || !store.adminCommand.Disable || store.adminCommand.ExpectedVersion != 3 || !store.adminCommand.Now.Equal(now) {
		t.Fatalf("view=%#v command=%#v error=%v", view, store.adminCommand, err)
	}
	if len(store.rateLimitRequests) != 1 || store.rateLimitRequests[0].PolicyCode != "admin.user-status" {
		t.Fatalf("rate limits=%#v", store.rateLimitRequests)
	}
	store.rateLimitRetry = 3 * time.Second
	if _, err := service.RestoreUser(context.Background(), adminActor(role), store.adminStatus.ID, 4, "request-admin-restore"); err == nil {
		t.Fatal("expected rate limit")
	}
}
