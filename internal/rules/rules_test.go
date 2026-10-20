package rules

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nathanmartins/mtg-mcp/internal/httpx"
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

func TestParse(t *testing.T) {
	t.Parallel()
	cr := Parse(sampleRulesTxt)

	for _, num := range []string{"100.1", testTrampleRule, "702.19a", "702.19b"} {
		if _, ok := cr.rules[num]; !ok {
			t.Errorf("expected rule %q to be parsed", num)
		}
	}
	if _, ok := cr.rules["100"]; ok {
		t.Error("section header '100. General' should not be parsed as a rule")
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

func TestRulesLookups(t *testing.T) {
	t.Parallel()
	cr := Parse(sampleRulesTxt)

	// Rule with subrules
	got, ok := cr.Rule(testTrampleRule)
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
	if _, found := cr.Rule("999.99"); found {
		t.Error("unknown rule should return ok=false")
	}

	// Search
	matches := cr.Search("trample", 0)
	if len(matches) == 0 || matches[0].Number != testTrampleRule {
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
	if _, found := cr.GlossaryTerm("nope"); found {
		t.Error("unknown glossary term should return ok=false")
	}
}

func TestGetWithURL(t *testing.T) {
	t.Parallel()
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

	cr, err := getWithURL(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("getWithURL: %v", err)
	}
	if _, ok := cr.Rule(testTrampleRule); !ok {
		t.Error("expected parsed rule 702.19 from fetched text")
	}
	if !strings.Contains(cr.SourceURL, "MagicCompRules") {
		t.Errorf("SourceURL not set correctly: %q", cr.SourceURL)
	}
}

func TestGetWithURL_NoLink(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body>no rules link here</body></html>`)
	}))
	defer server.Close()

	if _, err := getWithURL(t.Context(), server.URL); err == nil {
		t.Error("expected an error when the rules link is missing")
	}
}

func TestCache(t *testing.T) {
	t.Parallel()
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

	cache := &Cache{}

	r1, err := cache.getFromURL(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	r2, err := cache.getFromURL(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if r1 == nil || r1 != r2 {
		t.Errorf("cache hit should return the same non-nil rules; r1=%p r2=%p", r1, r2)
	}
	if pageHits != 1 {
		t.Errorf("expected 1 page fetch (cached), got %d", pageHits)
	}
}

func TestCache_StaleOnError(t *testing.T) {
	t.Parallel()
	cache := &Cache{
		rules:     Parse(sampleRulesTxt),
		fetchedAt: time.Now().Add(-2 * rulesCacheTTL), // expired
	}

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	cr, err := cache.getFromURL(t.Context(), dead.URL)
	if err != nil {
		t.Fatalf("should serve stale cache, got error: %v", err)
	}
	if _, ok := cr.Rule(testTrampleRule); !ok {
		t.Error("stale cache should still answer lookups")
	}
}

func TestRulesFormatters(t *testing.T) {
	t.Parallel()
	r := FormatRule(testTrampleRule, "702.19. Trample")
	if !strings.Contains(r, "# Rule 702.19") || !strings.Contains(r, "Trample") {
		t.Errorf("FormatRule wrong:\n%s", r)
	}

	empty := FormatRuleSearch("xyzzy", nil)
	if !strings.Contains(empty, "No matching rules") {
		t.Errorf("empty search should say so:\n%s", empty)
	}

	s := FormatRuleSearch("trample", []Match{{Number: testTrampleRule, Text: "702.19. Trample\nmore"}})
	if !strings.Contains(s, "- 702.19") || !strings.Contains(s, "trample") {
		t.Errorf("search format wrong:\n%s", s)
	}
	if strings.Contains(s, "\nmore") {
		t.Error("search listing should show only the first line of each match")
	}

	g := FormatGlossaryTerm("Trample", "A keyword ability.")
	if !strings.Contains(g, "# Trample") || !strings.Contains(g, "keyword ability") {
		t.Errorf("glossary format wrong:\n%s", g)
	}
}

func TestRulesHTTPGetRejectsOversizedBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// One byte over the cap.
		fmt.Fprint(w, strings.Repeat("a", rulesMaxBodyBytes+1))
	}))
	defer server.Close()

	if _, err := rulesHTTPGet(t.Context(), server.URL); err == nil {
		t.Errorf("expected an error when the response exceeds %d bytes", rulesMaxBodyBytes)
	}
}

func TestGetWithURLRejectsForeignHost(t *testing.T) {
	t.Parallel(
	// The page links to a .txt on a host unrelated to the page host — must be rejected, not fetched.
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><a href="http://evil.example/MagicCompRules 20260619.txt">r</a></html>`)
	}))
	defer server.Close()

	_, err := getWithURL(t.Context(), server.URL)
	if err == nil {
		t.Fatal("expected an error for a rules link on an untrusted host")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("error should mention the untrusted host; got: %v", err)
	}
}

func TestGetWithURLRejectsEmptyParse(t *testing.T) {
	t.Parallel(
	// A 200 response whose body has no parseable rules must be treated as a failure, not cached.
	)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".txt") {
			fmt.Fprint(w, "maintenance page — no rules here")
			return
		}
		fmt.Fprintf(w, `<html><a href="%s/MagicCompRules 20260619.txt">r</a></html>`, server.URL)
	}))
	defer server.Close()

	if _, err := getWithURL(t.Context(), server.URL); err == nil {
		t.Error("expected an error when the fetched text contains no parseable rules")
	}
}

func TestRulesHTTPGetReportsStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := rulesHTTPGet(t.Context(), server.URL)
	if !httpx.HasStatus(err, http.StatusServiceUnavailable) {
		t.Errorf("rulesHTTPGet() error = %v, want a 503 httpx.StatusError", err)
	}
	if err != nil && err.Error() != "rules source returned status 503" {
		t.Errorf("error text = %q", err.Error())
	}
}
