package project

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type collaborationTestStore struct {
	*fakeStore
	created         CreateChatCommand
	deleted         DeleteChatCommand
	member          ChatMemberCommand
	updated         UpdateChatCommand
	markedRead      MarkChatReadCommand
	markedNoticeID  string
	markedAll       bool
	uploaded        UploadProjectDocumentCommand
	deletedDocument DeleteProjectDocumentCommand
}

func (store *collaborationTestStore) ListProjectChats(context.Context, Actor, string) (ChatList, error) {
	return ChatList{}, nil
}
func (store *collaborationTestStore) CreateProjectChat(_ context.Context, command CreateChatCommand) (ChatSummary, error) {
	store.created = command
	return ChatSummary{ID: "00000000-0000-0000-0000-000000000010", Name: command.Name}, nil
}
func (store *collaborationTestStore) UpdateProjectChat(_ context.Context, command UpdateChatCommand) (ChatSummary, error) {
	store.updated = command
	return ChatSummary{ID: command.ChatID, Name: command.Name}, nil
}
func (store *collaborationTestStore) DeleteProjectChat(_ context.Context, command DeleteChatCommand) error {
	store.deleted = command
	return nil
}

func (store *collaborationTestStore) MarkProjectChatRead(_ context.Context, command MarkChatReadCommand) error {
	store.markedRead = command
	return nil
}
func (store *collaborationTestStore) ListProjectChatMembers(context.Context, Actor, string, string) (ChatMemberList, error) {
	return ChatMemberList{}, nil
}
func (store *collaborationTestStore) AddProjectChatMember(_ context.Context, command ChatMemberCommand) (ChatMember, error) {
	store.member = command
	return ChatMember{}, nil
}
func (store *collaborationTestStore) RemoveProjectChatMember(_ context.Context, command ChatMemberCommand) error {
	store.member = command
	return nil
}
func (store *collaborationTestStore) ListProjectDocuments(context.Context, Actor, string) (ProjectDocumentList, error) {
	return ProjectDocumentList{}, nil
}
func (store *collaborationTestStore) UploadProjectDocument(_ context.Context, command UploadProjectDocumentCommand) (ProjectDocument, error) {
	store.uploaded = command
	return ProjectDocument{Name: command.Name, SizeBytes: int64(len(command.Content))}, nil
}
func (store *collaborationTestStore) DownloadProjectDocument(context.Context, Actor, string, string) (ProjectDocumentContent, error) {
	return ProjectDocumentContent{}, nil
}
func (store *collaborationTestStore) DeleteProjectDocument(_ context.Context, command DeleteProjectDocumentCommand) error {
	store.deletedDocument = command
	return nil
}
func (store *collaborationTestStore) ListAccountNotifications(context.Context, Actor, int) (AccountNotificationList, error) {
	return AccountNotificationList{UnreadCount: 1}, nil
}
func (store *collaborationTestStore) MarkAccountNotificationRead(_ context.Context, _ Actor, notificationID string, _ time.Time) error {
	store.markedNoticeID = notificationID
	return nil
}
func (store *collaborationTestStore) MarkAllAccountNotificationsRead(context.Context, Actor, time.Time) error {
	store.markedAll = true
	return nil
}

