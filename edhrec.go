package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/nathanmartins/mtg-mcp/internal/httpx"
)

const (
	percentageMultiplier = 100.0
	priceTierBudget      = "budget"
	priceTierExpensive   = "expensive"
	maxThemesListed      = 15
	defaultEDHRECLimit   = 10
	defaultSetCardsLimit = 25
)

// EDHRECResponse represents the top-level response structure.
type EDHRECResponse struct {
	Header    string           `json:"header"`
	Container EDHRECContainer  `json:"container"`
	TagCounts []EDHRECTagCount `json:"tag_counts"`
}

// EDHRECTagCount is one theme EDHREC tracks for a commander, as listed on the commander page.
type EDHRECTagCount struct {
	Slug  string `json:"slug"`
	Value string `json:"value"`
	Count int    `json:"count"`
}

// EDHRECContainer wraps the JSON dictionary.
type EDHRECContainer struct {
	JSONDict EDHRECData `json:"json_dict"`
}

// EDHRECData contains the main data structure.
type EDHRECData struct {
	Card      EDHRECCardInfo   `json:"card"`
	CardLists []EDHRECCardList `json:"cardlists"`
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
	Name           string  `json:"name"`
	Sanitized      string  `json:"sanitized"`
	NumDecks       int     `json:"num_decks"`
	PotentialDecks int     `json:"potential_decks"`
	Synergy        float64 `json:"synergy"`
	Salt           float64 `json:"salt"`
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
	CardLists []EDHRECComboList `json:"cardlists"`
}

// EDHRECComboList represents a combo entry in the new format.
type EDHRECComboList struct {
	Header    string           `json:"header"`
	CardViews []EDHRECCardView `json:"cardviews"`
	Combo     *EDHRECCombo     `json:"combo,omitempty"`
}

// EDHRECCombo represents a card combo.
type EDHRECCombo struct {
	ComboID string   `json:"comboId"`
	Cards   []string `json:"cards"`
	Results []string `json:"results"`
}

// EDHRECRecFilter narrows a commander recommendation request. An empty field means "no filter".
// EDHREC serves the theme segment before the price tier: commanders/<slug>/<theme>/<tier>.json.
// The reverse order answers 403.
type EDHRECRecFilter struct {
	Theme     string
	PriceTier string
}

// segments renders the filter as URL path segments in EDHREC's required order.
func (f EDHRECRecFilter) segments() string {
	var path strings.Builder
	if f.Theme != "" {
		path.WriteString("/")
		path.WriteString(url.PathEscape(f.Theme))
	}
	if f.PriceTier != "" {
		path.WriteString("/")
		path.WriteString(url.PathEscape(f.PriceTier))
	}

	return path.String()
}

// edhrecSlugInvalidChars and edhrecSlugHyphenRuns are compiled once rather than on every
// SanitizeCardName call.
var (
	edhrecSlugInvalidChars = regexp.MustCompile("[^a-z0-9-]+")
	edhrecSlugHyphenRuns   = regexp.MustCompile("-+")
)

// SanitizeCardName converts a card name to EDHREC URL format.
func SanitizeCardName(name string) string {
	// Lowercase
	sanitized := strings.ToLower(name)

	// Remove special characters and replace spaces with hyphens
	sanitized = edhrecSlugInvalidChars.ReplaceAllString(strings.ReplaceAll(sanitized, " ", "-"), "")

	// Remove duplicate hyphens
	sanitized = edhrecSlugHyphenRuns.ReplaceAllString(sanitized, "-")

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}

// fetchEDHRECPage GETs an EDHREC JSON page and decodes the body into target. EDHREC is an
// undocumented reverse-engineered API: unknown slugs answer 403 with an XML body, so any
// non-200 status is reported as an error without attempting to decode.
func fetchEDHRECPage(ctx context.Context, reqURL, pageKind string, target any) error {
	return httpx.GetJSON(ctx, httpx.APIClient(), reqURL, "EDHREC "+pageKind, target)
}

