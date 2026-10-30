package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type optionalProjectField[T any] struct {
	Set   bool
	Value *T
}

func (field *optionalProjectField[T]) UnmarshalJSON(data []byte) error {
	field.Set = true
	if string(data) == "null" {
		field.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	field.Value = &value
	return nil
}

type projectUpdateBody struct {
	Name                optionalProjectField[string] `json:"name"`
	Type                optionalProjectField[string] `json:"type"`
	Address             optionalProjectField[string] `json:"address"`
	Status              optionalProjectField[string] `json:"status"`
	PlannedStartOn      optionalProjectField[string] `json:"plannedStartOn"`
	PlannedFinishOn     optionalProjectField[string] `json:"plannedFinishOn"`
	Description         optionalProjectField[string] `json:"description"`
	AutoApproveExpenses optionalProjectField[bool]   `json:"autoApproveExpenses"`
}

func (endpoint *AccountEndpoint) ListProjects(w http.ResponseWriter, r *http.Request, params accountgenerated.ListProjectsParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	pageSize, cursor, status := 0, "", ""
	if params.PageSize != nil {
		pageSize = *params.PageSize
	}
	if params.Cursor != nil {
		cursor = *params.Cursor
	}
	if params.Status != nil {
		status = string(*params.Status)
	}
	page, err := endpoint.projects.List(r.Context(), project.ListRequest{Actor: actor, PageSize: pageSize, Cursor: cursor, Status: status})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectView, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, projectResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectPage{Items: items, NextCursor: page.NextCursor, HasMore: page.HasMore})
}

func (endpoint *AccountEndpoint) CreateProject(w http.ResponseWriter, r *http.Request) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля проекта", requestID)
		return
	}
	request := project.CreateRequest{
		Name: body.Name, Type: body.Type, CurrencyCode: body.CurrencyCode,
		AutoApproveExpenses: body.AutoApproveExpenses, RequestID: requestID,
	}
	if body.Address != nil {
		request.Address = *body.Address
	}
	if body.Description != nil {
		request.Description = *body.Description
	}
	if body.CustomerUserId != nil {
		value := body.CustomerUserId.String()
		request.CustomerUserID = &value
	}
	if body.AdminUserIds != nil {
		for _, value := range *body.AdminUserIds {
			request.AdminUserIDs = append(request.AdminUserIDs, value.String())
		}
	}
	if body.PlannedStartOn != nil {
		value := body.PlannedStartOn.Time
		request.PlannedStartOn = &value
	}
	if body.PlannedFinishOn != nil {
		value := body.PlannedFinishOn.Time
		request.PlannedFinishOn = &value
	}
	view, err := endpoint.projects.Create(r.Context(), actor, request)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+view.ID)
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(view.Version, 10)))
	writeAccountJSON(w, http.StatusCreated, projectResponse(view))
}

func (endpoint *AccountEndpoint) GetProject(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	view, err := endpoint.projects.Get(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(view.Version, 10)))
	writeAccountJSON(w, http.StatusOK, projectResponse(view))
}

func (endpoint *AccountEndpoint) UpdateProject(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.UpdateProjectParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	version, err := parseExpectedProjectVersion(string(params.IfMatch))
	if err != nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Некорректная версия проекта", requestID)
		return
	}
	var body projectUpdateBody
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля проекта", requestID)
		return
	}
	request, ok := projectUpdateRequest(body)
	if !ok {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля проекта", requestID)
		return
	}
	view, err := endpoint.projects.Update(r.Context(), actor, projectID.String(), requestID, version, request)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(view.Version, 10)))
	writeAccountJSON(w, http.StatusOK, projectResponse(view))
}

func (endpoint *AccountEndpoint) DeleteProject(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.DeleteProjectParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	version, err := parseExpectedProjectVersion(string(params.IfMatch))
	if err != nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Некорректная версия проекта", requestID)
		return
	}
	if err := endpoint.projects.Archive(r.Context(), actor, projectID.String(), requestID, version); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) AssignProjectCustomer(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.AssignProjectCustomerParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	version, err := parseExpectedProjectVersion(string(params.IfMatch))
	if err != nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Некорректная версия проекта", requestID)
		return
	}
	var body accountgenerated.AssignProjectCustomerJSONBody
	if !decodeAccountJSON(r, &body) || body.CustomerUserId == uuid.Nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Выберите заказчика", requestID)
		return
	}
	view, err := endpoint.projects.AssignCustomer(r.Context(), actor, projectID.String(), body.CustomerUserId.String(), requestID, version)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(view.Version, 10)))
	writeAccountJSON(w, http.StatusOK, projectResponse(view))
}

