//go:build e2e

package mtgserver

import (
	"context"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/nathanmartins/mtg-mcp/internal/moxfield"
)

// toolE2ECase calls one MCP tool against the live APIs and lists text its output must contain.
type toolE2ECase struct {
	args map[string]any
	want []string
}

// toolE2ECases is the real-API counterpart of the mocked handler tests: one call per registered
// tool. TestEveryToolHasAnE2ECase keeps it in step with the registered tools. The Moxfield deck
// and user-deck tools need IDs discovered at run time, so TestMoxfieldToolsE2E covers them.
func toolE2ECases() map[string]toolE2ECase {
	return map[string]toolE2ECase{
		"search_cards":             {map[string]any{argQuery: testSolRingQuery, argLimit: 3.0}, []string{testSolRing}},
		"get_card_details":         {map[string]any{paramName: testSolRing}, []string{testSolRing, "Commander: legal"}},
		"check_commander_legality": {map[string]any{paramName: testManaCrypt}, []string{testManaCrypt, "BANNED"}},
		"get_card_rulings":         {map[string]any{paramName: testDoublingSeason}, []string{testDoublingSeason}},
		"get_card_price":           {map[string]any{paramName: testSolRing}, []string{testSolRing, "USD"}},
		"get_banned_list":          {map[string]any{}, []string{"Banned List", testManaCrypt}},
		"validate_deck": {
			map[string]any{
				paramCommander: testAtraxa,
				// Delver checks that a double-faced card named by its front face resolves on the
				// real collection endpoint; the snow basics must not count as duplicates.
				argDecklist: "1 Sol Ring\n1 Lightning Bolt\n1 Mana Crypt\n1 Delver of Secrets\n" +
					"4 Snow-Covered Island\n30 Forest",
			},
			[]string{"Lightning Bolt (R)", "Mana Crypt (banned)", "Every card was found", "No duplicates"},
		},
		"get_card_image": {map[string]any{paramName: testSolRing, paramSize: "small"}, []string{testSolRing}},
		"search_moxfield_decks": {
			map[string]any{paramCommander: testAtraxa, argLimit: 1.0},
			[]string{testAtraxaShort},
		},
		"get_archidekt_deck":       {map[string]any{argDeckID: "18257175"}, []string{"Deathstar Hearthhull"}},
		"get_archidekt_user_decks": {map[string]any{argUsername: "nathanmartins"}, []string{"nathanmartins"}},
		"search_archidekt_decks": {
			map[string]any{paramCommander: testAtraxa, argLimit: 3.0},
			[]string{"Archidekt Decks: Atraxa"},
		},
		"get_rule": {map[string]any{"rule": "702.19"}, []string{testTrample}},
		// Matches come back in rulebook order, so with a small limit 702.19 is not among them.
		"search_rules": {
			map[string]any{argKeyword: testKeywordTrample, argLimit: 2.0},
			[]string{`Rules matching "trample" (2)`},
		},
		"get_glossary_term": {map[string]any{"term": testTrample}, []string{testTrample}},
		"get_edhrec_recommendations": {
			map[string]any{paramCommander: testAtraxa, argLimit: 3.0},
			[]string{testAtraxaShort},
		},
		// Letters, not a page name: checks the WUBRG mapping against the live pages.
		"get_edhrec_combos":    {map[string]any{argColors: "uw", argLimit: 3.0}, []string{"Popular Combos"}},
		"get_edhrec_set_cards": {map[string]any{paramSet: "blb", argLimit: 3.0}, []string{"EDHREC Set Overview"}},
	}
}

// registeredE2EServer returns the production server registered on a fresh MCP server.
func registeredE2EServer(t *testing.T) *server.MCPServer {
	t.Helper()
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mcpServer := server.NewMCPServer("e2e", "0.0.0")
	s.Register(mcpServer)
	return mcpServer
}

