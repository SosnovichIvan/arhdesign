package project

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeCipher struct {
	plaintext   []byte
	messageType string
	err         error
}

func (cipher *fakeCipher) Encrypt(plaintext []byte, messageType string) ([]byte, int, error) {
	cipher.plaintext, cipher.messageType = append([]byte(nil), plaintext...), messageType
	return []byte("encrypted-project-notification"), 7, cipher.err
}

type fakeStore struct {
	createCommand    CreateCommand
	createView       View
	createErr        error
	listQuery        ListQuery
	listViews        []View
	listErr          error
	getActor         Actor
	getID            string
	getView          View
	getErr           error
	assignCommand    AssignCustomerCommand
	assignView       View
	assignErr        error
	updateCommand    UpdateCommand
	updateView       View
	updateErr        error
	archiveCommand   ArchiveCommand
	archiveErr       error
	upcomingQuery    UpcomingQuery
	upcomingItems    []UpcomingItem
	upcomingErr      error
	globalQuery      GlobalCalendarQuery
	globalProjects   []GlobalCalendarProject
	globalMore       bool
	globalTruncated  bool
	taskCommand      CreateTaskCommand
	taskItem         UpcomingItem
	tasks            []Task
	canCreateTask    bool
	updateTask       UpdateTaskCommand
	updatedTask      Task
	taskErr          error
	meetingCommand   CreateMeetingCommand
	meetingItem      UpcomingItem
	meetingErr       error
	members          []Member
	membersManage    bool
	membersErr       error
	candidateQuery   MemberCandidateQuery
	candidates       []MemberCandidate
	memberCommand    MemberCommand
	member           Member
	memberErr        error
	financeSummary   FinanceSummary
	expenses         []Expense
	canCreateExpense bool
	expenseCommand   CreateExpenseCommand
	expense          Expense
	financeErr       error
	materials        []Material
	material         Material
	materialCreate   CreateMaterialCommand
	materialUpdate   UpdateMaterialCommand
	materialDelete   DeleteMaterialCommand
	chatQuery        ChatQuery
	chatPage         ProjectChatPage
	chatCommand      CreateChatMessageCommand
	chatMessage      ChatMessage
	chatContext      ChatContext
	contextCommand   OpenContextChatCommand
	chatErr          error
}

func (store *fakeStore) CreateProject(_ context.Context, command CreateCommand) (View, error) {
	store.createCommand = command
	return store.createView, store.createErr
}
func (store *fakeStore) ListProjects(_ context.Context, query ListQuery) ([]View, error) {
	store.listQuery = query
	return store.listViews, store.listErr
}
func (store *fakeStore) GetProject(_ context.Context, actor Actor, id string) (View, error) {
	store.getActor, store.getID = actor, id
	return store.getView, store.getErr
}
func (store *fakeStore) AssignProjectCustomer(_ context.Context, command AssignCustomerCommand) (View, error) {
	store.assignCommand = command
	return store.assignView, store.assignErr
}
func (store *fakeStore) UpdateProject(_ context.Context, command UpdateCommand) (View, error) {
	store.updateCommand = command
	return store.updateView, store.updateErr
}
func (store *fakeStore) ArchiveProject(_ context.Context, command ArchiveCommand) error {
	store.archiveCommand = command
	return store.archiveErr
}
func (store *fakeStore) ListProjectUpcoming(_ context.Context, query UpcomingQuery) ([]UpcomingItem, error) {
	store.upcomingQuery = query
	return store.upcomingItems, store.upcomingErr
}
func (store *fakeStore) ListGlobalCalendar(_ context.Context, query GlobalCalendarQuery) ([]GlobalCalendarProject, bool, bool, error) {
	store.globalQuery = query
	return store.globalProjects, store.globalMore, store.globalTruncated, store.upcomingErr
}
func (store *fakeStore) CreateProjectTask(_ context.Context, command CreateTaskCommand) (UpcomingItem, error) {
	store.taskCommand = command
	return store.taskItem, store.taskErr
}
func (store *fakeStore) ListProjectTasks(_ context.Context, _ TaskQuery) ([]Task, bool, error) {
	return store.tasks, store.canCreateTask, store.taskErr
}
func (store *fakeStore) UpdateProjectTask(_ context.Context, command UpdateTaskCommand) (Task, error) {
	store.updateTask = command
	return store.updatedTask, store.taskErr
}
func (store *fakeStore) CreateProjectMeeting(_ context.Context, command CreateMeetingCommand) (UpcomingItem, error) {
	store.meetingCommand = command
	return store.meetingItem, store.meetingErr
}
func (store *fakeStore) ListProjectMembers(_ context.Context, _ Actor, _ string) ([]Member, bool, error) {
	return store.members, store.membersManage, store.membersErr
}
func (store *fakeStore) SearchProjectMemberCandidates(_ context.Context, query MemberCandidateQuery) ([]MemberCandidate, error) {
	store.candidateQuery = query
	return store.candidates, store.membersErr
}
func (store *fakeStore) AddProjectMember(_ context.Context, command MemberCommand) (Member, error) {
	store.memberCommand = command
	return store.member, store.memberErr
}
func (store *fakeStore) RemoveProjectMember(_ context.Context, command MemberCommand) error {
	store.memberCommand = command
	return store.memberErr
}
func (store *fakeStore) GetProjectFinanceSummary(_ context.Context, _ Actor, _ string) (FinanceSummary, error) {
	return store.financeSummary, store.financeErr
}
func (store *fakeStore) ListProjectExpenses(_ context.Context, _ Actor, _ string) ([]Expense, bool, error) {
	return store.expenses, store.canCreateExpense, store.financeErr
}
func (store *fakeStore) CreateProjectExpense(_ context.Context, command CreateExpenseCommand) (Expense, error) {
	store.expenseCommand = command
	return store.expense, store.financeErr
}
func (store *fakeStore) ListProjectMaterials(context.Context, Actor, string) ([]Material, bool, bool, error) {
	return store.materials, true, true, store.financeErr
}
func (store *fakeStore) CreateProjectMaterial(_ context.Context, command CreateMaterialCommand) (Material, error) {
	store.materialCreate = command
	return store.material, store.financeErr
}
func (store *fakeStore) UpdateProjectMaterial(_ context.Context, command UpdateMaterialCommand) (Material, error) {
	store.materialUpdate = command
	return store.material, store.financeErr
}
func (store *fakeStore) DeleteProjectMaterial(_ context.Context, command DeleteMaterialCommand) error {
	store.materialDelete = command
	return store.financeErr
}
func (store *fakeStore) ListProjectChat(_ context.Context, query ChatQuery) (ProjectChatPage, error) {
	store.chatQuery = query
	return store.chatPage, store.chatErr
}
func (store *fakeStore) CreateProjectChatMessage(_ context.Context, command CreateChatMessageCommand) (ChatMessage, error) {
	store.chatCommand = command
	return store.chatMessage, store.chatErr
}
func (store *fakeStore) OpenProjectContextChat(_ context.Context, command OpenContextChatCommand) (ChatContext, error) {
	store.contextCommand = command
	return store.chatContext, store.chatErr
}
func (store *fakeStore) ListProjectContextChat(_ context.Context, query ChatQuery) (ProjectChatPage, error) {
	store.chatQuery = query
	return store.chatPage, store.chatErr
}
func (store *fakeStore) CreateProjectContextChatMessage(_ context.Context, command CreateChatMessageCommand) (ChatMessage, error) {
	store.chatCommand = command
	return store.chatMessage, store.chatErr
}