func (endpoint *AccountEndpoint) ListProjectMembers(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	list, err := endpoint.projects.ListMembers(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectMember, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, projectMemberResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectMemberList{Items: items, CanManage: list.CanManage})
}

func (endpoint *AccountEndpoint) SearchProjectMemberCandidates(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.SearchProjectMemberCandidatesParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	items, err := endpoint.projects.SearchMemberCandidates(r.Context(), actor, projectID.String(), params.Query)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	response := make([]accountgenerated.ProjectMemberCandidate, 0, len(items))
	for _, item := range items {
		response = append(response, projectMemberCandidateResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectMemberCandidateList{Items: response})
}

func (endpoint *AccountEndpoint) AddProjectMember(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.AddProjectMemberRequest
	if !decodeAccountJSON(r, &body) || body.UserId == uuid.Nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Выберите участника", requestID)
		return
	}
	member, err := endpoint.projects.AddMember(r.Context(), actor, projectID.String(), body.UserId.String(), requestID)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, projectMemberResponse(member))
}

func (endpoint *AccountEndpoint) RemoveProjectMember(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, userID openapi_types.UUID) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	if err := endpoint.projects.RemoveMember(r.Context(), actor, projectID.String(), userID.String(), requestID); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) GetProjectFinanceSummary(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	summary, err := endpoint.projects.FinanceSummary(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectFinanceSummary{
		CurrencyCode: summary.CurrencyCode, ConfirmedIncomeMinor: summary.ConfirmedIncomeMinor,
		ConfirmedExpenseMinor: summary.ConfirmedExpenseMinor, AvailableBalanceMinor: summary.AvailableBalanceMinor,
		PendingExpenseMinor: summary.PendingExpenseMinor, CanCreateExpense: summary.CanCreateExpense,
	})
}

func (endpoint *AccountEndpoint) ListProjectExpenses(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	list, err := endpoint.projects.ListExpenses(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectExpense, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, projectExpenseResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectExpenseList{Items: items, CanCreateExpense: list.CanCreateExpense})
}

func (endpoint *AccountEndpoint) CreateProjectExpense(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.CreateProjectExpenseParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectExpenseRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля расхода", requestID)
		return
	}
	request := project.CreateExpenseRequest{AmountMinor: body.AmountMinor, Category: string(body.Category), Description: body.Description, VendorName: body.VendorName, IdempotencyKey: params.IdempotencyKey}
	if body.PlannedPaymentOn != nil {
		value := body.PlannedPaymentOn.Time
		request.PlannedPaymentOn = &value
	}
	item, err := endpoint.projects.CreateExpense(r.Context(), actor, projectID.String(), requestID, request)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+projectID.String()+"/expenses/"+item.ID)
	writeAccountJSON(w, http.StatusCreated, projectExpenseResponse(item))
}

func (endpoint *AccountEndpoint) ListProjectMaterials(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	list, err := endpoint.projects.ListMaterials(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectMaterial, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, projectMaterialResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectMaterialList{Items: items, CanCreate: list.CanCreate, CanViewFinancialInformation: list.CanViewFinancialInformation})
}

func (endpoint *AccountEndpoint) CreateProjectMaterial(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.ProjectMaterialInput
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля материала", requestID)
		return
	}
	item, err := endpoint.projects.CreateMaterial(r.Context(), actor, projectID.String(), requestID, materialInput(body))
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+projectID.String()+"/materials/"+item.ID)
	writeAccountJSON(w, http.StatusCreated, projectMaterialResponse(item))
}

func (endpoint *AccountEndpoint) UpdateProjectMaterial(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, materialID openapi_types.UUID, params accountgenerated.UpdateProjectMaterialParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	version, err := parseExpectedProjectVersion(string(params.IfMatch))
	if err != nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Некорректная версия материала", requestID)
		return
	}
	var body accountgenerated.ProjectMaterialInput
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля материала", requestID)
		return
	}
	item, err := endpoint.projects.UpdateMaterial(r.Context(), actor, projectID.String(), materialID.String(), requestID, version, materialInput(body))
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, projectMaterialResponse(item))
}

