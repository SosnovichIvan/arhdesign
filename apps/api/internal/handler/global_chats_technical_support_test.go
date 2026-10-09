package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
	"github.com/google/uuid"
)

const (
	handlerChatID    = "00000000-0000-0000-0000-000000000301"
	handlerMemberID  = "00000000-0000-0000-0000-000000000302"
	handlerMessageID = "00000000-0000-0000-0000-000000000303"
)

type handlerGlobalChatStore struct {
	items        []globalchat.Summary
	candidates   []globalchat.Candidate
	created      globalchat.Summary
	conversation globalchat.Conversation
	message      globalchat.Message
	err          error
}

func (store *handlerGlobalChatStore) ListGlobalChats(context.Context, globalchat.ListQuery) ([]globalchat.Summary, error) {
	return store.items, store.err
}
func (store *handlerGlobalChatStore) SearchGlobalChatCandidates(context.Context, string, string) ([]globalchat.Candidate, error) {
	return store.candidates, store.err
}
func (store *handlerGlobalChatStore) CreateGlobalChat(context.Context, globalchat.CreateCommand) (globalchat.Summary, error) {
	return store.created, store.err
}
func (store *handlerGlobalChatStore) GetGlobalChat(context.Context, globalchat.ConversationQuery) (globalchat.Conversation, error) {
	return store.conversation, store.err
}
func (store *handlerGlobalChatStore) CreateGlobalChatMessage(context.Context, globalchat.SendCommand) (globalchat.Message, error) {
	return store.message, store.err
}
func (store *handlerGlobalChatStore) MarkGlobalChatRead(context.Context, globalchat.Actor, string, time.Time) error {
	return store.err
}

func chatHandlerEndpoint(t *testing.T, store *handlerGlobalChatStore) *AccountEndpoint {
	t.Helper()
	role := "technical_admin"
	endpoint := accountEndpointForTest(t, activeAccount(&role))
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{71}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := globalchat.NewService(store, bytes.Repeat([]byte{72}, 32), cipher, func() time.Time { return time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	endpoint.ConfigureGlobalChats(service)
	return endpoint
}

func chatFixtures() (globalchat.Summary, globalchat.Message) {
	lastName := "Иванова"
	message := globalchat.Message{ID: handlerMessageID, ChatID: handlerChatID, Body: "Привет", Author: globalchat.Author{UserID: endpointActorID, Login: "svetlana", FirstName: "Светлана"}, CreatedAt: time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC), Version: 1}
	summary := globalchat.Summary{ID: handlerChatID, Kind: "direct", DisplayName: "Мария Иванова", Members: []globalchat.Member{{Candidate: globalchat.Candidate{UserID: handlerMemberID, Login: "maria", Email: "maria@example.test", FirstName: "Мария", LastName: &lastName}, JoinedAt: time.Date(2026, 9, 28, 13, 0, 0, 0, time.UTC)}}, LastMessage: &message, LastActivityAt: message.CreatedAt, Version: 1}
	return summary, message
}

func TestGlobalChatHandlersHappyPath(t *testing.T) {
	summary, message := chatFixtures()
	nextCursor := handlerMessageID
	secondSummary := summary
	secondSummary.ID = handlerMemberID
	secondSummary.LastActivityAt = summary.LastActivityAt.Add(-time.Minute)
	store := &handlerGlobalChatStore{items: []globalchat.Summary{summary, secondSummary}, candidates: []globalchat.Candidate{summary.Members[0].Candidate}, created: summary, message: message, conversation: globalchat.Conversation{Chat: summary, CurrentUserID: endpointActorID, Messages: []globalchat.Message{message}, NextCursor: &nextCursor, HasMore: true, CanSend: true}}
	endpoint := chatHandlerEndpoint(t, store)

	list := httptest.NewRecorder()
	pageSize := 1
	firstPage, err := endpoint.globalChats.List(context.Background(), globalchat.ListRequest{Actor: globalchat.Actor{UserID: endpointActorID, Privileged: true}, PageSize: pageSize})
	if err != nil || firstPage.NextCursor == nil {
		t.Fatalf("prepare list cursor page=%#v error=%v", firstPage, err)
	}
	cursor := *firstPage.NextCursor
	endpoint.ListGlobalChats(list, authenticatedRequest(http.MethodGet, "/api/v1/chats", ""), accountgenerated.ListGlobalChatsParams{PageSize: &pageSize, Cursor: &cursor})
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "Мария Иванова") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	search := httptest.NewRecorder()
	endpoint.SearchGlobalChatCandidates(search, authenticatedRequest(http.MethodGet, "/api/v1/chat-user-candidates?query=mar", ""), accountgenerated.SearchGlobalChatCandidatesParams{Query: "mar"})
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "maria@example.test") {
		t.Fatalf("search status=%d body=%s", search.Code, search.Body.String())
	}

	create := httptest.NewRecorder()
	endpoint.CreateGlobalChat(create, authenticatedRequest(http.MethodPost, "/api/v1/chats", `{"kind":"direct","memberUserIds":["`+handlerMemberID+`"]}`))
	if create.Code != http.StatusCreated || create.Header().Get("Location") != "/api/v1/chats/"+handlerChatID {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	groupName := "Рабочая группа"
	store.created.Name = &groupName
	createGroup := httptest.NewRecorder()
	endpoint.CreateGlobalChat(createGroup, authenticatedRequest(http.MethodPost, "/api/v1/chats", `{"kind":"group","name":"Рабочая группа","memberUserIds":["`+handlerMemberID+`"]}`))
	if createGroup.Code != http.StatusCreated {
		t.Fatalf("group create status=%d body=%s", createGroup.Code, createGroup.Body.String())
	}

	get := httptest.NewRecorder()
	chatUUID := uuid.MustParse(handlerChatID)
	before := uuid.MustParse(handlerMessageID)
	endpoint.GetGlobalChat(get, authenticatedRequest(http.MethodGet, "/api/v1/chats/"+handlerChatID, ""), chatUUID, accountgenerated.GetGlobalChatParams{PageSize: &pageSize, Before: &before})
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "Привет") {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}

	send := httptest.NewRecorder()
	endpoint.CreateGlobalChatMessage(send, authenticatedRequest(http.MethodPost, "/api/v1/chats/"+handlerChatID+"/messages", `{"body":"Привет","clientMessageId":"`+handlerMessageID+`"}`), chatUUID)
	if send.Code != http.StatusCreated {
		t.Fatalf("send status=%d body=%s", send.Code, send.Body.String())
	}

	read := httptest.NewRecorder()
	endpoint.MarkGlobalChatRead(read, authenticatedRequest(http.MethodPost, "/api/v1/chats/"+handlerChatID+"/read", ""), chatUUID)
	if read.Code != http.StatusNoContent {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
}

