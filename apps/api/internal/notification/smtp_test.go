package notification

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := transport.Send(ctx, EmailMessage{From: "site@example.com", To: "owner@example.com", Subject: "Test", Text: "Hello"}); err != nil {
		t.Fatal(err)
	}
	<-finished
}

func TestSMTPTransportPropagatesProtocolStageFailures(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		failure  string
		contains string
	}{
		{name: "greeting", failure: "greeting", contains: "500"},
		{name: "starttls", failure: "starttls", contains: "454"},
		{name: "mail", failure: "mail", contains: "550"},
		{name: "recipient", failure: "recipient", contains: "551"},
		{name: "data", failure: "data", contains: "554"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
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
				if testCase.failure == "greeting" {
					_, _ = connection.Write([]byte("500 unavailable\r\n"))
					return
				}
				reader := bufio.NewReader(connection)
				_, _ = connection.Write([]byte("220 test SMTP\r\n"))
				for {
					line, readErr := reader.ReadString('\n')
					if readErr != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO") && testCase.failure == "starttls":
						_, _ = connection.Write([]byte("250-test\r\n250 STARTTLS\r\n"))
					case strings.HasPrefix(line, "STARTTLS"):
						_, _ = connection.Write([]byte("454 TLS unavailable\r\n"))
						return
					case strings.HasPrefix(line, "EHLO"):
						_, _ = connection.Write([]byte("250 test\r\n"))
					case strings.HasPrefix(line, "MAIL FROM") && testCase.failure == "mail":
						_, _ = connection.Write([]byte("550 sender rejected\r\n"))
						return
					case strings.HasPrefix(line, "MAIL FROM"):
						_, _ = connection.Write([]byte("250 ok\r\n"))
					case strings.HasPrefix(line, "RCPT TO") && testCase.failure == "recipient":
						_, _ = connection.Write([]byte("551 recipient rejected\r\n"))
						return
					case strings.HasPrefix(line, "RCPT TO"):
						_, _ = connection.Write([]byte("250 ok\r\n"))
					case strings.HasPrefix(line, "DATA"):
						_, _ = connection.Write([]byte("554 data rejected\r\n"))
						return
					}
				}
			}()
			transport := NewSMTPTransport(listener.Addr().String(), "", "")
			err = transport.Send(context.Background(), EmailMessage{From: "site@example.com", To: "owner@example.com", Subject: "Failure", Text: "Body"})
			if err == nil || !strings.Contains(err.Error(), testCase.contains) {
				t.Fatalf("error=%v", err)
			}
			<-finished
		})
	}
}

func TestSMTPTransportReturnsDialError(t *testing.T) {
	transport := NewSMTPTransport("127.0.0.1:1", "", "")
	if err := transport.Send(context.Background(), EmailMessage{From: "site@example.com", To: "owner@example.com"}); err == nil {
		t.Fatal("unavailable SMTP server must return an error")
	}
}

