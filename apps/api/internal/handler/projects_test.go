package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type endpointProjectStore struct {
	create           project.CreateCommand
	view             project.View
	items            []project.View
	err              error
	assign           project.AssignCustomerCommand
	update           project.UpdateCommand
	archive          project.ArchiveCommand
	upcoming         []project.UpcomingItem
	globalProjects   []project.GlobalCalendarProject
	globalMore       bool
	globalTruncated  bool
	task             project.CreateTaskCommand
	tasks            []project.Task
	canCreateTask    bool
	updateTask       project.UpdateTaskCommand
	updatedTask      project.Task
	meeting          project.CreateMeetingCommand
	members          []project.Member
	manage           bool
	candidates       []project.MemberCandidate
	memberCommand    project.MemberCommand
	member           project.Member
	financeSummary   project.FinanceSummary
	expenses         []project.Expense
	canCreateExpense bool
	expenseCommand   project.CreateExpenseCommand
	expense          project.Expense
	materials        []project.Material
	material         project.Material
	materialCreate   project.CreateMaterialCommand
	materialUpdate   project.UpdateMaterialCommand
	materialDelete   project.DeleteMaterialCommand
	chatQuery        project.ChatQuery
	chatPage         project.ProjectChatPage
	chatCommand      project.CreateChatMessageCommand
	chatMessage      project.ChatMessage
	chatContext      project.ChatContext
	contextCommand   project.OpenContextChatCommand
	chats            project.ChatList
	createdChat      project.ChatSummary
	chatMembers      project.ChatMemberList
	chatMember       project.ChatMember
	documents        project.ProjectDocumentList
	document         project.ProjectDocument
	documentContent  project.ProjectDocumentContent
	notifications    project.AccountNotificationList
}

func (store *endpointProjectStore) CreateProject(_ context.Context, command project.CreateCommand) (project.View, error) {
	store.create = command
	return store.view, store.err
}
func (store *endpointProjectStore) ListProjects(context.Context, project.ListQuery) ([]project.View, error) {
	return store.items, store.err
}
func (store *endpointProjectStore) GetProject(context.Context, project.Actor, string) (project.View, error) {
	return store.view, store.err
}
func (store *endpointProjectStore) AssignProjectCustomer(_ context.Context, command project.AssignCustomerCommand) (project.View, error) {
	store.assign = command
	return store.view, store.err
}
func (store *endpointProjectStore) UpdateProject(_ context.Context, command project.UpdateCommand) (project.View, error) {
	store.update = command
	return store.view, store.err
}
func (store *endpointProjectStore) ArchiveProject(_ context.Context, command project.ArchiveCommand) error {
	store.archive = command
	return store.err
}
func (store *endpointProjectStore) ListProjectUpcoming(context.Context, project.UpcomingQuery) ([]project.UpcomingItem, error) {
	return store.upcoming, store.err
}
func (store *endpointProjectStore) ListGlobalCalendar(context.Context, project.GlobalCalendarQuery) ([]project.GlobalCalendarProject, bool, bool, error) {
	return store.globalProjects, store.globalMore, store.globalTruncated, store.err
}
func (store *endpointProjectStore) CreateProjectTask(_ context.Context, command project.CreateTaskCommand) (project.UpcomingItem, error) {
	store.task = command
	if len(store.upcoming) > 0 {
		return store.upcoming[0], store.err
	}
	return project.UpcomingItem{}, store.err
}
func (store *endpointProjectStore) ListProjectTasks(context.Context, project.TaskQuery) ([]project.Task, bool, error) {
	return store.tasks, store.canCreateTask, store.err
}
func (store *endpointProjectStore) UpdateProjectTask(_ context.Context, command project.UpdateTaskCommand) (project.Task, error) {
	store.updateTask = command
	return store.updatedTask, store.err
}
func (store *endpointProjectStore) CreateProjectMeeting(_ context.Context, command project.CreateMeetingCommand) (project.UpcomingItem, error) {
	store.meeting = command
	if len(store.upcoming) > 0 {
		return store.upcoming[0], store.err
	}
	return project.UpcomingItem{}, store.err
}
func (store *endpointProjectStore) ListProjectMembers(context.Context, project.Actor, string) ([]project.Member, bool, error) {
	return store.members, store.manage, store.err
}
func (store *endpointProjectStore) SearchProjectMemberCandidates(_ context.Context, _ project.MemberCandidateQuery) ([]project.MemberCandidate, error) {
	return store.candidates, store.err
}
func (store *endpointProjectStore) AddProjectMember(_ context.Context, command project.MemberCommand) (project.Member, error) {
	store.memberCommand = command
	return store.member, store.err
}
func (store *endpointProjectStore) RemoveProjectMember(_ context.Context, command project.MemberCommand) error {
	store.memberCommand = command
	return store.err
}
func (store *endpointProjectStore) GetProjectFinanceSummary(context.Context, project.Actor, string) (project.FinanceSummary, error) {
	return store.financeSummary, store.err
}
func (store *endpointProjectStore) ListProjectExpenses(context.Context, project.Actor, string) ([]project.Expense, bool, error) {
	return store.expenses, store.canCreateExpense, store.err
}
func (store *endpointProjectStore) CreateProjectExpense(_ context.Context, command project.CreateExpenseCommand) (project.Expense, error) {
	store.expenseCommand = command
	return store.expense, store.err
}
func (store *endpointProjectStore) ListProjectMaterials(context.Context, project.Actor, string) ([]project.Material, bool, bool, error) {
	return store.materials, true, true, store.err
}
func (store *endpointProjectStore) CreateProjectMaterial(_ context.Context, command project.CreateMaterialCommand) (project.Material, error) {
	store.materialCreate = command
	return store.material, store.err
}
func (store *endpointProjectStore) UpdateProjectMaterial(_ context.Context, command project.UpdateMaterialCommand) (project.Material, error) {
	store.materialUpdate = command
	return store.material, store.err
}
func (store *endpointProjectStore) DeleteProjectMaterial(_ context.Context, command project.DeleteMaterialCommand) error {
	store.materialDelete = command
	return store.err
}
func (store *endpointProjectStore) ListProjectChat(_ context.Context, query project.ChatQuery) (project.ProjectChatPage, error) {
	store.chatQuery = query
	return store.chatPage, store.err
}
func (store *endpointProjectStore) CreateProjectChatMessage(_ context.Context, command project.CreateChatMessageCommand) (project.ChatMessage, error) {
	store.chatCommand = command
	return store.chatMessage, store.err
}
func (store *endpointProjectStore) OpenProjectContextChat(_ context.Context, command project.OpenContextChatCommand) (project.ChatContext, error) {
	store.contextCommand = command
	return store.chatContext, store.err
}
func (store *endpointProjectStore) ListProjectContextChat(_ context.Context, query project.ChatQuery) (project.ProjectChatPage, error) {
	store.chatQuery = query
	return store.chatPage, store.err
}
func (store *endpointProjectStore) CreateProjectContextChatMessage(_ context.Context, command project.CreateChatMessageCommand) (project.ChatMessage, error) {
	store.chatCommand = command
	return store.chatMessage, store.err
}
func (store *endpointProjectStore) ListProjectChats(context.Context, project.Actor, string) (project.ChatList, error) {
	return store.chats, store.err
}
func (store *endpointProjectStore) CreateProjectChat(context.Context, project.CreateChatCommand) (project.ChatSummary, error) {
	return store.createdChat, store.err
}
func (store *endpointProjectStore) UpdateProjectChat(context.Context, project.UpdateChatCommand) (project.ChatSummary, error) {
	return store.createdChat, store.err
}
func (store *endpointProjectStore) DeleteProjectChat(context.Context, project.DeleteChatCommand) error {
	return store.err
}
func (store *endpointProjectStore) ListProjectChatMembers(context.Context, project.Actor, string, string) (project.ChatMemberList, error) {
	return store.chatMembers, store.err
}
func (store *endpointProjectStore) AddProjectChatMember(context.Context, project.ChatMemberCommand) (project.ChatMember, error) {
	return store.chatMember, store.err
}
func (store *endpointProjectStore) RemoveProjectChatMember(context.Context, project.ChatMemberCommand) error {
	return store.err
}
func (store *endpointProjectStore) MarkProjectChatRead(context.Context, project.MarkChatReadCommand) error {
	return store.err
}
func (store *endpointProjectStore) ListAccountNotifications(context.Context, project.Actor, int) (project.AccountNotificationList, error) {
	return store.notifications, store.err
}
func (store *endpointProjectStore) MarkAccountNotificationRead(context.Context, project.Actor, string, time.Time) error {
	return store.err
}
func (store *endpointProjectStore) MarkAllAccountNotificationsRead(context.Context, project.Actor, time.Time) error {
	return store.err
}
func (store *endpointProjectStore) ListProjectDocuments(context.Context, project.Actor, string) (project.ProjectDocumentList, error) {
	return store.documents, store.err
}
func (store *endpointProjectStore) UploadProjectDocument(context.Context, project.UploadProjectDocumentCommand) (project.ProjectDocument, error) {
	return store.document, store.err
}
func (store *endpointProjectStore) DownloadProjectDocument(context.Context, project.Actor, string, string) (project.ProjectDocumentContent, error) {
	return store.documentContent, store.err
}
func (store *endpointProjectStore) DeleteProjectDocument(context.Context, project.DeleteProjectDocumentCommand) error {
	return store.err
}

const endpointActorID = "00000000-0000-0000-0000-000000000001"
const endpointProjectID = "00000000-0000-0000-0000-000000000010"
const endpointMemberID = "00000000-0000-0000-0000-000000000020"

func projectEndpointForTest(t *testing.T, accountStore *endpointAccountStore, projectStore *endpointProjectStore) *AccountEndpoint {
	t.Helper()
	endpoint := accountEndpointForTest(t, accountStore)
	service, err := project.NewService(projectStore, []byte(strings.Repeat("p", 32)), func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	endpoint.ConfigureProjects(service)
	return endpoint
}

func activeAccount(globalRole *string) *endpointAccountStore {
	return &endpointAccountStore{accessFound: true, accessAccount: account.AccountView{
		ID: endpointActorID, Login: "svetlana", Email: "svetlana@example.com", FirstName: "Светлана",
		ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер", Status: "active", GlobalRole: globalRole, Version: 1,
	}}
}

func projectView() project.View {
	return project.View{ID: endpointProjectID, Name: "Полянка", Type: "interior_design", Status: "draft", CurrencyCode: "RUB", CreatedByUserID: endpointActorID, CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), Version: 1, AdminUserIDs: []string{endpointActorID}}
}

func authenticatedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://designer-svetlana.ru")
	request.Header.Set("X-CSRF-Token", strings.Repeat("c", 43))
	request.Header.Set("X-Request-ID", "request-project")
	request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_session", Value: strings.Repeat("a", 43)})
	request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: strings.Repeat("c", 43)})
	return request
}