const actorID = "00000000-0000-0000-0000-000000000001"
const customerID = "00000000-0000-0000-0000-000000000002"

func newService(t *testing.T, store Store) (*Service, time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	service, err := NewService(store, []byte(strings.Repeat("k", 32)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return service, now.UTC()
}

func validCreate() CreateRequest {
	return CreateRequest{Name: " Квартира на Полянке ", Type: " interior_design ", CurrencyCode: " rub ", RequestID: "request-123"}
}

func TestProjectChatValidationAndCommands(t *testing.T) {
	messageID := "00000000-0000-0000-0000-000000000033"
	store := &fakeStore{
		chatPage:    ProjectChatPage{ChatID: customerID, ProjectID: customerID},
		chatMessage: ChatMessage{ID: messageID, Body: "Готово"},
	}
	service, now := newService(t, store)

	page, err := service.ListChat(context.Background(), Actor{UserID: actorID}, customerID, 0, "")
	if err != nil || page.ChatID != customerID || store.chatQuery.PageSize != 50 || store.chatQuery.ProjectID != customerID {
		t.Fatalf("page=%#v query=%#v err=%v", page, store.chatQuery, err)
	}
	cursor := "00000000-0000-0000-0000-000000000044"
	_, err = service.ListChat(context.Background(), Actor{UserID: actorID}, customerID, 25, cursor)
	if err != nil || store.chatQuery.BeforeID == nil || *store.chatQuery.BeforeID != cursor {
		t.Fatalf("query=%#v err=%v", store.chatQuery, err)
	}

	created, err := service.SendChatMessage(context.Background(), Actor{UserID: actorID}, customerID, CreateChatMessageRequest{
		Body: "  Готово  ", ClientMessageID: cursor, RequestID: "request-chat",
	})
	if err != nil || created.ID != messageID {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	if store.chatCommand.Body != "Готово" || store.chatCommand.ClientMessageID != cursor || !store.chatCommand.Now.Equal(now) || store.chatCommand.RequestID != "request-chat" {
		t.Fatalf("command=%#v", store.chatCommand)
	}
}

func TestProjectChatRejectsInvalidInput(t *testing.T) {
	service, _ := newService(t, &fakeStore{})
	actor := Actor{UserID: actorID}
	validClientID := "00000000-0000-0000-0000-000000000044"

	for name, input := range map[string]struct {
		pageSize int
		cursor   string
	}{
		"large page": {pageSize: 101},
		"bad cursor": {pageSize: 20, cursor: "not-a-uuid"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.ListChat(context.Background(), actor, customerID, input.pageSize, input.cursor); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}

	requests := map[string]CreateChatMessageRequest{
		"empty body":       {Body: "   ", ClientMessageID: validClientID, RequestID: "request-chat"},
		"bad client id":    {Body: "Текст", ClientMessageID: "bad", RequestID: "request-chat"},
		"short request id": {Body: "Текст", ClientMessageID: validClientID, RequestID: "short"},
	}
	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			if _, err := service.SendChatMessage(context.Background(), actor, customerID, request); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestProjectContextChatValidationAndCommands(t *testing.T) {
	chatID := "00000000-0000-0000-0000-000000000055"
	contextID := "00000000-0000-0000-0000-000000000066"
	store := &fakeStore{
		chatContext: ChatContext{ChatID: chatID, ProjectID: customerID, ContextType: "task", ContextID: contextID, ContextTitle: "Планировка", Name: "Обсуждение планировки"},
		chatPage:    ProjectChatPage{ChatID: chatID, ProjectID: customerID},
		chatMessage: ChatMessage{ID: contextID, ChatID: chatID, Body: "Проверила"},
	}
	service, now := newService(t, store)
	actor := Actor{UserID: actorID}

	opened, err := service.OpenContextChat(context.Background(), actor, customerID, OpenContextChatRequest{
		ContextType: " task ", ContextID: contextID, Name: " Обсуждение планировки ", RequestID: "request-context-chat",
	})
	if err != nil || opened.ChatID != chatID || store.contextCommand.Name != "Обсуждение планировки" || store.contextCommand.ContextType != "task" || !store.contextCommand.Now.Equal(now) {
		t.Fatalf("opened=%#v command=%#v err=%v", opened, store.contextCommand, err)
	}
	page, err := service.ListContextChat(context.Background(), actor, customerID, chatID, 25, "")
	if err != nil || page.ChatID != chatID || store.chatQuery.ChatID != chatID || store.chatQuery.PageSize != 25 {
		t.Fatalf("page=%#v query=%#v err=%v", page, store.chatQuery, err)
	}
	message, err := service.SendContextChatMessage(context.Background(), actor, customerID, chatID, CreateChatMessageRequest{
		Body: " Проверила ", ClientMessageID: contextID, RequestID: "request-context-message",
	})
	if err != nil || message.ChatID != chatID || store.chatCommand.ChatID != chatID || store.chatCommand.Body != "Проверила" {
		t.Fatalf("message=%#v command=%#v err=%v", message, store.chatCommand, err)
	}
}

func TestProjectContextChatRejectsInvalidInput(t *testing.T) {
	service, _ := newService(t, &fakeStore{})
	actor := Actor{UserID: actorID}
	contextID := "00000000-0000-0000-0000-000000000066"
	chatID := "00000000-0000-0000-0000-000000000055"

	for name, request := range map[string]OpenContextChatRequest{
		"unsupported type": {ContextType: "event", ContextID: contextID, Name: "Событие", RequestID: "request-context"},
		"invalid context":  {ContextType: "task", ContextID: "bad", Name: "Задача", RequestID: "request-context"},
		"short name":       {ContextType: "task", ContextID: contextID, Name: "x", RequestID: "request-context"},
		"short request":    {ContextType: "task", ContextID: contextID, Name: "Задача", RequestID: "short"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.OpenContextChat(context.Background(), actor, customerID, request); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if _, err := service.ListContextChat(context.Background(), actor, customerID, "bad", 50, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("list err=%v", err)
	}
	if _, err := service.SendContextChatMessage(context.Background(), actor, customerID, chatID, CreateChatMessageRequest{Body: "", ClientMessageID: contextID, RequestID: "request-context"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("send err=%v", err)
	}
}

func TestProjectCollaborationEventsCreateEncryptedTelegramNotifications(t *testing.T) {
	store := &fakeStore{chatMessage: ChatMessage{ID: customerID}}
	service, now := newService(t, store)
	cipher := &fakeCipher{}
	if err := service.ConfigureNotifications(cipher); err != nil {
		t.Fatal(err)
	}
	lastName := "Иванова"
	actor := Actor{UserID: actorID, Login: "anna", FirstName: "Анна", LastName: &lastName}
	clientMessageID := "00000000-0000-0000-0000-000000000044"

	if _, err := service.SendChatMessage(context.Background(), actor, customerID, CreateChatMessageRequest{Body: "Проверьте планировку", ClientMessageID: clientMessageID, RequestID: "request-chat"}); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.chatCommand.Notification, cipher, "project.chat.message_created", "Анна Иванова (@anna)", "Проверьте планировку")

	if _, err := service.SendContextChatMessage(context.Background(), actor, customerID, customerID, CreateChatMessageRequest{Body: "Уточните материал", ClientMessageID: clientMessageID, RequestID: "request-context"}); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.chatCommand.Notification, cipher, "project.context_chat.message_created", "Анна Иванова (@anna)", "Уточните материал")

	due := now.Add(48 * time.Hour)
	if _, err := service.CreateTask(context.Background(), actor, customerID, CreateTaskRequest{Title: "Подготовить планы", AssigneeUserIDs: []string{customerID}, DueAt: due, RequestID: "request-task"}); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.taskCommand.Notification, cipher, "project.task.assigned", "Подготовить планы", due.Format(time.RFC3339))

	starts := now.Add(72 * time.Hour)
	if _, err := service.CreateMeeting(context.Background(), actor, customerID, CreateMeetingRequest{Title: "Встреча на объекте", StartsAt: starts, EndsAt: starts.Add(time.Hour), RequestID: "request-meeting"}); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.meetingCommand.Notification, cipher, "project.meeting.created", "Встреча на объекте", starts.Format(time.RFC3339))

	if _, err := service.UpdateTask(context.Background(), actor, customerID, customerID, UpdateTaskRequest{Status: TaskInProgress, ExpectedVersion: 1, RequestID: "request-task-status"}); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.updateTask.Notification, cipher, "project.task.status_changed", "В работе")

	vendor := "Свет"
	if _, err := service.CreateExpense(context.Background(), actor, customerID, "request-expense", CreateExpenseRequest{AmountMinor: 125_050, Category: "materials", Description: "Светильники", VendorName: &vendor, IdempotencyKey: "expense-notify-key"}); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.expenseCommand.Notification, cipher, "project.expense.created", "Светильники", "1250.50")

	material := MaterialInput{Category: "materials", Name: "Диван", SupplierName: "Фабрика", ContractAmountMinor: 250_000, PaymentStatus: "unpaid", DeliveryStatus: "expected"}
	if _, err := service.CreateMaterial(context.Background(), actor, customerID, "request-material", material); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.materialCreate.Notification, cipher, "project.material.created", "Диван", "Фабрика")
	if _, err := service.UpdateMaterial(context.Background(), actor, customerID, customerID, "request-material-update", 1, material); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.materialUpdate.Notification, cipher, "project.material.updated", "Диван")
	if err := service.DeleteMaterial(context.Background(), actor, customerID, customerID, "request-material-delete", 2, "Дубликат"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.materialDelete.Notification, cipher, "project.material.deleted", "Дубликат")

	if _, err := service.AddMember(context.Background(), actor, customerID, customerID, "request-member-add"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.memberCommand.Notification, cipher, "project.member.added", "добавлен новый участник")
	if err := service.RemoveMember(context.Background(), actor, customerID, customerID, "request-member-remove"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.memberCommand.Notification, cipher, "project.member.removed", "удалён из проекта")
}

