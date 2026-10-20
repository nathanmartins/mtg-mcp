package archidekt

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/nathanmartins/mtg-mcp/internal/httpx"
)

const (
	// archidektAPIPageSize is the page size Archidekt hardcodes. The API has no
	// page-size parameter: pageSize, limit and perPage are all ignored upstream.
	archidektAPIPageSize = 60
	// SearchMaxLimit is the largest logical page we serve, bounded by the
	// fixed upstream page size so one request never needs more than two API pages.
	SearchMaxLimit = 60
	// SearchDefaultLimit is the default number of decks per logical page.
	SearchDefaultLimit = 10
	// archidektCountCap is the saturation value Archidekt reports for broad filters.
	archidektCountCap = 1000
	// archidektCountUnknown is what Archidekt reports when it does not compute a total.
	archidektCountUnknown = -1
	// archidektCommanderFormat is Archidekt's deckFormat id for Commander/EDH.
	archidektCommanderFormat = "3"
	// MinBracket is the lowest EDH bracket the bracket filter accepts.
	MinBracket = 1
	// MaxBracket is the highest EDH bracket the bracket filter accepts.
	MaxBracket = 4
	// MaxDeckSize bounds the deck-size filter (Commander decks are 100 cards;
	// the cap only exists to reject nonsense input).
	MaxDeckSize = 1000
	// archidektColorLetters lists the colour letters Archidekt's colors filter accepts.
	archidektColorLetters = "WUBRG"
	// archidektOptionalFilters is how many optional filters the formatter can list.
	archidektOptionalFilters = 4
	// SearchMaxPage bounds logical pagination. Archidekt's count saturates at
	// archidektCountCap, so nothing is reachable beyond that many results; the bound also
	// keeps (page-1)*limit from overflowing int and producing a negative window offset.
	SearchMaxPage = archidektCountCap

	archidektSortViews   = "views"
	archidektSortUpdated = "updated"
	archidektSortCreated = "created"
	archidektSortPrice   = "price"
	archidektSortSize    = "size"
)

// SearchParams describes one logical page of an Archidekt deck search.
type SearchParams struct {
	Commander string
	Bracket   int    // 1-4 filters by EDH bracket; 0 means no filter
	Sort      string // logical sort key; must be one of archidektSortFields
	Ascending bool
	Page      int    // 1-based logical page
	Limit     int    // decks per logical page, 1..archidektSearchMaxLimit
	Colors    string // e.g. "WU"; empty means no filter
	DeckSize  int    // exact card count, e.g. 99; 0 means no filter
	Author    string // Archidekt username; empty means no filter
	DeckName  string // deck-name substring; empty means no filter
}

// SearchResult is one logical page of deck search results. Total is only
// meaningful when TotalKnown is true; TotalCapped marks a saturated upstream count,
// meaning the real total is larger than Total.
type SearchResult struct {
	Decks       []DeckSummary
	Page        int
	Limit       int
	Total       int
	TotalCapped bool
	TotalKnown  bool
}

// normalized clamps parameters to the range the upstream API and formatter can honour.
func (p SearchParams) normalized() SearchParams {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Page > SearchMaxPage {
		p.Page = SearchMaxPage
	}
	if p.Limit < 1 {
		p.Limit = SearchDefaultLimit
	}
	if p.Limit > SearchMaxLimit {
		p.Limit = SearchMaxLimit
	}
	return p
}

// archidektSortField pairs a tool-level sort key with Archidekt's upstream orderBy field.
type archidektSortField struct {
	Key   string
	Field string
}

// archidektSortFields lists the accepted sort keys and their upstream fields, in the
// order the tool documents them. Single source for validation, the error message and the
// tool description: Archidekt silently ignores an unknown orderBy instead of rejecting
// it, so an undetected typo would return a wrongly ordered page.
func archidektSortFields() []archidektSortField {
	return []archidektSortField{
		{archidektSortViews, "viewCount"},
		{archidektSortUpdated, "updatedAt"},
		{archidektSortCreated, "createdAt"},
		{archidektSortPrice, "price"},
		{archidektSortSize, "size"},
	}
}

// SortValues renders the accepted sort keys for error messages and tool docs.
func SortValues() string {
	fields := archidektSortFields()
	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.Key)
	}
	return strings.Join(keys, ", ")
}

// OrderBy renders the upstream orderBy value for a sort key. Archidekt accepts
// unknown orderBy values and silently ignores them, so validation must happen here.
func OrderBy(sort string, ascending bool) (string, error) {
	for _, field := range archidektSortFields() {
		if field.Key != sort {
			continue
		}
		if ascending {
			return field.Field, nil
		}
		return "-" + field.Field, nil
	}
	return "", fmt.Errorf("unsupported sort %q (accepted: %s)", sort, SortValues())
}

