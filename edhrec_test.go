package main

import (
	"context"
	"encoding/json"
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
		mockResponse  EDHRECResponse
		mockStatus    int
		wantErr       bool
		checkURL      bool
		expectedURL   string
	}{
		{
			name:          "successful request",
			commanderName: "Atraxa, Praetors' Voice",
			mockResponse: EDHRECResponse{
				Container: EDHRECContainer{
					JSONDict: EDHRECData{
						Card: EDHRECCardInfo{
							Name:      "Atraxa, Praetors' Voice",
							Sanitized: "atraxa-praetors-voice",
							ColorID:   []string{"W", "U", "B", "G"},
							NumDecks:  50000,
						},
						NumDecks: 50000,
						CardLists: []EDHRECCardList{
							{
								Header: "High Synergy Cards",
								Tag:    "highsynergy",
								CardViews: []EDHRECCardView{
									{
										Name:      "Doubling Season",
										Sanitized: "doubling-season",
										Inclusion: 25000,
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

			ctx := context.Background()
			got, err := getCommanderRecommendationsWithURL(ctx, tt.commanderName, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCommanderRecommendations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				if got.Card.Name != tt.mockResponse.Container.JSONDict.Card.Name {
					t.Errorf("GetCommanderRecommendations() card name = %v, want %v",
						got.Card.Name, tt.mockResponse.Container.JSONDict.Card.Name)
				}
			}
		})
	}
}

func TestGetCombosForColors(t *testing.T) {
	tests := []struct {
		name         string
		colors       string
		mockResponse EDHRECComboResponse
		mockStatus   int
		wantErr      bool
	}{
		{
			name:   "colorless combos",
			colors: "colorless",
			mockResponse: EDHRECComboResponse{
				Container: EDHRECComboContainer{
					JSONDict: EDHRECComboData{
						CardLists: []EDHRECComboList{
							{
								Header: "Basalt Monolith + Forsaken Monument",
								CardViews: []EDHRECCardView{
									{Name: "Basalt Monolith"},
									{Name: "Forsaken Monument"},
								},
								Combo: &EDHRECCombo{
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

			ctx := context.Background()
			got, err := getCombosForColorsWithURL(ctx, tt.colors, server.URL)

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
	data := &EDHRECData{
		Card: EDHRECCardInfo{
			Name:     "Test Commander",
			ColorID:  []string{"W", "U"},
			NumDecks: 1000,
		},
		NumDecks: 1000,
		CardLists: []EDHRECCardList{
			{
				Header: "High Synergy Cards",
				CardViews: []EDHRECCardView{
					{
						Name:      "Card 1",
						Inclusion: 500,
						Synergy:   0.35,
						Salt:      1.5,
					},
					{
						Name:      "Card 2",
						Inclusion: 400,
						Synergy:   0.25,
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		data          *EDHRECData
		limit         int
		wantContains  []string
		wantCardCount int
	}{
		{
			name:  "with limit",
			data:  data,
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
			data:  data,
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
			got := FormatCommanderRecsForDisplay(tt.data, tt.limit)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatCommanderRecsForDisplay() missing %q in output", want)
				}
			}

			// Check if the correct number of cards are shown
			if tt.limit > 0 {
				card2Count := strings.Count(got, "Card 2")
				if tt.wantCardCount == 1 && card2Count > 0 {
					t.Error("FormatCommanderRecsForDisplay() should limit cards but found Card 2")
				}
			}
		})
	}
}

func TestFormatCombosForDisplay(t *testing.T) {
	data := &EDHRECComboData{
		CardLists: []EDHRECComboList{
			{
				Header: "Card A + Card B (1000 decks)",
				CardViews: []EDHRECCardView{
					{Name: "Card A"},
					{Name: "Card B"},
				},
				Combo: &EDHRECCombo{
					ComboID: "123-456",
					Results: []string{"Win the game", "Infinite mana"},
				},
			},
			{
				Header: "Card C + Card D (500 decks)",
				CardViews: []EDHRECCardView{
					{Name: "Card C"},
					{Name: "Card D"},
				},
				Combo: &EDHRECCombo{
					ComboID: "789-012",
					Results: []string{"Infinite tokens"},
				},
			},
		},
	}

	tests := []struct {
		name         string
		data         *EDHRECComboData
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
			got := FormatCombosForDisplay(tt.data, tt.limit)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatCombosForDisplay() missing %q in output", want)
				}
			}

			if tt.limit == 1 && strings.Contains(got, "Card C + Card D") {
				t.Error("FormatCombosForDisplay() should limit combos but found second combo")
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
		mockResp   EDHRECResponse
		wantCards  int
		wantErr    bool
	}{
		{
			name:       "successful request flattens card lists",
			category:   "salt",
			page:       0,
			mockStatus: http.StatusOK,
			mockResp: EDHRECResponse{
				Container: EDHRECContainer{
					JSONDict: EDHRECData{
						CardLists: []EDHRECCardList{
							{
								Header: "Saltiest Cards",
								CardViews: []EDHRECCardView{
									{Name: "Armageddon"},
									{Name: "Stasis"},
								},
							},
							{
								Header: "More Salt",
								CardViews: []EDHRECCardView{
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

			got, err := getTopCardsForCategoryWithURL(context.Background(), tt.category, tt.page, server.URL)
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
		data := &EDHRECData{
			Card:     EDHRECCardInfo{Name: "Test Commander"},
			NumDecks: 100,
			CardLists: []EDHRECCardList{
				{Header: "Skipped Section", CardViews: []EDHRECCardView{}},
				{Header: "Non-Empty Section", CardViews: []EDHRECCardView{
					{Name: "Sol Ring", Inclusion: 90},
				}},
			},
		}
		got := FormatCommanderRecsForDisplay(data, 10)
		if strings.Contains(got, "Skipped Section") {
			t.Error("FormatCommanderRecsForDisplay() should skip card lists with no cards")
		}
		if !strings.Contains(got, "Non-Empty Section") {
			t.Error("FormatCommanderRecsForDisplay() missing non-empty section")
		}
	})

	t.Run("zero synergy and zero salt omitted", func(t *testing.T) {
		data := &EDHRECData{
			Card:     EDHRECCardInfo{Name: "Test Commander"},
			NumDecks: 100,
			CardLists: []EDHRECCardList{
				{Header: "Top Cards", CardViews: []EDHRECCardView{
					{Name: "Sol Ring", Inclusion: 90, Synergy: 0, Salt: 0},
				}},
			},
		}
		got := FormatCommanderRecsForDisplay(data, 10)
		if strings.Contains(got, "Synergy:") {
			t.Error("FormatCommanderRecsForDisplay() should omit Synergy line when zero")
		}
		if strings.Contains(got, "Salt Score:") {
			t.Error("FormatCommanderRecsForDisplay() should omit Salt Score line when zero")
		}
	})

	t.Run("truncation suffix shown", func(t *testing.T) {
		data := &EDHRECData{
			Card:     EDHRECCardInfo{Name: "Test Commander"},
			NumDecks: 100,
			CardLists: []EDHRECCardList{
				{Header: "Top Cards", CardViews: []EDHRECCardView{
					{Name: "Sol Ring", Inclusion: 90},
					{Name: "Arcane Signet", Inclusion: 80},
					{Name: "Command Tower", Inclusion: 70},
				}},
			},
		}
		got := FormatCommanderRecsForDisplay(data, 1)
		if !strings.Contains(got, "and 2 more cards") {
			t.Errorf("FormatCommanderRecsForDisplay() missing truncation suffix, got:\n%s", got)
		}
	})
}
