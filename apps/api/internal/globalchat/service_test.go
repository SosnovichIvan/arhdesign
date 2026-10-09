package globalchat

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	actorID   = "00000000-0000-0000-0000-000000000001"
	memberID  = "00000000-0000-0000-0000-000000000002"
	chatID    = "00000000-0000-0000-0000-000000000003"
	messageID = "00000000-0000-0000-0000-000000000004"
)

type fakeStore struct {
	listQuery         ListQuery
	items             []Summary
	candidates        []Candidate
	createCommand     CreateCommand
	created           Summary
	conversation      Conversation
	conversationQuery ConversationQuery
	sendCommand       SendCommand
	message           Message
	readActor         Actor
	readChatID        string
	err               error
}

func (store *fakeStore) ListGlobalChats(_ context.Context, query ListQuery) ([]Summary, error) {
	store.listQuery = query
	return store.items, store.err
}
func (store *fakeStore) SearchGlobalChatCandidates(_ context.Context, userID, query string) ([]Candidate, error) {
	store.listQuery.Actor.UserID, store.listQuery.BeforeID = userID, &query
	return store.candidates, store.err
}
func (store *fakeStore) CreateGlobalChat(_ context.Context, command CreateCommand) (Summary, error) {
	store.createCommand = command
	return store.created, store.err
}
func (store *fakeStore) GetGlobalChat(_ context.Context, query ConversationQuery) (Conversation, error) {
	store.conversationQuery = query
	return store.conversation, store.err
}
func (store *fakeStore) CreateGlobalChatMessage(_ context.Context, command SendCommand) (Message, error) {
	store.sendCommand = command
	return store.message, store.err
}
func (store *fakeStore) MarkGlobalChatRead(_ context.Context, actor Actor, id string, _ time.Time) error {
	store.readActor, store.readChatID = actor, id
	return store.err
}

type fakeCipher struct {
	payload, messageType []byte
	err                  error
}

func (cipher *fakeCipher) Encrypt(payload []byte, messageType string) ([]byte, int, error) {
	cipher.payload, cipher.messageType = append([]byte(nil), payload...), []byte(messageType)
	if cipher.err != nil {
		return nil, 0, cipher.err
	}
	return []byte("ciphertext"), 2, nil
}

func newService(t *testing.T, store *fakeStore) (*Service, *fakeCipher) {
	t.Helper()
	cipher := &fakeCipher{}
	service, err := NewService(store, []byte("global-chat-test-cursor-key-32----"), cipher, func() time.Time { return time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	return service, cipher
}

func actor() Actor { return Actor{UserID: actorID, Login: "owner", FirstName: "Анна"} }

func TestListCreatesAndValidatesSignedCursor(t *testing.T) {
	store := &fakeStore{items: []Summary{{ID: chatID, LastActivityAt: time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)}, {ID: messageID, LastActivityAt: time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)}}}
	service, _ := newService(t, store)
	page, err := service.List(context.Background(), ListRequest{Actor: actor(), PageSize: 1})
	if err != nil || !page.HasMore || page.NextCursor == nil || len(page.Items) != 1 || store.listQuery.PageSize != 2 {
		t.Fatalf("page=%#v query=%#v err=%v", page, store.listQuery, err)
	}
	store.items = store.items[:1]
	if _, err = service.List(context.Background(), ListRequest{Actor: actor(), PageSize: 1, Cursor: *page.NextCursor}); err != nil || store.listQuery.BeforeID == nil || *store.listQuery.BeforeID != chatID {
		t.Fatalf("cursor query=%#v err=%v", store.listQuery, err)
	}
	for _, value := range []string{"bad", *page.NextCursor + "x"} {
		if _, err = service.List(context.Background(), ListRequest{Actor: actor(), Cursor: value}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("cursor=%q err=%v", value, err)
		}
	}
}

func TestCreateDirectAndGroupValidation(t *testing.T) {
	store := &fakeStore{created: Summary{ID: chatID}}
	service, _ := newService(t, store)
	if _, err := service.Create(context.Background(), actor(), CreateRequest{Kind: "direct", MemberUserIDs: []string{memberID}, RequestID: "request-direct"}); err != nil || store.createCommand.Kind != "direct" {
		t.Fatalf("command=%#v err=%v", store.createCommand, err)
	}
	if _, err := service.Create(context.Background(), actor(), CreateRequest{Kind: "group", Name: "  Команда  ", MemberUserIDs: []string{memberID}, RequestID: "request-group"}); err != nil || store.createCommand.Name != "Команда" {
		t.Fatalf("command=%#v err=%v", store.createCommand, err)
	}
	invalid := []CreateRequest{
		{Kind: "direct", Name: "Имя", MemberUserIDs: []string{memberID}, RequestID: "request-invalid"},
		{Kind: "direct", MemberUserIDs: []string{memberID, messageID}, RequestID: "request-invalid"},
		{Kind: "group", Name: "x", MemberUserIDs: []string{memberID}, RequestID: "request-invalid"},
		{Kind: "group", Name: "Команда", MemberUserIDs: []string{actorID}, RequestID: "request-invalid"},
		{Kind: "group", Name: "Команда", MemberUserIDs: []string{"bad"}, RequestID: "request-invalid"},
	}
	for _, request := range invalid {
		if _, err := service.Create(context.Background(), actor(), request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("request=%#v err=%v", request, err)
		}
	}
}

