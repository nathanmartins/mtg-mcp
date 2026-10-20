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
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: "Sol Ring",
			want:  "sol-ring",
		},
		{
			name:  "name with comma",
			input: "Atraxa, Praetors' Voice",
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
			got := SanitizeCardName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeCardName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCommanderRecommendations(t *testing.T) {
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
			commanderName: "Atraxa, Praetors' Voice",
			mockResponse: Response{
				Container: Container{
					JSONDict: Data{
						Card: CardInfo{
							Name:      "Atraxa, Praetors' Voice",
							Sanitized: "atraxa-praetors-voice",
							ColorID:   []string{"W", "U", "B", "G"},
							NumDecks:  50000,
						},
						CardLists: []CardList{
							{
								Header: "High Synergy Cards",
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
			commanderName: "Atraxa, Praetors' Voice",
			filter:        RecFilter{},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice.json",
		},
		{
			name:          "theme filter composes URL",
			commanderName: "Atraxa, Praetors' Voice",
			filter:        RecFilter{Theme: "infect"},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice/infect.json",
		},
		{
			name:          "price tier filter composes URL",
			commanderName: "Atraxa, Praetors' Voice",
			filter:        RecFilter{PriceTier: "budget"},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice/budget.json",
		},
		{
			name:          "theme and price tier compose in theme-then-tier order",
			commanderName: "Atraxa, Praetors' Voice",
			filter:        RecFilter{Theme: "infect", PriceTier: "budget"},
			mockStatus:    http.StatusOK,
			checkURL:      true,
			expectedURL:   "/commanders/atraxa-praetors-voice/infect/budget.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			got, err := GetCommanderPageWithURL(ctx, tt.commanderName, tt.filter, server.URL)

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

func TestGetSetCardsWithURL(t *testing.T) {
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
			expectedURL: "/sets/rna.json",
		},
		{
			name:        "uppercase code is lowercased",
			setCode:     "RNA",
			mockStatus:  http.StatusOK,
			expectedURL: "/sets/rna.json",
		},
		{
			name:        "whitespace is trimmed",
			setCode:     " rna ",
			mockStatus:  http.StatusOK,
			expectedURL: "/sets/rna.json",
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.expectedURL != "" && !strings.HasSuffix(r.URL.Path, tt.expectedURL) {
					t.Errorf("Request URL = %v, want suffix %v", r.URL.Path, tt.expectedURL)
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(Response{Header: "Ravnica Allegiance"})
				}
			}))
			defer server.Close()

			_, err := GetSetCardsWithURL(t.Context(), tt.setCode, server.URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSetCardsWithURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetCombosForColors(t *testing.T) {
	tests := []struct {
		name         string
		colors       string
		mockResponse ComboResponse
		mockStatus   int
		wantErr      bool
	}{
		{
			name:   "colorless combos",
			colors: "colorless",
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
			colors:     "invalid",
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

			ctx := t.Context()
			got, err := GetCombosForColorsWithURL(ctx, tt.colors, server.URL)

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

func TestFormatCommanderRecsForDisplay(t *testing.T) {
	page := &Response{
		Container: Container{
			JSONDict: Data{
				Card: CardInfo{
					Name:     "Test Commander",
					ColorID:  []string{"W", "U"},
					NumDecks: 1000,
				},
				CardLists: []CardList{
					{
						Header: "High Synergy Cards",
						CardViews: []CardView{
							{
								Name:     "Card 1",
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
			wantContains: []string{
				"Test Commander",
				"High Synergy Cards",
				"Card 1",
				"Synergy:",
				"Salt Score:",
			},
			wantCardCount: 1,
		},
		{
			name:  "without limit",
			page:  page,
			limit: 0,
			wantContains: []string{
				"Test Commander",
				"Card 1",
				"Card 2",
			},
			wantCardCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestFormatCommanderRecsForDisplayDeckStats(t *testing.T) {
	t.Run("nonzero total decks shows percentage", func(t *testing.T) {
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: "Test Commander", NumDecks: 1000},
					CardLists: []CardList{
						{Header: "Top Cards", CardViews: []CardView{{Name: "Sol Ring", NumDecks: 500}}},
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
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: "Test Commander", NumDecks: 0},
					CardLists: []CardList{
						{Header: "Top Cards", CardViews: []CardView{{Name: "Sol Ring", NumDecks: 500}}},
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

func TestFormatCommanderRecsForDisplayThemes(t *testing.T) {
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
			JSONDict: Data{Card: CardInfo{Name: "Test Commander"}},
		},
	}

	t.Run("unfiltered shows available themes", func(t *testing.T) {
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
		got := FormatCommanderRecs(page, RecFilter{Theme: "infect"}, 10)
		if strings.Contains(got, "## Available Themes") {
			t.Error("Available Themes section should be suppressed when a filter is active")
		}
	})
}

func TestFormatCommanderRecsForDisplayHeader(t *testing.T) {
	t.Run("uses page header when present", func(t *testing.T) {
		page := &Response{
			Header: "X (Commander) - Budget Infect",
			Container: Container{
				JSONDict: Data{Card: CardInfo{Name: "X"}},
			},
		}
		got := FormatCommanderRecs(page, RecFilter{Theme: "infect", PriceTier: "budget"}, 10)
		if !strings.Contains(got, "X (Commander) - Budget Infect") {
			t.Errorf("expected page header in title, got:\n%s", got)
		}
	})

	t.Run("falls back to card name when header is empty", func(t *testing.T) {
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

func TestFormatSetCardsForDisplay(t *testing.T) {
	t.Run("commander and card views", func(t *testing.T) {
		page := &Response{
			Header: "Ravnica Allegiance",
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

func TestFormatCombosForDisplay(t *testing.T) {
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

func TestGetTopCardsForCategory(t *testing.T) {
	tests := []struct {
		name       string
		category   string
		page       int
		mockStatus int
		mockResp   Response
		wantCards  int
		wantErr    bool
	}{
		{
			name:       "successful request flattens card lists",
			category:   "salt",
			page:       0,
			mockStatus: http.StatusOK,
			mockResp: Response{
				Container: Container{
					JSONDict: Data{
						CardLists: []CardList{
							{
								Header: "Saltiest Cards",
								CardViews: []CardView{
									{Name: "Armageddon"},
									{Name: "Stasis"},
								},
							},
							{
								Header: "More Salt",
								CardViews: []CardView{
									{Name: "Winter Orb"},
								},
							},
						},
					},
				},
			},
			wantCards: 3,
			wantErr:   false,
		},
		{
			name:       "404 not found",
			category:   "missing",
			page:       1,
			mockStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "500 server error",
			category:   "salt",
			page:       0,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResp)
				}
			}))
			defer server.Close()

			got, err := getTopCardsForCategoryWithURL(t.Context(), tt.category, tt.page, server.URL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getTopCardsForCategoryWithURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.wantCards {
				t.Errorf("got %d cards, want %d", len(got), tt.wantCards)
			}
		})
	}
}

func TestFormatCommanderRecsForDisplayEdgeCases(t *testing.T) {
	t.Run("empty card list skipped", func(t *testing.T) {
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: "Test Commander", NumDecks: 100},
					CardLists: []CardList{
						{Header: "Skipped Section", CardViews: []CardView{}},
						{Header: "Non-Empty Section", CardViews: []CardView{
							{Name: "Sol Ring", NumDecks: 90},
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
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: "Test Commander", NumDecks: 100},
					CardLists: []CardList{
						{Header: "Top Cards", CardViews: []CardView{
							{Name: "Sol Ring", NumDecks: 90, Synergy: 0, Salt: 0},
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
		page := &Response{
			Container: Container{
				JSONDict: Data{
					Card: CardInfo{Name: "Test Commander", NumDecks: 100},
					CardLists: []CardList{
						{Header: "Top Cards", CardViews: []CardView{
							{Name: "Sol Ring", NumDecks: 90},
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
