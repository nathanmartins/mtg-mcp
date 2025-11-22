package main

import (
	"context"
	"net/http"
	"time"
)

// HTTPGet performs an HTTP GET request with context and timeout.
func HTTPGet(ctx context.Context, url string) (*http.Response, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	return client.Do(req)
}
