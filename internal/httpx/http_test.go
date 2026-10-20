package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		wantErr        bool
		checkHeaders   bool
	}{
		{
			name:           "successful request",
			serverResponse: `{"status":"ok"}`,
			serverStatus:   http.StatusOK,
			wantErr:        false,
			checkHeaders:   true,
		},
		{
			name:           "server returns 404",
			serverResponse: `{"error":"not found"}`,
			serverStatus:   http.StatusNotFound,
			wantErr:        false, // Get doesn't error on status codes
			checkHeaders:   true,
		},
		{
			name:           "server returns 500",
			serverResponse: `{"error":"internal error"}`,
			serverStatus:   http.StatusInternalServerError,
			wantErr:        false,
			checkHeaders:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel(
			// Create test server
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check headers if requested
				if tt.checkHeaders {
					if ua := r.Header.Get("User-Agent"); ua != "MTG-Commander-MCP-Server/1.0" {
						t.Errorf("User-Agent = %v, want MTG-Commander-MCP-Server/1.0", ua)
					}
					if accept := r.Header.Get("Accept"); accept != "application/json" {
						t.Errorf("Accept = %v, want application/json", accept)
					}
				}

				w.WriteHeader(tt.serverStatus)
				_, _ = w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			// Test the function using the internal client-aware variant
			ctx := t.Context()
			resp, err := GetWithClient(ctx, server.URL, server.Client())

			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				defer func() {
					_ = resp.Body.Close()
				}()
				if resp.StatusCode != tt.serverStatus {
					t.Errorf("Get() status = %v, want %v", resp.StatusCode, tt.serverStatus)
				}
			}
		})
	}
}

func TestGet_ContextCancellation(t *testing.T) {
	t.Parallel(
	// Create a server that delays response
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	resp, err := GetWithClient(ctx, server.URL, server.Client())
	if err == nil {
		_ = resp.Body.Close()
		t.Error("Get() expected error with cancelled context, got nil")
	}
}

func TestGet_Timeout(t *testing.T) {
	t.Parallel(
	// Note: synctest doesn't work well with httptest.NewServer since it uses real TCP connections.
	// Instead, we use a short real timeout (50ms) to keep the test fast.
	// See: https://go.dev/blog/synctest (recommends net.Pipe for in-memory testing)
	)

	// Create a server that blocks until request context is cancelled
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Wait for context cancellation
		<-r.Context().Done()
	}))
	defer server.Close()

	// Use a short timeout to make test run fast (50ms)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	resp, err := GetWithClient(ctx, server.URL, server.Client())

	// Should timeout due to context deadline
	if err == nil {
		_ = resp.Body.Close()
		t.Error("Get() expected timeout error, got nil")
	}
}

func TestGet_InvalidURL(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	resp, err := Get(ctx, "://invalid-url")
	if err == nil {
		_ = resp.Body.Close()
		t.Error("Get() expected error with invalid URL, got nil")
	}
}
