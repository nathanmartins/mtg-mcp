package main

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/nathanmartins/mtg-mcp/internal/httpx"
)

// moxfieldSource names Moxfield in upstream status and decode errors.
const moxfieldSource = "moxfield API"

// isMoxfieldRateLimited reports whether err is a Moxfield HTTP 429, which callers treat
// differently from hard failures.
func isMoxfieldRateLimited(err error) bool {
	return httpx.HasStatus(err, http.StatusTooManyRequests)
}

// MoxfieldDeck represents a deck from Moxfield.
type MoxfieldDeck struct {
	ID           string                       `json:"id"`
	PublicID     string                       `json:"publicId"`
	Name         string                       `json:"name"`
	Format       string                       `json:"format"`
	Description  string                       `json:"description"`
	Mainboard    map[string]MoxfieldCardEntry `json:"mainboard"`
	Sideboard    map[string]MoxfieldCardEntry `json:"sideboard"`
	Commanders   map[string]MoxfieldCardEntry `json:"commanders"`
	Maybeboard   map[string]MoxfieldCardEntry `json:"maybeboard"`
	CreatedAt    string                       `json:"createdAtUtc"`
	LastUpdated  string                       `json:"lastUpdatedAtUtc"`
	ViewCount    int                          `json:"viewCount"`
	LikeCount    int                          `json:"likeCount"`
	CommentCount int                          `json:"commentCount"`
	Authors      any                          `json:"authors,omitempty"` // Can be string slice or object
}

// MoxfieldCardEntry represents a card in a Moxfield deck.
type MoxfieldCardEntry struct {
	Quantity int              `json:"quantity"`
	Card     MoxfieldCardInfo `json:"card"`
}

// MoxfieldCardInfo represents card information.
type MoxfieldCardInfo struct {
	Name     string `json:"name"`
	Set      string `json:"set"`
	TypeLine string `json:"type_line"`
	ManaCost string `json:"mana_cost"`
	Rarity   string `json:"rarity"`
}

// MoxfieldUserDecksResponse represents paginated user decks.
type MoxfieldUserDecksResponse struct {
	PageNumber   int                   `json:"pageNumber"`
	PageSize     int                   `json:"pageSize"`
	TotalResults int                   `json:"totalResults"`
	TotalPages   int                   `json:"totalPages"`
	Data         []MoxfieldDeckSummary `json:"data"`
}

// MoxfieldDeckSummary represents a deck summary in list view.
type MoxfieldDeckSummary struct {
	PublicID  string `json:"publicId"`
	Name      string `json:"name"`
	Format    string `json:"format"`
	PublicURL string `json:"publicUrl"`
	ViewCount int    `json:"viewCount"`
	LikeCount int    `json:"likeCount"`
}

// MoxfieldSearchResponse represents search results from Moxfield.
type MoxfieldSearchResponse struct {
	PageNumber   int                   `json:"pageNumber"`
	PageSize     int                   `json:"pageSize"`
	TotalResults int                   `json:"totalResults"`
	TotalPages   int                   `json:"totalPages"`
	Data         []MoxfieldDeckSummary `json:"data"`
}

// MoxfieldSearchParams represents search parameters. Only fields the Moxfield API
// actually honours are present: every commander-oriented parameter (query, q, board,
// commanderName, commanders) is silently ignored upstream and must not be sent.
type MoxfieldSearchParams struct {
	CardName      string // decks containing this card
	Format        string
	SortType      string
	SortDirection string
	PageSize      int
	PageNumber    int
}

const (
	// moxfieldSearchDefaultLimit is the default number of decks returned per page.
	moxfieldSearchDefaultLimit = 10
	// moxfieldTotalCap is the saturation value Moxfield reports in totalResults for any
	// broad card-name search. It is not a count, and totalPages is derived from it, so
	// neither number may be presented to the caller as a real total.
	moxfieldTotalCap = 10000
	// moxfieldDirectionAscending and moxfieldDirectionDescending are Moxfield's wire
	// values for sortDirection. They are not the tool's vocabulary: the caller passes
	// asc/desc and the output must be rendered back in those terms.
	moxfieldDirectionAscending  = "Ascending"
	moxfieldDirectionDescending = "Descending"
)

// moxfieldSortKeys lists the sortType values the Moxfield API accepts, in the order the
// tool documents them. Anything else (e.g. "price") is answered with HTTP 400 upstream.
// This is the single source for validation, the error message and the tool description.
func moxfieldSortKeys() []string {
	return []string{"updated", "created", "views", "likes", "comments", "relevance"}
}

// moxfieldSortValues renders the accepted sort keys for error messages and tool docs.
func moxfieldSortValues() string {
	return strings.Join(moxfieldSortKeys(), ", ")
}

// moxfieldSortType validates a sort key against the values Moxfield accepts.
func moxfieldSortType(sort string) (string, error) {
	if slices.Contains(moxfieldSortKeys(), sort) {
		return sort, nil
	}
	return "", fmt.Errorf("unsupported sort %q (accepted: %s)", sort, moxfieldSortValues())
}

