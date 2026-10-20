// Package mtgserver implements the MTG Commander MCP tools and resources on top of the
// upstream API clients.
package mtgserver

import (
	"fmt"

	"github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanmartins/mtg-mcp/internal/archidekt"
	"github.com/nathanmartins/mtg-mcp/internal/edhrec"
	"github.com/nathanmartins/mtg-mcp/internal/moxfield"
	"github.com/nathanmartins/mtg-mcp/internal/rules"
)

const (
	maxSearchLimit               = 50
	defaultSplitLimit            = 2
	maxPageSize                  = 100
	deckValidationBasicCardCount = 99
	deckValidationCommanderCount = 100
	cardSortOrderName            = "name"
	defaultFormat                = "commander"
	paramCommander               = "commander"
	paramName                    = "name"
	paramLanguage                = "language"
	paramSize                    = "size"
	paramSet                     = "set"
	paramCollectorNumber         = "collector_number"
	paramFace                    = "face"
	paramTheme                   = "theme"
	paramPriceTier               = "price_tier"
	faceFront                    = "front"
	faceBack                     = "back"
	defaultCardImageLanguage     = "en"
	defaultCardImageSize         = imageSizeNormal
	mimeTypeTextPlain            = "text/plain"
)

// Server wraps the MCP server with MTG-specific functionality.
// New wires it to the production API clients; tests swap in clients pointed at httptest servers.
type Server struct {
	scryfallClient *scryfall.Client
	moxfield       *moxfield.Client
	archidekt      *archidekt.Client
	edhrec         *edhrec.Client
	rules          *rules.Cache
}

// New creates a new MTG Commander MCP server.
func New() (*Server, error) {
	client, err := scryfall.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Scryfall client: %w", err)
	}

	return &Server{
		scryfallClient: client,
		moxfield:       moxfield.NewClient(),
		archidekt:      archidekt.NewClient(),
		edhrec:         edhrec.NewClient(),
		rules:          &rules.Cache{},
	}, nil
}

// Register adds every MTG tool and resource to mcpServer.
func (s *Server) Register(mcpServer *server.MCPServer) {
	s.registerTools(mcpServer)
	s.registerResources(mcpServer)
}

