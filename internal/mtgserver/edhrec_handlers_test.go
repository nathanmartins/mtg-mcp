package mtgserver

import (
	"strings"
	"testing"

	"github.com/nathanmartins/mtg-mcp/internal/edhrec"
)

func TestRecFilterFromArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    map[string]any
		want    edhrec.RecFilter
		wantErr bool
	}{
		{
			name: "no args",
			args: map[string]any{},
			want: edhrec.RecFilter{},
		},
		{
			name: "price tier is lowercased",
			args: map[string]any{paramPriceTier: "Budget"},
			want: edhrec.RecFilter{PriceTier: edhrec.PriceTierBudget},
		},
		{
			name:    "invalid price tier names both valid values",
			args:    map[string]any{paramPriceTier: "middle"},
			wantErr: true,
		},
		{
			name:    "wrong-typed price tier is an error",
			args:    map[string]any{paramPriceTier: 42},
			wantErr: true,
		},
		{
			name: "theme is trimmed and lowercased",
			args: map[string]any{paramTheme: " Infect "},
			want: edhrec.RecFilter{Theme: testThemeInfect},
		},
		{
			name:    "wrong-typed theme is an error",
			args:    map[string]any{paramTheme: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := recFilterFromArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("recFilterFromArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.name == "invalid price tier names both valid values" {
					missingBudget := !strings.Contains(err.Error(), edhrec.PriceTierBudget)
					missingExpensive := !strings.Contains(err.Error(), edhrec.PriceTierExpensive)
					if missingBudget || missingExpensive {
						t.Errorf("error should name both valid tiers, got: %v", err)
					}
				}
				return
			}
			if got != tt.want {
				t.Errorf("recFilterFromArgs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
