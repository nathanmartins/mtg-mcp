package moxfield

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestExtractPublicID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full URL",
			input: "https://www.moxfield.com/decks/abc123",
			want:  testShortID,
		},
		{
			name:  "URL with trailing slash",
			input: "https://www.moxfield.com/decks/xyz789/",
			want:  "xyz789",
		},
		{
			name:  "just ID",
			input: "def456",
			want:  "def456",
		},
		{
			name:  "URL with query parameters",
			input: "https://www.moxfield.com/decks/ghi789?tab=visual",
			want:  "ghi789?tab=visual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractPublicID(tt.input)
			if got != tt.want {
				t.Errorf("ExtractPublicID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDeck(t *testing.T) {
	t.Parallel()
	mockDeck := Deck{
		ID:          "test-id",
		PublicID:    testShortID,
		Name:        testDeckName,
		Format:      testFormatCommander,
		Description: "A test deck",
		ViewCount:   100,
		LikeCount:   50,
		Authors:     []string{"TestAuthor"},
		Commanders: map[string]CardEntry{testFormatCommander: {
			Quantity: 1,
			Card: CardInfo{
				Name: "Test Commander",
			},
		},
		},
		Mainboard: map[string]CardEntry{
			"card1": {
				Quantity: 1,
				Card: CardInfo{
					Name: testSolRing,
				},
			},
		},
	}

	tests := []struct {
		name         string
		publicID     string
		mockDeck     Deck
		mockStatus   int
		wantErr      bool
		checkHeaders bool
	}{
		{
			name:         "successful request",
			publicID:     testShortID,
			mockDeck:     mockDeck,
			mockStatus:   http.StatusOK,
			wantErr:      false,
			checkHeaders: true,
		},
		{
			name:       "404 not found",
			publicID:   "nonexistent",
			mockStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "500 server error",
			publicID:   "error",
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.checkHeaders {
					if ua := r.Header.Get("User-Agent"); ua != "MTG-Commander-MCP-Server/1.0" {
						t.Errorf("User-Agent = %v, want MTG-Commander-MCP-Server/1.0", ua)
					}
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockDeck)
				}
			}))
			defer server.Close()

			ctx := t.Context()
			got, err := (&Client{BaseURL: server.URL}).GetDeck(ctx, tt.publicID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeck() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Name != tt.mockDeck.Name {
					t.Errorf("GetDeck() name = %v, want %v", got.Name, tt.mockDeck.Name)
				}
				if got.PublicID != tt.mockDeck.PublicID {
					t.Errorf("GetDeck() publicID = %v, want %v", got.PublicID, tt.mockDeck.PublicID)
				}
			}
		})
	}
}

func TestGetUserDecks(t *testing.T) {
	t.Parallel()
	mockResponse := UserDecksResponse{
		PageNumber:   1,
		PageSize:     20,
		TotalResults: 2,
		TotalPages:   1,
		Data: []DeckSummary{
			{
				PublicID:  "deck1",
				Name:      "Deck 1",
				Format:    testFormatCommander,
				PublicURL: "https://moxfield.com/decks/deck1",
				ViewCount: 100,
				LikeCount: 10,
			},
			{
				PublicID:  "deck2",
				Name:      "Deck 2",
				Format:    testFormatCommander,
				PublicURL: "https://moxfield.com/decks/deck2",
				ViewCount: 200,
				LikeCount: 20,
			},
		},
	}

	tests := []struct {
		name         string
		username     string
		pageSize     int
		mockResponse UserDecksResponse
		mockStatus   int
		wantErr      bool
	}{
		{
			name:         "successful request",
			username:     "testuser",
			pageSize:     20,
			mockResponse: mockResponse,
			mockStatus:   http.StatusOK,
			wantErr:      false,
		},
		{
			name:         "with custom page size",
			username:     "testuser",
			pageSize:     50,
			mockResponse: mockResponse,
			mockStatus:   http.StatusOK,
			wantErr:      false,
		},
		{
			name:       "user not found",
			username:   "nonexistent",
			pageSize:   20,
			mockStatus: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("authorUserNames"); got != tt.username {
					t.Errorf("authorUserNames = %q, want %q", got, tt.username)
				}
				if got := r.URL.Query().Get("pageSize"); got != strconv.Itoa(tt.pageSize) {
					t.Errorf("pageSize = %q, want %d", got, tt.pageSize)
				}
				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			ctx := t.Context()
			got, err := (&Client{SearchURL: server.URL}).GetUserDecks(ctx, tt.username, tt.pageSize)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserDecks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if len(got.Data) != len(tt.mockResponse.Data) {
					t.Errorf("GetUserDecks() deck count = %v, want %v", len(got.Data), len(tt.mockResponse.Data))
				}
			}
		})
	}
}

