package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (endpoint *AccountEndpoint) ListUsers(w http.ResponseWriter, r *http.Request, params accountgenerated.ListUsersParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return
	}
	request := account.AdminUserListRequest{Actor: actor}
	if params.PageSize != nil {
		request.PageSize = int(*params.PageSize)
	}
	if params.Cursor != nil {
		request.Cursor = string(*params.Cursor)
	}
	if params.Status != nil {
		request.Status = string(*params.Status)
	}
	if params.Identifier != nil {
		request.Identifier = *params.Identifier
	}
	page, err := endpoint.service.ListUsers(r.Context(), request)
	if err != nil {
		endpoint.writeAdminUserError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.AdminUserView, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, adminUserResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.UserPage{Items: items, HasMore: page.HasMore, NextCursor: page.NextCursor})
}

func (endpoint *AccountEndpoint) DisableUser(w http.ResponseWriter, r *http.Request, userID accountgenerated.UserId, params accountgenerated.DisableUserParams) {
	endpoint.changeAdminUserStatus(w, r, userID, string(params.IfMatch), true)
}

func (endpoint *AccountEndpoint) RestoreUser(w http.ResponseWriter, r *http.Request, userID accountgenerated.UserId, params accountgenerated.RestoreUserParams) {
	endpoint.changeAdminUserStatus(w, r, userID, string(params.IfMatch), false)
}

func (endpoint *AccountEndpoint) changeAdminUserStatus(w http.ResponseWriter, r *http.Request, userID accountgenerated.UserId, etag string, disable bool) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return
	}
	version, err := parseExpectedProjectVersion(etag)
	if err != nil || version < 1 {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Некорректная версия пользователя", requestID)
		return
	}
	var view account.AdminUserView
	if disable {
		view, err = endpoint.service.DisableUser(r.Context(), actor, userID.String(), version, requestID)
	} else {
		view, err = endpoint.service.RestoreUser(r.Context(), actor, userID.String(), version, requestID)
	}
	if err != nil {
		endpoint.writeAdminUserError(w, err, requestID)
		return
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(view.Version, 10)))
	writeAccountJSON(w, http.StatusOK, adminUserResponse(view))
}

func (endpoint *AccountEndpoint) writeAdminUserError(w http.ResponseWriter, err error, requestID string) {
	var rateLimit account.RateLimitError
	switch {
	case errors.Is(err, account.ErrInvalidInput):
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте параметры запроса", requestID)
	case errors.Is(err, account.ErrForbidden):
		writeAccountError(w, http.StatusForbidden, "forbidden", "Недостаточно прав", requestID)
	case errors.Is(err, account.ErrAdminUserNotFound):
		writeAccountError(w, http.StatusNotFound, "not_found", "Пользователь не найден", requestID)
	case errors.Is(err, account.ErrAdminUserConflict):
		writeAccountError(w, http.StatusConflict, "conflict", "Данные пользователя уже изменены", requestID)
	case errors.Is(err, account.ErrLastSuperAdmin):
		writeAccountError(w, http.StatusConflict, "last_super_admin", "Нельзя отключить последнего супер-администратора", requestID)
	case errors.As(err, &rateLimit):
		seconds := int64((rateLimit.RetryAfter + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
		writeAccountError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов. Повторите позже", requestID)
	case errors.Is(err, account.ErrServiceUnavailable):
		writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис временно недоступен", requestID)
	default:
		slog.Error("admin user request failed", "request_id", requestID, "error", err)
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось выполнить запрос", requestID)
	}
}

func adminUserResponse(view account.AdminUserView) accountgenerated.AdminUserView {
	response := accountgenerated.AdminUserView{
		Id: uuid.MustParse(view.ID), Login: view.Login, Email: openapi_types.Email(view.Email), FirstName: view.FirstName,
		LastName: view.LastName, MiddleName: view.MiddleName, Status: accountgenerated.AccountStatus(view.Status),
		ProfessionalRole: accountgenerated.ProfessionalRole{Code: view.ProfessionalRoleCode, Name: view.ProfessionalRoleName},
		Version:          view.Version, RegisteredAt: view.RegisteredAt, LastInteractiveLoginAt: view.LastInteractiveLoginAt,
	}
	if view.GlobalRole != nil {
		role := accountgenerated.AdminUserViewGlobalRole(*view.GlobalRole)
		response.GlobalRole = &role
	}
	return response
}
