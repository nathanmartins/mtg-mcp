package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MTGCommanderServer) handleSearchCards(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		GetLogger().Error().Err(err).Str("tool", "search_cards").Msg("Missing required query parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	const defaultLimit = 10
	limit := defaultLimit
	args := request.GetArguments()
	if limitVal, hasLimit := args["limit"]; hasLimit {
		if limitFloat, ok := limitVal.(float64); ok {
			limit = int(limitFloat)
			if limit > maxSearchLimit {
				limit = maxSearchLimit
			}
		}
	}

	GetLogger().Info().
		Str("tool", "search_cards").
		Str("query", query).
		Int("limit", limit).
		Msg("Searching for cards")

	searchOpts := scryfall.SearchCardsOptions{
		Unique: "cards",
		Order:  cardSortOrderName,
	}

	result, err := s.scryfallClient.SearchCards(ctx, query, searchOpts)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "search_cards").
			Str("query", query).
			Msg("Scryfall search failed")
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	if len(result.Cards) == 0 {
		GetLogger().Info().
			Str("tool", "search_cards").
			Str("query", query).
			Msg("No cards found")
		return mcp.NewToolResultText("No cards found matching your query."), nil
	}

	GetLogger().Info().
		Str("tool", "search_cards").
		Str("query", query).
		Int("results_found", result.TotalCards).
		Int("results_returned", len(result.Cards)).
		Msg("Search completed successfully")

	if len(result.Cards) > limit {
		result.Cards = result.Cards[:limit]
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "Found %d cards (showing first %d):\n\n", result.TotalCards, len(result.Cards))

	for i, card := range result.Cards {
		_, _ = fmt.Fprintf(&output, "%d. **%s** %s\n", i+1, card.Name, card.ManaCost)
		_, _ = fmt.Fprintf(&output, "   Type: %s\n", card.TypeLine)
		if card.OracleText != "" {
			_, _ = fmt.Fprintf(&output, "   Text: %s\n", card.OracleText)
		}
		_, _ = fmt.Fprintf(&output, "   Set: %s (%s)\n", card.SetName, strings.ToUpper(card.Set))
		_, _ = fmt.Fprintf(&output, "   Commander Legal: %s\n\n", card.Legalities.Commander)
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetCardDetails(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	name, err := request.RequireString(paramName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	card, err := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", err)), nil
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# %s %s\n\n", card.Name, card.ManaCost)
	_, _ = fmt.Fprintf(&output, "**Type:** %s\n", card.TypeLine)
	_, _ = fmt.Fprintf(&output, "**Set:** %s (%s) #%s\n", card.SetName, strings.ToUpper(card.Set), card.CollectorNumber)
	_, _ = fmt.Fprintf(&output, "**Rarity:** %s\n\n", card.Rarity)

	if card.OracleText != "" {
		_, _ = fmt.Fprintf(&output, "**Oracle Text:**\n%s\n\n", card.OracleText)
	}

	if card.Power != nil && card.Toughness != nil {
		_, _ = fmt.Fprintf(&output, "**Power/Toughness:** %s/%s\n", *card.Power, *card.Toughness)
	}

	if card.Loyalty != nil {
		_, _ = fmt.Fprintf(&output, "**Loyalty:** %s\n", *card.Loyalty)
	}

	if len(card.ColorIdentity) > 0 {
		colors := make([]string, len(card.ColorIdentity))
		for i, c := range card.ColorIdentity {
			colors[i] = string(c)
		}
		_, _ = fmt.Fprintf(&output, "**Color Identity:** %s\n", strings.Join(colors, ", "))
	}

	output.WriteString("\n**Format Legalities:**\n")
	_, _ = fmt.Fprintf(&output, "- Commander: %s\n", card.Legalities.Commander)
	_, _ = fmt.Fprintf(&output, "- Legacy: %s\n", card.Legalities.Legacy)
	_, _ = fmt.Fprintf(&output, "- Vintage: %s\n", card.Legalities.Vintage)
	_, _ = fmt.Fprintf(&output, "- Modern: %s\n", card.Legalities.Modern)
	_, _ = fmt.Fprintf(&output, "- Standard: %s\n", card.Legalities.Standard)

	if card.Artist != nil {
		_, _ = fmt.Fprintf(&output, "\n**Artist:** %s\n", *card.Artist)
	}

	_, _ = fmt.Fprintf(&output, "\n**Scryfall Link:** %s\n", card.ScryfallURI)

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleCheckLegality(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	name, err := request.RequireString(paramName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	card, err := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", err)), nil
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# Legality Check: %s\n\n", card.Name)

	legality := string(card.Legalities.Commander)
	status := strings.ToUpper(legality[:1]) + legality[1:]
	_, _ = fmt.Fprintf(&output, "**Commander Format:** %s\n\n", status)

	switch card.Legalities.Commander {
	case scryfall.LegalityBanned:
		output.WriteString("⚠️ This card is **BANNED** in Commander format.\n\n")
	case scryfall.LegalityLegal:
		output.WriteString("✅ This card is **LEGAL** in Commander format.\n\n")
	case scryfall.LegalityNotLegal, scryfall.LegalityRestricted:
		output.WriteString("❌ This card is **NOT LEGAL** in Commander format.\n\n")
	}

	output.WriteString("**All Format Legalities:**\n")
	_, _ = fmt.Fprintf(&output, "- Standard: %s\n", card.Legalities.Standard)
	_, _ = fmt.Fprintf(&output, "- Pioneer: %s\n", card.Legalities.Pioneer)
	_, _ = fmt.Fprintf(&output, "- Modern: %s\n", card.Legalities.Modern)
	_, _ = fmt.Fprintf(&output, "- Legacy: %s\n", card.Legalities.Legacy)
	_, _ = fmt.Fprintf(&output, "- Vintage: %s\n", card.Legalities.Vintage)
	_, _ = fmt.Fprintf(&output, "- Pauper: %s\n", card.Legalities.Pauper)
	_, _ = fmt.Fprintf(&output, "- Commander: %s\n", card.Legalities.Commander)

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetRulings(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	name, err := request.RequireString(paramName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	card, err := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", err)), nil
	}

	rulings, err := s.scryfallClient.GetRulings(ctx, card.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get rulings: %v", err)), nil
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# Rulings for %s\n\n", card.Name)

	if len(rulings) == 0 {
		output.WriteString("No official rulings found for this card.\n")
	} else {
		_, _ = fmt.Fprintf(&output, "Found %d ruling(s):\n\n", len(rulings))
		for i, ruling := range rulings {
			_, _ = fmt.Fprintf(&output, "%d. **%s** (%s)\n", i+1, ruling.PublishedAt, ruling.Source)
			_, _ = fmt.Fprintf(&output, "   %s\n\n", ruling.Comment)
		}
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetPrice(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	name, err := request.RequireString(paramName)
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
		searchQuery := fmt.Sprintf(`!"%s" set:%s`, name, setCode)
		result, searchErr := s.scryfallClient.SearchCards(ctx, searchQuery, scryfall.SearchCardsOptions{})
		if searchErr != nil {
			return nil, fmt.Errorf("failed to search for card: %w", searchErr)
		}
		if len(result.Cards) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("Card not found in set %s", setCode)), nil
		}
		card = result.Cards[0]
	} else {
		c, getErr := s.scryfallClient.GetCardByName(ctx, name, false, scryfall.GetCardByNameOptions{})
		if getErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Card not found: %v", getErr)), nil
		}
		card = c
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# Pricing for %s\n", card.Name)
	_, _ = fmt.Fprintf(&output, "Set: %s (%s) #%s\n\n", card.SetName, strings.ToUpper(card.Set), card.CollectorNumber)

	usdToBRL, err := exchangeRateFetcher(ctx)
	if err != nil {
		GetLogger().Warn().Err(err).Msg("Failed to get exchange rate, using fallback")
		usdToBRL = 5.40
	}

	hasPricing := false

	if card.Prices.USD != "" {
		_, _ = fmt.Fprintf(&output, "**USD:** $%s\n", card.Prices.USD)
		_, _ = fmt.Fprintf(&output, "**BRL:** R$ %.2f (converted)\n", convertToBRL(card.Prices.USD, usdToBRL))
		hasPricing = true
	}

	if card.Prices.USDFoil != "" {
		_, _ = fmt.Fprintf(&output, "**USD (Foil):** $%s\n", card.Prices.USDFoil)
		_, _ = fmt.Fprintf(
			&output,
			"**BRL (Foil):** R$ %.2f (converted)\n",
			convertToBRL(card.Prices.USDFoil, usdToBRL),
		)
		hasPricing = true
	}

	if card.Prices.EUR != "" {
		_, _ = fmt.Fprintf(&output, "**EUR:** €%s\n", card.Prices.EUR)
		hasPricing = true
	}

	if card.Prices.EURFoil != "" {
		_, _ = fmt.Fprintf(&output, "**EUR (Foil):** €%s\n", card.Prices.EURFoil)
		hasPricing = true
	}

	if card.Prices.Tix != "" {
		_, _ = fmt.Fprintf(&output, "**MTGO Tix:** %s\n", card.Prices.Tix)
		hasPricing = true
	}

	if !hasPricing {
		output.WriteString("No pricing data available for this card.\n")
	} else {
		_, _ = fmt.Fprintf(&output, "\n*Exchange rate: 1 USD = %.4f BRL*\n", usdToBRL)
		output.WriteString(
			"*Note: BRL prices are converted from USD and may not reflect Brazilian market conditions*\n",
		)
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleGetBannedList(
	ctx context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	searchQuery := "banned:commander"
	result, err := s.scryfallClient.SearchCards(ctx, searchQuery, scryfall.SearchCardsOptions{
		Order: cardSortOrderName,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch banned list: %v", err)), nil
	}

	var output strings.Builder
	output.WriteString("# Commander Format Banned List\n\n")
	_, _ = fmt.Fprintf(&output, "Total banned cards: %d\n\n", result.TotalCards)

	for i, card := range result.Cards {
		_, _ = fmt.Fprintf(&output, "%d. %s\n", i+1, card.Name)
	}

	output.WriteString("\n*Source: Scryfall (powered by Wizards of the Coast official data)*\n")
	output.WriteString("*Last updated: This query fetches real-time data*\n")

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleValidateDeck(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	commanderName, err := request.RequireString(paramCommander)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	decklistStr, err := request.RequireString("decklist")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cardNames := parseDecklistString(decklistStr)

	var output strings.Builder
	output.WriteString("# Commander Deck Validation\n\n")

	commander, err := s.scryfallClient.GetCardByName(ctx, commanderName, false, scryfall.GetCardByNameOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Commander card not found: %v", err)), nil
	}

	colorIdentity := make([]string, len(commander.ColorIdentity))
	for i, c := range commander.ColorIdentity {
		colorIdentity[i] = string(c)
	}

	_, _ = fmt.Fprintf(&output, "**Commander:** %s\n", commander.Name)
	_, _ = fmt.Fprintf(&output, "**Color Identity:** %s\n\n", strings.Join(colorIdentity, ", "))

	if commander.Legalities.Commander == scryfall.LegalityBanned {
		output.WriteString("❌ **ERROR:** Your commander is banned in Commander format!\n\n")
	}

	isLegendary := strings.Contains(strings.ToLower(commander.TypeLine), "legendary")
	canBeCommander := isLegendary || strings.Contains(strings.ToLower(commander.OracleText), "can be your commander")

	if !canBeCommander {
		output.WriteString(
			"❌ **ERROR:** This card cannot be a commander (must be legendary or have special text allowing it)!\n\n",
		)
	}

	totalCards := len(cardNames)
	_, _ = fmt.Fprintf(&output, "**Deck Size:** %d cards ", totalCards)
	switch totalCards {
	case deckValidationBasicCardCount:
		output.WriteString("✅\n")
	case deckValidationCommanderCount:
		output.WriteString("(Note: 100 cards including commander, should be 99 in decklist)\n")
	default:
		output.WriteString("❌ (should be 99 cards plus commander)\n")
	}

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
		output.WriteString("❌ Found duplicates:\n")
		for _, dup := range duplicates {
			_, _ = fmt.Fprintf(&output, "  - %s\n", dup)
		}
	}

	output.WriteString(
		"\n*Note: Full color identity and banned card validation requires checking each card individually, which may take some time.*",
	)

	return mcp.NewToolResultText(output.String()), nil
}

// parseDecklistString converts a decklist from JSON or text format to card names.
func parseDecklistString(decklistStr string) []string {
	var cardNames []string

	if unmarshalErr := json.Unmarshal([]byte(decklistStr), &cardNames); unmarshalErr != nil {
		cardNames = parseTextDecklist(decklistStr)
	}

	return cardNames
}

// parseTextDecklist parses a text-format decklist into card names.
// Supports grouped quantities (e.g., "9 Plains" produces 9 entries).
func parseTextDecklist(decklistStr string) []string {
	var cardNames []string
	lines := strings.Split(decklistStr, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, qty := parseCardLine(line)
		for range qty {
			cardNames = append(cardNames, name)
		}
	}

	return cardNames
}

// parseCardLine extracts card name and quantity from a line (e.g., "4 Lightning Bolt" -> ("Lightning Bolt", 4)).
func parseCardLine(line string) (string, int) {
	parts := strings.SplitN(line, " ", defaultSplitLimit)
	if len(parts) != defaultSplitLimit {
		return line, 1
	}

	var qty int
	if _, scanErr := fmt.Sscanf(parts[0], "%d", &qty); scanErr == nil && qty > 0 {
		return strings.TrimSpace(parts[1]), qty
	}

	return line, 1
}

const exchangeRateURL = "https://api.frankfurter.app/latest?from=USD&to=BRL"

// httpGetter performs an HTTP GET; HTTPGet satisfies it in production, and a stub satisfies it in tests.
type httpGetter func(ctx context.Context, rawURL string) (*http.Response, error)

// exchangeRateFetcher fetches the current USD to BRL exchange rate.
// It is a package variable, so tests can substitute a deterministic implementation.
var exchangeRateFetcher = getUSDToBRLRate //nolint:gochecknoglobals // test seam for the price handler

// getUSDToBRLRate fetches the current USD to BRL exchange rate.
func getUSDToBRLRate(ctx context.Context) (float64, error) {
	return getExchangeRate(ctx, exchangeRateURL, HTTPGet)
}

// getExchangeRate fetches and decodes the USD to BRL rate from the given URL using the provided getter.
func getExchangeRate(ctx context.Context, rawURL string, get httpGetter) (float64, error) {
	resp, err := get(ctx, rawURL)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result struct {
		Rates struct {
			BRL float64 `json:"BRL"`
		} `json:"rates"`
	}

	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return 0, decodeErr
	}

	return result.Rates.BRL, nil
}

// convertToBRL converts a USD price string to BRL using the given exchange rate.
func convertToBRL(priceStr string, rate float64) float64 {
	var price float64
	_, _ = fmt.Sscanf(priceStr, "%f", &price)
	return price * rate
}
