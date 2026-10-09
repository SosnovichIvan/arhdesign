package project

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput       = errors.New("invalid project input")
	ErrForbidden          = errors.New("project action forbidden")
	ErrNotFound           = errors.New("project not found")
	ErrConflict           = errors.New("project conflict")
	ErrAccountUnavailable = errors.New("project account unavailable")
)

var allowedStatuses = map[string]struct{}{"draft": {}, "active": {}, "paused": {}, "completed": {}, "pending_deletion": {}, "archived": {}}
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type Actor struct {
	UserID     string
	SuperAdmin bool
	Login      string
	FirstName  string
	LastName   *string
}

type CreateRequest struct {
	Name, Type, Address, CurrencyCode, Description, RequestID string
	CustomerUserID                                            *string
	AdminUserIDs                                              []string
	PlannedStartOn, PlannedFinishOn                           *time.Time
	AutoApproveExpenses                                       bool
}

type Optional[T any] struct {
	Set   bool
	Value T
}

type UpdateRequest struct {
	Name, Type, Status   *string
	Address, Description Optional[*string]
	PlannedStartOn       Optional[*time.Time]
	PlannedFinishOn      Optional[*time.Time]
	AutoApproveExpenses  *bool
}

type View struct {
	ID, Name, Type, Status, CurrencyCode, CreatedByUserID string
	Address, Description                                  *string
	CustomerUserID                                        *string
	AdminUserIDs                                          []string
	PlannedStartOn, PlannedFinishOn                       *time.Time
	AutoApproveExpenses                                   bool
	CreatedAt                                             time.Time
	Version                                               int64
}

type ListRequest struct {
	Actor    Actor
	PageSize int
	Cursor   string
	Status   string
}

type Page struct {
	Items      []View
	NextCursor *string
	HasMore    bool
}

type CreateCommand struct {
	Actor
	CreateRequest
	CustomerUserID *string
	AdminUserIDs   []string
	Now            time.Time
	Notification   *CreatedNotification
}

type EventNotification struct {
	MessageType       string
	Plaintext         string
	PayloadCiphertext []byte
	PayloadKeyVersion int
}

type CreatedNotification = EventNotification

type PayloadEncryptor interface {
	Encrypt([]byte, string) ([]byte, int, error)
}

type ListQuery struct {
	Actor
	PageSize        int
	Status          string
	BeforeCreatedAt *time.Time
	BeforeID        *string
}

type AssignCustomerCommand struct {
	Actor
	ProjectID, CustomerUserID, RequestID string
	ExpectedVersion                      int64
	Now                                  time.Time
}

type UpdateCommand struct {
	Actor
	ProjectID, RequestID string
	ExpectedVersion      int64
	UpdateRequest
	Now time.Time
}

type ArchiveCommand struct {
	Actor
	ProjectID, RequestID string
	ExpectedVersion      int64
	Now                  time.Time
}

type Store interface {
	CreateProject(context.Context, CreateCommand) (View, error)
	ListProjects(context.Context, ListQuery) ([]View, error)
	GetProject(context.Context, Actor, string) (View, error)
	UpdateProject(context.Context, UpdateCommand) (View, error)
	ArchiveProject(context.Context, ArchiveCommand) error
	AssignProjectCustomer(context.Context, AssignCustomerCommand) (View, error)
	ListProjectUpcoming(context.Context, UpcomingQuery) ([]UpcomingItem, error)
	ListGlobalCalendar(context.Context, GlobalCalendarQuery) ([]GlobalCalendarProject, bool, bool, error)
	ListProjectTasks(context.Context, TaskQuery) ([]Task, bool, error)
	CreateProjectTask(context.Context, CreateTaskCommand) (UpcomingItem, error)
	UpdateProjectTask(context.Context, UpdateTaskCommand) (Task, error)
	CreateProjectMeeting(context.Context, CreateMeetingCommand) (UpcomingItem, error)
	ListProjectMembers(context.Context, Actor, string) ([]Member, bool, error)
	SearchProjectMemberCandidates(context.Context, MemberCandidateQuery) ([]MemberCandidate, error)
	AddProjectMember(context.Context, MemberCommand) (Member, error)
	RemoveProjectMember(context.Context, MemberCommand) error
	GetProjectFinanceSummary(context.Context, Actor, string) (FinanceSummary, error)
	ListProjectExpenses(context.Context, Actor, string) ([]Expense, bool, error)
	CreateProjectExpense(context.Context, CreateExpenseCommand) (Expense, error)
	ListProjectMaterials(context.Context, Actor, string) ([]Material, bool, bool, error)
	CreateProjectMaterial(context.Context, CreateMaterialCommand) (Material, error)
	UpdateProjectMaterial(context.Context, UpdateMaterialCommand) (Material, error)
	DeleteProjectMaterial(context.Context, DeleteMaterialCommand) error
	ListProjectChat(context.Context, ChatQuery) (ProjectChatPage, error)
	CreateProjectChatMessage(context.Context, CreateChatMessageCommand) (ChatMessage, error)
	OpenProjectContextChat(context.Context, OpenContextChatCommand) (ChatContext, error)
	ListProjectContextChat(context.Context, ChatQuery) (ProjectChatPage, error)
	CreateProjectContextChatMessage(context.Context, CreateChatMessageCommand) (ChatMessage, error)
}

