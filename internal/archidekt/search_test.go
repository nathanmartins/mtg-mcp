package archidekt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// syntheticArchidektPage builds a full 60-deck upstream page whose deck names
// encode their absolute position, so slicing bugs are visible in assertions.
func syntheticArchidektPage(apiPage, count int) UserDecksResponse {
	results := make([]DeckSummary, 0, archidektAPIPageSize)
	base := (apiPage - 1) * archidektAPIPageSize
	for i := range archidektAPIPageSize {
		results = append(results, DeckSummary{
			ID:         base + i + 1,
			Name:       fmt.Sprintf("deck-%d", base+i+1),
			DeckFormat: 3,
			ViewCount:  1000 - (base + i),
			UpdatedAt:  "2026-01-01T00:00:00Z",
			Owner:      Owner{Username: testOwner},
		})
	}
	next := ""
	if apiPage*archidektAPIPageSize < count {
		next = fmt.Sprintf("http://archidekt.com/api/decks/v3/?page=%d", apiPage+1)
	}
	return UserDecksResponse{Count: count, Next: next, Results: results}
}

// archidektPageSlice builds the upstream page apiPage would return for a result set of
// total decks, so short pages and pages past the end behave like the real API.
func archidektPageSlice(total, apiPage int) UserDecksResponse {
	start := (apiPage - 1) * archidektAPIPageSize
	end := min(start+archidektAPIPageSize, total)
	results := make([]DeckSummary, 0, archidektAPIPageSize)
	for i := start; i < end; i++ {
		results = append(results, DeckSummary{
			ID:         i + 1,
			Name:       fmt.Sprintf("deck-%d", i+1),
			DeckFormat: 3,
			UpdatedAt:  "2026-01-01T00:00:00Z",
			Owner:      Owner{Username: testOwner},
		})
	}
	next := ""
	if end < total {
		next = fmt.Sprintf("http://archidekt.com/api/decks/v3/?page=%d", apiPage+1)
	}
	return UserDecksResponse{Count: total, Next: next, Results: results}
}

