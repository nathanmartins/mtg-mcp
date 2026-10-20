// Package httpx holds the HTTP GET and JSON helpers shared by the upstream API clients.
package httpx

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultTimeout = 10 * time.Second
	// APITimeout bounds a single deck-site API request. MCP request contexts carry no
	// deadline, and several tools issue requests in sequence (Moxfield commander
	// verification, Archidekt pages that straddle two upstream pages), so an unbounded
	// client would let one hung upstream request stall the whole tool call.
	APITimeout = 15 * time.Second
)

// StatusError reports a non-200 upstream response. The status is preserved so callers
// can react to specific codes, such as Archidekt's 404 or Moxfield's 429.
type StatusError struct {
	Source     string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s returned status %d", e.Source, e.StatusCode)
}

// HasStatus reports whether err is a *StatusError carrying the given status code.
func HasStatus(err error, code int) bool {
	statusErr, ok := errors.AsType[*StatusError](err)
	return ok && statusErr.StatusCode == code
}

// APIClient returns a client with the deck-site API timeout.
func APIClient() *http.Client {
	return &http.Client{Timeout: APITimeout}
}

// Get performs an HTTP GET request with context and timeout.
// The URL must use the https scheme.
func Get(ctx context.Context, rawURL string) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid or non-HTTPS URL: %s", rawURL)
	}

	return GetWithClient(ctx, parsed.String(), &http.Client{Timeout: DefaultTimeout})
}

// GetWithClient performs an HTTP GET request using the provided client (used for testing).
func GetWithClient(ctx context.Context, rawURL string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	return client.Do(req)
}

// GetJSON GETs reqURL with client and decodes a 200 response body into target with
// encoding/json/v2, which matches field names case-sensitively and rejects invalid UTF-8 and
// duplicate names. Any other status is returned as a *StatusError naming source, without
// decoding the body: upstream error pages are often not JSON (EDHREC answers unknown slugs
// with XML).
func GetJSON(ctx context.Context, client *http.Client, reqURL, source string, target any) error {
	resp, err := GetWithClient(ctx, reqURL, client)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return &StatusError{Source: source, StatusCode: resp.StatusCode}
	}

	if decodeErr := json.UnmarshalRead(resp.Body, target); decodeErr != nil {
		return fmt.Errorf("failed to decode %s response: %w", source, decodeErr)
	}

	return nil
}
