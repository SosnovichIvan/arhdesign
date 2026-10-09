//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func TestProjectNotificationPersistencePropagatesTransactionFailures(t *testing.T) {
	pool := newMigrationSchemaPool(t, "project_notification_failures")
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	notification := &project.EventNotification{
		MessageType:       "project.task.updated",
		Plaintext:         "Задача изменена",
		PayloadCiphertext: []byte("ciphertext"),
		PayloadKeyVersion: 1,
	}
	assertClosedTransaction := func(t *testing.T, call func(pgx.Tx) error) {
		t.Helper()
		if err := call(tx); err == nil {
			t.Fatal("closed transaction failure was not propagated")
		}
	}

	cases := []struct {
		name string
		call func(pgx.Tx) error
	}{
		{"scoped notification", func(tx pgx.Tx) error {
			return enqueueScopedProjectNotification(ctx, tx, "project", "actor", "event", "entity", "target", notificationScopeTask, notification, now)
		}},
		{"project member notification", func(tx pgx.Tx) error {
			return enqueueProjectMemberNotification(ctx, tx, "project", "actor", "event", notification, now)
		}},
		{"chat member notification", func(tx pgx.Tx) error {
			return enqueueChatMemberNotification(ctx, tx, "project", "chat", "actor", "event", notification, now)
		}},
		{"task assignee notification", func(tx pgx.Tx) error {
			return enqueueTaskAssigneeNotification(ctx, tx, "project", "task", "actor", notification, now)
		}},
		{"context chat notification", func(tx pgx.Tx) error {
			return enqueueContextChatNotification(ctx, tx, "project", "chat", "message", "actor", notification, now)
		}},
		{"project member account notification", func(tx pgx.Tx) error {
			return insertProjectMemberAccountNotifications(ctx, tx, "project", "actor", "event", notification, now)
		}},
		{"context chat account notification", func(tx pgx.Tx) error {
			return insertContextChatAccountNotifications(ctx, tx, "project", "chat", "message", "actor", notification, now)
		}},
		{"task assignee account notification", func(tx pgx.Tx) error {
			return insertTaskAssigneeAccountNotifications(ctx, tx, "project", "task", "actor", notification, now)
		}},
		{"scoped account notification", func(tx pgx.Tx) error {
			return insertScopedAccountNotifications(ctx, tx, "project", "actor", "event", "entity", "target", notificationScopeTask, notification, now)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertClosedTransaction(t, test.call)
		})
	}
}