func TestSearchDecksHandlesShortUpstreamPages(t *testing.T) {
	t.Parallel()
	fixtureServer := func(total int, log *pageLog) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.add(r.URL.Query().Get("page"))
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
	// search runs one logical-page search against its own fixture holding total decks and
	// returns the upstream pages it requested.
	search := func(t *testing.T, total, page, limit int) (*SearchResult, []string) {
		t.Helper()
		var log pageLog
		server := fixtureServer(total, &log)
		t.Cleanup(server.Close)

		got, err := (&Client{BaseURL: server.URL}).SearchDecks(t.Context(), SearchParams{
			Commander: testAtraxaShort, Sort: archidektSortViews, Page: page, Limit: limit,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return got, log.list()
	}

	t.Run("window inside a short page", func(t *testing.T) {
		t.Parallel()
		got, requestedPages := search(t, 25, 2, 10)
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
		t.Parallel()
		got, requestedPages := search(t, 25, 9, 10)
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
		t.Parallel()
		got, requestedPages := search(t, 55, 2, 50)
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
	t.Parallel()
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
			t.Parallel()
			gotPage, gotOffset, gotPages := archidektPageWindow(tt.page, tt.limit)
			if gotPage != tt.wantAPIPage || gotOffset != tt.wantOffset || gotPages != tt.wantAPIPages {
				t.Errorf("archidektPageWindow(%d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tt.page, tt.limit, gotPage, gotOffset, gotPages,
					tt.wantAPIPage, tt.wantOffset, tt.wantAPIPages)
			}
		})
	}
}

func TestSearchDecksHonoursLimitAndPage(t *testing.T) {
	t.Parallel()
	// newClient returns a client for a fresh 500-deck fixture and the log of upstream pages it
	// requests, so parallel subtests never share request counts.
	newClient := func(t *testing.T) (*Client, *pageLog) {
		t.Helper()
		log := &pageLog{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			log.add(q.Get("page"))
			if q.Get("pageSize") != "" || q.Get("limit") != "" {
				t.Errorf("must not send an upstream page-size parameter, got %q", r.URL.RawQuery)
			}
			apiPage := 1
			if q.Get("page") == "2" {
				apiPage = 2
			}
			_ = json.NewEncoder(w).Encode(syntheticArchidektPage(apiPage, 500))
		}))
		t.Cleanup(server.Close)
		return &Client{BaseURL: server.URL}, log
	}

	t.Run("limit is exact", func(t *testing.T) {
		t.Parallel()
		client, log := newClient(t)
		got, err := client.SearchDecks(t.Context(), SearchParams{
			Commander: testAtraxa, Sort: archidektSortViews, Page: 1, Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Decks) != 10 {
			t.Fatalf("returned %d decks, want 10", len(got.Decks))
		}
		if got.Decks[0].Name != "deck-1" || got.Decks[9].Name != "deck-10" {
			t.Errorf("wrong window: first=%q last=%q", got.Decks[0].Name, got.Decks[9].Name)
		}
		if len(log.list()) != 1 {
			t.Errorf("expected 1 upstream request, got %d", len(log.list()))
		}
	})

	t.Run("logical page 2 slices inside the same api page", func(t *testing.T) {
		t.Parallel()
		client, log := newClient(t)
		got, err := client.SearchDecks(t.Context(), SearchParams{
			Commander: testAtraxa, Sort: archidektSortViews, Page: 2, Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Decks[0].Name != "deck-11" {
			t.Errorf("first deck = %q, want deck-11", got.Decks[0].Name)
		}
		if len(log.list()) != 1 {
			t.Errorf("expected 1 upstream request, got %d", len(log.list()))
		}
	})

	t.Run("straddling window fetches two api pages", func(t *testing.T) {
		t.Parallel()
		client, log := newClient(t)
		got, err := client.SearchDecks(t.Context(), SearchParams{
			Commander: testAtraxa, Sort: archidektSortViews, Page: 2, Limit: 50,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Decks) != 50 {
			t.Fatalf("returned %d decks, want 50", len(got.Decks))
		}
		if got.Decks[0].Name != "deck-51" || got.Decks[49].Name != "deck-100" {
			t.Errorf("wrong window: first=%q last=%q", got.Decks[0].Name, got.Decks[49].Name)
		}
		if len(log.list()) != 2 {
			t.Errorf("expected 2 upstream requests, got %d (%v)", len(log.list()), log.list())
		}
	})
}

func TestOrderBy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		sort      string
		ascending bool
		want      string
		wantErr   bool
	}{
		{sort: archidektSortViews, want: "-viewCount"},
		{sort: archidektSortViews, ascending: true, want: "viewCount"},
		{sort: "updated", want: "-updatedAt"},
		{sort: "created", want: "-createdAt"},
		{sort: "price", want: "-price"},
		{sort: testSortSize, want: "-size"},
		{sort: "popularity", wantErr: true},
		{sort: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.sort+strconv.FormatBool(tt.ascending), func(t *testing.T) {
			t.Parallel()
			got, err := OrderBy(tt.sort, tt.ascending)
			if (err != nil) != tt.wantErr {
				t.Fatalf("OrderBy(%q, %v) error = %v, wantErr %v", tt.sort, tt.ascending, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("OrderBy(%q, %v) = %q, want %q", tt.sort, tt.ascending, got, tt.want)
			}
		})
	}
}

func TestSearchDecksSendsOrderByAndBracket(t *testing.T) {
	t.Parallel()
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

	_, err := (&Client{BaseURL: server.URL}).SearchDecks(t.Context(), SearchParams{
		Commander: testAtraxaShort, Bracket: 4, Sort: "updated", Ascending: true, Page: 1, Limit: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"orderBy=updatedAt", "edhBracket=4", "deckFormat=3", "commanderName=Atraxa"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestSearchDecksRejectsUnknownSortWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("upstream must not be called for an invalid sort")
	}))
	defer server.Close()

	_, err := (&Client{BaseURL: server.URL}).SearchDecks(t.Context(), SearchParams{
		Commander: testAtraxaShort, Sort: "trending", Page: 1, Limit: 10,
	})
	if err == nil {
		t.Fatal("expected an error for an unsupported sort key")
	}
}

func TestInterpretArchidektCount(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			total, capped, known := interpretArchidektCount(tt.count)
			if total != tt.wantTotal || capped != tt.wantCapped || known != tt.wantKnown {
				t.Errorf("interpretArchidektCount(%d) = (%d, %v, %v), want (%d, %v, %v)",
					tt.count, total, capped, known, tt.wantTotal, tt.wantCapped, tt.wantKnown)
			}
		})
	}
}

