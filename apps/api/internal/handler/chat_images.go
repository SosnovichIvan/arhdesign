package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/chatimage"
	"github.com/google/uuid"
)

func (endpoint *AccountEndpoint) ListChatImages(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ImageChatId, params accountgenerated.ListChatImagesParams) {
	access, requestID, ok := endpoint.chatImageAccess(w, r, chatID, params.ProjectId)
	if !ok {
		return
	}
	items, next, err := endpoint.chatImages.List(r.Context(), access, optionalString(params.Cursor))
	if err != nil {
		writeChatImageError(w, err, requestID)
		return
	}
	result := accountgenerated.ChatImagePage{Items: make([]accountgenerated.ChatImage, 0, len(items)), NextCursor: next}
	for _, item := range items {
		result.Items = append(result.Items, chatImageResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, result)
}

func (endpoint *AccountEndpoint) UploadChatImage(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ImageChatId, params accountgenerated.UploadChatImageParams) {
	access, requestID, ok := endpoint.chatImageAccess(w, r, chatID, params.ProjectId)
	if !ok {
		return
	}
	name, err := decodeProjectDocumentName(params.XFileName)
	if err != nil || r.ContentLength > chatimage.MaxBytes {
		writeChatImageError(w, chatimage.ErrInvalid, requestID)
		return
	}
	value, err := endpoint.chatImages.Upload(r.Context(), access, params.IdempotencyKey.String(), name, r.Body)
	if err != nil {
		writeChatImageError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, chatImageResponse(value))
}

func (endpoint *AccountEndpoint) ViewChatImage(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ImageChatId, imageID uuid.UUID, params accountgenerated.ViewChatImageParams) {
	access, requestID, ok := endpoint.chatImageAccess(w, r, chatID, params.ProjectId)
	if !ok {
		return
	}
	value, err := endpoint.chatImages.View(r.Context(), access, imageID.String())
	if err != nil {
		writeChatImageError(w, err, requestID)
		return
	}
	w.Header().Set("Content-Type", value.MediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(value.Bytes)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(value.Bytes)
}

func (endpoint *AccountEndpoint) SetChatImagePin(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ImageChatId, imageID uuid.UUID, params accountgenerated.SetChatImagePinParams) {
	access, requestID, ok := endpoint.chatImageAccess(w, r, chatID, params.ProjectId)
	if !ok {
		return
	}
	var input struct {
		Pinned *bool `json:"pinned"`
	}
	if !decodeAccountJSON(r, &input) || input.Pinned == nil {
		writeChatImageError(w, chatimage.ErrInvalid, requestID)
		return
	}
	if err := endpoint.chatImages.Pin(r.Context(), access, imageID.String(), *input.Pinned); err != nil {
		writeChatImageError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) DeleteChatImage(w http.ResponseWriter, r *http.Request, chatID accountgenerated.ImageChatId, imageID uuid.UUID, params accountgenerated.DeleteChatImageParams) {
	access, requestID, ok := endpoint.chatImageAccess(w, r, chatID, params.ProjectId)
	if !ok {
		return
	}
	if err := endpoint.chatImages.Delete(r.Context(), access, imageID.String()); err != nil {
		writeChatImageError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) chatImageAccess(w http.ResponseWriter, r *http.Request, chatID uuid.UUID, projectID *uuid.UUID) (chatimage.Access, string, bool) {
	requestID := accountRequestID(w, r)
	w.Header().Set("Cache-Control", "private, no-store")
	account, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return chatimage.Access{}, requestID, false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if _, ok = endpoint.validAuthenticatedMutation(r); !ok {
			writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
			return chatimage.Access{}, requestID, false
		}
	}
	if endpoint.chatImages == nil {
		writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Изображения временно недоступны", requestID)
		return chatimage.Access{}, requestID, false
	}
	access := chatimage.Access{ChatID: chatID.String(), UserID: account.ID, SuperAdmin: account.IsGlobalAdministrator()}
	if projectID != nil {
		access.ProjectID = projectID.String()
	}
	return access, requestID, true
}

func writeChatImageError(w http.ResponseWriter, err error, requestID string) {
	switch {
	case errors.Is(err, chatimage.ErrInvalid):
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте параметры. Поддерживаются изображения JPEG, PNG и GIF размером до 10 МБ", requestID)
	case errors.Is(err, chatimage.ErrNotFound):
		writeAccountError(w, http.StatusNotFound, "not_found", "Чат или изображение недоступны", requestID)
	case errors.Is(err, chatimage.ErrForbidden):
		writeAccountError(w, http.StatusForbidden, "forbidden", "Недостаточно прав, чтобы удалить изображение", requestID)
	case errors.Is(err, chatimage.ErrConflict):
		writeAccountError(w, http.StatusConflict, "conflict", "Заново выберите файл для загрузки", requestID)
	default:
		slog.Error("chat image request failed", "request_id", requestID, "error", err)
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось выполнить действие с изображением", requestID)
	}
}

func chatImageResponse(value chatimage.Image) accountgenerated.ChatImage {
	return accountgenerated.ChatImage{
		Id: uuid.MustParse(value.ID), Name: value.Name, MediaType: accountgenerated.ChatImageMediaType(value.MediaType),
		SizeBytes: value.SizeBytes, Width: value.Width, Height: value.Height, CreatedAt: value.CreatedAt,
		Pinned: value.Pinned, CanDelete: value.CanDelete,
	}
}
