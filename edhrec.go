package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const percentageMultiplier = 100.0

// EDHRECResponse represents the top-level response structure.
type EDHRECResponse struct {
	Container EDHRECContainer `json:"container"`
}

// EDHRECContainer wraps the JSON dictionary.
type EDHRECContainer struct {
	JSONDict EDHRECData `json:"json_dict"`
}

// EDHRECData contains the main data structure.
type EDHRECData struct {
	Card      EDHRECCardInfo   `json:"card"`
	CardLists []EDHRECCardList `json:"cardlists"`
	NumDecks  int              `json:"num_decks"`
}

// EDHRECCardInfo represents commander information.
type EDHRECCardInfo struct {
	Name      string   `json:"name"`
	Sanitized string   `json:"sanitized"`
	ColorID   []string `json:"color_id"`
	NumDecks  int      `json:"num_decks"`
}

// EDHRECCardList represents a category of cards.
type EDHRECCardList struct {
	Header    string           `json:"header"`
	Tag       string           `json:"tag"`
	CardViews []EDHRECCardView `json:"cardviews"`
}

// EDHRECCardView represents a card with statistics.
type EDHRECCardView struct {
	Name      string             `json:"name"`
	Sanitized string             `json:"sanitized"`
	Inclusion int                `json:"inclusion"`
	NumDecks  int                `json:"num_decks"`
	Synergy   float64            `json:"synergy"`
	Label     string             `json:"label"`
	Salt      float64            `json:"salt"`
	Prices    map[string]float64 `json:"prices"`
}

// EDHRECComboResponse represents combo data.
type EDHRECComboResponse struct {
	Container EDHRECComboContainer `json:"container"`
}

// EDHRECComboContainer wraps combo data.
type EDHRECComboContainer struct {
	JSONDict EDHRECComboData `json:"json_dict"`
}

// EDHRECComboData contains combo information.
type EDHRECComboData struct {
	ComboCounts []EDHRECCombo `json:"combocounts"`
}

// EDHRECCombo represents a card combo.
type EDHRECCombo struct {
	ComboID    string   `json:"comboId"`
	Colors     string   `json:"colors"`
	Count      int      `json:"count"`
	Percentage float64  `json:"percentage"`
	Rank       int      `json:"rank"`
	CardIDs    []string `json:"cardIds"`
	CardNames  []string `json:"cardNames"`
	Results    []string `json:"results"`
}

// SanitizeCardName converts a card name to EDHREC URL format.
func SanitizeCardName(name string) string {
	// Lowercase
	sanitized := strings.ToLower(name)

	// Remove special characters and replace spaces with hyphens
	reg := regexp.MustCompile("[^a-z0-9-]+")
	sanitized = reg.ReplaceAllString(strings.ReplaceAll(sanitized, " ", "-"), "")

	// Remove duplicate hyphens
	reg2 := regexp.MustCompile("-+")
	sanitized = reg2.ReplaceAllString(sanitized, "-")

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}

// GetCommanderRecommendations fetches EDHREC recommendations for a commander.
func GetCommanderRecommendations(ctx context.Context, commanderName string) (*EDHRECData, error) {
	return getCommanderRecommendationsWithURL(ctx, commanderName, "https://json.edhrec.com/pages")
}

// getCommanderRecommendationsWithURL fetches recommendations with a custom base URL.
func getCommanderRecommendationsWithURL(ctx context.Context, commanderName, baseURL string) (*EDHRECData, error) {
	sanitized := SanitizeCardName(commanderName)
	url := fmt.Sprintf("%s/commanders/%s.json", baseURL, sanitized)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EDHREC API returned status %d for %s", resp.StatusCode, commanderName)
	}

	var edhrecResp EDHRECResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&edhrecResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &edhrecResp.Container.JSONDict, nil
}

// GetCombosForColors fetches combos for a color combination.
func GetCombosForColors(ctx context.Context, colors string) (*EDHRECComboData, error) {
	return getCombosForColorsWithURL(ctx, colors, "https://json.edhrec.com/pages")
}

// getCombosForColorsWithURL fetches combos with a custom base URL.
func getCombosForColorsWithURL(ctx context.Context, colors, baseURL string) (*EDHRECComboData, error) {
	// Color codes: w (white), u (blue), b (black), r (red), g (green)
	// Examples: "wu" (azorius), "ubr" (grixis), "wubrg" (5-color)
	url := fmt.Sprintf("%s/combos/%s.json", baseURL, strings.ToLower(colors))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EDHREC combos API returned status %d", resp.StatusCode)
	}

	var comboResp EDHRECComboResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&comboResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode combo response: %w", decodeErr)
	}

	return &comboResp.Container.JSONDict, nil
}

