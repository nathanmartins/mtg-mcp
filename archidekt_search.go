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
	// archidektAPIPageSize is the page size Archidekt hardcodes. The API has no
	// page-size parameter: pageSize, limit and perPage are all ignored upstream.
	archidektAPIPageSize = 60
	// archidektSearchMaxLimit is the largest logical page we serve, bounded by the
	// fixed upstream page size so one request never needs more than two API pages.
	archidektSearchMaxLimit = 60
	// archidektSearchDefaultLimit is the default number of decks per logical page.
	archidektSearchDefaultLimit = 10
	// archidektCountCap is the saturation value Archidekt reports for broad filters.
	archidektCountCap = 1000
	// archidektCountUnknown is what Archidekt reports when it does not compute a total.
	archidektCountUnknown = -1
	// archidektCommanderFormat is Archidekt's deckFormat id for Commander/EDH.
	archidektCommanderFormat = "3"
	// archidektMinBracket and archidektMaxBracket bound the EDH bracket filter.
	archidektMinBracket = 1
	archidektMaxBracket = 4
	// archidektMaxDeckSize bounds the deck-size filter (Commander decks are 100 cards;
	// the cap only exists to reject nonsense input).
	archidektMaxDeckSize = 1000
	// archidektColorLetters lists the colour letters Archidekt's colors filter accepts.
	archidektColorLetters = "WUBRG"
	// archidektOptionalFilters is how many optional filters the formatter can list.
	archidektOptionalFilters = 4

	archidektSortViews   = "views"
	archidektSortUpdated = "updated"
	archidektSortCreated = "created"
	archidektSortPrice   = "price"
	archidektSortSize    = "size"
	// archidektSortValues lists accepted sort keys for error messages.
	archidektSortValues = "created, price, size, updated, views"
)

// ArchidektSearchParams describes one logical page of an Archidekt deck search.
type ArchidektSearchParams struct {
	Commander string
	Bracket   int    // 1-4 filters by EDH bracket; 0 means no filter
	Sort      string // logical sort key; empty defaults to archidektSortViews
	Ascending bool
	Page      int    // 1-based logical page
	Limit     int    // decks per logical page, 1..archidektSearchMaxLimit
	Colors    string // e.g. "WU"; empty means no filter
	DeckSize  int    // exact card count, e.g. 99; 0 means no filter
	Author    string // Archidekt username; empty means no filter
	DeckName  string // deck-name substring; empty means no filter
}

// ArchidektSearchResult is one logical page of deck search results. Total is only
// meaningful when TotalKnown is true; TotalCapped marks a saturated upstream count,
// meaning the real total is larger than Total.
type ArchidektSearchResult struct {
	Decks       []ArchidektDeckSummary
	Page        int
	Limit       int
	Total       int
	TotalCapped bool
	TotalKnown  bool
}

// normalized clamps parameters to the range the upstream API and formatter can honour.
func (p ArchidektSearchParams) normalized() ArchidektSearchParams {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = archidektSearchDefaultLimit
	}
	if p.Limit > archidektSearchMaxLimit {
		p.Limit = archidektSearchMaxLimit
	}
	return p
}

// archidektOrderBy renders the upstream orderBy value for a sort key. Archidekt accepts
// unknown orderBy values and silently ignores them, so validation must happen here.
func archidektOrderBy(sort string, ascending bool) (string, error) {
	var field string
	switch sort {
	case archidektSortViews:
		field = "viewCount"
	case archidektSortUpdated:
		field = "updatedAt"
	case archidektSortCreated:
		field = "createdAt"
	case archidektSortPrice:
		field = "price"
	case archidektSortSize:
		field = "size"
	default:
		return "", fmt.Errorf("unsupported sort %q (accepted: %s)", sort, archidektSortValues)
	}

	if ascending {
		return field, nil
	}
	return "-" + field, nil
}

// normalizeArchidektColors converts a colour string such as "WU" into the
// comma-separated form Archidekt expects ("W,U"). Duplicate or unknown letters are
// rejected: Archidekt would accept them and return a silently different result set.
func normalizeArchidektColors(colors string) (string, error) {
	trimmed := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(colors), ",", ""))
	if trimmed == "" {
		return "", nil
	}

	seen := make(map[rune]bool, len(trimmed))
	letters := make([]string, 0, len(trimmed))
	for _, letter := range trimmed {
		if !strings.ContainsRune(archidektColorLetters, letter) {
			return "", fmt.Errorf("invalid color %q (accepted letters: %s)", string(letter), archidektColorLetters)
		}
		if seen[letter] {
			return "", fmt.Errorf("duplicate color %q", string(letter))
		}
		seen[letter] = true
		letters = append(letters, string(letter))
	}

	return strings.Join(letters, ","), nil
}