func TestProjectCollaborationEventFailsBeforeStoreWhenEncryptionFails(t *testing.T) {
	store := &fakeStore{}
	service, now := newService(t, store)
	cipher := &fakeCipher{err: errors.New("cipher unavailable")}
	if err := service.ConfigureNotifications(cipher); err != nil {
		t.Fatal(err)
	}
	_, err := service.CreateTask(context.Background(), Actor{UserID: actorID}, customerID, CreateTaskRequest{Title: "Задача", DueAt: now.Add(time.Hour), RequestID: "request-task"})
	if err == nil || store.taskCommand.ProjectID != "" {
		t.Fatalf("error=%v command=%#v", err, store.taskCommand)
	}
}

func assertProjectNotification(t *testing.T, notification *EventNotification, cipher *fakeCipher, messageType string, fragments ...string) {
	t.Helper()
	if notification == nil || notification.MessageType != messageType || notification.PayloadKeyVersion != 7 || cipher.messageType != messageType {
		t.Fatalf("notification=%#v cipher type=%q", notification, cipher.messageType)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(cipher.plaintext, &payload); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range fragments {
		if !strings.Contains(payload.Text, fragment) {
			t.Fatalf("payload %q does not contain %q", payload.Text, fragment)
		}
	}
}

func TestFinanceSummaryAndExpenseValidation(t *testing.T) {
	store := &fakeStore{financeSummary: FinanceSummary{CurrencyCode: "RUB", PendingExpenseMinor: 12_500}, canCreateExpense: true, expense: Expense{ID: customerID}}
	service, now := newService(t, store)
	summary, err := service.FinanceSummary(context.Background(), Actor{UserID: actorID}, customerID)
	if err != nil || summary.PendingExpenseMinor != 12_500 {
		t.Fatalf("summary=%#v err=%v", summary, err)
	}
	list, err := service.ListExpenses(context.Background(), Actor{UserID: actorID}, customerID)
	if err != nil || !list.CanCreateExpense {
		t.Fatalf("list=%#v err=%v", list, err)
	}
	vendor := "  Свет  "
	planned := now.AddDate(0, 0, 3)
	request := CreateExpenseRequest{AmountMinor: 125_050, Category: "materials", Description: "  Светильники  ", VendorName: &vendor, PlannedPaymentOn: &planned, IdempotencyKey: "expense-key-1"}
	if _, err = service.CreateExpense(context.Background(), Actor{UserID: actorID}, customerID, "request-123", request); err != nil {
		t.Fatal(err)
	}
	if store.expenseCommand.Request.Description != "Светильники" || *store.expenseCommand.Request.VendorName != "Свет" || store.expenseCommand.RequestHash == [32]byte{} || !store.expenseCommand.Now.Equal(now) {
		t.Fatalf("command=%#v", store.expenseCommand)
	}
	request.IdempotencyKey = "short"
	if _, err = service.CreateExpense(context.Background(), Actor{UserID: actorID}, customerID, "request-123", request); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("short key err=%v", err)
	}
	request.IdempotencyKey = "expense-key-2"
	request.AmountMinor = 0
	if _, err = service.CreateExpense(context.Background(), Actor{UserID: actorID}, customerID, "request-123", request); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("zero amount err=%v", err)
	}
	if _, err = service.FinanceSummary(context.Background(), Actor{UserID: "bad"}, customerID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid summary actor error=%v", err)
	}
	if _, err = service.FinanceSummary(context.Background(), Actor{UserID: actorID}, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid summary project error=%v", err)
	}
	if _, err = service.ListExpenses(context.Background(), Actor{UserID: "bad"}, customerID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid list actor error=%v", err)
	}
	if _, err = service.ListExpenses(context.Background(), Actor{UserID: actorID}, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid list project error=%v", err)
	}
	longVendor := strings.Repeat("п", 201)
	for name, invalid := range map[string]CreateExpenseRequest{
		"project":     {AmountMinor: 1, Category: "other", Description: "Расход", IdempotencyKey: "expense-key"},
		"category":    {AmountMinor: 1, Category: "invalid", Description: "Расход", IdempotencyKey: "expense-key"},
		"vendor":      {AmountMinor: 1, Category: "other", Description: "Расход", VendorName: &longVendor, IdempotencyKey: "expense-key"},
		"description": {AmountMinor: 1, Category: "other", Description: "x", IdempotencyKey: "expense-key"},
	} {
		t.Run(name, func(t *testing.T) {
			projectID := customerID
			if name == "project" {
				projectID = "bad"
			}
			if _, createErr := service.CreateExpense(context.Background(), Actor{UserID: actorID}, projectID, "request-123", invalid); createErr == nil {
				t.Fatalf("input=%#v", invalid)
			}
		})
	}
}

