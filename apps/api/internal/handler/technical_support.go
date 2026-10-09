package handler

import (
	"errors"
	"net/http"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
)

func (endpoint *AccountEndpoint) ReportFrontendError(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.technicalActor(w, r, requestID)
	if !ok {
		return
	}
	if _, ok = endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	var body accountgenerated.FrontendErrorReportRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Некорректный отчёт об ошибке", requestID)
		return
	}
	digest := ""
	if body.Digest != nil {
		digest = *body.Digest
	}
	err := endpoint.technical.ReportFrontend(r.Context(), actor, technicalsupport.FrontendErrorRequest{Message: body.Message, Path: body.Path, Fingerprint: body.Fingerprint, Digest: digest, RequestID: requestID})
	if err != nil {
		endpoint.writeTechnicalSupportError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusAccepted, accountgenerated.SimpleStatus{Status: "accepted"})
}

func (endpoint *AccountEndpoint) CreateTechnicalFeedback(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.technicalActor(w, r, requestID)
	if !ok {
		return
	}
	if _, ok = endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	var body accountgenerated.TechnicalFeedbackRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте сообщение", requestID)
		return
	}
	err := endpoint.technical.CreateFeedback(r.Context(), actor, technicalsupport.FeedbackRequest{Category: string(body.Category), Message: body.Message, Path: body.Path, RequestID: requestID})
	if err != nil {
		endpoint.writeTechnicalSupportError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusAccepted, accountgenerated.SimpleStatus{Status: "accepted"})
}

func (endpoint *AccountEndpoint) technicalActor(w http.ResponseWriter, r *http.Request, requestID string) (technicalsupport.Actor, bool) {
	if endpoint.technical == nil {
		writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис поддержки временно недоступен", requestID)
		return technicalsupport.Actor{}, false
	}
	view, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return technicalsupport.Actor{}, false
	}
	return technicalsupport.Actor{UserID: view.ID, Login: view.Login, FirstName: view.FirstName}, true
}

func (endpoint *AccountEndpoint) writeTechnicalSupportError(w http.ResponseWriter, err error, requestID string) {
	switch {
	case errors.Is(err, technicalsupport.ErrInvalidInput):
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте данные сообщения", requestID)
	case errors.Is(err, technicalsupport.ErrRateLimited):
		w.Header().Set("Retry-After", "3600")
		writeAccountError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много сообщений. Повторите позже", requestID)
	default:
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось отправить сообщение", requestID)
	}
}
