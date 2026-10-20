// Package moxfield is a client for the undocumented Moxfield deck API: deck reads, user deck
// lists, deck search, and commander verification of search results.
package moxfield

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nathanmartins/mtg-mcp/internal/deckfmt"
	"github.com/nathanmartins/mtg-mcp/internal/httpx"
)

// apiSource names Moxfield in upstream status and decode errors.
const apiSource = "moxfield API"

// Client talks to the Moxfield API. Deck reads and deck search live on different hosts, so
// each has its own URL. The zero value has no URLs and makes no live requests; NewClient
// returns a client for the production API.
type Client struct {
	// BaseURL serves deck reads and user deck lists (DefaultBaseURL in production).
	BaseURL string
	// SearchURL is the full deck-search endpoint (DefaultSearchURL in production).
	SearchURL string
	// VerifyDelay spaces commander-verification requests; zero means no pause.
	VerifyDelay time.Duration
}

// NewClient returns a client for the production Moxfield API.
func NewClient() *Client {
	return &Client{BaseURL: DefaultBaseURL, SearchURL: DefaultSearchURL, VerifyDelay: VerifyDelay}
}

// isRateLimited reports whether err is a Moxfield HTTP 429, which callers treat
// differently from hard failures.
func isRateLimited(err error) bool {
	return httpx.HasStatus(err, http.StatusTooManyRequests)
}

// Deck represents a deck from Moxfield.
type Deck struct {
	ID           string               `json:"id"`
	PublicID     string               `json:"publicId"`
	Name         string               `json:"name"`
	Format       string               `json:"format"`
	Description  string               `json:"description"`
	Mainboard    map[string]CardEntry `json:"mainboard"`
	Sideboard    map[string]CardEntry `json:"sideboard"`
	Commanders   map[string]CardEntry `json:"commanders"`
	Maybeboard   map[string]CardEntry `json:"maybeboard"`
	CreatedAt    string               `json:"createdAtUtc"`
	LastUpdated  string               `json:"lastUpdatedAtUtc"`
	ViewCount    int                  `json:"viewCount"`
	LikeCount    int                  `json:"likeCount"`
	CommentCount int                  `json:"commentCount"`
	Authors      any                  `json:"authors,omitempty"` // Usually user objects; see authorNames
}

// CardEntry represents a card in a Moxfield deck.
type CardEntry struct {
	Quantity int      `json:"quantity"`
	Card     CardInfo `json:"card"`
}

// CardInfo represents card information.
type CardInfo struct {
	Name     string `json:"name"`
	Set      string `json:"set"`
	TypeLine string `json:"type_line"`
	ManaCost string `json:"mana_cost"`
	Rarity   string `json:"rarity"`
}

// UserDecksResponse represents paginated user decks.
type UserDecksResponse struct {
	PageNumber   int           `json:"pageNumber"`
	PageSize     int           `json:"pageSize"`
	TotalResults int           `json:"totalResults"`
	TotalPages   int           `json:"totalPages"`
	Data         []DeckSummary `json:"data"`
}

// DeckSummary represents a deck summary in list view.
type DeckSummary struct {
	PublicID  string `json:"publicId"`
	Name      string `json:"name"`
	Format    string `json:"format"`
	PublicURL string `json:"publicUrl"`
	ViewCount int    `json:"viewCount"`
	LikeCount int    `json:"likeCount"`
}

// SearchResponse represents search results from Moxfield.
type SearchResponse struct {
	PageNumber   int           `json:"pageNumber"`
	PageSize     int           `json:"pageSize"`
	TotalResults int           `json:"totalResults"`
	TotalPages   int           `json:"totalPages"`
	Data         []DeckSummary `json:"data"`
}

// SearchParams represents search parameters. Only fields the Moxfield API
// actually honours are present: every commander-oriented parameter (query, q, board,
// commanderName, commanders) is silently ignored upstream and must not be sent.
type SearchParams struct {
	CardName      string // decks containing this card
	Format        string
	SortType      string
	SortDirection string
	PageSize      int
	PageNumber    int
}

const (
	// DefaultBaseURL is the live Moxfield API root for deck reads and user deck lists.
	DefaultBaseURL = "https://api.moxfield.com/v2"
	// DefaultSearchURL is a different host from DefaultBaseURL:
	// deck reads live on api.moxfield.com, deck search only exists on api2.
	DefaultSearchURL = "https://api2.moxfield.com/v2/decks/search"
	// SearchDefaultLimit is the default number of decks returned per page.
	SearchDefaultLimit = 10
	// TotalCap is the saturation value Moxfield reports in totalResults for any
	// broad card-name search. It is not a count, and totalPages is derived from it, so
	// neither number may be presented to the caller as a real total.
	TotalCap = 10000
	// DirectionAscending and DirectionDescending are Moxfield's wire
	// values for sortDirection. They are not the tool's vocabulary: the caller passes
	// asc/desc and the output must be rendered back in those terms.
	DirectionAscending = "Ascending"
	// DirectionDescending is Moxfield's wire value for descending order.
	DirectionDescending = "Descending"
)

