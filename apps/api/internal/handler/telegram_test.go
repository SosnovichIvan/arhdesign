package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
)

type telegramAccountAuthFake struct {
	step                      string
	beginErr, stepErr         error
	submitErr, completeErr    error
	beginChat, submitChat     int64
	completeChat, completeMsg int64
	login, username, password string
}

func (service *telegramAccountAuthFake) BeginTelegramConfirmation(_ context.Context, chatID int64, _ string) error {
	service.beginChat = chatID
	service.step = "awaiting_login"
	return service.beginErr
}
func (service *telegramAccountAuthFake) TelegramConfirmationStep(context.Context, int64) (string, error) {
	return service.step, service.stepErr
}
func (service *telegramAccountAuthFake) SubmitTelegramLogin(_ context.Context, chatID int64, login string) error {
	service.submitChat, service.login = chatID, login
	service.step = "awaiting_password"
	return service.submitErr
}
func (service *telegramAccountAuthFake) CompleteTelegramConfirmation(_ context.Context, chatID, messageID int64, username, password, _ string) error {
	service.completeChat, service.completeMsg = chatID, messageID
	service.username, service.password = username, password
	return service.completeErr
}

type telegramDeleterFake struct{ chatID, messageID int64 }

func (deleter *telegramDeleterFake) DeleteMessage(_ context.Context, chatID, messageID int64) error {
	deleter.chatID, deleter.messageID = chatID, messageID
	return nil
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

func TestTelegramConfirmationDialogRunsInsidePrivateBotAndDeletesPassword(t *testing.T) {
	accounts := &telegramAccountAuthFake{}
	deleter := &telegramDeleterFake{}
	endpoint := NewTelegramWebhook("webhook-secret", accounts, deleter)

	start := httptest.NewRecorder()
	endpoint.ServeHTTP(start, telegramRequest(`{"update_id":1,"message":{"message_id":10,"text":"/start register","chat":{"id":7,"type":"private"},"from":{"username":"sveta"}}}`))
	if start.Code != http.StatusOK || accounts.beginChat != 7 || !strings.Contains(telegramResponse(t, start)["text"].(string), "Введите логин") {
		t.Fatalf("start status=%d chat=%d body=%s", start.Code, accounts.beginChat, start.Body.String())
	}

	login := httptest.NewRecorder()
	endpoint.ServeHTTP(login, telegramRequest(`{"update_id":2,"message":{"message_id":11,"text":"sveta.design","chat":{"id":7,"type":"private"},"from":{"username":"sveta"}}}`))
	if accounts.login != "sveta.design" || !strings.Contains(telegramResponse(t, login)["text"].(string), "Введите пароль") {
		t.Fatalf("login=%q body=%s", accounts.login, login.Body.String())
	}

	password := httptest.NewRecorder()
	endpoint.ServeHTTP(password, telegramRequest(`{"update_id":3,"message":{"message_id":12,"text":"secret-password","chat":{"id":7,"type":"private"},"from":{"username":"sveta"}}}`))
	if accounts.password != "secret-password" || accounts.completeChat != 7 || accounts.completeMsg != 12 {
		t.Fatalf("complete chat=%d message=%d password=%q", accounts.completeChat, accounts.completeMsg, accounts.password)
	}
	if deleter.chatID != 7 || deleter.messageID != 12 {
		t.Fatalf("password message was not deleted: %#v", deleter)
	}
	if !strings.Contains(telegramResponse(t, password)["text"].(string), "Профиль подтверждён") {
		t.Fatalf("success text missing: %s", password.Body.String())
	}
	if strings.Contains(password.Body.String(), "secret-password") {
		t.Fatalf("credential was echoed into the Telegram response: %s", password.Body.String())
	}
}

func TestTelegramPasswordIsDeletedOnInvalidCredentials(t *testing.T) {
	accounts := &telegramAccountAuthFake{step: "awaiting_password", completeErr: errors.New("invalid credentials")}
	deleter := &telegramDeleterFake{}
	endpoint := NewTelegramWebhook("webhook-secret", accounts, deleter)
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(response, telegramRequest(`{"update_id":4,"message":{"message_id":19,"text":"wrong","chat":{"id":8,"type":"private"},"from":{}}}`))
	if deleter.messageID != 19 || !strings.Contains(telegramResponse(t, response)["text"].(string), "Не удалось подтвердить") {
		t.Fatalf("delete=%#v body=%s", deleter, response.Body.String())
	}
}

func TestTelegramStartWelcomeAndSecurityBoundary(t *testing.T) {
	endpoint := NewTelegramWebhook("webhook-secret", &telegramAccountAuthFake{}, &telegramDeleterFake{})
	welcome := httptest.NewRecorder()
	endpoint.ServeHTTP(welcome, telegramRequest(`{"update_id":5,"message":{"message_id":20,"text":"/start","chat":{"id":9,"type":"private"},"from":{"first_name":"Иван"}}}`))
	payload := telegramResponse(t, welcome)
	if !strings.Contains(payload["text"].(string), "Добро пожаловать Иван") || !strings.Contains(payload["text"].(string), "бот личного кабинета") {
		t.Fatalf("welcome text missing: %#v", payload)
	}

	withoutName := httptest.NewRecorder()
	endpoint.ServeHTTP(withoutName, telegramRequest(`{"update_id":51,"message":{"message_id":201,"text":"/menu","chat":{"id":9,"type":"private"},"from":{"first_name":"Иван\nПлохой ввод"}}}`))
	if text := telegramResponse(t, withoutName)["text"].(string); strings.Contains(text, "Иван") || !strings.HasPrefix(text, "Добро пожаловать\n") {
		t.Fatalf("unsafe first name must not be reflected: %q", text)
	}

	badSecret := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`{}`))
	endpoint.ServeHTTP(badSecret, request)
	if badSecret.Code != http.StatusUnauthorized {
		t.Fatalf("bad secret status=%d", badSecret.Code)
	}

	group := httptest.NewRecorder()
	endpoint.ServeHTTP(group, telegramRequest(`{"update_id":6,"message":{"message_id":21,"text":"/start register","chat":{"id":-10,"type":"group"},"from":{}}}`))
	if group.Code != http.StatusOK || group.Body.Len() != 0 {
		t.Fatalf("group update must be ignored, status=%d body=%s", group.Code, group.Body.String())
	}

	wrongMethod := httptest.NewRecorder()
	endpoint.ServeHTTP(wrongMethod, httptest.NewRequest(http.MethodGet, "/api/telegram/webhook", nil))
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status=%d", wrongMethod.Code)
	}

	malformed := httptest.NewRecorder()
	endpoint.ServeHTTP(malformed, telegramRequest(`{`))
	if malformed.Code != http.StatusOK || malformed.Body.Len() != 0 {
		t.Fatalf("malformed update status=%d body=%s", malformed.Code, malformed.Body.String())
	}
}

