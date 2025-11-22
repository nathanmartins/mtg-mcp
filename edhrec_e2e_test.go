package main

import (
	"context"
	"testing"
	"time"
)

// TestEDHRECCommanderRecommendationsE2E tests real EDHREC API for commander recommendations.
func TestEDHRECCommanderRecommendationsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with a popular commander
	data, err := GetCommanderRecommendations(ctx, "Atraxa, Praetors' Voice")
	if err != nil {
		t.Fatalf("GetCommanderRecommendations() failed: %v", err)
	}

	// Verify response structure
	if data.Card.Name == "" {
		t.Error("Expected card name to be populated")
	}

	if data.NumDecks == 0 {
		t.Error("Expected num_decks to be greater than 0")
	}

	if len(data.CardLists) == 0 {
		t.Error("Expected at least one card list")
	}

	// Check that we have some recommendations
	foundCards := false
	for _, cardList := range data.CardLists {
		if len(cardList.CardViews) > 0 {
			foundCards = true
			break
		}
	}

	if !foundCards {
		t.Error("Expected to find card recommendations")
	}

	t.Logf("✓ Successfully fetched recommendations for %s (%d decks)", data.Card.Name, data.NumDecks)
}

// TestEDHRECCombosE2E tests real EDHREC API for color combos.
func TestEDHRECCombosE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with Dimir (UB) color combination
	data, err := GetCombosForColors(ctx, "ub")
	if err != nil {
		t.Fatalf("GetCombosForColors() failed: %v", err)
	}

	// Verify response structure
	if len(data.ComboCounts) == 0 {
		t.Error("Expected at least one combo")
	}

	// Check first combo structure
	combo := data.ComboCounts[0]
	if len(combo.CardNames) == 0 {
		t.Error("Expected combo to have card names")
	}

	if combo.Count == 0 {
		t.Error("Expected combo count to be greater than 0")
	}

	t.Logf("✓ Successfully fetched %d combos for UB colors", len(data.ComboCounts))
}

// TestEDHRECSanitizationE2E tests that card name sanitization works with real API.
func TestEDHRECSanitizationE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCases := []struct {
		name string
		want string
	}{
		{"Teferi, Hero of Dominaria", "teferi-hero-of-dominaria"},
		{"Jace, the Mind Sculptor", "jace-the-mind-sculptor"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := GetCommanderRecommendations(ctx, tc.name)
			if err != nil {
				t.Fatalf("GetCommanderRecommendations() failed for %s: %v", tc.name, err)
			}

			if data.Card.Name == "" {
				t.Errorf("Expected card name to be populated for %s", tc.name)
			}

			t.Logf("✓ Successfully handled card name: %s", tc.name)
		})
	}
}

// TestEDHRECFormatOutputE2E tests that formatting functions work with real data.
func TestEDHRECFormatOutputE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch real data
	data, err := GetCommanderRecommendations(ctx, "Atraxa, Praetors' Voice")
	if err != nil {
		t.Fatalf("GetCommanderRecommendations() failed: %v", err)
	}

	// Test formatting with limit
	output := FormatCommanderRecsForDisplay(data, 5)
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
		if !contains(output, expected) {
			t.Errorf("Expected output to contain %q", expected)
		}
	}

	t.Logf("✓ Successfully formatted commander recommendations")
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

// indexOf returns the index of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
