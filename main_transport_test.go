package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfiguredMCPTransport(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      string
		wantError bool
	}{
		{name: "default", want: defaultMCPTransport},
		{name: "stdio", value: "stdio", want: defaultMCPTransport},
		{name: "http alias", value: "HTTP", want: "streamable-http"},
		{name: "streamable http", value: " streamable-http ", want: "streamable-http"},
		{name: "unknown", value: "sse", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("MTG_MCP_TRANSPORT", test.value)
			got, err := configuredMCPTransport()
			if test.wantError {
				if err == nil {
					t.Fatal("configuredMCPTransport() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("configuredMCPTransport() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("configuredMCPTransport() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeMCPHTTPPath(t *testing.T) {
	tests := map[string]string{
		"":       defaultMCPHTTPPath,
		"mcp":    "/mcp",
		"/mcp/":  "/mcp",
		"/tools": "/tools",
	}

	for input, want := range tests {
		if got := normalizeMCPHTTPPath(input); got != want {
			t.Errorf("normalizeMCPHTTPPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStaticBearerMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := staticBearerMiddleware(next, "secret")

	tests := []struct {
		name   string
		header string
		status int
	}{
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong scheme", header: "Basic secret", status: http.StatusUnauthorized},
		{name: "wrong token", header: "Bearer other", status: http.StatusUnauthorized},
		{name: "valid", header: "Bearer secret", status: http.StatusNoContent},
		{name: "case insensitive scheme", header: "bearer secret", status: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", nil)
			if test.header != "" {
				request.Header.Set("Authorization", test.header)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestHealthzHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://example.test/healthz", nil)
	response := httptest.NewRecorder()

	healthzHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != "ok\n" {
		t.Fatalf("body = %q, want %q", response.Body.String(), "ok\n")
	}
}