// NormalizeColors converts a colour string such as "WU" into the
// comma-separated form Archidekt expects ("W,U"). Duplicate or unknown letters are
// rejected: Archidekt would accept them and return a silently different result set.
func NormalizeColors(colors string) (string, error) {
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

// SearchDecks searches public Commander decks on Archidekt.
func (c *Client) SearchDecks(ctx context.Context, params SearchParams) (*SearchResult, error) {
	params = params.normalized()

	// Normalise again here rather than trusting the caller: this is the library entry
	// point as well as the handler's, the value is sent on every upstream page and echoed
	// in the output, and Archidekt would accept a malformed colour string and quietly
	// return an unfiltered page. Normalisation is idempotent, so the handler's own call
	// costs nothing.
	colors, err := NormalizeColors(params.Colors)
	if err != nil {
		return nil, err
	}
	params.Colors = colors

	orderBy, err := OrderBy(params.Sort, params.Ascending)
	if err != nil {
		return nil, err
	}

	firstAPIPage, offset, apiPages := archidektPageWindow(params.Page, params.Limit)
	result := &SearchResult{Page: params.Page, Limit: params.Limit}
	collected := make([]DeckSummary, 0, apiPages*archidektAPIPageSize)

	for i := range apiPages {
		page, fetchErr := c.fetchSearchPage(ctx, params, orderBy, firstAPIPage+i)
		if fetchErr != nil {
			return nil, fetchErr
		}
		if i == 0 {
			result.Total, result.TotalCapped, result.TotalKnown = interpretArchidektCount(page.Count)
		}
		// The logical window is arithmetic over fixed-size upstream pages, so a page that
		// is not full while more pages exist means Archidekt changed its page size and
		// every offset here is wrong. Fail loudly instead of serving a silently wrong
		// slice of results.
		if page.Next != "" && len(page.Results) != archidektAPIPageSize {
			return nil, fmt.Errorf(
				"archidekt page size changed: got %d results, expected %d",
				len(page.Results), archidektAPIPageSize)
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

// fetchSearchPage requests a single fixed-size upstream page of results.
// params.Colors must already be normalised by SearchDecks.
func (c *Client) fetchSearchPage(
	ctx context.Context,
	params SearchParams,
	orderBy string,
	apiPage int,
) (*UserDecksResponse, error) {
	queryParams := url.Values{}
	queryParams.Set("commanderName", params.Commander)
	queryParams.Set("deckFormat", archidektCommanderFormat)
	queryParams.Set("orderBy", orderBy)
	if params.Colors != "" {
		queryParams.Set("colors", params.Colors)
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
	if params.Bracket >= MinBracket && params.Bracket <= MaxBracket {
		queryParams.Set("edhBracket", strconv.Itoa(params.Bracket))
	}
	if apiPage > 1 {
		queryParams.Set("page", strconv.Itoa(apiPage))
	}

	reqURL := fmt.Sprintf("%s/decks/v3/?%s", c.BaseURL, queryParams.Encode())

	var result UserDecksResponse
	if err := httpx.GetJSON(ctx, httpx.APIClient(), reqURL, apiSource, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// archidektDirectionLabel renders a sort order as the asc/desc value the search tool accepts.
// Archidekt's "-" orderBy prefix must never reach the output, or the tool would echo a
// direction the caller cannot type back.
func archidektDirectionLabel(ascending bool) string {
	if ascending {
		return "asc"
	}
	return "desc"
}

// FormatSearchResults renders one logical page of search results.
func FormatSearchResults(params SearchParams, result *SearchResult) string {
	var output strings.Builder

	if params.Bracket >= MinBracket && params.Bracket <= MaxBracket {
		_, _ = fmt.Fprintf(&output, "# Archidekt Decks: %s (Bracket %d)\n\n", params.Commander, params.Bracket)
	} else {
		_, _ = fmt.Fprintf(&output, "# Archidekt Decks: %s\n\n", params.Commander)
	}

	_, _ = fmt.Fprintf(&output, "**Sort:** %s (%s)\n", params.Sort, archidektDirectionLabel(params.Ascending))
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
func formatArchidektTotal(result *SearchResult) string {
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
func archidektHasMorePages(result *SearchResult) bool {
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
func formatArchidektFilters(params SearchParams) string {
	filters := make([]string, 0, archidektOptionalFilters)
	if params.Colors != "" {
		filters = append(filters, "colors "+strings.ReplaceAll(params.Colors, ",", ""))
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
