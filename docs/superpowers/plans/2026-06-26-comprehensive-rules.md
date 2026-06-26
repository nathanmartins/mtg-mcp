# Comprehensive Rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add autonomous access to the official WotC Comprehensive Rules via a `rules://comprehensive` resource plus `get_rule` / `search_rules` / `get_glossary_term` tools.

**Architecture:** New `rules.go` holds fetch (runtime, scrape page → discover dated `.txt` → download), a pure parser into numbered rules + glossary, lookup methods, and formatters. The server caches the parsed rules in-memory with a 24h TTL. Handlers + registration live in `main.go`, mirroring the Archidekt integration.

**Tech Stack:** Go 1.26.4, `github.com/mark3labs/mcp-go`, `github.com/rs/zerolog`. Standard library only for HTTP/regex/parsing.

## Global Constraints

- Go 1.26.4; standard library only (no new module dependencies).
- Follow the repo creator's patterns: `GetX`/`getXWithURL` split (tests inject an `httptest` URL); one file per concern (`rules.go`); handlers are `func (s *MTGCommanderServer) handleX(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)` in `main.go`; `handler → fetch → Format…ForDisplay` pipeline; build a plain `&http.Client{}` in fetchers (NOT the HTTPS-only `HTTPGet`) so http test servers work; diagnostics through `GetLogger()`, never stdout; package-level vars carry `//nolint:gochecknoglobals` with a reason.
- golangci-lint v2 limits: `funlen` 200 lines / 70 statements, `golines` max-len 120, plus `cyclop`, `exhaustive`, `goimports`. Run `make check` (or `golangci-lint run ./...`) — must be 0 issues.
- Bump `totalToolCount` (14 → 17) and `totalResourceCount` (2 → 3) in `main.go`.
- Unit tests are `-short`-safe and httptest-backed; E2E tests are named `Test…E2E` and `t.Skip()` when `testing.Short()`.
- PATH for all `go`/lint commands: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"`.

---

## File Structure

- **Create `rules.go`** — constants, `ComprehensiveRules` + `RuleMatch` types, `rulesCache` type, parser (`parseComprehensiveRules`, `parseNumberedRules`, `parseGlossary`, `isAllLetters`), lookups (`Rule`, `Search`, `GlossaryTerm`), fetch (`GetComprehensiveRules`, `getComprehensiveRulesWithURL`, `rulesHTTPGet`), cache methods (`comprehensiveRules`, `comprehensiveRulesFromURL`), formatters (`FormatRuleForDisplay`, `FormatRuleSearchForDisplay`, `FormatGlossaryTermForDisplay`).
- **Modify `main.go`** — add `rules *rulesCache` field to `MTGCommanderServer`; init it in `NewMTGCommanderServer`; bump the two count constants; add `registerRulesTools` + call it from `registerTools`; add the resource in `registerResources`; add 4 handlers.
- **Create `rules_test.go`** — unit tests (parser, lookups, fetch via httptest, cache, formatters, handlers).
- **Create `rules_e2e_test.go`** — real-WotC E2E test.

---

### Task 1: Types, constants, and parser

**Files:**
- Create: `rules.go`
- Test: `rules_test.go`

**Interfaces:**
- Produces: `type ComprehensiveRules struct{ SourceURL, Raw string; ruleNums []string; rules, glossary map[string]string }`; `func parseComprehensiveRules(raw string) *ComprehensiveRules`.

- [ ] **Step 1: Write the failing test**

Create `rules_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

const sampleRulesTxt = `Magic: The Gathering Comprehensive Rules

These rules are effective as of June 19, 2026.

100. General

100.1. These Magic rules apply to any Magic game with two or more players.

702.19. Trample

702.19a Trample is a static ability that modifies the rules for assigning combat damage.

702.19b The controller of an attacking creature with trample first assigns damage to the creatures blocking it.

Glossary

Trample
A keyword ability that allows excess combat damage to be assigned to the defending player. See rule 702.19.