func TestGlobalChatHandlersRejectUnavailableMalformedAndDomainErrors(t *testing.T) {
	unavailable := accountEndpointForTest(t, activeAccount(nil))
	response := httptest.NewRecorder()
	unavailable.ListGlobalChats(response, authenticatedRequest(http.MethodGet, "/api/v1/chats", ""), accountgenerated.ListGlobalChatsParams{})
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable=%d", response.Code)
	}

	endpoint := chatHandlerEndpoint(t, &handlerGlobalChatStore{})
	unauthenticated := httptest.NewRecorder()
	endpoint.ListGlobalChats(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil), accountgenerated.ListGlobalChatsParams{})
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d", unauthenticated.Code)
	}
	malformed := httptest.NewRecorder()
	endpoint.CreateGlobalChat(malformed, authenticatedRequest(http.MethodPost, "/api/v1/chats", `{`))
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed=%d", malformed.Code)
	}

	for _, testCase := range []struct {
		err    error
		status int
	}{{globalchat.ErrInvalidInput, 400}, {globalchat.ErrNotFound, 404}, {globalchat.ErrConflict, 409}, {errors.New("database"), 500}} {
		w := httptest.NewRecorder()
		endpoint.writeGlobalChatError(w, testCase.err, "request-chat")
		if w.Code != testCase.status {
			t.Fatalf("err=%v status=%d", testCase.err, w.Code)
		}
	}

	chatUUID := uuid.MustParse(handlerChatID)
	failed := chatHandlerEndpoint(t, &handlerGlobalChatStore{err: globalchat.ErrNotFound})
	domainCalls := []func(*httptest.ResponseRecorder){
		func(w *httptest.ResponseRecorder) {
			failed.ListGlobalChats(w, authenticatedRequest(http.MethodGet, "/chats", ""), accountgenerated.ListGlobalChatsParams{})
		},
		func(w *httptest.ResponseRecorder) {
			failed.SearchGlobalChatCandidates(w, authenticatedRequest(http.MethodGet, "/candidates", ""), accountgenerated.SearchGlobalChatCandidatesParams{Query: "user"})
		},
		func(w *httptest.ResponseRecorder) {
			failed.CreateGlobalChat(w, authenticatedRequest(http.MethodPost, "/chats", `{"kind":"direct","memberUserIds":["`+handlerMemberID+`"]}`))
		},
		func(w *httptest.ResponseRecorder) {
			failed.GetGlobalChat(w, authenticatedRequest(http.MethodGet, "/chat", ""), chatUUID, accountgenerated.GetGlobalChatParams{})
		},
		func(w *httptest.ResponseRecorder) {
			failed.CreateGlobalChatMessage(w, authenticatedRequest(http.MethodPost, "/messages", `{"body":"Привет","clientMessageId":"`+handlerMessageID+`"}`), chatUUID)
		},
		func(w *httptest.ResponseRecorder) {
			failed.MarkGlobalChatRead(w, authenticatedRequest(http.MethodPost, "/read", ""), chatUUID)
		},
	}
	for index, call := range domainCalls {
		w := httptest.NewRecorder()
		call(w)
		if w.Code != http.StatusNotFound {
			t.Fatalf("domain call %d status=%d body=%s", index, w.Code, w.Body.String())
		}
	}
	for index, call := range []func(*httptest.ResponseRecorder){
		func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/chats", `{}`)
			request.Header.Del("X-CSRF-Token")
			endpoint.CreateGlobalChat(w, request)
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.CreateGlobalChatMessage(w, authenticatedRequest(http.MethodPost, "/messages", `{`), chatUUID)
		},
		func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/messages", `{}`)
			request.Header.Del("X-CSRF-Token")
			endpoint.CreateGlobalChatMessage(w, request, chatUUID)
		},
		func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/read", "")
			request.Header.Del("X-CSRF-Token")
			endpoint.MarkGlobalChatRead(w, request, chatUUID)
		},
	} {
		w := httptest.NewRecorder()
		call(w)
		want := http.StatusBadRequest
		if index == 0 || index == 2 || index == 3 {
			want = http.StatusForbidden
		}
		if w.Code != want {
			t.Fatalf("invalid call %d status=%d want=%d", index, w.Code, want)
		}
	}
}

