// Package archidekt is a client for the Archidekt deck API: deck reads, user deck lists and
// Commander deck search, plus the Markdown rendering of each.
package archidekt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/nathanmartins/mtg-mcp/internal/deckfmt"
	"github.com/nathanmartins/mtg-mcp/internal/httpx"
)

const (
	DefaultBaseURL             = "https://archidekt.com/api"
	archidektCommanderCategory = "Commander"
	// apiSource names Archidekt in upstream status and decode errors.
	apiSource = "archidekt API"
)

// Deck represents a deck from Archidekt.
type Deck struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
	DeckFormat  int         `json:"deckFormat"`
	EdhBracket  *int        `json:"edhBracket"`
	Description string      `json:"description"`
	ViewCount   int         `json:"viewCount"`
	Private     bool        `json:"private"`
	Unlisted    bool        `json:"unlisted"`
	Owner       Owner       `json:"owner"`
	Categories  []Category  `json:"categories"`
	Cards       []CardEntry `json:"cards"`
}

// Owner represents the owner of an Archidekt deck.
type Owner struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// Category represents a card category within an Archidekt deck.
type Category struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	IsPremier      bool   `json:"isPremier"`
	IncludedInDeck bool   `json:"includedInDeck"`
}

// CardEntry represents a card entry within an Archidekt deck.
type CardEntry struct {
	ID         int      `json:"id"`
	Quantity   int      `json:"quantity"`
	Modifier   string   `json:"modifier"`
	Categories []string `json:"categories"`
	Card       Card     `json:"card"`
}

// Card represents the physical card data from Archidekt.
type Card struct {
	UID             string     `json:"uid"`
	CollectorNumber string     `json:"collectorNumber"`
	Rarity          string     `json:"rarity"`
	Edition         Edition    `json:"edition"`
	OracleCard      OracleCard `json:"oracleCard"`
	Prices          Prices     `json:"prices"`
}

// Edition represents the set/edition of an Archidekt card.
type Edition struct {
	Code string `json:"editioncode"`
	Name string `json:"editionname"`
}

// OracleCard represents the oracle (canonical) data for an Archidekt card.
type OracleCard struct {
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

// Prices holds TCG and CardKingdom pricing for an Archidekt card.
type Prices struct {
	TCG     float64 `json:"tcg"`
	TCGFoil float64 `json:"tcgfoil"`
	CK      float64 `json:"ck"`
	CKFoil  float64 `json:"ckfoil"`
}

// DeckSummary is a lightweight deck entry from paginated list responses.
type DeckSummary struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	DeckFormat int    `json:"deckFormat"`
	EdhBracket *int   `json:"edhBracket"`
	ViewCount  int    `json:"viewCount"`
	Private    bool   `json:"private"`
	UpdatedAt  string `json:"updatedAt"`
	Owner      Owner  `json:"owner"`
}

// UserDecksResponse represents a paginated list of Archidekt decks.
type UserDecksResponse struct {
	Count   int           `json:"count"`
	Next    string        `json:"next"`
	Results []DeckSummary `json:"results"`
}

// GetDeck fetches a deck by its numeric ID.
func GetDeck(ctx context.Context, deckID int) (*Deck, error) {
	return GetDeckWithURL(ctx, deckID, DefaultBaseURL)
}

// GetDeckWithURL fetches a deck with a custom base URL (used for testing).
func GetDeckWithURL(ctx context.Context, deckID int, baseURL string) (*Deck, error) {
	reqURL := fmt.Sprintf("%s/decks/%d/", baseURL, deckID)

	var deck Deck
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, apiSource, &deck); err != nil {
		if httpx.HasStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("deck %d not found (it may be private or deleted)", deckID)
		}
		return nil, err
	}
	return &deck, nil
}

// GetUserDecks fetches a page of public decks for the given username.
func GetUserDecks(ctx context.Context, username string, page int) (*UserDecksResponse, error) {
	return GetUserDecksWithURL(ctx, username, page, DefaultBaseURL)
}

