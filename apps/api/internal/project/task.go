package project

import (
	"context"
	"time"
)

const (
	TaskNew              = "new"
	TaskInProgress       = "in_progress"
	TaskReview           = "review"
	TaskChangesRequested = "changes_requested"
	TaskAccepted         = "accepted"
)

var taskStatuses = map[string]struct{}{
	TaskNew: {}, TaskInProgress: {}, TaskReview: {}, TaskChangesRequested: {}, TaskAccepted: {},
}

type TaskAssignee struct {
	UserID, Login, FirstName string
	LastName                 *string
}

type Task struct {
	ID, ProjectID, Title, Status string
	Description, ContextChatID   *string
	DueAt, CreatedAt             time.Time
	StartedAt, CompletedAt       *time.Time
	Assignees                    []TaskAssignee
	CanEdit, CanChangeStatus     bool
	Version                      int64
}

type TaskList struct {
	Items     []Task
	CanCreate bool
}

type TaskQuery struct {
	Actor
	ProjectID string
}

type UpdateTaskRequest struct {
	Status          string
	ExpectedVersion int64
	RequestID       string
}

type UpdateTaskCommand struct {
	Actor
	ProjectID, TaskID, Status, RequestID string
	ExpectedVersion                      int64
	Now                                  time.Time
	Notification                         *EventNotification
}

func (service *Service) ListTasks(ctx context.Context, actor Actor, projectID string) (TaskList, error) {
	if !validUUID(actor.UserID) {
		return TaskList{}, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return TaskList{}, ErrNotFound
	}
	items, canCreate, err := service.store.ListProjectTasks(ctx, TaskQuery{Actor: actor, ProjectID: projectID})
	return TaskList{Items: items, CanCreate: canCreate}, err
}

func (service *Service) UpdateTask(ctx context.Context, actor Actor, projectID, taskID string, request UpdateTaskRequest) (Task, error) {
	if !validUUID(actor.UserID) || len(request.RequestID) < 8 || request.ExpectedVersion < 1 {
		return Task{}, ErrInvalidInput
	}
	if !validUUID(projectID) || !validUUID(taskID) {
		return Task{}, ErrNotFound
	}
	if _, ok := taskStatuses[request.Status]; !ok {
		return Task{}, ErrInvalidInput
	}
	notification, err := service.eventNotification("project.task.status_changed", "Статус задачи изменён\nЗадача: "+taskID+"\nНовый статус: "+taskStatusLabel(request.Status))
	if err != nil {
		return Task{}, err
	}
	return service.store.UpdateProjectTask(ctx, UpdateTaskCommand{
		Actor: actor, ProjectID: projectID, TaskID: taskID, Status: request.Status,
		ExpectedVersion: request.ExpectedVersion, RequestID: request.RequestID, Now: service.now().UTC(),
		Notification: notification,
	})
}

func taskStatusLabel(status string) string {
	return map[string]string{
		TaskNew: "Новая", TaskInProgress: "В работе", TaskReview: "На проверке",
		TaskChangesRequested: "Нужны изменения", TaskAccepted: "Принята",
	}[status]
}
