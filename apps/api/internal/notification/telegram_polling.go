package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type TelegramUpdate struct {
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

type TelegramReply struct {
	ChatID int64
	Text   string
	Action string
}

type TelegramUpdateProcessor interface {
	ProcessTelegramUpdate(context.Context, TelegramUpdate) TelegramReply
}

type TelegramPoller struct {
	client    HTTPClient
	endpoint  string
	logger    *slog.Logger
	processor TelegramUpdateProcessor
	sender    Telegram
}

func NewTelegramPoller(token string, client HTTPClient, processor TelegramUpdateProcessor, logger *slog.Logger) (*TelegramPoller, error) {
	if token == "" || client == nil || processor == nil {
		return nil, fmt.Errorf("Telegram poller requires token, client and processor")
	}
	if logger == nil {
		logger = slog.Default()
	}
	endpoint := "https://api.telegram.org/bot" + token
	return &TelegramPoller{client: client, endpoint: endpoint, logger: logger, processor: processor, sender: NewTelegram(token, "", client)}, nil
}

func (poller *TelegramPoller) Run(ctx context.Context) {
	offset := int64(0)
	backoff := time.Second
	webhookDeleted := false
	for ctx.Err() == nil {
		if !webhookDeleted {
			requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
			err := poller.deleteWebhook(requestContext)
			cancel()
			if err != nil {
				poller.logger.Warn("Telegram polling setup failed", "error_class", "telegram_proxy")
				if !waitTelegramRetry(ctx, backoff) {
					return
				}
				backoff = nextTelegramPollingBackoff(backoff)
				continue
			}
			webhookDeleted = true
			backoff = time.Second
		}

		updates, err := poller.fetch(ctx, offset)
		if err != nil {
			poller.logger.Warn("Telegram polling request failed", "error_class", "telegram_proxy")
			if !waitTelegramRetry(ctx, backoff) {
				return
			}
			backoff = nextTelegramPollingBackoff(backoff)
			continue
		}
		backoff = time.Second
		for _, update := range updates {
			reply := poller.processor.ProcessTelegramUpdate(ctx, update)
			if reply.Text != "" {
				if err := poller.sender.SendAction(ctx, reply.ChatID, reply.Text, reply.Action); err != nil {
					poller.logger.Warn("Telegram polling response failed", "error_class", "telegram_proxy")
				}
			}
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
		}
	}
}

func (poller *TelegramPoller) deleteWebhook(ctx context.Context) error {
	return poller.call(ctx, "/deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}

func (poller *TelegramPoller) fetch(ctx context.Context, offset int64) ([]TelegramUpdate, error) {
	var response struct {
		Result []TelegramUpdate `json:"result"`
	}
	err := poller.call(ctx, "/getUpdates", map[string]any{"offset": offset, "timeout": 25, "allowed_updates": []string{"message"}}, &response)
	return response.Result, err
}

func (poller *TelegramPoller) call(ctx context.Context, method string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Telegram request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, poller.endpoint+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Telegram request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := poller.client.Do(request)
	if err != nil {
		return fmt.Errorf("Telegram transport unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return fmt.Errorf("Telegram returned a non-success response")
	}
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || !envelope.OK {
		return fmt.Errorf("Telegram returned an invalid response")
	}
	if target != nil {
		wrapped, err := json.Marshal(map[string]json.RawMessage{"result": envelope.Result})
		if err != nil || json.Unmarshal(wrapped, target) != nil {
			return fmt.Errorf("Telegram returned an invalid result")
		}
	}
	return nil
}

func nextTelegramPollingBackoff(current time.Duration) time.Duration {
	if current >= 30*time.Second {
		return 30 * time.Second
	}
	return min(current*2, 30*time.Second)
}

func waitTelegramRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