func TestTaskListAndUpdateValidation(t *testing.T) {
	store := &fakeStore{tasks: []Task{{ID: customerID}}, canCreateTask: true, updatedTask: Task{ID: customerID}}
	service, _ := newService(t, store)
	actor := Actor{UserID: actorID}
	list, err := service.ListTasks(context.Background(), actor, customerID)
	if err != nil || !list.CanCreate || len(list.Items) != 1 {
		t.Fatalf("list=%#v error=%v", list, err)
	}
	request := UpdateTaskRequest{Status: TaskInProgress, ExpectedVersion: 1, RequestID: "request-task-update"}
	if _, err = service.UpdateTask(context.Background(), actor, customerID, customerID, request); err != nil || store.updateTask.Status != TaskInProgress {
		t.Fatalf("command=%#v error=%v", store.updateTask, err)
	}
	for _, testCase := range []struct {
		actor     Actor
		projectID string
		taskID    string
		request   UpdateTaskRequest
	}{
		{Actor{UserID: "bad"}, customerID, customerID, request},
		{actor, "bad", customerID, request},
		{actor, customerID, "bad", request},
		{actor, customerID, customerID, UpdateTaskRequest{Status: "invalid", ExpectedVersion: 1, RequestID: "request-task-update"}},
		{actor, customerID, customerID, UpdateTaskRequest{Status: TaskInProgress, ExpectedVersion: 0, RequestID: "request-task-update"}},
		{actor, customerID, customerID, UpdateTaskRequest{Status: TaskInProgress, ExpectedVersion: 1, RequestID: "short"}},
	} {
		if _, updateErr := service.UpdateTask(context.Background(), testCase.actor, testCase.projectID, testCase.taskID, testCase.request); updateErr == nil {
			t.Fatalf("case=%#v", testCase)
		}
	}
	if _, err = service.ListTasks(context.Background(), Actor{UserID: "bad"}, customerID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid list actor error=%v", err)
	}
	if _, err = service.ListTasks(context.Background(), actor, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid list project error=%v", err)
	}
}

