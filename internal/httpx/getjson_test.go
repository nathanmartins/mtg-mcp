package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sample struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func serve(t *testing.T, status int, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestGetJSON(t *testing.T) {
	t.Parallel()
	t.Run("decodes a 200 body", func(t *testing.T) {
		t.Parallel()
		var got sample
		err := GetJSON(
			t.Context(),
			APIClient(),
			serve(t, http.StatusOK, `{"name":"Sol Ring","count":2}`),
			"test API",
			&got,
		)
		if err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		if got != (sample{Name: "Sol Ring", Count: 2}) {
			t.Errorf("GetJSON() decoded %+v", got)
		}
	})

	t.Run("reports a non-200 status without decoding", func(t *testing.T) {
		t.Parallel()
		var got sample
		err := GetJSON(t.Context(), APIClient(), serve(t, http.StatusTooManyRequests, `<html>slow down</html>`),
			"test API", &got)
		if !HasStatus(err, http.StatusTooManyRequests) {
			t.Fatalf("GetJSON() error = %v, want a 429 StatusError", err)
		}
		if err.Error() != "test API returned status 429" {
			t.Errorf("error text = %q", err.Error())
		}
	})

	t.Run("names the source in decode errors", func(t *testing.T) {
		t.Parallel()
		var got sample
		err := GetJSON(t.Context(), APIClient(), serve(t, http.StatusOK, `{"name":`), "test API", &got)
		if err == nil || !strings.Contains(err.Error(), "failed to decode test API response") {
			t.Errorf("GetJSON() error = %v, want a decode error naming the source", err)
		}
	})

	// encoding/json/v2 matches names case-sensitively, unlike v1: a differently cased key is
	// ignored rather than filling the field.
	t.Run("matches field names case-sensitively", func(t *testing.T) {
		t.Parallel()
		var got sample
		err := GetJSON(t.Context(), APIClient(), serve(t, http.StatusOK, `{"Name":"Sol Ring"}`), "test API", &got)
		if err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		if got.Name != "" {
			t.Errorf("Name = %q, want it left empty for a differently cased key", got.Name)
		}
	})

	// v2 also rejects duplicate object names, which v1 silently resolves to the last one.
	t.Run("rejects duplicate names", func(t *testing.T) {
		t.Parallel()
		var got sample
		err := GetJSON(t.Context(), APIClient(), serve(t, http.StatusOK, `{"name":"a","name":"b"}`), "test API", &got)
		if err == nil {
			t.Error("GetJSON() should reject a body with duplicate names")
		}
	})
}

func TestHasStatus(t *testing.T) {
	t.Parallel()
	err := &StatusError{Source: "x", StatusCode: http.StatusNotFound}
	if !HasStatus(err, http.StatusNotFound) {
		t.Error("HasStatus() should match the error's own status")
	}
	if HasStatus(err, http.StatusTooManyRequests) {
		t.Error("HasStatus() should not match a different status")
	}
	if HasStatus(io.EOF, http.StatusNotFound) {
		t.Error("HasStatus() should not match a non-status error")
	}
}

func TestAPIClientIsShared(t *testing.T) {
	t.Parallel()
	first, second := APIClient(), APIClient()
	if first != second {
		t.Error("APIClient() should return one shared client so connections are reused")
	}
	if APIClient().Timeout != APITimeout {
		t.Errorf("APIClient().Timeout = %v, want %v", APIClient().Timeout, APITimeout)
	}
}
