package notification

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewTelegramHTTPClient creates the single transport used by Telegram polling,
// outgoing notifications and retention cleanup. A configured proxy never
// falls back to a direct request.
func NewTelegramHTTPClient(rawProxyURL string) (*http.Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default HTTP transport is unavailable")
	}
	telegramTransport := transport.Clone()
	if rawProxyURL != "" {
		proxyURL, err := url.ParseRequestURI(rawProxyURL)
		validPath := proxyURL != nil && (proxyURL.Path == "" || proxyURL.Path == "/")
		if err != nil || proxyURL.Scheme != "http" || proxyURL.Hostname() == "" || proxyURL.Port() == "" || proxyURL.User != nil || !validPath || proxyURL.RawQuery != "" || proxyURL.Fragment != "" || strings.ContainsAny(rawProxyURL, "\r\n\t ") {
			return nil, fmt.Errorf("Telegram proxy must be an HTTP host:port without credentials")
		}
		if _, err := net.LookupPort("tcp", proxyURL.Port()); err != nil {
			return nil, fmt.Errorf("Telegram proxy port is invalid")
		}
		telegramTransport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{Transport: telegramTransport, Timeout: 40 * time.Second}, nil
}