Deathtouch
A keyword ability that causes damage dealt by an object to be lethal. See rule 702.2.
`

func TestParseComprehensiveRules(t *testing.T) {
	cr := parseComprehensiveRules(sampleRulesTxt)

	for _, num := range []string{"100.1", "702.19", "702.19a", "702.19b"} {
		if _, ok := cr.rules[num]; !ok {
			t.Errorf("expected rule %q to be parsed", num)
		}
	}
	if !strings.Contains(cr.rules["702.19a"], "static ability") {
		t.Errorf("rule 702.19a text wrong: %q", cr.rules["702.19a"])
	}
	if _, ok := cr.glossary["trample"]; !ok {
		t.Error("expected glossary term 'trample'")
	}
	if _, ok := cr.glossary["deathtouch"]; !ok {
		t.Error("expected glossary term 'deathtouch'")
	}
	if cr.Raw != strings.ReplaceAll(sampleRulesTxt, "\r\n", "\n") {
		t.Error("Raw should hold the full normalized text")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestParseComprehensiveRules .`
Expected: FAIL — `undefined: parseComprehensiveRules`.

- [ ] **Step 3: Write minimal implementation**

Create `rules.go`:

```go
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
var rulesTxtLinkPattern = regexp.MustCompile(`https?://[^"\s]*MagicCompRules[^"]*?\.txt`) //nolint:gochecknoglobals // compiled regex reused across calls

