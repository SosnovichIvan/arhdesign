package notification

import (
	"context"
	"fmt"
)

type EmailMessage struct {
	From    string
	To      string
	Subject string
	Text    string
}

type EmailTransport interface {
	Send(context.Context, EmailMessage) error
}

type Email struct {
	from      string
	recipient string
	transport EmailTransport
}

func NewEmail(from string, recipient string, transport EmailTransport) Email {
	return Email{from: from, recipient: recipient, transport: transport}
}

func (Email) Name() string { return "email" }

func (adapter Email) Send(ctx context.Context, submission Submission) error {
	return adapter.transport.Send(ctx, EmailMessage{
		From: adapter.from, To: adapter.recipient, Subject: "Новая заявка с сайта", Text: formatSubmission(submission),
	})
}

func formatSubmission(submission Submission) string {
	return fmt.Sprintf("Имя: %s\nКонтакт: %s\nТип проекта: %s\nО проекте: %s", submission.Name, submission.Contact, submission.ProjectType, submission.ProjectDetails)
}