func TestSearchDecks(t *testing.T) {
	t.Parallel()
	mockResponse := SearchResponse{
		PageNumber:   1,
		PageSize:     20,
		TotalResults: 50,
		TotalPages:   3,
		Data: []DeckSummary{
			{
				PublicID:  "deck1",
				Name:      "Atraxa Superfriends",
				Format:    testFormatCommander,
				PublicURL: "https://moxfield.com/decks/deck1",
				ViewCount: 1000,
				LikeCount: 100,
			},
			{
				PublicID:  "deck2",
				Name:      "Atraxa Infect",
				Format:    testFormatCommander,
				PublicURL: "https://moxfield.com/decks/deck2",
				ViewCount: 500,
				LikeCount: 50,
			},
		},
	}

	tests := []struct {
		name         string
		params       SearchParams
		mockResponse SearchResponse
		mockStatus   int
		wantErr      bool
		checkQuery   bool
	}{
		{
			name: "search by commander",
			params: SearchParams{
				CardName:      "Atraxa",
				Format:        testFormatCommander,
				SortType:      testSortUpdated,
				SortDirection: DirectionDescending,
				PageSize:      20,
				PageNumber:    1,
			},
			mockResponse: mockResponse,
			mockStatus:   http.StatusOK,
			wantErr:      false,
			checkQuery:   true,
		},
		{
			name: "with different sort",
			params: SearchParams{
				CardName:      "Atraxa",
				Format:        testFormatCommander,
				SortType:      testSortViews,
				SortDirection: DirectionDescending,
				PageSize:      10,
				PageNumber:    1,
			},
			mockResponse: mockResponse,
			mockStatus:   http.StatusOK,
			wantErr:      false,
		},
		{
			name: "server error",
			params: SearchParams{
				CardName:   testName,
				PageSize:   20,
				PageNumber: 1,
			},
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.checkQuery {
					query := r.URL.Query()
					if query.Get("cardName") != tt.params.CardName {
						t.Errorf("cardName parameter = %v, want %v", query.Get("cardName"), tt.params.CardName)
					}
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			ctx := t.Context()
			got, err := (&Client{SearchURL: server.URL}).SearchDecks(ctx, tt.params)

			if (err != nil) != tt.wantErr {
				t.Errorf("SearchDecks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if len(got.Data) != len(tt.mockResponse.Data) {
					t.Errorf("SearchDecks() deck count = %v, want %v", len(got.Data), len(tt.mockResponse.Data))
				}
				if got.TotalResults != tt.mockResponse.TotalResults {
					t.Errorf(
						"SearchDecks() total results = %v, want %v",
						got.TotalResults,
						tt.mockResponse.TotalResults,
					)
				}
			}
		})
	}
}