func (endpoint *AccountEndpoint) DeleteProjectMaterial(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, materialID openapi_types.UUID, params accountgenerated.DeleteProjectMaterialParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	version, err := parseExpectedProjectVersion(string(params.IfMatch))
	if err != nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Некорректная версия материала", requestID)
		return
	}
	var body accountgenerated.DeleteProjectMaterialRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Укажите причину удаления", requestID)
		return
	}
	if err = endpoint.projects.DeleteMaterial(r.Context(), actor, projectID.String(), materialID.String(), requestID, version, body.Reason); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) GetProjectChat(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.GetProjectChatParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
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
	page, err := endpoint.projects.ListChat(r.Context(), actor, projectID.String(), pageSize, before)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, projectChatPageResponse(page))
}

func (endpoint *AccountEndpoint) ListProjectChats(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	list, err := endpoint.projects.ListChats(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectChatSummary, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, projectChatSummaryResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectChatList{Items: items, CanCreate: list.CanCreate})
}

func (endpoint *AccountEndpoint) CreateProjectChat(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectChatRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Укажите название чата", requestID)
		return
	}
	members := []string{}
	if body.MemberUserIds != nil {
		for _, value := range *body.MemberUserIds {
			members = append(members, value.String())
		}
	}
	chat, err := endpoint.projects.CreateChat(r.Context(), actor, projectID.String(), project.CreateChatRequest{Name: body.Name, MemberUserIDs: members, RequestID: requestID})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, projectChatSummaryResponse(chat))
}

func (endpoint *AccountEndpoint) UpdateProjectChat(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.UpdateProjectChatRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Укажите название чата", requestID)
		return
	}
	chat, err := endpoint.projects.UpdateChat(r.Context(), actor, projectID.String(), chatID.String(), body.Name, requestID, body.Version)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, projectChatSummaryResponse(chat))
}

func (endpoint *AccountEndpoint) DeleteProjectChat(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId, params accountgenerated.DeleteProjectChatParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	if err := endpoint.projects.DeleteChat(r.Context(), actor, projectID.String(), chatID.String(), requestID, params.Version); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) ListProjectChatMembers(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	list, err := endpoint.projects.ListChatMembers(r.Context(), actor, projectID.String(), chatID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectChatMember, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, projectChatMemberResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectChatMemberList{Items: items, CanManage: list.CanManage})
}

func (endpoint *AccountEndpoint) AddProjectChatMember(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.ProjectChatMemberRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Выберите участника", requestID)
		return
	}
	member, err := endpoint.projects.AddChatMember(r.Context(), actor, projectID.String(), chatID.String(), body.UserId.String(), requestID)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, projectChatMemberResponse(member))
}

func (endpoint *AccountEndpoint) RemoveProjectChatMember(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId, userID openapi_types.UUID) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	if err := endpoint.projects.RemoveChatMember(r.Context(), actor, projectID.String(), chatID.String(), userID.String(), requestID); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) MarkProjectChatRead(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.MarkProjectChatReadRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Не удалось отметить сообщения прочитанными", requestID)
		return
	}
	if err := endpoint.projects.MarkChatRead(r.Context(), actor, projectID.String(), chatID.String(), body.MessageId.String()); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) ListProjectDocuments(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	list, err := endpoint.projects.ListDocuments(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectDocument, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, projectDocumentResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectDocumentList{Items: items, CanUpload: list.CanUpload})
}

func (endpoint *AccountEndpoint) UploadProjectDocument(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.UploadProjectDocumentParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	if r.ContentLength < 1 || r.ContentLength > project.MaxProjectDocumentBytes {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Файл должен быть размером не более 10 МБ", requestID)
		return
	}
	mediaType := r.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	document, err := endpoint.projects.UploadDocument(r.Context(), actor, projectID.String(), params.XFileName, mediaType, requestID, r.Body, r.ContentLength)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, projectDocumentResponse(document))
}

func (endpoint *AccountEndpoint) DownloadProjectDocument(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, documentID openapi_types.UUID) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	document, err := endpoint.projects.DownloadDocument(r.Context(), actor, projectID.String(), documentID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("Content-Type", document.MediaType)
	w.Header().Set("Content-Length", strconv.FormatInt(document.SizeBytes, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": document.Name}))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(document.Content)
}

func (endpoint *AccountEndpoint) DeleteProjectDocument(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, documentID openapi_types.UUID) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	if err := endpoint.projects.DeleteDocument(r.Context(), actor, projectID.String(), documentID.String(), requestID); err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (endpoint *AccountEndpoint) CreateProjectChatMessage(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectChatMessageRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Введите сообщение", requestID)
		return
	}
	message, err := endpoint.projects.SendChatMessage(r.Context(), actor, projectID.String(), project.CreateChatMessageRequest{
		Body: body.Body, ClientMessageID: body.ClientMessageId.String(), RequestID: requestID,
	})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, projectChatMessageResponse(message))
}