// getCommanderPageWithURL fetches a commander page from baseURL, optionally narrowed by filter.
func getCommanderPageWithURL(
	ctx context.Context,
	commanderName string,
	filter EDHRECRecFilter,
	baseURL string,
) (*EDHRECResponse, error) {
	sanitized := SanitizeCardName(commanderName)
	reqURL := fmt.Sprintf("%s/commanders/%s%s.json", baseURL, url.PathEscape(sanitized), filter.segments())

	var page EDHRECResponse
	if err := fetchEDHRECPage(ctx, reqURL, "commander page", &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// GetCommanderRecommendations fetches unfiltered EDHREC recommendations for a commander.
func GetCommanderRecommendations(ctx context.Context, commanderName string) (*EDHRECData, error) {
	page, err := getCommanderPageWithURL(ctx, commanderName, EDHRECRecFilter{}, defaultEDHRECBaseURL)
	if err != nil {
		return nil, err
	}

	return &page.Container.JSONDict, nil
}

// GetCombosForColors fetches combos for a color combination.
func GetCombosForColors(ctx context.Context, colors string) (*EDHRECComboData, error) {
	return getCombosForColorsWithURL(ctx, colors, defaultEDHRECBaseURL)
}

// getCombosForColorsWithURL fetches combos with a custom base URL.
func getCombosForColorsWithURL(ctx context.Context, colors, baseURL string) (*EDHRECComboData, error) {
	// Color codes: w (white), u (blue), b (black), r (red), g (green)
	// Examples: "wu" (azorius), "ubr" (grixis), "wubrg" (5-color)
	reqURL := fmt.Sprintf("%s/combos/%s.json", baseURL, url.PathEscape(strings.ToLower(colors)))

	var comboResp EDHRECComboResponse
	if err := fetchEDHRECPage(ctx, reqURL, "combos", &comboResp); err != nil {
		return nil, err
	}

	return &comboResp.Container.JSONDict, nil
}

// GetSetCards fetches the EDHREC page for a Magic set code.
func GetSetCards(ctx context.Context, setCode string) (*EDHRECResponse, error) {
	return getSetCardsWithURL(ctx, setCode, defaultEDHRECBaseURL)
}

// getSetCardsWithURL fetches a set page with a custom base URL. EDHREC keys set pages by the
// lowercase short set code ("rna", "c21"); any other casing answers 403.
func getSetCardsWithURL(ctx context.Context, setCode, baseURL string) (*EDHRECResponse, error) {
	code := strings.ToLower(strings.TrimSpace(setCode))
	reqURL := fmt.Sprintf("%s/sets/%s.json", baseURL, url.PathEscape(code))

	var page EDHRECResponse
	if err := fetchEDHRECPage(ctx, reqURL, "set page", &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// topThemes returns the highest-count themes, capped at maxThemesListed, plus the number
// omitted. tag_counts arrives sorted by count descending.
func topThemes(tags []EDHRECTagCount) ([]EDHRECTagCount, int) {
	count := len(tags)
	if count > maxThemesListed {
		count = maxThemesListed
	}

	return tags[:count], len(tags) - count
}

// commanderThemeHint returns a caller-facing message naming the themes EDHREC lists for a
// commander. It returns "" when the unfiltered page is unreachable, lists no themes, or already
// contains the requested slug — in those cases the failure is not the theme and the caller must
// surface the underlying EDHREC error instead.
func commanderThemeHint(ctx context.Context, commanderName, theme, baseURL string) string {
	page, err := getCommanderPageWithURL(ctx, commanderName, EDHRECRecFilter{}, baseURL)
	if err != nil || len(page.TagCounts) == 0 {
		return ""
	}

	suggestion := ""

	for _, tag := range page.TagCounts {
		if strings.EqualFold(tag.Slug, theme) {
			return ""
		}

		if suggestion == "" && strings.EqualFold(tag.Value, theme) {
			suggestion = fmt.Sprintf("Did you mean %q? ", tag.Slug)
		}
	}

	shown, omitted := topThemes(page.TagCounts)

	entries := make([]string, len(shown))
	for i, tag := range shown {
		entries[i] = fmt.Sprintf("%s (%d decks)", tag.Slug, tag.Count)
	}

	joined := strings.Join(entries, ", ")
	if omitted > 0 {
		joined += fmt.Sprintf(", and %d more", omitted)
	}

	return suggestion + fmt.Sprintf(
		"theme %q is not available for %s. Available themes include: %s. %d themes total.",
		theme, commanderName, joined, len(page.TagCounts),
	)
}

// writeCardDeckStats writes one card's deck line. denominator is the deck universe the card
// competes in: the commander's own deck count on recommendation pages, or the card's
// potential_decks on set pages. A non-positive denominator prints the raw count only.
func writeCardDeckStats(output *strings.Builder, numDecks, denominator int) {
	if denominator <= 0 {
		_, _ = fmt.Fprintf(output, "   - Decks: %d\n", numDecks)

		return
	}

	percentage := float64(numDecks) / float64(denominator) * percentageMultiplier
	_, _ = fmt.Fprintf(output, "   - Decks: %d of %d (%.1f%%)\n", numDecks, denominator, percentage)
}

// writeRecommendationCard writes one recommendation card's body, including its deck stats and
// the optional synergy/salt lines.
func writeRecommendationCard(output *strings.Builder, index int, card EDHRECCardView, denominator int) {
	_, _ = fmt.Fprintf(output, "%d. **%s**\n", index+1, card.Name)
	writeCardDeckStats(output, card.NumDecks, denominator)

	if card.Synergy != 0 {
		_, _ = fmt.Fprintf(output, "   - Synergy: %.2f\n", card.Synergy)
	}

	if card.Salt > 0 {
		_, _ = fmt.Fprintf(output, "   - Salt Score: %.2f/4.0\n", card.Salt)
	}

	output.WriteString("\n")
}

// FormatCommanderRecsForDisplay formats EDHREC recommendations for text display. filter is used
// only to decide whether to advertise the commander's other themes.
func FormatCommanderRecsForDisplay(page *EDHRECResponse, filter EDHRECRecFilter, limit int) string {
	data := &page.Container.JSONDict

	var output strings.Builder

	title := page.Header
	if title == "" {
		title = data.Card.Name
	}

	_, _ = fmt.Fprintf(&output, "# EDHREC Recommendations for %s\n\n", title)
	_, _ = fmt.Fprintf(&output, "**Total Decks:** %d\n", data.Card.NumDecks)

	if len(data.Card.ColorID) > 0 {
		_, _ = fmt.Fprintf(&output, "**Color Identity:** %s\n\n", strings.Join(data.Card.ColorID, ", "))
	}

	if filter.Theme == "" && filter.PriceTier == "" && len(page.TagCounts) > 0 {
		shown, omitted := topThemes(page.TagCounts)

		output.WriteString("\n## Available Themes\n\n")
		for _, tag := range shown {
			_, _ = fmt.Fprintf(&output, "- %s (%d decks)\n", tag.Slug, tag.Count)
		}

		if omitted > 0 {
			_, _ = fmt.Fprintf(&output, "\n*...and %d more themes*\n", omitted)
		}
	}

	// Show each card list category
	for _, cardList := range data.CardLists {
		if len(cardList.CardViews) == 0 {
			continue
		}

		_, _ = fmt.Fprintf(&output, "\n## %s\n\n", cardList.Header)

		// Limit number of cards shown per category
		count := len(cardList.CardViews)
		if limit > 0 && count > limit {
			count = limit
		}

		for i := range count {
			writeRecommendationCard(&output, i, cardList.CardViews[i], data.Card.NumDecks)
		}

		if len(cardList.CardViews) > count {
			_, _ = fmt.Fprintf(&output, "*...and %d more cards*\n", len(cardList.CardViews)-count)
		}
	}

	return output.String()
}

// FormatCombosForDisplay formats combo data for text display.
func FormatCombosForDisplay(data *EDHRECComboData, limit int) string {
	var output strings.Builder

	output.WriteString("# Popular Combos\n\n")
	_, _ = fmt.Fprintf(&output, "**Total Combos:** %d\n\n", len(data.CardLists))

	count := len(data.CardLists)
	if limit > 0 && count > limit {
		count = limit
	}

	for i := range count {
		comboList := data.CardLists[i]

		_, _ = fmt.Fprintf(&output, "%d. **%s**\n", i+1, comboList.Header)

		if len(comboList.CardViews) > 0 {
			cardNames := make([]string, len(comboList.CardViews))
			for j, card := range comboList.CardViews {
				cardNames[j] = card.Name
			}
			_, _ = fmt.Fprintf(&output, "   **Cards:** %s\n", strings.Join(cardNames, " + "))
		}

		if comboList.Combo != nil && len(comboList.Combo.Results) > 0 {
			_, _ = fmt.Fprintf(&output, "   **Results:** %s\n", strings.Join(comboList.Combo.Results, ", "))
		}

		output.WriteString("\n")
	}

	if len(data.CardLists) > count {
		_, _ = fmt.Fprintf(&output, "*...and %d more combos*\n", len(data.CardLists)-count)
	}

	return output.String()
}

// FormatSetCardsForDisplay formats an EDHREC set page for text display. setCode is echoed so the
// caller can see which code was resolved. limit caps each cardlist; limit <= 0 shows everything.
func FormatSetCardsForDisplay(page *EDHRECResponse, setCode string, limit int) string {
	var output strings.Builder

	title := page.Header
	if title == "" {
		_, _ = fmt.Fprintf(&output, "# EDHREC Set Overview: %s\n\n", setCode)
	} else {
		_, _ = fmt.Fprintf(&output, "# EDHREC Set Overview: %s (%s)\n\n", title, setCode)
	}

	for _, cardList := range page.Container.JSONDict.CardLists {
		if len(cardList.CardViews) == 0 {
			continue
		}

		_, _ = fmt.Fprintf(&output, "## %s (%d cards)\n\n", cardList.Header, len(cardList.CardViews))

		count := len(cardList.CardViews)
		if limit > 0 && count > limit {
			count = limit
		}

		for i := range count {
			card := cardList.CardViews[i]
			_, _ = fmt.Fprintf(&output, "%d. **%s**\n", i+1, card.Name)
			writeCardDeckStats(&output, card.NumDecks, card.PotentialDecks)
			output.WriteString("\n")
		}

		if len(cardList.CardViews) > count {
			_, _ = fmt.Fprintf(&output, "*...and %d more cards*\n\n", len(cardList.CardViews)-count)
		}
	}

	return output.String()
}

// getTopCardsForCategoryWithURL fetches top cards with a custom base URL.
func getTopCardsForCategoryWithURL(
	ctx context.Context,
	category string,
	page int,
	baseURL string,
) ([]EDHRECCardView, error) {
	// Categories: salt, commanders, themes, etc.
	reqURL := fmt.Sprintf("%s/top/%s--%d.json", baseURL, url.PathEscape(category), page)

	var edhrecResp EDHRECResponse
	if err := fetchEDHRECPage(ctx, reqURL, "top cards", &edhrecResp); err != nil {
		return nil, err
	}

	// Extract cards from all card lists
	var allCards []EDHRECCardView
	for _, cardList := range edhrecResp.Container.JSONDict.CardLists {
		allCards = append(allCards, cardList.CardViews...)
	}

	return allCards, nil
}