func TestSMTPTransportRejectsInvalidAddressesBeforeDial(t *testing.T) {
	transport := NewSMTPTransport("127.0.0.1:1", "", "")
	for name, message := range map[string]EmailMessage{
		"sender":    {From: "invalid sender", To: "owner@example.com"},
		"recipient": {From: "site@example.com", To: "invalid recipient"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := transport.Send(context.Background(), message); err == nil || !strings.Contains(err.Error(), "invalid email") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestSMTPTransportRequiresAuthSupport(t *testing.T) {
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
			if strings.HasPrefix(line, "EHLO") {
				_, _ = connection.Write([]byte("250 test\r\n"))
			} else if strings.HasPrefix(line, "QUIT") {
				_, _ = connection.Write([]byte("221 bye\r\n"))
				return
			}
		}
	}()

	transport := NewSMTPTransport(listener.Addr().String(), "admin", "password")
	if err := transport.Send(context.Background(), EmailMessage{From: "site@example.com", To: "owner@example.com"}); err == nil || !strings.Contains(err.Error(), "requires TLS") {
		t.Fatalf("error = %v; want TLS requirement error", err)
	}
	<-finished
}

func TestSMTPTransportUsesImplicitTLS(t *testing.T) {
	certificate, roots := smtpTestCertificate(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
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
		_, _ = connection.Write([]byte("220 secure test SMTP\r\n"))
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, _ = connection.Write([]byte("250-secure-test\r\n250 AUTH PLAIN\r\n"))
			case strings.HasPrefix(line, "AUTH PLAIN"):
				_, _ = connection.Write([]byte("235 authenticated\r\n"))
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
	transport := NewSMTPTransport(listener.Addr().String(), "no-reply@example.com", "test-password")
	transport.implicitTLS = true
	transport.tlsConfig = &tls.Config{ServerName: "localhost", RootCAs: roots, MinVersion: tls.VersionTLS12}
	if err := transport.Send(context.Background(), EmailMessage{From: "site@example.com", To: "owner@example.com", Subject: "TLS", Text: "secure"}); err != nil {
		t.Fatal(err)
	}
	<-finished
}

func TestSMTPTransportUpgradesWithSTARTTLS(t *testing.T) {
	certificate, roots := smtpTestCertificate(t)
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
		_, _ = connection.Write([]byte("220 STARTTLS test SMTP\r\n"))
		if line, _ := reader.ReadString('\n'); !strings.HasPrefix(line, "EHLO") {
			return
		}
		_, _ = connection.Write([]byte("250-test\r\n250 STARTTLS\r\n"))
		if line, _ := reader.ReadString('\n'); !strings.HasPrefix(line, "STARTTLS") {
			return
		}
		_, _ = connection.Write([]byte("220 begin TLS\r\n"))
		secure := tls.Server(connection, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
		if handshakeErr := secure.Handshake(); handshakeErr != nil {
			return
		}
		reader = bufio.NewReader(secure)
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, _ = secure.Write([]byte("250 secure-test\r\n"))
			case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
				_, _ = secure.Write([]byte("250 ok\r\n"))
			case strings.HasPrefix(line, "DATA"):
				_, _ = secure.Write([]byte("354 send content\r\n"))
				for {
					dataLine, dataErr := reader.ReadString('\n')
					if dataErr != nil || dataLine == ".\r\n" {
						break
					}
				}
				_, _ = secure.Write([]byte("250 queued\r\n"))
			case strings.HasPrefix(line, "QUIT"):
				_, _ = secure.Write([]byte("221 bye\r\n"))
				return
			}
		}
	}()
	transport := NewSMTPTransport(listener.Addr().String(), "", "")
	transport.tlsConfig = &tls.Config{ServerName: "localhost", RootCAs: roots, MinVersion: tls.VersionTLS12}
	if err := transport.Send(context.Background(), EmailMessage{From: "site@example.com", To: "owner@example.com", Subject: "STARTTLS", Text: "secure"}); err != nil {
		t.Fatal(err)
	}
	<-finished
}

func TestSMTPTransportRejectsAddressWithoutPort(t *testing.T) {
	transport := NewSMTPTransport("smtp.example.com", "", "")
	if err := transport.Send(context.Background(), EmailMessage{}); err == nil || !strings.Contains(err.Error(), "host and port") {
		t.Fatalf("error=%v", err)
	}
}

func TestEncodeSMTPMessageProducesDeliverableMIME(t *testing.T) {
	payload, err := encodeSMTPMessage(EmailMessage{
		From:    "no-reply@designer-svetlana.ru",
		To:      "user@example.com",
		Subject: "Подтвердите электронную почту — arhDesign",
		Text:    "Здравствуйте!\nВторая строка.",
	}, time.Date(2026, time.September, 24, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60)))
	if err != nil {
		t.Fatal(err)
	}
	message := string(payload)
	for _, expected := range []string{
		"From: <no-reply@designer-svetlana.ru>\r\n",
		"To: <user@example.com>\r\n",
		"Subject: =?UTF-8?q?",
		"Date: Thu, 24 Sep 2026 12:00:00 +0300\r\n",
		"Message-ID: <",
		"@designer-svetlana.ru>\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n",
		"=D0=97=D0=B4=D1=80=D0=B0=D0=B2=D1=81=D1=82=D0=B2=D1=83=D0=B9=D1=82=D0=B5!\r\n",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message missing %q:\n%s", expected, message)
		}
	}
	if strings.Contains(message, "Subject: Подтвердите") {
		t.Fatal("non-ASCII subject must be MIME encoded")
	}
}

func TestSMTPTransportRejectsInvalidMessageAddresses(t *testing.T) {
	transport := NewSMTPTransport("127.0.0.1:1", "", "")
	if err := transport.Send(context.Background(), EmailMessage{From: "invalid", To: "owner@example.com"}); err == nil || !strings.Contains(err.Error(), "sender") {
		t.Fatalf("sender error=%v", err)
	}
	if err := transport.Send(context.Background(), EmailMessage{From: "site@example.com", To: "invalid"}); err == nil || !strings.Contains(err.Error(), "recipient") {
		t.Fatalf("recipient error=%v", err)
	}
}

func smtpTestCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certificatePEM)
	return certificate, roots
}

func TestSMTPTransportString(t *testing.T) {
	if NewSMTPTransport(" smtp.example.com:587 ", "", "").String() != "smtp.example.com:587" {
		t.Fatal("transport string must be trimmed")
	}
}
