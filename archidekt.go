package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	archidektCommanderCategory = "Commander"
	archidektSearchMaxLimit    = 20
)

// ArchidektDeck represents a deck from Archidekt.
type ArchidektDeck struct {
	ID          int                  `json:"id"`
	Name        string               `json:"name"`
	CreatedAt   string               `json:"createdAt"`
	UpdatedAt   string               `json:"updatedAt"`
	DeckFormat  int                  `json:"deckFormat"`
	EdhBracket  *int                 `json:"edhBracket"`
	Description string               `json:"description"`
	ViewCount   int                  `json:"viewCount"`
	Private     bool                 `json:"private"`
	Unlisted    bool                 `json:"unlisted"`
	Owner       ArchidektOwner       `json:"owner"`
	Categories  []ArchidektCategory  `json:"categories"`
	Cards       []ArchidektCardEntry `json:"cards"`
}

// ArchidektOwner represents the owner of an Archidekt deck.
type ArchidektOwner struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// ArchidektCategory represents a card category within an Archidekt deck.
type ArchidektCategory struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	IsPremier      bool   `json:"isPremier"`
	IncludedInDeck bool   `json:"includedInDeck"`
}

// ArchidektCardEntry represents a card entry within an Archidekt deck.
type ArchidektCardEntry struct {
	ID         int           `json:"id"`
	Quantity   int           `json:"quantity"`
	Modifier   string        `json:"modifier"`
	Categories []string      `json:"categories"`
	Card       ArchidektCard `json:"card"`
}

// ArchidektCard represents the physical card data from Archidekt.
type ArchidektCard struct {
	UID             string              `json:"uid"`
	CollectorNumber string              `json:"collectorNumber"`
	Rarity          string              `json:"rarity"`
	Edition         ArchidektEdition    `json:"edition"`
	OracleCard      ArchidektOracleCard `json:"oracleCard"`
	Prices          ArchidektPrices     `json:"prices"`
}

// ArchidektEdition represents the set/edition of an Archidekt card.
type ArchidektEdition struct {
	Code string `json:"editioncode"`
	Name string `json:"editionname"`
}

// ArchidektOracleCard represents the oracle (canonical) data for an Archidekt card.
type ArchidektOracleCard struct {
	Name          string   `json:"name"`
	ManaCost      string   `json:"manaCost"`
	CMC           int      `json:"cmc"`
	Types         []string `json:"types"`
	SuperTypes    []string `json:"superTypes"`
	SubTypes      []string `json:"subTypes"`
	ColorIdentity []string `json:"colorIdentity"`
	Text          string   `json:"text"`
	Power         string   `json:"power"`
	Toughness     string   `json:"toughness"`
}

// ArchidektPrices holds TCG and CardKingdom pricing for an Archidekt card.
type ArchidektPrices struct {
	TCG     float64 `json:"tcg"`
	TCGFoil float64 `json:"tcgfoil"`
	CK      float64 `json:"ck"`
	CKFoil  float64 `json:"ckfoil"`
}

// ArchidektDeckSummary is a lightweight deck entry from paginated list responses.
type ArchidektDeckSummary struct {
	ID         int            `json:"id"`
	Name       string         `json:"name"`
	DeckFormat int            `json:"deckFormat"`
	EdhBracket *int           `json:"edhBracket"`
	ViewCount  int            `json:"viewCount"`
	Private    bool           `json:"private"`
	UpdatedAt  string         `json:"updatedAt"`
	Owner      ArchidektOwner `json:"owner"`
}

// ArchidektUserDecksResponse represents a paginated list of Archidekt decks.
type ArchidektUserDecksResponse struct {
	Count   int                    `json:"count"`
	Next    string                 `json:"next"`
	Results []ArchidektDeckSummary `json:"results"`
}

// GetArchidektDeck fetches a deck by its numeric ID.
func GetArchidektDeck(ctx context.Context, deckID int) (*ArchidektDeck, error) {
	return getArchidektDeckWithURL(ctx, deckID, "https://archidekt.com/api")
}

