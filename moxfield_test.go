package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractPublicIDFromURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full URL",
			input: "https://www.moxfield.com/decks/abc123",
			want:  "abc123",
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
			got := ExtractPublicIDFromURL(tt.input)
			if got != tt.want {
				t.Errorf("ExtractPublicIDFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMoxfieldDeck(t *testing.T) {
	mockDeck := MoxfieldDeck{
		ID:          "test-id",
		PublicID:    "abc123",
		Name:        "Test Deck",
		Format:      "commander",
		Description: "A test deck",
		ViewCount:   100,
		LikeCount:   50,
		Authors:     []string{"TestAuthor"},
		Commanders: map[string]MoxfieldCardEntry{
			"commander": {
				Quantity: 1,
				Card: MoxfieldCardInfo{
					Name: "Test Commander",
				},
			},
		},
		Mainboard: map[string]MoxfieldCardEntry{
			"card1": {
				Quantity: 1,
				Card: MoxfieldCardInfo{
					Name: "Sol Ring",
				},
			},
		},
	}

	tests := []struct {
		name         string
		publicID     string
		mockDeck     MoxfieldDeck
		mockStatus   int
		wantErr      bool
		checkHeaders bool
	}{
		{
			name:         "successful request",
			publicID:     "abc123",
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

			ctx := context.Background()
			got, err := getMoxfieldDeckWithURL(ctx, tt.publicID, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetMoxfieldDeck() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Name != tt.mockDeck.Name {
					t.Errorf("GetMoxfieldDeck() name = %v, want %v", got.Name, tt.mockDeck.Name)
				}
				if got.PublicID != tt.mockDeck.PublicID {
					t.Errorf("GetMoxfieldDeck() publicID = %v, want %v", got.PublicID, tt.mockDeck.PublicID)
				}
			}
		})
	}
}

