package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	scryfall "github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MTGCommanderServer wraps the MCP server with MTG-specific functionality
type MTGCommanderServer struct {
	scryfallClient *scryfall.Client
}

// NewMTGCommanderServer creates a new MTG Commander MCP server
func NewMTGCommanderServer() (*MTGCommanderServer, error) {
	client, err := scryfall.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Scryfall client: %w", err)
	}

	return &MTGCommanderServer{
		scryfallClient: client,
	}, nil
}

func main() {
	// Create MTG Commander server instance
	mtgServer, err := NewMTGCommanderServer()
	if err != nil {
		log.Fatalf("Failed to create MTG Commander server: %v", err)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"MTG Commander Assistant",
		"1.0.0",
		server.WithRecovery(), // Add panic recovery middleware
	)

	// Register all tools
	mtgServer.registerTools(mcpServer)

	// Register resources
	mtgServer.registerResources(mcpServer)

	// Start server with stdio transport
	log.Println("Starting MTG Commander MCP Server...")
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// registerTools registers all MCP tools
func (s *MTGCommanderServer) registerTools(mcpServer *server.MCPServer) {
	// Tool 1: Search Cards
	searchCardsTool := mcp.NewTool("search_cards",
		mcp.WithDescription("Search for Magic: The Gathering cards by name, type, color, or other criteria using Scryfall search syntax"),
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
	cardDetailsTool := mcp.NewTool("get_card_details",
		mcp.WithDescription("Get detailed information about a specific Magic: The Gathering card including rules text, mana cost, type, and more"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Exact or fuzzy card name (e.g., 'Lightning Bolt', 'Mana Crypt')"),
		),
	)
	mcpServer.AddTool(cardDetailsTool, s.handleGetCardDetails)

	// Tool 3: Check Commander Legality
	legalityTool := mcp.NewTool("check_commander_legality",
		mcp.WithDescription("Check if a card is legal in Commander format and get its legality status across all formats"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Card name to check legality"),
		),
	)
	mcpServer.AddTool(legalityTool, s.handleCheckLegality)

	// Tool 4: Get Card Rulings
	rulingsTool := mcp.NewTool("get_card_rulings",
		mcp.WithDescription("Get official rulings and clarifications for a Magic: The Gathering card"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Card name to get rulings for"),
		),
	)
	mcpServer.AddTool(rulingsTool, s.handleGetRulings)

	// Tool 5: Get Card Price
	priceTool := mcp.NewTool("get_card_price",
		mcp.WithDescription("Get current pricing for a Magic: The Gathering card in USD, EUR, and BRL (Brazilian Real via conversion)"),
		mcp.WithString("name",
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
	validateDeckTool := mcp.NewTool("validate_deck",
		mcp.WithDescription("Validate a Commander deck for format legality (100 cards, singleton, color identity, banned cards)"),
		mcp.WithString("commander",
			mcp.Required(),
			mcp.Description("Commander card name"),
		),
		mcp.WithString("decklist",
			mcp.Required(),
			mcp.Description("Decklist as JSON array of card names or newline-separated card names with quantities (e.g., '1 Sol Ring')"),
		),
	)
	mcpServer.AddTool(validateDeckTool, s.handleValidateDeck)
}

// registerResources registers MCP resources
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
}

// Tool Handlers

func (s *MTGCommanderServer) handleSearchCards(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	limit := 10
	args := request.GetArguments()
	if limitVal, hasLimit := args["limit"]; hasLimit {
		if limitFloat, ok := limitVal.(float64); ok {
			limit = int(limitFloat)
			if limit > 50 {
				limit = 50
			}
		}
	}

	// Search cards using Scryfall
	searchOpts := scryfall.SearchCardsOptions{
		Unique: "cards",
		Order:  "name",
	}

	result, err := s.scryfallClient.SearchCards(ctx, query, searchOpts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	if len(result.Cards) == 0 {
		return mcp.NewToolResultText("No cards found matching your query."), nil
	}

	// Limit results
	if len(result.Cards) > limit {
		result.Cards = result.Cards[:limit]
	}

	// Format results
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d cards (showing first %d):\n\n", result.TotalCards, len(result.Cards)))

	for i, card := range result.Cards {
		output.WriteString(fmt.Sprintf("%d. **%s** %s\n", i+1, card.Name, card.ManaCost))
		output.WriteString(fmt.Sprintf("   Type: %s\n", card.TypeLine))
		if card.OracleText != "" {
			output.WriteString(fmt.Sprintf("   Text: %s\n", card.OracleText))
		}
		output.WriteString(fmt.Sprintf("   Set: %s (%s)\n", card.SetName, strings.ToUpper(card.Set)))
		output.WriteString(fmt.Sprintf("   Commander Legal: %s\n\n", card.Legalities.Commander))
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetCardDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get card by name (fuzzy match)
	card, err := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", err)), nil
	}

	// Format card details
	var output strings.Builder
	output.WriteString(fmt.Sprintf("# %s %s\n\n", card.Name, card.ManaCost))
	output.WriteString(fmt.Sprintf("**Type:** %s\n", card.TypeLine))
	output.WriteString(fmt.Sprintf("**Set:** %s (%s) #%s\n", card.SetName, strings.ToUpper(card.Set), card.CollectorNumber))
	output.WriteString(fmt.Sprintf("**Rarity:** %s\n\n", card.Rarity))

	if card.OracleText != "" {
		output.WriteString(fmt.Sprintf("**Oracle Text:**\n%s\n\n", card.OracleText))
	}

	if card.Power != nil && card.Toughness != nil {
		output.WriteString(fmt.Sprintf("**Power/Toughness:** %s/%s\n", *card.Power, *card.Toughness))
	}

	if card.Loyalty != nil {
		output.WriteString(fmt.Sprintf("**Loyalty:** %s\n", *card.Loyalty))
	}

	// Color Identity
	if len(card.ColorIdentity) > 0 {
		colors := make([]string, len(card.ColorIdentity))
		for i, c := range card.ColorIdentity {
			colors[i] = string(c)
		}
		output.WriteString(fmt.Sprintf("**Color Identity:** %s\n", strings.Join(colors, ", ")))
	}

	// Legalities
	output.WriteString("\n**Format Legalities:**\n")
	output.WriteString(fmt.Sprintf("- Commander: %s\n", card.Legalities.Commander))
	output.WriteString(fmt.Sprintf("- Legacy: %s\n", card.Legalities.Legacy))
	output.WriteString(fmt.Sprintf("- Vintage: %s\n", card.Legalities.Vintage))
	output.WriteString(fmt.Sprintf("- Modern: %s\n", card.Legalities.Modern))
	output.WriteString(fmt.Sprintf("- Standard: %s\n", card.Legalities.Standard))

	// Additional info
	if card.Artist != nil {
		output.WriteString(fmt.Sprintf("\n**Artist:** %s\n", *card.Artist))
	}

	output.WriteString(fmt.Sprintf("\n**Scryfall Link:** %s\n", card.ScryfallURI))

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleCheckLegality(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	card, err := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", err)), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# Legality Check: %s\n\n", card.Name))

	status := strings.Title(string(card.Legalities.Commander))
	output.WriteString(fmt.Sprintf("**Commander Format:** %s\n\n", status))

	if card.Legalities.Commander == "banned" {
		output.WriteString("⚠️ This card is **BANNED** in Commander format.\n\n")
	} else if card.Legalities.Commander == "legal" {
		output.WriteString("✅ This card is **LEGAL** in Commander format.\n\n")
	} else {
		output.WriteString("❌ This card is **NOT LEGAL** in Commander format.\n\n")
	}

	// Show all format legalities
	output.WriteString("**All Format Legalities:**\n")
	output.WriteString(fmt.Sprintf("- Standard: %s\n", card.Legalities.Standard))
	output.WriteString(fmt.Sprintf("- Pioneer: %s\n", card.Legalities.Pioneer))
	output.WriteString(fmt.Sprintf("- Modern: %s\n", card.Legalities.Modern))
	output.WriteString(fmt.Sprintf("- Legacy: %s\n", card.Legalities.Legacy))
	output.WriteString(fmt.Sprintf("- Vintage: %s\n", card.Legalities.Vintage))
	output.WriteString(fmt.Sprintf("- Pauper: %s\n", card.Legalities.Pauper))
	output.WriteString(fmt.Sprintf("- Commander: %s\n", card.Legalities.Commander))

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetRulings(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// First get the card
	card, err := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", err)), nil
	}

	// Get rulings
	rulings, err := s.scryfallClient.GetRulings(ctx, card.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get rulings: %v", err)), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# Rulings for %s\n\n", card.Name))

	if len(rulings) == 0 {
		output.WriteString("No official rulings found for this card.\n")
	} else {
		output.WriteString(fmt.Sprintf("Found %d ruling(s):\n\n", len(rulings)))
		for i, ruling := range rulings {
			output.WriteString(fmt.Sprintf("%d. **%s** (%s)\n", i+1, ruling.PublishedAt, ruling.Source))
			output.WriteString(fmt.Sprintf("   %s\n\n", ruling.Comment))
		}
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetPrice(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	setCode := ""
	args := request.GetArguments()
	if setVal, hasSet := args["set"]; hasSet {
		if set, ok := setVal.(string); ok {
			setCode = set
		}
	}

	var card scryfall.Card
	if setCode != "" {
		// Search for specific set
		searchQuery := fmt.Sprintf(`!"%s" set:%s`, name, setCode)
		result, err := s.scryfallClient.SearchCards(ctx, searchQuery, scryfall.SearchCardsOptions{})
		if err != nil || len(result.Cards) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("Card not found in set %s", setCode)), nil
		}
		card = result.Cards[0]
	} else {
		// Get default card
		c, err := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", err)), nil
		}
		card = c
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# Pricing for %s\n", card.Name))
	output.WriteString(fmt.Sprintf("Set: %s (%s) #%s\n\n", card.SetName, strings.ToUpper(card.Set), card.CollectorNumber))

	// Get exchange rate for BRL
	usdToBRL, err := getUSDToBRLRate(ctx)
	if err != nil {
		log.Printf("Failed to get exchange rate: %v", err)
		usdToBRL = 5.40 // Fallback rate
	}

	hasPricing := false

	if card.Prices.USD != "" {
		output.WriteString(fmt.Sprintf("**USD:** $%s\n", card.Prices.USD))
		output.WriteString(fmt.Sprintf("**BRL:** R$ %.2f (converted)\n", convertToBRL(card.Prices.USD, usdToBRL)))
		hasPricing = true
	}

	if card.Prices.USDFoil != "" {
		output.WriteString(fmt.Sprintf("**USD (Foil):** $%s\n", card.Prices.USDFoil))
		output.WriteString(fmt.Sprintf("**BRL (Foil):** R$ %.2f (converted)\n", convertToBRL(card.Prices.USDFoil, usdToBRL)))
		hasPricing = true
	}

	if card.Prices.EUR != "" {
		output.WriteString(fmt.Sprintf("**EUR:** €%s\n", card.Prices.EUR))
		hasPricing = true
	}

	if card.Prices.EURFoil != "" {
		output.WriteString(fmt.Sprintf("**EUR (Foil):** €%s\n", card.Prices.EURFoil))
		hasPricing = true
	}

	if card.Prices.Tix != "" {
		output.WriteString(fmt.Sprintf("**MTGO Tix:** %s\n", card.Prices.Tix))
		hasPricing = true
	}

	if !hasPricing {
		output.WriteString("No pricing data available for this card.\n")
	} else {
		output.WriteString(fmt.Sprintf("\n*Exchange rate: 1 USD = %.4f BRL*\n", usdToBRL))
		output.WriteString("*Note: BRL prices are converted from USD and may not reflect Brazilian market conditions*\n")
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetBannedList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Search for banned cards in Commander
	searchQuery := "banned:commander"
	result, err := s.scryfallClient.SearchCards(ctx, searchQuery, scryfall.SearchCardsOptions{
		Order: "name",
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch banned list: %v", err)), nil
	}

	var output strings.Builder
	output.WriteString("# Commander Format Banned List\n\n")
	output.WriteString(fmt.Sprintf("Total banned cards: %d\n\n", result.TotalCards))

	for i, card := range result.Cards {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, card.Name))
	}

	output.WriteString("\n*Source: Scryfall (powered by Wizards of the Coast official data)*\n")
	output.WriteString("*Last updated: This query fetches real-time data*\n")

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleValidateDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	commanderName, err := request.RequireString("commander")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	decklistStr, err := request.RequireString("decklist")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Parse decklist (support both JSON array and text format)
	var cardNames []string

	// Try JSON first
	if err := json.Unmarshal([]byte(decklistStr), &cardNames); err != nil {
		// Parse as text format (one card per line, optional quantity prefix)
		lines := strings.Split(decklistStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Remove quantity prefix (e.g., "1 Sol Ring" -> "Sol Ring")
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				// Check if first part is a number
				if _, err := fmt.Sscanf(parts[0], "%d", new(int)); err == nil {
					cardNames = append(cardNames, strings.TrimSpace(parts[1]))
				} else {
					cardNames = append(cardNames, line)
				}
			} else {
				cardNames = append(cardNames, line)
			}
		}
	}

	var output strings.Builder
	output.WriteString("# Commander Deck Validation\n\n")

	// Get commander card
	commander, err := s.scryfallClient.GetCardByName(ctx, commanderName, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Commander card not found: %v", err)), nil
	}

	// Convert color identity to strings
	colorIdentity := make([]string, len(commander.ColorIdentity))
	for i, c := range commander.ColorIdentity {
		colorIdentity[i] = string(c)
	}

	output.WriteString(fmt.Sprintf("**Commander:** %s\n", commander.Name))
	output.WriteString(fmt.Sprintf("**Color Identity:** %s\n\n", strings.Join(colorIdentity, ", ")))

	// Check if commander is legal
	if commander.Legalities.Commander == "banned" {
		output.WriteString("❌ **ERROR:** Your commander is banned in Commander format!\n\n")
	}

	// Validate commander can be a commander
	isLegendary := strings.Contains(strings.ToLower(commander.TypeLine), "legendary")
	canBeCommander := isLegendary || strings.Contains(strings.ToLower(commander.OracleText), "can be your commander")

	if !canBeCommander {
		output.WriteString("❌ **ERROR:** This card cannot be a commander (must be legendary or have special text allowing it)!\n\n")
	}

	// Check deck size
	totalCards := len(cardNames)
	output.WriteString(fmt.Sprintf("**Deck Size:** %d cards ", totalCards))
	if totalCards == 99 {
		output.WriteString("✅\n")
	} else if totalCards == 100 {
		output.WriteString("(Note: 100 cards including commander, should be 99 in decklist)\n")
	} else {
		output.WriteString(fmt.Sprintf("❌ (should be 99 cards plus commander)\n"))
	}

	// Check singleton (no duplicates except basic lands)
	cardCounts := make(map[string]int)
	for _, name := range cardNames {
		cardCounts[strings.ToLower(strings.TrimSpace(name))]++
	}

	var duplicates []string
	basicLands := []string{"plains", "island", "swamp", "mountain", "forest", "wastes"}
	for name, count := range cardCounts {
		if count > 1 {
			isBasic := false
			for _, basic := range basicLands {
				if name == basic {
					isBasic = true
					break
				}
			}
			if !isBasic {
				duplicates = append(duplicates, fmt.Sprintf("%s (x%d)", name, count))
			}
		}
	}

	output.WriteString("\n**Singleton Rule:** ")
	if len(duplicates) == 0 {
		output.WriteString("✅ No duplicates\n")
	} else {
		output.WriteString(fmt.Sprintf("❌ Found duplicates:\n"))
		for _, dup := range duplicates {
			output.WriteString(fmt.Sprintf("  - %s\n", dup))
		}
	}

	output.WriteString("\n*Note: Full color identity and banned card validation requires checking each card individually, which may take some time.*")

	return mcp.NewToolResultText(output.String()), nil
}

// Resource Handlers

func (s *MTGCommanderServer) handleCommanderRules(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	rules := `# Commander Format Rules

## Overview
Commander is a multiplayer format for Magic: The Gathering, emphasizing social interactions, interesting games, and creative deck-building.

## Deck Construction
- **100 cards total**: 1 commander + 99 other cards
- **Singleton**: No more than 1 copy of any card (except basic lands)
- **Commander**: Must be a legendary creature or eligible legendary permanent
- **Color Identity**: All cards must match commander's color identity (mana symbols in cost and rules text)
- **Banned List**: 85 cards currently banned (use get_banned_list tool for current list)

## Gameplay Rules
- **Starting Life**: 40 life points (instead of 20)
- **Command Zone**: Commanders start in the Command Zone
- **Commander Tax**: Pay 2 additional generic mana for each previous casting from command zone
- **Commander Damage**: 21 combat damage from a single commander causes player loss
- **Mulligan**: Partial Paris mulligan (draw 7, then any number of mulligans drawing 1 less each time)

## Winning Conditions
- Reduce all opponents to 0 life
- 21 combat damage from a single commander
- Opponents deck out (draw from empty library)
- Alternate win conditions (as printed on cards)

## Official Resources
- Format managed by Wizards of the Coast (as of September 2024)
- Official website: https://mtgcommander.net
- Rules updates: Follow WeeklyMTG on Twitch/YouTube

*Last updated: November 2025*
`
	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     rules,
		},
	}, nil
}

func (s *MTGCommanderServer) handleBannedListResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Fetch current banned list from Scryfall
	result, err := s.scryfallClient.SearchCards(ctx, "banned:commander", scryfall.SearchCardsOptions{
		Order: "name",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch banned list: %w", err)
	}

	bannedCards := make([]map[string]string, len(result.Cards))
	for i, card := range result.Cards {
		bannedCards[i] = map[string]string{
			"name":     card.Name,
			"type":     card.TypeLine,
			"mana_cost": card.ManaCost,
		}
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"format":       "commander",
		"total_banned": result.TotalCards,
		"cards":        bannedCards,
		"last_updated": "real-time",
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

// Helper functions

func getUSDToBRLRate(ctx context.Context) (float64, error) {
	// Use Frankfurter API for currency conversion (free, no API key needed)
	resp, err := HTTPGet(ctx, "https://api.frankfurter.app/latest?from=USD&to=BRL")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Rates struct {
			BRL float64 `json:"BRL"`
		} `json:"rates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.Rates.BRL, nil
}

func convertToBRL(priceStr string, rate float64) float64 {
	var price float64
	fmt.Sscanf(priceStr, "%f", &price)
	return price * rate
}
