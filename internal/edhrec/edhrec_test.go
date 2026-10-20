package edhrec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizeCardName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: testSolRing,
			want:  "sol-ring",
		},
		{
			name:  "name with comma",
			input: testAtraxa,
			want:  "atraxa-praetors-voice",
		},
		{
			name:  "name with apostrophe",
			input: "Jace's Ingenuity",
			want:  "jaces-ingenuity",
		},
		{
			name:  "name with special characters",
			input: "Teferi, Hero of Dominaria",
			want:  "teferi-hero-of-dominaria",
		},
		{
			name:  "name with multiple spaces",
			input: "Black   Lotus",
			want:  "black-lotus",
		},
		{
			name:  "name with hyphens",
			input: "Will-o'-the-Wisp",
			want:  "will-o-the-wisp",
		},
		{
			name:  "already sanitized",
			input: "lightning-bolt",
			want:  "lightning-bolt",
		},
		{
			name:  "all caps",
			input: "TEFERI",
			want:  "teferi",
		},
		{
			name:  "with numbers",
			input: "Mox Opal 2",
			want:  "mox-opal-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeCardName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeCardName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCommanderPage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		commanderName string
		filter        RecFilter
		mockResponse  Response
		mockStatus    int
		wantErr       bool
		checkURL      bool
		expectedURL   string
	}{
		{
			name:          "successful request",
			commanderName: testAtraxa,
			mockResponse: Response{
				Container: Container{
					JSONDict: Data{
						Card: CardInfo{
							Name:      testAtraxa,
							Sanitized: "atraxa-praetors-voice",
							ColorID:   []string{"W", "U", "B", "G"},
							NumDecks:  50000,
						},
						CardLists: []CardList{
							{
								Header: testHighSynergyHeader,
								Tag:    "highsynergy",
								CardViews: []CardView{
									{
										Name:      "Doubling Season",
										Sanitized: "doubling-season",
										NumDecks:  25000,
										Synergy:   0.35,
									},
								},
							},
						},
					},
				},
			},
			mockStatus:  http.StatusOK,
			wantErr:     false,
			checkURL:    true,
			expectedURL: "/commanders/atraxa-praetors-voice.json",
		},
		{
			name:          "404 not found",
			commanderName: "Nonexistent Commander",
			mockStatus:    http.StatusNotFound,
			wantErr:       true,
		},
		{
			name:          "500 server error",
			commanderName: "Atraxa",
			mockStatus:    http.StatusInternalServerError,
			wantErr:       true,
		},
		{
			name:          "unfiltered URL",
			commanderName: testAtraxa,
			filter:        RecFilter{},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice.json",
		},
		{
			name:          "theme filter composes URL",
			commanderName: testAtraxa,
			filter:        RecFilter{Theme: testThemeInfect},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice/infect.json",
		},
		{
			name:          "price tier filter composes URL",
			commanderName: testAtraxa,
			filter:        RecFilter{PriceTier: PriceTierBudget},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice/budget.json",
		},
		{
			name:          "theme and price tier compose in theme-then-tier order",
			commanderName: testAtraxa,
			filter:        RecFilter{Theme: testThemeInfect, PriceTier: PriceTierBudget},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice/infect/budget.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.checkURL && !strings.HasSuffix(r.URL.Path, tt.expectedURL) {
					t.Errorf("Request URL = %v, want suffix %v", r.URL.Path, tt.expectedURL)
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			ctx := t.Context()
			got, err := (&Client{BaseURL: server.URL}).GetCommanderPage(ctx, tt.commanderName, tt.filter)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCommanderPageWithURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Container.JSONDict.Card.Name != tt.mockResponse.Container.JSONDict.Card.Name {
					t.Errorf("GetCommanderPageWithURL() card name = %v, want %v",
						got.Container.JSONDict.Card.Name, tt.mockResponse.Container.JSONDict.Card.Name)
				}
			}
		})
	}
}