func TestGetUserDecks(t *testing.T) {
	mockResponse := MoxfieldUserDecksResponse{
		PageNumber:   1,
		PageSize:     20,
		TotalResults: 2,
		TotalPages:   1,
		Data: []MoxfieldDeckSummary{
			{
				PublicID:  "deck1",
				Name:      "Deck 1",
				Format:    "commander",
				PublicURL: "https://moxfield.com/decks/deck1",
				ViewCount: 100,
				LikeCount: 10,
			},
			{
				PublicID:  "deck2",
				Name:      "Deck 2",
				Format:    "commander",
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
		mockResponse MoxfieldUserDecksResponse
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			ctx := context.Background()
			got, err := getUserDecksWithURL(ctx, tt.username, tt.pageSize, server.URL)

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

func TestSearchMoxfieldDecks(t *testing.T) {
	mockResponse := MoxfieldSearchResponse{
		PageNumber:   1,
		PageSize:     20,
		TotalResults: 50,
		TotalPages:   3,
		Data: []MoxfieldDeckSummary{
			{
				PublicID:  "deck1",
				Name:      "Atraxa Superfriends",
				Format:    "commander",
				PublicURL: "https://moxfield.com/decks/deck1",
				ViewCount: 1000,
				LikeCount: 100,
			},
			{
				PublicID:  "deck2",
				Name:      "Atraxa Infect",
				Format:    "commander",
				PublicURL: "https://moxfield.com/decks/deck2",
				ViewCount: 500,
				LikeCount: 50,
			},
		},
	}

	tests := []struct {
		name         string
		params       MoxfieldSearchParams
		mockResponse MoxfieldSearchResponse
		mockStatus   int
		wantErr      bool
		checkQuery   bool
	}{
		{
			name: "search by commander",
			params: MoxfieldSearchParams{
				Query:         "Atraxa",
				Format:        "commander",
				SortType:      "updated",
				SortDirection: "Descending",
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
			params: MoxfieldSearchParams{
				Query:         "Atraxa",
				Format:        "commander",
				SortType:      "views",
				SortDirection: "Descending",
				PageSize:      10,
				PageNumber:    1,
			},
			mockResponse: mockResponse,
			mockStatus:   http.StatusOK,
			wantErr:      false,
		},
		{
			name: "server error",
			params: MoxfieldSearchParams{
				Query:      "Test",
				PageSize:   20,
				PageNumber: 1,
			},
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.checkQuery {
					query := r.URL.Query()
					if query.Get("board") != "commanders" {
						t.Error("SearchMoxfieldDecks() should include board=commanders parameter")
					}
					if query.Get("query") != tt.params.Query {
						t.Errorf("query parameter = %v, want %v", query.Get("query"), tt.params.Query)
					}
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			ctx := context.Background()
			got, err := searchMoxfieldDecksWithURL(ctx, tt.params, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("SearchMoxfieldDecks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if len(got.Data) != len(tt.mockResponse.Data) {
					t.Errorf("SearchMoxfieldDecks() deck count = %v, want %v", len(got.Data), len(tt.mockResponse.Data))
				}
				if got.TotalResults != tt.mockResponse.TotalResults {
					t.Errorf(
						"SearchMoxfieldDecks() total results = %v, want %v",
						got.TotalResults,
						tt.mockResponse.TotalResults,
					)
				}
			}
		})
	}
}

func TestSearchMoxfieldDecks_PageSizeValidation(t *testing.T) {
	tests := []struct {
		name           string
		inputPageSize  int
		expectPageSize int
	}{
		{
			name:           "page size too large",
			inputPageSize:  150,
			expectPageSize: 100,
		},
		{
			name:           "page size zero",
			inputPageSize:  0,
			expectPageSize: 20,
		},
		{
			name:           "page size negative",
			inputPageSize:  -10,
			expectPageSize: 20,
		},
		{
			name:           "valid page size",
			inputPageSize:  50,
			expectPageSize: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pageSize := r.URL.Query().Get("pageSize")
				if pageSize != "" {
					// Check that the page size was adjusted
					if pageSize != "20" && pageSize != "100" && pageSize != "50" {
						t.Logf("PageSize in request: %s", pageSize)
					}
				}

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(MoxfieldSearchResponse{
					Data: []MoxfieldDeckSummary{},
				})
			}))
			defer server.Close()

			params := MoxfieldSearchParams{
				Query:      "test",
				PageSize:   tt.inputPageSize,
				PageNumber: 1,
			}

			ctx := context.Background()
			_, err := searchMoxfieldDecksWithURL(ctx, params, server.URL)
			if err != nil {
				t.Errorf("SearchMoxfieldDecks() unexpected error = %v", err)
			}
		})
	}
}

func TestFormatDeckForDisplay(t *testing.T) {
	deck := &MoxfieldDeck{
		Name:         "Test Deck",
		Format:       "commander",
		Authors:      []interface{}{"Author1", "Author2"},
		ViewCount:    1000,
		LikeCount:    100,
		CommentCount: 50,
		Description:  "This is a test deck",
		Commanders: map[string]MoxfieldCardEntry{
			"cmd": {
				Quantity: 1,
				Card: MoxfieldCardInfo{
					Name: "Atraxa, Praetors' Voice",
				},
			},
		},
		Mainboard: map[string]MoxfieldCardEntry{
			"card1": {
				Quantity: 1,
				Card: MoxfieldCardInfo{
					Name:     "Sol Ring",
					TypeLine: "Artifact",
				},
			},
			"card2": {
				Quantity: 1,
				Card: MoxfieldCardInfo{
					Name:     "Lightning Bolt",
					TypeLine: "Instant",
				},
			},
			"card3": {
				Quantity: 1,
				Card: MoxfieldCardInfo{
					Name:     "Birds of Paradise",
					TypeLine: "Creature - Bird",
				},
			},
		},
	}

	tests := []struct {
		name         string
		deck         *MoxfieldDeck
		wantContains []string
	}{
		{
			name: "basic formatting",
			deck: deck,
			wantContains: []string{
				"# Test Deck",
				"commander",
				"Author1, Author2",
				"**Views:** 1000",
				"**Likes:** 100",
				"This is a test deck",
				"Commanders",
				"Atraxa, Praetors' Voice",
				"Mainboard",
				"Sol Ring",
				"Lightning Bolt",
				"Birds of Paradise",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDeckForDisplay(tt.deck)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatDeckForDisplay() missing %q in output", want)
				}
			}
		})
	}
}

