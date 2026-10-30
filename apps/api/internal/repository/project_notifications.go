package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

const (
	notificationScopeFinancials = "financials"
	notificationScopeMaterials  = "materials"
	notificationScopeTask       = "task"
	notificationScopeMemberAdd  = "member_add"
	notificationScopeAdmins     = "admins"
)

func enqueueScopedProjectNotification(ctx context.Context, tx pgx.Tx, projectID, actorID, eventID, entityID, targetUserID, scope string, notification *project.EventNotification, now time.Time) error {
	if notification == nil {
		return nil
	}
	if err := insertScopedAccountNotifications(ctx, tx, projectID, actorID, eventID, entityID, targetUserID, scope, notification, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_outbox (
			recipient_user_id,channel,message_type,idempotency_key,
			payload_ciphertext,payload_key_version,metadata,created_at,updated_at
		)
		SELECT DISTINCT membership.user_id,'telegram',$7::text,$7::text || ':' || $3::text || ':' || membership.user_id,
		       $8::bytea,$9::integer,jsonb_build_object('projectId',$1::text,'eventId',$3::text,'entityId',NULLIF($4::text,'')),$10::timestamptz,$10::timestamptz
		FROM project_memberships membership
		JOIN users ON users.id=membership.user_id AND users.status='active'
		JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
		LEFT JOIN account_notification_preferences preferences ON preferences.user_id=users.id
		WHERE membership.project_id=$1::uuid AND membership.revoked_at IS NULL AND membership.user_id<>$2::uuid
		  AND COALESCE(preferences.telegram_events_enabled,TRUE)
		  AND (
		    ($6::text='financials' AND (membership.project_role IN ('customer','project_admin') OR membership.privileges @> '{"financials.view":true}'::jsonb))
		    OR ($6::text='materials' AND (membership.project_role IN ('customer','project_admin') OR membership.privileges @> '{"materials.view":true}'::jsonb))
		    OR ($6::text='task' AND (
		      membership.project_role IN ('customer','project_admin')
		      OR membership.privileges @> '{"tasks.view_all":true}'::jsonb
		      OR EXISTS (SELECT 1 FROM project_task_assignees assignee WHERE assignee.task_id=NULLIF($4::text,'')::uuid AND assignee.user_id=membership.user_id)
		    ))
		    OR ($6::text='member_add' AND (membership.user_id=NULLIF($5::text,'')::uuid OR membership.project_role='project_admin'))
		    OR ($6::text='admins' AND membership.project_role='project_admin')
		  )
		ON CONFLICT (idempotency_key) DO NOTHING
	`, projectID, actorID, eventID, entityID, targetUserID, scope, notification.MessageType, notification.PayloadCiphertext, notification.PayloadKeyVersion, now)
	if err != nil {
		return fmt.Errorf("enqueue scoped project notification: %w", err)
	}
	return nil
}

func enqueueProjectMemberNotification(ctx context.Context, tx pgx.Tx, projectID, actorID, eventID string, notification *project.EventNotification, now time.Time) error {
	if notification == nil {
		return nil
	}
	if err := insertProjectMemberAccountNotifications(ctx, tx, projectID, actorID, eventID, notification, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_outbox (
			recipient_user_id,channel,message_type,idempotency_key,
			payload_ciphertext,payload_key_version,metadata,created_at,updated_at
		)
		SELECT membership.user_id,'telegram',$4::text,$4::text || ':' || $3::text || ':' || membership.user_id,
		       $5,$6,jsonb_build_object('projectId',$1::text,'eventId',$3::text),$7,$7
		FROM project_memberships membership
		JOIN users ON users.id=membership.user_id AND users.status='active'
		JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
		LEFT JOIN account_notification_preferences preferences ON preferences.user_id=users.id
		WHERE membership.project_id=$1::uuid AND membership.revoked_at IS NULL AND membership.user_id<>$2::uuid
		  AND COALESCE(preferences.telegram_events_enabled,TRUE)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, projectID, actorID, eventID, notification.MessageType, notification.PayloadCiphertext, notification.PayloadKeyVersion, now)
	if err != nil {
		return fmt.Errorf("enqueue project member notification: %w", err)
	}
	return nil
}

func enqueueChatMemberNotification(ctx context.Context, tx pgx.Tx, projectID, chatID, actorID, eventID string, notification *project.EventNotification, now time.Time) error {
	if notification == nil {
		return nil
	}
	title, body := accountNotificationCopy(notification)
	href := accountNotificationHref(projectID, chatID, notification.MessageType)
	if _, err := tx.Exec(ctx, `
		INSERT INTO account_notifications(recipient_user_id,project_id,actor_user_id,event_type,event_id,title,body,href,created_at)
		SELECT membership.user_id,$1::uuid,$3::uuid,$5::text,$4::text,$6::text,$7::text,$8::text,$9::timestamptz
		FROM chat_memberships membership
		JOIN users ON users.id=membership.user_id AND users.status='active'
		WHERE membership.chat_id=$2::uuid AND membership.project_id=$1::uuid AND membership.removed_at IS NULL AND membership.user_id<>$3::uuid
		ON CONFLICT (recipient_user_id,event_type,event_id) DO NOTHING
	`, projectID, chatID, actorID, eventID, notification.MessageType, title, body, href, now); err != nil {
		return fmt.Errorf("insert chat member account notifications: %w", err)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_outbox (
			recipient_user_id,channel,message_type,idempotency_key,
			payload_ciphertext,payload_key_version,metadata,created_at,updated_at
		)
		SELECT membership.user_id,'telegram',$5::text,$5::text || ':' || $4::text || ':' || membership.user_id,
		       $6,$7,jsonb_build_object('projectId',$1::text,'chatId',$2::text,'eventId',$4::text),$8,$8
		FROM chat_memberships membership
		JOIN users ON users.id=membership.user_id AND users.status='active'
		JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
		LEFT JOIN account_notification_preferences preferences ON preferences.user_id=users.id
		WHERE membership.chat_id=$2::uuid AND membership.project_id=$1::uuid AND membership.removed_at IS NULL
		  AND membership.user_id<>$3::uuid AND COALESCE(preferences.telegram_events_enabled,TRUE)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, projectID, chatID, actorID, eventID, notification.MessageType, notification.PayloadCiphertext, notification.PayloadKeyVersion, now)
	if err != nil {
		return fmt.Errorf("enqueue chat member notification: %w", err)
	}
	return nil
}

func enqueueTaskAssigneeNotification(ctx context.Context, tx pgx.Tx, projectID, taskID, actorID string, notification *project.EventNotification, now time.Time) error {
	if notification == nil {
		return nil
	}
	if err := insertTaskAssigneeAccountNotifications(ctx, tx, projectID, taskID, actorID, notification, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_outbox (
			recipient_user_id,channel,message_type,idempotency_key,
			payload_ciphertext,payload_key_version,metadata,created_at,updated_at
		)
		SELECT assignee.user_id,'telegram',$4::text,$4::text || ':' || $2::text || ':' || assignee.user_id,
		       $5,$6,jsonb_build_object('projectId',$1::text,'taskId',$2::text),$7,$7
		FROM project_task_assignees assignee
		JOIN project_memberships membership ON membership.project_id=$1::uuid AND membership.user_id=assignee.user_id AND membership.revoked_at IS NULL
		JOIN users ON users.id=assignee.user_id AND users.status='active'
		JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
		LEFT JOIN account_notification_preferences preferences ON preferences.user_id=users.id
		WHERE assignee.task_id=$2::uuid AND assignee.user_id<>$3::uuid
		  AND COALESCE(preferences.telegram_events_enabled,TRUE)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, projectID, taskID, actorID, notification.MessageType, notification.PayloadCiphertext, notification.PayloadKeyVersion, now)
	if err != nil {
		return fmt.Errorf("enqueue task assignee notification: %w", err)
	}
	return nil
}

func enqueueContextChatNotification(ctx context.Context, tx pgx.Tx, projectID, chatID, messageID, actorID string, notification *project.EventNotification, now time.Time) error {
	if notification == nil {
		return nil
	}
	if err := insertContextChatAccountNotifications(ctx, tx, projectID, chatID, messageID, actorID, notification, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_outbox (
			recipient_user_id,channel,message_type,idempotency_key,
			payload_ciphertext,payload_key_version,metadata,created_at,updated_at
		)
		SELECT membership.user_id,'telegram',$5::text,$5::text || ':' || $3::text || ':' || membership.user_id,
		       $6,$7,jsonb_build_object('projectId',$1::text,'chatId',$2::text,'messageId',$3::text),$8,$8
		FROM chats chat
		JOIN chat_memberships membership ON membership.chat_id=chat.id AND membership.project_id=chat.project_id AND membership.removed_at IS NULL
		JOIN users ON users.id=membership.user_id AND users.status='active'
		JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
		LEFT JOIN account_notification_preferences preferences ON preferences.user_id=users.id
		WHERE chat.id=$2::uuid AND chat.project_id=$1::uuid AND chat.archived_at IS NULL
		  AND membership.user_id<>$4::uuid AND COALESCE(preferences.telegram_events_enabled,TRUE)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, projectID, chatID, messageID, actorID, notification.MessageType, notification.PayloadCiphertext, notification.PayloadKeyVersion, now)
	if err != nil {
		return fmt.Errorf("enqueue context chat notification: %w", err)
	}
	return nil
}

func insertProjectMemberAccountNotifications(ctx context.Context, tx pgx.Tx, projectID, actorID, eventID string, notification *project.EventNotification, now time.Time) error {
	title, body := accountNotificationCopy(notification)
	href := accountNotificationHref(projectID, "", notification.MessageType)
	_, err := tx.Exec(ctx, `
		INSERT INTO account_notifications(recipient_user_id,project_id,actor_user_id,event_type,event_id,title,body,href,created_at)
		SELECT membership.user_id,$1::uuid,$2::uuid,$4::text,$3::text,$5::text,$6::text,$7::text,$8::timestamptz
		FROM project_memberships membership
		JOIN users ON users.id=membership.user_id AND users.status='active'
		WHERE membership.project_id=$1::uuid AND membership.revoked_at IS NULL AND membership.user_id<>$2::uuid
		ON CONFLICT (recipient_user_id,event_type,event_id) DO NOTHING
	`, projectID, actorID, eventID, notification.MessageType, title, body, href, now)
	if err != nil {
		return fmt.Errorf("insert project member account notifications: %w", err)
	}
	return nil
}

func insertContextChatAccountNotifications(ctx context.Context, tx pgx.Tx, projectID, chatID, messageID, actorID string, notification *project.EventNotification, now time.Time) error {
	title, body := accountNotificationCopy(notification)
	_, err := tx.Exec(ctx, `
		INSERT INTO account_notifications(recipient_user_id,project_id,actor_user_id,event_type,event_id,title,body,href,created_at)
		SELECT membership.user_id,$1::uuid,$4::uuid,$5::text,$3::text,$6::text,$7::text,'/account/projects/' || $1::text || '/chat/' || $2::text,$8::timestamptz
		FROM chats chat
		JOIN chat_memberships membership ON membership.chat_id=chat.id AND membership.project_id=chat.project_id AND membership.removed_at IS NULL
		JOIN users ON users.id=membership.user_id AND users.status='active'
		WHERE chat.id=$2::uuid AND chat.project_id=$1::uuid AND chat.archived_at IS NULL AND membership.user_id<>$4::uuid
		ON CONFLICT (recipient_user_id,event_type,event_id) DO NOTHING
	`, projectID, chatID, messageID, actorID, notification.MessageType, title, body, now)
	if err != nil {
		return fmt.Errorf("insert context chat account notifications: %w", err)
	}
	return nil
}

func insertTaskAssigneeAccountNotifications(ctx context.Context, tx pgx.Tx, projectID, taskID, actorID string, notification *project.EventNotification, now time.Time) error {
	title, body := accountNotificationCopy(notification)
	_, err := tx.Exec(ctx, `
		INSERT INTO account_notifications(recipient_user_id,project_id,actor_user_id,event_type,event_id,title,body,href,created_at)
		SELECT assignee.user_id,$1::uuid,$3::uuid,$4::text,$2::text,$5::text,$6::text,'/account/projects/' || $1::text || '/tasks',$7::timestamptz
		FROM project_task_assignees assignee
		JOIN project_memberships membership ON membership.project_id=$1::uuid AND membership.user_id=assignee.user_id AND membership.revoked_at IS NULL
		JOIN users ON users.id=assignee.user_id AND users.status='active'
		WHERE assignee.task_id=$2::uuid AND assignee.user_id<>$3::uuid
		ON CONFLICT (recipient_user_id,event_type,event_id) DO NOTHING
	`, projectID, taskID, actorID, notification.MessageType, title, body, now)
	if err != nil {
		return fmt.Errorf("insert task assignee account notifications: %w", err)
	}
	return nil
}

func insertScopedAccountNotifications(ctx context.Context, tx pgx.Tx, projectID, actorID, eventID, entityID, targetUserID, scope string, notification *project.EventNotification, now time.Time) error {
	title, body := accountNotificationCopy(notification)
	href := "/account/projects/" + projectID
	switch scope {
	case notificationScopeFinancials:
		href += "/finances"
	case notificationScopeMaterials:
		href += "/materials"
	case notificationScopeTask:
		href += "/tasks"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO account_notifications(recipient_user_id,project_id,actor_user_id,event_type,event_id,title,body,href,created_at)
		SELECT DISTINCT membership.user_id,$1::uuid,$2::uuid,$7::text,$3::text,$8::text,$9::text,$10::text,$11::timestamptz
		FROM project_memberships membership
		JOIN users ON users.id=membership.user_id AND users.status='active'
		WHERE membership.project_id=$1::uuid AND membership.revoked_at IS NULL AND membership.user_id<>$2::uuid
		  AND (
		    ($6::text='financials' AND (membership.project_role IN ('customer','project_admin') OR membership.privileges @> '{"financials.view":true}'::jsonb))
		    OR ($6::text='materials' AND (membership.project_role IN ('customer','project_admin') OR membership.privileges @> '{"materials.view":true}'::jsonb))
		    OR ($6::text='task' AND (membership.project_role IN ('customer','project_admin') OR membership.privileges @> '{"tasks.view_all":true}'::jsonb OR EXISTS (SELECT 1 FROM project_task_assignees assignee WHERE assignee.task_id=NULLIF($4::text,'')::uuid AND assignee.user_id=membership.user_id)))
		    OR ($6::text='member_add' AND (membership.user_id=NULLIF($5::text,'')::uuid OR membership.project_role='project_admin'))
		    OR ($6::text='admins' AND membership.project_role='project_admin')
		  )
		ON CONFLICT (recipient_user_id,event_type,event_id) DO NOTHING
	`, projectID, actorID, eventID, entityID, targetUserID, scope, notification.MessageType, title, body, href, now)
	if err != nil {
		return fmt.Errorf("insert scoped account notifications: %w", err)
	}
	return nil
}

