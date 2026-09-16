package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// moxfieldHTTPTimeout bounds a single Moxfield request. Commander verification issues
// one request per candidate deck, so an unbounded client would let a single hung
// upstream request stall the whole tool call.
const moxfieldHTTPTimeout = 15 * time.Second

// moxfieldStatusError reports a non-200 response from the Moxfield API. The status is
// preserved so callers can treat rate limiting (429) differently from hard failures.
type moxfieldStatusError struct {
	StatusCode int
}

func (e *moxfieldStatusError) Error() string {
	return fmt.Sprintf("moxfield API returned status %d", e.StatusCode)
}

// isMoxfieldRateLimited reports whether err is a Moxfield HTTP 429.
func isMoxfieldRateLimited(err error) bool {
	var statusErr *moxfieldStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusTooManyRequests
}

// moxfieldHTTPClient returns a client with an explicit timeout.
func moxfieldHTTPClient() *http.Client {
	return &http.Client{Timeout: moxfieldHTTPTimeout}
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
	Authors      interface{}                  `json:"authors,omitempty"` // Can be string slice or object
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
	// moxfieldSortValues lists the sortType values the Moxfield API accepts;
	// anything else (e.g. "price") is answered with HTTP 400.
	moxfieldSortValues = "comments, created, likes, relevance, updated, views"
	// moxfieldSearchDefaultLimit is the default number of decks returned per page.
	moxfieldSearchDefaultLimit = 10
)

// moxfieldSortType validates a sort key against the values Moxfield accepts.
func moxfieldSortType(sort string) (string, error) {
	switch sort {
	case "updated", "created", "views", "likes", "comments", "relevance":
		return sort, nil
	default:
		return "", fmt.Errorf("unsupported sort %q (accepted: %s)", sort, moxfieldSortValues)
	}
}

// moxfieldSortDirection maps our asc/desc surface onto Moxfield's capitalised values.
func moxfieldSortDirection(direction string) (string, error) {
	switch direction {
	case sortDirectionAsc:
		return "Ascending", nil
	case sortDirectionDesc:
		return "Descending", nil
	default:
		return "", fmt.Errorf("invalid sort_direction %q (accepted: %s, %s)",
			direction, sortDirectionAsc, sortDirectionDesc)
	}
}

// GetMoxfieldDeck fetches a deck by its public ID.
func GetMoxfieldDeck(ctx context.Context, publicID string) (*MoxfieldDeck, error) {
	return getMoxfieldDeckWithURL(ctx, publicID, "https://api.moxfield.com/v2")
}

// getMoxfieldDeckWithURL fetches a deck with a custom base URL.
func getMoxfieldDeckWithURL(ctx context.Context, publicID, baseURL string) (*MoxfieldDeck, error) {
	reqURL := fmt.Sprintf("%s/decks/all/%s", baseURL, url.PathEscape(publicID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := moxfieldHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &moxfieldStatusError{StatusCode: resp.StatusCode}
	}

	var deck MoxfieldDeck
	if decodeErr := json.NewDecoder(resp.Body).Decode(&deck); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := moxfieldHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &moxfieldStatusError{StatusCode: resp.StatusCode}
	}

	var decksResp MoxfieldUserDecksResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&decksResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := moxfieldHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &moxfieldStatusError{StatusCode: resp.StatusCode}
	}

	var searchResp MoxfieldSearchResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&searchResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", decodeErr)
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

// deckCardGroups organizes deck cards by type for display formatting.
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

// groupDeckCards categorizes mainboard cards by type.
func groupDeckCards(mainboard map[string]MoxfieldCardEntry) deckCardGroups {
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

	for _, entry := range mainboard {
		cardLine := fmt.Sprintf("%dx %s", entry.Quantity, entry.Card.Name)
		groups.totalCards += entry.Quantity

		typeLine := strings.ToLower(entry.Card.TypeLine)
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

// formatDeckHeader formats deck metadata and commander information.
func formatDeckHeader(deck *MoxfieldDeck) string {
	var output strings.Builder

	_, _ = fmt.Fprintf(&output, "# %s\n\n", deck.Name)
	_, _ = fmt.Fprintf(&output, "**Format:** %s\n", deck.Format)

	// Authors field can be either []string or an object, handle gracefully
	if deck.Authors != nil {
		authors, ok := deck.Authors.([]interface{})
		if !ok || len(authors) == 0 {
			goto skipAuthors
		}

		authorStrs := make([]string, 0, len(authors))
		for _, author := range authors {
			if authorStr, isString := author.(string); isString {
				authorStrs = append(authorStrs, authorStr)
			}
		}

		if len(authorStrs) > 0 {
			_, _ = fmt.Fprintf(&output, "**Author:** %s\n", strings.Join(authorStrs, ", "))
		}
	}
skipAuthors:

	_, _ = fmt.Fprintf(&output, "**Views:** %d | **Likes:** %d | **Comments:** %d\n",
		deck.ViewCount, deck.LikeCount, deck.CommentCount)

	if deck.LastUpdated != "" {
		_, _ = fmt.Fprintf(&output, "**Last Updated:** %s\n", deck.LastUpdated)
	}

	if deck.Description != "" {
		_, _ = fmt.Fprintf(&output, "\n**Description:**\n%s\n", deck.Description)
	}

	if len(deck.Commanders) > 0 {
		output.WriteString("\n## Commanders\n")
		for _, entry := range deck.Commanders {
			_, _ = fmt.Fprintf(&output, "- %dx %s\n", entry.Quantity, entry.Card.Name)
		}
	}

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

	// Sideboard
	if len(deck.Sideboard) > 0 {
		output.WriteString("\n## Sideboard\n")
		for _, entry := range deck.Sideboard {
			_, _ = fmt.Fprintf(&output, "- %dx %s\n", entry.Quantity, entry.Card.Name)
		}
	}

	// Maybeboard
	if len(deck.Maybeboard) > 0 {
		output.WriteString("\n## Maybeboard\n")
		for _, entry := range deck.Maybeboard {
			_, _ = fmt.Fprintf(&output, "- %dx %s\n", entry.Quantity, entry.Card.Name)
		}
	}

	return output.String()
}
