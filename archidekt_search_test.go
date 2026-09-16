package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// syntheticArchidektPage builds a full 60-deck upstream page whose deck names
// encode their absolute position, so slicing bugs are visible in assertions.
func syntheticArchidektPage(apiPage, count int) ArchidektUserDecksResponse {
	results := make([]ArchidektDeckSummary, 0, archidektAPIPageSize)
	base := (apiPage - 1) * archidektAPIPageSize
	for i := range archidektAPIPageSize {
		results = append(results, ArchidektDeckSummary{
			ID:         base + i + 1,
			Name:       fmt.Sprintf("deck-%d", base+i+1),
			DeckFormat: 3,
			ViewCount:  1000 - (base + i),
			UpdatedAt:  "2026-01-01T00:00:00Z",
			Owner:      ArchidektOwner{Username: "owner"},
		})
	}
	next := ""
	if apiPage*archidektAPIPageSize < count {
		next = fmt.Sprintf("http://archidekt.com/api/decks/v3/?page=%d", apiPage+1)
	}
	return ArchidektUserDecksResponse{Count: count, Next: next, Results: results}
}

// archidektPageSlice builds the upstream page apiPage would return for a result set of
// total decks, so short pages and pages past the end behave like the real API.
func archidektPageSlice(total, apiPage int) ArchidektUserDecksResponse {
	start := (apiPage - 1) * archidektAPIPageSize
	end := min(start+archidektAPIPageSize, total)
	results := make([]ArchidektDeckSummary, 0, archidektAPIPageSize)
	for i := start; i < end; i++ {
		results = append(results, ArchidektDeckSummary{
			ID:         i + 1,
			Name:       fmt.Sprintf("deck-%d", i+1),
			DeckFormat: 3,
			UpdatedAt:  "2026-01-01T00:00:00Z",
			Owner:      ArchidektOwner{Username: "owner"},
		})
	}
	next := ""
	if end < total {
		next = fmt.Sprintf("http://archidekt.com/api/decks/v3/?page=%d", apiPage+1)
	}
	return ArchidektUserDecksResponse{Count: total, Next: next, Results: results}
}

func TestSearchArchidektDecksHandlesShortUpstreamPages(t *testing.T) {
	var requestedPages []string
	fixtureServer := func(total int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPages = append(requestedPages, r.URL.Query().Get("page"))
			apiPage := 1
			if raw := r.URL.Query().Get("page"); raw != "" {
				parsed, err := strconv.Atoi(raw)
				if err != nil {
					t.Errorf("unparseable page parameter %q", raw)
				}
				apiPage = parsed
			}
			_ = json.NewEncoder(w).Encode(archidektPageSlice(total, apiPage))
		}))
	}

	t.Run("window inside a short page", func(t *testing.T) {
		requestedPages = nil
		server := fixtureServer(25)
		defer server.Close()

		got, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
			Commander: "Atraxa", Sort: archidektSortViews, Page: 2, Limit: 10,
		}, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Decks) != 10 {
			t.Fatalf("returned %d decks, want 10", len(got.Decks))
		}
		if got.Decks[0].Name != "deck-11" || got.Decks[9].Name != "deck-20" {
			t.Errorf("wrong window: first=%q last=%q", got.Decks[0].Name, got.Decks[9].Name)
		}
		if len(requestedPages) != 1 {
			t.Errorf("expected 1 upstream request, got %d (%v)", len(requestedPages), requestedPages)
		}
	})

	t.Run("page past the end returns no decks", func(t *testing.T) {
		requestedPages = nil
		server := fixtureServer(25)
		defer server.Close()

		got, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
			Commander: "Atraxa", Sort: archidektSortViews, Page: 9, Limit: 10,
		}, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Decks) != 0 {
			t.Errorf("returned %d decks, want 0", len(got.Decks))
		}
		if got.Total != 25 || !got.TotalKnown {
			t.Errorf("total = (%d, known=%v), want (25, known=true)", got.Total, got.TotalKnown)
		}
		if len(requestedPages) != 1 {
			t.Errorf("expected 1 upstream request, got %d (%v)", len(requestedPages), requestedPages)
		}
	})

	t.Run("straddling window stops at a short first page", func(t *testing.T) {
		requestedPages = nil
		server := fixtureServer(55)
		defer server.Close()

		got, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
			Commander: "Atraxa", Sort: archidektSortViews, Page: 2, Limit: 50,
		}, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Decks) != 5 {
			t.Fatalf("returned %d decks, want 5", len(got.Decks))
		}
		if got.Decks[0].Name != "deck-51" || got.Decks[4].Name != "deck-55" {
			t.Errorf("wrong window: first=%q last=%q", got.Decks[0].Name, got.Decks[4].Name)
		}
		if len(requestedPages) != 1 {
			t.Errorf("expected the exhausted upstream to stop after 1 request, got %d (%v)",
				len(requestedPages), requestedPages)
		}
	})
}