func TestSearchGetSendAndMarkRead(t *testing.T) {
	store := &fakeStore{candidates: []Candidate{{UserID: memberID}}, conversation: Conversation{CurrentUserID: actorID}, message: Message{ID: messageID}}
	service, cipher := newService(t, store)
	if items, err := service.SearchCandidates(context.Background(), actor(), "  MEM  "); err != nil || len(items) != 1 || store.listQuery.BeforeID == nil || *store.listQuery.BeforeID != "mem" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	if _, err := service.SearchCandidates(context.Background(), actor(), "x"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("search err=%v", err)
	}
	if _, err := service.Get(context.Background(), actor(), chatID, 50, messageID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), actor(), "bad", 50, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("get err=%v", err)
	}
	message, err := service.Send(context.Background(), actor(), chatID, SendRequest{Body: "  Привет  ", ClientMessageID: messageID, RequestID: "request-message"})
	if err != nil || message.ID != messageID || store.sendCommand.Body != "Привет" || store.sendCommand.Notification == nil || string(cipher.messageType) != "global_chat.message_created" {
		t.Fatalf("message=%#v command=%#v err=%v", message, store.sendCommand, err)
	}
	if err = service.MarkRead(context.Background(), actor(), chatID); err != nil || store.readChatID != chatID {
		t.Fatalf("read=%q err=%v", store.readChatID, err)
	}
	if _, err = service.Send(context.Background(), actor(), chatID, SendRequest{Body: "", ClientMessageID: messageID, RequestID: "request-message"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("send err=%v", err)
	}
}

func TestNewServiceRejectsDependencies(t *testing.T) {
	if _, err := NewService(nil, []byte("short"), nil, nil); err == nil {
		t.Fatal("missing dependencies accepted")
	}
	service, err := NewService(&fakeStore{}, []byte("global-chat-test-cursor-key-32----"), &fakeCipher{}, nil)
	if err != nil || service.now == nil {
		t.Fatalf("default clock err=%v", err)
	}
}

func TestValidationAndDependencyErrors(t *testing.T) {
	store := &fakeStore{}
	service, cipher := newService(t, store)
	invalidLists := []ListRequest{{Actor: Actor{}}, {Actor: actor(), PageSize: -1}, {Actor: actor(), PageSize: 51}}
	for _, request := range invalidLists {
		if _, err := service.List(context.Background(), request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("list %#v err=%v", request, err)
		}
	}
	store.err = errors.New("store unavailable")
	if _, err := service.List(context.Background(), ListRequest{Actor: actor()}); !errors.Is(err, store.err) {
		t.Fatalf("list store err=%v", err)
	}
	store.err = nil
	if _, err := service.Get(context.Background(), actor(), chatID, 101, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("page err=%v", err)
	}
	if _, err := service.Get(context.Background(), actor(), chatID, 10, "bad"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("before err=%v", err)
	}
	if _, err := service.Get(context.Background(), actor(), chatID, 0, messageID); err != nil || store.conversationQuery.PageSize != 50 || store.conversationQuery.BeforeID == nil {
		t.Fatalf("query=%#v err=%v", store.conversationQuery, err)
	}
	if err := service.MarkRead(context.Background(), Actor{}, chatID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("read actor err=%v", err)
	}
	if err := service.MarkRead(context.Background(), actor(), "bad"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("read chat err=%v", err)
	}
	cipher.err = errors.New("encryption unavailable")
	if _, err := service.Send(context.Background(), actor(), chatID, SendRequest{Body: strings.Repeat("я", 1001), ClientMessageID: messageID, RequestID: "request-message"}); !errors.Is(err, cipher.err) {
		t.Fatalf("cipher err=%v", err)
	}
}

func TestCursorRejectsMalformedPayloads(t *testing.T) {
	service, _ := newService(t, &fakeStore{})
	values := []string{
		"%%%.valid",
		base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".%%%",
		base64.RawURLEncoding.EncodeToString([]byte("{}")) + "." + base64.RawURLEncoding.EncodeToString([]byte("wrong")),
	}
	for _, value := range values {
		if _, err := service.decodeCursor(value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("cursor=%q err=%v", value, err)
		}
	}
	payload := []byte(`{"at":"0001-01-01T00:00:00Z","id":"bad"}`)
	mac := hmac.New(sha256.New, service.cursorKey)
	_, _ = mac.Write(payload)
	value := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := service.decodeCursor(value); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("payload err=%v", err)
	}
}
