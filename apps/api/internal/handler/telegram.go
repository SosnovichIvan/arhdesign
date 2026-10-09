package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
)

type TelegramAccountAuth interface {
	BeginTelegramConfirmation(context.Context, int64, string) error
	TelegramConfirmationStep(context.Context, int64) (string, error)
	SubmitTelegramLogin(context.Context, int64, string) error
	CompleteTelegramConfirmation(context.Context, int64, int64, string, string, string) error
}

type TelegramMessageDeleter interface {
	DeleteMessage(context.Context, int64, int64) error
}

type TelegramWebhook struct {
	secret   string
	accounts TelegramAccountAuth
	deleter  TelegramMessageDeleter
}

type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64  `json:"message_id"`
		Text      string `json:"text"`
		Chat      struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		From struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"from"`
	} `json:"message"`
}

func NewTelegramWebhook(secret string, accounts TelegramAccountAuth, deleter TelegramMessageDeleter) *TelegramWebhook {
	return &TelegramWebhook{secret: secret, accounts: accounts, deleter: deleter}
}

func (endpoint *TelegramWebhook) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if endpoint.secret == "" || subtle.ConstantTimeCompare([]byte(request.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(endpoint.secret)) != 1 {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	var update telegramUpdate
	if json.NewDecoder(request.Body).Decode(&update) != nil || update.Message == nil || update.Message.Chat.Type != "private" {
		writer.WriteHeader(http.StatusOK)
		return
	}
	message := update.Message
	text := strings.TrimSpace(message.Text)
	requestID := fmt.Sprintf("telegram-%d-%d", message.Chat.ID, message.MessageID)

	if text == "/start register" || text == "/start confirm" || text == "Подтвердить профиль" || text == "/confirm" {
		if endpoint.accounts == nil || endpoint.accounts.BeginTelegramConfirmation(request.Context(), message.Chat.ID, requestID) != nil {
			endpoint.respond(writer, message.Chat.ID, "Не удалось начать подтверждение. Попробуйте позже.", "")
			return
		}
		endpoint.respond(writer, message.Chat.ID, "Введите логин, указанный при регистрации на designer-svetlana.ru:", "")
		return
	}
	if text == "/start" || text == "/menu" {
		welcome := "Добро пожаловать"
		if firstName := telegramFirstName(message.From.FirstName); firstName != "" {
			welcome += " " + firstName
		}
		endpoint.respond(writer, message.Chat.ID, welcome+"\n\nЭто бот личного кабинета Светланы Полисмаковой. Здесь можно подтвердить профиль и получать уведомления о событиях, которые относятся к вашим проектам.", "Подтвердить профиль")
		return
	}
	if endpoint.accounts == nil {
		endpoint.respond(writer, message.Chat.ID, "Сервис подтверждения временно недоступен.", "Подтвердить профиль")
		return
	}
	step, err := endpoint.accounts.TelegramConfirmationStep(request.Context(), message.Chat.ID)
	if err != nil {
		endpoint.respond(writer, message.Chat.ID, "Сессия подтверждения не найдена или истекла. Запустите подтверждение снова.", "Подтвердить профиль")
		return
	}
	switch step {
	case "awaiting_login":
		if err := endpoint.accounts.SubmitTelegramLogin(request.Context(), message.Chat.ID, text); err != nil {
			endpoint.respond(writer, message.Chat.ID, "Сессия подтверждения истекла. Запустите подтверждение снова.", "Подтвердить профиль")
			return
		}
		endpoint.respond(writer, message.Chat.ID, "Введите пароль. Сообщение с паролем будет удалено сразу после проверки:", "")
	case "awaiting_password":
		if endpoint.deleter != nil {
			_ = endpoint.deleter.DeleteMessage(request.Context(), message.Chat.ID, message.MessageID)
		}
		err := endpoint.accounts.CompleteTelegramConfirmation(request.Context(), message.Chat.ID, message.MessageID, message.From.Username, text, requestID)
		if err == nil {
			endpoint.respond(writer, message.Chat.ID, "Профиль подтверждён. Telegram подключён к вашей учётной записи.", "")
			return
		}
		var limited account.RateLimitError
		if errors.As(err, &limited) {
			endpoint.respond(writer, message.Chat.ID, "Слишком много попыток. Повторите позже.", "Подтвердить профиль")
			return
		}
		endpoint.respond(writer, message.Chat.ID, "Не удалось подтвердить профиль. Проверьте логин и пароль и начните заново.", "Подтвердить профиль")
	default:
		endpoint.respond(writer, message.Chat.ID, "Сессия подтверждения недоступна. Запустите подтверждение снова.", "Подтвердить профиль")
	}
}

func telegramFirstName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 64 || strings.ContainsAny(value, "\r\n\t") {
		return ""
	}
	return value
}

func (endpoint *TelegramWebhook) respond(writer http.ResponseWriter, chatID int64, text, action string) {
	payload := map[string]any{"method": "sendMessage", "chat_id": chatID, "text": text}
	if action != "" {
		payload["reply_markup"] = map[string]any{"keyboard": [][]string{{action}}, "resize_keyboard": true, "is_persistent": true}
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(payload)
}
