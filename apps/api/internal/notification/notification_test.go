package notification

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeSender struct {
	error  error
	name   string
	called bool
}

func (sender *fakeSender) Name() string { return sender.name }

func (sender *fakeSender) Send(ctx context.Context, _ Submission) error {
	sender.called = true
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("notification context has no deadline")
	}
	return sender.error
}

func TestDispatcherIsolatesAdapterErrors(t *testing.T) {
	failing := &fakeSender{name: "email", error: errors.New("mail unavailable")}
	succeeding := &fakeSender{name: "telegram"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := NewDispatcher(logger, time.Second, failing, succeeding)

	dispatcher.Notify(context.Background(), Submission{Name: "Анна"})

	if !failing.called || !succeeding.called {
		t.Fatal("every adapter must be called even if a previous adapter fails")
	}
}

func TestNoopDispatcher(t *testing.T) {
	NoopDispatcher{}.Notify(context.Background(), Submission{})
}

type fakeEmailTransport struct{ message EmailMessage }

func (transport *fakeEmailTransport) Send(_ context.Context, message EmailMessage) error {
	transport.message = message
	return nil
}

func TestEmailFormatsSubmission(t *testing.T) {
	transport := &fakeEmailTransport{}
	adapter := NewEmail("site@example.com", "owner@example.com", transport)
	if err := adapter.Send(context.Background(), Submission{Name: "Анна", Contact: "anna@example.com", ProjectType: "Квартира", ProjectDetails: "Нужен проект"}); err != nil {
		t.Fatal(err)
	}
	if transport.message.To != "owner@example.com" || transport.message.Subject != "Новая заявка с сайта" {
		t.Fatalf("unexpected email: %#v", transport.message)
	}
}

func TestAdapterNamesAndNoopDispatcher(t *testing.T) {
	if NewEmail("site@example.com", "owner@example.com", &fakeEmailTransport{}).Name() != "email" {
		t.Fatal("email adapter must expose its name")
	}
	(NoopDispatcher{}).Notify(context.Background(), Submission{})
}