func TestGetSetCards(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setCode     string
		mockStatus  int
		wantErr     bool
		expectedURL string
	}{
		{
			name:        "lowercase code",
			setCode:     "rna",
			mockStatus:  http.StatusOK,
			expectedURL: testRNAPath,
		},
		{
			name:        "uppercase code is lowercased",
			setCode:     "RNA",
			mockStatus:  http.StatusOK,
			expectedURL: testRNAPath,
		},
		{
			name:        "whitespace is trimmed",
			setCode:     " rna ",
			mockStatus:  http.StatusOK,
			expectedURL: testRNAPath,
		},
		{
			name:       "403 forbidden yields an error",
			setCode:    "notaset",
			mockStatus: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.expectedURL != "" && !strings.HasSuffix(r.URL.Path, tt.expectedURL) {
					t.Errorf("Request URL = %v, want suffix %v", r.URL.Path, tt.expectedURL)
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(Response{Header: testRNAName})
				}
			}))
			defer server.Close()

			_, err := (&Client{BaseURL: server.URL}).GetSetCards(t.Context(), tt.setCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSetCardsWithURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetCombosForColors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		colors       string
		mockResponse ComboResponse
		mockStatus   int
		wantErr      bool
	}{
		{
			name:   "colorless combos",
			colors: testColorless,
			mockResponse: ComboResponse{
				Container: ComboContainer{
					JSONDict: ComboData{
						CardLists: []ComboList{
							{
								Header: "Basalt Monolith + Forsaken Monument",
								CardViews: []CardView{
									{Name: "Basalt Monolith"},
									{Name: "Forsaken Monument"},
								},
								Combo: &Combo{
									ComboID: "combo-1",
									Results: []string{"Infinite colorless mana"},
								},
							},
						},
					},
				},
			},
			mockStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "404 not found",
			colors:     "wu",
			mockStatus: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			ctx := t.Context()
			got, err := (&Client{BaseURL: server.URL}).GetCombosForColors(ctx, tt.colors)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCombosForColors() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if len(got.CardLists) != len(tt.mockResponse.Container.JSONDict.CardLists) {
					t.Errorf("GetCombosForColors() combo count = %v, want %v",
						len(got.CardLists), len(tt.mockResponse.Container.JSONDict.CardLists))
				}
			}
		})
	}
}

func TestFormatCommanderRecs(t *testing.T) {
	t.Parallel()
	page := &Response{
		Container: Container{
			JSONDict: Data{
				Card: CardInfo{
					Name:     testCommander,
					ColorID:  []string{"W", "U"},
					NumDecks: 1000,
				},
				CardLists: []CardList{
					{
						Header: testHighSynergyHeader,
						CardViews: []CardView{
							{
								Name:     testCard1,
								NumDecks: 500,
								Synergy:  0.35,
								Salt:     1.5,
							},
							{
								Name:     "Card 2",
								NumDecks: 400,
								Synergy:  0.25,
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		page          *Response
		limit         int
		wantContains  []string
		wantCardCount int
	}{
		{
			name:  "with limit",
			page:  page,
			limit: 1,
			wantContains: []string{testCommander, testHighSynergyHeader, testCard1, "Synergy:",
				"Salt Score:",
			},
			wantCardCount: 1,
		},
		{
			name:          "without limit",
			page:          page,
			limit:         0,
			wantContains:  []string{testCommander, testCard1, "Card 2"},
			wantCardCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatCommanderRecs(tt.page, RecFilter{}, tt.limit)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatCommanderRecs() missing %q in output", want)
				}
			}

			// Check if the correct number of cards are shown
			if tt.limit > 0 {
				card2Count := strings.Count(got, "Card 2")
				if tt.wantCardCount == 1 && card2Count > 0 {
					t.Error("FormatCommanderRecs() should limit cards but found Card 2")
				}
			}
		})
	}
}

func TestFormatCommanderRecsDeckStats(t *testing.T) {
	t.Parallel()
	t.Run("nonzero total decks shows percentage", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: testCommander, NumDecks: 1000},
					CardLists: []CardList{
						{Header: testTopCardsHeader, CardViews: []CardView{{Name: testSolRing, NumDecks: 500}}},
					},
				},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{}, 10)
		if !strings.Contains(got, "Decks: 500 of 1000 (50.0%)") {
			t.Errorf("expected 50.0%% deck stat, got:\n%s", got)
		}
		if strings.Contains(got, "NaN") {
			t.Errorf("output must never contain NaN, got:\n%s", got)
		}
	})

	t.Run("zero total decks shows raw count only", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: testCommander, NumDecks: 0},
					CardLists: []CardList{
						{Header: testTopCardsHeader, CardViews: []CardView{{Name: testSolRing, NumDecks: 500}}},
					},
				},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{}, 10)
		if !strings.Contains(got, "Decks: 500") {
			t.Errorf("expected raw deck count, got:\n%s", got)
		}
		if strings.Contains(got, "NaN") || strings.Contains(got, "%") {
			t.Errorf("zero denominator must not print a percentage, got:\n%s", got)
		}
	})
}

