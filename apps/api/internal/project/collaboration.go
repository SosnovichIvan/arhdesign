package project

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

const MaxProjectDocumentBytes int64 = 10 * 1024 * 1024

type ChatSummary struct {
	ID, ProjectID, Kind, Name            string
	ContextType, ContextID, ContextTitle *string
	MemberCount                          int
	UnreadCount                          int
	CreatedAt                            time.Time
	CanManage                            bool
	Version                              int64
}

type ChatList struct {
	Items     []ChatSummary
	CanCreate bool
}

type ChatMember struct {
	UserID, Login, FirstName string
	LastName                 *string
	JoinedAt                 time.Time
	Removable                bool
}

type ChatMemberList struct {
	Items     []ChatMember
	CanManage bool
}

type CreateChatRequest struct {
	Name          string
	MemberUserIDs []string
	RequestID     string
}
type CreateChatCommand struct {
	Actor
	ProjectID, Name, RequestID string
	MemberUserIDs              []string
	Now                        time.Time
	Notification               *EventNotification
}
type UpdateChatCommand struct {
	Actor
	ProjectID, ChatID, Name, RequestID string
	Version                            int64
	Now                                time.Time
	Notification                       *EventNotification
}
type DeleteChatCommand struct {
	Actor
	ProjectID, ChatID, RequestID string
	Version                      int64
	Now                          time.Time
	Notification                 *EventNotification
}
type ChatMemberCommand struct {
	Actor
	ProjectID, ChatID, UserID, RequestID string
	Now                                  time.Time
	Notification                         *EventNotification
}

type MarkChatReadCommand struct {
	Actor
	ProjectID, ChatID, MessageID string
	Now                          time.Time
}

type AccountNotification struct {
	ID, ProjectID, EventType, Title, Body, Href string
	CreatedAt                                   time.Time
	ReadAt                                      *time.Time
}

type AccountNotificationList struct {
	Items       []AccountNotification
	UnreadCount int
}

type ProjectDocument struct {
	ID, ProjectID, Name, MediaType, UploadedByUserID string
	SizeBytes                                        int64
	CreatedAt                                        time.Time
	CanDelete                                        bool
	Version                                          int64
}
type ProjectDocumentList struct {
	Items     []ProjectDocument
	CanUpload bool
}
type ProjectDocumentContent struct {
	ProjectDocument
	Content []byte
}
type UploadProjectDocumentCommand struct {
	Actor
	ProjectID, Name, MediaType, RequestID string
	Content                               []byte
	Now                                   time.Time
	Notification                          *EventNotification
}
type DeleteProjectDocumentCommand struct {
	Actor
	ProjectID, DocumentID, RequestID string
	Now                              time.Time
	Notification                     *EventNotification
}

type CollaborationStore interface {
	ListProjectChats(context.Context, Actor, string) (ChatList, error)
	CreateProjectChat(context.Context, CreateChatCommand) (ChatSummary, error)
	UpdateProjectChat(context.Context, UpdateChatCommand) (ChatSummary, error)
	ListProjectChatMembers(context.Context, Actor, string, string) (ChatMemberList, error)
	AddProjectChatMember(context.Context, ChatMemberCommand) (ChatMember, error)
	RemoveProjectChatMember(context.Context, ChatMemberCommand) error
	ListProjectDocuments(context.Context, Actor, string) (ProjectDocumentList, error)
	UploadProjectDocument(context.Context, UploadProjectDocumentCommand) (ProjectDocument, error)
	DownloadProjectDocument(context.Context, Actor, string, string) (ProjectDocumentContent, error)
	DeleteProjectDocument(context.Context, DeleteProjectDocumentCommand) error
}

type ChatLifecycleStore interface {
	DeleteProjectChat(context.Context, DeleteChatCommand) error
	MarkProjectChatRead(context.Context, MarkChatReadCommand) error
}

type AccountNotificationStore interface {
	ListAccountNotifications(context.Context, Actor, int) (AccountNotificationList, error)
	MarkAccountNotificationRead(context.Context, Actor, string, time.Time) error
	MarkAllAccountNotificationsRead(context.Context, Actor, time.Time) error
}

func (service *Service) collaborationStore() (CollaborationStore, error) {
	store, ok := service.store.(CollaborationStore)
	if !ok {
		return nil, errors.New("collaboration store is not configured")
	}
	return store, nil
}