func TestProjectEndpointsCreateListGetAndAssign(t *testing.T) {
	projectStore := &endpointProjectStore{view: projectView(), items: []project.View{projectView()}}
	endpoint := projectEndpointForTest(t, activeAccount(nil), projectStore)

	create := httptest.NewRecorder()
	endpoint.CreateProject(create, authenticatedRequest(http.MethodPost, "/api/v1/projects", `{"name":"Полянка","type":"interior_design","currencyCode":"RUB","customerUserId":null,"autoApproveExpenses":true}`))
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"name":"Полянка"`) || create.Header().Get("ETag") != `"1"` || projectStore.create.CustomerUserID == nil || *projectStore.create.CustomerUserID != endpointActorID {
		t.Fatalf("create status=%d body=%s command=%#v", create.Code, create.Body.String(), projectStore.create)
	}

	list := httptest.NewRecorder()
	endpoint.ListProjects(list, authenticatedRequest(http.MethodGet, "/api/v1/projects", ""), accountgenerated.ListProjectsParams{})
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"items"`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	pageSize := accountgenerated.PageSize(10)
	status := accountgenerated.ProjectStatus("active")
	cursor := accountgenerated.Cursor("invalid-cursor")
	list = httptest.NewRecorder()
	endpoint.ListProjects(list, authenticatedRequest(http.MethodGet, "/api/v1/projects", ""), accountgenerated.ListProjectsParams{PageSize: &pageSize, Status: &status, Cursor: &cursor})
	if list.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor status=%d body=%s", list.Code, list.Body.String())
	}

	get := httptest.NewRecorder()
	endpoint.GetProject(get, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID, ""), uuid.MustParse(endpointProjectID))
	if get.Code != http.StatusOK || get.Header().Get("ETag") != `"1"` {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}

	role := "super_admin"
	endpoint = projectEndpointForTest(t, activeAccount(&role), projectStore)
	assign := httptest.NewRecorder()
	endpoint.AssignProjectCustomer(assign, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/customer", `{"customerUserId":"00000000-0000-0000-0000-000000000002"}`), uuid.MustParse(endpointProjectID), accountgenerated.AssignProjectCustomerParams{IfMatch: `"1"`})
	if assign.Code != http.StatusOK || projectStore.assign.CustomerUserID != "00000000-0000-0000-0000-000000000002" {
		t.Fatalf("assign status=%d body=%s command=%#v", assign.Code, assign.Body.String(), projectStore.assign)
	}
}

