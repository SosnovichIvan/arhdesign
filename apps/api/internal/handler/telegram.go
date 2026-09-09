package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
	"net/http"
	"strings"
	"sync"
	"time"
)

type telegramMessageSender interface {
	SendMessage(context.Context, int64, string, bool, bool) error
}
type dialog struct {
	login    string
	password bool
	until    time.Time
}
type TelegramWebhook struct {
	secret, username, password string
	store                      repository.TelegramSubscriberStore
	sender                     telegramMessageSender
	dialogs                    map[int64]dialog
	mu                         sync.Mutex
}
type telegramUpdate struct {
	Message *struct {
		Text string `json:"text"`
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		From struct {
			Username string `json:"username"`
		} `json:"from"`
	} `json:"message"`
}

func NewTelegramWebhook(secret, username, password string, store repository.TelegramSubscriberStore, sender telegramMessageSender) *TelegramWebhook {
	return &TelegramWebhook{secret: secret, username: username, password: password, store: store, sender: sender, dialogs: map[int64]dialog{}}
}
func (e *TelegramWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(e.secret)) != 1 {
		w.WriteHeader(401)
		return
	}
	var u telegramUpdate
	if json.NewDecoder(r.Body).Decode(&u) != nil || u.Message == nil || u.Message.Chat.Type != "private" {
		w.WriteHeader(200)
		return
	}
	m := u.Message
	text := strings.TrimSpace(m.Text)
	active, err := e.store.TelegramSubscriberActive(r.Context(), m.Chat.ID)
	if err != nil {
		w.WriteHeader(200)
		return
	}
	if text == "/start" || text == "/menu" {
		e.clear(m.Chat.ID)
		e.reply(r.Context(), m.Chat.ID, active, "Меню уведомлений")
		w.WriteHeader(200)
		return
	}
	if !active && (text == "Подписаться" || text == "/subscribe") {
		e.set(m.Chat.ID, dialog{until: time.Now().Add(5 * time.Minute)})
		e.send(r.Context(), m.Chat.ID, "Введите логин:", false)
		w.WriteHeader(200)
		return
	}
	if active && (text == "Отписаться" || text == "/unsubscribe") {
		_ = e.store.DeactivateTelegramSubscriber(r.Context(), m.Chat.ID)
		e.clear(m.Chat.ID)
		e.reply(r.Context(), m.Chat.ID, false, "Подписка отключена.")
		w.WriteHeader(200)
		return
	}
	d, ok := e.get(m.Chat.ID)
	if !ok {
		e.reply(r.Context(), m.Chat.ID, active, "Выберите действие в меню.")
		w.WriteHeader(200)
		return
	}
	if !d.password {
		d.login = text
		d.password = true
		e.set(m.Chat.ID, d)
		e.send(r.Context(), m.Chat.ID, "Введите пароль:", false)
		w.WriteHeader(200)
		return
	}
	e.clear(m.Chat.ID)
	if subtle.ConstantTimeCompare([]byte(d.login), []byte(e.username)) == 1 && subtle.ConstantTimeCompare([]byte(text), []byte(e.password)) == 1 {
		if e.store.ActivateTelegramSubscriber(r.Context(), m.Chat.ID, m.From.Username) == nil {
			e.reply(r.Context(), m.Chat.ID, true, "Вы подписались на новые заявки.")
		} else {
			e.reply(r.Context(), m.Chat.ID, false, "Не удалось включить подписку.")
		}
	} else {
		e.reply(r.Context(), m.Chat.ID, false, "Ошибка: логин или пароль не совпадают.")
	}
	w.WriteHeader(200)
}
func (e *TelegramWebhook) send(c context.Context, id int64, text string, menu bool) {
	_ = e.sender.SendMessage(c, id, text, menu, false)
}
func (e *TelegramWebhook) reply(c context.Context, id int64, active bool, text string) {
	action := "Подписаться"
	if active {
		action = "Отписаться"
	}
	_ = e.sender.SendMessage(c, id, text+"\n\nДоступно: "+action, true, active)
}
func (e *TelegramWebhook) set(id int64, d dialog) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dialogs[id] = d
}
func (e *TelegramWebhook) clear(id int64) { e.mu.Lock(); defer e.mu.Unlock(); delete(e.dialogs, id) }
func (e *TelegramWebhook) get(id int64) (dialog, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	d, ok := e.dialogs[id]
	if !ok || !d.until.After(time.Now()) {
		delete(e.dialogs, id)
		return dialog{}, false
	}
	return d, true
}
