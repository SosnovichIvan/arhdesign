package handler

import (
	"context"
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

type telegramSenderFake struct {
	messages []string
	menus    []bool
}

func (sender *telegramSenderFake) SendMessage(_ context.Context, _ int64, text string, menu, _ bool) error {
	sender.messages = append(sender.messages, text)
	sender.menus = append(sender.menus, menu)
	return nil
}
func telegramRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(body))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "webhook-secret")
	return request
}
func TestTelegramSubscriptionDialog(t *testing.T) {
	store, sender := &telegramStoreFake{}, &telegramSenderFake{}
	endpoint := NewTelegramWebhook("webhook-secret", "admin", "password", store, sender)
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
	}
	if !store.active || store.activated != 1 || !strings.Contains(sender.messages[len(sender.messages)-1], "Вы подписались") {
		t.Fatalf("unexpected subscription: %#v %#v", store, sender.messages)
	}
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(response, telegramRequest(`{"message":{"text":"Отписаться","chat":{"id":7,"type":"private"},"from":{}}}`))
	if store.active || store.deactivated != 1 {
		t.Fatal("unsubscribe must deactivate chat")
	}
}
func TestTelegramWebhookRejectsBadSecretAndCredentials(t *testing.T) {
	store, sender := &telegramStoreFake{}, &telegramSenderFake{}
	endpoint := NewTelegramWebhook("webhook-secret", "admin", "password", store, sender)
	bad := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(response, bad)
	if response.Code != http.StatusUnauthorized {
		t.Fatal("bad secret must be rejected")
	}
	for _, body := range []string{`{"message":{"text":"Подписаться","chat":{"id":8,"type":"private"},"from":{}}}`, `{"message":{"text":"wrong","chat":{"id":8,"type":"private"},"from":{}}}`, `{"message":{"text":"wrong","chat":{"id":8,"type":"private"},"from":{}}}`} {
		endpoint.ServeHTTP(httptest.NewRecorder(), telegramRequest(body))
	}
	if store.active || !strings.Contains(sender.messages[len(sender.messages)-1], "Ошибка") {
		t.Fatal("wrong credentials must not subscribe")
	}
}
