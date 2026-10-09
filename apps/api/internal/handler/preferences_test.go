package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/preferences"
	"github.com/google/uuid"
)

type endpointPreferencesStore struct {
	user         preferences.UserSettings
	project      preferences.ProjectUserSettings
	actor        preferences.Actor
	userID       string
	projectID    string
	savedUser    preferences.UserSettings
	savedProject preferences.ProjectUserSettings
	err          error
}

func (store *endpointPreferencesStore) GetUserSettings(_ context.Context, userID string) (preferences.UserSettings, error) {
	store.userID = userID
	return store.user, store.err
}
func (store *endpointPreferencesStore) SaveUserSettings(_ context.Context, userID string, settings preferences.UserSettings) (preferences.UserSettings, error) {
	store.userID, store.savedUser = userID, settings
	return settings, store.err
}
func (store *endpointPreferencesStore) GetProjectUserSettings(_ context.Context, actor preferences.Actor, projectID string) (preferences.ProjectUserSettings, error) {
	store.actor, store.projectID = actor, projectID
	return store.project, store.err
}
func (store *endpointPreferencesStore) SaveProjectUserSettings(_ context.Context, actor preferences.Actor, projectID string, settings preferences.ProjectUserSettings) (preferences.ProjectUserSettings, error) {
	store.actor, store.projectID, store.savedProject = actor, projectID, settings
	return settings, store.err
}

func preferencesEndpointForTest(t *testing.T, store *endpointPreferencesStore) *AccountEndpoint {
	t.Helper()
	endpoint := accountEndpointForTest(t, activeAccount(nil))
	service, err := preferences.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.ConfigurePreferences(service)
	return endpoint
}

func TestPreferencesEndpointsReadAndSaveAuthenticatedSettings(t *testing.T) {
	store := &endpointPreferencesStore{user: preferences.UserSettings{Theme: "dark"}, project: preferences.ProjectUserSettings{UpcomingDays: 14}}
	endpoint := preferencesEndpointForTest(t, store)

	response := httptest.NewRecorder()
	endpoint.GetUserSettings(response, authenticatedRequest(http.MethodGet, "/api/v1/settings", ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"theme":"dark"`) || store.userID != endpointActorID {
		t.Fatalf("get user status=%d body=%s userID=%s", response.Code, response.Body.String(), store.userID)
	}

	response = httptest.NewRecorder()
	endpoint.UpdateUserSettings(response, authenticatedRequest(http.MethodPut, "/api/v1/settings", `{"theme":"light"}`))
	if response.Code != http.StatusOK || store.savedUser.Theme != "light" {
		t.Fatalf("save user status=%d body=%s settings=%#v", response.Code, response.Body.String(), store.savedUser)
	}

	response = httptest.NewRecorder()
	endpoint.GetProjectUserSettings(response, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/settings", ""), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"upcomingDays":14`) || store.actor.UserID != endpointActorID {
		t.Fatalf("get project status=%d body=%s actor=%#v", response.Code, response.Body.String(), store.actor)
	}

	response = httptest.NewRecorder()
	endpoint.UpdateProjectUserSettings(response, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/settings", `{"upcomingDays":30}`), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusOK || store.savedProject.UpcomingDays != 30 {
		t.Fatalf("save project status=%d body=%s settings=%#v", response.Code, response.Body.String(), store.savedProject)
	}
}

func TestPreferencesEndpointsRejectInvalidCsrfInputAndHiddenProject(t *testing.T) {
	store := &endpointPreferencesStore{}
	endpoint := preferencesEndpointForTest(t, store)

	for _, body := range []string{`{"theme":"system"}`, `{"theme":"dark","extra":true}`} {
		response := httptest.NewRecorder()
		endpoint.UpdateUserSettings(response, authenticatedRequest(http.MethodPut, "/api/v1/settings", body))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}

	request := authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/settings", `{"upcomingDays":7}`)
	request.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.UpdateProjectUserSettings(response, request, uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusForbidden {
		t.Fatalf("csrf status=%d body=%s", response.Code, response.Body.String())
	}

	for _, body := range []string{`{"upcomingDays":0}`, `{"upcomingDays":7,"extra":true}`, `{"upcomingDays":`} {
		response = httptest.NewRecorder()
		endpoint.UpdateProjectUserSettings(response, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/settings", body), uuid.MustParse(endpointProjectID))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("project body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}

	store.err = preferences.ErrNotFound
	response = httptest.NewRecorder()
	endpoint.GetProjectUserSettings(response, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/settings", ""), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusNotFound {
		t.Fatalf("hidden status=%d body=%s", response.Code, response.Body.String())
	}

	store.err = errors.New("database unavailable")
	response = httptest.NewRecorder()
	endpoint.GetUserSettings(response, authenticatedRequest(http.MethodGet, "/api/v1/settings", ""))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("storage status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	endpoint.GetProjectUserSettings(response, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/settings", ""), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("project storage status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	endpoint.UpdateUserSettings(response, authenticatedRequest(http.MethodPut, "/api/v1/settings", `{"theme":"dark"}`))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("save user storage status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	endpoint.UpdateProjectUserSettings(response, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/settings", `{"upcomingDays":14}`), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("save project storage status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPreferencesEndpointsRequireAuthenticatedAccount(t *testing.T) {
	endpoint := preferencesEndpointForTest(t, &endpointPreferencesStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)

	endpoint.GetUserSettings(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPreferencesEndpointReportsUnavailableService(t *testing.T) {
	endpoint := accountEndpointForTest(t, activeAccount(nil))
	response := httptest.NewRecorder()
	endpoint.GetUserSettings(response, authenticatedRequest(http.MethodGet, "/api/v1/settings", ""))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

var _ accountgenerated.ServerInterface = (*AccountEndpoint)(nil)
