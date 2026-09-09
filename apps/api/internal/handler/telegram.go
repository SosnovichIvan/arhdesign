package handler

import (
	"crypto/subtle"
	"encoding/json"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
	"net/http"
	"strings"
	"sync"
	"time"
)

type dialog struct {
	login    string
	password bool
	until    time.Time
}
type TelegramWebhook struct {
	secret, username, password string
	store                      repository.TelegramSubscriberStore
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

func NewTelegramWebhook(secret, username, password string, store repository.TelegramSubscriberStore) *TelegramWebhook {
	return &TelegramWebhook{secret: secret, username: username, password: password, store: store, dialogs: map[int64]dialog{}}
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
		e.reply(w, m.Chat.ID, active, "Меню уведомлений")
		return
	}
	if !active && (text == "Подписаться" || text == "/subscribe") {
		e.set(m.Chat.ID, dialog{until: time.Now().Add(5 * time.Minute)})
		e.send(w, m.Chat.ID, "Введите логин:")
		return
	}
	if active && (text == "Отписаться" || text == "/unsubscribe") {
		_ = e.store.DeactivateTelegramSubscriber(r.Context(), m.Chat.ID)
		e.clear(m.Chat.ID)
		e.reply(w, m.Chat.ID, false, "Подписка отключена.")
		return
	}
	d, ok := e.get(m.Chat.ID)
	if !ok {
		e.reply(w, m.Chat.ID, active, "Выберите действие в меню.")
		return
	}
	if !d.password {
		d.login = text
		d.password = true
		e.set(m.Chat.ID, d)
		e.send(w, m.Chat.ID, "Введите пароль:")
		return
	}
	e.clear(m.Chat.ID)
	if subtle.ConstantTimeCompare([]byte(d.login), []byte(e.username)) == 1 && subtle.ConstantTimeCompare([]byte(text), []byte(e.password)) == 1 {
		if e.store.ActivateTelegramSubscriber(r.Context(), m.Chat.ID, m.From.Username) == nil {
			e.reply(w, m.Chat.ID, true, "Вы подписались на новые заявки.")
		} else {
			e.reply(w, m.Chat.ID, false, "Не удалось включить подписку.")
		}
	} else {
		e.reply(w, m.Chat.ID, false, "Ошибка: логин или пароль не совпадают.")
	}
}
func (e *TelegramWebhook) send(w http.ResponseWriter, id int64, text string) {
	e.respond(w, id, text, "")
}
func (e *TelegramWebhook) reply(w http.ResponseWriter, id int64, active bool, text string) {
	action := "Подписаться"
	if active {
		action = "Отписаться"
	}
	e.respond(w, id, text+"\n\nДоступно: "+action, action)
}
func (e *TelegramWebhook) respond(w http.ResponseWriter, id int64, text, action string) {
	payload := map[string]any{"method": "sendMessage", "chat_id": id, "text": text}
	if action != "" {
		payload["reply_markup"] = map[string]any{"keyboard": [][]string{{action}}, "resize_keyboard": true}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
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
