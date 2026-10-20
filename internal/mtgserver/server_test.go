package mtgserver

import (
	"maps"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanmartins/mtg-mcp/internal/archidekt"
	"github.com/nathanmartins/mtg-mcp/internal/moxfield"
)

func TestNewMTGCommanderServer(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s == nil || s.scryfallClient == nil {
		t.Fatal("expected a server with an initialized Scryfall client")
	}
}

// TestNewWiresProductionClients checks New uses the production API clients.
func TestNewWiresProductionClients(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s.moxfield == nil || *s.moxfield != *moxfield.NewClient() {
		t.Errorf("moxfield client = %+v, want moxfield.NewClient()", s.moxfield)
	}
	if s.archidekt == nil || *s.archidekt != *archidekt.NewClient() {
		t.Errorf("archidekt client = %+v, want archidekt.NewClient()", s.archidekt)
	}
}

// TestRegisterToolsAndResources pins the exact tool and resource surface Register exposes,
// so adding, renaming or dropping one is a deliberate change to this list.
func TestRegisterToolsAndResources(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	mcpServer := server.NewMCPServer("test", "0.0.0", server.WithRecovery())
	s.Register(mcpServer)

	wantTools := []string{
		"check_commander_legality", "get_archidekt_deck", "get_archidekt_user_decks", "get_banned_list",
		"get_card_details", "get_card_image", "get_card_price", "get_card_rulings", "get_edhrec_combos",
		"get_edhrec_recommendations", "get_edhrec_set_cards", "get_glossary_term", "get_moxfield_deck",
		"get_moxfield_user_decks", "get_rule", "search_archidekt_decks", "search_cards", "search_moxfield_decks",
		"search_rules", "validate_deck",
	}
	if got := slices.Sorted(maps.Keys(mcpServer.ListTools())); !slices.Equal(got, wantTools) {
		t.Errorf("registered tools = %v\nwant %v", got, wantTools)
	}

	wantResources := []string{
		"commander://banned-list",
		"commander://rules",
		cardImageWidgetURI,
		"rules://comprehensive",
	}
	slices.Sort(wantResources)
	if got := slices.Sorted(maps.Keys(mcpServer.ListResources())); !slices.Equal(got, wantResources) {
		t.Errorf("registered resources = %v\nwant %v", got, wantResources)
	}
}