func TestSearchDecks_PageSizeValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		inputPageSize  int
		expectPageSize string
	}{
		{
			name:           "page size too large",
			inputPageSize:  150,
			expectPageSize: "20",
		},
		{
			name:           "page size zero",
			inputPageSize:  0,
			expectPageSize: "20",
		},
		{
			name:           "page size negative",
			inputPageSize:  -10,
			expectPageSize: "20",
		},
		{
			name:           "valid page size",
			inputPageSize:  50,
			expectPageSize: "50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var gotPageSize string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPageSize = r.URL.Query().Get("pageSize")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(SearchResponse{
					Data: []DeckSummary{},
				})
			}))
			defer server.Close()

			params := SearchParams{
				CardName:   "test",
				PageSize:   tt.inputPageSize,
				PageNumber: 1,
			}

			ctx := t.Context()
			_, err := (&Client{SearchURL: server.URL}).SearchDecks(ctx, params)
			if err != nil {
				t.Errorf("SearchDecks() unexpected error = %v", err)
			}
			if gotPageSize != tt.expectPageSize {
				t.Errorf("pageSize parameter = %q, want %q", gotPageSize, tt.expectPageSize)
			}
		})
	}
}

func TestSearchDecksSendsVerifiedParams(t *testing.T) {
	t.Parallel()
	var gotQuery url.Values
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).
			Encode(SearchResponse{PageNumber: 2, PageSize: 50, TotalResults: 120, TotalPages: 3})
	}))
	defer server.Close()

	client := &Client{SearchURL: server.URL + "/v2/decks/search"}
	_, err := client.SearchDecks(t.Context(), SearchParams{
		CardName:      testAtraxa,
		Format:        testFormatCommander,
		SortType:      testSortViews,
		SortDirection: DirectionDescending,
		PageSize:      50,
		PageNumber:    2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/v2/decks/search" {
		t.Errorf("path = %q, want /v2/decks/search", gotPath)
	}
	want := map[string]string{
		"cardName":      testAtraxa,
		"fmt":           testFormatCommander,
		"sortType":      testSortViews,
		"sortDirection": DirectionDescending,
		"pageSize":      "50",
		"pageNumber":    "2",
	}
	for key, value := range want {
		if gotQuery.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, gotQuery.Get(key), value)
		}
	}
	// Moxfield ignores these entirely; sending them pretends we filter when we do not.
	for _, forbidden := range []string{"query", "q", "board", "commanderName", "commanders"} {
		if gotQuery.Get(forbidden) != "" {
			t.Errorf("must not send %s (ignored upstream)", forbidden)
		}
	}
}

func TestParseSortType(t *testing.T) {
	t.Parallel()
	for _, valid := range []string{testSortUpdated, "created", testSortViews, "likes", "comments", "relevance"} {
		if got, err := ParseSortType(valid); err != nil || got != valid {
			t.Errorf("ParseSortType(%q) = (%q, %v), want (%q, nil)", valid, got, err, valid)
		}
	}
	// price is rejected by the Moxfield API with HTTP 400, so reject it locally.
	for _, invalid := range []string{"price", "", "trending"} {
		if _, err := ParseSortType(invalid); err == nil {
			t.Errorf("ParseSortType(%q) should fail", invalid)
		}
	}
}

func TestSortDirection(t *testing.T) {
	t.Parallel()
	if got := SortDirection(false); got != DirectionDescending {
		t.Errorf("descending = %q, want Descending", got)
	}
	if got := SortDirection(true); got != "Ascending" {
		t.Errorf("ascending = %q, want Ascending", got)
	}
}

// TestMoxfieldSearchURLWiring pins the fix for the production HTTP 404: deck reads and deck
// search live on different Moxfield hosts, and the server must be built with the search URL.
func TestMoxfieldSearchURLWiring(t *testing.T) {
	t.Parallel()
	if DefaultSearchURL == DefaultBaseURL {
		t.Fatal("the search URL must differ from the deck-read base URL")
	}
	parsed, parseErr := url.Parse(DefaultSearchURL)
	if parseErr != nil {
		t.Fatalf("unparseable search URL: %v", parseErr)
	}
	if parsed.Host != "api2.moxfield.com" {
		t.Errorf("search host = %q, want api2.moxfield.com", parsed.Host)
	}
	if parsed.Path != "/v2/decks/search" {
		t.Errorf("search path = %q, want /v2/decks/search", parsed.Path)
	}
}