func (endpoint *AccountEndpoint) OpenProjectContextChat(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.OpenProjectContextChatRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Укажите название и контекст обсуждения", requestID)
		return
	}
	chat, err := endpoint.projects.OpenContextChat(r.Context(), actor, projectID.String(), project.OpenContextChatRequest{
		ContextType: string(body.ContextType), ContextID: body.ContextId.String(), Name: body.Name, RequestID: requestID,
	})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, projectChatContextResponse(chat))
}

func (endpoint *AccountEndpoint) GetProjectContextChat(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId, params accountgenerated.GetProjectContextChatParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
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
	page, err := endpoint.projects.ListContextChat(r.Context(), actor, projectID.String(), chatID.String(), pageSize, before)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, projectChatPageResponse(page))
}

func (endpoint *AccountEndpoint) CreateProjectContextChatMessage(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, chatID accountgenerated.ChatId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectChatMessageRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Введите сообщение", requestID)
		return
	}
	message, err := endpoint.projects.SendContextChatMessage(r.Context(), actor, projectID.String(), chatID.String(), project.CreateChatMessageRequest{
		Body: body.Body, ClientMessageID: body.ClientMessageId.String(), RequestID: requestID,
	})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusCreated, projectChatMessageResponse(message))
}

func (endpoint *AccountEndpoint) ListProjectUpcomingItems(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.ListProjectUpcomingItemsParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	feed, err := endpoint.projects.ListUpcoming(r.Context(), actor, projectID.String(), params.Days)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectUpcomingItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		items = append(items, projectUpcomingResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectUpcomingFeed{Days: feed.Days, RangeStart: feed.RangeStart, RangeEnd: feed.RangeEnd, Items: items})
}

func (endpoint *AccountEndpoint) ListProjectCalendar(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, params accountgenerated.ListProjectCalendarParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	feed, err := endpoint.projects.ListCalendar(r.Context(), actor, projectID.String(), params.From, params.To)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectUpcomingItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		items = append(items, projectUpcomingResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectCalendarFeed{RangeStart: feed.RangeStart, RangeEnd: feed.RangeEnd, Items: items})
}

func (endpoint *AccountEndpoint) ListGlobalCalendar(w http.ResponseWriter, r *http.Request, params accountgenerated.ListGlobalCalendarParams) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	projectIDs := make([]string, 0)
	if params.ProjectId != nil {
		projectIDs = make([]string, 0, len(*params.ProjectId))
		for _, projectID := range *params.ProjectId {
			projectIDs = append(projectIDs, projectID.String())
		}
	}
	feed, err := endpoint.projects.ListGlobalCalendar(r.Context(), actor, params.From, params.To, projectIDs)
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	projects := make([]accountgenerated.GlobalCalendarProject, 0, len(feed.Projects))
	for _, lane := range feed.Projects {
		items := make([]accountgenerated.ProjectUpcomingItem, 0, len(lane.Items))
		for _, item := range lane.Items {
			items = append(items, projectUpcomingResponse(item))
		}
		response := accountgenerated.GlobalCalendarProject{Id: uuid.MustParse(lane.ID), Name: lane.Name, Status: accountgenerated.ProjectStatus(lane.Status), Items: items}
		if lane.PlannedStartOn != nil {
			response.PlannedStartOn = &openapi_types.Date{Time: *lane.PlannedStartOn}
		}
		if lane.PlannedFinishOn != nil {
			response.PlannedFinishOn = &openapi_types.Date{Time: *lane.PlannedFinishOn}
		}
		projects = append(projects, response)
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.GlobalCalendarFeed{RangeStart: feed.RangeStart, RangeEnd: feed.RangeEnd, Projects: projects, HasMoreProjects: feed.HasMoreProjects, TruncatedEvents: feed.TruncatedEvents})
}

func (endpoint *AccountEndpoint) CreateProjectTask(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectTaskRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля задачи", requestID)
		return
	}
	description := ""
	if body.Description != nil {
		description = *body.Description
	}
	assignees := make([]string, 0)
	if body.AssigneeUserIds != nil {
		for _, assignee := range *body.AssigneeUserIds {
			assignees = append(assignees, assignee.String())
		}
	}
	item, err := endpoint.projects.CreateTask(r.Context(), actor, projectID.String(), project.CreateTaskRequest{Title: body.Title, Description: description, AssigneeUserIDs: assignees, DueAt: body.DueAt, RequestID: requestID})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+projectID.String()+"/tasks/"+item.ID)
	writeAccountJSON(w, http.StatusCreated, projectUpcomingResponse(item))
}

