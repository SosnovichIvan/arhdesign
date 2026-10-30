package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/google/uuid"
)

func TestAdminUserEndpointsListDisableAndRestore(t *testing.T) {
	role := "super_admin"
	now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	view := account.AdminUserView{AccountView: account.AccountView{
		ID: endpointMemberID, Login: "ivan", Email: "ivan@example.com", FirstName: "Иван", Status: "active",
		ProfessionalRoleCode: "architect", ProfessionalRoleName: "Архитектор", GlobalRole: &role, Version: 2,
	},
		RegisteredAt: now, LastInteractiveLoginAt: &now,
	}
	store := activeAccount(&role)
	store.adminUsers = []account.AdminUserView{view}
	store.adminStatus = view
	endpoint := accountEndpointForTest(t, store)

	pageSize := accountgenerated.PageSize(20)
	identifier := "ivan"
	list := httptest.NewRecorder()
	endpoint.ListUsers(list, authenticatedRequest(http.MethodGet, "/api/v1/users", ""), accountgenerated.ListUsersParams{PageSize: &pageSize, Identifier: &identifier})
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"login":"ivan"`) || store.adminQuery.Identifier != "ivan" {
		t.Fatalf("list status=%d body=%s query=%#v", list.Code, list.Body.String(), store.adminQuery)
	}

	disable := httptest.NewRecorder()
	endpoint.DisableUser(disable, authenticatedRequest(http.MethodPost, "/api/v1/users/"+endpointMemberID+"/disable", ""), uuid.MustParse(endpointMemberID), accountgenerated.DisableUserParams{IfMatch: `"2"`})
	if disable.Code != http.StatusOK || !store.adminCommand.Disable || store.adminCommand.ExpectedVersion != 2 || disable.Header().Get("ETag") != `"2"` {
		t.Fatalf("disable status=%d body=%s command=%#v", disable.Code, disable.Body.String(), store.adminCommand)
	}
	restore := httptest.NewRecorder()
	endpoint.RestoreUser(restore, authenticatedRequest(http.MethodPost, "/api/v1/users/"+endpointMemberID+"/restore", ""), uuid.MustParse(endpointMemberID), accountgenerated.RestoreUserParams{IfMatch: `"2"`})
	if restore.Code != http.StatusOK || store.adminCommand.Disable {
		t.Fatalf("restore status=%d body=%s command=%#v", restore.Code, restore.Body.String(), store.adminCommand)
	}
}

func TestAdminUserEndpointErrorsAreExplicit(t *testing.T) {
	role := "super_admin"
	for _, test := range []struct {
		err  error
		want int
	}{
		{account.ErrInvalidInput, http.StatusBadRequest},
		{account.ErrForbidden, http.StatusForbidden},
		{account.ErrAdminUserNotFound, http.StatusNotFound},
		{account.ErrAdminUserConflict, http.StatusConflict},
		{account.ErrLastSuperAdmin, http.StatusConflict},
		{account.RateLimitError{RetryAfter: 2 * time.Second}, http.StatusTooManyRequests},
		{account.ErrServiceUnavailable, http.StatusServiceUnavailable},
		{errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		store := activeAccount(&role)
		store.adminStatusErr = test.err
		endpoint := accountEndpointForTest(t, store)
		response := httptest.NewRecorder()
		endpoint.DisableUser(response, authenticatedRequest(http.MethodPost, "/disable", ""), uuid.MustParse(endpointMemberID), accountgenerated.DisableUserParams{IfMatch: `"1"`})
		if response.Code != test.want {
			t.Fatalf("error=%v status=%d want=%d body=%s", test.err, response.Code, test.want, response.Body.String())
		}
	}
}

func TestAdminUserMutationRejectsMissingCsrfAndBadVersion(t *testing.T) {
	role := "super_admin"
	endpoint := accountEndpointForTest(t, activeAccount(&role))
	request := authenticatedRequest(http.MethodPost, "/disable", "")
	request.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.DisableUser(response, request, uuid.MustParse(endpointMemberID), accountgenerated.DisableUserParams{IfMatch: `"1"`})
	if response.Code != http.StatusForbidden {
		t.Fatalf("csrf status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	endpoint.DisableUser(response, authenticatedRequest(http.MethodPost, "/disable", ""), uuid.MustParse(endpointMemberID), accountgenerated.DisableUserParams{IfMatch: "bad"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("version status=%d", response.Code)
	}
}