func TestMaterialLifecycleValidationAndCommands(t *testing.T) {
	store := &fakeStore{material: Material{ID: customerID}, materials: []Material{{ID: customerID}}}
	service, now := newService(t, store)
	list, err := service.ListMaterials(context.Background(), Actor{UserID: actorID}, customerID)
	if err != nil || len(list.Items) != 1 || !list.CanCreate || !list.CanViewFinancialInformation {
		t.Fatalf("list=%#v error=%v", list, err)
	}
	input := MaterialInput{Category: " materials ", Name: " Диван ", SupplierName: " Фабрика ", ContractAmountMinor: 250_000, PaymentStatus: "unpaid", DeliveryStatus: "expected"}
	created, err := service.CreateMaterial(context.Background(), Actor{UserID: actorID}, customerID, "request-material", input)
	if err != nil || created.ID != customerID || store.materialCreate.Input.Name != "Диван" || !store.materialCreate.Now.Equal(now) {
		t.Fatalf("created=%#v command=%#v err=%v", created, store.materialCreate, err)
	}
	_, err = service.UpdateMaterial(context.Background(), Actor{UserID: actorID}, customerID, customerID, "request-material", 2, input)
	if err != nil || store.materialUpdate.ExpectedVersion != 2 {
		t.Fatalf("update=%#v err=%v", store.materialUpdate, err)
	}
	if err = service.DeleteMaterial(context.Background(), Actor{UserID: actorID}, customerID, customerID, "request-material", 3, " Дубликат "); err != nil || store.materialDelete.Reason != "Дубликат" {
		t.Fatalf("delete=%#v err=%v", store.materialDelete, err)
	}
	actual := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	input.ActualDeliveryOn = &actual
	if _, err = service.CreateMaterial(context.Background(), Actor{UserID: actorID}, customerID, "request-material", input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid delivery semantics, got %v", err)
	}
	longContact, longContract, longNotes := strings.Repeat("к", 501), strings.Repeat("д", 201), strings.Repeat("н", 5001)
	badID := "not-a-uuid"
	badInputs := map[string]MaterialInput{
		"category":          {Category: "unknown", Name: "Диван", SupplierName: "Фабрика", ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"payment":           {Category: "materials", Name: "Диван", SupplierName: "Фабрика", ContractAmountMinor: 1, PaymentStatus: "unknown", DeliveryStatus: "expected"},
		"delivery":          {Category: "materials", Name: "Диван", SupplierName: "Фабрика", ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "unknown"},
		"short name":        {Category: "materials", Name: "x", SupplierName: "Фабрика", ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"short supplier":    {Category: "materials", Name: "Диван", SupplierName: "x", ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"amount":            {Category: "materials", Name: "Диван", SupplierName: "Фабрика", ContractAmountMinor: 0, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"contact":           {Category: "materials", Name: "Диван", SupplierName: "Фабрика", ContactInfo: &longContact, ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"contract":          {Category: "materials", Name: "Диван", SupplierName: "Фабрика", ContractReference: &longContract, ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"notes":             {Category: "materials", Name: "Диван", SupplierName: "Фабрика", Notes: &longNotes, ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"linked expense":    {Category: "materials", Name: "Диван", SupplierName: "Фабрика", LinkedExpenseID: &badID, ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
		"actual unexpected": {Category: "materials", Name: "Диван", SupplierName: "Фабрика", ActualDeliveryOn: &actual, ContractAmountMinor: 1, PaymentStatus: "unpaid", DeliveryStatus: "expected"},
	}
	for name, invalid := range badInputs {
		t.Run(name, func(t *testing.T) {
			if _, createErr := service.CreateMaterial(context.Background(), Actor{UserID: actorID}, customerID, "request-material", invalid); !errors.Is(createErr, ErrInvalidInput) {
				t.Fatalf("input=%#v error=%v", invalid, createErr)
			}
		})
	}
	if _, err = service.ListMaterials(context.Background(), Actor{UserID: "bad"}, customerID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid list actor error=%v", err)
	}
	if _, err = service.ListMaterials(context.Background(), Actor{UserID: actorID}, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid list project error=%v", err)
	}
	if _, err = service.UpdateMaterial(context.Background(), Actor{UserID: actorID}, customerID, customerID, "request-material", 0, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid update version error=%v", err)
	}
	if err = service.DeleteMaterial(context.Background(), Actor{UserID: actorID}, customerID, customerID, "request-material", 1, "x"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid delete reason error=%v", err)
	}
}

func TestCreateAssignsOrdinaryCreatorAndAllowsCustomerlessSuperAdmin(t *testing.T) {
	store := &fakeStore{createView: View{ID: actorID}}
	service, now := newService(t, store)

	if _, err := service.Create(context.Background(), Actor{UserID: actorID}, validCreate()); err != nil {
		t.Fatal(err)
	}
	if store.createCommand.CustomerUserID == nil || *store.createCommand.CustomerUserID != actorID || len(store.createCommand.AdminUserIDs) != 1 || store.createCommand.AdminUserIDs[0] != actorID {
		t.Fatalf("ordinary command=%#v", store.createCommand)
	}
	if store.createCommand.Name != "Квартира на Полянке" || store.createCommand.Type != "interior_design" || store.createCommand.CurrencyCode != "RUB" || !store.createCommand.Now.Equal(now) {
		t.Fatalf("normalized command=%#v", store.createCommand)
	}

	request := validCreate()
	request.AdminUserIDs = []string{customerID, customerID}
	if _, err := service.Create(context.Background(), Actor{UserID: actorID, SuperAdmin: true}, request); err != nil {
		t.Fatal(err)
	}
	if store.createCommand.CustomerUserID != nil || len(store.createCommand.AdminUserIDs) != 1 || store.createCommand.AdminUserIDs[0] != customerID {
		t.Fatalf("super-admin command=%#v", store.createCommand)
	}
}

func TestCreateQueuesEncryptedNotificationOnlyForOrdinaryCreator(t *testing.T) {
	store := &fakeStore{createView: View{ID: actorID}}
	service, _ := newService(t, store)
	if err := service.ConfigureNotifications(nil); err == nil {
		t.Fatal("expected nil notification cipher to be rejected")
	}
	cipher := &fakeCipher{}
	if err := service.ConfigureNotifications(cipher); err != nil {
		t.Fatal(err)
	}
	lastName := "Полисмакова"
	actor := Actor{UserID: actorID, Login: "sveta", FirstName: "Светлана", LastName: &lastName}
	if _, err := service.Create(context.Background(), actor, validCreate()); err != nil {
		t.Fatal(err)
	}
	if store.createCommand.Notification == nil || store.createCommand.Notification.MessageType != "project.created" || store.createCommand.Notification.PayloadKeyVersion != 7 {
		t.Fatalf("notification=%#v", store.createCommand.Notification)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(cipher.plaintext, &payload); err != nil || !strings.Contains(payload.Text, "Светлана Полисмакова (@sveta)") || !strings.Contains(payload.Text, "Квартира на Полянке") {
		t.Fatalf("payload=%q error=%v", payload.Text, err)
	}
	if _, err := service.Create(context.Background(), Actor{UserID: actorID, SuperAdmin: true}, validCreate()); err != nil {
		t.Fatal(err)
	}
	if store.createCommand.Notification != nil {
		t.Fatalf("super-admin notification=%#v", store.createCommand.Notification)
	}
	cipher.err = errors.New("cipher unavailable")
	if _, err := service.Create(context.Background(), actor, validCreate()); err == nil {
		t.Fatal("expected encryption failure")
	}
}

func TestCollaborationMutationsStopWhenNotificationEncryptionFails(t *testing.T) {
	store := &fakeStore{}
	service, now := newService(t, store)
	cipher := &fakeCipher{err: errors.New("cipher unavailable")}
	if err := service.ConfigureNotifications(cipher); err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: actorID, Login: "svetlana"}
	clientMessageID := "00000000-0000-0000-0000-000000000044"
	materialInput := MaterialInput{Category: "materials", Name: "Керамогранит", SupplierName: "Поставщик", ContractAmountMinor: 125050, PaymentStatus: "unpaid", DeliveryStatus: "expected"}
	tests := []struct {
		name string
		call func() error
	}{
		{"project chat", func() error {
			_, err := service.SendChatMessage(context.Background(), actor, customerID, CreateChatMessageRequest{Body: "Комментарий", ClientMessageID: clientMessageID, RequestID: "request-chat"})
			return err
		}},
		{"context chat", func() error {
			_, err := service.SendContextChatMessage(context.Background(), actor, customerID, customerID, CreateChatMessageRequest{Body: "Комментарий", ClientMessageID: clientMessageID, RequestID: "request-context-chat"})
			return err
		}},
		{"task create", func() error {
			_, err := service.CreateTask(context.Background(), actor, customerID, CreateTaskRequest{Title: "Чертежи", DueAt: now.Add(time.Hour), RequestID: "request-task"})
			return err
		}},
		{"task update", func() error {
			_, err := service.UpdateTask(context.Background(), actor, customerID, customerID, UpdateTaskRequest{Status: TaskInProgress, ExpectedVersion: 1, RequestID: "request-task-update"})
			return err
		}},
		{"meeting", func() error {
			_, err := service.CreateMeeting(context.Background(), actor, customerID, CreateMeetingRequest{Title: "Встреча", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), RequestID: "request-meeting"})
			return err
		}},
		{"expense", func() error {
			_, err := service.CreateExpense(context.Background(), actor, customerID, "request-expense", CreateExpenseRequest{AmountMinor: 100, Category: "other", Description: "Расход", IdempotencyKey: "expense-key"})
			return err
		}},
		{"material create", func() error {
			_, err := service.CreateMaterial(context.Background(), actor, customerID, "request-material", materialInput)
			return err
		}},
		{"material update", func() error {
			_, err := service.UpdateMaterial(context.Background(), actor, customerID, customerID, "request-material", 1, materialInput)
			return err
		}},
		{"material delete", func() error {
			return service.DeleteMaterial(context.Background(), actor, customerID, customerID, "request-material", 1, "Другой поставщик")
		}},
		{"member add", func() error {
			_, err := service.AddMember(context.Background(), actor, customerID, customerID, "request-member")
			return err
		}},
		{"member remove", func() error {
			return service.RemoveMember(context.Background(), actor, customerID, customerID, "request-member")
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); err == nil || !strings.Contains(err.Error(), "cipher unavailable") {
				t.Fatalf("error=%v", err)
			}
		})
	}
	if preview := notificationPreview(strings.Repeat("я", 6), 3); preview != "яяя…" {
		t.Fatalf("preview=%q", preview)
	}
}

func TestCreateValidationAndStoreErrors(t *testing.T) {
	store := &fakeStore{}
	service, _ := newService(t, store)
	finish := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	start := finish.AddDate(0, 0, 1)
	cases := []CreateRequest{
		{Name: "x", Type: "ok", CurrencyCode: "RUB", RequestID: "request-1"},
		{Name: "ok", Type: "x", CurrencyCode: "RUB", RequestID: "request-1"},
		{Name: "ok", Type: "ok", CurrencyCode: "12", RequestID: "request-1"},
		{Name: "ok", Type: "ok", CurrencyCode: "RUB", RequestID: "short"},
		{Name: "ok", Type: "ok", CurrencyCode: "RUB", RequestID: "request-1", PlannedStartOn: &start, PlannedFinishOn: &finish},
		{Name: "ok", Type: "ok", CurrencyCode: "RUB", RequestID: "request-1", CustomerUserID: stringPointer("invalid")},
	}
	for _, request := range cases {
		if _, err := service.Create(context.Background(), Actor{UserID: actorID}, request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("request=%#v error=%v", request, err)
		}
	}
	request := validCreate()
	request.CustomerUserID = stringPointer(customerID)
	if _, err := service.Create(context.Background(), Actor{UserID: actorID}, request); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign customer error=%v", err)
	}
	request = validCreate()
	request.AdminUserIDs = []string{customerID}
	if _, err := service.Create(context.Background(), Actor{UserID: actorID}, request); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign administrator error=%v", err)
	}
	store.createErr = context.DeadlineExceeded
	if _, err := service.Create(context.Background(), Actor{UserID: actorID}, validCreate()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("store error=%v", err)
	}
}

func TestListUsesSignedKeysetCursor(t *testing.T) {
	created := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{listViews: []View{{ID: actorID, CreatedAt: created}, {ID: customerID, CreatedAt: created.Add(-time.Hour)}}}
	service, _ := newService(t, store)
	page, err := service.List(context.Background(), ListRequest{Actor: Actor{UserID: actorID}, PageSize: 1, Status: "active"})
	if err != nil || !page.HasMore || page.NextCursor == nil || len(page.Items) != 1 || store.listQuery.PageSize != 2 {
		t.Fatalf("page=%#v query=%#v error=%v", page, store.listQuery, err)
	}
	store.listViews = nil
	if _, err = service.List(context.Background(), ListRequest{Actor: Actor{UserID: actorID}, PageSize: 1, Cursor: *page.NextCursor}); err != nil || store.listQuery.BeforeID == nil || *store.listQuery.BeforeID != actorID {
		t.Fatalf("decoded query=%#v error=%v", store.listQuery, err)
	}
	if _, err = service.List(context.Background(), ListRequest{Actor: Actor{UserID: actorID}, Cursor: *page.NextCursor + "x"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("tampered cursor error=%v", err)
	}
	for _, cursor := range []string{"not-base64!", "YQ", service.encodeCursor(projectCursor{CreatedAt: created, ID: "bad"}), service.encodeCursor(projectCursor{ID: actorID})} {
		if _, err = service.List(context.Background(), ListRequest{Actor: Actor{UserID: actorID}, Cursor: cursor}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("cursor=%q error=%v", cursor, err)
		}
	}
	for _, request := range []ListRequest{{Actor: Actor{UserID: "bad"}}, {Actor: Actor{UserID: actorID}, PageSize: 51}, {Actor: Actor{UserID: actorID}, Status: "bad"}} {
		if _, err = service.List(context.Background(), request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("request=%#v error=%v", request, err)
		}
	}
}

func TestGetAndAssignCustomerEnforceIdentifiersAndRole(t *testing.T) {
	store := &fakeStore{getView: View{ID: customerID}, assignView: View{ID: customerID}}
	service, now := newService(t, store)
	if _, err := service.Get(context.Background(), Actor{UserID: actorID}, customerID); err != nil || store.getID != customerID {
		t.Fatalf("get id=%q error=%v", store.getID, err)
	}
	if _, err := service.Get(context.Background(), Actor{UserID: "bad"}, customerID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad actor error=%v", err)
	}
	if _, err := service.Get(context.Background(), Actor{UserID: actorID}, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad project error=%v", err)
	}
	if _, err := service.AssignCustomer(context.Background(), Actor{UserID: actorID}, customerID, customerID, "request-123", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ordinary assignment error=%v", err)
	}
	actor := Actor{UserID: actorID, SuperAdmin: true}
	if _, err := service.AssignCustomer(context.Background(), actor, customerID, customerID, "request-123", 2); err != nil {
		t.Fatal(err)
	}
	if store.assignCommand.ExpectedVersion != 2 || !store.assignCommand.Now.Equal(now) || store.assignCommand.CustomerUserID != customerID {
		t.Fatalf("assignment=%#v", store.assignCommand)
	}
	for _, values := range [][4]string{{"bad", customerID, "request-123", "1"}, {customerID, "bad", "request-123", "1"}, {customerID, customerID, "short", "1"}, {customerID, customerID, "request-123", "0"}} {
		version := int64(1)
		if values[3] == "0" {
			version = 0
		}
		if _, err := service.AssignCustomer(context.Background(), actor, values[0], values[1], values[2], version); err == nil {
			t.Fatalf("expected validation error for %#v", values)
		}
	}
}

func TestUpcomingFeedAndCreationNormalizeAndValidate(t *testing.T) {
	store := &fakeStore{upcomingItems: []UpcomingItem{{ID: customerID, ProjectID: actorID, Kind: UpcomingTask, Title: "Планы"}}, globalProjects: []GlobalCalendarProject{{ID: customerID, Name: "Полянка", Status: "active"}}, globalMore: true, globalTruncated: true, taskItem: UpcomingItem{ID: customerID}, meetingItem: UpcomingItem{ID: customerID}}
	service, now := newService(t, store)
	actor := Actor{UserID: actorID}

	feed, err := service.ListUpcoming(context.Background(), actor, customerID, 14)
	if err != nil || feed.Days != 14 || len(feed.Items) != 1 || !feed.RangeStart.Equal(now) || !feed.RangeEnd.Equal(now.Add(14*24*time.Hour)) {
		t.Fatalf("feed=%#v query=%#v error=%v", feed, store.upcomingQuery, err)
	}
	calendarStart, calendarEnd := now.Add(-24*time.Hour), now.Add(31*24*time.Hour)
	calendar, err := service.ListCalendar(context.Background(), actor, customerID, calendarStart, calendarEnd)
	if err != nil || len(calendar.Items) != 1 || !calendar.RangeStart.Equal(calendarStart) || !calendar.RangeEnd.Equal(calendarEnd) || !store.upcomingQuery.RangeStart.Equal(calendarStart) {
		t.Fatalf("calendar=%#v query=%#v error=%v", calendar, store.upcomingQuery, err)
	}
	global, err := service.ListGlobalCalendar(context.Background(), actor, calendarStart, calendarEnd, []string{customerID})
	if err != nil || len(global.Projects) != 1 || !global.HasMoreProjects || !global.TruncatedEvents || store.globalQuery.ProjectIDs[0] != customerID || !store.globalQuery.RangeStart.Equal(calendarStart) {
		t.Fatalf("global calendar=%#v query=%#v error=%v", global, store.globalQuery, err)
	}
	for _, bounds := range [][2]time.Time{{time.Time{}, calendarEnd}, {calendarStart, calendarStart}, {calendarStart, calendarStart.Add(371 * 24 * time.Hour)}} {
		if _, err = service.ListCalendar(context.Background(), actor, customerID, bounds[0], bounds[1]); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid calendar bounds=%#v error=%v", bounds, err)
		}
		if _, err = service.ListGlobalCalendar(context.Background(), actor, bounds[0], bounds[1], nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid global calendar bounds=%#v error=%v", bounds, err)
		}
	}
	for _, ids := range [][]string{{"bad"}, {customerID, customerID}} {
		if _, err = service.ListGlobalCalendar(context.Background(), actor, calendarStart, calendarEnd, ids); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid global calendar projects=%#v error=%v", ids, err)
		}
	}
	for _, invalid := range []struct {
		actor Actor
		id    string
		days  int
	}{{Actor{UserID: "bad"}, customerID, 7}, {actor, "bad", 7}, {actor, customerID, 0}, {actor, customerID, 91}} {
		if _, err = service.ListUpcoming(context.Background(), invalid.actor, invalid.id, invalid.days); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid feed=%#v error=%v", invalid, err)
		}
	}

	due := now.Add(48 * time.Hour)
	if _, err = service.CreateTask(context.Background(), actor, customerID, CreateTaskRequest{Title: "  Подготовить планы  ", Description: "  Первый этаж  ", DueAt: due, RequestID: "request-task"}); err != nil {
		t.Fatal(err)
	}
	if store.taskCommand.Title != "Подготовить планы" || store.taskCommand.Description != "Первый этаж" || !store.taskCommand.DueAt.Equal(due) || !store.taskCommand.Now.Equal(now) {
		t.Fatalf("task command=%#v", store.taskCommand)
	}
	starts, ends := now.Add(24*time.Hour), now.Add(25*time.Hour)
	if _, err = service.CreateMeeting(context.Background(), actor, customerID, CreateMeetingRequest{Title: "  Встреча на объекте ", Description: "  Замеры ", Location: "  Москва ", StartsAt: starts, EndsAt: ends, RequestID: "request-meeting"}); err != nil {
		t.Fatal(err)
	}
	if store.meetingCommand.Title != "Встреча на объекте" || store.meetingCommand.Location != "Москва" || !store.meetingCommand.StartsAt.Equal(starts) || !store.meetingCommand.EndsAt.Equal(ends) {
		t.Fatalf("meeting command=%#v", store.meetingCommand)
	}

	for _, request := range []CreateTaskRequest{{Title: "x", DueAt: due, RequestID: "request-task"}, {Title: "Задача", DueAt: now, RequestID: "request-task"}, {Title: "Задача", DueAt: due, RequestID: "short"}} {
		if _, err = service.CreateTask(context.Background(), actor, customerID, request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid task=%#v error=%v", request, err)
		}
	}
	for _, request := range []CreateMeetingRequest{{Title: "x", StartsAt: starts, EndsAt: ends, RequestID: "request-meeting"}, {Title: "Встреча", StartsAt: now, EndsAt: ends, RequestID: "request-meeting"}, {Title: "Встреча", StartsAt: starts, EndsAt: starts, RequestID: "request-meeting"}} {
		if _, err = service.CreateMeeting(context.Background(), actor, customerID, request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid meeting=%#v error=%v", request, err)
		}
	}
	store.upcomingErr = context.DeadlineExceeded
	if _, err = service.ListGlobalCalendar(context.Background(), actor, calendarStart, calendarEnd, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("global calendar store error=%v", err)
	}
	if _, err = service.ListCalendar(context.Background(), actor, customerID, calendarStart, calendarEnd); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("calendar store error=%v", err)
	}
	if _, err = service.ListUpcoming(context.Background(), actor, customerID, 7); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("list store error=%v", err)
	}
	store.taskErr = context.DeadlineExceeded
	if _, err = service.CreateTask(context.Background(), actor, customerID, CreateTaskRequest{Title: "Задача", DueAt: due, RequestID: "request-task"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("task store error=%v", err)
	}
	store.meetingErr = context.DeadlineExceeded
	if _, err = service.CreateMeeting(context.Background(), actor, customerID, CreateMeetingRequest{Title: "Встреча", StartsAt: starts, EndsAt: ends, RequestID: "request-meeting"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("meeting store error=%v", err)
	}
}

func TestUpdateValidatesFieldsAndArchiveRequiresSuperAdmin(t *testing.T) {
	store := &fakeStore{updateView: View{ID: customerID}}
	service, now := newService(t, store)
	name := "  Обновлённый проект  "
	status := "active"
	empty := "   "
	finish := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	request := UpdateRequest{Name: &name, Status: &status, Address: Optional[*string]{Set: true, Value: &empty}, PlannedFinishOn: Optional[*time.Time]{Set: true, Value: &finish}}
	if _, err := service.Update(context.Background(), Actor{UserID: actorID}, customerID, "request-update", 2, request); err != nil {
		t.Fatal(err)
	}
	if store.updateCommand.Name == nil || *store.updateCommand.Name != "Обновлённый проект" || store.updateCommand.Address.Value != nil || store.updateCommand.ExpectedVersion != 2 || !store.updateCommand.Now.Equal(now) {
		t.Fatalf("update command=%#v", store.updateCommand)
	}
	for _, invalid := range []UpdateRequest{{}, {Name: stringPointer("x")}, {Status: stringPointer("archived")}} {
		if _, err := service.Update(context.Background(), Actor{UserID: actorID}, customerID, "request-update", 1, invalid); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid update=%#v error=%v", invalid, err)
		}
	}
	start := finish.AddDate(0, 0, 1)
	if _, err := service.Update(context.Background(), Actor{UserID: actorID}, customerID, "request-update", 1, UpdateRequest{PlannedStartOn: Optional[*time.Time]{Set: true, Value: &start}, PlannedFinishOn: Optional[*time.Time]{Set: true, Value: &finish}}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid date range error=%v", err)
	}
	if err := service.Archive(context.Background(), Actor{UserID: actorID}, customerID, "request-archive", 2); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ordinary archive error=%v", err)
	}
	if err := service.Archive(context.Background(), Actor{UserID: actorID, SuperAdmin: true}, customerID, "request-archive", 2); err != nil || store.archiveCommand.ExpectedVersion != 2 || !store.archiveCommand.Now.Equal(now) {
		t.Fatalf("archive=%#v error=%v", store.archiveCommand, err)
	}
}

