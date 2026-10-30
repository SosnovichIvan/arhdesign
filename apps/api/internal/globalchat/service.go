package globalchat

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid global chat input")
	ErrNotFound     = errors.New("global chat not found")
	ErrConflict     = errors.New("global chat conflict")
)

type Actor struct {
	UserID     string
	Privileged bool
	Login      string
	FirstName  string
}

type Candidate struct {
	UserID, Login, Email, FirstName string
	LastName                        *string
}

type Member struct {
	Candidate
	JoinedAt time.Time
}

type Author struct {
	UserID, Login, FirstName string
	LastName                 *string
}

type Message struct {
	ID, ChatID, Body    string
	Author              Author
	CreatedAt           time.Time
	EditedAt, DeletedAt *time.Time
	Version             int64
}

type Summary struct {
	ID, Kind, DisplayName string
	Name                  *string
	Members               []Member
	LastMessage           *Message
	UnreadCount           int
	LastActivityAt        time.Time
	Version               int64
}

type Page struct {
	Items      []Summary
	NextCursor *string
	HasMore    bool
}

type Conversation struct {
	Chat          Summary
	CurrentUserID string
	Messages      []Message
	NextCursor    *string
	HasMore       bool
	CanSend       bool
}

type ListRequest struct {
	Actor
	PageSize int
	Cursor   string
}

type ListQuery struct {
	Actor
	PageSize             int
	BeforeLastActivityAt *time.Time
	BeforeID             *string
}

type CreateRequest struct {
	Kind, Name, RequestID string
	MemberUserIDs         []string
}

type CreateCommand struct {
	Actor
	Kind, Name, RequestID string
	MemberUserIDs         []string
	Now                   time.Time
}

type ConversationQuery struct {
	Actor
	ChatID   string
	PageSize int
	BeforeID *string
}

type SendRequest struct {
	Body, ClientMessageID, RequestID string
}

type Notification struct {
	MessageType       string
	PayloadCiphertext []byte
	PayloadKeyVersion int
}

type SendCommand struct {
	Actor
	ChatID, Body, ClientMessageID, RequestID string
	Now                                      time.Time
	Notification                             *Notification
}

type Store interface {
	ListGlobalChats(context.Context, ListQuery) ([]Summary, error)
	SearchGlobalChatCandidates(context.Context, string, string) ([]Candidate, error)
	CreateGlobalChat(context.Context, CreateCommand) (Summary, error)
	GetGlobalChat(context.Context, ConversationQuery) (Conversation, error)
	CreateGlobalChatMessage(context.Context, SendCommand) (Message, error)
	MarkGlobalChatRead(context.Context, Actor, string, time.Time) error
}

type PayloadEncryptor interface {
	Encrypt([]byte, string) ([]byte, int, error)
}

type Service struct {
	store     Store
	cursorKey []byte
	cipher    PayloadEncryptor
	now       func() time.Time
}

func NewService(store Store, cursorKey []byte, cipher PayloadEncryptor, now func() time.Time) (*Service, error) {
	if store == nil || len(cursorKey) < 32 || cipher == nil {
		return nil, errors.New("global chat service requires store, cursor key and notification cipher")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, cursorKey: append([]byte(nil), cursorKey...), cipher: cipher, now: now}, nil
}

func (service *Service) List(ctx context.Context, request ListRequest) (Page, error) {
	if !validActor(request.Actor) {
		return Page{}, ErrInvalidInput
	}
	if request.PageSize == 0 {
		request.PageSize = 25
	}
	if request.PageSize < 1 || request.PageSize > 50 {
		return Page{}, ErrInvalidInput
	}
	query := ListQuery{Actor: request.Actor, PageSize: request.PageSize + 1}
	if request.Cursor != "" {
		cursor, err := service.decodeCursor(request.Cursor)
		if err != nil {
			return Page{}, ErrInvalidInput
		}
		query.BeforeLastActivityAt, query.BeforeID = &cursor.At, &cursor.ID
	}
	items, err := service.store.ListGlobalChats(ctx, query)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(page.Items) > request.PageSize {
		page.HasMore = true
		page.Items = page.Items[:request.PageSize]
		last := page.Items[len(page.Items)-1]
		value := service.encodeCursor(chatCursor{At: last.LastActivityAt, ID: last.ID})
		page.NextCursor = &value
	}
	return page, nil
}

func (service *Service) SearchCandidates(ctx context.Context, actor Actor, query string) ([]Candidate, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if !validActor(actor) || len([]rune(query)) < 2 || len([]rune(query)) > 100 {
		return nil, ErrInvalidInput
	}
	return service.store.SearchGlobalChatCandidates(ctx, actor.UserID, query)
}