func (endpoint *AccountEndpoint) ListProjectTasks(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	list, err := endpoint.projects.ListTasks(r.Context(), actor, projectID.String())
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	items := make([]accountgenerated.ProjectTask, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, projectTaskResponse(item))
	}
	writeAccountJSON(w, http.StatusOK, accountgenerated.ProjectTaskList{Items: items, CanCreate: list.CanCreate})
}

func (endpoint *AccountEndpoint) UpdateProjectTask(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId, taskID openapi_types.UUID, params accountgenerated.UpdateProjectTaskParams) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.UpdateProjectTaskRequest
	if !decodeAccountJSON(r, &body) || body.Status == nil {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте данные задачи", requestID)
		return
	}
	item, err := endpoint.projects.UpdateTask(r.Context(), actor, projectID.String(), taskID.String(), project.UpdateTaskRequest{Status: string(*body.Status), ExpectedVersion: params.IfMatch, RequestID: requestID})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	writeAccountJSON(w, http.StatusOK, projectTaskResponse(item))
}

func (endpoint *AccountEndpoint) CreateProjectMeeting(w http.ResponseWriter, r *http.Request, projectID accountgenerated.ProjectId) {
	requestID := accountRequestID(w, r)
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	actor, ok := endpoint.projectActor(w, r, requestID)
	if !ok {
		return
	}
	var body accountgenerated.CreateProjectMeetingRequest
	if !decodeAccountJSON(r, &body) {
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте поля встречи", requestID)
		return
	}
	description, location := "", ""
	if body.Description != nil {
		description = *body.Description
	}
	if body.Location != nil {
		location = *body.Location
	}
	item, err := endpoint.projects.CreateMeeting(r.Context(), actor, projectID.String(), project.CreateMeetingRequest{Title: body.Title, Description: description, Location: location, StartsAt: body.StartsAt, EndsAt: body.EndsAt, RequestID: requestID})
	if err != nil {
		endpoint.writeProjectError(w, err, requestID)
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+projectID.String()+"/meetings/"+item.ID)
	writeAccountJSON(w, http.StatusCreated, projectUpcomingResponse(item))
}

func (endpoint *AccountEndpoint) projectActor(w http.ResponseWriter, r *http.Request, requestID string) (project.Actor, bool) {
	if endpoint.projects == nil {
		writeAccountError(w, http.StatusServiceUnavailable, "service_unavailable", "Сервис проектов временно недоступен", requestID)
		return project.Actor{}, false
	}
	view, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return project.Actor{}, false
	}
	return project.Actor{UserID: view.ID, SuperAdmin: view.IsGlobalAdministrator(), Login: view.Login, FirstName: view.FirstName, LastName: view.LastName}, true
}

func parseExpectedProjectVersion(value string) (int64, error) {
	return strconv.ParseInt(strings.Trim(value, "\""), 10, 64)
}

func projectUpdateRequest(body projectUpdateBody) (project.UpdateRequest, bool) {
	request := project.UpdateRequest{}
	if body.Name.Set {
		if body.Name.Value == nil {
			return project.UpdateRequest{}, false
		}
		request.Name = body.Name.Value
	}
	if body.Type.Set {
		if body.Type.Value == nil {
			return project.UpdateRequest{}, false
		}
		request.Type = body.Type.Value
	}
	if body.Status.Set {
		if body.Status.Value == nil {
			return project.UpdateRequest{}, false
		}
		request.Status = body.Status.Value
	}
	if body.AutoApproveExpenses.Set {
		if body.AutoApproveExpenses.Value == nil {
			return project.UpdateRequest{}, false
		}
		request.AutoApproveExpenses = body.AutoApproveExpenses.Value
	}
	request.Address = project.Optional[*string]{Set: body.Address.Set, Value: body.Address.Value}
	request.Description = project.Optional[*string]{Set: body.Description.Set, Value: body.Description.Value}
	start, ok := optionalProjectDate(body.PlannedStartOn)
	if !ok {
		return project.UpdateRequest{}, false
	}
	finish, ok := optionalProjectDate(body.PlannedFinishOn)
	if !ok {
		return project.UpdateRequest{}, false
	}
	request.PlannedStartOn, request.PlannedFinishOn = start, finish
	return request, true
}