func accountNotificationCopy(notification *project.EventNotification) (string, string) {
	title := "Изменение в проекте"
	switch {
	case strings.Contains(notification.MessageType, "chat") && strings.Contains(notification.MessageType, "message"):
		title = "Новое сообщение"
	case strings.Contains(notification.MessageType, "chat"):
		title = "Изменение чата"
	case strings.Contains(notification.MessageType, "task"):
		title = "Изменение задачи"
	case strings.Contains(notification.MessageType, "meeting"):
		title = "Изменение встречи"
	case strings.Contains(notification.MessageType, "material"):
		title = "Изменение материала"
	case strings.Contains(notification.MessageType, "expense") || strings.Contains(notification.MessageType, "finance"):
		title = "Изменение финансов"
	case strings.Contains(notification.MessageType, "member"):
		title = "Изменение команды проекта"
	case strings.Contains(notification.MessageType, "document"):
		title = "Изменение документации"
	}
	body := strings.TrimSpace(notification.Plaintext)
	if body == "" {
		body = title
	}
	runes := []rune(body)
	if len(runes) > 5000 {
		body = string(runes[:4999]) + "…"
	}
	return title, body
}

func accountNotificationHref(projectID, entityID, messageType string) string {
	base := "/account/projects/" + projectID
	switch {
	case strings.Contains(messageType, "document"):
		return base + "/documents"
	case strings.Contains(messageType, "material"):
		return base + "/materials"
	case strings.Contains(messageType, "expense") || strings.Contains(messageType, "finance"):
		return base + "/finances"
	case strings.Contains(messageType, "task"):
		return base + "/tasks"
	case strings.Contains(messageType, "meeting"):
		return base + "/calendar"
	case strings.Contains(messageType, "chat") && entityID != "":
		return base + "/chat/" + entityID
	case strings.Contains(messageType, "chat"):
		return base + "/chat"
	default:
		return base
	}
}
