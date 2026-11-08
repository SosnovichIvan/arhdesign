package project

import (
	"context"
	"strings"
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
	AssigneeUserIDs *[]string
	ExpectedVersion int64
	RequestID       string
}

type UpdateTaskCommand struct {
	Actor
	ProjectID, TaskID, RequestID string
	Status                       string
	AssigneeUserIDs              *[]string
	ExpectedVersion              int64
	Now                          time.Time
	Notification                 *EventNotification
	AssignmentNotification       *EventNotification
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
	if !validUUID(actor.UserID) || len(request.RequestID) < 8 || request.ExpectedVersion < 1 || (request.Status == "" && request.AssigneeUserIDs == nil) {
		return Task{}, ErrInvalidInput
	}
	if !validUUID(projectID) || !validUUID(taskID) {
		return Task{}, ErrNotFound
	}
	var statusNotification *EventNotification
	if request.Status != "" {
		status := strings.TrimSpace(request.Status)
		if _, ok := taskStatuses[status]; !ok {
			return Task{}, ErrInvalidInput
		}
		request.Status = status
		var err error
		statusNotification, err = service.eventNotification("project.task.status_changed", "Статус задачи изменён\nЗадача: "+taskID+"\nНовый статус: "+taskStatusLabel(status))
		if err != nil {
			return Task{}, err
		}
	}
	var assignmentNotification *EventNotification
	if request.AssigneeUserIDs != nil {
		assignees := uniqueIDs(*request.AssigneeUserIDs)
		if len(assignees) != len(*request.AssigneeUserIDs) || len(assignees) < 1 || len(assignees) > 50 {
			return Task{}, ErrInvalidInput
		}
		for _, assignee := range assignees {
			if !validUUID(assignee) {
				return Task{}, ErrInvalidInput
			}
		}
		request.AssigneeUserIDs = &assignees
		var err error
		assignmentNotification, err = service.eventNotification("project.task.assignees_changed", "Изменён состав ответственных по задаче\nЗадача: "+taskID)
		if err != nil {
			return Task{}, err
		}
	}
	return service.store.UpdateProjectTask(ctx, UpdateTaskCommand{
		Actor: actor, ProjectID: projectID, TaskID: taskID, Status: request.Status, AssigneeUserIDs: request.AssigneeUserIDs,
		ExpectedVersion: request.ExpectedVersion, RequestID: request.RequestID, Now: service.now().UTC(),
		Notification: statusNotification, AssignmentNotification: assignmentNotification,
	})
}

func taskStatusLabel(status string) string {
	return map[string]string{
		TaskNew: "Новая", TaskInProgress: "В работе", TaskReview: "На проверке",
		TaskChangesRequested: "Нужны изменения", TaskAccepted: "Принята",
	}[status]
}