func optionalProjectDate(field optionalProjectField[string]) (project.Optional[*time.Time], bool) {
	result := project.Optional[*time.Time]{Set: field.Set}
	if !field.Set || field.Value == nil {
		return result, true
	}
	value, err := time.Parse("2006-01-02", *field.Value)
	if err != nil {
		return project.Optional[*time.Time]{}, false
	}
	result.Value = &value
	return result, true
}

func (endpoint *AccountEndpoint) writeProjectError(w http.ResponseWriter, err error, requestID string) {
	switch {
	case errors.Is(err, project.ErrInvalidInput):
		writeAccountError(w, http.StatusBadRequest, "validation_error", "Проверьте данные проекта", requestID)
	case errors.Is(err, project.ErrForbidden):
		writeAccountError(w, http.StatusForbidden, "forbidden", "Недостаточно прав", requestID)
	case errors.Is(err, project.ErrNotFound):
		writeAccountError(w, http.StatusNotFound, "not_found", "Проект не найден", requestID)
	case errors.Is(err, project.ErrConflict):
		writeAccountError(w, http.StatusConflict, "conflict", "Проект уже изменён. Обновите страницу", requestID)
	case errors.Is(err, project.ErrAccountUnavailable):
		writeAccountError(w, http.StatusConflict, "account_unavailable", "Пользователь недоступен", requestID)
	default:
		slog.Error("project request failed", "request_id", requestID, "error", err)
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось выполнить запрос", requestID)
	}
}

func projectResponse(view project.View) accountgenerated.ProjectView {
	response := accountgenerated.ProjectView{
		Id: uuid.MustParse(view.ID), Name: view.Name, Type: view.Type,
		Address: view.Address, Status: accountgenerated.ProjectStatus(view.Status), CurrencyCode: view.CurrencyCode,
		Description: view.Description, AutoApproveExpenses: view.AutoApproveExpenses,
		CreatedByUserId: uuid.MustParse(view.CreatedByUserID), CreatedAt: view.CreatedAt, Version: view.Version,
	}
	if view.CustomerUserID != nil {
		value := openapi_types.UUID(uuid.MustParse(*view.CustomerUserID))
		response.CustomerUserId = &value
	}
	if view.AdminUserIDs != nil {
		values := make([]openapi_types.UUID, 0, len(view.AdminUserIDs))
		for _, value := range view.AdminUserIDs {
			values = append(values, openapi_types.UUID(uuid.MustParse(value)))
		}
		response.AdminUserIds = &values
	}
	if view.PlannedStartOn != nil {
		value := openapi_types.Date{Time: *view.PlannedStartOn}
		response.PlannedStartOn = &value
	}
	if view.PlannedFinishOn != nil {
		value := openapi_types.Date{Time: *view.PlannedFinishOn}
		response.PlannedFinishOn = &value
	}
	return response
}

func projectUpcomingResponse(item project.UpcomingItem) accountgenerated.ProjectUpcomingItem {
	return accountgenerated.ProjectUpcomingItem{
		Id: uuid.MustParse(item.ID), ProjectId: uuid.MustParse(item.ProjectID), Kind: accountgenerated.ProjectUpcomingKind(item.Kind),
		Title: item.Title, Description: item.Description, Status: item.Status, Location: item.Location,
		EffectiveAt: item.EffectiveAt, EndsAt: item.EndsAt, CreatedAt: item.CreatedAt, Version: item.Version,
	}
}

func projectTaskResponse(item project.Task) accountgenerated.ProjectTask {
	assignees := make([]accountgenerated.ProjectTaskAssignee, 0, len(item.Assignees))
	for _, assignee := range item.Assignees {
		assignees = append(assignees, accountgenerated.ProjectTaskAssignee{UserId: uuid.MustParse(assignee.UserID), Login: assignee.Login, FirstName: assignee.FirstName, LastName: assignee.LastName})
	}
	response := accountgenerated.ProjectTask{
		Id: uuid.MustParse(item.ID), ProjectId: uuid.MustParse(item.ProjectID), Title: item.Title,
		Description: item.Description, Status: accountgenerated.ProjectTaskStatus(item.Status), DueAt: item.DueAt,
		StartedAt: item.StartedAt, CompletedAt: item.CompletedAt, Assignees: assignees,
		CanEdit: item.CanEdit, CanChangeStatus: item.CanChangeStatus, CreatedAt: item.CreatedAt, Version: item.Version,
	}
	if item.ContextChatID != nil {
		value := openapi_types.UUID(uuid.MustParse(*item.ContextChatID))
		response.ContextChatId = &value
	}
	return response
}

