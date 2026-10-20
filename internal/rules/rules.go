// Package rules fetches, parses and caches the official Magic: The Gathering Comprehensive Rules.
package rules

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nathanmartins/mtg-mcp/internal/httpx"
	"github.com/nathanmartins/mtg-mcp/internal/logging"
)

const (
	rulesPageURL  = "https://magic.wizards.com/en/rules"
	rulesCacheTTL = 24 * time.Hour
	// SearchDefaultLimit is the default number of matches search_rules returns.
	SearchDefaultLimit = 10
	// SearchMaxLimit caps the number of matches search_rules returns.
	SearchMaxLimit    = 20
	rulesHTTPTimeout  = 30 * time.Second
	rulesMaxBodyBytes = 8 << 20 // 8 MiB cap on fetched bodies (the rules .txt is ~1 MiB)
)

// rulesTxtLinkPattern discovers the comprehensive rules .txt URL embedded in the WotC rules page.
// The filename carries a date and a space (e.g. "MagicCompRules 20260619.txt"), so spaces are allowed.
var rulesTxtLinkPattern = regexp.MustCompile(
	`https?://[^"\s]*MagicCompRules[^"]*?\.txt`,
)

// ruleNumberPattern matches a rule/subrule number at the start of a line (e.g. "702.19." or "702.19a").
var ruleNumberPattern = regexp.MustCompile(
	`^(\d+\.\d+[a-z]*)\.?\s`,
)

// Rules holds the parsed Magic: The Gathering comprehensive rules.
type Rules struct {
	SourceURL string            // the .txt URL the rules were fetched from
	Raw       string            // full original text (served by the rules://comprehensive resource)
	ruleNums  []string          // rule numbers in document order
	rules     map[string]string // rule number -> full text of that rule/subrule
	glossary  map[string]string // lowercased term -> definition
}

// Parse parses the raw rules .txt into numbered rules and a glossary.
func Parse(raw string) *Rules {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	cr := &Rules{
		Raw:      raw,
		rules:    map[string]string{},
		glossary: map[string]string{},
	}
	lines := strings.Split(raw, "\n")

	// The word "Glossary" can appear in the table of contents; the real section is the last occurrence.
	glossaryStart := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "Glossary" {
			glossaryStart = i
		}
	}

	parseNumberedRules(cr, lines[:glossaryStart])
	if glossaryStart < len(lines) {
		parseGlossary(cr, lines[glossaryStart+1:])
	}
	return cr
}

// parseNumberedRules groups lines into entries keyed by rule number.
func parseNumberedRules(cr *Rules, lines []string) {
	var curNum string
	var buf []string
	flush := func() {
		if curNum != "" {
			cr.ruleNums = append(cr.ruleNums, curNum)
			cr.rules[curNum] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
		buf = nil
	}
	for _, line := range lines {
		if m := ruleNumberPattern.FindStringSubmatch(line); m != nil {
			flush()
			curNum = m[1]
		}
		if curNum != "" {
			buf = append(buf, line)
		}
	}
	flush()
}

// parseGlossary parses "term\n definition\n\n" blocks into the glossary map.
func parseGlossary(cr *Rules, lines []string) {
	var term string
	var buf []string
	flush := func() {
		if term != "" {
			cr.glossary[strings.ToLower(term)] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
		buf = nil
		term = ""
	}
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			flush()
			continue
		}
		if term == "" {
			term = t
		} else {
			buf = append(buf, line)
		}
	}
	flush()
}

// Match is a single result from Search.
type Match struct {
	Number string
	Text   string
}

// Rule returns a rule's text. A parent number (e.g. "702.19") includes its subrules;
// a subrule number (e.g. "702.19a") returns just that subrule.
func (cr *Rules) Rule(number string) (string, bool) {
	number = strings.TrimSpace(number)
	text, ok := cr.rules[number]
	if !ok {
		return "", false
	}
	var b strings.Builder
	b.WriteString(text)
	for _, n := range cr.ruleNums {
		if len(n) > len(number) && strings.HasPrefix(n, number) && isAllLetters(n[len(number):]) {
			b.WriteString("\n\n")
			b.WriteString(cr.rules[n])
		}
	}
	return b.String(), true
}

// Search returns up to limit rules whose text contains keyword (case-insensitive), in document order.
func (cr *Rules) Search(keyword string, limit int) []Match {
	if limit <= 0 {
		limit = SearchDefaultLimit
	}
	kw := strings.ToLower(strings.TrimSpace(keyword))
	var matches []Match
	for _, n := range cr.ruleNums {
		if strings.Contains(strings.ToLower(cr.rules[n]), kw) {
			matches = append(matches, Match{Number: n, Text: cr.rules[n]})
			if len(matches) >= limit {
				break
			}
		}
	}
	return matches
}

// GlossaryTerm returns a glossary definition by term (case-insensitive).
func (cr *Rules) GlossaryTerm(term string) (string, bool) {
	def, ok := cr.glossary[strings.ToLower(strings.TrimSpace(term))]
	return def, ok
}