func callTool(t *testing.T, mcpServer *server.MCPServer, name string, args map[string]any) string {
	t.Helper()
	tool := mcpServer.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q is not registered", name)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	var req mcp.CallToolRequest
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := tool.Handler(ctx, req)
	if err != nil {
		t.Fatalf("%s returned a Go error: %v", name, err)
	}
	text := resultText(t, res)
	if res.IsError {
		t.Fatalf("%s returned an error result: %s", name, text)
	}
	return text
}

func TestEveryToolHasAnE2ECase(t *testing.T) {
	covered := slices.Collect(maps.Keys(toolE2ECases()))
	covered = append(covered, "get_moxfield_deck", "get_moxfield_user_decks") // TestMoxfieldToolsE2E
	slices.Sort(covered)
	registered := slices.Sorted(maps.Keys(registeredE2EServer(t).ListTools()))
	if !slices.Equal(covered, registered) {
		t.Errorf("e2e cases cover %v\nregistered tools are %v", covered, registered)
	}
}

func TestToolsE2E(t *testing.T) {
	mcpServer := registeredE2EServer(t)
	for _, name := range slices.Sorted(maps.Keys(toolE2ECases())) {
		tc := toolE2ECases()[name]
		t.Run(name, func(t *testing.T) {
			text := callTool(t, mcpServer, name, tc.args)
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Errorf("%s output missing %q:\n%s", name, want, text)
				}
			}
		})
	}
}

// TestMoxfieldToolsE2E covers the two Moxfield tools whose arguments must be discovered live:
// it finds a deck through search, reads it with get_moxfield_deck, and lists its author's decks.
func TestMoxfieldToolsE2E(t *testing.T) {
	mcpServer := registeredE2EServer(t)

	search, err := moxfield.NewClient().SearchDecks(t.Context(), moxfield.SearchParams{
		CardName: testSolRing, Format: defaultFormat, PageSize: 5, PageNumber: 1,
	})
	if err != nil || len(search.Data) == 0 {
		t.Fatalf("finding a Moxfield deck failed: %v", err)
	}

	deckText := callTool(t, mcpServer, "get_moxfield_deck", map[string]any{argDeckID: search.Data[0].PublicID})
	if !strings.Contains(deckText, search.Data[0].Name) {
		t.Errorf("get_moxfield_deck output missing the deck name %q:\n%s", search.Data[0].Name, deckText)
	}

	author := regexp.MustCompile(`\*\*Author:\*\* ([^,\n]+)`).FindStringSubmatch(deckText)
	if author == nil {
		t.Skip("the deck lists no author, so there is no username to look up")
	}
	decksText := callTool(t, mcpServer, "get_moxfield_user_decks",
		map[string]any{argUsername: author[1], argPageSize: 5.0})
	if decksText == "" {
		t.Errorf("get_moxfield_user_decks returned no text for %q", author[1])
	}
}

func TestResourcesE2E(t *testing.T) {
	mcpServer := registeredE2EServer(t)
	want := map[string]string{
		testRulesURI:              "Commander Format Rules",
		"commander://banned-list": testManaCrypt,
		"rules://comprehensive":   "Comprehensive Rules",
		cardImageWidgetURI:        "<html",
	}
	resources := mcpServer.ListResources()
	if got := slices.Sorted(maps.Keys(resources)); !slices.Equal(got, slices.Sorted(maps.Keys(want))) {
		t.Fatalf("registered resources %v, e2e covers %v", got, slices.Sorted(maps.Keys(want)))
	}
	for uri, substring := range want {
		t.Run(uri, func(t *testing.T) {
			var req mcp.ReadResourceRequest
			req.Params.URI = uri
			contents, err := resources[uri].Handler(t.Context(), req)
			if err != nil {
				t.Fatalf("reading %s: %v", uri, err)
			}
			text, ok := contents[0].(*mcp.TextResourceContents)
			if !ok {
				t.Fatalf("%s returned %T, want text contents", uri, contents[0])
			}
			if !strings.Contains(text.Text, substring) {
				t.Errorf("%s missing %q", uri, substring)
			}
		})
	}
}
