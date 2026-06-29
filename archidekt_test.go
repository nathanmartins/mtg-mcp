package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractArchidektDeckID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:  "full URL",
			input: "https://archidekt.com/decks/12345",
			want:  12345,
		},
		{
			name:  "URL with trailing slash",
			input: "https://archidekt.com/decks/67890/",
			want:  67890,
		},
		{
			name:  "URL with query string",
			input: "https://archidekt.com/decks/99999?tab=visual",
			want:  99999,
		},
		{
			name:  "www URL",
			input: "https://www.archidekt.com/decks/11111",
			want:  11111,
		},
		{
			name:  "numeric ID string",
			input: "42",
			want:  42,
		},
		{
			name:    "non-numeric string",
			input:   "not-a-number",
			wantErr: true,
		},
		{
			name:    "URL without numeric ID",
			input:   "https://archidekt.com/decks/abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractArchidektDeckID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractArchidektDeckID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExtractArchidektDeckID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArchidektFormatName(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{3, "Commander"},
		{1, "Standard"},
		{15, "Pioneer"},
		{999, "Unknown (999)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := archidektFormatName(tt.code)
			if got != tt.want {
				t.Errorf("archidektFormatName(%d) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestGetArchidektDeck(t *testing.T) {
	bracket := 2
	mockDeck := ArchidektDeck{
		ID:         12345,
		Name:       "Test Commander Deck",
		DeckFormat: 3,
		EdhBracket: &bracket,
		ViewCount:  500,
		UpdatedAt:  "2024-01-01T00:00:00Z",
		Owner:      ArchidektOwner{ID: 1, Username: "testuser"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Commander", IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: "Creature", IsPremier: false, IncludedInDeck: true},
		},
		Cards: []ArchidektCardEntry{
			{
				ID:         1,
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{
						Name:     "Atraxa, Praetors' Voice",
						ManaCost: "{G}{W}{U}{B}",
						Types:    []string{"Creature"},
					},
				},
			},
			{
				ID:         2,
				Quantity:   1,
				Categories: []string{"Artifact"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{
						Name:     "Sol Ring",
						ManaCost: "{1}",
						Types:    []string{"Artifact"},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		deckID     int
		mockStatus int
		mockDeck   *ArchidektDeck
		wantErr    bool
	}{
		{
			name:       "successful fetch",
			deckID:     12345,
			mockStatus: http.StatusOK,
			mockDeck:   &mockDeck,
			wantErr:    false,
		},
		{
			name:       "deck not found",
			deckID:     99999,
			mockStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "server error",
			deckID:     1,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if ua := r.Header.Get("User-Agent"); ua != "MTG-Commander-MCP-Server/1.0" {
					t.Errorf("User-Agent = %v, want MTG-Commander-MCP-Server/1.0", ua)
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK && tt.mockDeck != nil {
					_ = json.NewEncoder(w).Encode(tt.mockDeck)
				}
			}))
			defer server.Close()

			got, err := getArchidektDeckWithURL(context.Background(), tt.deckID, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetArchidektDeck() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Name != mockDeck.Name {
					t.Errorf("GetArchidektDeck() name = %v, want %v", got.Name, mockDeck.Name)
				}
				if got.ID != mockDeck.ID {
					t.Errorf("GetArchidektDeck() id = %v, want %v", got.ID, mockDeck.ID)
				}
			}
		})
	}
}

func TestGetArchidektUserDecks(t *testing.T) {
	mockResponse := ArchidektUserDecksResponse{
		Count: 2,
		Results: []ArchidektDeckSummary{
			{
				ID:         111,
				Name:       "Deck One",
				DeckFormat: 3,
				ViewCount:  100,
				UpdatedAt:  "2024-01-01T00:00:00Z",
				Owner:      ArchidektOwner{Username: "testuser"},
			},
			{
				ID:         222,
				Name:       "Deck Two",
				DeckFormat: 15,
				ViewCount:  50,
				UpdatedAt:  "2024-02-01T00:00:00Z",
				Owner:      ArchidektOwner{Username: "testuser"},
			},
		},
	}

	tests := []struct {
		name       string
		username   string
		page       int
		mockStatus int
		wantErr    bool
	}{
		{
			name:       "successful fetch",
			username:   "testuser",
			page:       1,
			mockStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "page defaults to 1 when zero",
			username:   "testuser",
			page:       0,
			mockStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server error",
			username:   "testuser",
			page:       1,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify exact owner param
				if r.URL.Query().Get("exactowner") != "true" {
					t.Error("expected exactowner=true")
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(mockResponse)
				}
			}))
			defer server.Close()

			got, err := getArchidektUserDecksWithURL(context.Background(), tt.username, tt.page, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetArchidektUserDecks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Count != mockResponse.Count {
					t.Errorf("GetArchidektUserDecks() count = %v, want %v", got.Count, mockResponse.Count)
				}
				if len(got.Results) != len(mockResponse.Results) {
					t.Errorf(
						"GetArchidektUserDecks() results len = %v, want %v",
						len(got.Results),
						len(mockResponse.Results),
					)
				}
			}
		})
	}
}

