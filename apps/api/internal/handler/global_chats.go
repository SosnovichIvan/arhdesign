package handler

import (
	"errors"
	"net/http"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (endpoint *AccountEndpoint) ListGlobalChats(w http.ResponseWriter, r *http.Request, params accountgenerated.ListGlobalChatsParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.globalChatActor(w, r, requestID)
	if !ok {
		return
	}
	pageSize, cursor := 0, ""
	if params.PageSize != nil {
		pageSize = int(*params.PageSize)
	}
	if params.Cursor != nil {
		cursor = string(*params.Cursor)
	}
	page, err := endpoint.globalChats.List(r.Context(), globalchat.ListRequest{Actor: actor, PageSize: pageSize, Cursor: cursor})
	if err != nil {
		endpoint.writeGlobalChatError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.GlobalChatSummary, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, globalChatSummaryResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.GlobalChatPage{Items: items, HasMore: page.HasMore, NextCursor: page.NextCursor})
}

func (endpoint *AccountEndpoint) SearchGlobalChatCandidates(w http.ResponseWriter, r *http.Request, params accountgenerated.SearchGlobalChatCandidatesParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.globalChatActor(w, r, requestID)
	if !ok {
		return
	}
	items, err := endpoint.globalChats.SearchCandidates(r.Context(), actor, params.Query)
	if err != nil {
		endpoint.writeGlobalChatError(w, err, requestID)
		return
	}
	response := make([]accountgenerated.GlobalChatCandidate, 0, len(items))
	for _, item := range items {
		response = append(response, globalChatCandidateResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.GlobalChatCandidateList{Items: response})
}

func (endpoint *AccountEndpoint) CreateGlobalChat(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.globalChatActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateGlobalChatRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте параметры чата", requestID)
		return
	}
	memberIDs := make([]string, 0, len(body.MemberUserIds))
	for _, value := range body.MemberUserIds {
		memberIDs = append(memberIDs, value.String())
	}
	name := ""
	if body.Name != nil {
		name = *body.Name
	}
	created, err := endpoint.globalChats.Create(r.Context(), actor, globalchat.CreateRequest{Kind: string(body.Kind), Name: name, MemberUserIDs: memberIDs, RequestID: requestID})
	if err != nil {
		endpoint.writeGlobalChatError(w, err, requestID)
		return
	}
	w.Header().Set("Location", "/api/v1/chats/"+created.ID)
	writeAccountJSON(w, http.StatusCreated, globalChatSummaryResponse(created))
}

func (endpoint *AccountEndpoint) GetGlobalChat(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ChatId, params accountgenerated.GetGlobalChatParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.globalChatActor(w, r, requestID)
	if !ok {
		return
	}
	pageSize, before := 0, ""
	if params.PageSize != nil {
		pageSize = *params.PageSize
	}
	if params.Before != nil {
		before = params.Before.String()
	}
	conversation, err := endpoint.globalChats.Get(r.Context(), actor, chatID.String(), pageSize, before)
	if err != nil {
		endpoint.writeGlobalChatError(w, err, requestID)
		return
	}
	messages := make([]accountgenerated.GlobalChatMessage, 0, len(conversation.Messages))
	for _, message := range conversation.Messages {
		messages = append(messages, globalChatMessageResponse(message))
	}
	currentUserID, _ := uuid.Parse(conversation.CurrentUserID)
	var nextCursor *openapi_types.UUID
	if conversation.NextCursor != nil {
		value, _ := uuid.Parse(*conversation.NextCursor)
		nextCursor = &value
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.GlobalChatConversation{Chat: globalChatSummaryResponse(conversation.Chat), CurrentUserId: currentUserID, Messages: messages, NextCursor: nextCursor, HasMore: conversation.HasMore, CanSend: conversation.CanSend})
}

func (endpoint *AccountEndpoint) CreateGlobalChatMessage(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ChatId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.globalChatActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectChatMessageRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте сообщение", requestID)
		return
	}
	message, err := endpoint.globalChats.Send(r.Context(), actor, chatID.String(), globalchat.SendRequest{Body: body.Body, ClientMessageID: body.ClientMessageId.String(), RequestID: requestID})
	if err != nil {
		endpoint.writeGlobalChatError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, globalChatMessageResponse(message))
}