func TestProjectMemberEndpointsListSearchAddAndRemove(t *testing.T) {
	joined := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	member := project.Member{
		UserID: endpointMemberID, Login: "designer.one", Email: "designer@example.com", FirstName: "Анна",
		ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер", ProjectRoles: []string{"executor"}, JoinedAt: joined, Removable: true,
	}
	candidate := project.MemberCandidate{UserID: endpointMemberID, Login: "designer.one", Email: "designer@example.com", FirstName: "Анна", ProfessionalRoleCode: "designer", ProfessionalRoleName: "Дизайнер"}
	store := &endpointProjectStore{view: projectView(), members: []project.Member{member}, manage: true, candidates: []project.MemberCandidate{candidate}, member: member}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)

	list := httptest.NewRecorder()
	endpoint.ListProjectMembers(list, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/members", ""), uuid.MustParse(endpointProjectID))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"canManage":true`) || !strings.Contains(list.Body.String(), `"login":"designer.one"`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	search := httptest.NewRecorder()
	endpoint.SearchProjectMemberCandidates(search, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/member-candidates?query=designer", ""), uuid.MustParse(endpointProjectID), accountgenerated.SearchProjectMemberCandidatesParams{Query: "designer"})
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), `"email":"designer@example.com"`) {
		t.Fatalf("search status=%d body=%s", search.Code, search.Body.String())
	}

	add := httptest.NewRecorder()
	endpoint.AddProjectMember(add, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/members", `{"userId":"`+endpointMemberID+`"}`), uuid.MustParse(endpointProjectID))
	if add.Code != http.StatusCreated || store.memberCommand.UserID != endpointMemberID || !strings.Contains(add.Body.String(), `"projectRoles":["executor"]`) {
		t.Fatalf("add status=%d body=%s command=%#v", add.Code, add.Body.String(), store.memberCommand)
	}

	remove := httptest.NewRecorder()
	endpoint.RemoveProjectMember(remove, authenticatedRequest(http.MethodDelete, "/api/v1/projects/"+endpointProjectID+"/members/"+endpointMemberID, ""), uuid.MustParse(endpointProjectID), uuid.MustParse(endpointMemberID))
	if remove.Code != http.StatusNoContent || store.memberCommand.UserID != endpointMemberID {
		t.Fatalf("remove status=%d command=%#v", remove.Code, store.memberCommand)
	}

	store.err = project.ErrForbidden
	forbidden := httptest.NewRecorder()
	endpoint.SearchProjectMemberCandidates(forbidden, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/member-candidates?query=designer", ""), uuid.MustParse(endpointProjectID), accountgenerated.SearchProjectMemberCandidatesParams{Query: "designer"})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden search status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestProjectFinanceEndpointsSummaryListAndCreateExpense(t *testing.T) {
	planned := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	expense := project.Expense{ID: endpointMemberID, ProjectID: endpointProjectID, AmountMinor: 125_050, CurrencyCode: "RUB", Category: "materials", Description: "Светильники", PlannedPaymentOn: &planned, Status: "auto_approved", CreatedByUserID: endpointActorID, CreatedAt: planned.AddDate(0, 0, -2), Version: 1}
	store := &endpointProjectStore{view: projectView(), financeSummary: project.FinanceSummary{CurrencyCode: "RUB", PendingExpenseMinor: 125_050, CanCreateExpense: true}, expenses: []project.Expense{expense}, canCreateExpense: true, expense: expense}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)

	summary := httptest.NewRecorder()
	endpoint.GetProjectFinanceSummary(summary, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/finance-summary", ""), uuid.MustParse(endpointProjectID))
	if summary.Code != http.StatusOK || !strings.Contains(summary.Body.String(), `"pendingExpenseMinor":125050`) || !strings.Contains(summary.Body.String(), `"canCreateExpense":true`) {
		t.Fatalf("summary status=%d body=%s", summary.Code, summary.Body.String())
	}

	list := httptest.NewRecorder()
	endpoint.ListProjectExpenses(list, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/expenses", ""), uuid.MustParse(endpointProjectID))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"description":"Светильники"`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	create := httptest.NewRecorder()
	body := `{"amountMinor":125050,"category":"materials","description":"Светильники","plannedPaymentOn":"2026-10-05"}`
	endpoint.CreateProjectExpense(create, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/expenses", body), uuid.MustParse(endpointProjectID), accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-key-1"})
	if create.Code != http.StatusCreated || store.expenseCommand.Request.AmountMinor != 125_050 || store.expenseCommand.Request.IdempotencyKey != "expense-key-1" || store.expenseCommand.Request.PlannedPaymentOn == nil || create.Header().Get("Location") == "" {
		t.Fatalf("create status=%d body=%s command=%#v", create.Code, create.Body.String(), store.expenseCommand)
	}

	bad := httptest.NewRecorder()
	endpoint.CreateProjectExpense(bad, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/expenses", `{}`), uuid.MustParse(endpointProjectID), accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-key-2"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad create status=%d body=%s", bad.Code, bad.Body.String())
	}

	missingCSRFRequest := authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/expenses", body)
	missingCSRFRequest.Header.Del("X-CSRF-Token")
	missingCSRF := httptest.NewRecorder()
	endpoint.CreateProjectExpense(missingCSRF, missingCSRFRequest, uuid.MustParse(endpointProjectID), accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-key-3"})
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d body=%s", missingCSRF.Code, missingCSRF.Body.String())
	}

	store.err = project.ErrForbidden
	forbidden := httptest.NewRecorder()
	endpoint.CreateProjectExpense(forbidden, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/expenses", body), uuid.MustParse(endpointProjectID), accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-key-4"})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestProjectMaterialEndpointsListCreateUpdateAndDelete(t *testing.T) {
	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	linkedExpenseID := "00000000-0000-0000-0000-000000000077"
	contextChatID := "00000000-0000-0000-0000-000000000078"
	contact, contract, notes, expenseDescription := "supplier@example.com", "Счёт 42", "Проверить оттенок", "Керамогранит"
	material := project.Material{
		ID: endpointMemberID, ProjectID: endpointProjectID, Category: "materials", Name: "Керамогранит", SupplierName: "Поставщик",
		CurrencyCode: "RUB", PaymentStatus: "partial", DeliveryStatus: "delivered", CreatedByUserID: endpointActorID,
		ContactInfo: &contact, ContractReference: &contract, LinkedExpenseID: &linkedExpenseID, LinkedExpenseDescription: &expenseDescription,
		Notes: &notes, ContextChatID: &contextChatID, PlannedDeliveryOn: &date, ActualDeliveryOn: &date, InstallationOn: &date,
		ContractAmountMinor: 125050, PaidAmountMinor: 50000, RemainingAmountMinor: 75050,
		CreatedAt: date.Add(-time.Hour), UpdatedAt: date, Version: 1, CanEdit: true, CanDelete: true,
	}
	store := &endpointProjectStore{view: projectView(), materials: []project.Material{material}, material: material}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)

	list := httptest.NewRecorder()
	endpoint.ListProjectMaterials(list, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/materials", ""), uuid.MustParse(endpointProjectID))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"name":"Керамогранит"`) || !strings.Contains(list.Body.String(), `"canViewFinancialInformation":true`) || !strings.Contains(list.Body.String(), linkedExpenseID) || !strings.Contains(list.Body.String(), contextChatID) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	body := `{"category":"materials","name":"Керамогранит","supplierName":"Поставщик","contactInfo":"supplier@example.com","contractReference":"Счёт 42","contractAmountMinor":125050,"linkedExpenseId":"` + linkedExpenseID + `","paymentStatus":"partial","deliveryStatus":"delivered","plannedDeliveryOn":"2026-10-05","actualDeliveryOn":"2026-10-05","installationOn":"2026-10-05","notes":"Проверить оттенок"}`
	create := httptest.NewRecorder()
	endpoint.CreateProjectMaterial(create, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/materials", body), uuid.MustParse(endpointProjectID))
	if create.Code != http.StatusCreated || store.materialCreate.Input.LinkedExpenseID == nil || *store.materialCreate.Input.LinkedExpenseID != linkedExpenseID || store.materialCreate.Input.ActualDeliveryOn == nil || create.Header().Get("Location") == "" {
		t.Fatalf("create status=%d body=%s command=%#v", create.Code, create.Body.String(), store.materialCreate)
	}

	store.material.Version = 2
	update := httptest.NewRecorder()
	endpoint.UpdateProjectMaterial(update, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/materials/"+endpointMemberID, body), uuid.MustParse(endpointProjectID), openapi_types.UUID(uuid.MustParse(endpointMemberID)), accountgenerated.UpdateProjectMaterialParams{IfMatch: `"1"`})
	if update.Code != http.StatusOK || store.materialUpdate.ExpectedVersion != 1 || store.materialUpdate.Input.Name != "Керамогранит" || !strings.Contains(update.Body.String(), `"version":2`) {
		t.Fatalf("update status=%d body=%s command=%#v", update.Code, update.Body.String(), store.materialUpdate)
	}

	remove := httptest.NewRecorder()
	endpoint.DeleteProjectMaterial(remove, authenticatedRequest(http.MethodDelete, "/api/v1/projects/"+endpointProjectID+"/materials/"+endpointMemberID, `{"reason":"Выбран другой поставщик"}`), uuid.MustParse(endpointProjectID), openapi_types.UUID(uuid.MustParse(endpointMemberID)), accountgenerated.DeleteProjectMaterialParams{IfMatch: `"2"`})
	if remove.Code != http.StatusNoContent || store.materialDelete.ExpectedVersion != 2 || store.materialDelete.Reason != "Выбран другой поставщик" {
		t.Fatalf("delete status=%d body=%s command=%#v", remove.Code, remove.Body.String(), store.materialDelete)
	}
}

func TestProjectMaterialMutationsRejectInvalidRequests(t *testing.T) {
	endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{view: projectView()})
	materialID := openapi_types.UUID(uuid.MustParse(endpointMemberID))
	validBody := `{"category":"materials","name":"Керамогранит","supplierName":"Поставщик","contractAmountMinor":125050,"paymentStatus":"unpaid","deliveryStatus":"expected"}`

	missingCSRF := authenticatedRequest(http.MethodPost, "/materials", validBody)
	missingCSRF.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.CreateProjectMaterial(response, missingCSRF, uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusForbidden {
		t.Fatalf("create csrf status=%d", response.Code)
	}

	for _, testCase := range []struct {
		name string
		call func(http.ResponseWriter)
	}{
		{"create body", func(w http.ResponseWriter) {
			endpoint.CreateProjectMaterial(w, authenticatedRequest(http.MethodPost, "/materials", `{}`), uuid.MustParse(endpointProjectID))
		}},
		{"update version", func(w http.ResponseWriter) {
			endpoint.UpdateProjectMaterial(w, authenticatedRequest(http.MethodPut, "/materials/id", validBody), uuid.MustParse(endpointProjectID), materialID, accountgenerated.UpdateProjectMaterialParams{IfMatch: "bad"})
		}},
		{"update body", func(w http.ResponseWriter) {
			endpoint.UpdateProjectMaterial(w, authenticatedRequest(http.MethodPut, "/materials/id", `{}`), uuid.MustParse(endpointProjectID), materialID, accountgenerated.UpdateProjectMaterialParams{IfMatch: `"1"`})
		}},
		{"delete version", func(w http.ResponseWriter) {
			endpoint.DeleteProjectMaterial(w, authenticatedRequest(http.MethodDelete, "/materials/id", `{"reason":"Причина"}`), uuid.MustParse(endpointProjectID), materialID, accountgenerated.DeleteProjectMaterialParams{IfMatch: "bad"})
		}},
		{"delete body", func(w http.ResponseWriter) {
			endpoint.DeleteProjectMaterial(w, authenticatedRequest(http.MethodDelete, "/materials/id", `{}`), uuid.MustParse(endpointProjectID), materialID, accountgenerated.DeleteProjectMaterialParams{IfMatch: `"1"`})
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.call(response)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestProjectCollaborationEndpointsMapServiceFailures(t *testing.T) {
	store := &endpointProjectStore{view: projectView(), err: project.ErrForbidden}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)
	projectID := uuid.MustParse(endpointProjectID)
	entityID := openapi_types.UUID(uuid.MustParse(endpointMemberID))
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	materialBody := `{"category":"materials","name":"Керамогранит","supplierName":"Поставщик","contractAmountMinor":125050,"paymentStatus":"unpaid","deliveryStatus":"expected"}`
	taskBody := `{"title":"Подготовить планы","dueAt":"2026-10-26T10:00:00Z"}`
	meetingBody := `{"title":"Встреча на объекте","startsAt":"2026-10-26T10:00:00Z","endsAt":"2026-10-26T11:00:00Z"}`
	chatBody := `{"body":"Проверьте чертежи","clientMessageId":"00000000-0000-0000-0000-000000000031"}`

	tests := []struct {
		name string
		call func(http.ResponseWriter)
	}{
		{"members list", func(w http.ResponseWriter) {
			endpoint.ListProjectMembers(w, authenticatedRequest(http.MethodGet, "/members", ""), projectID)
		}},
		{"members add", func(w http.ResponseWriter) {
			endpoint.AddProjectMember(w, authenticatedRequest(http.MethodPost, "/members", `{"userId":"`+endpointMemberID+`"}`), projectID)
		}},
		{"members remove", func(w http.ResponseWriter) {
			endpoint.RemoveProjectMember(w, authenticatedRequest(http.MethodDelete, "/members/id", ""), projectID, uuid.MustParse(endpointMemberID))
		}},
		{"finance summary", func(w http.ResponseWriter) {
			endpoint.GetProjectFinanceSummary(w, authenticatedRequest(http.MethodGet, "/finance-summary", ""), projectID)
		}},
		{"expenses list", func(w http.ResponseWriter) {
			endpoint.ListProjectExpenses(w, authenticatedRequest(http.MethodGet, "/expenses", ""), projectID)
		}},
		{"materials list", func(w http.ResponseWriter) {
			endpoint.ListProjectMaterials(w, authenticatedRequest(http.MethodGet, "/materials", ""), projectID)
		}},
		{"materials create", func(w http.ResponseWriter) {
			endpoint.CreateProjectMaterial(w, authenticatedRequest(http.MethodPost, "/materials", materialBody), projectID)
		}},
		{"materials update", func(w http.ResponseWriter) {
			endpoint.UpdateProjectMaterial(w, authenticatedRequest(http.MethodPut, "/materials/id", materialBody), projectID, entityID, accountgenerated.UpdateProjectMaterialParams{IfMatch: `"1"`})
		}},
		{"materials delete", func(w http.ResponseWriter) {
			endpoint.DeleteProjectMaterial(w, authenticatedRequest(http.MethodDelete, "/materials/id", `{"reason":"Другой поставщик"}`), projectID, entityID, accountgenerated.DeleteProjectMaterialParams{IfMatch: `"1"`})
		}},
		{"chat list", func(w http.ResponseWriter) {
			endpoint.GetProjectChat(w, authenticatedRequest(http.MethodGet, "/chat", ""), projectID, accountgenerated.GetProjectChatParams{})
		}},
		{"chat send", func(w http.ResponseWriter) {
			endpoint.CreateProjectChatMessage(w, authenticatedRequest(http.MethodPost, "/chat/messages", chatBody), projectID)
		}},
		{"context open", func(w http.ResponseWriter) {
			endpoint.OpenProjectContextChat(w, authenticatedRequest(http.MethodPut, "/chats/context", `{"contextType":"task","contextId":"`+endpointMemberID+`","name":"Чертежи"}`), projectID)
		}},
		{"context list", func(w http.ResponseWriter) {
			endpoint.GetProjectContextChat(w, authenticatedRequest(http.MethodGet, "/chats/id", ""), projectID, entityID, accountgenerated.GetProjectContextChatParams{})
		}},
		{"context send", func(w http.ResponseWriter) {
			endpoint.CreateProjectContextChatMessage(w, authenticatedRequest(http.MethodPost, "/chats/id/messages", chatBody), projectID, entityID)
		}},
		{"upcoming list", func(w http.ResponseWriter) {
			endpoint.ListProjectUpcomingItems(w, authenticatedRequest(http.MethodGet, "/upcoming", ""), projectID, accountgenerated.ListProjectUpcomingItemsParams{Days: 7})
		}},
		{"calendar list", func(w http.ResponseWriter) {
			endpoint.ListProjectCalendar(w, authenticatedRequest(http.MethodGet, "/calendar", ""), projectID, accountgenerated.ListProjectCalendarParams{From: from, To: to})
		}},
		{"global calendar list", func(w http.ResponseWriter) {
			endpoint.ListGlobalCalendar(w, authenticatedRequest(http.MethodGet, "/calendar", ""), accountgenerated.ListGlobalCalendarParams{From: from, To: to})
		}},
		{"task create", func(w http.ResponseWriter) {
			endpoint.CreateProjectTask(w, authenticatedRequest(http.MethodPost, "/tasks", taskBody), projectID)
		}},
		{"tasks list", func(w http.ResponseWriter) {
			endpoint.ListProjectTasks(w, authenticatedRequest(http.MethodGet, "/tasks", ""), projectID)
		}},
		{"task update", func(w http.ResponseWriter) {
			endpoint.UpdateProjectTask(w, authenticatedRequest(http.MethodPatch, "/tasks/id", `{"status":"in_progress"}`), projectID, entityID, accountgenerated.UpdateProjectTaskParams{IfMatch: 1})
		}},
		{"meeting create", func(w http.ResponseWriter) {
			endpoint.CreateProjectMeeting(w, authenticatedRequest(http.MethodPost, "/meetings", meetingBody), projectID)
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.call(response)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestProjectCollaborationEndpointsRejectUnauthenticatedReads(t *testing.T) {
	endpoint := projectEndpointForTest(t, &endpointAccountStore{}, &endpointProjectStore{})
	projectID := uuid.MustParse(endpointProjectID)
	chatID := openapi_types.UUID(uuid.MustParse(endpointMemberID))
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	tests := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"members", func(w http.ResponseWriter, r *http.Request) { endpoint.ListProjectMembers(w, r, projectID) }},
		{"finance", func(w http.ResponseWriter, r *http.Request) { endpoint.GetProjectFinanceSummary(w, r, projectID) }},
		{"expenses", func(w http.ResponseWriter, r *http.Request) { endpoint.ListProjectExpenses(w, r, projectID) }},
		{"materials", func(w http.ResponseWriter, r *http.Request) { endpoint.ListProjectMaterials(w, r, projectID) }},
		{"chat", func(w http.ResponseWriter, r *http.Request) {
			endpoint.GetProjectChat(w, r, projectID, accountgenerated.GetProjectChatParams{})
		}},
		{"context chat", func(w http.ResponseWriter, r *http.Request) {
			endpoint.GetProjectContextChat(w, r, projectID, chatID, accountgenerated.GetProjectContextChatParams{})
		}},
		{"upcoming", func(w http.ResponseWriter, r *http.Request) {
			endpoint.ListProjectUpcomingItems(w, r, projectID, accountgenerated.ListProjectUpcomingItemsParams{Days: 7})
		}},
		{"calendar", func(w http.ResponseWriter, r *http.Request) {
			endpoint.ListProjectCalendar(w, r, projectID, accountgenerated.ListProjectCalendarParams{From: from, To: to})
		}},
		{"global calendar", func(w http.ResponseWriter, r *http.Request) {
			endpoint.ListGlobalCalendar(w, r, accountgenerated.ListGlobalCalendarParams{From: from, To: to})
		}},
		{"tasks", func(w http.ResponseWriter, r *http.Request) { endpoint.ListProjectTasks(w, r, projectID) }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.call(response, httptest.NewRequest(http.MethodGet, "/resource", nil))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestProjectCollaborationEndpointsRejectUnauthenticatedMutations(t *testing.T) {
	endpoint := projectEndpointForTest(t, &endpointAccountStore{}, &endpointProjectStore{})
	projectID := uuid.MustParse(endpointProjectID)
	entityID := openapi_types.UUID(uuid.MustParse(endpointMemberID))
	tests := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"member add", func(w http.ResponseWriter, r *http.Request) { endpoint.AddProjectMember(w, r, projectID) }},
		{"member remove", func(w http.ResponseWriter, r *http.Request) {
			endpoint.RemoveProjectMember(w, r, projectID, uuid.MustParse(endpointMemberID))
		}},
		{"expense create", func(w http.ResponseWriter, r *http.Request) {
			endpoint.CreateProjectExpense(w, r, projectID, accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-unauthenticated"})
		}},
		{"material create", func(w http.ResponseWriter, r *http.Request) { endpoint.CreateProjectMaterial(w, r, projectID) }},
		{"material update", func(w http.ResponseWriter, r *http.Request) {
			endpoint.UpdateProjectMaterial(w, r, projectID, entityID, accountgenerated.UpdateProjectMaterialParams{IfMatch: `"1"`})
		}},
		{"material delete", func(w http.ResponseWriter, r *http.Request) {
			endpoint.DeleteProjectMaterial(w, r, projectID, entityID, accountgenerated.DeleteProjectMaterialParams{IfMatch: `"1"`})
		}},
		{"chat send", func(w http.ResponseWriter, r *http.Request) { endpoint.CreateProjectChatMessage(w, r, projectID) }},
		{"context open", func(w http.ResponseWriter, r *http.Request) { endpoint.OpenProjectContextChat(w, r, projectID) }},
		{"context send", func(w http.ResponseWriter, r *http.Request) {
			endpoint.CreateProjectContextChatMessage(w, r, projectID, entityID)
		}},
		{"task create", func(w http.ResponseWriter, r *http.Request) { endpoint.CreateProjectTask(w, r, projectID) }},
		{"task update", func(w http.ResponseWriter, r *http.Request) {
			endpoint.UpdateProjectTask(w, r, projectID, entityID, accountgenerated.UpdateProjectTaskParams{IfMatch: 1})
		}},
		{"meeting create", func(w http.ResponseWriter, r *http.Request) { endpoint.CreateProjectMeeting(w, r, projectID) }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.call(response, authenticatedRequest(http.MethodPost, "/resource", `{}`))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRemainingProjectEndpointsRejectUnauthenticatedActors(t *testing.T) {
	endpoint := projectEndpointForTest(t, &endpointAccountStore{}, &endpointProjectStore{})
	projectID := uuid.MustParse(endpointProjectID)
	entityID := uuid.MustParse(endpointMemberID)
	chatID := openapi_types.UUID(entityID)
	reads := []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.ListProjects(w, r, accountgenerated.ListProjectsParams{})
		},
		func(w http.ResponseWriter, r *http.Request) { endpoint.GetProject(w, r, projectID) },
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.SearchProjectMemberCandidates(w, r, projectID, accountgenerated.SearchProjectMemberCandidatesParams{Query: "user"})
		},
		func(w http.ResponseWriter, r *http.Request) { endpoint.ListProjectChats(w, r, projectID) },
		func(w http.ResponseWriter, r *http.Request) { endpoint.ListProjectChatMembers(w, r, projectID, chatID) },
		func(w http.ResponseWriter, r *http.Request) { endpoint.ListProjectDocuments(w, r, projectID) },
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.DownloadProjectDocument(w, r, projectID, entityID)
		},
	}
	for index, call := range reads {
		response := httptest.NewRecorder()
		call(response, httptest.NewRequest(http.MethodGet, "/resource", nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("read %d status=%d body=%s", index, response.Code, response.Body.String())
		}
	}
	mutations := []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) { endpoint.CreateProject(w, r) },
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.UpdateProject(w, r, projectID, accountgenerated.UpdateProjectParams{IfMatch: `"1"`})
		},
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.DeleteProject(w, r, projectID, accountgenerated.DeleteProjectParams{IfMatch: `"1"`})
		},
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.AssignProjectCustomer(w, r, projectID, accountgenerated.AssignProjectCustomerParams{IfMatch: `"1"`})
		},
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.CreateProjectExpense(w, r, projectID, accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-unauthenticated"})
		},
		func(w http.ResponseWriter, r *http.Request) { endpoint.CreateProjectChat(w, r, projectID) },
		func(w http.ResponseWriter, r *http.Request) { endpoint.UpdateProjectChat(w, r, projectID, chatID) },
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.DeleteProjectChat(w, r, projectID, chatID, accountgenerated.DeleteProjectChatParams{Version: 1})
		},
		func(w http.ResponseWriter, r *http.Request) { endpoint.AddProjectChatMember(w, r, projectID, chatID) },
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.RemoveProjectChatMember(w, r, projectID, chatID, entityID)
		},
		func(w http.ResponseWriter, r *http.Request) { endpoint.MarkProjectChatRead(w, r, projectID, chatID) },
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.UploadProjectDocument(w, r, projectID, accountgenerated.UploadProjectDocumentParams{XFileName: "plan.pdf"})
		},
		func(w http.ResponseWriter, r *http.Request) {
			endpoint.DeleteProjectDocument(w, r, projectID, entityID)
		},
	}
	for index, call := range mutations {
		response := httptest.NewRecorder()
		call(response, authenticatedRequest(http.MethodPost, "/resource", `{}`))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("mutation %d status=%d body=%s", index, response.Code, response.Body.String())
		}
	}
}

func TestProjectEndpointMalformedPayloadAndOptionalMappingBranches(t *testing.T) {
	now := time.Now().UTC()
	store := &endpointProjectStore{
		view: projectView(),
		upcoming: []project.UpcomingItem{{
			ID:          endpointMemberID,
			ProjectID:   endpointProjectID,
			Kind:        project.UpcomingTask,
			Title:       "Подготовить планы",
			EffectiveAt: now.Add(24 * time.Hour),
			CreatedAt:   now,
			Version:     1,
		}},
		document: project.ProjectDocument{
			ID:               endpointMemberID,
			ProjectID:        endpointProjectID,
			Name:             "plan.pdf",
			MediaType:        "application/octet-stream",
			SizeBytes:        3,
			UploadedByUserID: endpointActorID,
			CreatedAt:        now,
			Version:          1,
		},
	}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)
	projectID := uuid.MustParse(endpointProjectID)
	entityID := openapi_types.UUID(uuid.MustParse(endpointMemberID))
	malformed := func(method string) *http.Request { return authenticatedRequest(method, "/resource", `{`) }
	checks := []func(*httptest.ResponseRecorder){
		func(w *httptest.ResponseRecorder) { endpoint.CreateProject(w, malformed(http.MethodPost)) },
		func(w *httptest.ResponseRecorder) {
			endpoint.CreateProjectExpense(w, malformed(http.MethodPost), projectID, accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-malformed-json"})
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.CreateProjectMaterial(w, malformed(http.MethodPost), projectID)
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.UpdateProjectMaterial(w, malformed(http.MethodPut), projectID, entityID, accountgenerated.UpdateProjectMaterialParams{IfMatch: `"1"`})
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.DeleteProjectMaterial(w, malformed(http.MethodDelete), projectID, entityID, accountgenerated.DeleteProjectMaterialParams{IfMatch: `"1"`})
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.OpenProjectContextChat(w, malformed(http.MethodPut), projectID)
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.CreateProjectContextChatMessage(w, malformed(http.MethodPost), projectID, entityID)
		},
	}
	for index, check := range checks {
		response := httptest.NewRecorder()
		check(response)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("malformed case %d status=%d body=%s", index, response.Code, response.Body.String())
		}
	}

	csrfRequest := authenticatedRequest(http.MethodPut, "/customer", `{}`)
	csrfRequest.Header.Del("X-CSRF-Token")
	csrfResponse := httptest.NewRecorder()
	endpoint.AssignProjectCustomer(csrfResponse, csrfRequest, projectID, accountgenerated.AssignProjectCustomerParams{IfMatch: `"1"`})
	if csrfResponse.Code != http.StatusForbidden {
		t.Fatalf("assign customer csrf status=%d", csrfResponse.Code)
	}

	upload := httptest.NewRecorder()
	endpoint.UploadProjectDocument(upload, authenticatedRequest(http.MethodPost, "/documents", "pdf"), projectID, accountgenerated.UploadProjectDocumentParams{XFileName: "plan.pdf"})
	if upload.Code != http.StatusCreated {
		t.Fatalf("default media type upload status=%d body=%s", upload.Code, upload.Body.String())
	}

	task := httptest.NewRecorder()
	endpoint.CreateProjectTask(task, authenticatedRequest(http.MethodPost, "/tasks", `{"title":"Подготовить планы","dueAt":"2026-10-26T10:00:00Z","assigneeUserIds":["`+endpointMemberID+`"]}`), projectID)
	if task.Code != http.StatusCreated {
		t.Fatalf("task assignees status=%d body=%s", task.Code, task.Body.String())
	}

	var optional optionalProjectField[string]
	if err := json.Unmarshal([]byte(`123`), &optional); err == nil {
		t.Fatal("invalid optional project field value accepted")
	}
}

