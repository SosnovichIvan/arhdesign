package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Telegram struct {
	chatID string
	client HTTPClient
	url    string
}

type TelegramSubscriberStore interface {
	ActiveTelegramChatIDs(context.Context) ([]int64, error)
}

type TelegramSubscribers struct {
	token  string
	store  TelegramSubscriberStore
	client HTTPClient
}

func NewTelegramSubscribers(token string, store TelegramSubscriberStore, client HTTPClient) TelegramSubscribers {
	return TelegramSubscribers{token: token, store: store, client: client}
}
func (TelegramSubscribers) Name() string { return "telegram" }
func (adapter TelegramSubscribers) Send(ctx context.Context, submission Submission) error {
	chatIDs, err := adapter.store.ActiveTelegramChatIDs(ctx)
	if err != nil {
		return err
	}
	for _, chatID := range chatIDs {
		if err := (Telegram{chatID: fmt.Sprint(chatID), client: adapter.client, url: "https://api.telegram.org/bot" + adapter.token + "/sendMessage"}).Send(ctx, submission); err != nil {
			return err
		}
	}
	return nil
}

func NewTelegram(token string, chatID string, client HTTPClient) Telegram {
	return Telegram{chatID: chatID, client: client, url: "https://api.telegram.org/bot" + token + "/sendMessage"}
}

func (Telegram) Name() string { return "telegram" }

func (adapter Telegram) Send(ctx context.Context, submission Submission) error {
	return adapter.send(ctx, adapter.chatID, "Новая заявка с сайта\n"+formatSubmission(submission), false, false)
}

func (adapter Telegram) SendMessage(ctx context.Context, chatID int64, text string, menu, active bool) error {
	return adapter.send(ctx, fmt.Sprint(chatID), text, menu, active)
}

func (adapter Telegram) send(ctx context.Context, chatID, text string, menu, active bool) error {
	payload := map[string]any{"chat_id": chatID, "text": text}
	if menu {
		action := "Подписаться"
		if active {
			action = "Отписаться"
		}
		payload["reply_markup"] = map[string]any{"keyboard": [][]string{{action}}, "resize_keyboard": true}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, adapter.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := adapter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return fmt.Errorf("telegram returned HTTP %d", response.StatusCode)
	}
	return nil
}
