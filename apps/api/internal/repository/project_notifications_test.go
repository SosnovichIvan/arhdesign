package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
)

func TestAccountNotificationPresentationCatalog(t *testing.T) {
	projectID := "00000000-0000-0000-0000-000000000001"
	chatID := "00000000-0000-0000-0000-000000000002"
	tests := []struct {
		messageType string
		entityID    string
		title       string
		href        string
	}{
		{"project.task.assigned", "", "Изменение задачи", "/account/projects/" + projectID + "/tasks"},
		{"project.meeting.created", "", "Изменение встречи", "/account/projects/" + projectID + "/calendar"},
		{"project.material.updated", "", "Изменение материала", "/account/projects/" + projectID + "/materials"},
		{"project.expense.created", "", "Изменение финансов", "/account/projects/" + projectID + "/finances"},
		{"project.finance.updated", "", "Изменение финансов", "/account/projects/" + projectID + "/finances"},
		{"project.member.added", "", "Изменение команды проекта", "/account/projects/" + projectID},
		{"project.document.uploaded", "", "Изменение документации", "/account/projects/" + projectID + "/documents"},
		{"project.chat.created", chatID, "Изменение чата", "/account/projects/" + projectID + "/chat/" + chatID},
		{"project.chat.updated", "", "Изменение чата", "/account/projects/" + projectID + "/chat"},
		{"project.chat.message_created", chatID, "Новое сообщение", "/account/projects/" + projectID + "/chat/" + chatID},
		{"project.updated", "", "Изменение в проекте", "/account/projects/" + projectID},
	}
	for _, test := range tests {
		t.Run(test.messageType, func(t *testing.T) {
			notification := &project.EventNotification{MessageType: test.messageType, Plaintext: "Описание события"}
			title, body := accountNotificationCopy(notification)
			if title != test.title || body != "Описание события" {
				t.Fatalf("title=%q body=%q", title, body)
			}
			if href := accountNotificationHref(projectID, test.entityID, test.messageType); href != test.href {
				t.Fatalf("href=%q want=%q", href, test.href)
			}
		})
	}
}

func TestAccountNotificationCopyProvidesFallbackAndBoundsBody(t *testing.T) {
	title, body := accountNotificationCopy(&project.EventNotification{MessageType: "project.updated", Plaintext: "  "})
	if title != "Изменение в проекте" || body != title {
		t.Fatalf("fallback title=%q body=%q", title, body)
	}

	longBody := strings.Repeat("я", 5001)
	_, body = accountNotificationCopy(&project.EventNotification{MessageType: "project.updated", Plaintext: longBody})
	if len([]rune(body)) != 5000 || !strings.HasSuffix(body, "…") {
		t.Fatalf("bounded body has %d runes and suffix=%t", len([]rune(body)), strings.HasSuffix(body, "…"))
	}
}

func TestNilProjectNotificationsAreNoops(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		call func() error
	}{
		{"scoped", func() error {
			return enqueueScopedProjectNotification(ctx, nil, "project", "actor", "event", "entity", "target", notificationScopeTask, nil, now)
		}},
		{"project members", func() error { return enqueueProjectMemberNotification(ctx, nil, "project", "actor", "event", nil, now) }},
		{"chat members", func() error {
			return enqueueChatMemberNotification(ctx, nil, "project", "chat", "actor", "event", nil, now)
		}},
		{"task assignees", func() error { return enqueueTaskAssigneeNotification(ctx, nil, "project", "task", "actor", nil, now) }},
		{"context chat", func() error {
			return enqueueContextChatNotification(ctx, nil, "project", "chat", "message", "actor", nil, now)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err != nil {
				t.Fatalf("nil notification must be a no-op: %v", err)
			}
		})
	}
}