// registerTools registers all MCP tools.
func (s *Server) registerTools(mcpServer *server.MCPServer) {
	// Tool 1: Search Cards
	searchCardsTool := mcp.NewTool(
		"search_cards",
		mcp.WithDescription(
			"Search for Magic: The Gathering cards by name, type, color, or other criteria using Scryfall search syntax",
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query (e.g., 'sol ring', 'c:blue type:creature', 'commander')"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 10, max: 50)"),
		),
	)
	mcpServer.AddTool(searchCardsTool, s.handleSearchCards)

	// Tool 2: Get Card Details
	cardDetailsTool := mcp.NewTool(
		"get_card_details",
		mcp.WithDescription(
			"Get detailed information about a specific Magic: The Gathering card including rules text, mana cost, type, and more",
		),
		mcp.WithString(paramName,
			mcp.Required(),
			mcp.Description("Exact or fuzzy card name (e.g., 'Lightning Bolt', 'Mana Crypt')"),
		),
	)
	mcpServer.AddTool(cardDetailsTool, s.handleGetCardDetails)

	// Tool 3: Check Commander Legality
	legalityTool := mcp.NewTool(
		"check_commander_legality",
		mcp.WithDescription(
			"Check if a card is legal in Commander format and get its legality status across all formats",
		),
		mcp.WithString(paramName,
			mcp.Required(),
			mcp.Description("Card name to check legality"),
		),
	)
	mcpServer.AddTool(legalityTool, s.handleCheckLegality)

	// Tool 4: Get Card Rulings
	rulingsTool := mcp.NewTool("get_card_rulings",
		mcp.WithDescription("Get official rulings and clarifications for a Magic: The Gathering card"),
		mcp.WithString(paramName,
			mcp.Required(),
			mcp.Description("Card name to get rulings for"),
		),
	)
	mcpServer.AddTool(rulingsTool, s.handleGetRulings)

	// Tool 5: Get Card Price
	priceTool := mcp.NewTool(
		"get_card_price",
		mcp.WithDescription(
			"Get current pricing for a Magic: The Gathering card in USD, EUR, and BRL (Brazilian Real via conversion)",
		),
		mcp.WithString(paramName,
			mcp.Required(),
			mcp.Description("Card name to get pricing for"),
		),
		mcp.WithString(paramSet,
			mcp.Description("Specific set code (optional, e.g., 'MH2', 'CMR')"),
		),
	)
	mcpServer.AddTool(priceTool, s.handleGetPrice)

	// Tool 6: Get Banned List
	bannedListTool := mcp.NewTool("get_banned_list",
		mcp.WithDescription("Get the current list of cards banned in Commander format"),
	)
	mcpServer.AddTool(bannedListTool, s.handleGetBannedList)

	// Tool 7: Validate Deck
	validateDeckTool := mcp.NewTool(
		"validate_deck",
		mcp.WithDescription(
			"Validate a Commander deck for format legality (100 cards, singleton, color identity, banned cards)",
		),
		mcp.WithString(paramCommander,
			mcp.Required(),
			mcp.Description("Commander card name"),
		),
		mcp.WithString(
			"decklist",
			mcp.Required(),
			mcp.Description(
				"Decklist as JSON array of card names or newline-separated card names with quantities (e.g., '1 Sol Ring')",
			),
		),
	)
	mcpServer.AddTool(validateDeckTool, s.handleValidateDeck)

	// Tool 8: Get Moxfield Deck
	moxfieldDeckTool := mcp.NewTool(
		"get_moxfield_deck",
		mcp.WithDescription(
			"Fetch a deck from Moxfield by URL or deck ID, includes full decklist, metadata, and statistics",
		),
		mcp.WithString("deck_id",
			mcp.Required(),
			mcp.Description("Moxfield deck ID or full URL (e.g., 'abc123' or 'https://www.moxfield.com/decks/abc123')"),
		),
	)
	mcpServer.AddTool(moxfieldDeckTool, s.handleGetMoxfieldDeck)

	// Tool 9: Get User Decks from Moxfield
	moxfieldUserDecksTool := mcp.NewTool("get_moxfield_user_decks",
		mcp.WithDescription("Get a list of decks for a specific Moxfield user"),
		mcp.WithString("username",
			mcp.Required(),
			mcp.Description("Moxfield username"),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Number of decks to return (default: 20, max: 100)"),
		),
	)
	mcpServer.AddTool(moxfieldUserDecksTool, s.handleGetMoxfieldUserDecks)

	// Tool 10: Search Moxfield Decks
	searchMoxfieldDecksTool := mcp.NewTool(
		"search_moxfield_decks",
		mcp.WithDescription(
			"Search Moxfield for decks built around a commander, with pagination and sorting",
		),
		mcp.WithString(paramCommander,
			mcp.Required(),
			mcp.Description("Commander card name (e.g., 'Atraxa, Praetors' Voice')"),
		),
		mcp.WithString("format",
			mcp.Description("MTG format to filter by (default: 'commander')"),
		),
		mcp.WithString("sort",
			mcp.Description(fmt.Sprintf("Sort field: %s (default: %s)", moxfield.SortValues(), searchSortDefault)),
		),
		mcp.WithString("sort_direction",
			mcp.Description("Sort direction: asc or desc (default: desc)"),
		),
		mcp.WithNumber("page",
			mcp.Description(
				"1-based page of candidate decks searched (default: 1). Moxfield cannot filter by "+
					"commander, so a page may verify no decks while a later page verifies many.",
			),
		),
		mcp.WithNumber("limit",
			mcp.Description("Verified decks per page (default: 10, max: 20 — the per-call verification budget)"),
		),
	)
	mcpServer.AddTool(searchMoxfieldDecksTool, s.handleSearchMoxfieldDecks)

	s.registerCardImageTool(mcpServer)
	s.registerArchidektTools(mcpServer)
	s.registerRulesTools(mcpServer)
	s.registerEDHRECTools(mcpServer)
}