func TestArchidektPageWindow(t *testing.T) {
	tests := []struct {
		name                                  string
		page, limit                           int
		wantAPIPage, wantOffset, wantAPIPages int
	}{
		{"first page", 1, 10, 1, 0, 1},
		{"second page fits in first api page", 2, 10, 1, 10, 1},
		{"page aligned to api page boundary", 7, 10, 2, 0, 1},
		{"window straddles two api pages", 2, 50, 1, 50, 2},
		{"window ends exactly on boundary", 6, 10, 1, 50, 1},
		{"full api page", 1, 60, 1, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotOffset, gotPages := archidektPageWindow(tt.page, tt.limit)
			if gotPage != tt.wantAPIPage || gotOffset != tt.wantOffset || gotPages != tt.wantAPIPages {
				t.Errorf("archidektPageWindow(%d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tt.page, tt.limit, gotPage, gotOffset, gotPages,
					tt.wantAPIPage, tt.wantOffset, tt.wantAPIPages)
			}
		})
	}
}

func TestSearchArchidektDecksHonoursLimitAndPage(t *testing.T) {
	var requestedPages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requestedPages = append(requestedPages, q.Get("page"))
		if q.Get("pageSize") != "" || q.Get("limit") != "" {
			t.Errorf("must not send an upstream page-size parameter, got %q", r.URL.RawQuery)
		}
		apiPage := 1
		if q.Get("page") == "2" {
			apiPage = 2
		}
		_ = json.NewEncoder(w).Encode(syntheticArchidektPage(apiPage, 500))
	}))
	defer server.Close()

	t.Run("limit is exact", func(t *testing.T) {
		requestedPages = nil
		got, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
			Commander: "Atraxa, Praetors' Voice", Sort: archidektSortViews, Page: 1, Limit: 10,
		}, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Decks) != 10 {
			t.Fatalf("returned %d decks, want 10", len(got.Decks))
		}
		if got.Decks[0].Name != "deck-1" || got.Decks[9].Name != "deck-10" {
			t.Errorf("wrong window: first=%q last=%q", got.Decks[0].Name, got.Decks[9].Name)
		}
		if len(requestedPages) != 1 {
			t.Errorf("expected 1 upstream request, got %d", len(requestedPages))
		}
	})

	t.Run("logical page 2 slices inside the same api page", func(t *testing.T) {
		requestedPages = nil
		got, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
			Commander: "Atraxa, Praetors' Voice", Sort: archidektSortViews, Page: 2, Limit: 10,
		}, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Decks[0].Name != "deck-11" {
			t.Errorf("first deck = %q, want deck-11", got.Decks[0].Name)
		}
		if len(requestedPages) != 1 {
			t.Errorf("expected 1 upstream request, got %d", len(requestedPages))
		}
	})

	t.Run("straddling window fetches two api pages", func(t *testing.T) {
		requestedPages = nil
		got, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
			Commander: "Atraxa, Praetors' Voice", Sort: archidektSortViews, Page: 2, Limit: 50,
		}, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Decks) != 50 {
			t.Fatalf("returned %d decks, want 50", len(got.Decks))
		}
		if got.Decks[0].Name != "deck-51" || got.Decks[49].Name != "deck-100" {
			t.Errorf("wrong window: first=%q last=%q", got.Decks[0].Name, got.Decks[49].Name)
		}
		if len(requestedPages) != 2 {
			t.Errorf("expected 2 upstream requests, got %d (%v)", len(requestedPages), requestedPages)
		}
	})
}

