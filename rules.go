package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	rulesPageURL            = "https://magic.wizards.com/en/rules"
	rulesCacheTTL           = 24 * time.Hour
	rulesSearchDefaultLimit = 10
	rulesSearchMaxLimit     = 20
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

// ComprehensiveRules holds the parsed Magic: The Gathering comprehensive rules.
type ComprehensiveRules struct {
	SourceURL string            // the .txt URL the rules were fetched from
	Raw       string            // full original text (served by the rules://comprehensive resource)
	ruleNums  []string          // rule numbers in document order
	rules     map[string]string // rule number -> full text of that rule/subrule
	glossary  map[string]string // lowercased term -> definition
}

// parseComprehensiveRules parses the raw rules .txt into numbered rules and a glossary.
func parseComprehensiveRules(raw string) *ComprehensiveRules {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	cr := &ComprehensiveRules{
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
func parseNumberedRules(cr *ComprehensiveRules, lines []string) {
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
func parseGlossary(cr *ComprehensiveRules, lines []string) {
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

// RuleMatch is a single result from Search.
type RuleMatch struct {
	Number string
	Text   string
}

// Rule returns a rule's text. A parent number (e.g. "702.19") includes its subrules;
// a subrule number (e.g. "702.19a") returns just that subrule.
func (cr *ComprehensiveRules) Rule(number string) (string, bool) {
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
func (cr *ComprehensiveRules) Search(keyword string, limit int) []RuleMatch {
	if limit <= 0 {
		limit = rulesSearchDefaultLimit
	}
	kw := strings.ToLower(strings.TrimSpace(keyword))
	var matches []RuleMatch
	for _, n := range cr.ruleNums {
		if strings.Contains(strings.ToLower(cr.rules[n]), kw) {
			matches = append(matches, RuleMatch{Number: n, Text: cr.rules[n]})
			if len(matches) >= limit {
				break
			}
		}
	}
	return matches
}

// GlossaryTerm returns a glossary definition by term (case-insensitive).
func (cr *ComprehensiveRules) GlossaryTerm(term string) (string, bool) {
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

// GetComprehensiveRules fetches and parses the current official comprehensive rules.
func GetComprehensiveRules(ctx context.Context) (*ComprehensiveRules, error) {
	return getComprehensiveRulesWithURL(ctx, rulesPageURL)
}

// getComprehensiveRulesWithURL fetches the rules using a custom page URL (used for testing).
func getComprehensiveRulesWithURL(ctx context.Context, pageURL string) (*ComprehensiveRules, error) {
	page, err := rulesHTTPGet(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rules page: %w", err)
	}

	link := rulesTxtLinkPattern.FindString(page)
	if link == "" {
		return nil, fmt.Errorf("could not find comprehensive rules .txt link on %s", pageURL)
	}

	encodedLink := strings.ReplaceAll(link, " ", "%20")
	txt, err := rulesHTTPGet(ctx, encodedLink)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rules text: %w", err)
	}

	cr := parseComprehensiveRules(txt)
	cr.SourceURL = encodedLink
	return cr, nil
}

// rulesHTTPGet performs a GET and returns the body as a string (plain client, like archidekt.go).
func rulesHTTPGet(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "MTG-Commander-MCP-Server/1.0")
	req.Header.Set("Accept", "text/plain, text/html")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("rules source returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// rulesCache holds the parsed comprehensive rules with a fetch timestamp.
type rulesCache struct {
	mu        sync.Mutex
	rules     *ComprehensiveRules
	fetchedAt time.Time
}

// comprehensiveRules returns the cached rules, fetching from the official site when needed.
func (s *MTGCommanderServer) comprehensiveRules(ctx context.Context) (*ComprehensiveRules, error) {
	return s.comprehensiveRulesFromURL(ctx, rulesPageURL)
}

// comprehensiveRulesFromURL serves the cache (refetching past the TTL); on fetch failure it serves
// the stale cache if present, otherwise returns the error.
func (s *MTGCommanderServer) comprehensiveRulesFromURL(
	ctx context.Context,
	pageURL string,
) (*ComprehensiveRules, error) {
	s.rules.mu.Lock()
	defer s.rules.mu.Unlock()

	if s.rules.rules != nil && time.Since(s.rules.fetchedAt) < rulesCacheTTL {
		return s.rules.rules, nil
	}

	fetched, err := getComprehensiveRulesWithURL(ctx, pageURL)
	if err != nil {
		if s.rules.rules != nil {
			GetLogger().Warn().Err(err).Msg("using stale comprehensive rules cache")
			return s.rules.rules, nil
		}
		return nil, err
	}

	s.rules.rules = fetched
	s.rules.fetchedAt = time.Now()
	return s.rules.rules, nil
}

// FormatRuleForDisplay renders a single rule (with any subrules) as text.
func FormatRuleForDisplay(number, text string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Rule %s\n\n%s\n", number, text)
	return b.String()
}

// FormatRuleSearchForDisplay renders search results as a one-line-per-rule list.
func FormatRuleSearchForDisplay(keyword string, matches []RuleMatch) string {
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

// FormatGlossaryTermForDisplay renders a glossary definition as text.
func FormatGlossaryTermForDisplay(term, def string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n", term, def)
	return b.String()
}