// ruleNumberPattern matches a rule/subrule number at the start of a line (e.g. "702.19." or "702.19a").
var ruleNumberPattern = regexp.MustCompile(`^(\d+\.\d+[a-z]*)\.?\s`) //nolint:gochecknoglobals // compiled regex reused across calls

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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestParseComprehensiveRules .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rules.go rules_test.go
git commit -m "feat: parse WotC comprehensive rules text into rules + glossary"
```

---

### Task 2: Lookup methods

**Files:**
- Modify: `rules.go`
- Test: `rules_test.go`

**Interfaces:**
- Consumes: `*ComprehensiveRules` from Task 1.
- Produces: `type RuleMatch struct{ Number, Text string }`; `func (cr *ComprehensiveRules) Rule(number string) (string, bool)`; `func (cr *ComprehensiveRules) Search(keyword string, limit int) []RuleMatch`; `func (cr *ComprehensiveRules) GlossaryTerm(term string) (string, bool)`.

- [ ] **Step 1: Write the failing test**

Append to `rules_test.go`:

```go
func TestComprehensiveRulesLookups(t *testing.T) {
	cr := parseComprehensiveRules(sampleRulesTxt)

	// Rule with subrules
	got, ok := cr.Rule("702.19")
	if !ok {
		t.Fatal("expected rule 702.19")
	}
	if !strings.Contains(got, "Trample") || !strings.Contains(got, "static ability") ||
		!strings.Contains(got, "first assigns damage") {
		t.Errorf("Rule(702.19) should include parent + subrules; got:\n%s", got)
	}

	// Subrule alone
	sub, ok := cr.Rule("702.19a")
	if !ok || strings.Contains(sub, "first assigns damage") {
		t.Errorf("Rule(702.19a) should return only that subrule; got:\n%s", sub)
	}

	// Unknown rule
	if _, ok := cr.Rule("999.99"); ok {
		t.Error("unknown rule should return ok=false")
	}

	// Search
	matches := cr.Search("trample", 0)
	if len(matches) == 0 || matches[0].Number != "702.19" {
		t.Errorf("Search(trample) first match should be 702.19; got %+v", matches)
	}
	if len(cr.Search("trample", 1)) != 1 {
		t.Error("Search limit should cap results")
	}

	// Glossary (case-insensitive)
	def, ok := cr.GlossaryTerm("Trample")
	if !ok || !strings.Contains(def, "keyword ability") {
		t.Errorf("GlossaryTerm(Trample) wrong; ok=%v def=%q", ok, def)
	}
	if _, ok := cr.GlossaryTerm("nope"); ok {
		t.Error("unknown glossary term should return ok=false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestComprehensiveRulesLookups .`
Expected: FAIL — `cr.Rule undefined`.

- [ ] **Step 3: Write minimal implementation**

Append to `rules.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestComprehensiveRulesLookups .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rules.go rules_test.go
git commit -m "feat: add rule/search/glossary lookups on comprehensive rules"
```

---

### Task 3: Runtime fetch + link discovery

**Files:**
- Modify: `rules.go`
- Test: `rules_test.go`

**Interfaces:**
- Consumes: `parseComprehensiveRules` (Task 1).
- Produces: `func GetComprehensiveRules(ctx context.Context) (*ComprehensiveRules, error)`; `func getComprehensiveRulesWithURL(ctx context.Context, pageURL string) (*ComprehensiveRules, error)`; `func rulesHTTPGet(ctx context.Context, rawURL string) (string, error)`.

- [ ] **Step 1: Write the failing test**

Append to `rules_test.go` (add imports `context`, `fmt`, `net/http`, `net/http/httptest` to the file's import block):

```go
func TestGetComprehensiveRulesWithURL(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".txt") {
			fmt.Fprint(w, sampleRulesTxt)
			return
		}
		// Page embeds a link to the .txt with a space in the filename, like the real WotC page.
		fmt.Fprintf(w, `<html><body><a href="%s/MagicCompRules 20260619.txt">rules</a></body></html>`, server.URL)
	}))
	defer server.Close()

	cr, err := getComprehensiveRulesWithURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("getComprehensiveRulesWithURL: %v", err)
	}
	if _, ok := cr.Rule("702.19"); !ok {
		t.Error("expected parsed rule 702.19 from fetched text")
	}
	if !strings.Contains(cr.SourceURL, "MagicCompRules") {
		t.Errorf("SourceURL not set correctly: %q", cr.SourceURL)
	}
}

func TestGetComprehensiveRulesWithURL_NoLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body>no rules link here</body></html>`)
	}))
	defer server.Close()

	if _, err := getComprehensiveRulesWithURL(context.Background(), server.URL); err == nil {
		t.Error("expected an error when the rules link is missing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestGetComprehensiveRulesWithURL .`
Expected: FAIL — `getComprehensiveRulesWithURL undefined`.

- [ ] **Step 3: Write minimal implementation**

Append to `rules.go`:

```go
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

	txt, err := rulesHTTPGet(ctx, strings.ReplaceAll(link, " ", "%20"))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rules text: %w", err)
	}

	cr := parseComprehensiveRules(txt)
	cr.SourceURL = link
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestGetComprehensiveRulesWithURL .`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add rules.go rules_test.go
git commit -m "feat: fetch comprehensive rules by discovering the WotC .txt link"
```

---

### Task 4: In-memory cache on the server

**Files:**
- Modify: `rules.go` (cache type + methods), `main.go` (struct field + init)
- Test: `rules_test.go`

**Interfaces:**
- Consumes: `getComprehensiveRulesWithURL` (Task 3).
- Produces: `type rulesCache struct{ mu sync.Mutex; rules *ComprehensiveRules; fetchedAt time.Time }`; `func (s *MTGCommanderServer) comprehensiveRules(ctx context.Context) (*ComprehensiveRules, error)`; `func (s *MTGCommanderServer) comprehensiveRulesFromURL(ctx context.Context, pageURL string) (*ComprehensiveRules, error)`. `MTGCommanderServer` gains a `rules *rulesCache` field.

- [ ] **Step 1: Write the failing test**

Append to `rules_test.go`:

```go
func TestComprehensiveRulesCache(t *testing.T) {
	var pageHits int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".txt") {
			fmt.Fprint(w, sampleRulesTxt)
			return
		}
		pageHits++
		fmt.Fprintf(w, `<html><a href="%s/MagicCompRules 20260619.txt">r</a></html>`, server.URL)
	}))
	defer server.Close()

	srv := &MTGCommanderServer{rules: &rulesCache{}}

	if _, err := srv.comprehensiveRulesFromURL(context.Background(), server.URL); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if _, err := srv.comprehensiveRulesFromURL(context.Background(), server.URL); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if pageHits != 1 {
		t.Errorf("expected 1 page fetch (cached), got %d", pageHits)
	}
}

