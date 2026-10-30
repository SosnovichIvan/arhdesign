package project

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ChatAuthor struct {
	UserID, Login, FirstName string
	LastName                 *string
}

type ChatMessage struct {
	ID, ChatID, ProjectID, Body string
	Author                      ChatAuthor
	CreatedAt                   time.Time
	EditedAt, DeletedAt         *time.Time
	Version                     int64
}

type ProjectChatPage struct {
	ChatID, ProjectID, CurrentUserID string
	Messages                         []ChatMessage
	NextCursor                       *string
	HasMore                          bool
	CanSend                          bool
	Context                          *ChatContext
}

type ChatContext struct {
	ChatID, ProjectID, ContextType, ContextID, ContextTitle, Name string
}

type ChatQuery struct {
	Actor
	ProjectID, ChatID string
	PageSize          int
	BeforeID          *string
}

type OpenContextChatRequest struct {
	ContextType, ContextID, Name, RequestID string
}

type OpenContextChatCommand struct {
	Actor
	ProjectID, ContextType, ContextID, Name, RequestID string
	Now                                                time.Time
}

type CreateChatMessageRequest struct {
	Body, ClientMessageID, RequestID string
}

type CreateChatMessageCommand struct {
	Actor
	ProjectID, ChatID, Body, ClientMessageID, RequestID string
	Now                                                 time.Time
	Notification                                        *EventNotification
}

var allowedChatContextTypes = map[string]struct{}{"task": {}, "material": {}, "expense": {}}

func (service *Service) ListChat(ctx context.Context, actor Actor, projectID string, pageSize int, before string) (ProjectChatPage, error) {
	query, err := chatQuery(actor, projectID, "", pageSize, before)
	if err != nil {
		return ProjectChatPage{}, ErrInvalidInput
	}
	return service.store.ListProjectChat(ctx, query)
}

func (service *Service) OpenContextChat(ctx context.Context, actor Actor, projectID string, request OpenContextChatRequest) (ChatContext, error) {
	request.ContextType = strings.TrimSpace(request.ContextType)
	request.Name = strings.TrimSpace(request.Name)
	if !validActorAndProject(actor, projectID) || !validUUID(request.ContextID) || len(request.RequestID) < 8 || len(request.RequestID) > 100 || len([]rune(request.Name)) < 2 || len([]rune(request.Name)) > 200 {
		return ChatContext{}, ErrInvalidInput
	}
	if _, allowed := allowedChatContextTypes[request.ContextType]; !allowed {
		return ChatContext{}, ErrInvalidInput
	}
	return service.store.OpenProjectContextChat(ctx, OpenContextChatCommand{
		Actor: actor, ProjectID: projectID, ContextType: request.ContextType, ContextID: request.ContextID,
		Name: request.Name, RequestID: request.RequestID, Now: service.now().UTC(),
	})
}

func (service *Service) ListContextChat(ctx context.Context, actor Actor, projectID, chatID string, pageSize int, before string) (ProjectChatPage, error) {
	query, err := chatQuery(actor, projectID, chatID, pageSize, before)
	if err != nil {
		return ProjectChatPage{}, ErrInvalidInput
	}
	return service.store.ListProjectContextChat(ctx, query)
}

func chatQuery(actor Actor, projectID, chatID string, pageSize int, before string) (ChatQuery, error) {
	if !validActorAndProject(actor, projectID) || (chatID != "" && !validUUID(chatID)) {
		return ChatQuery{}, ErrInvalidInput
	}
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize < 1 || pageSize > 100 {
		return ChatQuery{}, ErrInvalidInput
	}
	var beforeID *string
	if before != "" {
		parsed, err := uuid.Parse(before)
		if err != nil || parsed == uuid.Nil {
			return ChatQuery{}, ErrInvalidInput
		}
		value := parsed.String()
		beforeID = &value
	}
	return ChatQuery{Actor: actor, ProjectID: projectID, ChatID: chatID, PageSize: pageSize, BeforeID: beforeID}, nil
}

func (service *Service) SendChatMessage(ctx context.Context, actor Actor, projectID string, request CreateChatMessageRequest) (ChatMessage, error) {
	body := strings.TrimSpace(request.Body)
	clientMessageID, clientErr := uuid.Parse(strings.TrimSpace(request.ClientMessageID))
	if !validActorAndProject(actor, projectID) || clientErr != nil || clientMessageID == uuid.Nil || len([]rune(body)) < 1 || len([]rune(body)) > 5000 || len(request.RequestID) < 8 || len(request.RequestID) > 100 {
		return ChatMessage{}, ErrInvalidInput
	}
	notification, err := service.eventNotification("project.chat.message_created", "Новое сообщение в чате проекта\nОт: "+actorDisplayName(actor)+" (@"+actor.Login+")\n\n"+notificationPreview(body, 3000))
	if err != nil {
		return ChatMessage{}, err
	}
	return service.store.CreateProjectChatMessage(ctx, CreateChatMessageCommand{
		Actor: actor, ProjectID: projectID, Body: body, ClientMessageID: clientMessageID.String(), RequestID: request.RequestID, Now: service.now().UTC(),
		Notification: notification,
	})
}

func (service *Service) SendContextChatMessage(ctx context.Context, actor Actor, projectID, chatID string, request CreateChatMessageRequest) (ChatMessage, error) {
	body := strings.TrimSpace(request.Body)
	clientMessageID, clientErr := uuid.Parse(strings.TrimSpace(request.ClientMessageID))
	if !validActorAndProject(actor, projectID) || !validUUID(chatID) || clientErr != nil || clientMessageID == uuid.Nil || len([]rune(body)) < 1 || len([]rune(body)) > 5000 || len(request.RequestID) < 8 || len(request.RequestID) > 100 {
		return ChatMessage{}, ErrInvalidInput
	}
	notification, err := service.eventNotification("project.context_chat.message_created", "Новое сообщение в обсуждении проекта\nОт: "+actorDisplayName(actor)+" (@"+actor.Login+")\n\n"+notificationPreview(body, 3000))
	if err != nil {
		return ChatMessage{}, err
	}
	return service.store.CreateProjectContextChatMessage(ctx, CreateChatMessageCommand{
		Actor: actor, ProjectID: projectID, ChatID: chatID, Body: body,
		ClientMessageID: clientMessageID.String(), RequestID: request.RequestID, Now: service.now().UTC(),
		Notification: notification,
	})
}