// GetUserDecksWithURL fetches user decks with a custom base URL (used for testing).
func GetUserDecksWithURL(
	ctx context.Context,
	username string,
	page int,
	baseURL string,
) (*UserDecksResponse, error) {
	if page < 1 {
		page = 1
	}

	// ownerUsername is the filter the v3 endpoint honors (search_archidekt_decks uses it
	// too); "owner"/"exactowner" are silently ignored and return everyone's recent decks.
	queryParams := url.Values{}
	queryParams.Set("ownerUsername", username)
	queryParams.Set("page", strconv.Itoa(page))
	reqURL := fmt.Sprintf("%s/decks/v3/?%s", baseURL, queryParams.Encode())

	var result UserDecksResponse
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, apiSource, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractDeckID parses a numeric deck ID from an Archidekt URL or raw ID string.
// Supports formats like "https://archidekt.com/decks/12345" or just "12345".
func ExtractDeckID(input string) (int, error) {
	// Strip query string and trailing slash
	input = strings.TrimRight(input, "/")
	input, _, _ = strings.Cut(input, "?")

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

// FormatName returns a human-readable format name for a numeric format code.
func FormatName(code int) string {
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
func archidektPremierCategories(categories []Category) map[string]bool {
	premier := make(map[string]bool)
	for _, c := range categories {
		if c.IsPremier {
			premier[c.Name] = true
		}
	}
	return premier
}

// archidektExcludedCategories returns the set of category names explicitly NOT included in the deck
// (includedInDeck == false), e.g. "Maybeboard".
func archidektExcludedCategories(categories []Category) map[string]bool {
	excluded := make(map[string]bool)
	for _, c := range categories {
		if !c.IncludedInDeck {
			excluded[c.Name] = true
		}
	}
	return excluded
}

// Out-of-deck zone names. Commander has no real sideboard, so Archidekt's Sideboard category is
// always treated as out of the deck (regardless of its includedInDeck flag).
const (
	archidektSideboardZone  = "Sideboard"
	archidektMaybeboardZone = "Maybeboard"
)

// archidektIsCommander reports whether a card entry sits in a premier category (e.g. "Commander").
func archidektIsCommander(entry CardEntry, premier map[string]bool) bool {
	return slices.ContainsFunc(entry.Categories, func(cat string) bool { return premier[cat] })
}

// archidektEntryZone classifies a card entry's out-of-deck zone, returning archidektSideboardZone or
// archidektMaybeboardZone for cards outside the playable deck, or "" for cards that count toward it.
// A card belongs to a zone if ANY of its categories matches it, so cards tagged Sideboard alongside a
// type category (e.g. ["Sideboard", "Creature"]) are still treated as sideboard. excluded holds the
// category names flagged includedInDeck==false (e.g. Maybeboard).
func archidektEntryZone(entry CardEntry, excluded map[string]bool) string {
	if slices.Contains(entry.Categories, archidektSideboardZone) {
		return archidektSideboardZone
	}
	if slices.ContainsFunc(entry.Categories, func(cat string) bool { return excluded[cat] }) {
		return archidektMaybeboardZone
	}
	return ""
}

// archidektEntryInDeck reports whether a card entry counts toward the playable deck (mainboard),
// i.e. it is not in an out-of-deck zone (Sideboard / Maybeboard).
func archidektEntryInDeck(entry CardEntry, excluded map[string]bool) bool {
	return archidektEntryZone(entry, excluded) == ""
}

// formatArchidektZone renders an out-of-deck zone as "## Title" followed by its card lines,
// or "" when the zone is empty. Mirrors the Moxfield deck output style (plain heading, no count).
func formatArchidektZone(title string, cards []string) string {
	if len(cards) == 0 {
		return ""
	}
	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "\n## %s\n", title)
	for _, c := range cards {
		_, _ = fmt.Fprintf(&output, "- %s\n", c)
	}
	return output.String()
}

// FormatDeck formats an Archidekt deck for text display.
func FormatDeck(deck *Deck) string {
	var output strings.Builder

	premier := archidektPremierCategories(deck.Categories)
	excluded := archidektExcludedCategories(deck.Categories)

	// Header
	_, _ = fmt.Fprintf(&output, "# %s\n\n", deck.Name)
	_, _ = fmt.Fprintf(&output, "**Format:** %s\n", FormatName(deck.DeckFormat))
	_, _ = fmt.Fprintf(&output, "**Author:** %s\n", deck.Owner.Username)
	_, _ = fmt.Fprintf(&output, "**Views:** %d\n", deck.ViewCount)
	if deck.EdhBracket != nil {
		_, _ = fmt.Fprintf(&output, "**EDH Bracket:** %d\n", *deck.EdhBracket)
	}
	if deck.UpdatedAt != "" {
		_, _ = fmt.Fprintf(&output, "**Last Updated:** %s\n", deck.UpdatedAt)
	}
	_, _ = fmt.Fprintf(&output, "**Archidekt URL:** https://archidekt.com/decks/%d\n", deck.ID)

	// Commanders (cards in premier categories)
	var commanders []string
	for _, entry := range deck.Cards {
		if archidektIsCommander(entry, premier) {
			commanders = append(commanders, fmt.Sprintf("%dx %s", entry.Quantity, entry.Card.OracleCard.Name))
		}
	}
	if len(commanders) > 0 {
		output.WriteString("\n## Commanders\n")
		for _, c := range commanders {
			_, _ = fmt.Fprintf(&output, "- %s\n", c)
		}
	}

	// Group mainboard cards by type (exclude commander-category and out-of-deck cards)
	groups := groupArchidektDeckCards(deck.Cards, premier, excluded)

	output.WriteString("\n## Mainboard\n")
	_, _ = fmt.Fprintf(&output, "\n**Total Cards:** %d\n\n", groups.TotalCards+len(commanders))

	groups.Render(&output)

	// Out-of-deck zones (Sideboard / Maybeboard), listed separately
	var maybeboard, sideboard []string
	for _, entry := range deck.Cards {
		line := fmt.Sprintf("%dx %s", entry.Quantity, entry.Card.OracleCard.Name)
		switch archidektEntryZone(entry, excluded) {
		case archidektMaybeboardZone:
			maybeboard = append(maybeboard, line)
		case archidektSideboardZone:
			sideboard = append(sideboard, line)
		}
	}
	output.WriteString(formatArchidektZone(archidektSideboardZone, sideboard))
	output.WriteString(formatArchidektZone(archidektMaybeboardZone, maybeboard))

	return output.String()
}

// FormatLands returns only the land cards from a deck — useful for
// landbase comparisons without the token overhead of a full decklist.
func FormatLands(deck *Deck) string {
	var output strings.Builder

	premier := archidektPremierCategories(deck.Categories)
	excluded := archidektExcludedCategories(deck.Categories)

	_, _ = fmt.Fprintf(&output, "# %s — Landbase\n\n", deck.Name)
	_, _ = fmt.Fprintf(&output, "**Author:** %s\n", deck.Owner.Username)
	if deck.EdhBracket != nil {
		_, _ = fmt.Fprintf(&output, "**EDH Bracket:** %d\n", *deck.EdhBracket)
	}
	_, _ = fmt.Fprintf(&output, "**Archidekt URL:** https://archidekt.com/decks/%d\n\n", deck.ID)

	var lands []string
	for _, entry := range deck.Cards {
		// Skip commanders and cards outside the deck (Sideboard / Maybeboard)
		if archidektIsCommander(entry, premier) || !archidektEntryInDeck(entry, excluded) {
			continue
		}
		typeLine := strings.ToLower(strings.Join(entry.Card.OracleCard.Types, " "))
		if strings.Contains(typeLine, "land") {
			lands = append(lands, fmt.Sprintf("%dx %s", entry.Quantity, entry.Card.OracleCard.Name))
		}
	}

	_, _ = fmt.Fprintf(&output, "## Lands (%d)\n", len(lands))
	for _, l := range lands {
		_, _ = fmt.Fprintf(&output, "- %s\n", l)
	}

	return output.String()
}

// groupArchidektDeckCards categorizes non-commander, in-deck cards by oracle type,
// excluding cards that sit outside the deck (Sideboard / Maybeboard).
func groupArchidektDeckCards(cards []CardEntry, premier, excluded map[string]bool) deckfmt.CardGroups {
	var groups deckfmt.CardGroups
	for _, entry := range cards {
		// Skip commanders and cards outside the deck (Sideboard / Maybeboard)
		if archidektIsCommander(entry, premier) || !archidektEntryInDeck(entry, excluded) {
			continue
		}
		oracle := entry.Card.OracleCard
		groups.Add(strings.Join(oracle.Types, " "), oracle.Name, entry.Quantity)
	}
	return groups
}