// archidektPageWindow maps a logical page request onto the fixed-size upstream pages
// covering it, returning the first API page to request, the offset of the logical window
// inside that page, and how many API pages to fetch. A logical window may straddle two
// upstream pages, never more, because Limit is capped at the upstream page size.
func archidektPageWindow(page, limit int) (int, int, int) {
	absolute := (page - 1) * limit
	firstAPIPage := absolute/archidektAPIPageSize + 1
	offset := absolute % archidektAPIPageSize
	apiPages := 1
	if offset+limit > archidektAPIPageSize {
		apiPages = 2
	}
	return firstAPIPage, offset, apiPages
}

// interpretArchidektCount decodes the three meanings of Archidekt's count field —
// an exact total, the 1000 saturation cap, or -1 for "not computed" — returning the
// total, whether it is the saturated cap, and whether Archidekt reported it at all.
func interpretArchidektCount(count int) (int, bool, bool) {
	switch {
	case count == archidektCountUnknown:
		return 0, false, false
	case count >= archidektCountCap:
		return archidektCountCap, true, true
	default:
		return count, false, true
	}
}

// SearchArchidektDecks searches public Commander decks on Archidekt.
func SearchArchidektDecks(ctx context.Context, params ArchidektSearchParams) (*ArchidektSearchResult, error) {
	return searchArchidektDecksWithURL(ctx, params, defaultArchidektBaseURL)
}

// searchArchidektDecksWithURL is the testable variant that accepts a custom base URL.
func searchArchidektDecksWithURL(
	ctx context.Context,
	params ArchidektSearchParams,
	baseURL string,
) (*ArchidektSearchResult, error) {
	params = params.normalized()

	// Reject an invalid colour filter before the first request: Archidekt would accept
	// it and quietly return an unfiltered page.
	if _, err := normalizeArchidektColors(params.Colors); err != nil {
		return nil, err
	}

	orderBy, err := archidektOrderBy(params.Sort, params.Ascending)
	if err != nil {
		return nil, err
	}

	firstAPIPage, offset, apiPages := archidektPageWindow(params.Page, params.Limit)
	result := &ArchidektSearchResult{Page: params.Page, Limit: params.Limit}
	collected := make([]ArchidektDeckSummary, 0, apiPages*archidektAPIPageSize)

	for i := range apiPages {
		page, fetchErr := fetchArchidektSearchPage(ctx, params, orderBy, firstAPIPage+i, baseURL)
		if fetchErr != nil {
			return nil, fetchErr
		}
		if i == 0 {
			result.Total, result.TotalCapped, result.TotalKnown = interpretArchidektCount(page.Count)
		}
		collected = append(collected, page.Results...)
		if page.Next == "" {
			break
		}
	}

	if offset < len(collected) {
		end := min(offset+params.Limit, len(collected))
		result.Decks = collected[offset:end]
	}

	return result, nil
}

// fetchArchidektSearchPage requests a single fixed-size upstream page of results.
func fetchArchidektSearchPage(
	ctx context.Context,
	params ArchidektSearchParams,
	orderBy string,
	apiPage int,
	baseURL string,
) (*ArchidektUserDecksResponse, error) {
	queryParams := url.Values{}
	queryParams.Set("commanderName", params.Commander)
	queryParams.Set("deckFormat", archidektCommanderFormat)
	queryParams.Set("orderBy", orderBy)
	colors, err := normalizeArchidektColors(params.Colors)
	if err != nil {
		return nil, err
	}
	if colors != "" {
		queryParams.Set("colors", colors)
	}
	if params.DeckSize > 0 {
		queryParams.Set("size", strconv.Itoa(params.DeckSize))
	}
	if params.Author != "" {
		queryParams.Set("ownerUsername", params.Author)
	}
	if params.DeckName != "" {
		queryParams.Set("name", params.DeckName)
	}
	if params.Bracket >= archidektMinBracket && params.Bracket <= archidektMaxBracket {
		queryParams.Set("edhBracket", strconv.Itoa(params.Bracket))
	}
	if apiPage > 1 {
		queryParams.Set("page", strconv.Itoa(apiPage))
	}

	reqURL := fmt.Sprintf("%s/decks/v3/?%s", baseURL, queryParams.Encode())

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
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("archidekt API returned status %d", resp.StatusCode)
	}

	var result ArchidektUserDecksResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &result, nil
}

