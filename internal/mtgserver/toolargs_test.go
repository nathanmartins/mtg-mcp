package mtgserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestSortAscendingArg(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    map[string]any
		want    bool
		wantErr bool
	}{
		{name: "default is descending", args: map[string]any{}, want: false},
		{name: "asc", args: map[string]any{argSortDirection: "asc"}, want: true},
		{name: "desc", args: map[string]any{argSortDirection: "desc"}, want: false},
		{name: "unknown value", args: map[string]any{argSortDirection: "sideways"}, wantErr: true},
		{name: "empty value", args: map[string]any{argSortDirection: ""}, wantErr: true},
		{name: "wrong type", args: map[string]any{argSortDirection: 1.0}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	tests := []struct {
		name    string
		args    map[string]any
		want    int
		wantErr bool
	}{
		{name: "absent uses fallback", args: map[string]any{}, want: 10},
		{name: "in range", args: map[string]any{argLimit: 25.0}, want: 25},
		{name: "minimum allowed", args: map[string]any{argLimit: 1.0}, want: 1},
		{name: "above maximum is capped", args: map[string]any{argLimit: 500.0}, want: 50},
		{name: "below minimum", args: map[string]any{argLimit: 0.0}, wantErr: true},
		{name: "negative", args: map[string]any{argLimit: -3.0}, wantErr: true},
		{name: "fractional", args: map[string]any{argLimit: 2.5}, wantErr: true},
		{name: "string", args: map[string]any{argLimit: "10"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := cappedIntArg(tt.args, argLimit, 10, 1, 50)
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
	t.Parallel()
	s := &Server{}
	tests := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args    map[string]any
	}{
		{"search_cards fractional limit", s.handleSearchCards, map[string]any{argQuery: testSol, argLimit: 2.5}},
		{"search_cards zero limit", s.handleSearchCards, map[string]any{argQuery: testSol, argLimit: 0.0}},
		{"moxfield user decks string page_size", s.handleGetMoxfieldUserDecks,
			map[string]any{argUsername: "u", argPageSize: "20"}},
		{"archidekt user decks negative page", s.handleGetArchidektUserDecks,
			map[string]any{argUsername: "u", argPage: -1.0}},
		{
			"search_rules fractional limit",
			s.handleSearchRules,
			map[string]any{argKeyword: testKeywordTrample, argLimit: 1.5},
		},
		{"edhrec recommendations negative limit", s.handleGetEDHRECRecommendations,
			map[string]any{paramCommander: testAtraxaShort, argLimit: -1.0}},
		{"edhrec combos fractional limit", s.handleGetEDHRECCombos, map[string]any{argColors: "wu", argLimit: 0.5}},
		{"edhrec set cards string limit", s.handleGetEDHRECSetCards, map[string]any{paramSet: "blb", argLimit: "5"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
