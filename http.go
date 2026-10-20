package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultHTTPTimeout = 10 * time.Second
	// apiHTTPTimeout bounds a single deck-site API request. MCP request contexts carry no
	// deadline, and several tools issue requests in sequence (Moxfield commander
	// verification, Archidekt pages that straddle two upstream pages), so an unbounded
	// client would let one hung upstream request stall the whole tool call.
	apiHTTPTimeout = 15 * time.Second
)

// httpStatusError reports a non-200 upstream response. The status is preserved so callers
// can react to specific codes, such as Archidekt's 404 or Moxfield's 429.
type httpStatusError struct {
	Source     string
	StatusCode int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s returned status %d", e.Source, e.StatusCode)
}

// hasStatus reports whether err is an *httpStatusError carrying the given status code.
func hasStatus(err error, code int) bool {
	statusErr, ok := errors.AsType[*httpStatusError](err)
	return ok && statusErr.StatusCode == code
}

// apiHTTPClient returns a client with the deck-site API timeout.
func apiHTTPClient() *http.Client {
	return &http.Client{Timeout: apiHTTPTimeout}
}

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

// getJSON GETs reqURL with client and decodes a 200 response body into target. Any other
// status is returned as an *httpStatusError naming source, without decoding the body:
// upstream error pages are often not JSON (EDHREC answers unknown slugs with XML).
func getJSON(ctx context.Context, client *http.Client, reqURL, source string, target any) error {
	resp, err := httpGetWithClient(ctx, reqURL, client)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return &httpStatusError{Source: source, StatusCode: resp.StatusCode}
	}

	if decodeErr := json.NewDecoder(resp.Body).Decode(target); decodeErr != nil {
		return fmt.Errorf("failed to decode %s response: %w", source, decodeErr)
	}

	return nil
}
