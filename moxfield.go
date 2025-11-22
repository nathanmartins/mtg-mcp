package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

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

// MoxfieldSearchParams represents search parameters.
type MoxfieldSearchParams struct {
	Query         string
	Format        string
	SortType      string
	SortDirection string
	PageSize      int
	PageNumber    int
}

// GetMoxfieldDeck fetches a deck by its public ID.
func GetMoxfieldDeck(ctx context.Context, publicID string) (*MoxfieldDeck, error) {
	return getMoxfieldDeckWithURL(ctx, publicID, "https://api.moxfield.com/v2")
}

// getMoxfieldDeckWithURL fetches a deck with a custom base URL.
func getMoxfieldDeckWithURL(ctx context.Context, publicID, baseURL string) (*MoxfieldDeck, error) {
	url := fmt.Sprintf("%s/decks/all/%s", baseURL, publicID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("moxfield API returned status %d", resp.StatusCode)
	}

	var deck MoxfieldDeck
	if decodeErr := json.NewDecoder(resp.Body).Decode(&deck); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &deck, nil
}

// GetUserDecks fetches a user's deck list.
func GetUserDecks(ctx context.Context, username string, pageSize int) (*MoxfieldUserDecksResponse, error) {
	return getUserDecksWithURL(ctx, username, pageSize, "https://api.moxfield.com/v2")
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

	url := fmt.Sprintf("%s/users/%s/decks?pageSize=%d", baseURL, username, pageSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("moxfield API returned status %d", resp.StatusCode)
	}

	var decksResp MoxfieldUserDecksResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&decksResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &decksResp, nil
}

// SearchMoxfieldDecks searches for decks on Moxfield.
func SearchMoxfieldDecks(ctx context.Context, params MoxfieldSearchParams) (*MoxfieldSearchResponse, error) {
	return searchMoxfieldDecksWithURL(ctx, params, "https://api2.moxfield.com/v2/decks/search")
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

	// Build query parameters
	url := fmt.Sprintf("%s?pageSize=%d&pageNumber=%d",
		searchURL, params.PageSize, params.PageNumber)

	if params.Query != "" {
		url += fmt.Sprintf("&board=commanders&query=%s", params.Query)
	}
	if params.Format != "" {
		url += fmt.Sprintf("&fmt=%s", params.Format)
	}
	if params.SortType != "" {
		url += fmt.Sprintf("&sortType=%s", params.SortType)
	}
	if params.SortDirection != "" {
		url += fmt.Sprintf("&sortDirection=%s", params.SortDirection)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("moxfield search API returned status %d", resp.StatusCode)
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

	output.WriteString(fmt.Sprintf("# %s\n\n", deck.Name))
	output.WriteString(fmt.Sprintf("**Format:** %s\n", deck.Format))

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
			output.WriteString(fmt.Sprintf("**Author:** %s\n", strings.Join(authorStrs, ", ")))
		}
	}
skipAuthors:

	output.WriteString(fmt.Sprintf("**Views:** %d | **Likes:** %d | **Comments:** %d\n",
		deck.ViewCount, deck.LikeCount, deck.CommentCount))

	if deck.LastUpdated != "" {
		output.WriteString(fmt.Sprintf("**Last Updated:** %s\n", deck.LastUpdated))
	}

	if deck.Description != "" {
		output.WriteString(fmt.Sprintf("\n**Description:**\n%s\n", deck.Description))
	}

	if len(deck.Commanders) > 0 {
		output.WriteString("\n## Commanders\n")
		for _, entry := range deck.Commanders {
			output.WriteString(fmt.Sprintf("- %dx %s\n", entry.Quantity, entry.Card.Name))
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
	output.WriteString(fmt.Sprintf("**%s (%d):**\n", title, len(cards)))
	for _, c := range cards {
		output.WriteString(fmt.Sprintf("- %s\n", c))
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

	output.WriteString(fmt.Sprintf("\n**Total Cards:** %d\n\n", groups.totalCards+len(deck.Commanders)))

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
			output.WriteString(fmt.Sprintf("- %dx %s\n", entry.Quantity, entry.Card.Name))
		}
	}

	// Maybeboard
	if len(deck.Maybeboard) > 0 {
		output.WriteString("\n## Maybeboard\n")
		for _, entry := range deck.Maybeboard {
			output.WriteString(fmt.Sprintf("- %dx %s\n", entry.Quantity, entry.Card.Name))
		}
	}

	return output.String()
}