func TestFormatArchidektDeckForDisplay(t *testing.T) {
	bracket := 3
	deck := &ArchidektDeck{
		ID:         12345,
		Name:       "Hylda Control",
		DeckFormat: 3,
		EdhBracket: &bracket,
		ViewCount:  1000,
		UpdatedAt:  "2024-01-15T00:00:00Z",
		Owner:      ArchidektOwner{Username: "NorwegianWhaler"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Commander", IsPremier: true, IncludedInDeck: true},
		},
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{
						Name:  "Hylda of the Icy Crown",
						Types: []string{"Creature"},
					},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Artifact"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{
						Name:  "Sol Ring",
						Types: []string{"Artifact"},
					},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Instant"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{
						Name:  "Counterspell",
						Types: []string{"Instant"},
					},
				},
			},
		},
	}

	got := FormatArchidektDeckForDisplay(deck)

	wantContains := []string{
		"# Hylda Control",
		"Commander",
		"NorwegianWhaler",
		"1000",
		"EDH Bracket",
		"Hylda of the Icy Crown",
		"Sol Ring",
		"Counterspell",
		"https://archidekt.com/decks/12345",
	}

	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("FormatArchidektDeckForDisplay() missing %q in output", want)
		}
	}

	// Commander should NOT appear in mainboard section
	lines := strings.Split(got, "\n")
	inMainboard := false
	for _, line := range lines {
		if strings.Contains(line, "## Mainboard") {
			inMainboard = true
		}
		if inMainboard && strings.Contains(line, "Hylda of the Icy Crown") {
			t.Error("Commander card should not appear in Mainboard section")
		}
	}
}

func TestFormatArchidektLandsForDisplay(t *testing.T) {
	bracket := 4
	deck := &ArchidektDeck{
		ID:         12345,
		Name:       "Hearthhull Lands",
		DeckFormat: 3,
		EdhBracket: &bracket,
		Owner:      ArchidektOwner{Username: "testuser"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Commander", IsPremier: true, IncludedInDeck: true},
		},
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Hearthhull, the Worldseed", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Land"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Command Tower", Types: []string{"Land"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Artifact"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Sol Ring", Types: []string{"Artifact"}},
				},
			},
		},
	}

	got := FormatArchidektLandsForDisplay(deck)

	if !strings.Contains(got, "Command Tower") {
		t.Error("Expected Command Tower in lands output")
	}
	if strings.Contains(got, "Sol Ring") {
		t.Error("Sol Ring should not appear in lands-only output")
	}
	if strings.Contains(got, "Hearthhull, the Worldseed") {
		t.Error("Commander should not appear in lands-only output")
	}
	if !strings.Contains(got, "Landbase") {
		t.Error("Expected 'Landbase' heading in output")
	}
	if !strings.Contains(got, "https://archidekt.com/decks/12345") {
		t.Error("Expected deck URL in output")
	}
}

