package archidekt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractDeckID(t *testing.T) {
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
			got, err := ExtractDeckID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractDeckID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExtractDeckID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatName(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{3, archidektCommanderCategory},
		{1, "Standard"},
		{15, "Pioneer"},
		{999, "Unknown (999)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatName(tt.code)
			if got != tt.want {
				t.Errorf("FormatName(%d) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestGetDeck(t *testing.T) {
	mockDeck := Deck{
		ID:         12345,
		Name:       "Test Commander Deck",
		DeckFormat: 3,
		EdhBracket: new(2),
		ViewCount:  500,
		UpdatedAt:  "2024-01-01T00:00:00Z",
		Owner:      Owner{ID: 1, Username: testUser},
		Categories: []Category{
			{ID: 1, Name: archidektCommanderCategory, IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: testCreature, IsPremier: false, IncludedInDeck: true},
		},
		Cards: []CardEntry{
			{
				ID:         1,
				Quantity:   1,
				Categories: []string{archidektCommanderCategory},
				Card: Card{
					OracleCard: OracleCard{
						Name:     testAtraxa,
						ManaCost: "{G}{W}{U}{B}",
						Types:    []string{testCreature},
					},
				},
			},
			{
				ID:         2,
				Quantity:   1,
				Categories: []string{testArtifact},
				Card: Card{
					OracleCard: OracleCard{
						Name:     testSolRing,
						ManaCost: "{1}",
						Types:    []string{testArtifact},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		deckID     int
		mockStatus int
		mockDeck   *Deck
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

			got, err := (&Client{BaseURL: server.URL}).GetDeck(t.Context(), tt.deckID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeck() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Name != mockDeck.Name {
					t.Errorf("GetDeck() name = %v, want %v", got.Name, mockDeck.Name)
				}
				if got.ID != mockDeck.ID {
					t.Errorf("GetDeck() id = %v, want %v", got.ID, mockDeck.ID)
				}
			}
		})
	}
}

func TestGetUserDecks(t *testing.T) {
	mockResponse := UserDecksResponse{
		Count: 2,
		Results: []DeckSummary{
			{
				ID:         111,
				Name:       "Deck One",
				DeckFormat: 3,
				ViewCount:  100,
				UpdatedAt:  "2024-01-01T00:00:00Z",
				Owner:      Owner{Username: testUser},
			},
			{
				ID:         222,
				Name:       "Deck Two",
				DeckFormat: 15,
				ViewCount:  50,
				UpdatedAt:  "2024-02-01T00:00:00Z",
				Owner:      Owner{Username: testUser},
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
			username:   testUser,
			page:       1,
			mockStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "page defaults to 1 when zero",
			username:   testUser,
			page:       0,
			mockStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server error",
			username:   testUser,
			page:       1,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the owner filter Archidekt honors; "owner" is ignored upstream
				q := r.URL.Query()
				if q.Get("ownerUsername") != tt.username {
					t.Errorf("ownerUsername = %q, want %q", q.Get("ownerUsername"), tt.username)
				}
				if q.Has(testOwner) || q.Has("exactowner") {
					t.Error("owner/exactowner are ignored by Archidekt and must not be sent")
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(mockResponse)
				}
			}))
			defer server.Close()

			got, err := (&Client{BaseURL: server.URL}).GetUserDecks(t.Context(), tt.username, tt.page)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserDecks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Count != mockResponse.Count {
					t.Errorf("GetUserDecks() count = %v, want %v", got.Count, mockResponse.Count)
				}
				if len(got.Results) != len(mockResponse.Results) {
					t.Errorf(
						"GetUserDecks() results len = %v, want %v",
						len(got.Results),
						len(mockResponse.Results),
					)
				}
			}
		})
	}
}

func TestFormatDeck(t *testing.T) {
	deck := &Deck{
		ID:         12345,
		Name:       "Hylda Control",
		DeckFormat: 3,
		EdhBracket: new(3),
		ViewCount:  1000,
		UpdatedAt:  "2024-01-15T00:00:00Z",
		Owner:      Owner{Username: testOwnerName},
		Categories: []Category{
			{ID: 1, Name: archidektCommanderCategory, IsPremier: true, IncludedInDeck: true},
		},
		Cards: []CardEntry{
			{
				Quantity:   1,
				Categories: []string{archidektCommanderCategory},
				Card: Card{
					OracleCard: OracleCard{
						Name:  "Hylda of the Icy Crown",
						Types: []string{testCreature},
					},
				},
			},
			{
				Quantity:   1,
				Categories: []string{testArtifact},
				Card: Card{
					OracleCard: OracleCard{
						Name:  testSolRing,
						Types: []string{testArtifact},
					},
				},
			},
			{
				Quantity:   1,
				Categories: []string{testInstant},
				Card: Card{
					OracleCard: OracleCard{
						Name:  "Counterspell",
						Types: []string{testInstant},
					},
				},
			},
		},
	}

	got := FormatDeck(deck)

	wantContains := []string{
		"# Hylda Control", archidektCommanderCategory, testOwnerName, "1000",
		"EDH Bracket",
		"Hylda of the Icy Crown", testSolRing, "Counterspell",
		"https://archidekt.com/decks/12345",
	}

	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("FormatDeck() missing %q in output", want)
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

func TestFormatLands(t *testing.T) {
	deck := &Deck{
		ID:         12345,
		Name:       "Hearthhull Lands",
		DeckFormat: 3,
		EdhBracket: new(4),
		Owner:      Owner{Username: testUser},
		Categories: []Category{
			{ID: 1, Name: archidektCommanderCategory, IsPremier: true, IncludedInDeck: true},
		},
		Cards: []CardEntry{
			{
				Quantity:   1,
				Categories: []string{archidektCommanderCategory},
				Card: Card{
					OracleCard: OracleCard{Name: "Hearthhull, the Worldseed", Types: []string{testCreature}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{testLand},
				Card: Card{
					OracleCard: OracleCard{Name: testCommandTower, Types: []string{testLand}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{testArtifact},
				Card: Card{
					OracleCard: OracleCard{Name: testSolRing, Types: []string{testArtifact}},
				},
			},
		},
	}

	got := FormatLands(deck)

	if !strings.Contains(got, testCommandTower) {
		t.Error("Expected Command Tower in lands output")
	}
	if strings.Contains(got, testSolRing) {
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

func TestFormatDeckMaybeboardSeparated(t *testing.T) {
	deck := &Deck{
		ID:         555,
		Name:       "Maybe Deck",
		DeckFormat: 3,
		Owner:      Owner{Username: testUsername},
		Categories: []Category{
			{ID: 1, Name: archidektCommanderCategory, IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: testCreature, IsPremier: false, IncludedInDeck: true},
			{ID: 3, Name: archidektMaybeboardZone, IsPremier: false, IncludedInDeck: false},
		},
		Cards: []CardEntry{
			{
				Quantity:   1,
				Categories: []string{archidektCommanderCategory},
				Card: Card{
					OracleCard: OracleCard{Name: testAtraxa, Types: []string{testCreature}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{testCreature},
				Card: Card{
					OracleCard: OracleCard{Name: testLlanowarElves, Types: []string{testCreature}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{archidektMaybeboardZone},
				Card: Card{
					OracleCard: OracleCard{Name: "Maybe Card", Types: []string{"Sorcery"}},
				},
			},
		},
	}

	got := FormatDeck(deck)

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
	if !strings.Contains(mainPart, testLlanowarElves) {
		t.Error("mainboard creature missing from Mainboard section")
	}
	if !strings.Contains(mainPart, "**Total Cards:** 2") {
		t.Errorf("Total Cards should be 2 (commander + 1 mainboard), excluding maybeboard; got:\n%s", mainPart)
	}
}

func TestFormatLandsExcludesMaybeboard(t *testing.T) {
	deck := &Deck{
		ID:         556,
		Name:       "Maybe Lands",
		DeckFormat: 3,
		Owner:      Owner{Username: testUsername},
		Categories: []Category{
			{ID: 1, Name: archidektCommanderCategory, IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: testLand, IsPremier: false, IncludedInDeck: true},
			{ID: 3, Name: archidektMaybeboardZone, IsPremier: false, IncludedInDeck: false},
		},
		Cards: []CardEntry{
			{
				Quantity:   1,
				Categories: []string{testLand},
				Card: Card{
					OracleCard: OracleCard{Name: testCommandTower, Types: []string{testLand}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{archidektMaybeboardZone},
				Card:       Card{OracleCard: OracleCard{Name: "Bojuka Bog", Types: []string{testLand}}},
			},
		},
	}

	got := FormatLands(deck)

	if !strings.Contains(got, testCommandTower) {
		t.Error("mainboard land should appear in lands-only output")
	}
	if strings.Contains(got, "Bojuka Bog") {
		t.Error("maybeboard land should not appear in lands-only output")
	}
}

func TestFormatDeckSideboardSeparated(t *testing.T) {
	deck := &Deck{
		ID:         557,
		Name:       "Side Deck",
		DeckFormat: 3,
		Owner:      Owner{Username: testUsername},
		Categories: []Category{
			{ID: 1, Name: archidektCommanderCategory, IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: testCreature, IsPremier: false, IncludedInDeck: true},
			// Archidekt flags Sideboard as includedInDeck=true, but in Commander it is out of the deck.
			{ID: 3, Name: archidektSideboardZone, IsPremier: false, IncludedInDeck: true},
		},
		Cards: []CardEntry{
			{
				Quantity:   1,
				Categories: []string{archidektCommanderCategory},
				Card: Card{
					OracleCard: OracleCard{Name: testAtraxa, Types: []string{testCreature}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{testCreature},
				Card: Card{
					OracleCard: OracleCard{Name: testLlanowarElves, Types: []string{testCreature}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{archidektSideboardZone},
				Card: Card{
					OracleCard: OracleCard{Name: "Pithing Needle", Types: []string{testArtifact}},
				},
			},
			{
				// Mixed category: tagged Sideboard AND a type — must still be treated as sideboard.
				Quantity:   1,
				Categories: []string{archidektSideboardZone, testCreature},
				Card: Card{
					OracleCard: OracleCard{Name: "Scavenging Ooze", Types: []string{testCreature}},
				},
			},
		},
	}

	got := FormatDeck(deck)

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
	if !strings.Contains(mainPart, testLlanowarElves) {
		t.Error("mainboard creature missing from Mainboard section")
	}
	if !strings.Contains(mainPart, "**Total Cards:** 2") {
		t.Errorf("Total Cards should be 2 (commander + 1 mainboard), excluding sideboard; got:\n%s", mainPart)
	}
}

func TestFormatLandsExcludesSideboard(t *testing.T) {
	deck := &Deck{
		ID:         558,
		Name:       "Side Lands",
		DeckFormat: 3,
		Owner:      Owner{Username: testUsername},
		Categories: []Category{
			{ID: 1, Name: testLand, IsPremier: false, IncludedInDeck: true},
			{ID: 2, Name: archidektSideboardZone, IsPremier: false, IncludedInDeck: true},
		},
		Cards: []CardEntry{
			{
				Quantity:   1,
				Categories: []string{testLand},
				Card: Card{
					OracleCard: OracleCard{Name: testCommandTower, Types: []string{testLand}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{archidektSideboardZone},
				Card: Card{
					OracleCard: OracleCard{Name: "Reliquary Tower", Types: []string{testLand}},
				},
			},
		},
	}

	got := FormatLands(deck)

	if !strings.Contains(got, testCommandTower) {
		t.Error("mainboard land should appear in lands-only output")
	}
	if strings.Contains(got, "Reliquary Tower") {
		t.Error("sideboard land should not appear in lands-only output")
	}
}

func TestFormatZonesMatchMoxfieldStyle(t *testing.T) {
	deck := &Deck{
		ID:         559,
		Name:       "Zones Deck",
		DeckFormat: 3,
		Owner:      Owner{Username: testUsername},
		Categories: []Category{
			{ID: 1, Name: archidektCommanderCategory, IsPremier: true, IncludedInDeck: true},
			{ID: 2, Name: testCreature, IsPremier: false, IncludedInDeck: true},
			{ID: 3, Name: archidektSideboardZone, IsPremier: false, IncludedInDeck: true},
			{ID: 4, Name: archidektMaybeboardZone, IsPremier: false, IncludedInDeck: false},
		},
		Cards: []CardEntry{
			{
				Quantity:   1,
				Categories: []string{archidektCommanderCategory},
				Card: Card{
					OracleCard: OracleCard{Name: testAtraxa, Types: []string{testCreature}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{testCreature},
				Card: Card{
					OracleCard: OracleCard{Name: testLlanowarElves, Types: []string{testCreature}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{archidektSideboardZone},
				Card: Card{
					OracleCard: OracleCard{Name: "Pithing Needle", Types: []string{testArtifact}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{archidektMaybeboardZone},
				Card: Card{
					OracleCard: OracleCard{Name: "Demonic Tutor", Types: []string{"Sorcery"}},
				},
			},
		},
	}

	got := FormatDeck(deck)

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

func TestGroupDeckCards(t *testing.T) {
	premier := map[string]bool{archidektCommanderCategory: true}
	excluded := map[string]bool{archidektMaybeboardZone: true}

	cards := []CardEntry{
		// Commander — should be skipped
		{
			Quantity:   1,
			Categories: []string{archidektCommanderCategory},
			Card:       Card{OracleCard: OracleCard{Name: testAtraxaShort, Types: []string{testCreature}}},
		},
		// Sideboard — should be skipped
		{
			Quantity:   1,
			Categories: []string{archidektSideboardZone},
			Card:       Card{OracleCard: OracleCard{Name: "Swansong", Types: []string{testInstant}}},
		},
		// Maybeboard (excluded) — should be skipped
		{
			Quantity:   1,
			Categories: []string{archidektMaybeboardZone},
			Card: Card{
				OracleCard: OracleCard{Name: "Cyclonic Rift", Types: []string{testInstant}},
			},
		},
		// Unknown type — should go to others
		{
			Quantity:   1,
			Categories: []string{"Mainboard"},
			Card: Card{
				OracleCard: OracleCard{Name: "Treasure Token", Types: []string{"Token"}},
			},
		},
		// Normal creature — should be counted
		{
			Quantity:   1,
			Categories: []string{"Mainboard"},
			Card: Card{
				OracleCard: OracleCard{Name: "Birds of Paradise", Types: []string{testCreature}},
			},
		},
	}

	groups := groupArchidektDeckCards(cards, premier, excluded)

	if len(groups.Creatures) != 1 {
		t.Errorf("creatures = %v, want 1 entry", groups.Creatures)
	}
	if len(groups.Others) != 1 || groups.Others[0] != "1x Treasure Token" {
		t.Errorf("others = %v, want [1x Treasure Token]", groups.Others)
	}
	if groups.TotalCards != 2 {
		t.Errorf("totalCards = %d, want 2 (commander + sideboard + maybeboard excluded)", groups.TotalCards)
	}
}

func TestFormatDeckWithBracket(t *testing.T) {
	deck := &Deck{
		ID:         99,
		Name:       "Bracket Deck",
		DeckFormat: 3,
		EdhBracket: new(3),
		Owner:      Owner{Username: testUsername},
		Cards:      []CardEntry{},
	}

	got := FormatDeck(deck)

	if !strings.Contains(got, "**EDH Bracket:** 3") {
		t.Errorf("FormatDeck() missing EDH Bracket line, got:\n%s", got)
	}
}

func TestNewClient(t *testing.T) {
	if got := NewClient().BaseURL; got != DefaultBaseURL {
		t.Errorf("NewClient().BaseURL = %q, want %q", got, DefaultBaseURL)
	}
}