func TestUpdateNormalizesEveryEditableFieldAndRejectsInvalidMetadata(t *testing.T) {
	store := &fakeStore{updateView: View{ID: customerID}}
	service, _ := newService(t, store)
	validActor := Actor{UserID: actorID}
	validRequest := UpdateRequest{Type: stringPointer(" architecture ")}
	for _, test := range []struct {
		actor     Actor
		projectID string
		requestID string
		version   int64
	}{
		{Actor{UserID: "bad"}, customerID, "request-update", 1},
		{validActor, "bad", "request-update", 1},
		{validActor, customerID, "short", 1},
		{validActor, customerID, "request-update", 0},
	} {
		if _, err := service.Update(context.Background(), test.actor, test.projectID, test.requestID, test.version, validRequest); err == nil {
			t.Fatalf("expected invalid metadata for %#v", test)
		}
	}
	longName, shortType, longType := strings.Repeat("н", 201), "x", strings.Repeat("т", 101)
	longAddress, longDescription := strings.Repeat("а", 501), strings.Repeat("о", 5001)
	for _, request := range []UpdateRequest{
		{Name: &longName}, {Type: &shortType}, {Type: &longType},
		{Address: Optional[*string]{Set: true, Value: &longAddress}},
		{Description: Optional[*string]{Set: true, Value: &longDescription}},
	} {
		if _, err := service.Update(context.Background(), validActor, customerID, "request-update", 1, request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("request=%#v error=%v", request, err)
		}
	}
	address, description := "  Москва  ", "  Описание  "
	autoApprove := false
	request := UpdateRequest{Type: stringPointer(" architecture "), Address: Optional[*string]{Set: true, Value: &address}, Description: Optional[*string]{Set: true, Value: &description}, AutoApproveExpenses: &autoApprove}
	if _, err := service.Update(context.Background(), validActor, customerID, "request-update", 1, request); err != nil {
		t.Fatal(err)
	}
	if store.updateCommand.Type == nil || *store.updateCommand.Type != "architecture" || store.updateCommand.Address.Value == nil || *store.updateCommand.Address.Value != "Москва" || store.updateCommand.Description.Value == nil || *store.updateCommand.Description.Value != "Описание" {
		t.Fatalf("normalized update=%#v", store.updateCommand)
	}
	emptyDescription := "  "
	if _, err := service.Update(context.Background(), validActor, customerID, "request-update", 1, UpdateRequest{Description: Optional[*string]{Set: true, Value: &emptyDescription}}); err != nil || store.updateCommand.Description.Value != nil {
		t.Fatalf("empty description command=%#v error=%v", store.updateCommand, err)
	}
	store.updateErr = context.DeadlineExceeded
	if _, err := service.Update(context.Background(), validActor, customerID, "request-update", 1, validRequest); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("store update error=%v", err)
	}
}