// registerCardImageTool registers the get_card_image tool and binds it to the
// ui://mtg-card MCP Apps widget via the tool's _meta.ui (split out of registerTools
// to keep it within the funlen limit).
func (s *Server) registerCardImageTool(mcpServer *server.MCPServer) {
	// Tool 13: Get Card Image
	cardImageTool := mcp.NewTool(
		"get_card_image",
		mcp.WithDescription(
			"Get the image of a Magic: The Gathering card, preferring the requested language and "+
				"falling back to English. Returns the image inline plus its direct URL.",
		),
		mcp.WithString(paramName,
			mcp.Required(),
			mcp.Description("Exact or fuzzy card name (e.g., 'Sol Ring', 'Delver of Secrets')"),
		),
		mcp.WithString(paramLanguage,
			mcp.Description(
				"ISO 639-1 language code for the printing (e.g., 'it', 'fr', 'de', 'ja'). "+
					"Defaults to 'en'; falls back to English when no localized printing exists.",
			),
		),
		mcp.WithString(paramSize,
			mcp.Description(
				"Image size/format: small, normal (default), large, png, art_crop, or border_crop.",
			),
		),
		mcp.WithString(paramSet,
			mcp.Description(
				"Set code to pick a specific edition's printing (e.g., 'mh2', 'c21'). "+
					"Falls back to the default printing when the card is not in that set.",
			),
		),
		mcp.WithString(paramCollectorNumber,
			mcp.Description(
				"Collector number to pin the exact printing/variant within the set (requires set; "+
					"e.g., '263'). Falls back to the default printing when not found.",
			),
		),
		mcp.WithString(paramFace,
			mcp.Description(
				"For double-faced cards, which face to return: 'front' or 'back' (default: all faces).",
			),
		),
	)
	// Associate the tool with its MCP Apps widget at registration (the spec binds UI
	// via the tool's _meta.ui, enabling the host to prefetch the resource).
	cardImageTool.Meta = mcp.NewMetaFromMap(map[string]any{
		metaKeyUI: map[string]any{metaKeyResourceURI: cardImageWidgetURI},
	})
	mcpServer.AddTool(cardImageTool, s.handleGetCardImage)
}

