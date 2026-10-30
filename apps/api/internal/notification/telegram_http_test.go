package notification

import (
	"net/http"
	"testing"
)

func TestNewTelegramHTTPClientUsesConfiguredProxyWithoutEnvironmentFallback(t *testing.T) {
	client, err := NewTelegramHTTPClient("http://telegram-proxy:19840")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("configured Telegram proxy was not installed on the transport")
	}
	request, _ := http.NewRequest(http.MethodGet, "https://api.telegram.org", nil)
	proxyURL, err := transport.Proxy(request)
	if err != nil || proxyURL.String() != "http://telegram-proxy:19840" {
		t.Fatalf("proxy=%v error=%v", proxyURL, err)
	}
	if client.Timeout.Seconds() != 40 {
		t.Fatalf("unexpected client timeout: %s", client.Timeout)
	}
}

func TestNewTelegramHTTPClientWithoutProxyUsesDefaultTransportClone(t *testing.T) {
	client, err := NewTelegramHTTPClient("")
	if err != nil {
		t.Fatal(err)
	}
	if client.Transport == http.DefaultTransport {
		t.Fatal("Telegram client must not mutate the shared default transport")
	}
}

func TestNewTelegramHTTPClientRejectsUnsafeProxyURLs(t *testing.T) {
	for _, rawURL := range []string{
		"https://telegram-proxy:19840",
		"http://user:password@telegram-proxy:19840",
		"http://telegram-proxy:19840/path",
		"http://telegram-proxy",
		"http://telegram-proxy:not-a-port",
	} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := NewTelegramHTTPClient(rawURL); err == nil {
				t.Fatal("unsafe proxy URL accepted")
			}
		})
	}
}