func TestFormatSearchResults(t *testing.T) {
	t.Parallel()
	bracket := 4
	result := &SearchResult{
		Decks: []DeckSummary{{
			ID: 42, Name: "Toxic Love", ViewCount: 1234, EdhBracket: &bracket,
			UpdatedAt: "2026-01-02T00:00:00Z", Owner: Owner{Username: testPlayer},
		}},
		Page: 3, Limit: 10, Total: 671, TotalKnown: true,
	}

	t.Run("reports the window and exact total", func(t *testing.T) {
		t.Parallel()
		got := FormatSearchResults(SearchParams{
			Commander: testAtraxaShort, Bracket: 4, Sort: archidektSortViews, Page: 3, Limit: 10,
		}, result)
		for _, want := range []string{testAtraxaShort, "Bracket 4", "**Sort:** views (desc)",
			"**Total Results:** 671", "showing decks 21–21", "## 21. Toxic Love",
			"https://archidekt.com/decks/42",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("output missing %q\n%s", want, got)
			}
		}
	})

	t.Run("marks a capped total", func(t *testing.T) {
		t.Parallel()
		capped := &SearchResult{
			Decks:       result.Decks,
			Page:        1,
			Limit:       10,
			Total:       1000,
			TotalCapped: true,
			TotalKnown:  true,
		}
		got := FormatSearchResults(
			SearchParams{Commander: testAtraxaShort, Sort: archidektSortViews, Page: 1, Limit: 10},
			capped,
		)
		if !strings.Contains(got, "1000+") {
			t.Errorf("capped total must be shown as 1000+\n%s", got)
		}
	})

	t.Run("marks an unknown total", func(t *testing.T) {
		t.Parallel()
		unknown := &SearchResult{Decks: result.Decks, Page: 1, Limit: 10}
		got := FormatSearchResults(
			SearchParams{Commander: testAtraxaShort, Sort: archidektSortViews, Page: 1, Limit: 10},
			unknown,
		)
		if !strings.Contains(got, "unknown") {
			t.Errorf("unknown total must be stated\n%s", got)
		}
	})

	t.Run("no next-page hint once the known total is exhausted", func(t *testing.T) {
		t.Parallel()
		full := &SearchResult{
			Decks: archidektPageSlice(20, 1).Results[10:20],
			Page:  2, Limit: 10, Total: 20, TotalKnown: true,
		}
		got := FormatSearchResults(SearchParams{
			Commander: testAtraxaShort, Sort: archidektSortViews, Page: 2, Limit: 10,
		}, full)
		if strings.Contains(got, "request page 3") {
			t.Errorf("must not advertise a page beyond the known total\n%s", got)
		}
	})

	t.Run("next-page hint when the known total is larger", func(t *testing.T) {
		t.Parallel()
		more := &SearchResult{
			Decks: archidektPageSlice(25, 1).Results[10:20],
			Page:  2, Limit: 10, Total: 25, TotalKnown: true,
		}
		got := FormatSearchResults(SearchParams{
			Commander: testAtraxaShort, Sort: archidektSortViews, Page: 2, Limit: 10,
		}, more)
		if !strings.Contains(got, "request page 3") {
			t.Errorf("expected a next-page hint while decks remain\n%s", got)
		}
	})

	t.Run("next-page hint when the total is unknown", func(t *testing.T) {
		t.Parallel()
		unknownTotal := &SearchResult{
			Decks: archidektPageSlice(10, 1).Results,
			Page:  1, Limit: 10,
		}
		got := FormatSearchResults(SearchParams{
			Commander: testAtraxaShort, Sort: archidektSortViews, Page: 1, Limit: 10,
		}, unknownTotal)
		if !strings.Contains(got, "request page 2") {
			t.Errorf("expected a next-page hint when the total is unknown\n%s", got)
		}
	})

	t.Run("next-page hint survives a capped total", func(t *testing.T) {
		t.Parallel(
		// A capped total is a lower bound, so page*limit reaching it proves nothing.
		)

		capped := &SearchResult{
			Decks:       archidektPageSlice(archidektCountCap, 1).Results[:10],
			Page:        100,
			Limit:       10,
			Total:       archidektCountCap,
			TotalCapped: true,
			TotalKnown:  true,
		}
		got := FormatSearchResults(SearchParams{
			Commander: testAtraxaShort, Sort: archidektSortViews, Page: 100, Limit: 10,
		}, capped)
		if !strings.Contains(got, "request page 101") {
			t.Errorf("a capped total must keep the next-page hint\n%s", got)
		}

		// The same window with an exact total of the same size is genuinely exhausted.
		exact := &SearchResult{
			Decks: capped.Decks, Page: 100, Limit: 10, Total: archidektCountCap, TotalKnown: true,
		}
		got = FormatSearchResults(SearchParams{
			Commander: testAtraxaShort, Sort: archidektSortViews, Page: 100, Limit: 10,
		}, exact)
		if strings.Contains(got, "request page 101") {
			t.Errorf("an exhausted exact total must not advertise another page\n%s", got)
		}
	})

	t.Run("empty page", func(t *testing.T) {
		t.Parallel()
		empty := &SearchResult{Page: 9, Limit: 10, TotalKnown: true}
		got := FormatSearchResults(
			SearchParams{Commander: "Nobody", Sort: archidektSortViews, Page: 9, Limit: 10},
			empty,
		)
		if !strings.Contains(got, "No public decks found") {
			t.Errorf("expected empty-results message\n%s", got)
		}
	})
}

