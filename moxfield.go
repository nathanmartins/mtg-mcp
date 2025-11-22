package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const moxfieldBaseURL = "https://api.moxfield.com/v2"
const moxfieldSearchURL = "https://api2.moxfield.com/v2/decks/search"

// MoxfieldDeck represents a deck from Moxfield
type MoxfieldDeck struct {
	ID          string                       `json:"id"`
	PublicID    string                       `json:"publicId"`
	Name        string                       `json:"name"`
	Format      string                       `json:"format"`
	Description string                       `json:"description"`
	Mainboard   map[string]MoxfieldCardEntry `json:"mainboard"`
	Sideboard   map[string]MoxfieldCardEntry `json:"sideboard"`
	Commanders  map[string]MoxfieldCardEntry `json:"commanders"`
	Maybeboard  map[string]MoxfieldCardEntry `json:"maybeboard"`
	CreatedAt   string                       `json:"createdAtUtc"`
	LastUpdated string                       `json:"lastUpdatedAtUtc"`
	ViewCount   int                          `json:"viewCount"`
	LikeCount   int                          `json:"likeCount"`
	CommentCount int                         `json:"commentCount"`
	Authors     []string                     `json:"authors"`
}

// MoxfieldCardEntry represents a card in a Moxfield deck
type MoxfieldCardEntry struct {
	Quantity int              `json:"quantity"`
	Card     MoxfieldCardInfo `json:"card"`
}

// MoxfieldCardInfo represents card information
type MoxfieldCardInfo struct {
	Name     string `json:"name"`
	Set      string `json:"set"`
	TypeLine string `json:"type_line"`
	ManaCost string `json:"mana_cost"`
	Rarity   string `json:"rarity"`
}

// MoxfieldUserDecksResponse represents paginated user decks
type MoxfieldUserDecksResponse struct {
	PageNumber   int                    `json:"pageNumber"`
	PageSize     int                    `json:"pageSize"`
	TotalResults int                    `json:"totalResults"`
	TotalPages   int                    `json:"totalPages"`
	Data         []MoxfieldDeckSummary  `json:"data"`
}

// MoxfieldDeckSummary represents a deck summary in list view
type MoxfieldDeckSummary struct {
	PublicID  string `json:"publicId"`
	Name      string `json:"name"`
	Format    string `json:"format"`
	PublicURL string `json:"publicUrl"`
	ViewCount int    `json:"viewCount"`
	LikeCount int    `json:"likeCount"`
}

// MoxfieldSearchResponse represents search results from Moxfield
type MoxfieldSearchResponse struct {
	PageNumber   int                    `json:"pageNumber"`
	PageSize     int                    `json:"pageSize"`
	TotalResults int                    `json:"totalResults"`
	TotalPages   int                    `json:"totalPages"`
	Data         []MoxfieldDeckSummary  `json:"data"`
}

// MoxfieldSearchParams represents search parameters
type MoxfieldSearchParams struct {
	Query         string
	Format        string
	SortType      string
	SortDirection string
	PageSize      int
	PageNumber    int
}

// GetMoxfieldDeck fetches a deck by its public ID
func GetMoxfieldDeck(ctx context.Context, publicID string) (*MoxfieldDeck, error) {
	url := fmt.Sprintf("%s/decks/all/%s", moxfieldBaseURL, publicID)

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
	if err := json.NewDecoder(resp.Body).Decode(&deck); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &deck, nil
}

// GetUserDecks fetches a user's deck list
func GetUserDecks(ctx context.Context, username string, pageSize int) (*MoxfieldUserDecksResponse, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}

	url := fmt.Sprintf("%s/users/%s/decks?pageSize=%d", moxfieldBaseURL, username, pageSize)

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
	if err := json.NewDecoder(resp.Body).Decode(&decksResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &decksResp, nil
}

// SearchMoxfieldDecks searches for decks on Moxfield
func SearchMoxfieldDecks(ctx context.Context, params MoxfieldSearchParams) (*MoxfieldSearchResponse, error) {
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if params.PageNumber < 1 {
		params.PageNumber = 1
	}

	// Build query parameters
	url := fmt.Sprintf("%s?pageSize=%d&pageNumber=%d",
		moxfieldSearchURL, params.PageSize, params.PageNumber)

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
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &searchResp, nil
}

// ExtractPublicIDFromURL extracts the public ID from a Moxfield URL
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

