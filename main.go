package main

import (
	"fmt"
	"os"

	"github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Build-time variables injected via -ldflags.
var (
	version   = "dev"
	commit    = "none"    //nolint:gochecknoglobals // build-time variable injected via -ldflags
	buildDate = "unknown" //nolint:gochecknoglobals // build-time variable injected via -ldflags
)

const (
	maxSearchLimit               = 50
	defaultSplitLimit            = 2
	maxPageSize                  = 100
	deckValidationBasicCardCount = 99
	deckValidationCommanderCount = 100
	cardSortOrderName            = "name"
	defaultFormat                = "commander"
	defaultSortDirection         = "Descending"
	paramCommander               = "commander"
	paramName                    = "name"
	paramLanguage                = "language"
	paramSize                    = "size"
	defaultCardImageLanguage     = "en"
	defaultCardImageSize         = imageSizeNormal

	defaultMoxfieldBaseURL  = "https://api.moxfield.com/v2"
	defaultArchidektBaseURL = "https://archidekt.com/api"
	defaultEDHRECBaseURL    = "https://json.edhrec.com/pages"
)

// MTGCommanderServer wraps the MCP server with MTG-specific functionality.
// The base URL fields default to the live APIs but can be overridden in tests.
type MTGCommanderServer struct {
	scryfallClient   *scryfall.Client
	moxfieldBaseURL  string
	archidektBaseURL string
	edhrecBaseURL    string
}

// NewMTGCommanderServer creates a new MTG Commander MCP server.
func NewMTGCommanderServer() (*MTGCommanderServer, error) {
	client, err := scryfall.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Scryfall client: %w", err)
	}

	return &MTGCommanderServer{
		scryfallClient:   client,
		moxfieldBaseURL:  defaultMoxfieldBaseURL,
		archidektBaseURL: defaultArchidektBaseURL,
		edhrecBaseURL:    defaultEDHRECBaseURL,
	}, nil
}

func main() {
	// Handle flags before logger init so --version can print cleanly to stdout
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version":
			_, _ = fmt.Fprintf(os.Stdout, "mtg-mcp version %s (commit: %s, built: %s)\n", version, commit, buildDate)
			os.Exit(0)
		}
	}

	// Initialize logger
	logFilePath := "mtg-commander-server.log"
	if err := InitLogger(logFilePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log := GetLogger()
	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("build_date", buildDate).
		Msg("Initializing MTG Commander MCP Server")

	// Check for log level flag
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		SetLogLevel(logLevelDebug)
		log.Debug().Msg("Debug logging enabled")
	}

	// Create MTG Commander server instance
	log.Info().Msg("Creating MTG Commander server instance")
	mtgServer, err := NewMTGCommanderServer()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create MTG Commander server")
	}
	log.Info().Msg("MTG Commander server instance created successfully")

	// Create MCP server
	log.Info().Msg("Creating MCP server")
	mcpServer := server.NewMCPServer(
		"MTG Commander Assistant",
		"1.0.0",
		server.WithRecovery(), // Add panic recovery middleware
	)

	// Register all tools
	log.Info().Msg("Registering MCP tools")
	mtgServer.registerTools(mcpServer)
	log.Info().Msg("All tools registered successfully")

	// Register resources
	log.Info().Msg("Registering MCP resources")
	mtgServer.registerResources(mcpServer)
	log.Info().Msg("All resources registered successfully")

	// Start server with stdio transport
	log.Info().
		Str("transport", "stdio").
		Str("log_file", logFilePath).
		Msg("Starting MTG Commander MCP Server")

	if serveErr := server.ServeStdio(mcpServer); serveErr != nil {
		log.Fatal().Err(serveErr).Msg("Server error")
	}
}

// registerTools registers all MCP tools.
func (s *MTGCommanderServer) registerTools(mcpServer *server.MCPServer) {
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
		mcp.WithString("set",
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
			"Search for decks on Moxfield by commander name or other criteria, returns popular decks sorted by views/likes",
		),
		mcp.WithString(paramCommander,
			mcp.Required(),
			mcp.Description("Commander card name to search for (e.g., 'Atraxa, Praetors Voice')"),
		),
		mcp.WithString("format",
			mcp.Description("MTG format to filter by (default: 'commander')"),
		),
		mcp.WithString("sort_type",
			mcp.Description("Sort type: 'updated', 'views', 'likes' (default: 'updated')"),
		),
		mcp.WithString("sort_direction",
			mcp.Description("Sort direction: 'Ascending' or 'Descending' (default: 'Descending')"),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Number of decks to return (default: 20, max: 100)"),
		),
	)
	mcpServer.AddTool(searchMoxfieldDecksTool, s.handleSearchMoxfieldDecks)

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
		mcp.WithNumber("limit",
			mcp.Description("Maximum cards to show per category (default: 10)"),
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
				"Color combination (w=white, u=blue, b=black, r=red, g=green, e.g., 'wu' for Azorius, 'wubrg' for 5-color)",
			),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum combos to show (default: 10)"),
		),
	)
	mcpServer.AddTool(edhrecCombosTool, s.handleGetEDHRECCombos)

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
	)
	mcpServer.AddTool(cardImageTool, s.handleGetCardImage)

	s.registerArchidektTools(mcpServer)
}

// registerArchidektTools registers Archidekt-related MCP tools (split out to keep registerTools within length limits).
func (s *MTGCommanderServer) registerArchidektTools(mcpServer *server.MCPServer) {
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
			"Search for public Commander decks on Archidekt by commander name, sorted by view count descending",
		),
		mcp.WithString("commander",
			mcp.Required(),
			mcp.Description("Commander card name to search for (e.g. 'Atraxa, Praetors\\' Voice')"),
		),
		mcp.WithNumber("bracket",
			mcp.Description("Filter by EDH bracket (1–4). Omit to return all brackets."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Number of decks to return (default: 10, max: 20)"),
		),
	)
	mcpServer.AddTool(searchArchidektDecksTool, s.handleSearchArchidektDecks)
}

// registerResources registers MCP resources.
func (s *MTGCommanderServer) registerResources(mcpServer *server.MCPServer) {
	// Resource 1: Commander Rules
	rulesResource := mcp.NewResource(
		"commander://rules",
		"Commander Format Rules",
		mcp.WithResourceDescription("Official Commander format rules and deck construction guidelines"),
		mcp.WithMIMEType("text/plain"),
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

	// Resource template: card image widget. get_card_image points _meta.ui.resourceUri
	// at a ui://mtg-card/<payload> URI matching this template; the host reads it to
	// render the MCP Apps widget with the inlined card image(s).
	cardImageTemplate := mcp.NewResourceTemplate(
		cardImageURITemplate,
		"MTG Card Image",
		mcp.WithTemplateDescription("Inline card image widget rendered for get_card_image"),
		mcp.WithTemplateMIMEType(mimeMCPAppHTML),
	)
	mcpServer.AddResourceTemplate(cardImageTemplate, s.handleCardImageUIResource)

	// TEMPORARY: UI-render probe (tool + ui:// resource). Remove once resolved.
	s.registerUIRenderTest(mcpServer)
}
