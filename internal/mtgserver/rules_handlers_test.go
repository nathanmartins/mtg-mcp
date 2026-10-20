package mtgserver

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nathanmartins/mtg-mcp/internal/rules"
)

// sampleRulesTxt is a minimal Comprehensive Rules document for exercising the rules handlers.
const sampleRulesTxt = `Magic: The Gathering Comprehensive Rules

These rules are effective as of June 19, 2026.

100. General

100.1. These Magic rules apply to any Magic game with two or more players.

702.19. Trample

702.19a Trample is a static ability that modifies the rules for assigning combat damage.

702.19b The controller of an attacking creature with trample first assigns damage to the creatures blocking it.

Glossary

Trample
A keyword ability that allows excess combat damage to be assigned to the defending player. See rule 702.19.

Deathtouch
A keyword ability that causes damage dealt by an object to be lethal. See rule 702.2.
`

func TestRulesCountsAndHandlers(t *testing.T) {
	if totalToolCount != 20 {
		t.Errorf("totalToolCount = %d, want 20", totalToolCount)
	}
	if totalResourceCount != 3 {
		t.Errorf("totalResourceCount = %d, want 3", totalResourceCount)
	}

	// Pre-populate the cache so handlers do no network.
	srv := &Server{rules: rules.NewSeededCache(rules.Parse(sampleRulesTxt))}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"rule": "702.19"}
	res, err := srv.handleGetRule(t.Context(), req)
	if err != nil {
		t.Fatalf("handleGetRule: %v", err)
	}
	if !strings.Contains(toolResultText(t, res), "Trample") {
		t.Error("handleGetRule should return rule 702.19 text")
	}

	greq := mcp.CallToolRequest{}
	greq.Params.Arguments = map[string]any{"term": "Trample"}
	gres, err := srv.handleGetGlossaryTerm(t.Context(), greq)
	if err != nil {
		t.Fatalf("handleGetGlossaryTerm: %v", err)
	}
	if !strings.Contains(toolResultText(t, gres), "keyword ability") {
		t.Error("handleGetGlossaryTerm should return the definition")
	}

	sreq := mcp.CallToolRequest{}
	sreq.Params.Arguments = map[string]any{"keyword": "trample", "limit": float64(1)}
	sres, err := srv.handleSearchRules(t.Context(), sreq)
	if err != nil {
		t.Fatalf("handleSearchRules: %v", err)
	}
	stext := toolResultText(t, sres)
	if !strings.Contains(stext, "Rules matching") || !strings.Contains(stext, "702.19") {
		t.Errorf("handleSearchRules should list matching rule 702.19; got:\n%s", stext)
	}
	// limit=1 must cap the listing to a single "- " result line
	if n := strings.Count(stext, "\n- "); n != 1 {
		t.Errorf("expected exactly 1 result line with limit=1, got %d:\n%s", n, stext)
	}
}

// toolResultText extracts the text payload from a tool result.
func toolResultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
