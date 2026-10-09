//go:build integration

package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTechnicalAdministratorBootstrapIsIdempotentAndAuthenticates(t *testing.T) {
	pool := newMigrationSchemaPool(t, "technical_admin_bootstrap")
	applyMigrationPaths(t, pool, migrationPaths(t))
	service, _ := accountServiceForCollaborationTest(t, pool)

	view, created, err := service.EnsureTechnicalAdmin(context.Background(), "technical.admin", "technical@example.test", "Reliable technical password 2026!")
	if err != nil || !created || view.GlobalRole == nil || *view.GlobalRole != "technical_admin" || view.Status != "active" {
		t.Fatalf("view=%#v created=%v error=%v", view, created, err)
	}
	second, created, err := service.EnsureTechnicalAdmin(context.Background(), "technical.admin", "technical@example.test", "A changed password must not rotate automatically")
	if err != nil || created || second.ID != view.ID {
		t.Fatalf("second=%#v created=%v error=%v", second, created, err)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: "technical.admin", Password: "Reliable technical password 2026!", RequestID: "technical-login"}); err != nil {
		t.Fatalf("technical administrator login: %v", err)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: "technical.admin", Password: "A changed password must not rotate automatically", RequestID: "technical-login-changed"}); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatalf("bootstrap rotated credential unexpectedly: %v", err)
	}
	if err = service.BeginTelegramConfirmation(context.Background(), 82002, "technical-telegram-start"); err != nil {
		t.Fatal(err)
	}
	if err = service.SubmitTelegramLogin(context.Background(), 82002, "technical.admin"); err != nil {
		t.Fatal(err)
	}
	if err = service.CompleteTelegramConfirmation(context.Background(), 82002, 41, "ivan_technical", "Reliable technical password 2026!", "technical-telegram-done"); err != nil {
		t.Fatal(err)
	}
	var technicalSubscriptions int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM telegram_subscribers WHERE user_id=$1 AND chat_id=82002 AND unsubscribed_at IS NULL`, view.ID).Scan(&technicalSubscriptions); err != nil || technicalSubscriptions != 1 {
		t.Fatalf("technical subscriptions=%d error=%v", technicalSubscriptions, err)
	}
	migrated, created, err := service.EnsureTechnicalAdmin(context.Background(), "isosnovich", "isosnovich@yandex.ru", "Semen15052018!")
	if err != nil || created || migrated.ID != view.ID || migrated.Login != "isosnovich" || migrated.Email != "isosnovich@yandex.ru" {
		t.Fatalf("migrated=%#v created=%v error=%v", migrated, created, err)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: "isosnovich", Password: "Semen15052018!", RequestID: "technical-login-migrated"}); err != nil {
		t.Fatalf("migrated technical administrator login: %v", err)
	}
	if _, err = service.Login(context.Background(), account.LoginRequest{Identifier: "technical.admin", Password: "Reliable technical password 2026!", RequestID: "technical-login-obsolete"}); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatalf("obsolete technical administrator login error=%v", err)
	}

	if _, err = pool.Exec(context.Background(), `
		INSERT INTO users (login,login_normalized,email,email_normalized,status,global_role,email_verified_at)
		VALUES ('another.tech','another.tech','another-tech@example.test','another-tech@example.test','active','technical_admin',NOW())
	`); err == nil {
		t.Fatal("database accepted a second active technical administrator")
	}
}

func TestGlobalChatMembershipPrivilegeAndNotificationOutbox(t *testing.T) {
	pool := newMigrationSchemaPool(t, "global_chat_contract")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	ownerID := seedCollaborationUser(t, pool, "chat.owner", "Владелец", nil)
	memberID := seedCollaborationUser(t, pool, "chat.member", "Участник", nil)
	outsiderID := seedCollaborationUser(t, pool, "chat.outsider", "Посторонний", nil)
	technicalID := seedCollaborationUser(t, pool, "chat.technical", "Технический", stringPointer("technical_admin"))
	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)
	current := now
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{44}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := globalchat.NewService(store, bytes.Repeat([]byte{45}, 32), cipher, func() time.Time { return current })
	if err != nil {
		t.Fatal(err)
	}
	owner := globalchat.Actor{UserID: ownerID, Login: "chat.owner", FirstName: "Владелец"}
	created, err := service.Create(context.Background(), owner, globalchat.CreateRequest{Kind: "direct", MemberUserIDs: []string{memberID}, RequestID: "global-chat-create"})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := service.Create(context.Background(), owner, globalchat.CreateRequest{Kind: "direct", MemberUserIDs: []string{memberID}, RequestID: "global-chat-repeat"})
	if err != nil || repeated.ID != created.ID {
		t.Fatalf("repeated=%#v error=%v", repeated, err)
	}
	if _, err = service.Get(context.Background(), globalchat.Actor{UserID: outsiderID}, created.ID, 50, ""); !errors.Is(err, globalchat.ErrNotFound) {
		t.Fatalf("outsider read error=%v", err)
	}
	if conversation, getErr := service.Get(context.Background(), globalchat.Actor{UserID: technicalID, Privileged: true}, created.ID, 50, ""); getErr != nil || !conversation.CanSend {
		t.Fatalf("technical conversation=%#v error=%v", conversation, getErr)
	}
	candidates, err := service.SearchCandidates(context.Background(), owner, "chat.mem")
	if err != nil || len(candidates) != 1 || candidates[0].UserID != memberID {
		t.Fatalf("global chat candidates=%#v error=%v", candidates, err)
	}
	groupName := "Проектная команда"
	group, err := service.Create(context.Background(), owner, globalchat.CreateRequest{Kind: "group", Name: groupName, MemberUserIDs: []string{memberID, outsiderID}, RequestID: "global-group-create"})
	if err != nil || group.Name == nil || *group.Name != groupName || group.DisplayName != groupName || len(group.Members) != 3 {
		t.Fatalf("group=%#v error=%v", group, err)
	}
	page, err := service.List(context.Background(), globalchat.ListRequest{Actor: owner, PageSize: 10})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("global chat page=%#v error=%v", page, err)
	}
	firstPage, err := service.List(context.Background(), globalchat.ListRequest{Actor: owner, PageSize: 1})
	if err != nil || len(firstPage.Items) != 1 || !firstPage.HasMore || firstPage.NextCursor == nil {
		t.Fatalf("first global chat page=%#v error=%v", firstPage, err)
	}
	secondPage, err := service.List(context.Background(), globalchat.ListRequest{Actor: owner, PageSize: 1, Cursor: *firstPage.NextCursor})
	if err != nil || len(secondPage.Items) != 1 || secondPage.HasMore || secondPage.Items[0].ID == firstPage.Items[0].ID {
		t.Fatalf("second global chat page=%#v error=%v", secondPage, err)
	}
	emptyUserID := seedCollaborationUser(t, pool, "chat.empty", "Без чатов", nil)
	emptyPage, err := service.List(context.Background(), globalchat.ListRequest{Actor: globalchat.Actor{UserID: emptyUserID}, PageSize: 10})
	if err != nil || len(emptyPage.Items) != 0 {
		t.Fatalf("empty global chat page=%#v error=%v", emptyPage, err)
	}
	escapedCandidates, err := service.SearchCandidates(context.Background(), owner, `chat.%_\\`)
	if err != nil || len(escapedCandidates) != 0 {
		t.Fatalf("escaped candidate search=%#v error=%v", escapedCandidates, err)
	}
	missingUserID := "00000000-0000-0000-0000-000000000099"
	if _, err = service.Create(context.Background(), owner, globalchat.CreateRequest{Kind: "direct", MemberUserIDs: []string{missingUserID}, RequestID: "global-chat-missing-member"}); !errors.Is(err, globalchat.ErrNotFound) {
		t.Fatalf("missing global chat member error=%v", err)
	}

	bindTelegramRecipient(t, pool, memberID, 81002)
	current = now.Add(time.Minute)
	messageID := "00000000-0000-0000-0000-000000000401"
	message, err := service.Send(context.Background(), owner, created.ID, globalchat.SendRequest{Body: "Добрый день", ClientMessageID: messageID, RequestID: "global-chat-message"})
	if err != nil || message.Body != "Добрый день" {
		t.Fatalf("message=%#v error=%v", message, err)
	}
	repeatedMessage, err := service.Send(context.Background(), owner, created.ID, globalchat.SendRequest{Body: "Добрый день", ClientMessageID: messageID, RequestID: "global-chat-message-repeat"})
	if err != nil || repeatedMessage.ID != message.ID {
		t.Fatalf("repeated message=%#v error=%v", repeatedMessage, err)
	}
	current = now.Add(2 * time.Minute)
	secondMessage, err := service.Send(context.Background(), owner, created.ID, globalchat.SendRequest{Body: "Второе сообщение", ClientMessageID: "00000000-0000-0000-0000-000000000403", RequestID: "global-chat-second-message"})
	if err != nil || secondMessage.Body != "Второе сообщение" {
		t.Fatalf("second message=%#v error=%v", secondMessage, err)
	}
	var recipient, messageType string
	if err = pool.QueryRow(context.Background(), `SELECT recipient_user_id::text,message_type FROM notification_outbox WHERE message_type='global_chat.message_created'`).Scan(&recipient, &messageType); err != nil || recipient != memberID || messageType == "" {
		t.Fatalf("recipient=%q messageType=%q error=%v", recipient, messageType, err)
	}
	conversation, err := service.Get(context.Background(), globalchat.Actor{UserID: memberID}, created.ID, 1, "")
	if err != nil || len(conversation.Messages) != 1 || conversation.Messages[0].ID != secondMessage.ID || !conversation.HasMore || conversation.NextCursor == nil || !conversation.CanSend || conversation.Chat.UnreadCount != 2 {
		t.Fatalf("member conversation=%#v error=%v", conversation, err)
	}
	olderConversation, err := service.Get(context.Background(), globalchat.Actor{UserID: memberID}, created.ID, 1, *conversation.NextCursor)
	if err != nil || len(olderConversation.Messages) != 1 || olderConversation.Messages[0].ID != message.ID || olderConversation.HasMore {
		t.Fatalf("older conversation=%#v error=%v", olderConversation, err)
	}
	if err = service.MarkRead(context.Background(), globalchat.Actor{UserID: memberID}, created.ID); err != nil {
		t.Fatalf("mark global chat read: %v", err)
	}
	if err = service.MarkRead(context.Background(), globalchat.Actor{UserID: technicalID, Privileged: true}, missingUserID); err != nil {
		t.Fatalf("privileged mark-read of absent chat: %v", err)
	}
	if err = service.MarkRead(context.Background(), globalchat.Actor{UserID: outsiderID}, created.ID); !errors.Is(err, globalchat.ErrNotFound) {
		t.Fatalf("outsider mark-read error=%v", err)
	}
	if _, err = service.Send(context.Background(), owner, created.ID, globalchat.SendRequest{Body: "Другой текст", ClientMessageID: messageID, RequestID: "global-chat-message-conflict"}); !errors.Is(err, globalchat.ErrConflict) {
		t.Fatalf("conflicting repeated message error=%v", err)
	}
	if _, err = service.Send(context.Background(), globalchat.Actor{UserID: outsiderID, Login: "chat.outsider", FirstName: "Посторонний"}, created.ID, globalchat.SendRequest{Body: "Нет доступа", ClientMessageID: "00000000-0000-0000-0000-000000000402", RequestID: "global-chat-forbidden"}); !errors.Is(err, globalchat.ErrNotFound) {
		t.Fatalf("outsider send error=%v", err)
	}
}

func TestTechnicalReportsAggregateAndNotifyOnlyTechnicalAdministrator(t *testing.T) {
	pool := newMigrationSchemaPool(t, "technical_reports_contract")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	actorID := seedCollaborationUser(t, pool, "feedback.author", "Автор", nil)
	technicalID := seedCollaborationUser(t, pool, "technical.recipient", "Технический", stringPointer("technical_admin"))
	superID := seedCollaborationUser(t, pool, "business.admin", "Администратор", stringPointer("super_admin"))
	bindTelegramRecipient(t, pool, technicalID, 82001)
	bindTelegramRecipient(t, pool, superID, 82002)
	now := time.Date(2026, time.September, 28, 13, 0, 0, 0, time.UTC)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{46}, 32), 1)
	service, err := technicalsupport.NewService(store, cipher, bytes.Repeat([]byte{47}, 32), 1, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ReportFrontend(context.Background(), technicalsupport.Actor{UserID: actorID, Login: "feedback.author", FirstName: "Автор"}, technicalsupport.FrontendErrorRequest{Message: "Ошибка интерфейса", Path: "/account/chats", Fingerprint: "frontend-error-0001", RequestID: "frontend-request-1"}); err != nil {
		t.Fatal(err)
	}
	if err = service.ReportFrontend(context.Background(), technicalsupport.Actor{UserID: actorID, Login: "feedback.author", FirstName: "Автор"}, technicalsupport.FrontendErrorRequest{Message: "Ошибка интерфейса", Path: "/account/chats", Fingerprint: "frontend-error-0001", RequestID: "frontend-request-2"}); err != nil {
		t.Fatal(err)
	}
	if err = service.CreateFeedback(context.Background(), technicalsupport.Actor{UserID: actorID, Login: "feedback.author", FirstName: "Автор"}, technicalsupport.FeedbackRequest{Category: "suggestion", Message: "Добавьте экспорт истории", Path: "/account/chats", RequestID: "feedback-request-1"}); err != nil {
		t.Fatal(err)
	}

	var automaticRows, automaticOccurrences, feedbackRows int
	if err = pool.QueryRow(context.Background(), `SELECT count(*),COALESCE(sum(occurrence_count),0) FROM technical_reports WHERE kind='frontend_error'`).Scan(&automaticRows, &automaticOccurrences); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM technical_reports WHERE kind='feedback'`).Scan(&feedbackRows); err != nil {
		t.Fatal(err)
	}
	if automaticRows != 1 || automaticOccurrences != 2 || feedbackRows != 1 {
		t.Fatalf("automatic rows=%d occurrences=%d feedback=%d", automaticRows, automaticOccurrences, feedbackRows)
	}
	rows, err := pool.Query(context.Background(), `SELECT DISTINCT recipient_user_id::text FROM notification_outbox WHERE message_type LIKE 'technical.%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var recipients []string
	for rows.Next() {
		var recipient string
		if err = rows.Scan(&recipient); err != nil {
			t.Fatal(err)
		}
		recipients = append(recipients, recipient)
	}
	if len(recipients) != 1 || recipients[0] != technicalID {
		t.Fatalf("technical notification recipients=%v", recipients)
	}
}

func accountServiceForCollaborationTest(t *testing.T, pool *pgxpool.Pool) (*account.Service, *Postgres) {
	t.Helper()
	store := NewPostgres(pool)
	tokens, _ := account.NewTokenManager(bytes.Repeat([]byte{41}, 32), 1)
	cipher, _ := account.NewPayloadCipher(bytes.Repeat([]byte{42}, 32), 1)
	service, err := account.NewService(store, tokens, cipher, "https://designer-svetlana.ru", "from@example.test", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return service, store
}

func seedCollaborationUser(t *testing.T, pool *pgxpool.Pool, login, firstName string, globalRole *string) string {
	t.Helper()
	var userID string
	err := pool.QueryRow(context.Background(), `
		WITH inserted AS (
		  INSERT INTO users (login,login_normalized,email,email_normalized,status,global_role,email_verified_at)
		  VALUES ($1,$1,$1 || '@example.test',$1 || '@example.test','active',$3,NOW()) RETURNING id
		), profile AS (
		  INSERT INTO profiles (user_id,professional_role_id,first_name)
		  SELECT inserted.id,role.id,$2 FROM inserted CROSS JOIN professional_roles role WHERE role.code='customer'
		)
		SELECT id::text FROM inserted
	`, login, firstName, globalRole).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user %s: %v", login, err)
	}
	return userID
}

func bindTelegramRecipient(t *testing.T, pool *pgxpool.Pool, userID string, chatID int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO telegram_account_bindings (user_id,chat_id,chat_username,verified_at)
		VALUES ($1,$2,$3,$4)
	`, userID, chatID, fmt.Sprintf("user_%d", chatID), now); err != nil {
		t.Fatalf("bind Telegram account: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO telegram_subscribers (chat_id,username,user_id,verified_at,chat_type,subscribed_at,updated_at)
		VALUES ($2,$3,$1,$4,'private',$4,$4)
	`, userID, chatID, fmt.Sprintf("user_%d", chatID), now); err != nil {
		t.Fatalf("subscribe Telegram recipient: %v", err)
	}
}

func stringPointer(value string) *string { return &value }