// FormatDeckForDisplay formats a Moxfield deck for text display
func FormatDeckForDisplay(deck *MoxfieldDeck) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("# %s\n\n", deck.Name))
	output.WriteString(fmt.Sprintf("**Format:** %s\n", deck.Format))

	if len(deck.Authors) > 0 {
		output.WriteString(fmt.Sprintf("**Author:** %s\n", strings.Join(deck.Authors, ", ")))
	}

	output.WriteString(fmt.Sprintf("**Views:** %d | **Likes:** %d | **Comments:** %d\n",
		deck.ViewCount, deck.LikeCount, deck.CommentCount))

	if deck.LastUpdated != "" {
		output.WriteString(fmt.Sprintf("**Last Updated:** %s\n", deck.LastUpdated))
	}

	if deck.Description != "" {
		output.WriteString(fmt.Sprintf("\n**Description:**\n%s\n", deck.Description))
	}

	// Commanders
	if len(deck.Commanders) > 0 {
		output.WriteString("\n## Commanders\n")
		for _, entry := range deck.Commanders {
			output.WriteString(fmt.Sprintf("- %dx %s\n", entry.Quantity, entry.Card.Name))
		}
	}

	// Mainboard
	totalCards := 0
	output.WriteString("\n## Mainboard\n")

	// Group by card type
	creatures := []string{}
	instants := []string{}
	sorceries := []string{}
	artifacts := []string{}
	enchantments := []string{}
	planeswalkers := []string{}
	lands := []string{}
	others := []string{}

	for _, entry := range deck.Mainboard {
		cardLine := fmt.Sprintf("%dx %s", entry.Quantity, entry.Card.Name)
		totalCards += entry.Quantity

		typeLine := strings.ToLower(entry.Card.TypeLine)
		if strings.Contains(typeLine, "creature") {
			creatures = append(creatures, cardLine)
		} else if strings.Contains(typeLine, "instant") {
			instants = append(instants, cardLine)
		} else if strings.Contains(typeLine, "sorcery") {
			sorceries = append(sorceries, cardLine)
		} else if strings.Contains(typeLine, "artifact") {
			artifacts = append(artifacts, cardLine)
		} else if strings.Contains(typeLine, "enchantment") {
			enchantments = append(enchantments, cardLine)
		} else if strings.Contains(typeLine, "planeswalker") {
			planeswalkers = append(planeswalkers, cardLine)
		} else if strings.Contains(typeLine, "land") {
			lands = append(lands, cardLine)
		} else {
			others = append(others, cardLine)
		}
	}

	output.WriteString(fmt.Sprintf("\n**Total Cards:** %d\n\n", totalCards+len(deck.Commanders)))

	if len(creatures) > 0 {
		output.WriteString(fmt.Sprintf("**Creatures (%d):**\n", len(creatures)))
		for _, c := range creatures {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
		output.WriteString("\n")
	}

	if len(instants) > 0 {
		output.WriteString(fmt.Sprintf("**Instants (%d):**\n", len(instants)))
		for _, c := range instants {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
		output.WriteString("\n")
	}

	if len(sorceries) > 0 {
		output.WriteString(fmt.Sprintf("**Sorceries (%d):**\n", len(sorceries)))
		for _, c := range sorceries {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
		output.WriteString("\n")
	}

	if len(artifacts) > 0 {
		output.WriteString(fmt.Sprintf("**Artifacts (%d):**\n", len(artifacts)))
		for _, c := range artifacts {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
		output.WriteString("\n")
	}

	if len(enchantments) > 0 {
		output.WriteString(fmt.Sprintf("**Enchantments (%d):**\n", len(enchantments)))
		for _, c := range enchantments {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
		output.WriteString("\n")
	}

	if len(planeswalkers) > 0 {
		output.WriteString(fmt.Sprintf("**Planeswalkers (%d):**\n", len(planeswalkers)))
		for _, c := range planeswalkers {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
		output.WriteString("\n")
	}

	if len(lands) > 0 {
		output.WriteString(fmt.Sprintf("**Lands (%d):**\n", len(lands)))
		for _, c := range lands {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
		output.WriteString("\n")
	}

	if len(others) > 0 {
		output.WriteString(fmt.Sprintf("**Other (%d):**\n", len(others)))
		for _, c := range others {
			output.WriteString(fmt.Sprintf("- %s\n", c))
		}
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