func TestTelegramDialogFailureResponses(t *testing.T) {
	message := func(text string, id int) *http.Request {
		return telegramRequest(`{"message":{"message_id":` + fmt.Sprint(id) + `,"text":"` + text + `","chat":{"id":77,"type":"private"},"from":{}}}`)
	}
	tests := []struct {
		name, text, expected string
		accounts             *telegramAccountAuthFake
	}{
		{"start failure", "/start register", "Не удалось начать", &telegramAccountAuthFake{beginErr: errors.New("unavailable")}},
		{"expired", "value", "не найдена или истекла", &telegramAccountAuthFake{stepErr: errors.New("expired")}},
		{"login failure", "sveta.design", "Сессия подтверждения истекла", &telegramAccountAuthFake{step: "awaiting_login", submitErr: errors.New("expired")}},
		{"rate limit", "password", "Слишком много попыток", &telegramAccountAuthFake{step: "awaiting_password", completeErr: account.RateLimitError{RetryAfter: time.Minute}}},
		{"unknown state", "value", "Сессия подтверждения недоступна", &telegramAccountAuthFake{step: "unexpected"}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			NewTelegramWebhook("webhook-secret", test.accounts, nil).ServeHTTP(response, message(test.text, 30+index))
			if !strings.Contains(telegramResponse(t, response)["text"].(string), test.expected) {
				t.Fatalf("body=%s", response.Body.String())
			}
		})
	}

	for _, input := range []string{"/start confirm", "Подтвердить профиль", "/confirm"} {
		response := httptest.NewRecorder()
		accounts := &telegramAccountAuthFake{}
		NewTelegramWebhook("webhook-secret", accounts, nil).ServeHTTP(response, message(input, 50))
		if accounts.beginChat != 77 {
			t.Fatalf("legacy start %q did not begin confirmation", input)
		}
	}

	nilService := httptest.NewRecorder()
	NewTelegramWebhook("webhook-secret", nil, nil).ServeHTTP(nilService, message("hello", 60))
	if !strings.Contains(telegramResponse(t, nilService)["text"].(string), "временно недоступен") {
		t.Fatalf("body=%s", nilService.Body.String())
	}
}
