package project

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskServiceListsAndUpdatesWithValidatedContract(t *testing.T) {
	store := &fakeStore{tasks: []Task{{ID: "00000000-0000-0000-0000-000000000003", ProjectID: "00000000-0000-0000-0000-000000000002", Status: TaskNew, Version: 1}}, canCreateTask: true, updatedTask: Task{Status: TaskInProgress, Version: 2}}
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	service, _ := NewService(store, []byte("task-service-test-cursor-key-32---"), func() time.Time { return now })
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	list, err := service.ListTasks(context.Background(), actor, projectID)
	if err != nil || !list.CanCreate || len(list.Items) != 1 {
		t.Fatalf("list=%#v error=%v", list, err)
	}
	updated, err := service.UpdateTask(context.Background(), actor, projectID, "00000000-0000-0000-0000-000000000003", UpdateTaskRequest{Status: TaskInProgress, ExpectedVersion: 1, RequestID: "request-task-update"})
	if err != nil || updated.Status != TaskInProgress || store.updateTask.Now != now {
		t.Fatalf("updated=%#v command=%#v error=%v", updated, store.updateTask, err)
	}
}

func TestTaskServiceRejectsInvalidStatusAndAssignees(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	service, _ := NewService(store, []byte("task-service-test-cursor-key-32---"), func() time.Time { return now })
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	if _, err := service.UpdateTask(context.Background(), actor, projectID, "00000000-0000-0000-0000-000000000003", UpdateTaskRequest{Status: "unknown", ExpectedVersion: 1, RequestID: "request-task-update"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid status error=%v", err)
	}
	if _, err := service.CreateTask(context.Background(), actor, projectID, CreateTaskRequest{Title: "Задача", DueAt: now.Add(time.Hour), AssigneeUserIDs: []string{"00000000-0000-0000-0000-000000000004", "00000000-0000-0000-0000-000000000004"}, RequestID: "request-task-create"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("duplicate assignee error=%v", err)
	}
}