func projectMemberResponse(member project.Member) accountgenerated.ProjectMember {
	roles := make([]accountgenerated.ProjectRole, 0, len(member.ProjectRoles))
	for _, role := range member.ProjectRoles {
		roles = append(roles, accountgenerated.ProjectRole(role))
	}
	return accountgenerated.ProjectMember{
		UserId: uuid.MustParse(member.UserID), Login: member.Login, Email: openapi_types.Email(member.Email),
		FirstName: member.FirstName, LastName: member.LastName, MiddleName: member.MiddleName,
		ProfessionalRole: accountgenerated.ProfessionalRole{Code: member.ProfessionalRoleCode, Name: member.ProfessionalRoleName},
		ProjectRoles:     roles, JoinedAt: member.JoinedAt, Removable: member.Removable,
	}
}

func projectChatPageResponse(page project.ProjectChatPage) accountgenerated.ProjectChatPage {
	messages := make([]accountgenerated.ProjectChatMessage, 0, len(page.Messages))
	for _, message := range page.Messages {
		messages = append(messages, projectChatMessageResponse(message))
	}
	response := accountgenerated.ProjectChatPage{
		ChatId: uuid.MustParse(page.ChatID), ProjectId: uuid.MustParse(page.ProjectID), CurrentUserId: uuid.MustParse(page.CurrentUserID),
		Messages: messages, HasMore: page.HasMore, CanSend: page.CanSend,
	}
	if page.NextCursor != nil {
		value := openapi_types.UUID(uuid.MustParse(*page.NextCursor))
		response.NextCursor = &value
	}
	if page.Context != nil {
		value := projectChatContextResponse(*page.Context)
		response.Context = &value
	}
	return response
}

func projectChatSummaryResponse(value project.ChatSummary) accountgenerated.ProjectChatSummary {
	response := accountgenerated.ProjectChatSummary{Id: uuid.MustParse(value.ID), ProjectId: uuid.MustParse(value.ProjectID), Kind: accountgenerated.ProjectChatSummaryKind(value.Kind), Name: value.Name, MemberCount: value.MemberCount, UnreadCount: value.UnreadCount, CreatedAt: value.CreatedAt, CanManage: value.CanManage, Version: value.Version}
	if value.ContextType != nil {
		typed := accountgenerated.ProjectChatContextType(*value.ContextType)
		response.ContextType = &typed
	}
	if value.ContextID != nil {
		id := openapi_types.UUID(uuid.MustParse(*value.ContextID))
		response.ContextId = &id
	}
	response.ContextTitle = value.ContextTitle
	return response
}

func projectChatMemberResponse(value project.ChatMember) accountgenerated.ProjectChatMember {
	return accountgenerated.ProjectChatMember{UserId: uuid.MustParse(value.UserID), Login: value.Login, FirstName: value.FirstName, LastName: value.LastName, JoinedAt: value.JoinedAt, Removable: value.Removable}
}

func projectDocumentResponse(value project.ProjectDocument) accountgenerated.ProjectDocument {
	return accountgenerated.ProjectDocument{Id: uuid.MustParse(value.ID), ProjectId: uuid.MustParse(value.ProjectID), Name: value.Name, MediaType: value.MediaType, SizeBytes: value.SizeBytes, UploadedByUserId: uuid.MustParse(value.UploadedByUserID), CreatedAt: value.CreatedAt, CanDelete: value.CanDelete, Version: value.Version}
}

func projectChatContextResponse(value project.ChatContext) accountgenerated.ProjectChatContext {
	return accountgenerated.ProjectChatContext{
		ChatId: uuid.MustParse(value.ChatID), ProjectId: uuid.MustParse(value.ProjectID),
		ContextType: accountgenerated.ProjectChatContextType(value.ContextType), ContextId: uuid.MustParse(value.ContextID),
		ContextTitle: value.ContextTitle, Name: value.Name,
	}
}

func projectChatMessageResponse(message project.ChatMessage) accountgenerated.ProjectChatMessage {
	return accountgenerated.ProjectChatMessage{
		Id: uuid.MustParse(message.ID), ChatId: uuid.MustParse(message.ChatID), ProjectId: uuid.MustParse(message.ProjectID),
		Body: message.Body, CreatedAt: message.CreatedAt, EditedAt: message.EditedAt, DeletedAt: message.DeletedAt, Version: message.Version,
		Author: accountgenerated.ProjectChatAuthor{
			UserId: uuid.MustParse(message.Author.UserID), Login: message.Author.Login,
			FirstName: message.Author.FirstName, LastName: message.Author.LastName,
		},
	}
}