func TestTechnicalSupportHandlersAndErrorMapping(t *testing.T) {
	endpoint := accountEndpointForTest(t, activeAccount(nil))
	store := &middlewareTechnicalStore{}
	endpoint.ConfigureTechnicalSupport(technicalMiddlewareService(t, store))

	frontend := httptest.NewRecorder()
	endpoint.ReportFrontendError(frontend, authenticatedRequest(http.MethodPost, "/api/v1/technical/frontend-errors", `{"message":"runtime error","path":"/account/chats","fingerprint":"abcdefghijklmnop","digest":"digest-1"}`))
	if frontend.Code != http.StatusAccepted {
		t.Fatalf("frontend=%d body=%s", frontend.Code, frontend.Body.String())
	}

	feedback := httptest.NewRecorder()
	endpoint.CreateTechnicalFeedback(feedback, authenticatedRequest(http.MethodPost, "/api/v1/technical/feedback", `{"category":"complaint","message":"Не открывается чат","path":"/account/chats"}`))
	if feedback.Code != http.StatusAccepted || len(store.reports) != 2 {
		t.Fatalf("feedback=%d reports=%d body=%s", feedback.Code, len(store.reports), feedback.Body.String())
	}

	unavailable := accountEndpointForTest(t, activeAccount(nil))
	w := httptest.NewRecorder()
	unavailable.CreateTechnicalFeedback(w, authenticatedRequest(http.MethodPost, "/feedback", `{}`))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable=%d", w.Code)
	}

	for _, testCase := range []struct {
		err    error
		status int
	}{{technicalsupport.ErrInvalidInput, 400}, {technicalsupport.ErrRateLimited, 429}, {errors.New("database"), 500}} {
		w = httptest.NewRecorder()
		endpoint.writeTechnicalSupportError(w, testCase.err, "request-support")
		if w.Code != testCase.status {
			t.Fatalf("err=%v status=%d", testCase.err, w.Code)
		}
	}

	for index, call := range []func(*httptest.ResponseRecorder){
		func(w *httptest.ResponseRecorder) {
			endpoint.ReportFrontendError(w, authenticatedRequest(http.MethodPost, "/frontend", `{`))
		},
		func(w *httptest.ResponseRecorder) {
			endpoint.CreateTechnicalFeedback(w, authenticatedRequest(http.MethodPost, "/feedback", `{`))
		},
		func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/frontend", `{"message":"runtime error","path":"/account","fingerprint":"abcdefghijklmnop"}`)
			request.Header.Del("X-CSRF-Token")
			endpoint.ReportFrontendError(w, request)
		},
		func(w *httptest.ResponseRecorder) {
			request := authenticatedRequest(http.MethodPost, "/feedback", `{"category":"suggestion","message":"Пожелание"}`)
			request.Header.Del("X-CSRF-Token")
			endpoint.CreateTechnicalFeedback(w, request)
		},
	} {
		w = httptest.NewRecorder()
		call(w)
		want := http.StatusBadRequest
		if index >= 2 {
			want = http.StatusForbidden
		}
		if w.Code != want {
			t.Fatalf("technical invalid %d status=%d want=%d", index, w.Code, want)
		}
	}
	unauthenticated := httptest.NewRecorder()
	endpoint.ReportFrontendError(unauthenticated, httptest.NewRequest(http.MethodPost, "/frontend", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("technical unauthenticated status=%d", unauthenticated.Code)
	}
	failingStore := &middlewareTechnicalStore{err: context.DeadlineExceeded}
	failingEndpoint := accountEndpointForTest(t, activeAccount(nil))
	failingEndpoint.ConfigureTechnicalSupport(technicalMiddlewareService(t, failingStore))
	failed := httptest.NewRecorder()
	failingEndpoint.ReportFrontendError(failed, authenticatedRequest(http.MethodPost, "/frontend", `{"message":"runtime error","path":"/account","fingerprint":"abcdefghijklmnop"}`))
	if failed.Code != http.StatusInternalServerError {
		t.Fatalf("technical service failure status=%d body=%s", failed.Code, failed.Body.String())
	}
}