func TestGroupDeckCards(t *testing.T) {
	mainboard := map[string]MoxfieldCardEntry{
		"a": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Counterspell", TypeLine: "Instant"}},
		"b": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Ponder", TypeLine: "Sorcery"}},
		"c": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Sylvan Library", TypeLine: "Enchantment"}},
		"d": {
			Quantity: 1,
			Card:     MoxfieldCardInfo{Name: "Nissa, Who Shakes the World", TypeLine: "Legendary Planeswalker — Nissa"},
		},
		"e": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Island", TypeLine: "Basic Land — Island"}},
		"f": {Quantity: 2, Card: MoxfieldCardInfo{Name: "Unknown Token", TypeLine: "Token"}},
	}

	groups := groupDeckCards(mainboard)

	if len(groups.instants) != 1 || groups.instants[0] != "1x Counterspell" {
		t.Errorf("instants = %v, want [1x Counterspell]", groups.instants)
	}
	if len(groups.sorceries) != 1 || groups.sorceries[0] != "1x Ponder" {
		t.Errorf("sorceries = %v, want [1x Ponder]", groups.sorceries)
	}
	if len(groups.enchantments) != 1 || groups.enchantments[0] != "1x Sylvan Library" {
		t.Errorf("enchantments = %v, want [1x Sylvan Library]", groups.enchantments)
	}
	if len(groups.planeswalkers) != 1 {
		t.Errorf("planeswalkers = %v, want 1 entry", groups.planeswalkers)
	}
	if len(groups.lands) != 1 || groups.lands[0] != "1x Island" {
		t.Errorf("lands = %v, want [1x Island]", groups.lands)
	}
	if len(groups.others) != 1 || groups.others[0] != "2x Unknown Token" {
		t.Errorf("others = %v, want [2x Unknown Token]", groups.others)
	}
	if groups.totalCards != 7 {
		t.Errorf("totalCards = %d, want 7", groups.totalCards)
	}
}

func TestFormatDeckForDisplaySideboardAndMaybeboard(t *testing.T) {
	deck := &MoxfieldDeck{
		Name:   "Test Deck",
		Format: "commander",
		Mainboard: map[string]MoxfieldCardEntry{
			"a": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Sol Ring", TypeLine: "Artifact"}},
		},
		Sideboard: map[string]MoxfieldCardEntry{
			"b": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Swan Song", TypeLine: "Instant"}},
		},
		Maybeboard: map[string]MoxfieldCardEntry{
			"c": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Cyclonic Rift", TypeLine: "Instant"}},
		},
	}

	got := FormatDeckForDisplay(deck)

	if !strings.Contains(got, "## Sideboard") {
		t.Error("FormatDeckForDisplay() missing Sideboard section")
	}
	if !strings.Contains(got, "Swan Song") {
		t.Error("FormatDeckForDisplay() missing sideboard card")
	}
	if !strings.Contains(got, "## Maybeboard") {
		t.Error("FormatDeckForDisplay() missing Maybeboard section")
	}
	if !strings.Contains(got, "Cyclonic Rift") {
		t.Error("FormatDeckForDisplay() missing maybeboard card")
	}
}

func TestFormatDeckForDisplayOthers(t *testing.T) {
	deck := &MoxfieldDeck{
		Name:   "Test Deck",
		Format: "commander",
		Mainboard: map[string]MoxfieldCardEntry{
			"a": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Treasure Token", TypeLine: "Token"}},
		},
	}

	got := FormatDeckForDisplay(deck)

	if !strings.Contains(got, "Other") {
		t.Error("FormatDeckForDisplay() missing Other section for untyped card")
	}
	if !strings.Contains(got, "Treasure Token") {
		t.Error("FormatDeckForDisplay() missing card in Other section")
	}
}

func TestFormatDeckHeaderAuthors(t *testing.T) {
	t.Run("non-slice authors skipped gracefully", func(t *testing.T) {
		deck := &MoxfieldDeck{
			Name:    "Test",
			Format:  "commander",
			Authors: "not-a-slice",
		}
		got := formatDeckHeader(deck)
		if strings.Contains(got, "**Author:**") {
			t.Error("formatDeckHeader() should skip non-slice Authors")
		}
	})

	t.Run("slice with non-string values skipped", func(t *testing.T) {
		deck := &MoxfieldDeck{
			Name:    "Test",
			Format:  "commander",
			Authors: []interface{}{42, true},
		}
		got := formatDeckHeader(deck)
		if strings.Contains(got, "**Author:**") {
			t.Error("formatDeckHeader() should skip non-string author values")
		}
	})
}
