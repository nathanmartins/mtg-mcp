package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func readResourceRequest(uri string) mcp.ReadResourceRequest {
	var req mcp.ReadResourceRequest
	req.Params.URI = uri
	return req
}

func resourceText(t *testing.T, contents []mcp.ResourceContents) string {
	t.Helper()

	if len(contents) == 0 {
		t.Fatal("no resource contents")
	}
	trc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		// Handlers return a pointer in some mcp-go versions.
		if ptr, okPtr := contents[0].(*mcp.TextResourceContents); okPtr {
			return ptr.Text
		}
		t.Fatalf("content is not TextResourceContents: %T", contents[0])
	}
	return trc.Text
}

func TestHandleCommanderRules(t *testing.T) {
	s := &MTGCommanderServer{}

	contents, err := s.handleCommanderRules(context.Background(), readResourceRequest("commander://rules"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resourceText(t, contents)
	for _, want := range []string{"Commander Format Rules", "100 cards total", "Starting Life", "Commander Damage"} {
		if !strings.Contains(text, want) {
			t.Errorf("rules output missing %q", want)
		}
	}

	if contents[0].(*mcp.TextResourceContents).URI != "commander://rules" {
		t.Error("expected URI to be echoed back")
	}
	if contents[0].(*mcp.TextResourceContents).MIMEType != "text/plain" {
		t.Error("expected text/plain MIME type")
	}
}

func TestHandleBannedListResource(t *testing.T) {
	t.Run("success returns json", func(t *testing.T) {
		body := `{"object":"list","total_cards":1,"has_more":false,"data":[` +
			`{"id":"1","name":"Mana Crypt","type_line":"Artifact","mana_cost":"{0}",` +
			`"legalities":{"commander":"banned"}}]}`
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, body)
		})

		contents, err := s.handleBannedListResource(
			context.Background(),
			readResourceRequest("commander://banned-list"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		text := resourceText(t, contents)

		var parsed struct {
			Format      string `json:"format"`
			TotalBanned int    `json:"total_banned"`
			Cards       []struct {
				Name string `json:"name"`
			} `json:"cards"`
		}
		if unmarshalErr := json.Unmarshal([]byte(text), &parsed); unmarshalErr != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", unmarshalErr, text)
		}
		if parsed.Format != "commander" {
			t.Errorf("format = %q, want commander", parsed.Format)
		}
		if parsed.TotalBanned != 1 {
			t.Errorf("total_banned = %d, want 1", parsed.TotalBanned)
		}
		if len(parsed.Cards) != 1 || parsed.Cards[0].Name != "Mana Crypt" {
			t.Errorf("unexpected cards: %+v", parsed.Cards)
		}
	})

	t.Run("fetch failure returns error", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusInternalServerError)
		})

		_, err := s.handleBannedListResource(
			context.Background(),
			readResourceRequest("commander://banned-list"),
		)
		if err == nil {
			t.Error("expected error when banned list fetch fails")
		}
	})
}