// FormatCommanderRecsForDisplay formats EDHREC recommendations for text display.
func FormatCommanderRecsForDisplay(data *EDHRECData, limit int) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("# EDHREC Recommendations for %s\n\n", data.Card.Name))
	output.WriteString(fmt.Sprintf("**Total Decks:** %d\n", data.NumDecks))

	if len(data.Card.ColorID) > 0 {
		output.WriteString(fmt.Sprintf("**Color Identity:** %s\n\n", strings.Join(data.Card.ColorID, ", ")))
	}

	// Show each card list category
	for _, cardList := range data.CardLists {
		if len(cardList.CardViews) == 0 {
			continue
		}

		output.WriteString(fmt.Sprintf("\n## %s\n\n", cardList.Header))

		// Limit number of cards shown per category
		count := len(cardList.CardViews)
		if limit > 0 && count > limit {
			count = limit
		}

		for i := range count {
			card := cardList.CardViews[i]

			// Calculate percentage
			percentage := float64(card.Inclusion) / float64(data.NumDecks) * percentageMultiplier

			output.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, card.Name))
			output.WriteString(fmt.Sprintf("   - Inclusion: %d decks (%.1f%%)\n", card.Inclusion, percentage))

			if card.Synergy != 0 {
				output.WriteString(fmt.Sprintf("   - Synergy: %.2f\n", card.Synergy))
			}

			if card.Salt > 0 {
				output.WriteString(fmt.Sprintf("   - Salt Score: %.2f/4.0\n", card.Salt))
			}

			output.WriteString("\n")
		}

		if len(cardList.CardViews) > count {
			output.WriteString(fmt.Sprintf("*...and %d more cards*\n", len(cardList.CardViews)-count))
		}
	}

	return output.String()
}

// FormatCombosForDisplay formats combo data for text display.
func FormatCombosForDisplay(data *EDHRECComboData, limit int) string {
	var output strings.Builder

	output.WriteString("# Popular Combos\n\n")
	output.WriteString(fmt.Sprintf("**Total Combos:** %d\n\n", len(data.ComboCounts)))

	count := len(data.ComboCounts)
	if limit > 0 && count > limit {
		count = limit
	}

	for i := range count {
		combo := data.ComboCounts[i]

		output.WriteString(fmt.Sprintf("%d. **Combo #%d** (Rank #%d)\n", i+1, i+1, combo.Rank))

		if len(combo.CardNames) > 0 {
			output.WriteString(fmt.Sprintf("   **Cards:** %s\n", strings.Join(combo.CardNames, " + ")))
		}

		output.WriteString(fmt.Sprintf("   **Colors:** %s\n", strings.ToUpper(combo.Colors)))
		output.WriteString(fmt.Sprintf("   **Used in:** %d decks (%.2f%%)\n", combo.Count, combo.Percentage))

		if len(combo.Results) > 0 {
			output.WriteString(fmt.Sprintf("   **Results:** %s\n", strings.Join(combo.Results, ", ")))
		}

		output.WriteString("\n")
	}

	if len(data.ComboCounts) > count {
		output.WriteString(fmt.Sprintf("*...and %d more combos*\n", len(data.ComboCounts)-count))
	}

	return output.String()
}

// GetTopCardsForCategory fetches top cards for a specific category.
func GetTopCardsForCategory(ctx context.Context, category string, page int) ([]EDHRECCardView, error) {
	return getTopCardsForCategoryWithURL(ctx, category, page, "https://json.edhrec.com/pages")
}

// getTopCardsForCategoryWithURL fetches top cards with a custom base URL.
func getTopCardsForCategoryWithURL(
	ctx context.Context,
	category string,
	page int,
	baseURL string,
) ([]EDHRECCardView, error) {
	// Categories: salt, commanders, themes, etc.
	url := fmt.Sprintf("%s/top/%s--%d.json", baseURL, category, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EDHREC top cards API returned status %d", resp.StatusCode)
	}

	var edhrecResp EDHRECResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&edhrecResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	// Extract cards from all card lists
	var allCards []EDHRECCardView
	for _, cardList := range edhrecResp.Container.JSONDict.CardLists {
		allCards = append(allCards, cardList.CardViews...)
	}

	return allCards, nil
}