func TestFormatCommanderRecsThemes(t *testing.T) {
	t.Parallel()
	tags := make([]TagCount, 20)
	for i := range tags {
		tags[i] = TagCount{
			Slug: fmt.Sprintf("theme-%02d", i), Value: fmt.Sprintf("Theme %02d", i), Count: 100 - i,
		}
	}

	page := &Response{
		Header:    "Test Commander (Commander)",
		TagCounts: tags,
		Container: Container{
			JSONDict: Data{Card: CardInfo{Name: testCommander}},
		},
	}

	t.Run("unfiltered shows available themes", func(t *testing.T) {
		t.Parallel()
		got := FormatCommanderRecs(page, RecFilter{}, 10)
		if !strings.Contains(got, "## Available Themes") {
			t.Error("expected Available Themes section")
		}
		if !strings.Contains(got, "theme-00") {
			t.Error("expected highest-count theme slug")
		}
		if !strings.Contains(got, "and 5 more themes") {
			t.Errorf("expected omitted-theme count, got:\n%s", got)
		}
		if strings.Contains(got, "theme-15") {
			t.Error("16th theme slug should not be listed")
		}
	})

	t.Run("filtered suppresses available themes", func(t *testing.T) {
		t.Parallel()
		got := FormatCommanderRecs(page, RecFilter{Theme: testThemeInfect}, 10)
		if strings.Contains(got, "## Available Themes") {
			t.Error("Available Themes section should be suppressed when a filter is active")
		}
	})
}

func TestFormatCommanderRecsHeader(t *testing.T) {
	t.Parallel()
	t.Run("uses page header when present", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Header: "X (Commander) - Budget Infect",
			Container: Container{
				JSONDict: Data{Card: CardInfo{Name: "X"}},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{Theme: testThemeInfect, PriceTier: PriceTierBudget}, 10)
		if !strings.Contains(got, "X (Commander) - Budget Infect") {
			t.Errorf("expected page header in title, got:\n%s", got)
		}
	})

	t.Run("falls back to card name when header is empty", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Container: Container{
				JSONDict: Data{Card: CardInfo{Name: "Fallback Commander"}},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{}, 10)
		if !strings.Contains(got, "Fallback Commander") {
			t.Errorf("expected card name fallback in title, got:\n%s", got)
		}
	})
}

func TestFormatSetCards(t *testing.T) {
	t.Parallel()
	t.Run("commander and card views", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Header: testRNAName,
			Container: Container{
				JSONDict: Data{
					CardLists: []CardList{
						{
							Header:    "Commanders",
							CardViews: []CardView{{Name: "Teysa Karlov", NumDecks: 21469, PotentialDecks: 0}},
						},
						{
							Header: "Cards",
							CardViews: []CardView{
								{Name: "Smothering Tithe", NumDecks: 500, PotentialDecks: 2000},
							},
						},
					},
				},
			},
		}
		got := FormatSetCards(page, "rna", 25)
		if !strings.Contains(got, "(rna)") {
			t.Errorf("expected set code in title, got:\n%s", got)
		}
		if strings.Contains(got, "Decks: 21469 of") || !strings.Contains(got, "Decks: 21469\n") {
			t.Errorf("commander view should show a bare deck count, got:\n%s", got)
		}
		if !strings.Contains(got, "Decks: 500 of 2000 (25.0%)") {
			t.Errorf("card view should show a percentage, got:\n%s", got)
		}
	})

	t.Run("truncation and unlimited", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Container: Container{
				JSONDict: Data{
					CardLists: []CardList{
						{
							Header: "Cards",
							CardViews: []CardView{
								{Name: "Card One"},
								{Name: "Card Two"},
								{Name: "Card Three"},
							},
						},
					},
				},
			},
		}

		limited := FormatSetCards(page, "rna", 1)
		if !strings.Contains(limited, "and 2 more cards") {
			t.Errorf("expected truncation suffix, got:\n%s", limited)
		}

		unlimited := FormatSetCards(page, "rna", 0)
		for _, want := range []string{"Card One", "Card Two", "Card Three"} {
			if !strings.Contains(unlimited, want) {
				t.Errorf("limit 0 should show all cards, missing %q in:\n%s", want, unlimited)
			}
		}
		if strings.Contains(unlimited, "more cards") {
			t.Errorf("limit 0 should not truncate, got:\n%s", unlimited)
		}
	})
}