func projectMemberCandidateResponse(candidate project.MemberCandidate) accountgenerated.ProjectMemberCandidate {
	return accountgenerated.ProjectMemberCandidate{
		UserId: uuid.MustParse(candidate.UserID), Login: candidate.Login, Email: openapi_types.Email(candidate.Email),
		FirstName: candidate.FirstName, LastName: candidate.LastName, MiddleName: candidate.MiddleName,
		ProfessionalRole: accountgenerated.ProfessionalRole{Code: candidate.ProfessionalRoleCode, Name: candidate.ProfessionalRoleName},
	}
}

func projectExpenseResponse(expense project.Expense) accountgenerated.ProjectExpense {
	response := accountgenerated.ProjectExpense{
		Id: uuid.MustParse(expense.ID), ProjectId: uuid.MustParse(expense.ProjectID), AmountMinor: expense.AmountMinor,
		CurrencyCode: expense.CurrencyCode, Category: accountgenerated.ExpenseCategory(expense.Category), Description: expense.Description,
		VendorName: expense.VendorName, Status: accountgenerated.ExpenseStatus(expense.Status), CreatedByUserId: uuid.MustParse(expense.CreatedByUserID),
		CreatedAt: expense.CreatedAt, Version: expense.Version,
	}
	if expense.PlannedPaymentOn != nil {
		value := openapi_types.Date{Time: *expense.PlannedPaymentOn}
		response.PlannedPaymentOn = &value
	}
	if expense.ContextChatID != nil {
		value := openapi_types.UUID(uuid.MustParse(*expense.ContextChatID))
		response.ContextChatId = &value
	}
	return response
}

func materialInput(body accountgenerated.ProjectMaterialInput) project.MaterialInput {
	input := project.MaterialInput{Category: string(body.Category), Name: body.Name, SupplierName: body.SupplierName, ContactInfo: body.ContactInfo, ContractReference: body.ContractReference, ContractAmountMinor: body.ContractAmountMinor, PaymentStatus: string(body.PaymentStatus), DeliveryStatus: string(body.DeliveryStatus), Notes: body.Notes}
	if body.LinkedExpenseId != nil {
		value := body.LinkedExpenseId.String()
		input.LinkedExpenseID = &value
	}
	if body.PlannedDeliveryOn != nil {
		value := body.PlannedDeliveryOn.Time
		input.PlannedDeliveryOn = &value
	}
	if body.ActualDeliveryOn != nil {
		value := body.ActualDeliveryOn.Time
		input.ActualDeliveryOn = &value
	}
	if body.InstallationOn != nil {
		value := body.InstallationOn.Time
		input.InstallationOn = &value
	}
	return input
}

func projectMaterialResponse(item project.Material) accountgenerated.ProjectMaterial {
	response := accountgenerated.ProjectMaterial{Id: uuid.MustParse(item.ID), ProjectId: uuid.MustParse(item.ProjectID), Category: accountgenerated.MaterialCategory(item.Category), Name: item.Name, SupplierName: item.SupplierName, ContactInfo: item.ContactInfo, ContractReference: item.ContractReference, ContractAmountMinor: item.ContractAmountMinor, PaidAmountMinor: item.PaidAmountMinor, RemainingAmountMinor: item.RemainingAmountMinor, CurrencyCode: item.CurrencyCode, LinkedExpenseDescription: item.LinkedExpenseDescription, PaymentStatus: accountgenerated.MaterialPaymentStatus(item.PaymentStatus), DeliveryStatus: accountgenerated.MaterialDeliveryStatus(item.DeliveryStatus), Notes: item.Notes, CreatedByUserId: uuid.MustParse(item.CreatedByUserID), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Version: item.Version, CanEdit: item.CanEdit, CanDelete: item.CanDelete}
	if item.LinkedExpenseID != nil {
		value := uuid.MustParse(*item.LinkedExpenseID)
		response.LinkedExpenseId = &value
	}
	if item.PlannedDeliveryOn != nil {
		value := openapi_types.Date{Time: *item.PlannedDeliveryOn}
		response.PlannedDeliveryOn = &value
	}
	if item.ActualDeliveryOn != nil {
		value := openapi_types.Date{Time: *item.ActualDeliveryOn}
		response.ActualDeliveryOn = &value
	}
	if item.InstallationOn != nil {
		value := openapi_types.Date{Time: *item.InstallationOn}
		response.InstallationOn = &value
	}
	if item.ContextChatID != nil {
		value := openapi_types.UUID(uuid.MustParse(*item.ContextChatID))
		response.ContextChatId = &value
	}
	return response
}
