package providers

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// GetHTTPClient возвращает настроенный HTTP-клиент с поддержкой HTTP/HTTPS/SOCKS5/SOCKS5h прокси
func GetHTTPClient(proxyURL string) *http.Client {
	timeout := 10 * time.Second
	proxyURL = strings.TrimSpace(proxyURL)

	if proxyURL == "" {
		return &http.Client{
			Timeout: timeout,
		}
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return &http.Client{
			Timeout: timeout,
		}
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression: false,
	}

	if scheme == "socks5" || scheme == "socks5h" {
		dialer, err := proxy.FromURL(parsedURL, proxy.Direct)
		if err == nil {
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
					return contextDialer.DialContext(ctx, network, addr)
				}
				return dialer.Dial(network, addr)
			}
		}
	} else if scheme == "http" || scheme == "https" {
		transport.Proxy = http.ProxyURL(parsedURL)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