// getArchidektDeckWithURL fetches a deck with a custom base URL (used for testing).
func getArchidektDeckWithURL(ctx context.Context, deckID int, baseURL string) (*ArchidektDeck, error) {
	reqURL := fmt.Sprintf("%s/decks/%d/", baseURL, deckID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("deck %d not found (it may be private or deleted)", deckID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("archidekt API returned status %d", resp.StatusCode)
	}

	var deck ArchidektDeck
	if decodeErr := json.NewDecoder(resp.Body).Decode(&deck); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &deck, nil
}

// GetArchidektUserDecks fetches a page of public decks for the given username.
func GetArchidektUserDecks(ctx context.Context, username string, page int) (*ArchidektUserDecksResponse, error) {
	return getArchidektUserDecksWithURL(ctx, username, page, "https://archidekt.com/api")
}

// getArchidektUserDecksWithURL fetches user decks with a custom base URL (used for testing).
func getArchidektUserDecksWithURL(
	ctx context.Context,
	username string,
	page int,
	baseURL string,
) (*ArchidektUserDecksResponse, error) {
	if page < 1 {
		page = 1
	}

	queryParams := url.Values{}
	queryParams.Set("owner", username)
	queryParams.Set("exactowner", "true")
	queryParams.Set("page", strconv.Itoa(page))
	reqURL := fmt.Sprintf("%s/decks/?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("archidekt API returned status %d", resp.StatusCode)
	}

	var result ArchidektUserDecksResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &result, nil
}

// ExtractArchidektDeckID parses a numeric deck ID from an Archidekt URL or raw ID string.
// Supports formats like "https://archidekt.com/decks/12345" or just "12345".
func ExtractArchidektDeckID(input string) (int, error) {
	// Strip query string and trailing slash
	input = strings.TrimRight(input, "/")
	if idx := strings.Index(input, "?"); idx != -1 {
		input = input[:idx]
	}

	// Try direct numeric parse first
	if id, err := strconv.Atoi(input); err == nil {
		return id, nil
	}

	// Try extracting from URL: .../decks/{id}
	parts := strings.Split(input, "/")
	for i, part := range parts {
		if part == "decks" && i+1 < len(parts) {
			if id, err := strconv.Atoi(parts[i+1]); err == nil {
				return id, nil
			}
		}
	}

	return 0, fmt.Errorf("could not extract a numeric deck ID from %q", input)
}

// archidektFormatName returns a human-readable format name for a numeric format code.
func archidektFormatName(code int) string {
	formatNames := map[int]string{
		1:  "Standard",
		2:  "Modern",
		3:  archidektCommanderCategory,
		4:  "Legacy",
		5:  "Vintage",
		6:  "Pauper",
		7:  "Custom",
		8:  "Frontier",
		9:  "Future Standard",
		10: "Penny Dreadful",
		11: "1v1 Commander",
		12: "Duel Commander",
		13: "Brawl",
		14: "Oathbreaker",
		15: "Pioneer",
		16: "Historic",
		17: "Pauper Commander",
		18: "Alchemy",
		19: "Explorer",
		20: "Historic Brawl",
		21: "Gladiator",
		22: "Premodern",
		23: "Pre-EDH",
		24: "Timeless",
		25: "Canadian Highlander",
	}
	if name, ok := formatNames[code]; ok {
		return name
	}
	return fmt.Sprintf("Unknown (%d)", code)
}

// archidektPremierCategories returns the set of category names marked as premier (e.g. "Commander").
func archidektPremierCategories(categories []ArchidektCategory) map[string]bool {
	premier := make(map[string]bool)
	for _, c := range categories {
		if c.IsPremier {
			premier[c.Name] = true
		}
	}
	return premier
}

// FormatArchidektDeckForDisplay formats an Archidekt deck for text display.
func FormatArchidektDeckForDisplay(deck *ArchidektDeck) string {
	var output strings.Builder

	premier := archidektPremierCategories(deck.Categories)

	// Header
	fmt.Fprintf(&output, "# %s\n\n", deck.Name)
	fmt.Fprintf(&output, "**Format:** %s\n", archidektFormatName(deck.DeckFormat))
	fmt.Fprintf(&output, "**Author:** %s\n", deck.Owner.Username)
	fmt.Fprintf(&output, "**Views:** %d\n", deck.ViewCount)
	if deck.EdhBracket != nil {
		fmt.Fprintf(&output, "**EDH Bracket:** %d\n", *deck.EdhBracket)
	}
	if deck.UpdatedAt != "" {
		fmt.Fprintf(&output, "**Last Updated:** %s\n", deck.UpdatedAt)
	}
	fmt.Fprintf(&output, "**Archidekt URL:** https://archidekt.com/decks/%d\n", deck.ID)

	// Commanders (cards in premier categories)
	var commanders []string
	for _, entry := range deck.Cards {
		for _, cat := range entry.Categories {
			if premier[cat] {
				commanders = append(commanders, fmt.Sprintf("%dx %s", entry.Quantity, entry.Card.OracleCard.Name))
				break
			}
		}
	}
	if len(commanders) > 0 {
		output.WriteString("\n## Commanders\n")
		for _, c := range commanders {
			fmt.Fprintf(&output, "- %s\n", c)
		}
	}

	// Group mainboard cards by type (exclude commander-category cards)
	groups := groupArchidektDeckCards(deck.Cards, premier)

	output.WriteString("\n## Mainboard\n")
	fmt.Fprintf(&output, "\n**Total Cards:** %d\n\n", groups.totalCards+len(commanders))

	output.WriteString(formatCardGroup("Creatures", groups.creatures))
	output.WriteString(formatCardGroup("Instants", groups.instants))
	output.WriteString(formatCardGroup("Sorceries", groups.sorceries))
	output.WriteString(formatCardGroup("Artifacts", groups.artifacts))
	output.WriteString(formatCardGroup("Enchantments", groups.enchantments))
	output.WriteString(formatCardGroup("Planeswalkers", groups.planeswalkers))
	output.WriteString(formatCardGroup("Lands", groups.lands))
	if len(groups.others) > 0 {
		output.WriteString(formatCardGroup("Other", groups.others))
	}

	return output.String()
}

// SearchArchidektDecks searches public Commander decks by commander name, optionally filtered by
// EDH bracket, sorted by view count descending. bracket=0 means no filter; limit is clamped to 1–20.
func SearchArchidektDecks(
	ctx context.Context,
	commander string,
	bracket int,
	limit int,
) (*ArchidektUserDecksResponse, error) {
	return searchArchidektDecksWithURL(ctx, commander, bracket, limit, "https://archidekt.com/api")
}

// searchArchidektDecksWithURL is the testable variant that accepts a custom base URL.
func searchArchidektDecksWithURL(
	ctx context.Context,
	commander string,
	bracket int,
	limit int,
	baseURL string,
) (*ArchidektUserDecksResponse, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > archidektSearchMaxLimit {
		limit = archidektSearchMaxLimit
	}

	queryParams := url.Values{}
	queryParams.Set("cards__oracle_card__name__icontains", commander)
	queryParams.Set("cards__categories__icontains", archidektCommanderCategory)
	queryParams.Set("deckFormat", "3")
	queryParams.Set("orderBy", "-viewCount")
	queryParams.Set("page_size", strconv.Itoa(limit))
	if bracket >= 1 && bracket <= 4 {
		queryParams.Set("edhBracket", strconv.Itoa(bracket))
	}

	reqURL := fmt.Sprintf("%s/decks/?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("archidekt API returned status %d", resp.StatusCode)
	}

	var result ArchidektUserDecksResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &result, nil
}

// FormatArchidektSearchResultsForDisplay formats a deck search result list for text display.
func FormatArchidektSearchResultsForDisplay(commander string, bracket int, result *ArchidektUserDecksResponse) string {
	var output strings.Builder

	if bracket >= 1 && bracket <= 4 {
		fmt.Fprintf(&output, "# Archidekt Decks: %s (Bracket %d)\n\n", commander, bracket)
	} else {
		fmt.Fprintf(&output, "# Archidekt Decks: %s\n\n", commander)
	}
	fmt.Fprintf(&output, "**Total Results:** %d\n\n", result.Count)

	if len(result.Results) == 0 {
		output.WriteString("No public decks found for this commander.\n")
		return output.String()
	}

	for i, deck := range result.Results {
		fmt.Fprintf(&output, "## %d. %s\n", i+1, deck.Name)
		fmt.Fprintf(&output, "- **Author:** %s\n", deck.Owner.Username)
		fmt.Fprintf(&output, "- **Views:** %d\n", deck.ViewCount)
		if deck.EdhBracket != nil {
			fmt.Fprintf(&output, "- **EDH Bracket:** %d\n", *deck.EdhBracket)
		}
		fmt.Fprintf(&output, "- **Last Updated:** %s\n", deck.UpdatedAt)
		fmt.Fprintf(&output, "- **URL:** https://archidekt.com/decks/%d\n\n", deck.ID)
	}

	return output.String()
}

// groupArchidektDeckCards categorizes non-commander cards by oracle type.
func groupArchidektDeckCards(cards []ArchidektCardEntry, premier map[string]bool) deckCardGroups {
	groups := deckCardGroups{
		creatures:     []string{},
		instants:      []string{},
		sorceries:     []string{},
		artifacts:     []string{},
		enchantments:  []string{},
		planeswalkers: []string{},
		lands:         []string{},
		others:        []string{},
	}

	for _, entry := range cards {
		// Skip commander-category cards
		isCommander := false
		for _, cat := range entry.Categories {
			if premier[cat] {
				isCommander = true
				break
			}
		}
		if isCommander {
			continue
		}

		cardLine := fmt.Sprintf("%dx %s", entry.Quantity, entry.Card.OracleCard.Name)
		groups.totalCards += entry.Quantity

		typeLine := strings.ToLower(strings.Join(entry.Card.OracleCard.Types, " "))
		switch {
		case strings.Contains(typeLine, "creature"):
			groups.creatures = append(groups.creatures, cardLine)
		case strings.Contains(typeLine, "instant"):
			groups.instants = append(groups.instants, cardLine)
		case strings.Contains(typeLine, "sorcery"):
			groups.sorceries = append(groups.sorceries, cardLine)
		case strings.Contains(typeLine, "artifact"):
			groups.artifacts = append(groups.artifacts, cardLine)
		case strings.Contains(typeLine, "enchantment"):
			groups.enchantments = append(groups.enchantments, cardLine)
		case strings.Contains(typeLine, "planeswalker"):
			groups.planeswalkers = append(groups.planeswalkers, cardLine)
		case strings.Contains(typeLine, "land"):
			groups.lands = append(groups.lands, cardLine)
		default:
			groups.others = append(groups.others, cardLine)
		}
	}

	return groups
}
