package project

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	UpcomingTask    = "task"
	UpcomingMeeting = "meeting"
)

type UpcomingItem struct {
	ID, ProjectID, Kind, Title string
	Description, Status        *string
	Location                   *string
	EffectiveAt, CreatedAt     time.Time
	EndsAt                     *time.Time
	Version                    int64
}

type UpcomingFeed struct {
	Days                 int
	RangeStart, RangeEnd time.Time
	Items                []UpcomingItem
}

type CalendarFeed struct {
	RangeStart, RangeEnd time.Time
	Items                []UpcomingItem
}

type UpcomingQuery struct {
	Actor
	ProjectID       string
	RangeStart, End time.Time
}

type CreateTaskRequest struct {
	Title, Description, RequestID string
	AssigneeUserIDs               []string
	DueAt                         time.Time
}

type CreateTaskCommand struct {
	Actor
	ProjectID, Title, Description, RequestID string
	AssigneeUserIDs                          []string
	DueAt, Now                               time.Time
	Notification                             *EventNotification
}

type CreateMeetingRequest struct {
	Title, Description, Location, RequestID string
	StartsAt, EndsAt                        time.Time
}

type CreateMeetingCommand struct {
	Actor
	ProjectID, Title, Description, Location, RequestID string
	StartsAt, EndsAt, Now                              time.Time
	Notification                                       *EventNotification
}

func (service *Service) ListUpcoming(ctx context.Context, actor Actor, projectID string, days int) (UpcomingFeed, error) {
	if !validActorAndProject(actor, projectID) || days < 1 || days > 90 {
		return UpcomingFeed{}, ErrInvalidInput
	}
	start := service.now().UTC()
	end := start.Add(time.Duration(days) * 24 * time.Hour)
	items, err := service.store.ListProjectUpcoming(ctx, UpcomingQuery{Actor: actor, ProjectID: projectID, RangeStart: start, End: end})
	if err != nil {
		return UpcomingFeed{}, err
	}
	return UpcomingFeed{Days: days, RangeStart: start, RangeEnd: end, Items: items}, nil
}

func (service *Service) ListCalendar(ctx context.Context, actor Actor, projectID string, start, end time.Time) (CalendarFeed, error) {
	if !validActorAndProject(actor, projectID) || start.IsZero() || end.IsZero() || !end.After(start) || end.Sub(start) > 370*24*time.Hour {
		return CalendarFeed{}, ErrInvalidInput
	}
	start, end = start.UTC(), end.UTC()
	items, err := service.store.ListProjectUpcoming(ctx, UpcomingQuery{Actor: actor, ProjectID: projectID, RangeStart: start, End: end})
	if err != nil {
		return CalendarFeed{}, err
	}
	return CalendarFeed{RangeStart: start, RangeEnd: end, Items: items}, nil
}

func (service *Service) CreateTask(ctx context.Context, actor Actor, projectID string, request CreateTaskRequest) (UpcomingItem, error) {
	now := service.now().UTC()
	title, description := strings.TrimSpace(request.Title), strings.TrimSpace(request.Description)
	assignees := uniqueIDs(request.AssigneeUserIDs)
	if !validActorAndProject(actor, projectID) || len(request.RequestID) < 8 || len([]rune(title)) < 2 || len([]rune(title)) > 200 || len([]rune(description)) > 5000 || len(assignees) != len(request.AssigneeUserIDs) || len(assignees) > 50 || request.DueAt.IsZero() || !request.DueAt.After(now) {
		return UpcomingItem{}, ErrInvalidInput
	}
	for _, assignee := range assignees {
		if !validUUID(assignee) {
			return UpcomingItem{}, ErrInvalidInput
		}
	}
	notification, err := service.eventNotification("project.task.assigned", "Вам назначена задача\nЗадача: "+title+"\nСрок: "+request.DueAt.UTC().Format(time.RFC3339))
	if err != nil {
		return UpcomingItem{}, err
	}
	return service.store.CreateProjectTask(ctx, CreateTaskCommand{Actor: actor, ProjectID: projectID, Title: title, Description: description, AssigneeUserIDs: assignees, RequestID: request.RequestID, DueAt: request.DueAt.UTC(), Now: now, Notification: notification})
}

func (service *Service) CreateMeeting(ctx context.Context, actor Actor, projectID string, request CreateMeetingRequest) (UpcomingItem, error) {
	now := service.now().UTC()
	title, description, location := strings.TrimSpace(request.Title), strings.TrimSpace(request.Description), strings.TrimSpace(request.Location)
	if !validActorAndProject(actor, projectID) || len(request.RequestID) < 8 || len([]rune(title)) < 2 || len([]rune(title)) > 200 || len([]rune(description)) > 5000 || len([]rune(location)) > 500 || request.StartsAt.IsZero() || request.EndsAt.IsZero() || !request.StartsAt.After(now) || !request.EndsAt.After(request.StartsAt) {
		return UpcomingItem{}, ErrInvalidInput
	}
	notification, err := service.eventNotification("project.meeting.created", "Новая встреча по проекту\nВстреча: "+title+"\nНачало: "+request.StartsAt.UTC().Format(time.RFC3339))
	if err != nil {
		return UpcomingItem{}, err
	}
	return service.store.CreateProjectMeeting(ctx, CreateMeetingCommand{Actor: actor, ProjectID: projectID, Title: title, Description: description, Location: location, RequestID: request.RequestID, StartsAt: request.StartsAt.UTC(), EndsAt: request.EndsAt.UTC(), Now: now, Notification: notification})
}

func validActorAndProject(actor Actor, projectID string) bool {
	actorID, actorErr := uuid.Parse(actor.UserID)
	projectUUID, projectErr := uuid.Parse(projectID)
	return actorErr == nil && actorID != uuid.Nil && projectErr == nil && projectUUID != uuid.Nil
}