func TestArchidektOrderBy(t *testing.T) {
	tests := []struct {
		sort      string
		ascending bool
		want      string
		wantErr   bool
	}{
		{sort: "views", want: "-viewCount"},
		{sort: "views", ascending: true, want: "viewCount"},
		{sort: "updated", want: "-updatedAt"},
		{sort: "created", want: "-createdAt"},
		{sort: "price", want: "-price"},
		{sort: "size", want: "-size"},
		{sort: "popularity", wantErr: true},
		{sort: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.sort+strconv.FormatBool(tt.ascending), func(t *testing.T) {
			got, err := archidektOrderBy(tt.sort, tt.ascending)
			if (err != nil) != tt.wantErr {
				t.Fatalf("archidektOrderBy(%q, %v) error = %v, wantErr %v", tt.sort, tt.ascending, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("archidektOrderBy(%q, %v) = %q, want %q", tt.sort, tt.ascending, got, tt.want)
			}
		})
	}
}

func TestSearchArchidektDecksSendsOrderByAndBracket(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if ua := r.Header.Get("User-Agent"); ua != "MTG-Commander-MCP-Server/1.0" {
			t.Errorf("User-Agent = %q, want MTG-Commander-MCP-Server/1.0", ua)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Errorf("Accept = %q, want application/json", accept)
		}
		_ = json.NewEncoder(w).Encode(syntheticArchidektPage(1, 60))
	}))
	defer server.Close()

	_, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
		Commander: "Atraxa", Bracket: 4, Sort: "updated", Ascending: true, Page: 1, Limit: 5,
	}, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"orderBy=updatedAt", "edhBracket=4", "deckFormat=3", "commanderName=Atraxa"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestSearchArchidektDecksRejectsUnknownSortWithoutCallingAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("upstream must not be called for an invalid sort")
	}))
	defer server.Close()

	_, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
		Commander: "Atraxa", Sort: "trending", Page: 1, Limit: 10,
	}, server.URL)
	if err == nil {
		t.Fatal("expected an error for an unsupported sort key")
	}
}

func TestInterpretArchidektCount(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		wantTotal  int
		wantCapped bool
		wantKnown  bool
	}{
		{"exact total", 671, 671, false, true},
		{"saturated cap", 1000, 1000, true, true},
		{"not computed", -1, 0, false, false},
		{"zero results", 0, 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, capped, known := interpretArchidektCount(tt.count)
			if total != tt.wantTotal || capped != tt.wantCapped || known != tt.wantKnown {
				t.Errorf("interpretArchidektCount(%d) = (%d, %v, %v), want (%d, %v, %v)",
					tt.count, total, capped, known, tt.wantTotal, tt.wantCapped, tt.wantKnown)
			}
		})
	}
}

