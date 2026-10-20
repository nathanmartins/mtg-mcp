package mtgserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestSortAscendingArg(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		want    bool
		wantErr bool
	}{
		{name: "default is descending", args: map[string]any{}, want: false},
		{name: "asc", args: map[string]any{"sort_direction": "asc"}, want: true},
		{name: "desc", args: map[string]any{"sort_direction": "desc"}, want: false},
		{name: "unknown value", args: map[string]any{"sort_direction": "sideways"}, wantErr: true},
		{name: "empty value", args: map[string]any{"sort_direction": ""}, wantErr: true},
		{name: "wrong type", args: map[string]any{"sort_direction": 1.0}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sortAscendingArg(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortAscendingArg() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("sortAscendingArg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCappedIntArg(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		want    int
		wantErr bool
	}{
		{name: "absent uses fallback", args: map[string]any{}, want: 10},
		{name: "in range", args: map[string]any{"limit": 25.0}, want: 25},
		{name: "minimum allowed", args: map[string]any{"limit": 1.0}, want: 1},
		{name: "above maximum is capped", args: map[string]any{"limit": 500.0}, want: 50},
		{name: "below minimum", args: map[string]any{"limit": 0.0}, wantErr: true},
		{name: "negative", args: map[string]any{"limit": -3.0}, wantErr: true},
		{name: "fractional", args: map[string]any{"limit": 2.5}, wantErr: true},
		{name: "string", args: map[string]any{"limit": "10"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cappedIntArg(tt.args, "limit", 10, 1, 50)
			if (err != nil) != tt.wantErr {
				t.Fatalf("cappedIntArg() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("cappedIntArg() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestHandlersRejectInvalidNumbers checks every tool with an optional numeric argument
// reports a bad value instead of silently substituting its default. Validation runs before
// any upstream call, so a zero Server is enough.
func TestHandlersRejectInvalidNumbers(t *testing.T) {
	s := &Server{}
	tests := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args    map[string]any
	}{
		{"search_cards fractional limit", s.handleSearchCards, map[string]any{"query": "sol", "limit": 2.5}},
		{"search_cards zero limit", s.handleSearchCards, map[string]any{"query": "sol", "limit": 0.0}},
		{"moxfield user decks string page_size", s.handleGetMoxfieldUserDecks,
			map[string]any{"username": "u", "page_size": "20"}},
		{"archidekt user decks negative page", s.handleGetArchidektUserDecks,
			map[string]any{"username": "u", "page": -1.0}},
		{"search_rules fractional limit", s.handleSearchRules, map[string]any{"keyword": "trample", "limit": 1.5}},
		{"edhrec recommendations negative limit", s.handleGetEDHRECRecommendations,
			map[string]any{"commander": "Atraxa", "limit": -1.0}},
		{"edhrec combos fractional limit", s.handleGetEDHRECCombos, map[string]any{"colors": "wu", "limit": 0.5}},
		{"edhrec set cards string limit", s.handleGetEDHRECSetCards, map[string]any{"set": "blb", "limit": "5"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.handler(t.Context(), toolRequest(tt.args))
			if err != nil {
				t.Fatalf("handler returned a Go error: %v", err)
			}
			if !res.IsError {
				t.Errorf("expected an error result, got: %s", resultText(t, res))
			}
		})
	}
}
