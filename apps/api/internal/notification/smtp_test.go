package notification

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
)

func TestSMTPTransportSendsMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		_, _ = connection.Write([]byte("220 test SMTP\r\n"))
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, _ = connection.Write([]byte("250 test\r\n"))
			case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
				_, _ = connection.Write([]byte("250 ok\r\n"))
			case strings.HasPrefix(line, "DATA"):
				_, _ = connection.Write([]byte("354 send content\r\n"))
				for {
					dataLine, dataErr := reader.ReadString('\n')
					if dataErr != nil || dataLine == ".\r\n" {
						break
					}
				}
				_, _ = connection.Write([]byte("250 queued\r\n"))
			case strings.HasPrefix(line, "QUIT"):
				_, _ = connection.Write([]byte("221 bye\r\n"))
				return
			}
		}
	}()

	transport := NewSMTPTransport(listener.Addr().String(), "", "")
	if err := transport.Send(context.Background(), EmailMessage{From: "site@example.com", To: "owner@example.com", Subject: "Test", Text: "Hello"}); err != nil {
		t.Fatal(err)
	}
	<-finished
}

func TestSMTPTransportReturnsDialError(t *testing.T) {
	transport := NewSMTPTransport("127.0.0.1:1", "", "")
	if err := transport.Send(context.Background(), EmailMessage{}); err == nil {
		t.Fatal("unavailable SMTP server must return an error")
	}
}

func TestSMTPTransportString(t *testing.T) {
	if NewSMTPTransport(" smtp.example.com:587 ", "", "").String() != "smtp.example.com:587" {
		t.Fatal("transport string must be trimmed")
	}
}
