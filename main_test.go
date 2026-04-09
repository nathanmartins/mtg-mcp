package main

import (
	"testing"
)

func TestParseCardLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantName string
		wantQty  int
	}{
		{
			name:     "single card with quantity 1",
			line:     "1 Sol Ring",
			wantName: "Sol Ring",
			wantQty:  1,
		},
		{
			name:     "card with quantity 4",
			line:     "4 Lightning Bolt",
			wantName: "Lightning Bolt",
			wantQty:  4,
		},
		{
			name:     "grouped basic lands",
			line:     "9 Plains",
			wantName: "Plains",
			wantQty:  9,
		},
		{
			name:     "large quantity",
			line:     "20 Mountain",
			wantName: "Mountain",
			wantQty:  20,
		},
		{
			name:     "card name without quantity",
			line:     "Sol Ring",
			wantName: "Sol Ring",
			wantQty:  1,
		},
		{
			name:     "card name with only spaces",
			line:     "Lightning Bolt",
			wantName: "Lightning Bolt",
			wantQty:  1,
		},
		{
			name:     "card with quantity and multi-word name",
			line:     "1 Sword of the Animist",
			wantName: "Sword of the Animist",
			wantQty:  1,
		},
		{
			name:     "card with quantity zero",
			line:     "0 Plains",
			wantName: "0 Plains",
			wantQty:  1,
		},
		{
			name:     "quantity with no name",
			line:     "4",
			wantName: "4",
			wantQty:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotQty := parseCardLine(tt.line)
			if gotName != tt.wantName {
				t.Errorf("parseCardLine(%q) name = %q, want %q", tt.line, gotName, tt.wantName)
			}
			if gotQty != tt.wantQty {
				t.Errorf("parseCardLine(%q) qty = %d, want %d", tt.line, gotQty, tt.wantQty)
			}
		})
	}
}

func TestParseTextDecklist(t *testing.T) {
	tests := []struct {
		name      string
		decklist  string
		wantCards []string
	}{
		{
			name:      "single cards expanded",
			decklist:  "1 Sol Ring\n1 Lightning Bolt",
			wantCards: []string{"Sol Ring", "Lightning Bolt"},
		},
		{
			name:      "grouped basics expanded",
			decklist:  "3 Plains\n2 Island",
			wantCards: []string{"Plains", "Plains", "Plains", "Island", "Island"},
		},
		{
			name:     "mixed grouped and single",
			decklist: "4 Lightning Bolt\n1 Sol Ring\n20 Mountain",
			wantCards: []string{
				"Lightning Bolt", "Lightning Bolt", "Lightning Bolt", "Lightning Bolt",
				"Sol Ring",
				"Mountain", "Mountain", "Mountain", "Mountain", "Mountain",
				"Mountain", "Mountain", "Mountain", "Mountain", "Mountain",
				"Mountain", "Mountain", "Mountain", "Mountain", "Mountain",
				"Mountain", "Mountain", "Mountain", "Mountain", "Mountain",
			},
		},
		{
			name:      "blank lines ignored",
			decklist:  "1 Sol Ring\n\n1 Lightning Bolt\n",
			wantCards: []string{"Sol Ring", "Lightning Bolt"},
		},
		{
			name:     "commander deck format with grouped basics",
			decklist: "1 Bruna, Light of Alabaster\n1 Ethereal Armor\n9 Plains\n10 Island",
			wantCards: []string{
				"Bruna, Light of Alabaster",
				"Ethereal Armor",
				"Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains",
				"Island", "Island", "Island", "Island", "Island", "Island", "Island", "Island", "Island", "Island",
			},
		},
		{
			name:      "empty decklist",
			decklist:  "",
			wantCards: nil,
		},
		{
			name:      "only blank lines",
			decklist:  "\n\n\n",
			wantCards: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTextDecklist(tt.decklist)
			if len(got) != len(tt.wantCards) {
				t.Errorf("parseTextDecklist() got %d cards, want %d cards\n  got:  %v\n  want: %v",
					len(got), len(tt.wantCards), got, tt.wantCards)
				return
			}
			for i := range got {
				if got[i] != tt.wantCards[i] {
					t.Errorf("parseTextDecklist() card[%d] = %q, want %q", i, got[i], tt.wantCards[i])
				}
			}
		})
	}
}

func TestParseDecklistString(t *testing.T) {
	tests := []struct {
		name      string
		decklist  string
		wantCards []string
	}{
		{
			name:      "text format with grouped basics",
			decklist:  "2 Sol Ring\n3 Plains",
			wantCards: []string{"Sol Ring", "Sol Ring", "Plains", "Plains", "Plains"},
		},
		{
			name:      "JSON array format unchanged",
			decklist:  `["Sol Ring","Plains","Island"]`,
			wantCards: []string{"Sol Ring", "Plains", "Island"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDecklistString(tt.decklist)
			if len(got) != len(tt.wantCards) {
				t.Errorf("parseDecklistString() got %d cards, want %d cards\n  got:  %v\n  want: %v",
					len(got), len(tt.wantCards), got, tt.wantCards)
				return
			}
			for i := range got {
				if got[i] != tt.wantCards[i] {
					t.Errorf("parseDecklistString() card[%d] = %q, want %q", i, got[i], tt.wantCards[i])
				}
			}
		})
	}
}
