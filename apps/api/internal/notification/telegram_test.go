package notification

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failingHTTPClient struct{}

func (failingHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("network unavailable")
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
		response.WriteHeader(http.StatusOK)
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