type Service struct {
	store     Store
	cursorKey []byte
	now       func() time.Time
	cipher    PayloadEncryptor
}

func (service *Service) ConfigureNotifications(cipher PayloadEncryptor) error {
	if cipher == nil {
		return errors.New("project notification cipher is required")
	}
	service.cipher = cipher
	return nil
}

func NewService(store Store, cursorKey []byte, now func() time.Time) (*Service, error) {
	if store == nil || len(cursorKey) < 32 {
		return nil, errors.New("project service requires store and cursor key")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, cursorKey: append([]byte(nil), cursorKey...), now: now}, nil
}

func (service *Service) eventNotification(messageType, text string) (*EventNotification, error) {
	if service.cipher == nil {
		return nil, nil
	}
	payload, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})
	if err != nil {
		return nil, fmt.Errorf("encode %s notification: %w", messageType, err)
	}
	ciphertext, keyVersion, err := service.cipher.Encrypt(payload, messageType)
	if err != nil {
		return nil, fmt.Errorf("encrypt %s notification: %w", messageType, err)
	}
	return &EventNotification{MessageType: messageType, Plaintext: text, PayloadCiphertext: ciphertext, PayloadKeyVersion: keyVersion}, nil
}

func actorDisplayName(actor Actor) string {
	lastName := ""
	if actor.LastName != nil {
		lastName = *actor.LastName
	}
	name := strings.TrimSpace(actor.FirstName + " " + lastName)
	if name == "" {
		name = actor.Login
	}
	return name
}

func notificationPreview(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum]) + "…"
}

func (service *Service) Create(ctx context.Context, actor Actor, request CreateRequest) (View, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Type = strings.TrimSpace(request.Type)
	request.Address = strings.TrimSpace(request.Address)
	request.Description = strings.TrimSpace(request.Description)
	request.CurrencyCode = strings.ToUpper(strings.TrimSpace(request.CurrencyCode))
	if err := validateCreate(actor, request); err != nil {
		return View{}, err
	}
	customerID := request.CustomerUserID
	admins := uniqueIDs(request.AdminUserIDs)
	if !actor.SuperAdmin {
		customerID = &actor.UserID
		admins = []string{actor.UserID}
	}
	command := CreateCommand{Actor: actor, CreateRequest: request, CustomerUserID: customerID, AdminUserIDs: admins, Now: service.now().UTC()}
	if !actor.SuperAdmin && service.cipher != nil {
		const messageType = "project.created"
		lastName := ""
		if actor.LastName != nil {
			lastName = *actor.LastName
		}
		creator := strings.TrimSpace(actor.FirstName + " " + lastName)
		if creator == "" {
			creator = actor.Login
		}
		message, err := json.Marshal(struct {
			Text string `json:"text"`
		}{Text: "Создан новый проект\nПользователь: " + creator + " (@" + actor.Login + ")\nПроект: " + request.Name})
		if err != nil {
			return View{}, fmt.Errorf("encode project notification: %w", err)
		}
		ciphertext, keyVersion, err := service.cipher.Encrypt(message, messageType)
		if err != nil {
			return View{}, fmt.Errorf("encrypt project notification: %w", err)
		}
		command.Notification = &CreatedNotification{MessageType: messageType, PayloadCiphertext: ciphertext, PayloadKeyVersion: keyVersion}
	}
	return service.store.CreateProject(ctx, command)
}