func TestFormatArchidektDeckMaybeboardSeparated(t *testing.T) {
	deck := &ArchidektDeck{
		ID:         555,
		Name:       "Maybe Deck",
		DeckFormat: 3,
		Owner:      ArchidektOwner{Username: "tester"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Commander", IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: "Creature", IsPremier: false, IncludedInDeck: true},
			{ID: 3, Name: "Maybeboard", IsPremier: false, IncludedInDeck: false},
		},
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Atraxa, Praetors' Voice", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Creature"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Llanowar Elves", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Maybeboard"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Maybe Card", Types: []string{"Sorcery"}},
				},
			},
		},
	}

	got := FormatArchidektDeckForDisplay(deck)

	if !strings.Contains(got, "## Maybeboard") {
		t.Fatalf("expected a Maybeboard section in output:\n%s", got)
	}

	mainPart, maybePart, _ := strings.Cut(got, "## Maybeboard")

	if strings.Contains(mainPart, "Maybe Card") {
		t.Error("maybeboard card should not appear in the Mainboard section")
	}
	if !strings.Contains(maybePart, "Maybe Card") {
		t.Error("maybeboard card should appear in the Maybeboard section")
	}
	if !strings.Contains(mainPart, "Llanowar Elves") {
		t.Error("mainboard creature missing from Mainboard section")
	}
	if !strings.Contains(mainPart, "**Total Cards:** 2") {
		t.Errorf("Total Cards should be 2 (commander + 1 mainboard), excluding maybeboard; got:\n%s", mainPart)
	}
}

func TestFormatArchidektLandsExcludesMaybeboard(t *testing.T) {
	deck := &ArchidektDeck{
		ID:         556,
		Name:       "Maybe Lands",
		DeckFormat: 3,
		Owner:      ArchidektOwner{Username: "tester"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Commander", IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: "Land", IsPremier: false, IncludedInDeck: true},
			{ID: 3, Name: "Maybeboard", IsPremier: false, IncludedInDeck: false},
		},
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Land"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Command Tower", Types: []string{"Land"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Maybeboard"},
				Card:       ArchidektCard{OracleCard: ArchidektOracleCard{Name: "Bojuka Bog", Types: []string{"Land"}}},
			},
		},
	}

	got := FormatArchidektLandsForDisplay(deck)

	if !strings.Contains(got, "Command Tower") {
		t.Error("mainboard land should appear in lands-only output")
	}
	if strings.Contains(got, "Bojuka Bog") {
		t.Error("maybeboard land should not appear in lands-only output")
	}
}

func TestFormatArchidektDeckSideboardSeparated(t *testing.T) {
	deck := &ArchidektDeck{
		ID:         557,
		Name:       "Side Deck",
		DeckFormat: 3,
		Owner:      ArchidektOwner{Username: "tester"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Commander", IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: "Creature", IsPremier: false, IncludedInDeck: true},
			// Archidekt flags Sideboard as includedInDeck=true, but in Commander it is out of the deck.
			{ID: 3, Name: "Sideboard", IsPremier: false, IncludedInDeck: true},
		},
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Atraxa, Praetors' Voice", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Creature"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Llanowar Elves", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Sideboard"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Pithing Needle", Types: []string{"Artifact"}},
				},
			},
			{
				// Mixed category: tagged Sideboard AND a type — must still be treated as sideboard.
				Quantity:   1,
				Categories: []string{"Sideboard", "Creature"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Scavenging Ooze", Types: []string{"Creature"}},
				},
			},
		},
	}

	got := FormatArchidektDeckForDisplay(deck)

	if !strings.Contains(got, "## Sideboard") {
		t.Fatalf("expected a Sideboard section in output:\n%s", got)
	}

	mainPart, sidePart, _ := strings.Cut(got, "## Sideboard")

	if strings.Contains(mainPart, "Pithing Needle") {
		t.Error("sideboard card should not appear in the Mainboard section")
	}
	if strings.Contains(mainPart, "Scavenging Ooze") {
		t.Error("mixed-category sideboard card should not appear in the Mainboard section")
	}
	if !strings.Contains(sidePart, "Pithing Needle") {
		t.Error("sideboard card should appear in the Sideboard section")
	}
	if !strings.Contains(sidePart, "Scavenging Ooze") {
		t.Error("mixed-category sideboard card should appear in the Sideboard section")
	}
	if !strings.Contains(mainPart, "Llanowar Elves") {
		t.Error("mainboard creature missing from Mainboard section")
	}
	if !strings.Contains(mainPart, "**Total Cards:** 2") {
		t.Errorf("Total Cards should be 2 (commander + 1 mainboard), excluding sideboard; got:\n%s", mainPart)
	}
}

