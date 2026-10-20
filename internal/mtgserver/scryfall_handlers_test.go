package mtgserver

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
			line:     testDecklistSolRing,
			wantName: testSolRing,
			wantQty:  1,
		},
		{
			name:     "card with quantity 4",
			line:     "4 Lightning Bolt",
			wantName: testLightningBolt,
			wantQty:  4,
		},
		{
			name:     "grouped basic lands",
			line:     "9 Plains",
			wantName: testPlains,
			wantQty:  9,
		},
		{
			name:     "large quantity",
			line:     "20 Mountain",
			wantName: testMountain,
			wantQty:  20,
		},
		{
			name:     "card name without quantity",
			line:     testSolRing,
			wantName: testSolRing,
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
			wantCards: []string{testSolRing, testLightningBolt},
		},
		{
			name:      "grouped basics expanded",
			decklist:  "3 Plains\n2 Island",
			wantCards: []string{testPlains, testPlains, testPlains, testIsland, testIsland},
		},
		{
			name:     "mixed grouped and single",
			decklist: "4 Lightning Bolt\n1 Sol Ring\n20 Mountain",
			wantCards: []string{
				testLightningBolt,
				testLightningBolt,
				testLightningBolt,
				testLightningBolt,
				testSolRing,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
				testMountain,
			},
		},
		{
			name:      "blank lines ignored",
			decklist:  "1 Sol Ring\n\n1 Lightning Bolt\n",
			wantCards: []string{testSolRing, testLightningBolt},
		},
		{
			name:     "commander deck format with grouped basics",
			decklist: "1 Bruna, Light of Alabaster\n1 Ethereal Armor\n9 Plains\n10 Island",
			wantCards: []string{
				"Bruna, Light of Alabaster",
				"Ethereal Armor",
				testPlains,
				testPlains,
				testPlains,
				testPlains,
				testPlains,
				testPlains,
				testPlains,
				testPlains,
				testPlains,
				testIsland,
				testIsland,
				testIsland,
				testIsland,
				testIsland,
				testIsland,
				testIsland,
				testIsland,
				testIsland,
				testIsland,
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
			wantCards: []string{testSolRing, testSolRing, testPlains, testPlains, testPlains},
		},
		{
			name:      "JSON array format unchanged",
			decklist:  `["Sol Ring","Plains","Island"]`,
			wantCards: []string{testSolRing, testPlains, testIsland},
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

func TestConvertToBRL(t *testing.T) {
	tests := []struct {
		price  string
		rate   float64
		want   float64
		wantOK bool
	}{
		{price: "10.00", rate: 5.0, want: 50.0, wantOK: true},
		{price: "1.50", rate: 5.0, want: 7.5, wantOK: true},
		{price: "0.00", rate: 5.0, want: 0.0, wantOK: true},
		{price: "N/A", rate: 5.0},
		{price: "", rate: 5.0},
		// Sscanf used to accept a numeric prefix and silently drop the rest.
		{price: "1.50abc", rate: 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.price, func(t *testing.T) {
			got, ok := convertToBRL(tt.price, tt.rate)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf(
					"convertToBRL(%q, %v) = (%v, %v), want (%v, %v)",
					tt.price,
					tt.rate,
					got,
					ok,
					tt.want,
					tt.wantOK,
				)
			}
		})
	}
}