func (service *Service) List(ctx context.Context, request ListRequest) (Page, error) {
	if id, err := uuid.Parse(request.Actor.UserID); err != nil || id == uuid.Nil {
		return Page{}, ErrInvalidInput
	}
	if request.PageSize == 0 {
		request.PageSize = 20
	}
	if request.PageSize < 1 || request.PageSize > 50 {
		return Page{}, ErrInvalidInput
	}
	if request.Status != "" {
		if _, ok := allowedStatuses[request.Status]; !ok {
			return Page{}, ErrInvalidInput
		}
	}
	query := ListQuery{Actor: request.Actor, PageSize: request.PageSize + 1, Status: request.Status}
	if request.Cursor != "" {
		cursor, err := service.decodeCursor(request.Cursor)
		if err != nil {
			return Page{}, ErrInvalidInput
		}
		query.BeforeCreatedAt, query.BeforeID = &cursor.CreatedAt, &cursor.ID
	}
	items, err := service.store.ListProjects(ctx, query)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(page.Items) > request.PageSize {
		page.HasMore = true
		page.Items = page.Items[:request.PageSize]
		last := page.Items[len(page.Items)-1]
		next := service.encodeCursor(projectCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		page.NextCursor = &next
	}
	return page, nil
}

func (service *Service) Get(ctx context.Context, actor Actor, projectID string) (View, error) {
	if id, err := uuid.Parse(actor.UserID); err != nil || id == uuid.Nil {
		return View{}, ErrInvalidInput
	}
	if id, err := uuid.Parse(projectID); err != nil || id == uuid.Nil {
		return View{}, ErrNotFound
	}
	return service.store.GetProject(ctx, actor, projectID)
}

func (service *Service) Update(ctx context.Context, actor Actor, projectID, requestID string, expectedVersion int64, request UpdateRequest) (View, error) {
	if id, err := uuid.Parse(actor.UserID); err != nil || id == uuid.Nil || len(requestID) < 8 || expectedVersion < 1 {
		return View{}, ErrInvalidInput
	}
	if id, err := uuid.Parse(projectID); err != nil || id == uuid.Nil {
		return View{}, ErrNotFound
	}
	if request.Name == nil && request.Type == nil && request.Status == nil && !request.Address.Set && !request.Description.Set && !request.PlannedStartOn.Set && !request.PlannedFinishOn.Set && request.AutoApproveExpenses == nil {
		return View{}, ErrInvalidInput
	}
	if request.Name != nil {
		value := strings.TrimSpace(*request.Name)
		if len([]rune(value)) < 2 || len([]rune(value)) > 200 {
			return View{}, ErrInvalidInput
		}
		request.Name = &value
	}
	if request.Type != nil {
		value := strings.TrimSpace(*request.Type)
		if len([]rune(value)) < 2 || len([]rune(value)) > 100 {
			return View{}, ErrInvalidInput
		}
		request.Type = &value
	}
	if request.Status != nil {
		value := strings.TrimSpace(*request.Status)
		if _, allowed := map[string]struct{}{"draft": {}, "active": {}, "paused": {}, "completed": {}}[value]; !allowed {
			return View{}, ErrInvalidInput
		}
		request.Status = &value
	}
	if request.Address.Set && request.Address.Value != nil {
		value := strings.TrimSpace(*request.Address.Value)
		if len([]rune(value)) > 500 {
			return View{}, ErrInvalidInput
		}
		if value == "" {
			request.Address.Value = nil
		} else {
			request.Address.Value = &value
		}
	}
	if request.Description.Set && request.Description.Value != nil {
		value := strings.TrimSpace(*request.Description.Value)
		if len([]rune(value)) > 5000 {
			return View{}, ErrInvalidInput
		}
		if value == "" {
			request.Description.Value = nil
		} else {
			request.Description.Value = &value
		}
	}
	if request.PlannedStartOn.Set && request.PlannedFinishOn.Set && request.PlannedStartOn.Value != nil && request.PlannedFinishOn.Value != nil && request.PlannedFinishOn.Value.Before(*request.PlannedStartOn.Value) {
		return View{}, ErrInvalidInput
	}
	return service.store.UpdateProject(ctx, UpdateCommand{Actor: actor, ProjectID: projectID, RequestID: requestID, ExpectedVersion: expectedVersion, UpdateRequest: request, Now: service.now().UTC()})
}

func (service *Service) Archive(ctx context.Context, actor Actor, projectID, requestID string, expectedVersion int64) error {
	if !actor.SuperAdmin {
		return ErrForbidden
	}
	if id, err := uuid.Parse(actor.UserID); err != nil || id == uuid.Nil || len(requestID) < 8 || expectedVersion < 1 {
		return ErrInvalidInput
	}
	if id, err := uuid.Parse(projectID); err != nil || id == uuid.Nil {
		return ErrNotFound
	}
	return service.store.ArchiveProject(ctx, ArchiveCommand{Actor: actor, ProjectID: projectID, RequestID: requestID, ExpectedVersion: expectedVersion, Now: service.now().UTC()})
}

func (service *Service) AssignCustomer(ctx context.Context, actor Actor, projectID, customerUserID, requestID string, expectedVersion int64) (View, error) {
	if !actor.SuperAdmin {
		return View{}, ErrForbidden
	}
	if id, err := uuid.Parse(actor.UserID); err != nil || id == uuid.Nil {
		return View{}, ErrInvalidInput
	}
	if id, err := uuid.Parse(projectID); err != nil || id == uuid.Nil {
		return View{}, ErrNotFound
	}
	if id, err := uuid.Parse(customerUserID); err != nil || id == uuid.Nil || expectedVersion < 1 || len(requestID) < 8 {
		return View{}, ErrInvalidInput
	}
	return service.store.AssignProjectCustomer(ctx, AssignCustomerCommand{Actor: actor, ProjectID: projectID, CustomerUserID: customerUserID, RequestID: requestID, ExpectedVersion: expectedVersion, Now: service.now().UTC()})
}

func validateCreate(actor Actor, request CreateRequest) error {
	if id, err := uuid.Parse(actor.UserID); err != nil || id == uuid.Nil || len(request.RequestID) < 8 || len([]rune(request.Name)) < 2 || len([]rune(request.Name)) > 200 || len([]rune(request.Type)) < 2 || len([]rune(request.Type)) > 100 || len([]rune(request.Address)) > 500 || len([]rune(request.Description)) > 5000 || !currencyPattern.MatchString(request.CurrencyCode) {
		return ErrInvalidInput
	}
	if request.PlannedStartOn != nil && request.PlannedFinishOn != nil && request.PlannedFinishOn.Before(*request.PlannedStartOn) {
		return ErrInvalidInput
	}
	if request.CustomerUserID != nil {
		if id, err := uuid.Parse(*request.CustomerUserID); err != nil || id == uuid.Nil {
			return ErrInvalidInput
		}
		if !actor.SuperAdmin && *request.CustomerUserID != actor.UserID {
			return ErrForbidden
		}
	}
	for _, id := range request.AdminUserIDs {
		if parsed, err := uuid.Parse(id); err != nil || parsed == uuid.Nil {
			return ErrInvalidInput
		}
		if !actor.SuperAdmin && id != actor.UserID {
			return ErrForbidden
		}
	}
	return nil
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

type projectCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

func (service *Service) encodeCursor(cursor projectCursor) string {
	payload, _ := json.Marshal(cursor)
	signature := hmac.New(sha256.New, service.cursorKey)
	_, _ = signature.Write(payload)
	return base64.RawURLEncoding.EncodeToString(append(payload, signature.Sum(nil)...))
}

func (service *Service) decodeCursor(encoded string) (projectCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) <= sha256.Size {
		return projectCursor{}, ErrInvalidInput
	}
	payload, receivedSignature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	expectedSignature := hmac.New(sha256.New, service.cursorKey)
	_, _ = expectedSignature.Write(payload)
	if !hmac.Equal(receivedSignature, expectedSignature.Sum(nil)) {
		return projectCursor{}, ErrInvalidInput
	}
	var cursor projectCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.CreatedAt.IsZero() {
		return projectCursor{}, ErrInvalidInput
	}
	if _, err = uuid.Parse(cursor.ID); err != nil {
		return projectCursor{}, ErrInvalidInput
	}
	return cursor, nil
}
