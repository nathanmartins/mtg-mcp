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
			name:   "dimir combos",
			colors: "ub",
			mockResponse: EDHRECComboResponse{
				Container: EDHRECComboContainer{
					JSONDict: EDHRECComboData{
						ComboCounts: []EDHRECCombo{
							{
								ComboID:    "combo-1",
								Colors:     "UB",
								Count:      1000,
								Percentage: 15.5,
								Rank:       1,
								CardNames:  []string{"Thassa's Oracle", "Demonic Consultation"},
								Results:    []string{"Win the game"},
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
				if len(got.ComboCounts) != len(tt.mockResponse.Container.JSONDict.ComboCounts) {
					t.Errorf("GetCombosForColors() combo count = %v, want %v",
						len(got.ComboCounts), len(tt.mockResponse.Container.JSONDict.ComboCounts))
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
		ComboCounts: []EDHRECCombo{
			{
				Rank:       1,
				CardNames:  []string{"Card A", "Card B"},
				Colors:     "ub",
				Count:      1000,
				Percentage: 15.5,
				Results:    []string{"Win the game", "Infinite mana"},
			},
			{
				Rank:       2,
				CardNames:  []string{"Card C", "Card D"},
				Colors:     "rg",
				Count:      500,
				Percentage: 7.8,
				Results:    []string{"Infinite tokens"},
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
				"Combo #1",
				"Card A + Card B",
				"UB",
				"1000 decks",
				"Win the game",
			},
		},
		{
			name:  "without limit",
			data:  data,
			limit: 0,
			wantContains: []string{
				"Combo #1",
				"Combo #2",
				"Card C + Card D",
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

			if tt.limit == 1 && strings.Contains(got, "Combo #2") {
				t.Error("FormatCombosForDisplay() should limit combos but found Combo #2")
			}
		})
	}
}
