package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type telegramStoreFake struct {
	active                 bool
	activated, deactivated int
}

func (store *telegramStoreFake) ActivateTelegramSubscriber(context.Context, int64, string) error {
	store.active = true
	store.activated++
	return nil
}
func (store *telegramStoreFake) DeactivateTelegramSubscriber(context.Context, int64) error {
	store.active = false
	store.deactivated++
	return nil
}
func (store *telegramStoreFake) ActiveTelegramChatIDs(context.Context) ([]int64, error) {
	return nil, nil
}
func (store *telegramStoreFake) TelegramSubscriberActive(context.Context, int64) (bool, error) {
	return store.active, nil
}

func telegramRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(body))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "webhook-secret")
	return request
}
func telegramResponse(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}
func TestTelegramSubscriptionDialog(t *testing.T) {
	store := &telegramStoreFake{}
	endpoint := NewTelegramWebhook("webhook-secret", "admin", "password", store)
	var last map[string]any
	for _, body := range []string{
		`{"message":{"text":"Подписаться","chat":{"id":7,"type":"private"},"from":{"username":"sveta"}}}`,
		`{"message":{"text":"admin","chat":{"id":7,"type":"private"},"from":{"username":"sveta"}}}`,
		`{"message":{"text":"password","chat":{"id":7,"type":"private"},"from":{"username":"sveta"}}}`,
	} {
		response := httptest.NewRecorder()
		endpoint.ServeHTTP(response, telegramRequest(body))
		if response.Code != http.StatusOK {
			t.Fatalf("status=%d", response.Code)
		}
		last = telegramResponse(t, response)
	}
	if !store.active || store.activated != 1 || !strings.Contains(last["text"].(string), "Вы подписались") {
		t.Fatalf("unexpected subscription: %#v %#v", store, last)
	}
	keyboard := last["reply_markup"].(map[string]any)["keyboard"].([]any)
	if keyboard[0].([]any)[0] != "Отписаться" {
		t.Fatalf("expected unsubscribe button, got %#v", keyboard)
	}
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(response, telegramRequest(`{"message":{"text":"Отписаться","chat":{"id":7,"type":"private"},"from":{}}}`))
	if store.active || store.deactivated != 1 {
		t.Fatal("unsubscribe must deactivate chat")
	}
}
func TestTelegramWebhookRejectsBadSecretAndCredentials(t *testing.T) {
	store := &telegramStoreFake{}
	endpoint := NewTelegramWebhook("webhook-secret", "admin", "password", store)
	bad := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(response, bad)
	if response.Code != http.StatusUnauthorized {
		t.Fatal("bad secret must be rejected")
	}
	var last map[string]any
	for _, body := range []string{`{"message":{"text":"Подписаться","chat":{"id":8,"type":"private"},"from":{}}}`, `{"message":{"text":"wrong","chat":{"id":8,"type":"private"},"from":{}}}`, `{"message":{"text":"wrong","chat":{"id":8,"type":"private"},"from":{}}}`} {
		response := httptest.NewRecorder()
		endpoint.ServeHTTP(response, telegramRequest(body))
		last = telegramResponse(t, response)
	}
	if store.active || !strings.Contains(last["text"].(string), "Ошибка") {
		t.Fatal("wrong credentials must not subscribe")
	}
}