func TestFormatArchidektLandsExcludesSideboard(t *testing.T) {
	deck := &ArchidektDeck{
		ID:         558,
		Name:       "Side Lands",
		DeckFormat: 3,
		Owner:      ArchidektOwner{Username: "tester"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Land", IsPremier: false, IncludedInDeck: true},
			{ID: 2, Name: "Sideboard", IsPremier: false, IncludedInDeck: true},
		},
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Land"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Command Tower", Types: []string{"Land"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Sideboard"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Reliquary Tower", Types: []string{"Land"}},
				},
			},
		},
	}

	got := FormatArchidektLandsForDisplay(deck)

	if !strings.Contains(got, "Command Tower") {
		t.Error("mainboard land should appear in lands-only output")
	}
	if strings.Contains(got, "Reliquary Tower") {
		t.Error("sideboard land should not appear in lands-only output")
	}
}

func TestFormatArchidektZonesMatchMoxfieldStyle(t *testing.T) {
	deck := &ArchidektDeck{
		ID:         559,
		Name:       "Zones Deck",
		DeckFormat: 3,
		Owner:      ArchidektOwner{Username: "tester"},
		Categories: []ArchidektCategory{
			{ID: 1, Name: "Commander", IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: "Creature", IsPremier: false, IncludedInDeck: true},
			{ID: 3, Name: "Sideboard", IsPremier: false, IncludedInDeck: true},
			{ID: 4, Name: "Maybeboard", IsPremier: false, IncludedInDeck: false},
		},
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Atraxa, Praetors' Voice", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Creature"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Llanowar Elves", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Sideboard"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Pithing Needle", Types: []string{"Artifact"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Maybeboard"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Demonic Tutor", Types: []string{"Sorcery"}},
				},
			},
		},
	}

	got := FormatArchidektDeckForDisplay(deck)

	// Moxfield style: plain headers with no "(N)" count.
	if strings.Contains(got, "## Sideboard (") || strings.Contains(got, "## Maybeboard (") {
		t.Errorf("zone headers must not include a count to match Moxfield style; got:\n%s", got)
	}
	if !strings.Contains(got, "## Sideboard\n") {
		t.Error("expected plain '## Sideboard' header")
	}
	if !strings.Contains(got, "## Maybeboard\n") {
		t.Error("expected plain '## Maybeboard' header")
	}

	// Moxfield order: Sideboard before Maybeboard.
	if strings.Index(got, "## Sideboard") > strings.Index(got, "## Maybeboard") {
		t.Error("Sideboard section should come before Maybeboard, matching Moxfield order")
	}
}