func TestFormatDeck(t *testing.T) {
	t.Parallel()
	deck := &Deck{
		Name:         testDeckName,
		Format:       testFormatCommander,
		Authors:      []any{"Author1", "Author2"},
		ViewCount:    1000,
		LikeCount:    100,
		CommentCount: 50,
		Description:  "This is a test deck",
		Commanders: map[string]CardEntry{
			"cmd": {
				Quantity: 1,
				Card: CardInfo{
					Name: testAtraxa,
				},
			},
		},
		Mainboard: map[string]CardEntry{
			"card1": {
				Quantity: 1,
				Card: CardInfo{
					Name:     testSolRing,
					TypeLine: "Artifact",
				},
			},
			"card2": {
				Quantity: 1,
				Card: CardInfo{
					Name:     "Lightning Bolt",
					TypeLine: testInstant,
				},
			},
			"card3": {
				Quantity: 1,
				Card: CardInfo{
					Name:     "Birds of Paradise",
					TypeLine: "Creature - Bird",
				},
			},
		},
	}

	tests := []struct {
		name         string
		deck         *Deck
		wantContains []string
	}{
		{
			name: "basic formatting",
			deck: deck,
			wantContains: []string{
				"# Test Deck", testFormatCommander, "Author1, Author2",
				"**Views:** 1000",
				"**Likes:** 100",
				"This is a test deck",
				"Commanders", testAtraxa, "Mainboard", testSolRing, "Lightning Bolt",
				"Birds of Paradise",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatDeck(tt.deck)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatDeck() missing %q in output", want)
				}
			}
		})
	}
}

func TestGroupDeckCards(t *testing.T) {
	t.Parallel()
	mainboard := map[string]CardEntry{
		"a": {Quantity: 1, Card: CardInfo{Name: "Counterspell", TypeLine: testInstant}},
		"b": {Quantity: 1, Card: CardInfo{Name: "Ponder", TypeLine: "Sorcery"}},
		"c": {Quantity: 1, Card: CardInfo{Name: "Sylvan Library", TypeLine: "Enchantment"}},
		"d": {
			Quantity: 1,
			Card:     CardInfo{Name: "Nissa, Who Shakes the World", TypeLine: "Legendary Planeswalker — Nissa"},
		},
		"e": {Quantity: 1, Card: CardInfo{Name: "Island", TypeLine: "Basic Land — Island"}},
		"f": {Quantity: 2, Card: CardInfo{Name: "Unknown Token", TypeLine: "Token"}},
	}

	groups := groupDeckCards(mainboard)

	if len(groups.Instants) != 1 || groups.Instants[0] != "1x Counterspell" {
		t.Errorf("instants = %v, want [1x Counterspell]", groups.Instants)
	}
	if len(groups.Sorceries) != 1 || groups.Sorceries[0] != "1x Ponder" {
		t.Errorf("sorceries = %v, want [1x Ponder]", groups.Sorceries)
	}
	if len(groups.Enchantments) != 1 || groups.Enchantments[0] != "1x Sylvan Library" {
		t.Errorf("enchantments = %v, want [1x Sylvan Library]", groups.Enchantments)
	}
	if len(groups.Planeswalkers) != 1 {
		t.Errorf("planeswalkers = %v, want 1 entry", groups.Planeswalkers)
	}
	if len(groups.Lands) != 1 || groups.Lands[0] != "1x Island" {
		t.Errorf("lands = %v, want [1x Island]", groups.Lands)
	}
	if len(groups.Others) != 1 || groups.Others[0] != "2x Unknown Token" {
		t.Errorf("others = %v, want [2x Unknown Token]", groups.Others)
	}
	if groups.TotalCards != 7 {
		t.Errorf("totalCards = %d, want 7", groups.TotalCards)
	}
}

