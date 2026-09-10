package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Submission struct {
	Name           string
	Contact        string
	ProjectType    string
	ProjectDetails string
}

type Sender interface {
	Name() string
	Send(context.Context, Submission) error
}

type Dispatcher struct {
	logger  *slog.Logger
	senders []Sender
	timeout time.Duration
}

func NewDispatcher(logger *slog.Logger, timeout time.Duration, senders ...Sender) *Dispatcher {
	return &Dispatcher{logger: logger, senders: senders, timeout: timeout}
}

func (dispatcher *Dispatcher) Notify(parent context.Context, submission Submission) {
	for _, sender := range dispatcher.senders {
		context, cancel := context.WithTimeout(parent, dispatcher.timeout)
		err := sender.Send(context, submission)
		cancel()
		if err != nil {
			dispatcher.logger.Warn("contact notification failed", "adapter", sender.Name(), "error_class", fmt.Sprintf("%T", err))
		}
	}
}

type NoopDispatcher struct{}

func (NoopDispatcher) Notify(context.Context, Submission) {}