func TestSearchArchidektDecks(t *testing.T) {
	bracket4 := 4
	bracket2 := 2
	mockResponse := ArchidektUserDecksResponse{
		Count: 2,
		Results: []ArchidektDeckSummary{
			{
				ID:         1001,
				Name:       "Hearthhull Stax",
				DeckFormat: 3,
				EdhBracket: &bracket4,
				ViewCount:  9000,
				UpdatedAt:  "2025-01-01T00:00:00Z",
				Owner:      ArchidektOwner{Username: "powerplayer"},
			},
			{
				ID:         1002,
				Name:       "Hearthhull Ramp",
				DeckFormat: 3,
				EdhBracket: &bracket2,
				ViewCount:  4500,
				UpdatedAt:  "2025-02-01T00:00:00Z",
				Owner:      ArchidektOwner{Username: "casualplayer"},
			},
		},
	}

	tests := []struct {
		name         string
		commander    string
		bracket      int
		limit        int
		mockStatus   int
		wantErr      bool
		wantBracket  bool
		wantPageSize string
	}{
		{
			name:         "successful search with bracket",
			commander:    "Hearthhull, the Worldseed",
			bracket:      4,
			limit:        10,
			mockStatus:   http.StatusOK,
			wantErr:      false,
			wantBracket:  true,
			wantPageSize: "10",
		},
		{
			name:         "successful search without bracket",
			commander:    "Hearthhull, the Worldseed",
			bracket:      0,
			limit:        5,
			mockStatus:   http.StatusOK,
			wantErr:      false,
			wantBracket:  false,
			wantPageSize: "5",
		},
		{
			name:         "limit clamped to 20",
			commander:    "Atraxa",
			bracket:      0,
			limit:        99,
			mockStatus:   http.StatusOK,
			wantErr:      false,
			wantPageSize: "20",
		},
		{
			name:       "server error",
			commander:  "Atraxa",
			bracket:    0,
			limit:      10,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if ua := r.Header.Get("User-Agent"); ua != "MTG-Commander-MCP-Server/1.0" {
					t.Errorf("User-Agent = %v, want MTG-Commander-MCP-Server/1.0", ua)
				}
				q := r.URL.Query()
				if q.Get("deckFormat") != "3" {
					t.Errorf("expected deckFormat=3, got %q", q.Get("deckFormat"))
				}
				if q.Get("orderBy") != "-viewCount" {
					t.Errorf("expected orderBy=-viewCount, got %q", q.Get("orderBy"))
				}
				if tt.wantBracket && q.Get("edhBracket") == "" {
					t.Error("expected edhBracket param to be set")
				}
				if !tt.wantBracket && q.Get("edhBracket") != "" {
					t.Errorf("expected no edhBracket param, got %q", q.Get("edhBracket"))
				}
				if tt.wantPageSize != "" && q.Get("pageSize") != tt.wantPageSize {
					t.Errorf("pageSize = %q, want %q", q.Get("pageSize"), tt.wantPageSize)
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(mockResponse)
				}
			}))
			defer server.Close()

			got, err := searchArchidektDecksWithURL(
				context.Background(),
				tt.commander,
				tt.bracket,
				tt.limit,
				server.URL,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("searchArchidektDecksWithURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != nil {
				if got.Count != mockResponse.Count {
					t.Errorf("Count = %d, want %d", got.Count, mockResponse.Count)
				}
				if len(got.Results) != len(mockResponse.Results) {
					t.Errorf("Results len = %d, want %d", len(got.Results), len(mockResponse.Results))
				}
			}
		})
	}
}

