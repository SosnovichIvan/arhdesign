package notification

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type SMTPTransport struct {
	address     string
	host        string
	username    string
	password    string
	implicitTLS bool
	tlsConfig   *tls.Config
}

func NewSMTPTransport(address string, username string, password string) SMTPTransport {
	address = strings.TrimSpace(address)
	host, port, _ := net.SplitHostPort(address)
	return SMTPTransport{address: address, host: host, username: username, password: password, implicitTLS: port == "465"}
}

func (transport SMTPTransport) Send(ctx context.Context, message EmailMessage) error {
	if transport.host == "" {
		return fmt.Errorf("SMTP address must include host and port")
	}
	from, err := mail.ParseAddress(message.From)
	if err != nil {
		return fmt.Errorf("invalid email sender: %w", err)
	}
	to, err := mail.ParseAddress(message.To)
	if err != nil {
		return fmt.Errorf("invalid email recipient: %w", err)
	}
	payload, err := encodeSMTPMessage(message, time.Now())
	if err != nil {
		return err
	}
	dialer := net.Dialer{}
	tlsConfig := transport.tlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{ServerName: transport.host, MinVersion: tls.VersionTLS12}
	}
	var connection net.Conn
	if transport.implicitTLS {
		connection, err = (&tls.Dialer{NetDialer: &dialer, Config: tlsConfig}).DialContext(ctx, "tcp", transport.address)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", transport.address)
	}
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(connection, transport.host)
	if err != nil {
		return err
	}
	defer client.Quit()
	encrypted := transport.implicitTLS
	if !encrypted {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return err
			}
			encrypted = true
		}
	}
	if transport.username != "" {
		if !encrypted {
			return fmt.Errorf("SMTP authentication requires TLS")
		}
		if ok, _ := client.Extension("AUTH"); !ok {
			return fmt.Errorf("SMTP server does not support AUTH")
		}
		if err := client.Auth(smtp.PlainAuth("", transport.username, transport.password, transport.host)); err != nil {
			return err
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return err
	}
	if err := client.Rcpt(to.Address); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	_, writeErr := writer.Write(payload)
	if closeErr := writer.Close(); writeErr != nil {
		return writeErr
	} else if closeErr != nil {
		return closeErr
	}
	return nil
}

func encodeSMTPMessage(message EmailMessage, now time.Time) ([]byte, error) {
	from, err := mail.ParseAddress(message.From)
	if err != nil {
		return nil, fmt.Errorf("invalid email sender: %w", err)
	}
	to, err := mail.ParseAddress(message.To)
	if err != nil {
		return nil, fmt.Errorf("invalid email recipient: %w", err)
	}
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return nil, fmt.Errorf("generate email message ID: %w", err)
	}
	domain := "localhost"
	if at := strings.LastIndexByte(from.Address, '@'); at >= 0 && at < len(from.Address)-1 {
		domain = from.Address[at+1:]
	}

	var payload bytes.Buffer
	fmt.Fprintf(&payload, "From: %s\r\n", from.String())
	fmt.Fprintf(&payload, "To: %s\r\n", to.String())
	fmt.Fprintf(&payload, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", message.Subject))
	fmt.Fprintf(&payload, "Date: %s\r\n", now.Format(time.RFC1123Z))
	fmt.Fprintf(&payload, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(randomID), domain)
	payload.WriteString("MIME-Version: 1.0\r\n")
	payload.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	payload.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	body := quotedprintable.NewWriter(&payload)
	if _, err := body.Write([]byte(normalizeCRLF(message.Text))); err != nil {
		return nil, fmt.Errorf("encode email body: %w", err)
	}
	if err := body.Close(); err != nil {
		return nil, fmt.Errorf("finalize email body: %w", err)
	}
	return payload.Bytes(), nil
}

func normalizeCRLF(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}

func (transport SMTPTransport) String() string {
	return strings.TrimSpace(transport.address)
}