// registerArchidektTools registers Archidekt-related MCP tools (split out to keep registerTools within length limits).
func (s *Server) registerArchidektTools(mcpServer *server.MCPServer) {
	// Tool 14: Get Archidekt Deck
	archidektDeckTool := mcp.NewTool(
		"get_archidekt_deck",
		mcp.WithDescription(
			"Fetch a deck from Archidekt by URL or numeric deck ID, includes full decklist and metadata",
		),
		mcp.WithString("deck_id",
			mcp.Required(),
			mcp.Description("Archidekt deck ID or full URL (e.g., '12345' or 'https://archidekt.com/decks/12345')"),
		),
		mcp.WithBoolean(
			"lands_only",
			mcp.Description(
				"If true, return only the land cards. Useful for landbase comparisons without fetching the full decklist.",
			),
		),
	)
	mcpServer.AddTool(archidektDeckTool, s.handleGetArchidektDeck)

	// Tool 15: Get Archidekt User Decks
	archidektUserDecksTool := mcp.NewTool(
		"get_archidekt_user_decks",
		mcp.WithDescription("Get a list of public decks for a specific Archidekt user"),
		mcp.WithString("username",
			mcp.Required(),
			mcp.Description("Archidekt username"),
		),
		mcp.WithNumber("page",
			mcp.Description("Page number for pagination (default: 1)"),
		),
	)
	mcpServer.AddTool(archidektUserDecksTool, s.handleGetArchidektUserDecks)

	// Tool 16: Search Archidekt Decks
	searchArchidektDecksTool := mcp.NewTool(
		"search_archidekt_decks",
		mcp.WithDescription(
			"Search public Commander decks on Archidekt by commander name, with pagination and sorting",
		),
		mcp.WithString(paramCommander,
			mcp.Required(),
			mcp.Description("Commander card name (e.g., 'Atraxa, Praetors' Voice')"),
		),
		mcp.WithNumber("bracket",
			mcp.Description("Filter by EDH bracket (1–4). Omit to return all brackets."),
		),
		mcp.WithString("sort",
			mcp.Description(fmt.Sprintf("Sort field: %s (default: %s)", archidekt.SortValues(), searchSortDefault)),
		),
		mcp.WithString("sort_direction",
			mcp.Description("Sort direction: asc or desc (default: desc)"),
		),
		mcp.WithNumber("page",
			mcp.Description("1-based result page (default: 1)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Decks per page (default: 10, max: 60)"),
		),
		mcp.WithString("colors",
			mcp.Description("Filter by colour identity letters, e.g. 'WU' (accepted: W, U, B, R, G)"),
		),
		mcp.WithNumber("deck_size",
			mcp.Description("Filter by exact deck card count, e.g. 99"),
		),
		mcp.WithString("author",
			mcp.Description("Filter by Archidekt username"),
		),
		mcp.WithString(paramName,
			mcp.Description("Filter by deck-name substring"),
		),
	)
	mcpServer.AddTool(searchArchidektDecksTool, s.handleSearchArchidektDecks)
}

// registerRulesTools registers Comprehensive Rules lookup tools.
func (s *Server) registerRulesTools(mcpServer *server.MCPServer) {
	// Tool 17: Get Rule
	getRuleTool := mcp.NewTool(
		"get_rule",
		mcp.WithDescription("Get a specific Comprehensive Rule by number (e.g. '702.19'); includes its subrules"),
		mcp.WithString("rule",
			mcp.Required(),
			mcp.Description("Rule number, e.g. '702.19' or a subrule like '702.19a'"),
		),
	)
	mcpServer.AddTool(getRuleTool, s.handleGetRule)

	// Tool 18: Search Rules
	searchRulesTool := mcp.NewTool(
		"search_rules",
		mcp.WithDescription("Search the Comprehensive Rules text by keyword"),
		mcp.WithString("keyword",
			mcp.Required(),
			mcp.Description("Keyword or phrase to search for"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default: 10, max: 20)"),
		),
	)
	mcpServer.AddTool(searchRulesTool, s.handleSearchRules)

	// Tool 19: Get Glossary Term
	glossaryTool := mcp.NewTool(
		"get_glossary_term",
		mcp.WithDescription("Get a Comprehensive Rules glossary definition by term"),
		mcp.WithString("term",
			mcp.Required(),
			mcp.Description("Glossary term, e.g. 'Trample'"),
		),
	)
	mcpServer.AddTool(glossaryTool, s.handleGetGlossaryTerm)
}