func TestFormatArchidektSearchResultsForDisplay(t *testing.T) {
	bracket := 4
	result := &ArchidektSearchResult{
		Decks: []ArchidektDeckSummary{{
			ID: 42, Name: "Toxic Love", ViewCount: 1234, EdhBracket: &bracket,
			UpdatedAt: "2026-01-02T00:00:00Z", Owner: ArchidektOwner{Username: "player"},
		}},
		Page: 3, Limit: 10, Total: 671, TotalKnown: true,
	}

	t.Run("reports the window and exact total", func(t *testing.T) {
		got := FormatArchidektSearchResultsForDisplay(ArchidektSearchParams{
			Commander: "Atraxa", Bracket: 4, Sort: "views", Page: 3, Limit: 10,
		}, result)
		for _, want := range []string{
			"Atraxa", "Bracket 4", "**Sort:** views (descending)",
			"**Total Results:** 671", "showing decks 21–21", "## 21. Toxic Love",
			"https://archidekt.com/decks/42",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("output missing %q\n%s", want, got)
			}
		}
	})

	t.Run("marks a capped total", func(t *testing.T) {
		capped := &ArchidektSearchResult{
			Decks:       result.Decks,
			Page:        1,
			Limit:       10,
			Total:       1000,
			TotalCapped: true,
			TotalKnown:  true,
		}
		got := FormatArchidektSearchResultsForDisplay(
			ArchidektSearchParams{Commander: "Atraxa", Sort: "views", Page: 1, Limit: 10},
			capped,
		)
		if !strings.Contains(got, "1000+") {
			t.Errorf("capped total must be shown as 1000+\n%s", got)
		}
	})

	t.Run("marks an unknown total", func(t *testing.T) {
		unknown := &ArchidektSearchResult{Decks: result.Decks, Page: 1, Limit: 10}
		got := FormatArchidektSearchResultsForDisplay(
			ArchidektSearchParams{Commander: "Atraxa", Sort: "views", Page: 1, Limit: 10},
			unknown,
		)
		if !strings.Contains(got, "unknown") {
			t.Errorf("unknown total must be stated\n%s", got)
		}
	})

	t.Run("no next-page hint once the known total is exhausted", func(t *testing.T) {
		full := &ArchidektSearchResult{
			Decks: archidektPageSlice(20, 1).Results[10:20],
			Page:  2, Limit: 10, Total: 20, TotalKnown: true,
		}
		got := FormatArchidektSearchResultsForDisplay(ArchidektSearchParams{
			Commander: "Atraxa", Sort: "views", Page: 2, Limit: 10,
		}, full)
		if strings.Contains(got, "request page 3") {
			t.Errorf("must not advertise a page beyond the known total\n%s", got)
		}
	})

	t.Run("next-page hint when the known total is larger", func(t *testing.T) {
		more := &ArchidektSearchResult{
			Decks: archidektPageSlice(25, 1).Results[10:20],
			Page:  2, Limit: 10, Total: 25, TotalKnown: true,
		}
		got := FormatArchidektSearchResultsForDisplay(ArchidektSearchParams{
			Commander: "Atraxa", Sort: "views", Page: 2, Limit: 10,
		}, more)
		if !strings.Contains(got, "request page 3") {
			t.Errorf("expected a next-page hint while decks remain\n%s", got)
		}
	})

	t.Run("next-page hint when the total is unknown", func(t *testing.T) {
		unknownTotal := &ArchidektSearchResult{
			Decks: archidektPageSlice(10, 1).Results,
			Page:  1, Limit: 10,
		}
		got := FormatArchidektSearchResultsForDisplay(ArchidektSearchParams{
			Commander: "Atraxa", Sort: "views", Page: 1, Limit: 10,
		}, unknownTotal)
		if !strings.Contains(got, "request page 2") {
			t.Errorf("expected a next-page hint when the total is unknown\n%s", got)
		}
	})

	t.Run("next-page hint survives a capped total", func(t *testing.T) {
		// A capped total is a lower bound, so page*limit reaching it proves nothing.
		capped := &ArchidektSearchResult{
			Decks:       archidektPageSlice(archidektCountCap, 1).Results[:10],
			Page:        100,
			Limit:       10,
			Total:       archidektCountCap,
			TotalCapped: true,
			TotalKnown:  true,
		}
		got := FormatArchidektSearchResultsForDisplay(ArchidektSearchParams{
			Commander: "Atraxa", Sort: "views", Page: 100, Limit: 10,
		}, capped)
		if !strings.Contains(got, "request page 101") {
			t.Errorf("a capped total must keep the next-page hint\n%s", got)
		}

		// The same window with an exact total of the same size is genuinely exhausted.
		exact := &ArchidektSearchResult{
			Decks: capped.Decks, Page: 100, Limit: 10, Total: archidektCountCap, TotalKnown: true,
		}
		got = FormatArchidektSearchResultsForDisplay(ArchidektSearchParams{
			Commander: "Atraxa", Sort: "views", Page: 100, Limit: 10,
		}, exact)
		if strings.Contains(got, "request page 101") {
			t.Errorf("an exhausted exact total must not advertise another page\n%s", got)
		}
	})

	t.Run("empty page", func(t *testing.T) {
		empty := &ArchidektSearchResult{Page: 9, Limit: 10, TotalKnown: true}
		got := FormatArchidektSearchResultsForDisplay(
			ArchidektSearchParams{Commander: "Nobody", Sort: "views", Page: 9, Limit: 10},
			empty,
		)
		if !strings.Contains(got, "No public decks found") {
			t.Errorf("expected empty-results message\n%s", got)
		}
	})
}

