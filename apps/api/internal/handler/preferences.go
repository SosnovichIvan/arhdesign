package handler

import (
	"errors"
	"net/http"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/preferences"
)

func (endpoint *AccountEndpoint) GetUserSettings(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.preferencesActor(w, r, requestID)
	if !ok {
		return
	}
	settings, err := endpoint.preferences.GetUser(r.Context(), actor.UserID)
	if err != nil {
		endpoint.writePreferencesError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.UserSettings{Theme: accountgenerated.ThemePreference(settings.Theme)})
}

func (endpoint *AccountEndpoint) UpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.preferencesActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.UpdateUserSettingsRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте настройки", requestID)
		return
	}
	settings, err := endpoint.preferences.SaveUser(r.Context(), actor.UserID, preferences.UserSettings{Theme: string(body.Theme)})
	if err != nil {
		endpoint.writePreferencesError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.UserSettings{Theme: accountgenerated.ThemePreference(settings.Theme)})
}

func (endpoint *AccountEndpoint) GetProjectUserSettings(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.preferencesActor(w, r, requestID)
	if !ok {
		return
	}
	settings, err := endpoint.preferences.GetProject(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writePreferencesError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectUserSettings{UpcomingDays: settings.UpcomingDays})
}

func (endpoint *AccountEndpoint) UpdateProjectUserSettings(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.preferencesActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.UpdateProjectUserSettingsRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте настройки проекта", requestID)
		return
	}
	settings, err := endpoint.preferences.SaveProject(r.Context(), actor, projectID.String(), preferences.ProjectUserSettings{UpcomingDays: body.UpcomingDays})
	if err != nil {
		endpoint.writePreferencesError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectUserSettings{UpcomingDays: settings.UpcomingDays})
}

func (endpoint *AccountEndpoint) preferencesActor(w http.ResponseWriter, r *http.Request, requestID string) (preferences.Actor, bool) {
	if endpoint.preferences == nil {
		writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис настроек временно недоступен", requestID)
		return preferences.Actor{}, false
	}
	view, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return preferences.Actor{}, false
	}
	return preferences.Actor{UserID: view.ID, SuperAdmin: view.IsGlobalAdministrator()}, true
}

func (endpoint *AccountEndpoint) writePreferencesError(w http.ResponseWriter, err error, requestID string) {
	switch {
	case errors.Is(err, preferences.ErrInvalidInput):
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте настройки", requestID)
	case errors.Is(err, preferences.ErrNotFound):
		writeAccountError(w, http.StatusNotFound, "not_found", "Проект не найден", requestID)
	default:
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось сохранить настройки", requestID)
	}
}