// registerEDHRECTools registers the EDHREC-backed MCP tools (split out of registerTools to keep
// it within the funlen limit).
func (s *Server) registerEDHRECTools(mcpServer *server.MCPServer) {
	// Tool 11: Get EDHREC Recommendations
	edhrecRecommendationsTool := mcp.NewTool(
		"get_edhrec_recommendations",
		mcp.WithDescription(
			"Get EDHREC card recommendations for a specific commander, including high synergy cards, top cards, and statistics",
		),
		mcp.WithString(paramCommander,
			mcp.Required(),
			mcp.Description("Commander card name (e.g., 'Atraxa, Praetors Voice')"),
		),
		mcp.WithString(paramTheme,
			mcp.Description(
				"EDHREC theme slug for this commander (e.g., 'infect', 'plus-1-plus-1-counters'). "+
					"Call without a theme first: the response lists the available theme slugs.",
			),
		),
		mcp.WithString(paramPriceTier,
			mcp.Enum(edhrec.PriceTierBudget, edhrec.PriceTierExpensive),
			mcp.Description("Restrict recommendations to a price tier: 'budget' or 'expensive'"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum cards to show per category (default: 10, 0 for no limit)"),
		),
	)
	mcpServer.AddTool(edhrecRecommendationsTool, s.handleGetEDHRECRecommendations)

	// Tool 12: Get EDHREC Combos
	edhrecCombosTool := mcp.NewTool("get_edhrec_combos",
		mcp.WithDescription("Get popular card combos for a color combination from EDHREC"),
		mcp.WithString(
			"colors",
			mcp.Required(),
			mcp.Description(
				"Color combination as WUBRG letters in any order ('wu' for Azorius, 'wubrg' for five-color, "+
					"'c' for colorless) or an EDHREC name such as 'azorius', 'esper' or 'yore-tiller'",
			),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum combos to show (default: 10)"),
		),
	)
	mcpServer.AddTool(edhrecCombosTool, s.handleGetEDHRECCombos)

	// Tool 20: Get EDHREC Set Cards
	edhrecSetCardsTool := mcp.NewTool(
		"get_edhrec_set_cards",
		mcp.WithDescription(
			"Get the Commander-relevant cards of a Magic set from EDHREC: the new commanders the set "+
				"introduced with their deck counts, and the set's cards with the share of eligible "+
				"decks playing them",
		),
		mcp.WithString(paramSet,
			mcp.Required(),
			mcp.Description(
				"Short set code, lowercase (e.g., 'rna', 'c21', 'blb'). Full set names are not accepted.",
			),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum cards to show per list (default: 25, 0 for no limit)"),
		),
	)
	mcpServer.AddTool(edhrecSetCardsTool, s.handleGetEDHRECSetCards)
}

// registerResources registers MCP resources.
func (s *Server) registerResources(mcpServer *server.MCPServer) {
	// Resource 1: Commander Rules
	rulesResource := mcp.NewResource(
		"commander://rules",
		"Commander Format Rules",
		mcp.WithResourceDescription("Official Commander format rules and deck construction guidelines"),
		mcp.WithMIMEType(mimeTypeTextPlain),
	)
	mcpServer.AddResource(rulesResource, s.handleCommanderRules)

	// Resource 2: Banned List Resource
	bannedResource := mcp.NewResource(
		"commander://banned-list",
		"Commander Banned List",
		mcp.WithResourceDescription("Current list of cards banned in Commander format"),
		mcp.WithMIMEType("application/json"),
	)
	mcpServer.AddResource(bannedResource, s.handleBannedListResource)

	// Resource 3: Comprehensive Rules (full text, fetched from WotC)
	comprehensiveResource := mcp.NewResource(
		"rules://comprehensive",
		"Magic Comprehensive Rules",
		mcp.WithResourceDescription("Full official Magic: The Gathering Comprehensive Rules (large)"),
		mcp.WithMIMEType(mimeTypeTextPlain),
	)
	mcpServer.AddResource(comprehensiveResource, s.handleComprehensiveRulesResource)

	// Card image widget: a fixed MCP Apps resource. get_card_image points
	// _meta.ui.resourceUri here (on the tool + result); per-card images arrive at the
	// widget as structuredContent via the host's ui/notifications/tool-result. MCP Apps
	// does not support resource templates, so this must be a concrete resource.
	cardImageResource := mcp.NewResource(
		cardImageWidgetURI,
		"MTG Card Image",
		mcp.WithResourceDescription("Inline card image widget rendered for get_card_image"),
		mcp.WithMIMEType(mimeMCPAppHTML),
	)
	mcpServer.AddResource(cardImageResource, s.handleCardImageUIResource)
}