func TestProjectMembersValidateIdentifiersNormalizeSearchAndForwardCommands(t *testing.T) {
	store := &fakeStore{
		members:       []Member{{UserID: customerID}},
		membersManage: true,
		candidates:    []MemberCandidate{{UserID: customerID}},
		member:        Member{UserID: customerID},
	}
	service, now := newService(t, store)
	actor := Actor{UserID: actorID}

	list, err := service.ListMembers(context.Background(), actor, customerID)
	if err != nil || !list.CanManage || len(list.Items) != 1 {
		t.Fatalf("member list=%#v error=%v", list, err)
	}
	candidates, err := service.SearchMemberCandidates(context.Background(), actor, customerID, "  DESIGNER.ONE  ")
	if err != nil || len(candidates) != 1 || store.candidateQuery.Query != "designer.one" || store.candidateQuery.Limit != 10 {
		t.Fatalf("candidates=%#v query=%#v error=%v", candidates, store.candidateQuery, err)
	}
	member, err := service.AddMember(context.Background(), actor, customerID, customerID, "request-member-add")
	if err != nil || member.UserID != customerID || !store.memberCommand.Now.Equal(now) || store.memberCommand.UserID != customerID {
		t.Fatalf("member=%#v command=%#v error=%v", member, store.memberCommand, err)
	}
	if err = service.RemoveMember(context.Background(), actor, customerID, customerID, "request-member-remove"); err != nil || store.memberCommand.RequestID != "request-member-remove" {
		t.Fatalf("remove command=%#v error=%v", store.memberCommand, err)
	}

	for _, test := range []struct {
		actor     Actor
		projectID string
		userID    string
		requestID string
	}{
		{Actor{UserID: "bad"}, customerID, customerID, "request-member"},
		{actor, "bad", customerID, "request-member"},
		{actor, customerID, "bad", "request-member"},
		{actor, customerID, customerID, "short"},
	} {
		if _, err = service.AddMember(context.Background(), test.actor, test.projectID, test.userID, test.requestID); err == nil {
			t.Fatalf("expected add validation error for %#v", test)
		}
		if err = service.RemoveMember(context.Background(), test.actor, test.projectID, test.userID, test.requestID); err == nil {
			t.Fatalf("expected remove validation error for %#v", test)
		}
	}
	for _, query := range []string{"x", strings.Repeat("x", 101)} {
		if _, err = service.SearchMemberCandidates(context.Background(), actor, customerID, query); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("query=%q error=%v", query, err)
		}
	}
	if _, err = service.ListMembers(context.Background(), actor, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid project list error=%v", err)
	}
	if _, err = service.ListMembers(context.Background(), Actor{UserID: "bad"}, customerID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid actor list error=%v", err)
	}
	if _, err = service.SearchMemberCandidates(context.Background(), actor, "bad", "sveta"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid search project error=%v", err)
	}
	store.membersErr = context.DeadlineExceeded
	if _, err = service.SearchMemberCandidates(context.Background(), actor, customerID, "sveta"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("store error=%v", err)
	}
}