func TestFormatDeckSideboardAndMaybeboard(t *testing.T) {
	t.Parallel()
	deck := &Deck{
		Name:   testDeckName,
		Format: testFormatCommander,
		Mainboard: map[string]CardEntry{
			"a": {Quantity: 1, Card: CardInfo{Name: testSolRing, TypeLine: "Artifact"}},
		},
		Sideboard: map[string]CardEntry{
			"b": {Quantity: 1, Card: CardInfo{Name: "Swan Song", TypeLine: testInstant}},
		},
		Maybeboard: map[string]CardEntry{
			"c": {Quantity: 1, Card: CardInfo{Name: "Cyclonic Rift", TypeLine: testInstant}},
		},
	}

	got := FormatDeck(deck)

	if !strings.Contains(got, "## Sideboard") {
		t.Error("FormatDeck() missing Sideboard section")
	}
	if !strings.Contains(got, "Swan Song") {
		t.Error("FormatDeck() missing sideboard card")
	}
	if !strings.Contains(got, "## Maybeboard") {
		t.Error("FormatDeck() missing Maybeboard section")
	}
	if !strings.Contains(got, "Cyclonic Rift") {
		t.Error("FormatDeck() missing maybeboard card")
	}
}

func TestFormatDeckOthers(t *testing.T) {
	t.Parallel()
	deck := &Deck{
		Name:   testDeckName,
		Format: testFormatCommander,
		Mainboard: map[string]CardEntry{
			"a": {Quantity: 1, Card: CardInfo{Name: "Treasure Token", TypeLine: "Token"}},
		},
	}

	got := FormatDeck(deck)

	if !strings.Contains(got, "Other") {
		t.Error("FormatDeck() missing Other section for untyped card")
	}
	if !strings.Contains(got, "Treasure Token") {
		t.Error("FormatDeck() missing card in Other section")
	}
}

func TestFormatDeckHeaderAuthors(t *testing.T) {
	t.Parallel(
	// The live API's shape, confirmed against a real deck read.
	)

	t.Run("user objects show their usernames", func(t *testing.T) {
		t.Parallel()
		deck := &Deck{
			Name:   testName,
			Format: testFormatCommander,
			Authors: []any{
				map[string]any{"userName": "fabudl", "displayName": "Fab", "badges": []any{}},
				map[string]any{"userName": "second", "displayName": "Second"},
			},
		}
		got := formatDeckHeader(deck)
		if !strings.Contains(got, "**Author:** fabudl, second") {
			t.Errorf("formatDeckHeader() should list author usernames, got:\n%s", got)
		}
	})

	t.Run("non-slice authors skipped gracefully", func(t *testing.T) {
		t.Parallel()
		deck := &Deck{
			Name:    testName,
			Format:  testFormatCommander,
			Authors: "not-a-slice",
		}
		got := formatDeckHeader(deck)
		if strings.Contains(got, "**Author:**") {
			t.Error("formatDeckHeader() should skip non-slice Authors")
		}
	})

	t.Run("slice with non-string values skipped", func(t *testing.T) {
		t.Parallel()
		deck := &Deck{
			Name:    testName,
			Format:  testFormatCommander,
			Authors: []any{42, true},
		}
		got := formatDeckHeader(deck)
		if strings.Contains(got, "**Author:**") {
			t.Error("formatDeckHeader() should skip non-string author values")
		}
	})
}

// TestNewClient pins the production settings: search lives on a different host from deck
// reads, and verification requests must be spaced.
func TestNewClient(t *testing.T) {
	t.Parallel()
	c := NewClient()
	if c.BaseURL != DefaultBaseURL || c.SearchURL != DefaultSearchURL {
		t.Errorf("NewClient() URLs = %q, %q", c.BaseURL, c.SearchURL)
	}
	if c.VerifyDelay != VerifyDelay || c.VerifyDelay <= 0 {
		t.Errorf("VerifyDelay = %v, want %v — production must space verification requests", c.VerifyDelay, VerifyDelay)
	}
}