func TestProjectCollaborationMutationsRejectCsrfAndMalformedBodies(t *testing.T) {
	endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{view: projectView()})
	projectID := uuid.MustParse(endpointProjectID)
	entityID := openapi_types.UUID(uuid.MustParse(endpointMemberID))
	withoutCSRF := func(method, body string) *http.Request {
		request := authenticatedRequest(method, "/resource", body)
		request.Header.Del("X-CSRF-Token")
		return request
	}
	csrfTests := []struct {
		name string
		call func(http.ResponseWriter)
	}{
		{"member add", func(w http.ResponseWriter) {
			endpoint.AddProjectMember(w, withoutCSRF(http.MethodPost, `{}`), projectID)
		}},
		{"member remove", func(w http.ResponseWriter) {
			endpoint.RemoveProjectMember(w, withoutCSRF(http.MethodDelete, ``), projectID, uuid.MustParse(endpointMemberID))
		}},
		{"material update", func(w http.ResponseWriter) {
			endpoint.UpdateProjectMaterial(w, withoutCSRF(http.MethodPut, `{}`), projectID, entityID, accountgenerated.UpdateProjectMaterialParams{IfMatch: `"1"`})
		}},
		{"material delete", func(w http.ResponseWriter) {
			endpoint.DeleteProjectMaterial(w, withoutCSRF(http.MethodDelete, `{}`), projectID, entityID, accountgenerated.DeleteProjectMaterialParams{IfMatch: `"1"`})
		}},
		{"context message", func(w http.ResponseWriter) {
			endpoint.CreateProjectContextChatMessage(w, withoutCSRF(http.MethodPost, `{}`), projectID, entityID)
		}},
		{"chat send", func(w http.ResponseWriter) {
			endpoint.CreateProjectChatMessage(w, withoutCSRF(http.MethodPost, `{}`), projectID)
		}},
		{"context open", func(w http.ResponseWriter) {
			endpoint.OpenProjectContextChat(w, withoutCSRF(http.MethodPut, `{}`), projectID)
		}},
		{"task create", func(w http.ResponseWriter) {
			endpoint.CreateProjectTask(w, withoutCSRF(http.MethodPost, `{}`), projectID)
		}},
		{"task update", func(w http.ResponseWriter) {
			endpoint.UpdateProjectTask(w, withoutCSRF(http.MethodPatch, `{}`), projectID, entityID, accountgenerated.UpdateProjectTaskParams{IfMatch: 1})
		}},
		{"meeting create", func(w http.ResponseWriter) {
			endpoint.CreateProjectMeeting(w, withoutCSRF(http.MethodPost, `{}`), projectID)
		}},
		{"project chat create", func(w http.ResponseWriter) {
			endpoint.CreateProjectChat(w, withoutCSRF(http.MethodPost, `{}`), projectID)
		}},
		{"project chat update", func(w http.ResponseWriter) {
			endpoint.UpdateProjectChat(w, withoutCSRF(http.MethodPatch, `{}`), projectID, entityID)
		}},
		{"project chat delete", func(w http.ResponseWriter) {
			endpoint.DeleteProjectChat(w, withoutCSRF(http.MethodDelete, ``), projectID, entityID, accountgenerated.DeleteProjectChatParams{Version: 1})
		}},
		{"project chat member add", func(w http.ResponseWriter) {
			endpoint.AddProjectChatMember(w, withoutCSRF(http.MethodPost, `{}`), projectID, entityID)
		}},
		{"project chat member remove", func(w http.ResponseWriter) {
			endpoint.RemoveProjectChatMember(w, withoutCSRF(http.MethodDelete, ``), projectID, entityID, uuid.MustParse(endpointMemberID))
		}},
		{"project chat read", func(w http.ResponseWriter) {
			endpoint.MarkProjectChatRead(w, withoutCSRF(http.MethodPost, `{}`), projectID, entityID)
		}},
		{"project document upload", func(w http.ResponseWriter) {
			endpoint.UploadProjectDocument(w, withoutCSRF(http.MethodPost, `pdf`), projectID, accountgenerated.UploadProjectDocumentParams{XFileName: "plan.pdf"})
		}},
		{"project document delete", func(w http.ResponseWriter) {
			endpoint.DeleteProjectDocument(w, withoutCSRF(http.MethodDelete, ``), projectID, entityID)
		}},
	}
	for _, testCase := range csrfTests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.call(response)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}

	badBodies := []struct {
		name string
		call func(http.ResponseWriter)
	}{
		{"member add", func(w http.ResponseWriter) {
			endpoint.AddProjectMember(w, authenticatedRequest(http.MethodPost, "/members", `{}`), projectID)
		}},
		{"expense create", func(w http.ResponseWriter) {
			endpoint.CreateProjectExpense(w, authenticatedRequest(http.MethodPost, "/expenses", `{}`), projectID, accountgenerated.CreateProjectExpenseParams{IdempotencyKey: "expense-malformed"})
		}},
		{"material create", func(w http.ResponseWriter) {
			endpoint.CreateProjectMaterial(w, authenticatedRequest(http.MethodPost, "/materials", `{}`), projectID)
		}},
		{"chat send", func(w http.ResponseWriter) {
			endpoint.CreateProjectChatMessage(w, authenticatedRequest(http.MethodPost, "/chat/messages", `{}`), projectID)
		}},
		{"context open", func(w http.ResponseWriter) {
			endpoint.OpenProjectContextChat(w, authenticatedRequest(http.MethodPut, "/chats/context", `{}`), projectID)
		}},
		{"context message", func(w http.ResponseWriter) {
			endpoint.CreateProjectContextChatMessage(w, authenticatedRequest(http.MethodPost, "/messages", `{}`), projectID, entityID)
		}},
		{"task create", func(w http.ResponseWriter) {
			endpoint.CreateProjectTask(w, authenticatedRequest(http.MethodPost, "/tasks", `{}`), projectID)
		}},
		{"task update", func(w http.ResponseWriter) {
			endpoint.UpdateProjectTask(w, authenticatedRequest(http.MethodPatch, "/tasks/id", `{}`), projectID, entityID, accountgenerated.UpdateProjectTaskParams{IfMatch: 1})
		}},
		{"meeting create", func(w http.ResponseWriter) {
			endpoint.CreateProjectMeeting(w, authenticatedRequest(http.MethodPost, "/meetings", `{}`), projectID)
		}},
		{"malformed chat", func(w http.ResponseWriter) {
			endpoint.CreateProjectChatMessage(w, authenticatedRequest(http.MethodPost, "/chat/messages", `{`), projectID)
		}},
		{"malformed meeting", func(w http.ResponseWriter) {
			endpoint.CreateProjectMeeting(w, authenticatedRequest(http.MethodPost, "/meetings", `{`), projectID)
		}},
	}
	for _, testCase := range badBodies {
		t.Run("bad "+testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.call(response)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestProjectEndpointsUpdateArchiveAndCurrentAccount(t *testing.T) {
	projectStore := &endpointProjectStore{view: projectView()}
	endpoint := projectEndpointForTest(t, activeAccount(nil), projectStore)

	current := httptest.NewRecorder()
	endpoint.GetCurrentAccount(current, authenticatedRequest(http.MethodGet, "/api/v1/me", ""))
	if current.Code != http.StatusOK || !strings.Contains(current.Body.String(), `"login":"svetlana"`) {
		t.Fatalf("current status=%d body=%s", current.Code, current.Body.String())
	}

	update := httptest.NewRecorder()
	endpoint.UpdateProject(update, authenticatedRequest(http.MethodPatch, "/api/v1/projects/"+endpointProjectID, `{"name":"Новая Полянка","type":"architecture","status":"active","address":null,"description":"Описание","plannedStartOn":"2026-10-01","plannedFinishOn":"2026-12-01","autoApproveExpenses":false}`), uuid.MustParse(endpointProjectID), accountgenerated.UpdateProjectParams{IfMatch: `"1"`})
	if update.Code != http.StatusOK || projectStore.update.Name == nil || *projectStore.update.Name != "Новая Полянка" || projectStore.update.Type == nil || projectStore.update.Status == nil || !projectStore.update.Description.Set || !projectStore.update.Address.Set || projectStore.update.Address.Value != nil || projectStore.update.PlannedStartOn.Value == nil || projectStore.update.AutoApproveExpenses == nil || *projectStore.update.AutoApproveExpenses {
		t.Fatalf("update status=%d body=%s command=%#v", update.Code, update.Body.String(), projectStore.update)
	}

	archive := httptest.NewRecorder()
	endpoint.DeleteProject(archive, authenticatedRequest(http.MethodDelete, "/api/v1/projects/"+endpointProjectID, ""), uuid.MustParse(endpointProjectID), accountgenerated.DeleteProjectParams{IfMatch: `"1"`})
	if archive.Code != http.StatusForbidden {
		t.Fatalf("ordinary archive status=%d body=%s", archive.Code, archive.Body.String())
	}
	role := "super_admin"
	endpoint = projectEndpointForTest(t, activeAccount(&role), projectStore)
	archive = httptest.NewRecorder()
	endpoint.DeleteProject(archive, authenticatedRequest(http.MethodDelete, "/api/v1/projects/"+endpointProjectID, ""), uuid.MustParse(endpointProjectID), accountgenerated.DeleteProjectParams{IfMatch: `"1"`})
	if archive.Code != http.StatusNoContent || projectStore.archive.ProjectID != endpointProjectID {
		t.Fatalf("super archive status=%d command=%#v", archive.Code, projectStore.archive)
	}
}

func TestProjectUpdateRejectsInvalidBodyDatesAndVersion(t *testing.T) {
	endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{view: projectView()})
	for _, test := range []struct {
		version string
		body    string
	}{
		{"bad", `{"name":"Проект"}`},
		{`"1"`, `{}`},
		{`"1"`, `{"name":null}`},
		{`"1"`, `{"type":null}`},
		{`"1"`, `{"status":null}`},
		{`"1"`, `{"autoApproveExpenses":null}`},
		{`"1"`, `{"plannedStartOn":"not-a-date"}`},
		{`"1"`, `{"plannedFinishOn":"not-a-date"}`},
		{`"1"`, `{"unknown":true}`},
	} {
		response := httptest.NewRecorder()
		endpoint.UpdateProject(response, authenticatedRequest(http.MethodPatch, "/api/v1/projects/"+endpointProjectID, test.body), uuid.MustParse(endpointProjectID), accountgenerated.UpdateProjectParams{IfMatch: test.version})
		if response.Code != http.StatusBadRequest {
			t.Fatalf("version=%s body=%s status=%d response=%s", test.version, test.body, response.Code, response.Body.String())
		}
	}
}

func TestProjectMutationEndpointsRejectCsrfVersionAndStoreFailures(t *testing.T) {
	role := "super_admin"
	store := &endpointProjectStore{view: projectView()}
	endpoint := projectEndpointForTest(t, activeAccount(&role), store)
	badUpdateCSRF := authenticatedRequest(http.MethodPatch, "/api/v1/projects/"+endpointProjectID, `{"name":"Проект"}`)
	badUpdateCSRF.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.UpdateProject(response, badUpdateCSRF, uuid.MustParse(endpointProjectID), accountgenerated.UpdateProjectParams{IfMatch: `"1"`})
	if response.Code != http.StatusForbidden {
		t.Fatalf("update csrf status=%d", response.Code)
	}

	badCSRF := authenticatedRequest(http.MethodDelete, "/api/v1/projects/"+endpointProjectID, "")
	badCSRF.Header.Del("X-CSRF-Token")
	response = httptest.NewRecorder()
	endpoint.DeleteProject(response, badCSRF, uuid.MustParse(endpointProjectID), accountgenerated.DeleteProjectParams{IfMatch: `"1"`})
	if response.Code != http.StatusForbidden {
		t.Fatalf("delete csrf status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	endpoint.DeleteProject(response, authenticatedRequest(http.MethodDelete, "/api/v1/projects/"+endpointProjectID, ""), uuid.MustParse(endpointProjectID), accountgenerated.DeleteProjectParams{IfMatch: "bad"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("delete version status=%d", response.Code)
	}
	store.err = project.ErrConflict
	response = httptest.NewRecorder()
	endpoint.DeleteProject(response, authenticatedRequest(http.MethodDelete, "/api/v1/projects/"+endpointProjectID, ""), uuid.MustParse(endpointProjectID), accountgenerated.DeleteProjectParams{IfMatch: `"1"`})
	if response.Code != http.StatusConflict {
		t.Fatalf("delete conflict status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	endpoint.UpdateProject(response, authenticatedRequest(http.MethodPatch, "/api/v1/projects/"+endpointProjectID, `{"name":"Проект"}`), uuid.MustParse(endpointProjectID), accountgenerated.UpdateProjectParams{IfMatch: `"1"`})
	if response.Code != http.StatusConflict {
		t.Fatalf("update conflict status=%d", response.Code)
	}

	unauthenticated := projectEndpointForTest(t, &endpointAccountStore{}, &endpointProjectStore{})
	response = httptest.NewRecorder()
	unauthenticated.GetCurrentAccount(response, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("current account status=%d", response.Code)
	}
	failedSession := projectEndpointForTest(t, &endpointAccountStore{accessErr: context.DeadlineExceeded}, &endpointProjectStore{})
	response = httptest.NewRecorder()
	failedSession.GetCurrentAccount(response, authenticatedRequest(http.MethodGet, "/api/v1/me", ""))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("current account store error status=%d", response.Code)
	}
}

func TestProjectEndpointConvertsCompleteRequestAndResponse(t *testing.T) {
	role := "super_admin"
	customerID := "00000000-0000-0000-0000-000000000002"
	address, description := "Москва", "Полный проект"
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	finish := time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)
	view := projectView()
	view.Address, view.Description, view.CustomerUserID = &address, &description, &customerID
	view.PlannedStartOn, view.PlannedFinishOn = &start, &finish
	store := &endpointProjectStore{view: view}
	endpoint := projectEndpointForTest(t, activeAccount(&role), store)
	response := httptest.NewRecorder()
	body := `{"name":"Полянка","type":"interior_design","address":"Москва","currencyCode":"RUB","customerUserId":"` + customerID + `","adminUserIds":["` + endpointActorID + `"],"plannedStartOn":"2026-10-01","plannedFinishOn":"2027-01-15","description":"Полный проект","autoApproveExpenses":true}`
	endpoint.CreateProject(response, authenticatedRequest(http.MethodPost, "/api/v1/projects", body))
	if response.Code != http.StatusCreated || store.create.CustomerUserID == nil || *store.create.CustomerUserID != customerID || len(store.create.AdminUserIDs) != 1 || store.create.PlannedStartOn == nil || store.create.PlannedFinishOn == nil {
		t.Fatalf("status=%d body=%s command=%#v", response.Code, response.Body.String(), store.create)
	}
	for _, fragment := range []string{`"address":"Москва"`, `"description":"Полный проект"`, `"customerUserId":"` + customerID + `"`, `"plannedStartOn":"2026-10-01"`, `"adminUserIds"`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("response lacks %s: %s", fragment, response.Body.String())
		}
	}
}

func TestProjectTaskAndExpenseResponsesIncludeOptionalContext(t *testing.T) {
	contextChatID := endpointMemberID
	planned := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	task := projectTaskResponse(project.Task{
		ID: endpointMemberID, ProjectID: endpointProjectID, Title: "Чертежи", Status: project.TaskNew,
		ContextChatID: &contextChatID, Assignees: []project.TaskAssignee{{UserID: endpointActorID, Login: "svetlana"}},
	})
	if task.ContextChatId == nil || task.ContextChatId.String() != endpointMemberID || len(task.Assignees) != 1 {
		t.Fatalf("task response=%#v", task)
	}
	expense := projectExpenseResponse(project.Expense{
		ID: endpointMemberID, ProjectID: endpointProjectID, AmountMinor: 125050, CurrencyCode: "RUB", Category: "materials",
		Description: "Керамогранит", Status: "pending", CreatedByUserID: endpointActorID,
		PlannedPaymentOn: &planned, ContextChatID: &contextChatID,
	})
	if expense.PlannedPaymentOn == nil || expense.ContextChatId == nil || expense.ContextChatId.String() != endpointMemberID {
		t.Fatalf("expense response=%#v", expense)
	}
	nextCursor := endpointMemberID
	chatPage := projectChatPageResponse(project.ProjectChatPage{
		ChatID: endpointMemberID, ProjectID: endpointProjectID, CurrentUserID: endpointActorID,
		HasMore: true, CanSend: true, NextCursor: &nextCursor,
		Context: &project.ChatContext{ChatID: endpointMemberID, ProjectID: endpointProjectID, ContextType: "task", ContextID: endpointActorID, ContextTitle: "Чертежи", Name: "Обсуждение"},
	})
	if chatPage.NextCursor == nil || chatPage.Context == nil || chatPage.Context.ContextTitle != "Чертежи" {
		t.Fatalf("chat page response=%#v", chatPage)
	}
}

func TestProjectChatEndpointsForwardBeforeCursor(t *testing.T) {
	store := &endpointProjectStore{view: projectView(), chatPage: project.ProjectChatPage{ChatID: endpointMemberID, ProjectID: endpointProjectID, CurrentUserID: endpointActorID}}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)
	projectID := uuid.MustParse(endpointProjectID)
	chatID := openapi_types.UUID(uuid.MustParse(endpointMemberID))
	before := openapi_types.UUID(uuid.MustParse(endpointActorID))
	response := httptest.NewRecorder()
	endpoint.GetProjectChat(response, authenticatedRequest(http.MethodGet, "/chat", ""), projectID, accountgenerated.GetProjectChatParams{Before: &before})
	if response.Code != http.StatusOK || store.chatQuery.BeforeID == nil || *store.chatQuery.BeforeID != endpointActorID {
		t.Fatalf("project chat status=%d query=%#v", response.Code, store.chatQuery)
	}
	response = httptest.NewRecorder()
	endpoint.GetProjectContextChat(response, authenticatedRequest(http.MethodGet, "/chats/id", ""), projectID, chatID, accountgenerated.GetProjectContextChatParams{Before: &before})
	if response.Code != http.StatusOK || store.chatQuery.BeforeID == nil || *store.chatQuery.BeforeID != endpointActorID {
		t.Fatalf("context chat status=%d query=%#v", response.Code, store.chatQuery)
	}
}