func (endpoint *AccountEndpoint) MarkGlobalChatRead(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ChatId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.globalChatActor(w, r, requestID)
	if !ok {
		return
	}
	if err := endpoint.globalChats.MarkRead(r.Context(), actor, chatID.String()); err != nil {
		endpoint.writeGlobalChatError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) globalChatActor(w http.ResponseWriter, r *http.Request, requestID string) (globalchat.Actor, bool) {
	if endpoint.globalChats == nil {
		writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис чатов временно недоступен", requestID)
		return globalchat.Actor{}, false
	}
	view, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return globalchat.Actor{}, false
	}
	return globalchat.Actor{UserID: view.ID, Privileged: view.IsGlobalAdministrator(), Login: view.Login, FirstName: view.FirstName}, true
}

func (endpoint *AccountEndpoint) writeGlobalChatError(w http.ResponseWriter, err error, requestID string) {
	switch {
	case errors.Is(err, globalchat.ErrInvalidInput):
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте параметры чата", requestID)
	case errors.Is(err, globalchat.ErrNotFound):
		writeAccountError(w, http.StatusNotFound, "not_found", "Чат не найден или недоступен", requestID)
	case errors.Is(err, globalchat.ErrConflict):
		writeAccountError(w, http.StatusConflict, "conflict", "Данные чата уже изменились", requestID)
	default:
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось выполнить действие с чатом", requestID)
	}
}

func globalChatCandidateResponse(value globalchat.Candidate) accountgenerated.GlobalChatCandidate {
	id, _ := uuid.Parse(value.UserID)
	return accountgenerated.GlobalChatCandidate{UserId: id, Login: value.Login, Email: openapi_types.Email(value.Email), FirstName: value.FirstName, LastName: value.LastName}
}

func globalChatSummaryResponse(value globalchat.Summary) accountgenerated.GlobalChatSummary {
	id, _ := uuid.Parse(value.ID)
	members := make([]accountgenerated.GlobalChatMember, 0, len(value.Members))
	for _, member := range value.Members {
		memberID, _ := uuid.Parse(member.UserID)
		members = append(members, accountgenerated.GlobalChatMember{UserId: memberID, Login: member.Login, Email: openapi_types.Email(member.Email), FirstName: member.FirstName, LastName: member.LastName, JoinedAt: member.JoinedAt})
	}
	var last *accountgenerated.GlobalChatMessage
	if value.LastMessage != nil {
		mapped := globalChatMessageResponse(*value.LastMessage)
		last = &mapped
	}
	return accountgenerated.GlobalChatSummary{Id: id, Kind: accountgenerated.GlobalChatKind(value.Kind), Name: value.Name, DisplayName: value.DisplayName, Members: members, LastMessage: last, UnreadCount: value.UnreadCount, LastActivityAt: value.LastActivityAt, Version: value.Version}
}

func globalChatMessageResponse(value globalchat.Message) accountgenerated.GlobalChatMessage {
	id, _ := uuid.Parse(value.ID)
	chatID, _ := uuid.Parse(value.ChatID)
	authorID, _ := uuid.Parse(value.Author.UserID)
	return accountgenerated.GlobalChatMessage{Id: id, ChatId: chatID, Body: value.Body, CreatedAt: value.CreatedAt, EditedAt: value.EditedAt, DeletedAt: value.DeletedAt, Version: value.Version, Author: accountgenerated.GlobalChatAuthor{UserId: authorID, Login: value.Author.Login, FirstName: value.Author.FirstName, LastName: value.Author.LastName}}
}
