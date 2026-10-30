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
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/notification"
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
	secret       string
	conversation *TelegramConversation
}

type TelegramConversation struct {
	accounts TelegramAccountAuth
	deleter  TelegramMessageDeleter
}

func NewTelegramConversation(accounts TelegramAccountAuth, deleter TelegramMessageDeleter) *TelegramConversation {
	return &TelegramConversation{accounts: accounts, deleter: deleter}
}

func NewTelegramWebhook(secret string, accounts TelegramAccountAuth, deleter TelegramMessageDeleter) *TelegramWebhook {
	return NewTelegramWebhookWithConversation(secret, NewTelegramConversation(accounts, deleter))
}

func NewTelegramWebhookWithConversation(secret string, conversation *TelegramConversation) *TelegramWebhook {
	return &TelegramWebhook{secret: secret, conversation: conversation}
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
	var update notification.TelegramUpdate
	if json.NewDecoder(request.Body).Decode(&update) != nil || update.Message == nil || update.Message.Chat.Type != "private" {
		writer.WriteHeader(http.StatusOK)
		return
	}
	reply := endpoint.conversation.ProcessTelegramUpdate(request.Context(), update)
	if reply.Text == "" {
		writer.WriteHeader(http.StatusOK)
		return
	}
	writeTelegramWebhookReply(writer, reply)
}

func (endpoint *TelegramConversation) ProcessTelegramUpdate(ctx context.Context, update notification.TelegramUpdate) notification.TelegramReply {
	if update.Message == nil || update.Message.Chat.Type != "private" {
		return notification.TelegramReply{}
	}
	message := update.Message
	text := strings.TrimSpace(message.Text)
	requestID := fmt.Sprintf("telegram-%d-%d", message.Chat.ID, message.MessageID)
	reply := func(text, action string) notification.TelegramReply {
		return notification.TelegramReply{ChatID: message.Chat.ID, Text: text, Action: action}
	}

	if text == "/start register" || text == "/start confirm" || text == "Подтвердить профиль" || text == "/confirm" {
		if endpoint.accounts == nil || endpoint.accounts.BeginTelegramConfirmation(ctx, message.Chat.ID, requestID) != nil {
			return reply("Не удалось начать подтверждение. Попробуйте позже.", "")
		}
		return reply("Введите логин, указанный при регистрации на designer-svetlana.ru:", "")
	}
	if text == "/start" || text == "/menu" {
		welcome := "Добро пожаловать"
		if firstName := telegramFirstName(message.From.FirstName); firstName != "" {
			welcome += " " + firstName
		}
		return reply(welcome+"\n\nЭто бот личного кабинета Светланы Полисмаковой. Здесь можно подтвердить профиль и получать уведомления о событиях, которые относятся к вашим проектам.", "Подтвердить профиль")
	}
	if endpoint.accounts == nil {
		return reply("Сервис подтверждения временно недоступен.", "Подтвердить профиль")
	}
	step, err := endpoint.accounts.TelegramConfirmationStep(ctx, message.Chat.ID)
	if err != nil {
		return reply("Сессия подтверждения не найдена или истекла. Запустите подтверждение снова.", "Подтвердить профиль")
	}
	switch step {
	case "awaiting_login":
		if err := endpoint.accounts.SubmitTelegramLogin(ctx, message.Chat.ID, text); err != nil {
			return reply("Сессия подтверждения истекла. Запустите подтверждение снова.", "Подтвердить профиль")
		}
		return reply("Введите пароль. Сообщение с паролем будет удалено сразу после проверки:", "")
	case "awaiting_password":
		if endpoint.deleter != nil {
			_ = endpoint.deleter.DeleteMessage(ctx, message.Chat.ID, message.MessageID)
		}
		err := endpoint.accounts.CompleteTelegramConfirmation(ctx, message.Chat.ID, message.MessageID, message.From.Username, text, requestID)
		if err == nil {
			return reply("Профиль подтверждён. Telegram подключён к вашей учётной записи.", "")
		}
		var limited account.RateLimitError
		if errors.As(err, &limited) {
			return reply("Слишком много попыток. Повторите позже.", "Подтвердить профиль")
		}
		return reply("Не удалось подтвердить профиль. Проверьте логин и пароль и начните заново.", "Подтвердить профиль")
	default:
		return reply("Сессия подтверждения недоступна. Запустите подтверждение снова.", "Подтвердить профиль")
	}
}

func telegramFirstName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 64 || strings.ContainsAny(value, "\r\n\t") {
		return ""
	}
	return value
}

func writeTelegramWebhookReply(writer http.ResponseWriter, reply notification.TelegramReply) {
	payload := map[string]any{"method": "sendMessage", "chat_id": reply.ChatID, "text": reply.Text}
	if reply.Action != "" {
		payload["reply_markup"] = map[string]any{"keyboard": [][]string{{reply.Action}}, "resize_keyboard": true, "is_persistent": true}
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(payload)
}