func TestFormatCombos(t *testing.T) {
	t.Parallel()
	data := &ComboData{
		CardLists: []ComboList{
			{
				Header: "Card A + Card B (1000 decks)",
				CardViews: []CardView{
					{Name: "Card A"},
					{Name: "Card B"},
				},
				Combo: &Combo{
					ComboID: "123-456",
					Results: []string{"Win the game", "Infinite mana"},
				},
			},
			{
				Header: "Card C + Card D (500 decks)",
				CardViews: []CardView{
					{Name: "Card C"},
					{Name: "Card D"},
				},
				Combo: &Combo{
					ComboID: "789-012",
					Results: []string{"Infinite tokens"},
				},
			},
		},
	}

	tests := []struct {
		name         string
		data         *ComboData
		limit        int
		wantContains []string
	}{
		{
			name:  "with limit",
			data:  data,
			limit: 1,
			wantContains: []string{
				"Popular Combos",
				"Card A + Card B",
				"1000 decks",
				"Win the game",
			},
		},
		{
			name:  "without limit",
			data:  data,
			limit: 0,
			wantContains: []string{
				"Card A + Card B",
				"Card C + Card D",
				"500 decks",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatCombos(tt.data, tt.limit)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatCombos() missing %q in output", want)
				}
			}

			if tt.limit == 1 && strings.Contains(got, "Card C + Card D") {
				t.Error("FormatCombos() should limit combos but found second combo")
			}
		})
	}
}

func TestFormatCommanderRecsEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("empty card list skipped", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: testCommander, NumDecks: 100},
					CardLists: []CardList{
						{Header: "Skipped Section", CardViews: []CardView{}},
						{Header: "Non-Empty Section", CardViews: []CardView{
							{Name: testSolRing, NumDecks: 90},
						}},
					},
				},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{}, 10)
		if strings.Contains(got, "Skipped Section") {
			t.Error("FormatCommanderRecs() should skip card lists with no cards")
		}
		if !strings.Contains(got, "Non-Empty Section") {
			t.Error("FormatCommanderRecs() missing non-empty section")
		}
	})

	t.Run("zero synergy and zero salt omitted", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: testCommander, NumDecks: 100},
					CardLists: []CardList{
						{Header: testTopCardsHeader, CardViews: []CardView{
							{Name: testSolRing, NumDecks: 90, Synergy: 0, Salt: 0},
						}},
					},
				},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{}, 10)
		if strings.Contains(got, "Synergy:") {
			t.Error("FormatCommanderRecs() should omit Synergy line when zero")
		}
		if strings.Contains(got, "Salt Score:") {
			t.Error("FormatCommanderRecs() should omit Salt Score line when zero")
		}
	})

	t.Run("truncation suffix shown", func(t *testing.T) {
		t.Parallel()
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: testCommander, NumDecks: 100},
					CardLists: []CardList{
						{Header: testTopCardsHeader, CardViews: []CardView{
							{Name: testSolRing, NumDecks: 90},
							{Name: "Arcane Signet", NumDecks: 80},
							{Name: "Command Tower", NumDecks: 70},
						}},
					},
				},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{}, 1)
		if !strings.Contains(got, "and 2 more cards") {
			t.Errorf("FormatCommanderRecs() missing truncation suffix, got:\n%s", got)
		}
	})
}

func TestComboPageName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		colors  string
		want    string
		wantErr bool
	}{
		{colors: "wu", want: testAzorius},
		{colors: "UW", want: testAzorius},
		{colors: " gw ", want: "selesnya"},
		{colors: "rbu", want: "grixis"},
		{colors: "gbrw", want: "dune-brood"},
		{colors: "wubrg", want: "five-color"},
		{colors: "w", want: "mono-white"},
		{colors: "c", want: testColorless},
		{colors: testColorless, want: testColorless},
		{colors: "Azorius", want: testAzorius},
		{colors: "yore-tiller", want: "yore-tiller"},
		{colors: "wuu", want: testAzorius},
		{colors: "", wantErr: true},
		{colors: "zz", wantErr: true},
		{colors: "wc", wantErr: true},
		{colors: "white", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ComboPageName(tt.colors)
		if (err != nil) != tt.wantErr {
			t.Errorf("ComboPageName(%q) error = %v, wantErr %v", tt.colors, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ComboPageName(%q) = %q, want %q", tt.colors, got, tt.want)
		}
	}
}

func TestGetCombosForColorsRequestsNamedPage(t *testing.T) {
	t.Parallel()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(ComboResponse{})
	}))
	defer server.Close()

	if _, err := (&Client{BaseURL: server.URL}).GetCombosForColors(t.Context(), "UW"); err != nil {
		t.Fatalf("GetCombosForColorsWithURL() error = %v", err)
	}
	if gotPath != "/combos/azorius.json" {
		t.Errorf("requested %q, want /combos/azorius.json", gotPath)
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()
	if got := NewClient().BaseURL; got != DefaultBaseURL {
		t.Errorf("NewClient().BaseURL = %q, want %q", got, DefaultBaseURL)
	}
}