func TestComprehensiveRulesCache_StaleOnError(t *testing.T) {
	srv := &MTGCommanderServer{rules: &rulesCache{
		rules:     parseComprehensiveRules(sampleRulesTxt),
		fetchedAt: time.Now().Add(-2 * rulesCacheTTL), // expired
	}}

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	cr, err := srv.comprehensiveRulesFromURL(context.Background(), dead.URL)
	if err != nil {
		t.Fatalf("should serve stale cache, got error: %v", err)
	}
	if _, ok := cr.Rule("702.19"); !ok {
		t.Error("stale cache should still answer lookups")
	}
}
```

(Add `time` to the test file's import block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestComprehensiveRulesCache .`
Expected: FAIL — `srv.rules undefined` / `comprehensiveRulesFromURL undefined`.

- [ ] **Step 3: Write minimal implementation**

In `rules.go`, append:

```go
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
```

In `main.go`, add the field to the `MTGCommanderServer` struct (after `scryfallClient`):

```go
	rules          *rulesCache
```

And initialize it in `NewMTGCommanderServer` — change the return to:

```go
	return &MTGCommanderServer{
		scryfallClient: client,
		rules:          &rulesCache{},
	}, nil
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestComprehensiveRulesCache .`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add rules.go main.go rules_test.go
git commit -m "feat: cache comprehensive rules in-memory with TTL and stale-on-error"
```

---

### Task 5: Display formatters

**Files:**
- Modify: `rules.go`
- Test: `rules_test.go`

**Interfaces:**
- Consumes: `RuleMatch` (Task 2).
- Produces: `func FormatRuleForDisplay(number, text string) string`; `func FormatRuleSearchForDisplay(keyword string, matches []RuleMatch) string`; `func FormatGlossaryTermForDisplay(term, def string) string`.

- [ ] **Step 1: Write the failing test**

Append to `rules_test.go`:

```go
func TestRulesFormatters(t *testing.T) {
	r := FormatRuleForDisplay("702.19", "702.19. Trample")
	if !strings.Contains(r, "# Rule 702.19") || !strings.Contains(r, "Trample") {
		t.Errorf("FormatRuleForDisplay wrong:\n%s", r)
	}

	empty := FormatRuleSearchForDisplay("xyzzy", nil)
	if !strings.Contains(empty, "No matching rules") {
		t.Errorf("empty search should say so:\n%s", empty)
	}

	s := FormatRuleSearchForDisplay("trample", []RuleMatch{{Number: "702.19", Text: "702.19. Trample\nmore"}})
	if !strings.Contains(s, "702.19") || !strings.Contains(s, "trample") {
		t.Errorf("search format wrong:\n%s", s)
	}
	if strings.Contains(s, "\nmore") {
		t.Error("search listing should show only the first line of each match")
	}

	g := FormatGlossaryTermForDisplay("Trample", "A keyword ability.")
	if !strings.Contains(g, "# Trample") || !strings.Contains(g, "keyword ability") {
		t.Errorf("glossary format wrong:\n%s", g)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestRulesFormatters .`
Expected: FAIL — `FormatRuleForDisplay undefined`.

- [ ] **Step 3: Write minimal implementation**

Append to `rules.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestRulesFormatters .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rules.go rules_test.go
git commit -m "feat: add display formatters for rules and glossary"
```

---

### Task 6: Handlers, registration, and constants (main.go)

**Files:**
- Modify: `main.go`
- Test: `rules_test.go`

**Interfaces:**
- Consumes: `s.comprehensiveRules` (Task 4), lookups (Task 2), formatters (Task 5).
- Produces: resource `rules://comprehensive`; tools `get_rule`, `search_rules`, `get_glossary_term`; constants `totalToolCount = 17`, `totalResourceCount = 3`.

- [ ] **Step 1: Write the failing test**

Append to `rules_test.go`:

```go
func TestRulesCountsAndHandlers(t *testing.T) {
	if totalToolCount != 17 {
		t.Errorf("totalToolCount = %d, want 17", totalToolCount)
	}
	if totalResourceCount != 3 {
		t.Errorf("totalResourceCount = %d, want 3", totalResourceCount)
	}

	// Pre-populate the cache so handlers do no network.
	srv := &MTGCommanderServer{rules: &rulesCache{
		rules:     parseComprehensiveRules(sampleRulesTxt),
		fetchedAt: time.Now(),
	}}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"rule": "702.19"}
	res, err := srv.handleGetRule(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetRule: %v", err)
	}
	if !strings.Contains(toolResultText(t, res), "Trample") {
		t.Error("handleGetRule should return rule 702.19 text")
	}

	greq := mcp.CallToolRequest{}
	greq.Params.Arguments = map[string]any{"term": "Trample"}
	gres, err := srv.handleGetGlossaryTerm(context.Background(), greq)
	if err != nil {
		t.Fatalf("handleGetGlossaryTerm: %v", err)
	}
	if !strings.Contains(toolResultText(t, gres), "keyword ability") {
		t.Error("handleGetGlossaryTerm should return the definition")
	}
}

// toolResultText extracts the text payload from a tool result.
func toolResultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
```

(Add `"github.com/mark3labs/mcp-go/mcp"` to the test file's import block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestRulesCountsAndHandlers .`
Expected: FAIL — `totalToolCount` is 14 / `handleGetRule undefined`.

- [ ] **Step 3: Write minimal implementation**

In `main.go`, bump the constants:

```go
	totalToolCount     = 17
	totalResourceCount = 3
```

Add a resource in `registerResources` (after the banned-list resource, before the closing brace):

```go
	// Resource 3: Comprehensive Rules (full text, fetched from WotC)
	comprehensiveResource := mcp.NewResource(
		"rules://comprehensive",
		"Magic Comprehensive Rules",
		mcp.WithResourceDescription("Full official Magic: The Gathering Comprehensive Rules (large)"),
		mcp.WithMIMEType("text/plain"),
	)
	mcpServer.AddResource(comprehensiveResource, s.handleComprehensiveRulesResource)
```

Add a `registerRulesTools` call inside `registerTools` (next to the existing `s.registerArchidektTools(mcpServer)` line):

```go
	s.registerRulesTools(mcpServer)
```

Add the new registration method (next to `registerArchidektTools`):

```go
// registerRulesTools registers Comprehensive Rules lookup tools.
func (s *MTGCommanderServer) registerRulesTools(mcpServer *server.MCPServer) {
	getRuleTool := mcp.NewTool(
		"get_rule",
		mcp.WithDescription("Get a specific Comprehensive Rule by number (e.g. '702.19'); includes its subrules"),
		mcp.WithString("rule",
			mcp.Required(),
			mcp.Description("Rule number, e.g. '702.19' or a subrule like '702.19a'"),
		),
	)
	mcpServer.AddTool(getRuleTool, s.handleGetRule)

	searchRulesTool := mcp.NewTool(
		"search_rules",
		mcp.WithDescription("Search the Comprehensive Rules text by keyword"),
		mcp.WithString("keyword",
			mcp.Required(),
			mcp.Description("Keyword or phrase to search for"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default: 10, max: 20)"),
		),
	)
	mcpServer.AddTool(searchRulesTool, s.handleSearchRules)

	glossaryTool := mcp.NewTool(
		"get_glossary_term",
		mcp.WithDescription("Get a Comprehensive Rules glossary definition by term"),
		mcp.WithString("term",
			mcp.Required(),
			mcp.Description("Glossary term, e.g. 'Trample'"),
		),
	)
	mcpServer.AddTool(glossaryTool, s.handleGetGlossaryTerm)
}
```

Add the four handlers (near the other resource/tool handlers):

```go
func (s *MTGCommanderServer) handleComprehensiveRulesResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load comprehensive rules: %w", err)
	}
	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     rules.Raw,
		},
	}, nil
}

func (s *MTGCommanderServer) handleGetRule(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	number, err := request.RequireString("rule")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	text, ok := rules.Rule(number)
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("Rule %s not found.", number)), nil
	}
	return mcp.NewToolResultText(FormatRuleForDisplay(number, text)), nil
}

func (s *MTGCommanderServer) handleSearchRules(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	keyword, err := request.RequireString("keyword")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := rulesSearchDefaultLimit
	if limitVal, ok := request.GetArguments()["limit"]; ok {
		if limitFloat, isNum := limitVal.(float64); isNum && limitFloat >= 1 {
			limit = int(limitFloat)
		}
	}
	if limit > rulesSearchMaxLimit {
		limit = rulesSearchMaxLimit
	}
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	return mcp.NewToolResultText(FormatRuleSearchForDisplay(keyword, rules.Search(keyword, limit))), nil
}

func (s *MTGCommanderServer) handleGetGlossaryTerm(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	term, err := request.RequireString("term")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	def, ok := rules.GlossaryTerm(term)
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("Glossary term %q not found.", term)), nil
	}
	return mcp.NewToolResultText(FormatGlossaryTermForDisplay(term, def)), nil
}
```

- [ ] **Step 4: Run tests + lint**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -count=1 ./... && golangci-lint run ./...`
Expected: tests `ok`, lint `0 issues`. If `golines` flags a long line, run `golangci-lint fmt` and re-run.

- [ ] **Step 5: Commit**

```bash
git add main.go rules_test.go
git commit -m "feat: expose comprehensive rules resource + get_rule/search_rules/get_glossary_term tools"
```

---

### Task 7: E2E test against the real WotC site

**Files:**
- Create: `rules_e2e_test.go`

**Interfaces:**
- Consumes: `GetComprehensiveRules` (Task 3).

- [ ] **Step 1: Write the test**

Create `rules_e2e_test.go`:

```go
package main

import (
	"context"
	"strings"
	"testing"
)

func TestComprehensiveRulesE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	rules, err := GetComprehensiveRules(context.Background())
	if err != nil {
		t.Fatalf("failed to fetch comprehensive rules: %v", err)
	}

	text, ok := rules.Rule("702.19")
	if !ok || !strings.Contains(text, "Trample") {
		t.Errorf("expected rule 702.19 to mention Trample; ok=%v", ok)
	}
	if _, ok := rules.GlossaryTerm("Trample"); !ok {
		t.Error("expected glossary to contain 'Trample'")
	}
	if !strings.Contains(rules.SourceURL, "MagicCompRules") {
		t.Errorf("unexpected SourceURL: %q", rules.SourceURL)
	}
}
```

- [ ] **Step 2: Run the E2E test (real network)**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -run TestComprehensiveRulesE2E -v .`
Expected: PASS (fetches the live rules; ~1s). If WotC is unreachable, it fails — that's expected for an E2E test.

- [ ] **Step 3: Verify it is skipped in short mode**

Run: `export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"; go test -short -run TestComprehensiveRulesE2E -v .`
Expected: `SKIP`.

- [ ] **Step 4: Commit**

```bash
git add rules_e2e_test.go
git commit -m "test: add E2E test for comprehensive rules fetch"
```

---

## Post-implementation (separate steps, after all tasks)

- Run `make check` (fmt + lint + unit tests) — must pass.
- Update `README.md` tool/resource counts and add the new tools/resource to the feature list.
- Add **ADR-007** (comprehensive rules resource/tools) and re-index codebase-memory + Serena.
- Push branch `feat/comprehensive-rules` and open a PR.

## Self-review notes

- **Spec coverage:** resource (Task 6) ✓; get_rule/search_rules/get_glossary_term (Tasks 2/5/6) ✓; runtime fetch + discovery (Task 3) ✓; cache + TTL + stale-on-error (Task 4) ✓; parser incl. glossary (Task 1) ✓; default/max search limit (Task 6 handler) ✓; unit + E2E tests (all tasks / Task 7) ✓; constants bump (Task 6) ✓.
- **Types consistent across tasks:** `ComprehensiveRules`, `RuleMatch{Number,Text}`, `rulesCache{mu,rules,fetchedAt}`, method names `Rule`/`Search`/`GlossaryTerm`/`comprehensiveRules`/`comprehensiveRulesFromURL`, formatters `FormatRuleForDisplay`/`FormatRuleSearchForDisplay`/`FormatGlossaryTermForDisplay` — used identically wherever referenced.
- **No placeholders:** every code/test step contains full code.
