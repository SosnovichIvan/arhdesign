package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Telegram struct {
	authorization string
	chatID        string
	client        HTTPClient
	url           string
}

type TelegramSubscriberStore interface {
	ActiveTelegramChatIDs(context.Context) ([]int64, error)
	RecordTelegramNotification(context.Context, int64, int64, time.Time) error
}

type TelegramSubscribers struct {
	authorization string
	store         TelegramSubscriberStore
	client        HTTPClient
	retention     time.Duration
	url           string
}

func NewTelegramSubscribers(token string, store TelegramSubscriberStore, client HTTPClient, retention time.Duration) TelegramSubscribers {
	return TelegramSubscribers{store: store, client: client, retention: retention, url: "https://api.telegram.org/bot" + token + "/sendMessage"}
}

func NewTelegramSubscribersViaRelay(relayURL, relaySecret string, store TelegramSubscriberStore, client HTTPClient, retention time.Duration) TelegramSubscribers {
	return TelegramSubscribers{authorization: "Bearer " + relaySecret, store: store, client: client, retention: retention, url: relayURL}
}
func (TelegramSubscribers) Name() string { return "telegram" }
func (adapter TelegramSubscribers) Send(ctx context.Context, submission Submission) error {
	chatIDs, err := adapter.store.ActiveTelegramChatIDs(ctx)
	if err != nil {
		return err
	}
	for _, chatID := range chatIDs {
		telegram := Telegram{authorization: adapter.authorization, chatID: fmt.Sprint(chatID), client: adapter.client, url: adapter.url}
		messageID, err := telegram.send(ctx, telegram.chatID, "Новая заявка с сайта\n"+formatSubmission(submission), false, false)
		if err != nil {
			return err
		}
		if err := adapter.store.RecordTelegramNotification(ctx, chatID, messageID, time.Now().Add(adapter.retention)); err != nil {
			_ = telegram.DeleteMessage(ctx, chatID, messageID)
			return err
		}
	}
	return nil
}

func NewTelegram(token string, chatID string, client HTTPClient) Telegram {
	return Telegram{chatID: chatID, client: client, url: "https://api.telegram.org/bot" + token + "/sendMessage"}
}

func NewTelegramViaRelay(relayURL, relaySecret string, client HTTPClient) Telegram {
	return Telegram{authorization: "Bearer " + relaySecret, client: client, url: relayURL}
}

func (Telegram) Name() string { return "telegram" }

func (adapter Telegram) Send(ctx context.Context, submission Submission) error {
	_, err := adapter.send(ctx, adapter.chatID, "Новая заявка с сайта\n"+formatSubmission(submission), false, false)
	return err
}

func (adapter Telegram) SendMessage(ctx context.Context, chatID int64, text string, menu, active bool) error {
	_, err := adapter.send(ctx, fmt.Sprint(chatID), text, menu, active)
	return err
}

func (adapter Telegram) send(ctx context.Context, chatID, text string, menu, active bool) (int64, error) {
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
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, adapter.url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	if adapter.authorization != "" {
		request.Header.Set("Authorization", adapter.authorization)
	}
	response, err := adapter.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return 0, fmt.Errorf("telegram returned HTTP %d", response.StatusCode)
	}
	var telegramResponse struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&telegramResponse); err != nil || !telegramResponse.OK || telegramResponse.Result.MessageID < 1 {
		return 0, fmt.Errorf("telegram returned an invalid success response")
	}
	return telegramResponse.Result.MessageID, nil
}

func (adapter Telegram) DeleteMessage(ctx context.Context, chatID, messageID int64) error {
	payload, err := json.Marshal(map[string]any{"chat_id": chatID, "message_id": messageID})
	if err != nil {
		return err
	}
	deleteURL := strings.TrimSuffix(adapter.url, "/sendMessage") + "/deleteMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, deleteURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if adapter.authorization != "" {
		request.Header.Set("Authorization", adapter.authorization)
	}
	response, err := adapter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("telegram delete returned HTTP %d", response.StatusCode)
	}
	return nil
}