// moxfieldSortDirection maps our asc/desc surface onto Moxfield's capitalised values.
func moxfieldSortDirection(direction string) (string, error) {
	switch direction {
	case sortDirectionAsc:
		return moxfieldDirectionAscending, nil
	case sortDirectionDesc:
		return moxfieldDirectionDescending, nil
	default:
		return "", fmt.Errorf("invalid sort_direction %q (accepted: %s, %s)",
			direction, sortDirectionAsc, sortDirectionDesc)
	}
}

// GetMoxfieldDeck fetches a deck by its public ID.
func GetMoxfieldDeck(ctx context.Context, publicID string) (*MoxfieldDeck, error) {
	return getMoxfieldDeckWithURL(ctx, publicID, defaultMoxfieldBaseURL)
}

// getMoxfieldDeckWithURL fetches a deck with a custom base URL.
func getMoxfieldDeckWithURL(ctx context.Context, publicID, baseURL string) (*MoxfieldDeck, error) {
	reqURL := fmt.Sprintf("%s/decks/all/%s", baseURL, url.PathEscape(publicID))

	var deck MoxfieldDeck
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, moxfieldSource, &deck); err != nil {
		return nil, err
	}
	return &deck, nil
}

// getUserDecksWithURL fetches user decks with a custom base URL.
func getUserDecksWithURL(
	ctx context.Context,
	username string,
	pageSize int,
	baseURL string,
) (*MoxfieldUserDecksResponse, error) {
	const maxPageSize = 100
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	queryParams := url.Values{}
	queryParams.Set("pageSize", strconv.Itoa(pageSize))
	reqURL := fmt.Sprintf("%s/users/%s/decks?%s", baseURL, url.PathEscape(username), queryParams.Encode())

	var decksResp MoxfieldUserDecksResponse
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, moxfieldSource, &decksResp); err != nil {
		return nil, err
	}
	return &decksResp, nil
}

// SearchMoxfieldDecks searches for decks on Moxfield.
func SearchMoxfieldDecks(ctx context.Context, params MoxfieldSearchParams) (*MoxfieldSearchResponse, error) {
	return searchMoxfieldDecksWithURL(ctx, params, defaultMoxfieldSearchURL)
}

// searchMoxfieldDecksWithURL searches decks with a custom search URL.
func searchMoxfieldDecksWithURL(
	ctx context.Context,
	params MoxfieldSearchParams,
	searchURL string,
) (*MoxfieldSearchResponse, error) {
	const maxPageSize = 100
	const defaultPageSize = 20
	if params.PageSize <= 0 || params.PageSize > maxPageSize {
		params.PageSize = defaultPageSize
	}
	if params.PageNumber < 1 {
		params.PageNumber = 1
	}

	queryParams := url.Values{}
	queryParams.Set("pageSize", strconv.Itoa(params.PageSize))
	queryParams.Set("pageNumber", strconv.Itoa(params.PageNumber))
	if params.CardName != "" {
		queryParams.Set("cardName", params.CardName)
	}
	if params.Format != "" {
		queryParams.Set("fmt", params.Format)
	}
	if params.SortType != "" {
		queryParams.Set("sortType", params.SortType)
	}
	if params.SortDirection != "" {
		queryParams.Set("sortDirection", params.SortDirection)
	}
	reqURL := fmt.Sprintf("%s?%s", searchURL, queryParams.Encode())

	var searchResp MoxfieldSearchResponse
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, moxfieldSource, &searchResp); err != nil {
		return nil, err
	}
	return &searchResp, nil
}

// ExtractPublicIDFromURL extracts the public ID from a Moxfield URL.
func ExtractPublicIDFromURL(url string) string {
	// Expected format: https://www.moxfield.com/decks/{publicId}
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "decks" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return url // Return as-is if no parsing needed
}

// deckCardGroups organizes deck cards by type for display formatting. It is shared by the
// Moxfield and Archidekt deck formatters so both render mainboards identically.
type deckCardGroups struct {
	creatures     []string
	instants      []string
	sorceries     []string
	artifacts     []string
	enchantments  []string
	planeswalkers []string
	lands         []string
	others        []string
	totalCards    int
}

// add files one card entry under the first matching type in typeLine. The order matters:
// an artifact creature is a creature, and a land creature is a creature too.
func (g *deckCardGroups) add(typeLine, name string, quantity int) {
	cardLine := fmt.Sprintf("%dx %s", quantity, name)
	g.totalCards += quantity

	typeLine = strings.ToLower(typeLine)
	switch {
	case strings.Contains(typeLine, "creature"):
		g.creatures = append(g.creatures, cardLine)
	case strings.Contains(typeLine, "instant"):
		g.instants = append(g.instants, cardLine)
	case strings.Contains(typeLine, "sorcery"):
		g.sorceries = append(g.sorceries, cardLine)
	case strings.Contains(typeLine, "artifact"):
		g.artifacts = append(g.artifacts, cardLine)
	case strings.Contains(typeLine, "enchantment"):
		g.enchantments = append(g.enchantments, cardLine)
	case strings.Contains(typeLine, "planeswalker"):
		g.planeswalkers = append(g.planeswalkers, cardLine)
	case strings.Contains(typeLine, "land"):
		g.lands = append(g.lands, cardLine)
	default:
		g.others = append(g.others, cardLine)
	}
}

