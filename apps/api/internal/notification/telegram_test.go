package notification

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingHTTPClient struct{}

func (failingHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("network unavailable")
}

type telegramSubscriberStoreStub struct {
	chatIDs   []int64
	messageID int64
	deleteAt  time.Time
	recordErr error
	listErr   error
}

func (store telegramSubscriberStoreStub) ActiveTelegramChatIDs(context.Context) ([]int64, error) {
	return store.chatIDs, store.listErr
}

func (store *telegramSubscriberStoreStub) RecordTelegramNotification(_ context.Context, _ int64, messageID int64, deleteAt time.Time) error {
	store.messageID = messageID
	store.deleteAt = deleteAt
	return store.recordErr
}

func TestTelegramSubscriberConstructorsAndName(t *testing.T) {
	store := &telegramSubscriberStoreStub{}
	direct := NewTelegramSubscribers("secret", store, http.DefaultClient, time.Hour)
	if direct.Name() != "telegram" || !strings.Contains(direct.url, "/botsecret/sendMessage") || direct.retention != time.Hour {
		t.Fatalf("unexpected direct subscriber adapter: %#v", direct)
	}
	(NoopDispatcher{}).Notify(context.Background(), Submission{})
}

func TestTelegramSendsJSONToBotAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"chat_id":"123"`) || !strings.Contains(string(body), "Анна") {
			t.Fatalf("unexpected telegram payload: %s", body)
		}
		_, _ = response.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	}))
	defer server.Close()

	adapter := Telegram{chatID: "123", client: server.Client(), url: server.URL}
	if err := adapter.Send(context.Background(), Submission{Name: "Анна", Contact: "anna@example.com", ProjectType: "Квартира", ProjectDetails: "Нужен проект"}); err != nil {
		t.Fatal(err)
	}
}

func TestNewTelegramUsesConfiguredBotURL(t *testing.T) {
	adapter := NewTelegram("secret", "123", http.DefaultClient)
	if adapter.Name() != "telegram" || !strings.Contains(adapter.url, "/botsecret/sendMessage") {
		t.Fatalf("unexpected adapter: %#v", adapter)
	}
}

func TestTelegramReturnsErrorForUnsuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusBadGateway) }))
	defer server.Close()
	adapter := Telegram{chatID: "123", client: server.Client(), url: server.URL}
	if err := adapter.Send(context.Background(), Submission{}); err == nil {
		t.Fatal("non-successful Telegram response must return an error")
	}
}

func TestTelegramReturnsClientError(t *testing.T) {
	adapter := Telegram{chatID: "123", client: failingHTTPClient{}, url: "https://example.test"}
	if err := adapter.Send(context.Background(), Submission{}); err == nil {
		t.Fatal("client error must be returned")
	}
}

func TestTelegramReturnsRequestConstructionError(t *testing.T) {
	adapter := Telegram{chatID: "123", client: failingHTTPClient{}, url: "://not-a-url"}
	if err := adapter.Send(context.Background(), Submission{}); err == nil {
		t.Fatal("invalid request URL must be returned")
	}
}

func TestTelegramSubscribersSendThroughAuthenticatedRelay(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/sendMessage" || request.Header.Get("Authorization") != "Bearer relay-secret" {
			t.Fatalf("unexpected relay request: %s, authorization %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"chat_id":"123"`) || strings.Contains(string(body), "relay-secret") {
			t.Fatalf("unexpected relay payload: %s", body)
		}
		_, _ = response.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	}))
	defer server.Close()

	store := &telegramSubscriberStoreStub{chatIDs: []int64{123}}
	adapter := NewTelegramSubscribersViaRelay(server.URL+"/sendMessage", "relay-secret", store, server.Client(), 24*time.Hour)
	if err := adapter.Send(context.Background(), Submission{Name: "Анна", Contact: "anna@example.com", ProjectType: "Квартира"}); err != nil {
		t.Fatal(err)
	}
	if store.messageID != 42 || time.Until(store.deleteAt) < 23*time.Hour {
		t.Fatalf("notification receipt was not recorded: %#v", store)
	}
}

func TestTelegramDeletesMessageThroughMatchingEndpoint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/deleteMessage" || request.Header.Get("Authorization") != "Bearer relay-secret" {
			t.Fatalf("unexpected delete request: %s", request.URL.Path)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"message_id":42`) {
			t.Fatalf("unexpected deletion payload: %s", body)
		}
		_, _ = response.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramViaRelay(server.URL+"/sendMessage", "relay-secret", server.Client())
	if err := adapter.DeleteMessage(context.Background(), 123, 42); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramSendMessageIncludesMenu(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "Отписаться") || !strings.Contains(string(body), "keyboard") {
			t.Fatalf("unexpected menu payload: %s", body)
		}
		_, _ = response.Write([]byte(`{"ok":true,"result":{"message_id":43}}`))
	}))
	defer server.Close()

	adapter := Telegram{client: server.Client(), url: server.URL}
	if err := adapter.SendMessage(context.Background(), 123, "Меню", true, true); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramRejectsInvalidSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()
	adapter := Telegram{chatID: "123", client: server.Client(), url: server.URL}
	if err := adapter.Send(context.Background(), Submission{}); err == nil {
		t.Fatal("invalid Telegram success response must fail")
	}
}

func TestTelegramDeletesImmediatelyWhenReceiptCannotBeRecorded(t *testing.T) {
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/deleteMessage" {
			deleted = true
			_, _ = response.Write([]byte(`{"ok":true,"result":true}`))
			return
		}
		_, _ = response.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	}))
	defer server.Close()
	store := &telegramSubscriberStoreStub{chatIDs: []int64{123}, recordErr: errors.New("database unavailable")}
	adapter := TelegramSubscribers{store: store, client: server.Client(), retention: time.Hour, url: server.URL + "/sendMessage"}
	if err := adapter.Send(context.Background(), Submission{}); err == nil || !deleted {
		t.Fatalf("send error = %v, deleted = %v; want error and immediate deletion", err, deleted)
	}
}

func TestTelegramSubscribersReportsLookupError(t *testing.T) {
	store := &telegramSubscriberStoreStub{listErr: errors.New("database unavailable")}
	adapter := NewTelegramSubscribers("secret", store, http.DefaultClient, time.Hour)
	if err := adapter.Send(context.Background(), Submission{}); err == nil {
		t.Fatal("subscriber lookup error must be returned")
	}
}

func TestTelegramDeleteReportsConstructionAndHTTPError(t *testing.T) {
	invalid := Telegram{client: http.DefaultClient, url: "://not-a-url/sendMessage"}
	if err := invalid.DeleteMessage(context.Background(), 123, 42); err == nil {
		t.Fatal("invalid delete URL must fail")
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	adapter := Telegram{client: server.Client(), url: server.URL + "/sendMessage"}
	if err := adapter.DeleteMessage(context.Background(), 123, 42); err == nil {
		t.Fatal("unsuccessful delete response must fail")
	}
}