func (service *Service) Create(ctx context.Context, actor Actor, request CreateRequest) (Summary, error) {
	request.Kind = strings.TrimSpace(request.Kind)
	request.Name = strings.TrimSpace(request.Name)
	if !validActor(actor) || len(request.RequestID) < 8 || len(request.RequestID) > 100 || (request.Kind != "direct" && request.Kind != "group") {
		return Summary{}, ErrInvalidInput
	}
	seen := map[string]struct{}{actor.UserID: {}}
	members := make([]string, 0, len(request.MemberUserIDs))
	for _, raw := range request.MemberUserIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil || id == uuid.Nil {
			return Summary{}, ErrInvalidInput
		}
		value := id.String()
		if _, exists := seen[value]; exists {
			return Summary{}, ErrInvalidInput
		}
		seen[value] = struct{}{}
		members = append(members, value)
	}
	if request.Kind == "direct" && (len(members) != 1 || request.Name != "") {
		return Summary{}, ErrInvalidInput
	}
	if request.Kind == "group" && (len(members) < 1 || len(members) > 49 || len([]rune(request.Name)) < 2 || len([]rune(request.Name)) > 200) {
		return Summary{}, ErrInvalidInput
	}
	return service.store.CreateGlobalChat(ctx, CreateCommand{Actor: actor, Kind: request.Kind, Name: request.Name, MemberUserIDs: members, RequestID: request.RequestID, Now: service.now().UTC()})
}

func (service *Service) Get(ctx context.Context, actor Actor, chatID string, pageSize int, before string) (Conversation, error) {
	if !validActor(actor) || !validUUID(chatID) {
		return Conversation{}, ErrInvalidInput
	}
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize < 1 || pageSize > 100 {
		return Conversation{}, ErrInvalidInput
	}
	var beforeID *string
	if before != "" {
		if !validUUID(before) {
			return Conversation{}, ErrInvalidInput
		}
		beforeID = &before
	}
	return service.store.GetGlobalChat(ctx, ConversationQuery{Actor: actor, ChatID: chatID, PageSize: pageSize, BeforeID: beforeID})
}

func (service *Service) Send(ctx context.Context, actor Actor, chatID string, request SendRequest) (Message, error) {
	body := strings.TrimSpace(request.Body)
	if !validActor(actor) || !validUUID(chatID) || !validUUID(request.ClientMessageID) || len(request.RequestID) < 8 || len(request.RequestID) > 100 || len([]rune(body)) < 1 || len([]rune(body)) > 5000 {
		return Message{}, ErrInvalidInput
	}
	messageType := "global_chat.message_created"
	preview := []rune(body)
	if len(preview) > 1000 {
		preview = append(preview[:1000], '…')
	}
	payload, err := json.Marshal(map[string]string{"text": "Новое сообщение в чате\nОт: " + actor.FirstName + " (@" + actor.Login + ")\n\n" + string(preview)})
	if err != nil {
		return Message{}, err
	}
	ciphertext, version, err := service.cipher.Encrypt(payload, messageType)
	if err != nil {
		return Message{}, err
	}
	return service.store.CreateGlobalChatMessage(ctx, SendCommand{Actor: actor, ChatID: chatID, Body: body, ClientMessageID: request.ClientMessageID, RequestID: request.RequestID, Now: service.now().UTC(), Notification: &Notification{MessageType: messageType, PayloadCiphertext: ciphertext, PayloadKeyVersion: version}})
}

func (service *Service) MarkRead(ctx context.Context, actor Actor, chatID string) error {
	if !validActor(actor) || !validUUID(chatID) {
		return ErrInvalidInput
	}
	return service.store.MarkGlobalChatRead(ctx, actor, chatID, service.now().UTC())
}

type chatCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func (service *Service) encodeCursor(value chatCursor) string {
	payload, _ := json.Marshal(value)
	mac := hmac.New(sha256.New, service.cursorKey)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (service *Service) decodeCursor(value string) (chatCursor, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return chatCursor{}, ErrInvalidInput
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return chatCursor{}, ErrInvalidInput
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return chatCursor{}, ErrInvalidInput
	}
	mac := hmac.New(sha256.New, service.cursorKey)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return chatCursor{}, ErrInvalidInput
	}
	var cursor chatCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.At.IsZero() || !validUUID(cursor.ID) {
		return chatCursor{}, ErrInvalidInput
	}
	return cursor, nil
}

func validActor(actor Actor) bool { return validUUID(actor.UserID) }
func validUUID(value string) bool {
	id, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil && id != uuid.Nil
}
