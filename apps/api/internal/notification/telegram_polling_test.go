package notification

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type pollingHTTPClientFunc func(*http.Request) (*http.Response, error)

func (function pollingHTTPClientFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

type pollingHTTPClient struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	paths  []string
	bodies []string
}

func (client *pollingHTTPClient) Do(request *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(request.Body)
	client.mu.Lock()
	client.paths = append(client.paths, request.URL.Path)
	client.bodies = append(client.bodies, string(body))
	client.mu.Unlock()
	responseBody := `{"ok":true,"result":true}`
	switch {
	case strings.HasSuffix(request.URL.Path, "/getUpdates"):
		responseBody = `{"ok":true,"result":[{"update_id":41,"message":{"message_id":7,"text":"/start","chat":{"id":99,"type":"private"},"from":{"first_name":"Иван"}}}]}`
	case strings.HasSuffix(request.URL.Path, "/sendMessage"):
		responseBody = `{"ok":true,"result":{"message_id":55}}`
		client.cancel()
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(responseBody)), Header: make(http.Header)}, nil
}

type pollingProcessor struct{ updates []TelegramUpdate }

func (processor *pollingProcessor) ProcessTelegramUpdate(_ context.Context, update TelegramUpdate) TelegramReply {
	processor.updates = append(processor.updates, update)
	return TelegramReply{ChatID: update.Message.Chat.ID, Text: "Добро пожаловать", Action: "Подписаться"}
}

func TestTelegramPollerDeletesWebhookProcessesUpdateAndReplies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &pollingHTTPClient{cancel: cancel}
	processor := &pollingProcessor{}
	poller, err := NewTelegramPoller("test-token", client, processor, nil)
	if err != nil {
		t.Fatal(err)
	}
	poller.Run(ctx)
	if len(processor.updates) != 1 || processor.updates[0].UpdateID != 41 {
		t.Fatalf("updates=%#v", processor.updates)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.paths) != 3 || !strings.HasSuffix(client.paths[0], "/deleteWebhook") || !strings.HasSuffix(client.paths[1], "/getUpdates") || !strings.HasSuffix(client.paths[2], "/sendMessage") {
		t.Fatalf("unexpected Telegram call sequence: %#v", client.paths)
	}
	if !strings.Contains(client.bodies[0], `"drop_pending_updates":false`) || !strings.Contains(client.bodies[2], "Подписаться") {
		t.Fatalf("unexpected request bodies: %#v", client.bodies)
	}
}

func TestNewTelegramPollerRequiresDependencies(t *testing.T) {
	if _, err := NewTelegramPoller("", &pollingHTTPClient{}, &pollingProcessor{}, nil); err == nil {
		t.Fatal("empty token accepted")
	}
}

func TestTelegramPollerCallRejectsTransportAndTelegramErrors(t *testing.T) {
	response := func(status int, body string) pollingHTTPClientFunc {
		return func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
	}
	for name, client := range map[string]HTTPClient{
		"transport": pollingHTTPClientFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }),
		"status":    response(http.StatusBadGateway, `bad gateway`),
		"json":      response(http.StatusOK, `{`),
		"not ok":    response(http.StatusOK, `{"ok":false,"result":null}`),
		"result":    response(http.StatusOK, `{"ok":true,"result":"invalid"}`),
	} {
		t.Run(name, func(t *testing.T) {
			poller := &TelegramPoller{client: client, endpoint: "https://api.telegram.test"}
			var target struct {
				Result []TelegramUpdate `json:"result"`
			}
			if err := poller.call(context.Background(), "/getUpdates", map[string]any{"offset": 1}, &target); err == nil {
				t.Fatal("invalid Telegram response accepted")
			}
		})
	}

	poller := &TelegramPoller{client: response(http.StatusOK, `{"ok":true,"result":true}`), endpoint: "://bad"}
	if err := poller.call(context.Background(), "/deleteWebhook", map[string]any{}, nil); err == nil {
		t.Fatal("invalid endpoint accepted")
	}
}

func TestTelegramPollingBackoffAndCancellation(t *testing.T) {
	if nextTelegramPollingBackoff(time.Second) != 2*time.Second || nextTelegramPollingBackoff(20*time.Second) != 30*time.Second || nextTelegramPollingBackoff(30*time.Second) != 30*time.Second {
		t.Fatal("unexpected bounded backoff")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitTelegramRetry(ctx, time.Hour) {
		t.Fatal("cancelled retry wait must stop")
	}
	if !waitTelegramRetry(context.Background(), time.Millisecond) {
		t.Fatal("elapsed retry wait must continue")
	}
}