func TestArchiveValidatesIdentifiersAndReturnsStoreError(t *testing.T) {
	store := &fakeStore{}
	service, _ := newService(t, store)
	actor := Actor{UserID: actorID, SuperAdmin: true}
	for _, test := range []struct {
		actor     Actor
		projectID string
		requestID string
		version   int64
	}{
		{Actor{UserID: "bad", SuperAdmin: true}, customerID, "request-archive", 1},
		{actor, "bad", "request-archive", 1},
		{actor, customerID, "short", 1},
		{actor, customerID, "request-archive", 0},
	} {
		if err := service.Archive(context.Background(), test.actor, test.projectID, test.requestID, test.version); err == nil {
			t.Fatalf("expected archive validation error for %#v", test)
		}
	}
	store.archiveErr = context.DeadlineExceeded
	if err := service.Archive(context.Background(), actor, customerID, "request-archive", 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("archive store error=%v", err)
	}
}

func TestNewServiceRejectsInvalidDependencies(t *testing.T) {
	if _, err := NewService(nil, []byte(strings.Repeat("k", 32)), time.Now); err == nil {
		t.Fatal("nil store must fail")
	}
	if _, err := NewService(&fakeStore{}, []byte("short"), time.Now); err == nil {
		t.Fatal("short cursor key must fail")
	}
	if service, err := NewService(&fakeStore{}, []byte(strings.Repeat("k", 32)), nil); err != nil || service.now == nil {
		t.Fatalf("default clock service=%#v error=%v", service, err)
	}
}

func stringPointer(value string) *string { return &value }
