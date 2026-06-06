package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultHTTPTimeout = 10 * time.Second

// HTTPGet performs an HTTP GET request with context and timeout.
// The URL must use the https scheme.
func HTTPGet(ctx context.Context, rawURL string) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid or non-HTTPS URL: %s", rawURL)
	}

	return httpGetWithClient(ctx, parsed.String(), &http.Client{Timeout: defaultHTTPTimeout})
}

// httpGetWithClient performs an HTTP GET request using the provided client (used for testing).
func httpGetWithClient(ctx context.Context, rawURL string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	return client.Do(req)
}