// writeTo renders every non-empty group, in the fixed order the deck output uses.
func (g *deckCardGroups) writeTo(output *strings.Builder) {
	output.WriteString(formatCardGroup("Creatures", g.creatures))
	output.WriteString(formatCardGroup("Instants", g.instants))
	output.WriteString(formatCardGroup("Sorceries", g.sorceries))
	output.WriteString(formatCardGroup("Artifacts", g.artifacts))
	output.WriteString(formatCardGroup("Enchantments", g.enchantments))
	output.WriteString(formatCardGroup("Planeswalkers", g.planeswalkers))
	output.WriteString(formatCardGroup("Lands", g.lands))
	output.WriteString(formatCardGroup("Other", g.others))
}

// sortedEntries returns a Moxfield board's entries ordered by their map key (the card
// name), so a deck renders the same way every time instead of in map iteration order.
func sortedEntries(board map[string]MoxfieldCardEntry) []MoxfieldCardEntry {
	entries := make([]MoxfieldCardEntry, 0, len(board))
	for _, key := range slices.Sorted(maps.Keys(board)) {
		entries = append(entries, board[key])
	}
	return entries
}

// groupDeckCards categorizes mainboard cards by type.
func groupDeckCards(mainboard map[string]MoxfieldCardEntry) deckCardGroups {
	var groups deckCardGroups
	for _, entry := range sortedEntries(mainboard) {
		groups.add(entry.Card.TypeLine, entry.Card.Name, entry.Quantity)
	}
	return groups
}

// moxfieldAuthors returns the deck's author names. The field is a list of names on most
// decks but an object on some, and anything that is not a list of strings is ignored.
func moxfieldAuthors(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, author := range list {
		if name, isString := author.(string); isString {
			names = append(names, name)
		}
	}
	return names
}

// writeMoxfieldBoard renders a board as "## Title" followed by its card lines, or
// nothing when the board is empty.
func writeMoxfieldBoard(output *strings.Builder, title string, board map[string]MoxfieldCardEntry) {
	if len(board) == 0 {
		return
	}
	_, _ = fmt.Fprintf(output, "\n## %s\n", title)
	for _, entry := range sortedEntries(board) {
		_, _ = fmt.Fprintf(output, "- %dx %s\n", entry.Quantity, entry.Card.Name)
	}
}

// formatDeckHeader formats deck metadata and commander information.
func formatDeckHeader(deck *MoxfieldDeck) string {
	var output strings.Builder

	_, _ = fmt.Fprintf(&output, "# %s\n\n", deck.Name)
	_, _ = fmt.Fprintf(&output, "**Format:** %s\n", deck.Format)

	if authors := moxfieldAuthors(deck.Authors); len(authors) > 0 {
		_, _ = fmt.Fprintf(&output, "**Author:** %s\n", strings.Join(authors, ", "))
	}

	_, _ = fmt.Fprintf(&output, "**Views:** %d | **Likes:** %d | **Comments:** %d\n",
		deck.ViewCount, deck.LikeCount, deck.CommentCount)

	if deck.LastUpdated != "" {
		_, _ = fmt.Fprintf(&output, "**Last Updated:** %s\n", deck.LastUpdated)
	}

	if deck.Description != "" {
		_, _ = fmt.Fprintf(&output, "\n**Description:**\n%s\n", deck.Description)
	}

	writeMoxfieldBoard(&output, "Commanders", deck.Commanders)

	return output.String()
}

// formatCardGroup formats a category of cards with a title.
func formatCardGroup(title string, cards []string) string {
	if len(cards) == 0 {
		return ""
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "**%s (%d):**\n", title, len(cards))
	for _, c := range cards {
		_, _ = fmt.Fprintf(&output, "- %s\n", c)
	}
	output.WriteString("\n")

	return output.String()
}

// FormatDeckForDisplay formats a Moxfield deck for text display.
func FormatDeckForDisplay(deck *MoxfieldDeck) string {
	var output strings.Builder

	output.WriteString(formatDeckHeader(deck))

	output.WriteString("\n## Mainboard\n")

	groups := groupDeckCards(deck.Mainboard)

	_, _ = fmt.Fprintf(&output, "\n**Total Cards:** %d\n\n", groups.totalCards+len(deck.Commanders))

	groups.writeTo(&output)

	writeMoxfieldBoard(&output, "Sideboard", deck.Sideboard)
	writeMoxfieldBoard(&output, "Maybeboard", deck.Maybeboard)

	return output.String()
}