func TestNormalizeArchidektColors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "single colour", input: "W", want: "W"},
		{name: "two colours", input: "WU", want: "W,U"},
		{name: "lowercase is accepted", input: "wu", want: "W,U"},
		{name: "already separated", input: "W,U", want: "W,U"},
		{name: "five colours", input: "WUBRG", want: "W,U,B,R,G"},
		{name: "duplicates are rejected", input: "WW", wantErr: true},
		{name: "unknown letter is rejected", input: "WX", wantErr: true},
		{name: "colourless is rejected", input: "C", wantErr: true},
		{name: "empty means no filter", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeArchidektColors(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeArchidektColors(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("normalizeArchidektColors(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSearchArchidektDecksSendsFilters(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(syntheticArchidektPage(1, 12))
	}))
	defer server.Close()

	_, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
		Commander: "Atraxa", Sort: archidektSortViews, Page: 1, Limit: 5,
		Colors: "WU", DeckSize: 99, Author: "NorwegianWhaler", DeckName: "budget",
	}, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"colors":        "W,U",
		"size":          "99",
		"ownerUsername": "NorwegianWhaler",
		"name":          "budget",
	}
	for key, value := range want {
		if gotQuery.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, gotQuery.Get(key), value)
		}
	}
	// Upstream ignores these, so sending them would be misleading noise.
	for _, forbidden := range []string{"cardName", "tagName", "tags", "owner", "pageSize"} {
		if gotQuery.Get(forbidden) != "" {
			t.Errorf("must not send %s (ignored or broken upstream)", forbidden)
		}
	}
}

func TestSearchArchidektDecksRejectsInvalidColors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("upstream must not be called for invalid colors")
	}))
	defer server.Close()

	_, err := searchArchidektDecksWithURL(context.Background(), ArchidektSearchParams{
		Commander: "Atraxa", Sort: archidektSortViews, Page: 1, Limit: 5, Colors: "WX",
	}, server.URL)
	if err == nil {
		t.Fatal("expected an error for an invalid colors value")
	}
}

func TestFormatArchidektSearchResultsShowsActiveFilters(t *testing.T) {
	result := &ArchidektSearchResult{
		Decks:      []ArchidektDeckSummary{{ID: 7, Name: "Budget Atraxa", Owner: ArchidektOwner{Username: "player"}}},
		Page:       1,
		Limit:      10,
		Total:      12,
		TotalKnown: true,
	}
	got := FormatArchidektSearchResultsForDisplay(ArchidektSearchParams{
		Commander: "Atraxa", Sort: archidektSortViews, Page: 1, Limit: 10,
		Colors: "WU", DeckSize: 99, Author: "player", DeckName: "budget",
	}, result)

	for _, want := range []string{"**Filters:**", "colors WU", "size 99", "author player", "name budget"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}