func TestFormatArchidektSearchResultsForDisplay(t *testing.T) {
	bracket := 4
	result := &ArchidektUserDecksResponse{
		Count: 1,
		Results: []ArchidektDeckSummary{
			{
				ID:         9999,
				Name:       "Hearthhull Stax",
				DeckFormat: 3,
				EdhBracket: &bracket,
				ViewCount:  8500,
				UpdatedAt:  "2025-03-01T00:00:00Z",
				Owner:      ArchidektOwner{Username: "powerplayer"},
			},
		},
	}

	t.Run("with bracket filter", func(t *testing.T) {
		got := FormatArchidektSearchResultsForDisplay("Hearthhull, the Worldseed", 4, result)
		for _, want := range []string{
			"Hearthhull, the Worldseed",
			"Bracket 4",
			"Hearthhull Stax",
			"powerplayer",
			"8500",
			"**EDH Bracket:** 4",
			"https://archidekt.com/decks/9999",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in output", want)
			}
		}
	})

	t.Run("without bracket filter", func(t *testing.T) {
		got := FormatArchidektSearchResultsForDisplay("Hearthhull, the Worldseed", 0, result)
		if strings.Contains(got, "Bracket 0") {
			t.Error("bracket 0 should not appear in header")
		}
		if !strings.Contains(got, "Hearthhull, the Worldseed") {
			t.Error("missing commander name in output")
		}
	})

	t.Run("empty results", func(t *testing.T) {
		empty := &ArchidektUserDecksResponse{Count: 0, Results: nil}
		got := FormatArchidektSearchResultsForDisplay("Unknown Commander", 0, empty)
		if !strings.Contains(got, "No public decks found") {
			t.Error("expected empty-results message")
		}
	})
}

func TestGroupArchidektDeckCards(t *testing.T) {
	premier := map[string]bool{"Commander": true}
	excluded := map[string]bool{"Maybeboard": true}

	cards := []ArchidektCardEntry{
		// Commander — should be skipped
		{
			Quantity:   1,
			Categories: []string{"Commander"},
			Card:       ArchidektCard{OracleCard: ArchidektOracleCard{Name: "Atraxa", Types: []string{"Creature"}}},
		},
		// Sideboard — should be skipped
		{
			Quantity:   1,
			Categories: []string{"Sideboard"},
			Card:       ArchidektCard{OracleCard: ArchidektOracleCard{Name: "Swansong", Types: []string{"Instant"}}},
		},
		// Maybeboard (excluded) — should be skipped
		{
			Quantity:   1,
			Categories: []string{"Maybeboard"},
			Card:       ArchidektCard{OracleCard: ArchidektOracleCard{Name: "Cyclonic Rift", Types: []string{"Instant"}}},
		},
		// Unknown type — should go to others
		{
			Quantity:   1,
			Categories: []string{"Mainboard"},
			Card:       ArchidektCard{OracleCard: ArchidektOracleCard{Name: "Treasure Token", Types: []string{"Token"}}},
		},
		// Normal creature — should be counted
		{
			Quantity:   1,
			Categories: []string{"Mainboard"},
			Card:       ArchidektCard{OracleCard: ArchidektOracleCard{Name: "Birds of Paradise", Types: []string{"Creature"}}},
		},
	}

	groups := groupArchidektDeckCards(cards, premier, excluded)

	if len(groups.creatures) != 1 {
		t.Errorf("creatures = %v, want 1 entry", groups.creatures)
	}
	if len(groups.others) != 1 || groups.others[0] != "1x Treasure Token" {
		t.Errorf("others = %v, want [1x Treasure Token]", groups.others)
	}
	if groups.totalCards != 2 {
		t.Errorf("totalCards = %d, want 2 (commander + sideboard + maybeboard excluded)", groups.totalCards)
	}
}

func TestFormatArchidektDeckForDisplayWithBracket(t *testing.T) {
	bracket := 3
	deck := &ArchidektDeck{
		ID:         99,
		Name:       "Bracket Deck",
		DeckFormat: 3,
		EdhBracket: &bracket,
		Owner:      ArchidektOwner{Username: "tester"},
		Cards:      []ArchidektCardEntry{},
	}

	got := FormatArchidektDeckForDisplay(deck)

	if !strings.Contains(got, "**EDH Bracket:** 3") {
		t.Errorf("FormatArchidektDeckForDisplay() missing EDH Bracket line, got:\n%s", got)
	}
}