func (service *Service) ListChats(ctx context.Context, actor Actor, projectID string) (ChatList, error) {
	if !validActorAndProject(actor, projectID) {
		return ChatList{}, ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ChatList{}, err
	}
	return store.ListProjectChats(ctx, actor, projectID)
}

func (service *Service) CreateChat(ctx context.Context, actor Actor, projectID string, request CreateChatRequest) (ChatSummary, error) {
	request.Name = strings.TrimSpace(request.Name)
	if !validActorAndProject(actor, projectID) || len([]rune(request.Name)) < 2 || len([]rune(request.Name)) > 200 || len(request.MemberUserIDs) > 100 || len(request.RequestID) < 8 {
		return ChatSummary{}, ErrInvalidInput
	}
	seen := map[string]struct{}{}
	for _, userID := range request.MemberUserIDs {
		if !validUUID(userID) {
			return ChatSummary{}, ErrInvalidInput
		}
		seen[userID] = struct{}{}
	}
	members := make([]string, 0, len(seen))
	for userID := range seen {
		members = append(members, userID)
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ChatSummary{}, err
	}
	notification, err := service.eventNotification("project.chat.created", "Создан чат проекта\nЧат: "+request.Name)
	if err != nil {
		return ChatSummary{}, err
	}
	return store.CreateProjectChat(ctx, CreateChatCommand{Actor: actor, ProjectID: projectID, Name: request.Name, MemberUserIDs: members, RequestID: request.RequestID, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) UpdateChat(ctx context.Context, actor Actor, projectID, chatID, name, requestID string, version int64) (ChatSummary, error) {
	name = strings.TrimSpace(name)
	if !validActorAndProject(actor, projectID) || !validUUID(chatID) || len([]rune(name)) < 2 || len([]rune(name)) > 200 || len(requestID) < 8 || version < 1 {
		return ChatSummary{}, ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ChatSummary{}, err
	}
	notification, err := service.eventNotification("project.chat.renamed", "Чат проекта переименован\nНовое название: "+name)
	if err != nil {
		return ChatSummary{}, err
	}
	return store.UpdateProjectChat(ctx, UpdateChatCommand{Actor: actor, ProjectID: projectID, ChatID: chatID, Name: name, RequestID: requestID, Version: version, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) DeleteChat(ctx context.Context, actor Actor, projectID, chatID, requestID string, version int64) error {
	if !validActorAndProject(actor, projectID) || !validUUID(chatID) || len(requestID) < 8 || version < 1 {
		return ErrInvalidInput
	}
	store, ok := service.store.(ChatLifecycleStore)
	if !ok {
		return errors.New("chat lifecycle store is not configured")
	}
	notification, err := service.eventNotification("project.chat.deleted", "Чат проекта удалён")
	if err != nil {
		return err
	}
	return store.DeleteProjectChat(ctx, DeleteChatCommand{Actor: actor, ProjectID: projectID, ChatID: chatID, RequestID: requestID, Version: version, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) ListChatMembers(ctx context.Context, actor Actor, projectID, chatID string) (ChatMemberList, error) {
	if !validActorAndProject(actor, projectID) || !validUUID(chatID) {
		return ChatMemberList{}, ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ChatMemberList{}, err
	}
	return store.ListProjectChatMembers(ctx, actor, projectID, chatID)
}

func (service *Service) AddChatMember(ctx context.Context, actor Actor, projectID, chatID, userID, requestID string) (ChatMember, error) {
	if !validActorAndProject(actor, projectID) || !validUUID(chatID) || !validUUID(userID) || len(requestID) < 8 {
		return ChatMember{}, ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ChatMember{}, err
	}
	notification, err := service.eventNotification("project.chat.member_added", "В чат проекта добавлен участник")
	if err != nil {
		return ChatMember{}, err
	}
	return store.AddProjectChatMember(ctx, ChatMemberCommand{Actor: actor, ProjectID: projectID, ChatID: chatID, UserID: userID, RequestID: requestID, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) RemoveChatMember(ctx context.Context, actor Actor, projectID, chatID, userID, requestID string) error {
	if !validActorAndProject(actor, projectID) || !validUUID(chatID) || !validUUID(userID) || len(requestID) < 8 {
		return ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return err
	}
	notification, err := service.eventNotification("project.chat.member_removed", "Из чата проекта удалён участник")
	if err != nil {
		return err
	}
	return store.RemoveProjectChatMember(ctx, ChatMemberCommand{Actor: actor, ProjectID: projectID, ChatID: chatID, UserID: userID, RequestID: requestID, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) MarkChatRead(ctx context.Context, actor Actor, projectID, chatID, messageID string) error {
	if !validActorAndProject(actor, projectID) || !validUUID(chatID) || !validUUID(messageID) {
		return ErrInvalidInput
	}
	store, ok := service.store.(ChatLifecycleStore)
	if !ok {
		return errors.New("chat lifecycle store is not configured")
	}
	return store.MarkProjectChatRead(ctx, MarkChatReadCommand{Actor: actor, ProjectID: projectID, ChatID: chatID, MessageID: messageID, Now: service.now().UTC()})
}

func (service *Service) ListNotifications(ctx context.Context, actor Actor, pageSize int) (AccountNotificationList, error) {
	if actor.UserID == "" || pageSize < 1 || pageSize > 50 {
		return AccountNotificationList{}, ErrInvalidInput
	}
	store, ok := service.store.(AccountNotificationStore)
	if !ok {
		return AccountNotificationList{}, errors.New("account notification store is not configured")
	}
	return store.ListAccountNotifications(ctx, actor, pageSize)
}

func (service *Service) MarkNotificationRead(ctx context.Context, actor Actor, notificationID string) error {
	if actor.UserID == "" || !validUUID(notificationID) {
		return ErrInvalidInput
	}
	store, ok := service.store.(AccountNotificationStore)
	if !ok {
		return errors.New("account notification store is not configured")
	}
	return store.MarkAccountNotificationRead(ctx, actor, notificationID, service.now().UTC())
}

func (service *Service) MarkAllNotificationsRead(ctx context.Context, actor Actor) error {
	if actor.UserID == "" {
		return ErrInvalidInput
	}
	store, ok := service.store.(AccountNotificationStore)
	if !ok {
		return errors.New("account notification store is not configured")
	}
	return store.MarkAllAccountNotificationsRead(ctx, actor, service.now().UTC())
}

func (service *Service) ListDocuments(ctx context.Context, actor Actor, projectID string) (ProjectDocumentList, error) {
	if !validActorAndProject(actor, projectID) {
		return ProjectDocumentList{}, ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ProjectDocumentList{}, err
	}
	return store.ListProjectDocuments(ctx, actor, projectID)
}

func (service *Service) UploadDocument(ctx context.Context, actor Actor, projectID, name, mediaType, requestID string, reader io.Reader, size int64) (ProjectDocument, error) {
	name, mediaType = strings.TrimSpace(name), strings.TrimSpace(mediaType)
	if !validActorAndProject(actor, projectID) || len([]rune(name)) < 1 || len([]rune(name)) > 240 || len(mediaType) < 3 || len(mediaType) > 127 || size < 1 || size > MaxProjectDocumentBytes || len(requestID) < 8 {
		return ProjectDocument{}, ErrInvalidInput
	}
	content, err := io.ReadAll(io.LimitReader(reader, MaxProjectDocumentBytes+1))
	if err != nil || int64(len(content)) != size || int64(len(content)) > MaxProjectDocumentBytes {
		return ProjectDocument{}, ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ProjectDocument{}, err
	}
	notification, err := service.eventNotification("project.document.uploaded", "В проект загружен документ\nДокумент: "+name)
	if err != nil {
		return ProjectDocument{}, err
	}
	return store.UploadProjectDocument(ctx, UploadProjectDocumentCommand{Actor: actor, ProjectID: projectID, Name: name, MediaType: mediaType, RequestID: requestID, Content: content, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) DownloadDocument(ctx context.Context, actor Actor, projectID, documentID string) (ProjectDocumentContent, error) {
	if !validActorAndProject(actor, projectID) || !validUUID(documentID) {
		return ProjectDocumentContent{}, ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return ProjectDocumentContent{}, err
	}
	return store.DownloadProjectDocument(ctx, actor, projectID, documentID)
}

func (service *Service) DeleteDocument(ctx context.Context, actor Actor, projectID, documentID, requestID string) error {
	if !validActorAndProject(actor, projectID) || !validUUID(documentID) || len(requestID) < 8 {
		return ErrInvalidInput
	}
	store, err := service.collaborationStore()
	if err != nil {
		return err
	}
	notification, err := service.eventNotification("project.document.deleted", "Документ удалён из проекта")
	if err != nil {
		return err
	}
	return store.DeleteProjectDocument(ctx, DeleteProjectDocumentCommand{Actor: actor, ProjectID: projectID, DocumentID: documentID, RequestID: requestID, Now: service.now().UTC(), Notification: notification})
}
