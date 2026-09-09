package notification

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

type SMTPTransport struct {
	address  string
	host     string
	username string
	password string
}

func NewSMTPTransport(address string, username string, password string) SMTPTransport {
	host, _, _ := net.SplitHostPort(address)
	return SMTPTransport{address: address, host: host, username: username, password: password}
}

func (transport SMTPTransport) Send(ctx context.Context, message EmailMessage) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "tcp", transport.address)
	if err != nil {
		return err
	}
	defer connection.Close()
	client, err := smtp.NewClient(connection, transport.host)
	if err != nil {
		return err
	}
	defer client.Quit()
	if transport.username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return fmt.Errorf("SMTP server does not support AUTH")
		}
		if err := client.Auth(smtp.PlainAuth("", transport.username, transport.password, transport.host)); err != nil {
			return err
		}
	}
	if err := client.Mail(message.From); err != nil {
		return err
	}
	if err := client.Rcpt(message.To); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	_, writeErr := writer.Write([]byte("Subject: " + message.Subject + "\r\n" + "Content-Type: text/plain; charset=UTF-8\r\n\r\n" + message.Text))
	if closeErr := writer.Close(); writeErr != nil {
		return writeErr
	} else if closeErr != nil {
		return closeErr
	}
	return nil
}

func (transport SMTPTransport) String() string {
	return strings.TrimSpace(transport.address)
}