func TestCreateChatNormalizesNameAndMembers(t *testing.T) {
	store := &collaborationTestStore{fakeStore: &fakeStore{}}
	service, err := NewService(store, bytes.Repeat([]byte("k"), 32), func() time.Time { return time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	memberID := "00000000-0000-0000-0000-000000000003"
	value, err := service.CreateChat(context.Background(), actor, projectID, CreateChatRequest{Name: "  Согласование кухни  ", MemberUserIDs: []string{memberID, memberID}, RequestID: "request-123"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Name != "Согласование кухни" || store.created.Name != "Согласование кухни" || len(store.created.MemberUserIDs) != 1 {
		t.Fatalf("unexpected normalized command: %#v", store.created)
	}
}

func TestUploadDocumentValidatesAndReadsExactBody(t *testing.T) {
	store := &collaborationTestStore{fakeStore: &fakeStore{}}
	service, err := NewService(store, bytes.Repeat([]byte("k"), 32), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	body := []byte("pdf")
	value, err := service.UploadDocument(context.Background(), actor, projectID, " plan.pdf ", "application/pdf", "request-123", bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if value.SizeBytes != 3 || store.uploaded.Name != "plan.pdf" || !bytes.Equal(store.uploaded.Content, body) {
		t.Fatalf("unexpected upload: %#v", store.uploaded)
	}
	if _, err = service.UploadDocument(context.Background(), actor, projectID, "empty.pdf", "application/pdf", "request-123", bytes.NewReader(nil), 0); err != ErrInvalidInput {
		t.Fatalf("empty upload error = %v", err)
	}
}

func TestCollaborationLifecycleCreatesUnifiedNotifications(t *testing.T) {
	store := &collaborationTestStore{fakeStore: &fakeStore{}}
	service, err := NewService(store, bytes.Repeat([]byte("k"), 32), func() time.Time { return time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	cipher := &fakeCipher{}
	if err = service.ConfigureNotifications(cipher); err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	chatID := "00000000-0000-0000-0000-000000000003"
	memberID := "00000000-0000-0000-0000-000000000004"
	documentID := "00000000-0000-0000-0000-000000000005"

	if _, err = service.CreateChat(context.Background(), actor, projectID, CreateChatRequest{Name: "Рабочий чат", MemberUserIDs: []string{memberID}, RequestID: "request-create-chat"}); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.created.Notification, cipher, "project.chat.created", "Рабочий чат")
	if _, err = service.AddChatMember(context.Background(), actor, projectID, chatID, memberID, "request-add-member"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.member.Notification, cipher, "project.chat.member_added", "добавлен участник")
	if err = service.DeleteChat(context.Background(), actor, projectID, chatID, "request-delete-chat", 1); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.deleted.Notification, cipher, "project.chat.deleted", "удалён")

	body := []byte("pdf")
	if _, err = service.UploadDocument(context.Background(), actor, projectID, "plan.pdf", "application/pdf", "request-upload-document", bytes.NewReader(body), int64(len(body))); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.uploaded.Notification, cipher, "project.document.uploaded", "plan.pdf")
	if err = service.DeleteDocument(context.Background(), actor, projectID, documentID, "request-delete-document"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotification(t, store.deletedDocument.Notification, cipher, "project.document.deleted", "Документ удалён")
}

func TestCollaborationReadAndManagementOperations(t *testing.T) {
	store := &collaborationTestStore{fakeStore: &fakeStore{}}
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	service, err := NewService(store, bytes.Repeat([]byte("k"), 32), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	chatID := "00000000-0000-0000-0000-000000000003"
	memberID := "00000000-0000-0000-0000-000000000004"
	messageID := "00000000-0000-0000-0000-000000000005"
	documentID := "00000000-0000-0000-0000-000000000006"
	notificationID := "00000000-0000-0000-0000-000000000007"

	if _, err = service.ListChats(context.Background(), actor, projectID); err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateChat(context.Background(), actor, projectID, chatID, "  Новый чат  ", "request-update-chat", 2)
	if err != nil || updated.Name != "Новый чат" || store.updated.Now != now {
		t.Fatalf("updated=%#v command=%#v error=%v", updated, store.updated, err)
	}
	if _, err = service.ListChatMembers(context.Background(), actor, projectID, chatID); err != nil {
		t.Fatal(err)
	}
	if err = service.RemoveChatMember(context.Background(), actor, projectID, chatID, memberID, "request-remove-member"); err != nil {
		t.Fatal(err)
	}
	if err = service.MarkChatRead(context.Background(), actor, projectID, chatID, messageID); err != nil || store.markedRead.MessageID != messageID {
		t.Fatalf("mark read=%#v error=%v", store.markedRead, err)
	}
	if list, listErr := service.ListNotifications(context.Background(), actor, 20); listErr != nil || list.UnreadCount != 1 {
		t.Fatalf("notifications=%#v error=%v", list, listErr)
	}
	if err = service.MarkNotificationRead(context.Background(), actor, notificationID); err != nil || store.markedNoticeID != notificationID {
		t.Fatalf("notification id=%q error=%v", store.markedNoticeID, err)
	}
	if err = service.MarkAllNotificationsRead(context.Background(), actor); err != nil || !store.markedAll {
		t.Fatalf("mark all=%v error=%v", store.markedAll, err)
	}
	if _, err = service.ListDocuments(context.Background(), actor, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.DownloadDocument(context.Background(), actor, projectID, documentID); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationRejectsInvalidCommandsAndMissingCapabilities(t *testing.T) {
	store := &collaborationTestStore{fakeStore: &fakeStore{}}
	service, err := NewService(store, bytes.Repeat([]byte("k"), 32), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	chatID := "00000000-0000-0000-0000-000000000003"
	memberID := "00000000-0000-0000-0000-000000000004"
	documentID := "00000000-0000-0000-0000-000000000005"

	invalid := []error{}
	_, valueErr := service.ListChats(context.Background(), Actor{}, projectID)
	invalid = append(invalid, valueErr)
	_, valueErr = service.CreateChat(context.Background(), actor, projectID, CreateChatRequest{Name: "x", RequestID: "short"})
	invalid = append(invalid, valueErr)
	_, valueErr = service.CreateChat(context.Background(), actor, projectID, CreateChatRequest{Name: "Чат", MemberUserIDs: []string{"bad"}, RequestID: "request-valid"})
	invalid = append(invalid, valueErr)
	_, valueErr = service.UpdateChat(context.Background(), actor, projectID, "bad", "Чат", "request-valid", 1)
	invalid = append(invalid, valueErr)
	invalid = append(invalid, service.DeleteChat(context.Background(), actor, projectID, chatID, "short", 1))
	_, valueErr = service.ListChatMembers(context.Background(), actor, projectID, "bad")
	invalid = append(invalid, valueErr)
	_, valueErr = service.AddChatMember(context.Background(), actor, projectID, chatID, "bad", "request-valid")
	invalid = append(invalid, valueErr)
	invalid = append(invalid, service.RemoveChatMember(context.Background(), actor, projectID, chatID, memberID, "short"))
	invalid = append(invalid, service.MarkChatRead(context.Background(), actor, projectID, chatID, "bad"))
	_, valueErr = service.ListNotifications(context.Background(), actor, 0)
	invalid = append(invalid, valueErr)
	invalid = append(invalid, service.MarkNotificationRead(context.Background(), actor, "bad"))
	invalid = append(invalid, service.MarkAllNotificationsRead(context.Background(), Actor{}))
	_, valueErr = service.ListDocuments(context.Background(), Actor{}, projectID)
	invalid = append(invalid, valueErr)
	_, valueErr = service.UploadDocument(context.Background(), actor, projectID, "", "x", "short", bytes.NewReader(nil), 0)
	invalid = append(invalid, valueErr)
	_, valueErr = service.UploadDocument(context.Background(), actor, projectID, "plan.pdf", "application/pdf", "request-valid", bytes.NewReader([]byte("pdf")), 2)
	invalid = append(invalid, valueErr)
	_, valueErr = service.UploadDocument(context.Background(), actor, projectID, "plan.pdf", "application/pdf", "request-valid", errorReader{}, 3)
	invalid = append(invalid, valueErr)
	_, valueErr = service.DownloadDocument(context.Background(), actor, projectID, "bad")
	invalid = append(invalid, valueErr)
	invalid = append(invalid, service.DeleteDocument(context.Background(), actor, projectID, documentID, "short"))
	for index, got := range invalid {
		if !errors.Is(got, ErrInvalidInput) {
			t.Fatalf("invalid case %d error=%v", index, got)
		}
	}

	limited, err := NewService(&fakeStore{}, bytes.Repeat([]byte("k"), 32), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = limited.ListChats(context.Background(), actor, projectID); err == nil {
		t.Fatal("expected missing collaboration store error")
	}
	if _, err = limited.CreateChat(context.Background(), actor, projectID, CreateChatRequest{Name: "Чат", RequestID: "request-valid"}); err == nil {
		t.Fatal("expected missing collaboration store error for chat creation")
	}
	if _, err = limited.UpdateChat(context.Background(), actor, projectID, chatID, "Чат", "request-valid", 1); err == nil {
		t.Fatal("expected missing collaboration store error for chat update")
	}
	if err = limited.DeleteChat(context.Background(), actor, projectID, chatID, "request-valid", 1); err == nil {
		t.Fatal("expected missing chat lifecycle store error")
	}
	if _, err = limited.ListChatMembers(context.Background(), actor, projectID, chatID); err == nil {
		t.Fatal("expected missing collaboration store error for chat members")
	}
	if _, err = limited.AddChatMember(context.Background(), actor, projectID, chatID, memberID, "request-valid"); err == nil {
		t.Fatal("expected missing collaboration store error for member addition")
	}
	if err = limited.RemoveChatMember(context.Background(), actor, projectID, chatID, memberID, "request-valid"); err == nil {
		t.Fatal("expected missing collaboration store error for member removal")
	}
	if err = limited.MarkChatRead(context.Background(), actor, projectID, chatID, documentID); err == nil {
		t.Fatal("expected missing chat lifecycle store error for read marker")
	}
	if _, err = limited.ListNotifications(context.Background(), actor, 20); err == nil {
		t.Fatal("expected missing notification store error for list")
	}
	if err = limited.MarkNotificationRead(context.Background(), actor, documentID); err == nil {
		t.Fatal("expected missing notification store error")
	}
	if err = limited.MarkAllNotificationsRead(context.Background(), actor); err == nil {
		t.Fatal("expected missing notification store error for mark all")
	}
	if _, err = limited.ListDocuments(context.Background(), actor, projectID); err == nil {
		t.Fatal("expected missing collaboration store error for document list")
	}
	if _, err = limited.UploadDocument(context.Background(), actor, projectID, "plan.pdf", "application/pdf", "request-valid", bytes.NewReader([]byte("pdf")), 3); err == nil {
		t.Fatal("expected missing collaboration store error for document upload")
	}
	if _, err = limited.DownloadDocument(context.Background(), actor, projectID, documentID); err == nil {
		t.Fatal("expected missing collaboration store error for document download")
	}
	if err = limited.DeleteDocument(context.Background(), actor, projectID, documentID, "request-valid"); err == nil {
		t.Fatal("expected missing collaboration store error for document delete")
	}
}

func TestCollaborationPropagatesNotificationEncryptionFailures(t *testing.T) {
	store := &collaborationTestStore{fakeStore: &fakeStore{}}
	service, err := NewService(store, bytes.Repeat([]byte("k"), 32), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	cipherErr := errors.New("notification encryption failed")
	if err = service.ConfigureNotifications(&fakeCipher{err: cipherErr}); err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000002"
	chatID := "00000000-0000-0000-0000-000000000003"
	memberID := "00000000-0000-0000-0000-000000000004"
	documentID := "00000000-0000-0000-0000-000000000005"
	checks := []func() error{
		func() error {
			_, callErr := service.CreateChat(context.Background(), actor, projectID, CreateChatRequest{Name: "Рабочий чат", RequestID: "request-create"})
			return callErr
		},
		func() error {
			_, callErr := service.UpdateChat(context.Background(), actor, projectID, chatID, "Рабочий чат", "request-update", 1)
			return callErr
		},
		func() error {
			return service.DeleteChat(context.Background(), actor, projectID, chatID, "request-delete", 1)
		},
		func() error {
			_, callErr := service.AddChatMember(context.Background(), actor, projectID, chatID, memberID, "request-add-member")
			return callErr
		},
		func() error {
			return service.RemoveChatMember(context.Background(), actor, projectID, chatID, memberID, "request-remove-member")
		},
		func() error {
			_, callErr := service.UploadDocument(context.Background(), actor, projectID, "plan.pdf", "application/pdf", "request-upload", bytes.NewReader([]byte("pdf")), 3)
			return callErr
		},
		func() error {
			return service.DeleteDocument(context.Background(), actor, projectID, documentID, "request-delete-document")
		},
	}
	for index, check := range checks {
		if callErr := check(); !errors.Is(callErr, cipherErr) {
			t.Fatalf("case %d error=%v", index, callErr)
		}
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