func TestProjectEndpointsRejectInvalidSessionCsrfBodyAndVersion(t *testing.T) {
	endpoint := projectEndpointForTest(t, &endpointAccountStore{}, &endpointProjectStore{view: projectView()})
	unauthenticated := httptest.NewRecorder()
	endpoint.ListProjects(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil), accountgenerated.ListProjectsParams{})
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", unauthenticated.Code)
	}

	endpoint = projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{view: projectView()})
	badCSRF := authenticatedRequest(http.MethodPost, "/api/v1/projects", `{}`)
	badCSRF.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.CreateProject(response, badCSRF)
	if response.Code != http.StatusForbidden {
		t.Fatalf("csrf status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	endpoint.CreateProject(response, authenticatedRequest(http.MethodPost, "/api/v1/projects", `{}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("body status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	endpoint.AssignProjectCustomer(response, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/customer", `{}`), uuid.MustParse(endpointProjectID), accountgenerated.AssignProjectCustomerParams{IfMatch: "wrong"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("version status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	endpoint.AssignProjectCustomer(response, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/customer", `{}`), uuid.MustParse(endpointProjectID), accountgenerated.AssignProjectCustomerParams{IfMatch: `"1"`})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing customer status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProjectEndpointErrorMappingAndMissingConfiguration(t *testing.T) {
	request := authenticatedRequest(http.MethodGet, "/api/v1/projects", "")
	missing := accountEndpointForTest(t, activeAccount(nil))
	response := httptest.NewRecorder()
	missing.ListProjects(response, request, accountgenerated.ListProjectsParams{})
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing service status=%d", response.Code)
	}

	for _, test := range []struct {
		err    error
		status int
	}{
		{project.ErrInvalidInput, http.StatusBadRequest},
		{project.ErrForbidden, http.StatusForbidden},
		{project.ErrNotFound, http.StatusNotFound},
		{project.ErrConflict, http.StatusConflict},
		{project.ErrAccountUnavailable, http.StatusConflict},
		{errors.New("database failed"), http.StatusInternalServerError},
	} {
		endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{err: test.err})
		response = httptest.NewRecorder()
		endpoint.ListProjects(response, authenticatedRequest(http.MethodGet, "/api/v1/projects", ""), accountgenerated.ListProjectsParams{})
		if response.Code != test.status {
			t.Fatalf("error=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
}

func TestProjectChatEndpointsListAndSend(t *testing.T) {
	createdAt := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	message := project.ChatMessage{
		ID: endpointMemberID, ChatID: "00000000-0000-0000-0000-000000000030", ProjectID: endpointProjectID,
		Body: "Обновила планировку", Author: project.ChatAuthor{UserID: endpointActorID, Login: "svetlana", FirstName: "Светлана"},
		CreatedAt: createdAt, Version: 1,
	}
	store := &endpointProjectStore{
		chatPage: project.ProjectChatPage{
			ChatID: message.ChatID, ProjectID: endpointProjectID, CurrentUserID: endpointActorID,
			Messages: []project.ChatMessage{message}, CanSend: true,
		},
		chatMessage: message,
	}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)

	pageSize := 25
	list := httptest.NewRecorder()
	endpoint.GetProjectChat(list, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/chat?pageSize=25", ""), uuid.MustParse(endpointProjectID), accountgenerated.GetProjectChatParams{PageSize: &pageSize})
	if list.Code != http.StatusOK || store.chatQuery.PageSize != pageSize || !strings.Contains(list.Body.String(), `"body":"Обновила планировку"`) || !strings.Contains(list.Body.String(), `"canSend":true`) {
		t.Fatalf("list status=%d body=%s query=%#v", list.Code, list.Body.String(), store.chatQuery)
	}

	clientMessageID := "00000000-0000-0000-0000-000000000031"
	send := httptest.NewRecorder()
	endpoint.CreateProjectChatMessage(send, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/chat/messages", `{"body":" Обновила планировку ","clientMessageId":"`+clientMessageID+`"}`), uuid.MustParse(endpointProjectID))
	if send.Code != http.StatusCreated || store.chatCommand.Body != "Обновила планировку" || store.chatCommand.ClientMessageID != clientMessageID || store.chatCommand.RequestID != "request-project" {
		t.Fatalf("send status=%d body=%s command=%#v", send.Code, send.Body.String(), store.chatCommand)
	}
}

func TestProjectChatSendRejectsCsrfAndInvalidBody(t *testing.T) {
	endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{})
	request := authenticatedRequest(http.MethodPost, "/chat/messages", `{}`)
	request.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.CreateProjectChatMessage(response, request, uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusForbidden {
		t.Fatalf("csrf status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	endpoint.CreateProjectChatMessage(response, authenticatedRequest(http.MethodPost, "/chat/messages", `{}`), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProjectContextChatEndpointsOpenListAndSend(t *testing.T) {
	chatID := "00000000-0000-0000-0000-000000000030"
	contextID := "00000000-0000-0000-0000-000000000040"
	message := project.ChatMessage{
		ID: endpointMemberID, ChatID: chatID, ProjectID: endpointProjectID, Body: "Уточнила срок",
		Author: project.ChatAuthor{UserID: endpointActorID, Login: "svetlana", FirstName: "Светлана"}, CreatedAt: time.Now(), Version: 1,
	}
	store := &endpointProjectStore{
		chatContext: project.ChatContext{ChatID: chatID, ProjectID: endpointProjectID, ContextType: "task", ContextID: contextID, ContextTitle: "Эскизы", Name: "Обсуждение эскизов"},
		chatPage:    project.ProjectChatPage{ChatID: chatID, ProjectID: endpointProjectID, CurrentUserID: endpointActorID, Context: &project.ChatContext{ChatID: chatID, ProjectID: endpointProjectID, ContextType: "task", ContextID: contextID, ContextTitle: "Эскизы", Name: "Обсуждение эскизов"}, Messages: []project.ChatMessage{message}, CanSend: true},
		chatMessage: message,
	}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)

	opened := httptest.NewRecorder()
	endpoint.OpenProjectContextChat(opened, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/chats/context", `{"contextType":"task","contextId":"`+contextID+`","name":"Обсуждение эскизов"}`), uuid.MustParse(endpointProjectID))
	if opened.Code != http.StatusOK || store.contextCommand.ContextID != contextID || store.contextCommand.Name != "Обсуждение эскизов" || !strings.Contains(opened.Body.String(), `"contextTitle":"Эскизы"`) {
		t.Fatalf("open status=%d body=%s command=%#v", opened.Code, opened.Body.String(), store.contextCommand)
	}

	pageSize := 25
	listed := httptest.NewRecorder()
	endpoint.GetProjectContextChat(listed, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/chats/"+chatID+"?pageSize=25", ""), uuid.MustParse(endpointProjectID), uuid.MustParse(chatID), accountgenerated.GetProjectContextChatParams{PageSize: &pageSize})
	if listed.Code != http.StatusOK || store.chatQuery.ChatID != chatID || !strings.Contains(listed.Body.String(), `"name":"Обсуждение эскизов"`) || !strings.Contains(listed.Body.String(), `"body":"Уточнила срок"`) {
		t.Fatalf("list status=%d body=%s query=%#v", listed.Code, listed.Body.String(), store.chatQuery)
	}

	clientMessageID := "00000000-0000-0000-0000-000000000050"
	sent := httptest.NewRecorder()
	endpoint.CreateProjectContextChatMessage(sent, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/chats/"+chatID+"/messages", `{"body":"Уточнила срок","clientMessageId":"`+clientMessageID+`"}`), uuid.MustParse(endpointProjectID), uuid.MustParse(chatID))
	if sent.Code != http.StatusCreated || store.chatCommand.ChatID != chatID || store.chatCommand.ClientMessageID != clientMessageID {
		t.Fatalf("send status=%d body=%s command=%#v", sent.Code, sent.Body.String(), store.chatCommand)
	}
}

func TestProjectContextChatOpenRejectsCsrfAndInvalidBody(t *testing.T) {
	endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{})
	request := authenticatedRequest(http.MethodPut, "/context", `{}`)
	request.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.OpenProjectContextChat(response, request, uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusForbidden {
		t.Fatalf("csrf status=%d", response.Code)
	}

	response = httptest.NewRecorder()
	endpoint.OpenProjectContextChat(response, authenticatedRequest(http.MethodPut, "/context", `{}`), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("body status=%d", response.Code)
	}
}

func TestProjectEndpointHandlesSessionStoreAndPerMethodFailures(t *testing.T) {
	endpoint := projectEndpointForTest(t, &endpointAccountStore{accessErr: context.DeadlineExceeded}, &endpointProjectStore{})
	response := httptest.NewRecorder()
	endpoint.ListProjects(response, authenticatedRequest(http.MethodGet, "/api/v1/projects", ""), accountgenerated.ListProjectsParams{})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("session store status=%d", response.Code)
	}
	store := &endpointProjectStore{err: project.ErrNotFound}
	endpoint = projectEndpointForTest(t, activeAccount(nil), store)
	response = httptest.NewRecorder()
	endpoint.GetProject(response, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID, ""), uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusNotFound {
		t.Fatalf("get failure status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	endpoint.CreateProject(response, authenticatedRequest(http.MethodPost, "/api/v1/projects", `{"name":"Полянка","type":"interior_design","currencyCode":"RUB","customerUserId":null,"autoApproveExpenses":true}`))
	if response.Code != http.StatusNotFound {
		t.Fatalf("create failure status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	endpoint.AssignProjectCustomer(response, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/customer", `{"customerUserId":"00000000-0000-0000-0000-000000000002"}`), uuid.MustParse(endpointProjectID), accountgenerated.AssignProjectCustomerParams{IfMatch: `"1"`})
	if response.Code != http.StatusForbidden {
		t.Fatalf("ordinary assignment status=%d body=%s", response.Code, response.Body.String())
	}
	role := "super_admin"
	endpoint = projectEndpointForTest(t, activeAccount(&role), store)
	response = httptest.NewRecorder()
	endpoint.AssignProjectCustomer(response, authenticatedRequest(http.MethodPut, "/api/v1/projects/"+endpointProjectID+"/customer", `{"customerUserId":"00000000-0000-0000-0000-000000000002"}`), uuid.MustParse(endpointProjectID), accountgenerated.AssignProjectCustomerParams{IfMatch: `"1"`})
	if response.Code != http.StatusNotFound {
		t.Fatalf("assignment store failure status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProjectUpcomingEndpointsCreateAndList(t *testing.T) {
	start := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	status, description, location := "new", "Проверить планы", "Объект"
	item := project.UpcomingItem{ID: "00000000-0000-0000-0000-000000000020", ProjectID: endpointProjectID, Kind: project.UpcomingTask, Title: "Подготовить планы", Description: &description, Status: &status, Location: &location, EffectiveAt: start, EndsAt: &end, CreatedAt: start.Add(-time.Hour), Version: 1}
	plannedStart, plannedFinish := start.AddDate(0, 0, -5), start.AddDate(0, 1, 0)
	store := &endpointProjectStore{view: projectView(), upcoming: []project.UpcomingItem{item}, globalProjects: []project.GlobalCalendarProject{{ID: endpointProjectID, Name: "Полянка", Status: "active", PlannedStartOn: &plannedStart, PlannedFinishOn: &plannedFinish, Items: []project.UpcomingItem{item}}}, globalMore: true, globalTruncated: true}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)

	list := httptest.NewRecorder()
	endpoint.ListProjectUpcomingItems(list, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/upcoming?days=14", ""), uuid.MustParse(endpointProjectID), accountgenerated.ListProjectUpcomingItemsParams{Days: 14})
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"title":"Подготовить планы"`) || !strings.Contains(list.Body.String(), `"days":14`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	calendar := httptest.NewRecorder()
	endpoint.ListProjectCalendar(calendar, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/calendar?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z", ""), uuid.MustParse(endpointProjectID), accountgenerated.ListProjectCalendarParams{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)})
	if calendar.Code != http.StatusOK || !strings.Contains(calendar.Body.String(), `"title":"Подготовить планы"`) || !strings.Contains(calendar.Body.String(), `"rangeStart":"2026-09-01T00:00:00Z"`) {
		t.Fatalf("calendar status=%d body=%s", calendar.Code, calendar.Body.String())
	}

	global := httptest.NewRecorder()
	projectIDs := []openapi_types.UUID{uuid.MustParse(endpointProjectID)}
	endpoint.ListGlobalCalendar(global, authenticatedRequest(http.MethodGet, "/api/v1/calendar?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z&projectId="+endpointProjectID, ""), accountgenerated.ListGlobalCalendarParams{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ProjectId: &projectIDs})
	if global.Code != http.StatusOK || !strings.Contains(global.Body.String(), `"name":"Полянка"`) || !strings.Contains(global.Body.String(), `"hasMoreProjects":true`) || !strings.Contains(global.Body.String(), `"truncatedEvents":true`) {
		t.Fatalf("global calendar status=%d body=%s", global.Code, global.Body.String())
	}

	task := httptest.NewRecorder()
	endpoint.CreateProjectTask(task, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/tasks", `{"title":"Подготовить планы","description":"Проверить планы","dueAt":"2026-09-26T10:00:00Z"}`), uuid.MustParse(endpointProjectID))
	if task.Code != http.StatusCreated || store.task.Title != "Подготовить планы" || task.Header().Get("Location") == "" {
		t.Fatalf("task status=%d body=%s command=%#v", task.Code, task.Body.String(), store.task)
	}

	store.upcoming[0].Kind, store.upcoming[0].Status = project.UpcomingMeeting, nil
	meeting := httptest.NewRecorder()
	endpoint.CreateProjectMeeting(meeting, authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/meetings", `{"title":"Встреча на объекте","description":"Проверить планы","location":"Объект","startsAt":"2026-09-26T10:00:00Z","endsAt":"2026-09-26T11:00:00Z"}`), uuid.MustParse(endpointProjectID))
	if meeting.Code != http.StatusCreated || store.meeting.Title != "Встреча на объекте" || store.meeting.Location != "Объект" {
		t.Fatalf("meeting status=%d body=%s command=%#v", meeting.Code, meeting.Body.String(), store.meeting)
	}
}

func TestProjectUpcomingMutationRejectsBadInputAndCsrf(t *testing.T) {
	endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{view: projectView()})
	badCSRF := authenticatedRequest(http.MethodPost, "/api/v1/projects/"+endpointProjectID+"/tasks", `{}`)
	badCSRF.Header.Del("X-CSRF-Token")
	response := httptest.NewRecorder()
	endpoint.CreateProjectTask(response, badCSRF, uuid.MustParse(endpointProjectID))
	if response.Code != http.StatusForbidden {
		t.Fatalf("task csrf status=%d", response.Code)
	}
	for _, test := range []struct {
		meeting bool
		body    string
	}{{false, `{}`}, {false, `{"title":"x","dueAt":"bad"}`}, {true, `{}`}, {true, `{"title":"Встреча","startsAt":"2026-09-26T11:00:00Z","endsAt":"2026-09-26T10:00:00Z"}`}} {
		response = httptest.NewRecorder()
		if test.meeting {
			endpoint.CreateProjectMeeting(response, authenticatedRequest(http.MethodPost, "/meeting", test.body), uuid.MustParse(endpointProjectID))
		} else {
			endpoint.CreateProjectTask(response, authenticatedRequest(http.MethodPost, "/task", test.body), uuid.MustParse(endpointProjectID))
		}
		if response.Code != http.StatusBadRequest {
			t.Fatalf("meeting=%v body=%s status=%d response=%s", test.meeting, test.body, response.Code, response.Body.String())
		}
	}
}

func TestProjectTaskEndpointsListAndUpdateStatus(t *testing.T) {
	started := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	task := project.Task{ID: endpointMemberID, ProjectID: endpointProjectID, Title: "Подготовить планы", Status: project.TaskNew, DueAt: started.Add(24 * time.Hour), CreatedAt: started, Version: 1, CanEdit: true, CanChangeStatus: true, Assignees: []project.TaskAssignee{{UserID: endpointActorID, Login: "svetlana", FirstName: "Светлана"}}}
	store := &endpointProjectStore{view: projectView(), tasks: []project.Task{task}, canCreateTask: true, updatedTask: task}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)

	list := httptest.NewRecorder()
	endpoint.ListProjectTasks(list, authenticatedRequest(http.MethodGet, "/api/v1/projects/"+endpointProjectID+"/tasks", ""), uuid.MustParse(endpointProjectID))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"title":"Подготовить планы"`) || !strings.Contains(list.Body.String(), `"canCreate":true`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	store.updatedTask.Status, store.updatedTask.Version = project.TaskInProgress, 2
	response := httptest.NewRecorder()
	endpoint.UpdateProjectTask(response, authenticatedRequest(http.MethodPatch, "/api/v1/projects/"+endpointProjectID+"/tasks/"+endpointMemberID, `{"status":"in_progress"}`), uuid.MustParse(endpointProjectID), uuid.MustParse(endpointMemberID), accountgenerated.UpdateProjectTaskParams{IfMatch: 1})
	if response.Code != http.StatusOK || store.updateTask.Status != project.TaskInProgress || store.updateTask.ExpectedVersion != 1 {
		t.Fatalf("update status=%d body=%s command=%#v", response.Code, response.Body.String(), store.updateTask)
	}
}

func TestProjectCollaborationDocumentAndNotificationEndpoints(t *testing.T) {
	chatID := "00000000-0000-0000-0000-000000000030"
	documentID := "00000000-0000-0000-0000-000000000040"
	notificationID := "00000000-0000-0000-0000-000000000050"
	messageID := "00000000-0000-0000-0000-000000000060"
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	contextType, contextID, contextTitle := "task", endpointMemberID, "Подготовить планы"
	chat := project.ChatSummary{ID: chatID, ProjectID: endpointProjectID, Kind: "project", Name: "Согласование кухни", ContextType: &contextType, ContextID: &contextID, ContextTitle: &contextTitle, MemberCount: 2, CreatedAt: now, CanManage: true, Version: 1}
	member := project.ChatMember{UserID: endpointMemberID, Login: "ivan", FirstName: "Иван", JoinedAt: now, Removable: true}
	document := project.ProjectDocument{ID: documentID, ProjectID: endpointProjectID, Name: "plan.pdf", MediaType: "application/pdf", SizeBytes: 3, UploadedByUserID: endpointActorID, CreatedAt: now, CanDelete: true, Version: 1}
	store := &endpointProjectStore{
		view: projectView(), chats: project.ChatList{Items: []project.ChatSummary{chat}, CanCreate: true}, createdChat: chat,
		chatMembers: project.ChatMemberList{Items: []project.ChatMember{member}, CanManage: true}, chatMember: member,
		documents: project.ProjectDocumentList{Items: []project.ProjectDocument{document}, CanUpload: true}, document: document,
		documentContent: project.ProjectDocumentContent{ProjectDocument: document, Content: []byte("pdf")},
		notifications:   project.AccountNotificationList{Items: []project.AccountNotification{{ID: notificationID, ProjectID: endpointProjectID, EventType: "project.chat.created", Title: "Создан чат", Body: "Согласование кухни", Href: "/account/projects/" + endpointProjectID + "/chat/" + chatID, CreatedAt: now}}, UnreadCount: 1},
	}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)
	projectUUID, chatUUID := uuid.MustParse(endpointProjectID), uuid.MustParse(chatID)

	checks := []struct {
		name string
		want int
		call func(*httptest.ResponseRecorder)
	}{
		{"list chats", http.StatusOK, func(w *httptest.ResponseRecorder) {
			endpoint.ListProjectChats(w, authenticatedRequest(http.MethodGet, "/chats", ""), projectUUID)
		}},
		{"create chat", http.StatusCreated, func(w *httptest.ResponseRecorder) {
			endpoint.CreateProjectChat(w, authenticatedRequest(http.MethodPost, "/chats", `{"name":"Согласование кухни","memberUserIds":["`+endpointMemberID+`"]}`), projectUUID)
		}},
		{"update chat", http.StatusOK, func(w *httptest.ResponseRecorder) {
			endpoint.UpdateProjectChat(w, authenticatedRequest(http.MethodPatch, "/chat", `{"name":"Согласование кухни","version":1}`), projectUUID, chatUUID)
		}},
		{"list chat members", http.StatusOK, func(w *httptest.ResponseRecorder) {
			endpoint.ListProjectChatMembers(w, authenticatedRequest(http.MethodGet, "/members", ""), projectUUID, chatUUID)
		}},
		{"add chat member", http.StatusCreated, func(w *httptest.ResponseRecorder) {
			endpoint.AddProjectChatMember(w, authenticatedRequest(http.MethodPost, "/members", `{"userId":"`+endpointMemberID+`"}`), projectUUID, chatUUID)
		}},
		{"remove chat member", http.StatusNoContent, func(w *httptest.ResponseRecorder) {
			endpoint.RemoveProjectChatMember(w, authenticatedRequest(http.MethodDelete, "/members/"+endpointMemberID, ""), projectUUID, chatUUID, uuid.MustParse(endpointMemberID))
		}},
		{"mark chat read", http.StatusNoContent, func(w *httptest.ResponseRecorder) {
			endpoint.MarkProjectChatRead(w, authenticatedRequest(http.MethodPost, "/read", `{"messageId":"`+messageID+`"}`), projectUUID, chatUUID)
		}},
		{"list documents", http.StatusOK, func(w *httptest.ResponseRecorder) {
			endpoint.ListProjectDocuments(w, authenticatedRequest(http.MethodGet, "/documents", ""), projectUUID)
		}},
		{"upload document", http.StatusCreated, func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/documents", "pdf")
			request.Header.Set("Content-Type", "application/pdf")
			endpoint.UploadProjectDocument(w, request, projectUUID, accountgenerated.UploadProjectDocumentParams{XFileName: "plan.pdf"})
		}},
		{"download document", http.StatusOK, func(w *httptest.ResponseRecorder) {
			endpoint.DownloadProjectDocument(w, authenticatedRequest(http.MethodGet, "/documents/"+documentID, ""), projectUUID, uuid.MustParse(documentID))
		}},
		{"delete document", http.StatusNoContent, func(w *httptest.ResponseRecorder) {
			endpoint.DeleteProjectDocument(w, authenticatedRequest(http.MethodDelete, "/documents/"+documentID, ""), projectUUID, uuid.MustParse(documentID))
		}},
		{"list notifications", http.StatusOK, func(w *httptest.ResponseRecorder) {
			endpoint.ListAccountNotifications(w, authenticatedRequest(http.MethodGet, "/notifications", ""), accountgenerated.ListAccountNotificationsParams{})
		}},
		{"mark notification", http.StatusNoContent, func(w *httptest.ResponseRecorder) {
			endpoint.MarkAccountNotificationRead(w, authenticatedRequest(http.MethodPost, "/notifications/"+notificationID+"/read", ""), uuid.MustParse(notificationID))
		}},
		{"mark all notifications", http.StatusNoContent, func(w *httptest.ResponseRecorder) {
			endpoint.MarkAllAccountNotificationsRead(w, authenticatedRequest(http.MethodPost, "/notifications/read-all", ""))
		}},
		{"delete chat", http.StatusNoContent, func(w *httptest.ResponseRecorder) {
			endpoint.DeleteProjectChat(w, authenticatedRequest(http.MethodDelete, "/chat", ""), projectUUID, chatUUID, accountgenerated.DeleteProjectChatParams{Version: 1})
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			check.call(response)
			if response.Code != check.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, check.want, response.Body.String())
			}
		})
	}
}

func TestAccountNotificationEndpointsRejectUnauthenticatedAndCsrfRequests(t *testing.T) {
	notificationID := uuid.MustParse("00000000-0000-0000-0000-000000000050")
	unauthenticatedEndpoint := projectEndpointForTest(t, &endpointAccountStore{}, &endpointProjectStore{})
	response := httptest.NewRecorder()
	unauthenticatedEndpoint.ListAccountNotifications(response, httptest.NewRequest(http.MethodGet, "/notifications", nil), accountgenerated.ListAccountNotificationsParams{})
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated notification list status=%d body=%s", response.Code, response.Body.String())
	}
	request := authenticatedRequest(http.MethodPost, "/notification/read", "")
	request.Header.Del("X-CSRF-Token")
	response = httptest.NewRecorder()
	unauthenticatedEndpoint.MarkAccountNotificationRead(response, request, notificationID)
	if response.Code != http.StatusForbidden {
		t.Fatalf("notification mutation without csrf status=%d body=%s", response.Code, response.Body.String())
	}

	endpoint := projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{})
	pageSize := 50
	list := httptest.NewRecorder()
	endpoint.ListAccountNotifications(list, authenticatedRequest(http.MethodGet, "/notifications", ""), accountgenerated.ListAccountNotificationsParams{PageSize: &pageSize})
	if list.Code != http.StatusOK {
		t.Fatalf("notification list status=%d body=%s", list.Code, list.Body.String())
	}
	for _, call := range []func(*httptest.ResponseRecorder, *http.Request){
		func(w *httptest.ResponseRecorder, request *http.Request) {
			endpoint.MarkAccountNotificationRead(w, request, notificationID)
		},
		func(w *httptest.ResponseRecorder, request *http.Request) {
			endpoint.MarkAllAccountNotificationsRead(w, request)
		},
	} {
		request := authenticatedRequest(http.MethodPost, "/notifications/read", "")
		request.Header.Del("X-CSRF-Token")
		response := httptest.NewRecorder()
		call(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("notification csrf status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func TestProjectCollaborationEndpointsExposeValidationAndDomainErrors(t *testing.T) {
	chatID := uuid.MustParse("00000000-0000-0000-0000-000000000030")
	documentID := uuid.MustParse("00000000-0000-0000-0000-000000000040")
	notificationID := uuid.MustParse("00000000-0000-0000-0000-000000000050")
	projectID := uuid.MustParse(endpointProjectID)
	store := &endpointProjectStore{view: projectView(), err: project.ErrNotFound}
	endpoint := projectEndpointForTest(t, activeAccount(nil), store)
	validChat := `{"name":"Согласование кухни","memberUserIds":["` + endpointMemberID + `"]}`
	validUpdate := `{"name":"Новое название","version":1}`
	validMember := `{"userId":"` + endpointMemberID + `"}`
	validRead := `{"messageId":"00000000-0000-0000-0000-000000000060"}`

	domainChecks := []struct {
		name string
		call func(*httptest.ResponseRecorder)
	}{
		{"list chats", func(w *httptest.ResponseRecorder) {
			endpoint.ListProjectChats(w, authenticatedRequest(http.MethodGet, "/chats", ""), projectID)
		}},
		{"create chat", func(w *httptest.ResponseRecorder) {
			endpoint.CreateProjectChat(w, authenticatedRequest(http.MethodPost, "/chats", validChat), projectID)
		}},
		{"update chat", func(w *httptest.ResponseRecorder) {
			endpoint.UpdateProjectChat(w, authenticatedRequest(http.MethodPatch, "/chat", validUpdate), projectID, chatID)
		}},
		{"delete chat", func(w *httptest.ResponseRecorder) {
			endpoint.DeleteProjectChat(w, authenticatedRequest(http.MethodDelete, "/chat", ""), projectID, chatID, accountgenerated.DeleteProjectChatParams{Version: 1})
		}},
		{"list members", func(w *httptest.ResponseRecorder) {
			endpoint.ListProjectChatMembers(w, authenticatedRequest(http.MethodGet, "/members", ""), projectID, chatID)
		}},
		{"add member", func(w *httptest.ResponseRecorder) {
			endpoint.AddProjectChatMember(w, authenticatedRequest(http.MethodPost, "/members", validMember), projectID, chatID)
		}},
		{"remove member", func(w *httptest.ResponseRecorder) {
			endpoint.RemoveProjectChatMember(w, authenticatedRequest(http.MethodDelete, "/member", ""), projectID, chatID, uuid.MustParse(endpointMemberID))
		}},
		{"mark read", func(w *httptest.ResponseRecorder) {
			endpoint.MarkProjectChatRead(w, authenticatedRequest(http.MethodPost, "/read", validRead), projectID, chatID)
		}},
		{"list documents", func(w *httptest.ResponseRecorder) {
			endpoint.ListProjectDocuments(w, authenticatedRequest(http.MethodGet, "/documents", ""), projectID)
		}},
		{"upload document", func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/documents", "pdf")
			request.Header.Set("Content-Type", "application/pdf")
			endpoint.UploadProjectDocument(w, request, projectID, accountgenerated.UploadProjectDocumentParams{XFileName: "plan.pdf"})
		}},
		{"download document", func(w *httptest.ResponseRecorder) {
			endpoint.DownloadProjectDocument(w, authenticatedRequest(http.MethodGet, "/document", ""), projectID, documentID)
		}},
		{"delete document", func(w *httptest.ResponseRecorder) {
			endpoint.DeleteProjectDocument(w, authenticatedRequest(http.MethodDelete, "/document", ""), projectID, documentID)
		}},
		{"list notifications", func(w *httptest.ResponseRecorder) {
			endpoint.ListAccountNotifications(w, authenticatedRequest(http.MethodGet, "/notifications", ""), accountgenerated.ListAccountNotificationsParams{})
		}},
		{"read notification", func(w *httptest.ResponseRecorder) {
			endpoint.MarkAccountNotificationRead(w, authenticatedRequest(http.MethodPost, "/notification", ""), notificationID)
		}},
		{"read all notifications", func(w *httptest.ResponseRecorder) {
			endpoint.MarkAllAccountNotificationsRead(w, authenticatedRequest(http.MethodPost, "/notifications/read-all", ""))
		}},
	}
	for _, check := range domainChecks {
		t.Run("domain "+check.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			check.call(response)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}

	endpoint = projectEndpointForTest(t, activeAccount(nil), &endpointProjectStore{view: projectView()})
	invalidChecks := []func(*httptest.ResponseRecorder){
		func(w *httptest.ResponseRecorder) {
			endpoint.CreateProjectChat(w, authenticatedRequest(http.MethodPost, "/chats", `{`), projectID)
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.UpdateProjectChat(w, authenticatedRequest(http.MethodPatch, "/chat", `{`), projectID, chatID)
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.AddProjectChatMember(w, authenticatedRequest(http.MethodPost, "/members", `{`), projectID, chatID)
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.MarkProjectChatRead(w, authenticatedRequest(http.MethodPost, "/read", `{`), projectID, chatID)
		},
		func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/documents", "")
			endpoint.UploadProjectDocument(w, request, projectID, accountgenerated.UploadProjectDocumentParams{XFileName: "empty.pdf"})
		},
	}
	for index, call := range invalidChecks {
		response := httptest.NewRecorder()
		call(response)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid check %d status=%d body=%s", index, response.Code, response.Body.String())
		}
	}
}