// isAllLetters reports whether s is non-empty and only lowercase a-z (a subrule suffix).
func isAllLetters(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// Get fetches and parses the current official comprehensive rules.
func Get(ctx context.Context) (*Rules, error) {
	return getWithURL(ctx, rulesPageURL)
}

// getWithURL fetches the rules using a custom page URL (used for testing).
func getWithURL(ctx context.Context, pageURL string) (*Rules, error) {
	page, err := rulesHTTPGet(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rules page: %w", err)
	}

	link := rulesTxtLinkPattern.FindString(page)
	if link == "" {
		return nil, fmt.Errorf("could not find comprehensive rules .txt link on %s", pageURL)
	}

	// Only follow a link that stays on the same host/domain as the page we scraped, so a
	// compromised or tampered page cannot redirect us to fetch arbitrary remote content.
	if !rulesLinkHostAllowed(pageURL, link) {
		return nil, fmt.Errorf("discovered rules link points to an untrusted host: %s", link)
	}

	encodedLink := strings.ReplaceAll(link, " ", "%20")
	txt, err := rulesHTTPGet(ctx, encodedLink)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rules text: %w", err)
	}

	cr := Parse(txt)
	if len(cr.rules) == 0 {
		return nil, fmt.Errorf("fetched rules text contained no parseable rules (%d bytes)", len(txt))
	}
	cr.SourceURL = encodedLink
	return cr, nil
}

// rulesLinkHostAllowed reports whether link may be fetched given the page it was discovered on:
// the link host must equal the page host or share its registrable (last-two-label) domain.
func rulesLinkHostAllowed(pageURL, link string) bool {
	page, pErr := url.Parse(pageURL)
	target, tErr := url.Parse(link)
	if pErr != nil || tErr != nil {
		return false
	}
	pageHost, linkHost := page.Hostname(), target.Hostname()
	if pageHost == "" || linkHost == "" {
		return false
	}
	if linkHost == pageHost {
		return true
	}
	base := registrableSuffix(pageHost)
	return linkHost == base || strings.HasSuffix(linkHost, "."+base)
}

// registrableLabels is the number of trailing dot-separated labels treated as the registrable
// domain (e.g. the "wizards.com" in "magic.wizards.com").
const registrableLabels = 2

// registrableSuffix returns the last registrableLabels labels of host (e.g. "wizards.com"
// for "magic.wizards.com"), or host itself if it has fewer labels.
func registrableSuffix(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < registrableLabels {
		return host
	}
	return strings.Join(parts[len(parts)-registrableLabels:], ".")
}

// rulesHTTPGet performs a GET and returns the body as a string. It asks for text or HTML
// rather than JSON, so it cannot use httpx.GetJSON, and it builds its own client because
// the rules are fetched only when the daily cache expires, so pooling connections would
// save nothing.
func rulesHTTPGet(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "text/plain, text/html")

	client := &http.Client{Timeout: rulesHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &httpx.StatusError{Source: "rules source", StatusCode: resp.StatusCode}
	}

	// Read at most rulesMaxBodyBytes; reading one extra byte lets us detect (rather than
	// silently truncate) an oversized response.
	body, err := io.ReadAll(io.LimitReader(resp.Body, rulesMaxBodyBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > rulesMaxBodyBytes {
		return "", fmt.Errorf("rules source response exceeded %d bytes", rulesMaxBodyBytes)
	}
	return string(body), nil
}

// Cache holds the parsed comprehensive rules with a fetch timestamp.
type Cache struct {
	mu        sync.Mutex
	rules     *Rules
	fetchedAt time.Time
}

// NewSeededCache returns a cache already holding cr, fresh as of now, so lookups are
// served without a fetch until the TTL expires.
func NewSeededCache(cr *Rules) *Cache {
	return &Cache{rules: cr, fetchedAt: time.Now()}
}

// Get returns the cached rules, fetching from the official site when needed.
func (c *Cache) Get(ctx context.Context) (*Rules, error) {
	return c.getFromURL(ctx, rulesPageURL)
}

// getFromURL serves the cache (refetching past the TTL); on fetch failure it serves the stale
// cache if present, otherwise returns the error.
func (c *Cache) getFromURL(ctx context.Context, pageURL string) (*Rules, error) {
	// Serve a fresh cache under the lock, but never hold the lock across the network fetch —
	// a slow upstream must not block other callers (or wedge the feature indefinitely).
	c.mu.Lock()
	if c.rules != nil && time.Since(c.fetchedAt) < rulesCacheTTL {
		cached := c.rules
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	fetched, err := getWithURL(ctx, pageURL)
	if err != nil {
		c.mu.Lock()
		stale := c.rules
		c.mu.Unlock()
		if stale != nil {
			logging.Get().Warn().Err(err).Msg("using stale comprehensive rules cache")
			return stale, nil
		}
		return nil, err
	}

	c.mu.Lock()
	c.rules = fetched
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return fetched, nil
}

// FormatRule renders a single rule (with any subrules) as text.
func FormatRule(number, text string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Rule %s\n\n%s\n", number, text)
	return b.String()
}

// FormatRuleSearch renders search results as a one-line-per-rule list.
func FormatRuleSearch(keyword string, matches []Match) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Rules matching %q (%d)\n\n", keyword, len(matches))
	if len(matches) == 0 {
		b.WriteString("No matching rules found.\n")
		return b.String()
	}
	for _, m := range matches {
		first := m.Text
		if i := strings.IndexByte(first, '\n'); i != -1 {
			first = first[:i]
		}
		fmt.Fprintf(&b, "- %s\n", first)
	}
	return b.String()
}

// FormatGlossaryTerm renders a glossary definition as text.
func FormatGlossaryTerm(term, def string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n", term, def)
	return b.String()
}