// sortKeys lists the sortType values the Moxfield API accepts, in the order the
// tool documents them. Anything else (e.g. "price") is answered with HTTP 400 upstream.
// This is the single source for validation, the error message and the tool description.
func sortKeys() []string {
	return []string{"updated", "created", "views", "likes", "comments", "relevance"}
}

// SortValues renders the accepted sort keys for error messages and tool docs.
func SortValues() string {
	return strings.Join(sortKeys(), ", ")
}

// ParseSortType validates a sort key against the values Moxfield accepts.
func ParseSortType(sort string) (string, error) {
	if slices.Contains(sortKeys(), sort) {
		return sort, nil
	}
	return "", fmt.Errorf("unsupported sort %q (accepted: %s)", sort, SortValues())
}

// SortDirection maps a sort order onto Moxfield's capitalised wire values.
func SortDirection(ascending bool) string {
	if ascending {
		return DirectionAscending
	}
	return DirectionDescending
}

// GetDeck fetches a deck by its public ID.
func (c *Client) GetDeck(ctx context.Context, publicID string) (*Deck, error) {
	reqURL := fmt.Sprintf("%s/decks/all/%s", c.BaseURL, url.PathEscape(publicID))

	var deck Deck
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, apiSource, &deck); err != nil {
		return nil, err
	}
	return &deck, nil
}

// GetUserDecks fetches a user's public decks, up to pageSize of them (capped at 100).
func (c *Client) GetUserDecks(ctx context.Context, username string, pageSize int) (*UserDecksResponse, error) {
	const maxPageSize = 100
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	queryParams := url.Values{}
	queryParams.Set("pageSize", strconv.Itoa(pageSize))
	reqURL := fmt.Sprintf("%s/users/%s/decks?%s", c.BaseURL, url.PathEscape(username), queryParams.Encode())

	var decksResp UserDecksResponse
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, apiSource, &decksResp); err != nil {
		return nil, err
	}
	return &decksResp, nil
}

// SearchDecks searches for decks on Moxfield.
func (c *Client) SearchDecks(ctx context.Context, params SearchParams) (*SearchResponse, error) {
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
	reqURL := fmt.Sprintf("%s?%s", c.SearchURL, queryParams.Encode())

	var searchResp SearchResponse
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, apiSource, &searchResp); err != nil {
		return nil, err
	}
	return &searchResp, nil
}

// ExtractPublicID extracts the public ID from a Moxfield URL.
func ExtractPublicID(url string) string {
	// Expected format: https://www.moxfield.com/decks/{publicId}
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "decks" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return url // Return as-is if no parsing needed
}

// sortedEntries returns a Moxfield board's entries ordered by their map key (the card
// name), so a deck renders the same way every time instead of in map iteration order.
func sortedEntries(board map[string]CardEntry) []CardEntry {
	entries := make([]CardEntry, 0, len(board))
	for _, key := range slices.Sorted(maps.Keys(board)) {
		entries = append(entries, board[key])
	}
	return entries
}

// groupDeckCards categorizes mainboard cards by type.
func groupDeckCards(mainboard map[string]CardEntry) deckfmt.CardGroups {
	var groups deckfmt.CardGroups
	for _, entry := range sortedEntries(mainboard) {
		groups.Add(entry.Card.TypeLine, entry.Card.Name, entry.Quantity)
	}
	return groups
}

// authorNames returns the deck's author usernames. The live API sends a list of user objects
// ({"userName": "...", "displayName": "..."}); plain name strings are accepted too. The username
// is used, not the display name, because it is what get_moxfield_user_decks takes. Anything else
// is ignored.
func authorNames(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, author := range list {
		switch author := author.(type) {
		case string:
			names = append(names, author)
		case map[string]any:
			if name, isString := author["userName"].(string); isString && name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// writeBoard renders a board as "## Title" followed by its card lines, or
// nothing when the board is empty.
func writeBoard(output *strings.Builder, title string, board map[string]CardEntry) {
	if len(board) == 0 {
		return
	}
	_, _ = fmt.Fprintf(output, "\n## %s\n", title)
	for _, entry := range sortedEntries(board) {
		_, _ = fmt.Fprintf(output, "- %dx %s\n", entry.Quantity, entry.Card.Name)
	}
}

// formatDeckHeader formats deck metadata and commander information.
func formatDeckHeader(deck *Deck) string {
	var output strings.Builder

	_, _ = fmt.Fprintf(&output, "# %s\n\n", deck.Name)
	_, _ = fmt.Fprintf(&output, "**Format:** %s\n", deck.Format)

	if authors := authorNames(deck.Authors); len(authors) > 0 {
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

	writeBoard(&output, "Commanders", deck.Commanders)

	return output.String()
}

// FormatDeck formats a Moxfield deck for text display.
func FormatDeck(deck *Deck) string {
	var output strings.Builder

	output.WriteString(formatDeckHeader(deck))

	output.WriteString("\n## Mainboard\n")

	groups := groupDeckCards(deck.Mainboard)

	_, _ = fmt.Fprintf(&output, "\n**Total Cards:** %d\n\n", groups.TotalCards+len(deck.Commanders))

	groups.Render(&output)

	writeBoard(&output, "Sideboard", deck.Sideboard)
	writeBoard(&output, "Maybeboard", deck.Maybeboard)

	return output.String()
}