func TestNormalizeColors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "single colour", input: "W", want: "W"},
		{name: "two colours", input: "WU", want: testColorsWU},
		{name: "lowercase is accepted", input: "wu", want: testColorsWU},
		{name: "already separated", input: testColorsWU, want: testColorsWU},
		{name: "five colours", input: "WUBRG", want: "W,U,B,R,G"},
		{name: "duplicates are rejected", input: "WW", wantErr: true},
		{name: "unknown letter is rejected", input: "WX", wantErr: true},
		{name: "colourless is rejected", input: "C", wantErr: true},
		{name: "empty means no filter", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeColors(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeColors(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizeColors(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSearchDecksSendsFilters(t *testing.T) {
	t.Parallel()
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(syntheticArchidektPage(1, 12))
	}))
	defer server.Close()

	_, err := (&Client{BaseURL: server.URL}).SearchDecks(t.Context(), SearchParams{
		Commander: testAtraxaShort, Sort: archidektSortViews, Page: 1, Limit: 5,
		Colors: "WU", DeckSize: 99, Author: testOwnerName, DeckName: testDeckNameFilter,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"colors": testColorsWU, testSortSize: "99", "ownerUsername": testOwnerName,
		"name": testDeckNameFilter,
	}
	for key, value := range want {
		if gotQuery.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, gotQuery.Get(key), value)
		}
	}
	// Upstream ignores these, so sending them would be misleading noise.
	for _, forbidden := range []string{"cardName", "tagName", "tags", testOwner, "pageSize"} {
		if gotQuery.Get(forbidden) != "" {
			t.Errorf("must not send %s (ignored or broken upstream)", forbidden)
		}
	}
}

func TestSearchDecksOmitsUnsetFilters(t *testing.T) {
	t.Parallel()
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(syntheticArchidektPage(1, 12))
	}))
	defer server.Close()

	params := SearchParams{Commander: testAtraxaShort, Sort: archidektSortViews, Page: 1, Limit: 5}
	result, err := (&Client{BaseURL: server.URL}).SearchDecks(t.Context(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// An empty filter must be absent, not empty: size=0 would filter for zero-card decks.
	for _, key := range []string{"colors", testSortSize, "ownerUsername", "name"} {
		if _, present := gotQuery[key]; present {
			t.Errorf("%s must be omitted when unset, got %q", key, gotQuery.Get(key))
		}
	}
	if text := FormatSearchResults(params, result); strings.Contains(text, "**Filters:**") {
		t.Errorf("no filters were set, so no filter line should be printed:\n%s", text)
	}
}

func TestSearchDecksRejectsInvalidColors(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("upstream must not be called for invalid colors")
	}))
	defer server.Close()

	_, err := (&Client{BaseURL: server.URL}).SearchDecks(t.Context(), SearchParams{
		Commander: testAtraxaShort, Sort: archidektSortViews, Page: 1, Limit: 5, Colors: "WX",
	})
	if err == nil {
		t.Fatal("expected an error for an invalid colors value")
	}
}

func TestFormatSearchResultsShowsActiveFilters(t *testing.T) {
	t.Parallel()
	result := &SearchResult{
		Decks:      []DeckSummary{{ID: 7, Name: "Budget Atraxa", Owner: Owner{Username: testPlayer}}},
		Page:       1,
		Limit:      10,
		Total:      12,
		TotalKnown: true,
	}
	got := FormatSearchResults(SearchParams{
		Commander: testAtraxaShort, Sort: archidektSortViews, Page: 1, Limit: 10,
		Colors: "WU", DeckSize: 99, Author: testPlayer, DeckName: testDeckNameFilter,
	}, result)

	for _, want := range []string{"**Filters:**", "colors WU", "size 99", "author player", "name budget"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// pageLog records the upstream page parameter of each request a fixture server receives. The
// handler runs on the server's goroutine, so access is guarded.
type pageLog struct {
	mu    sync.Mutex
	pages []string
}

func (l *pageLog) add(page string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pages = append(l.pages, page)
}

func (l *pageLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.pages)
}
