package handler

import (
	"net/http"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (endpoint *AccountEndpoint) ListAccountNotifications(w http.ResponseWriter, r *http.Request, params accountgenerated.ListAccountNotificationsParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	pageSize := 20
	if params.PageSize != nil {
		pageSize = *params.PageSize
	}
	list, err := endpoint.projects.ListNotifications(r.Context(), actor, pageSize)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.AccountNotification, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, accountgenerated.AccountNotification{
			Id: openapi_types.UUID(uuid.MustParse(item.ID)), ProjectId: openapi_types.UUID(uuid.MustParse(item.ProjectID)), EventType: item.EventType,
			Title: item.Title, Body: item.Body, Href: item.Href, CreatedAt: item.CreatedAt, ReadAt: item.ReadAt,
		})
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.AccountNotificationList{Items: items, UnreadCount: list.UnreadCount})
}

func (endpoint *AccountEndpoint) MarkAccountNotificationRead(w http.ResponseWriter, r *http.Request, notificationID openapi_types.UUID) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	if err := endpoint.projects.MarkNotificationRead(r.Context(), actor, notificationID.String()); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) MarkAllAccountNotificationsRead(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	if err := endpoint.projects.MarkAllNotificationsRead(r.Context(), actor); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
