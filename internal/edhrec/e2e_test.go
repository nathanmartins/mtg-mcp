//go:build e2e

package edhrec

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEDHRECCommanderRecommendationsE2E tests real EDHREC API for commander recommendations.
func TestEDHRECCommanderRecommendationsE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// Test with a popular commander
	page, err := NewClient().GetCommanderPage(ctx, testAtraxa, RecFilter{})
	if err != nil {
		t.Fatalf("GetCommanderPage() failed: %v", err)
	}
	data := &page.Container.JSONDict

	// Verify response structure
	if data.Card.Name == "" {
		t.Error("Expected card name to be populated")
	}

	// Note: EDHREC data might be empty for some commanders or during API updates
	if data.Card.NumDecks == 0 {
		t.Logf(
			"Warning: EDHREC returned 0 decks for %s (API might be updating or commander not tracked)",
			data.Card.Name,
		)
	}

	if len(data.CardLists) == 0 {
		t.Logf("Warning: No card lists returned (EDHREC data might be temporarily unavailable)")
	} else {
		// Check that we have some recommendations
		foundCards := false
		for _, cardList := range data.CardLists {
			if len(cardList.CardViews) > 0 {
				foundCards = true
				break
			}
		}

		if !foundCards {
			t.Logf("Warning: No card recommendations found in any list")
		}
	}

	t.Logf("✓ Successfully fetched recommendations for %s (%d decks, %d card lists)",
		data.Card.Name, data.Card.NumDecks, len(data.CardLists))
}

// TestEDHRECCombosE2E tests real EDHREC API for color combos.
func TestEDHRECCombosE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// Test with colorless which typically has well-known combos
	data, err := NewClient().GetCombosForColors(ctx, testColorless)
	if err != nil {
		t.Fatalf("GetCombosForColors() failed: %v", err)
	}

	// Verify response structure (new format uses CardLists)
	if len(data.CardLists) == 0 {
		t.Fatal("Expected at least one combo in colorless")
	}

	// Check first combo structure
	comboList := data.CardLists[0]
	if len(comboList.CardViews) == 0 {
		t.Error("Expected combo to have card views")
	}

	if comboList.Header == "" {
		t.Error("Expected combo to have a header")
	}

	// Verify we have actual card names
	if len(comboList.CardViews) > 0 && comboList.CardViews[0].Name == "" {
		t.Error("Expected card view to have a name")
	}

	t.Logf("✓ Successfully fetched %d combos for colorless", len(data.CardLists))
	t.Logf("  First combo: %s", comboList.Header)
}

// TestEDHRECSanitizationE2E tests that card name sanitization works with real API.
func TestEDHRECSanitizationE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	testCases := []struct {
		name string
		want string
	}{
		{"Edgar Markov", "edgar-markov"},
		{"The Ur-Dragon", "the-ur-dragon"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := NewClient().GetCommanderPage(ctx, tc.name, RecFilter{})
			if err != nil {
				t.Fatalf("GetCommanderPage() failed for %s: %v", tc.name, err)
			}
			data := &page.Container.JSONDict

			if data.Card.Name == "" {
				t.Errorf("Expected card name to be populated for %s", tc.name)
			}

			t.Logf("✓ Successfully handled card name: %s", tc.name)
		})
	}
}

// TestEDHRECFormatOutputE2E tests that formatting functions work with real data.
func TestEDHRECFormatOutputE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// Fetch real data
	page, err := NewClient().GetCommanderPage(ctx, testAtraxa, RecFilter{})
	if err != nil {
		t.Fatalf("GetCommanderPageWithURL() failed: %v", err)
	}

	data := page.Container.JSONDict

	// Skip if no data returned
	if data.Card.NumDecks == 0 && len(data.CardLists) == 0 {
		t.Skip("Skipping format test due to empty EDHREC data")
		return
	}

	// Test formatting with limit
	output := FormatCommanderRecs(page, RecFilter{}, 5)
	if output == "" {
		t.Error("Expected non-empty formatted output")
	}

	// Check that output contains expected sections
	expectedStrings := []string{
		"EDHREC Recommendations",
		"Total Decks:",
		data.Card.Name,
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q", expected)
		}
	}

	t.Logf("✓ Successfully formatted commander recommendations")
}

// TestEDHRECThemeAndBudgetFilterE2E proves the composed theme+tier path is real and narrows the
// deck pool: unfiltered > theme-only > theme+tier.
func TestEDHRECThemeAndBudgetFilterE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	const commander = "Atraxa, Praetors' Voice"

	unfiltered, err := NewClient().GetCommanderPage(ctx, commander, RecFilter{})
	if err != nil {
		t.Fatalf("unfiltered GetCommanderPage() failed: %v", err)
	}

	themeOnly, err := NewClient().GetCommanderPage(ctx, commander, RecFilter{Theme: testThemeInfect})
	if err != nil {
		t.Fatalf("theme-only GetCommanderPage() failed: %v", err)
	}

	combined, err := NewClient().GetCommanderPage(
		ctx, commander, RecFilter{Theme: testThemeInfect, PriceTier: PriceTierBudget},
	)
	if err != nil {
		t.Fatalf("combined GetCommanderPage() failed: %v", err)
	}

	unfilteredDecks := unfiltered.Container.JSONDict.Card.NumDecks
	themeOnlyDecks := themeOnly.Container.JSONDict.Card.NumDecks
	combinedDecks := combined.Container.JSONDict.Card.NumDecks

	if unfilteredDecks <= 0 || themeOnlyDecks <= 0 || combinedDecks <= 0 {
		t.Fatalf("expected all deck counts to be positive: unfiltered=%d theme=%d combined=%d",
			unfilteredDecks, themeOnlyDecks, combinedDecks)
	}

	if combinedDecks >= themeOnlyDecks || themeOnlyDecks >= unfilteredDecks {
		t.Errorf("expected combined < themeOnly < unfiltered, got combined=%d themeOnly=%d unfiltered=%d",
			combinedDecks, themeOnlyDecks, unfilteredDecks)
	}
}

// TestEDHRECInvalidThemeE2E confirms an unrouted theme slug surfaces as an error, not a decode
// panic on EDHREC's XML error body.
func TestEDHRECInvalidThemeE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	_, err := NewClient().GetCommanderPage(
		ctx, testAtraxa, RecFilter{Theme: "not-a-real-theme"},
	)
	if err == nil {
		t.Fatal("expected an error for an invalid theme slug")
	}
}

// TestEDHRECSetCardsE2E fetches a real set page and confirms the formatter never emits NaN.
func TestEDHRECSetCardsE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	page, err := NewClient().GetSetCards(ctx, "rna")
	if err != nil {
		t.Fatalf("GetSetCards() failed: %v", err)
	}

	if page.Header != testRNAName {
		t.Errorf("expected header %q, got %q", testRNAName, page.Header)
	}

	if len(page.Container.JSONDict.CardLists) == 0 {
		t.Fatal("expected at least one cardlist")
	}

	foundPotentialDecks := false
	for _, cardList := range page.Container.JSONDict.CardLists {
		for _, card := range cardList.CardViews {
			if card.PotentialDecks > 0 {
				foundPotentialDecks = true
			}
		}
	}
	if !foundPotentialDecks {
		t.Error("expected at least one cardview with PotentialDecks > 0")
	}

	output := FormatSetCards(page, "rna", 5)
	if output == "" {
		t.Error("expected non-empty formatted set output")
	}
	if strings.Contains(output, "NaN") {
		t.Errorf("set output must never contain NaN, got:\n%s", output)
	}
}