// FormatArchidektSearchResultsForDisplay renders one logical page of search results.
func FormatArchidektSearchResultsForDisplay(params ArchidektSearchParams, result *ArchidektSearchResult) string {
	var output strings.Builder

	if params.Bracket >= archidektMinBracket && params.Bracket <= archidektMaxBracket {
		_, _ = fmt.Fprintf(&output, "# Archidekt Decks: %s (Bracket %d)\n\n", params.Commander, params.Bracket)
	} else {
		_, _ = fmt.Fprintf(&output, "# Archidekt Decks: %s\n\n", params.Commander)
	}

	direction := "descending"
	if params.Ascending {
		direction = "ascending"
	}
	_, _ = fmt.Fprintf(&output, "**Sort:** %s (%s)\n", params.Sort, direction)
	if filters := formatArchidektFilters(params); filters != "" {
		_, _ = fmt.Fprintf(&output, "**Filters:** %s\n", filters)
	}
	_, _ = fmt.Fprintf(&output, "**Total Results:** %s\n", formatArchidektTotal(result))

	if len(result.Decks) == 0 {
		_, _ = fmt.Fprintf(&output, "**Page:** %d\n\n", result.Page)
		output.WriteString("No public decks found for this search.\n")
		return output.String()
	}

	first := (result.Page-1)*result.Limit + 1
	_, _ = fmt.Fprintf(&output, "**Page:** %d — showing decks %d–%d\n\n", result.Page, first, first+len(result.Decks)-1)

	for i, deck := range result.Decks {
		_, _ = fmt.Fprintf(&output, "## %d. %s\n", first+i, deck.Name)
		_, _ = fmt.Fprintf(&output, "- **Author:** %s\n", deck.Owner.Username)
		_, _ = fmt.Fprintf(&output, "- **Views:** %d\n", deck.ViewCount)
		if deck.EdhBracket != nil {
			_, _ = fmt.Fprintf(&output, "- **EDH Bracket:** %d\n", *deck.EdhBracket)
		}
		_, _ = fmt.Fprintf(&output, "- **Last Updated:** %s\n", deck.UpdatedAt)
		_, _ = fmt.Fprintf(&output, "- **URL:** https://archidekt.com/decks/%d\n\n", deck.ID)
	}

	if archidektHasMorePages(result) {
		_, _ = fmt.Fprintf(&output, "*More decks may be available — request page %d.*\n", result.Page+1)
	}

	return output.String()
}

// formatArchidektTotal renders the total without inventing numbers Archidekt did not report.
func formatArchidektTotal(result *ArchidektSearchResult) string {
	switch {
	case !result.TotalKnown:
		return "unknown (not reported by Archidekt)"
	case result.TotalCapped:
		return fmt.Sprintf("%d+ (upstream cap)", result.Total)
	default:
		return strconv.Itoa(result.Total)
	}
}

// archidektHasMorePages reports whether another logical page may exist. A capped total
// is a lower bound, so it can never prove the result set is exhausted.
func archidektHasMorePages(result *ArchidektSearchResult) bool {
	if len(result.Decks) < result.Limit {
		return false
	}
	if !result.TotalKnown || result.TotalCapped {
		return true
	}
	return result.Page*result.Limit < result.Total
}

// formatArchidektFilters lists the active optional filters so the caller can see
// exactly which constraints produced the page.
func formatArchidektFilters(params ArchidektSearchParams) string {
	filters := make([]string, 0, archidektOptionalFilters)
	if params.Colors != "" {
		filters = append(filters, "colors "+strings.ToUpper(strings.ReplaceAll(params.Colors, ",", "")))
	}
	if params.DeckSize > 0 {
		filters = append(filters, "size "+strconv.Itoa(params.DeckSize))
	}
	if params.Author != "" {
		filters = append(filters, "author "+params.Author)
	}
	if params.DeckName != "" {
		filters = append(filters, "name "+params.DeckName)
	}
	return strings.Join(filters, " · ")
}
