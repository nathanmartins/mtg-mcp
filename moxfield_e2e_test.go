package main

import (
	"context"
	"testing"
	"time"
)

// TestMoxfieldGetDeckE2E tests fetching a real deck from Moxfield.
func TestMoxfieldGetDeckE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a known public deck (this is a popular EDH deck)
	// Note: This deck ID should be stable, but may need updating if removed
	publicID := "6HxHcsIL70W0wT0xGHbRqw"

	deck, err := GetMoxfieldDeck(ctx, publicID)
	if err != nil {
		t.Fatalf("GetMoxfieldDeck() failed: %v", err)
	}

	// Verify deck structure
	if deck.Name == "" {
		t.Error("Expected deck name to be populated")
	}

	if deck.Format == "" {
		t.Error("Expected deck format to be populated")
	}

	if len(deck.Mainboard) == 0 {
		t.Error("Expected mainboard to have cards")
	}

	// Verify card structure
	for _, entry := range deck.Mainboard {
		if entry.Card.Name == "" {
			t.Error("Expected card to have a name")
		}
		if entry.Quantity <= 0 {
			t.Error("Expected quantity to be positive")
		}
		break // Just check first card
	}

	t.Logf("✓ Successfully fetched deck: %s (Format: %s)", deck.Name, deck.Format)
}

// TestMoxfieldGetUserDecksE2E tests fetching user decks from Moxfield.
func TestMoxfieldGetUserDecksE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a known public user with decks
	username := "CommanderSpellbook"

	response, err := GetUserDecks(ctx, username, 10)
	if err != nil {
		t.Fatalf("GetUserDecks() failed: %v", err)
	}

	// Verify response structure
	if response.PageSize <= 0 {
		t.Error("Expected page size to be positive")
	}

	if response.TotalResults < 0 {
		t.Error("Expected total results to be non-negative")
	}

	// If user has decks, verify structure
	if len(response.Data) > 0 {
		deck := response.Data[0]
		if deck.Name == "" {
			t.Error("Expected deck to have a name")
		}
		if deck.PublicID == "" {
			t.Error("Expected deck to have a public ID")
		}

		t.Logf("✓ Successfully fetched %d decks for user %s", len(response.Data), username)
	} else {
		t.Logf("✓ User %s has no public decks", username)
	}
}

// TestMoxfieldSearchDecksE2E tests searching for decks on Moxfield.
func TestMoxfieldSearchDecksE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Search for a popular commander
	params := MoxfieldSearchParams{
		Query:      "Atraxa",
		Format:     "commander",
		PageSize:   5,
		PageNumber: 1,
	}

	response, err := SearchMoxfieldDecks(ctx, params)
	if err != nil {
		t.Fatalf("SearchMoxfieldDecks() failed: %v", err)
	}

	// Verify response structure
	if response.PageNumber != 1 {
		t.Errorf("Expected page number 1, got %d", response.PageNumber)
	}

	if response.PageSize <= 0 {
		t.Error("Expected page size to be positive")
	}

	// Verify we got some results
	if len(response.Data) == 0 {
		t.Error("Expected to find at least one deck for Atraxa")
	}

	// Verify deck structure
	if len(response.Data) > 0 {
		deck := response.Data[0]
		if deck.Name == "" {
			t.Error("Expected deck to have a name")
		}
		if deck.PublicID == "" {
			t.Error("Expected deck to have a public ID")
		}

		t.Logf("✓ Successfully found %d decks matching query '%s'", len(response.Data), params.Query)
	}
}

// TestMoxfieldExtractPublicIDE2E tests extracting public IDs from URLs.
func TestMoxfieldExtractPublicIDE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "full URL",
			url:      "https://www.moxfield.com/decks/6HxHcsIL70W0wT0xGHbRqw",
			expected: "6HxHcsIL70W0wT0xGHbRqw",
		},
		{
			name:     "just ID",
			url:      "6HxHcsIL70W0wT0xGHbRqw",
			expected: "6HxHcsIL70W0wT0xGHbRqw",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractPublicIDFromURL(tc.url)
			if got != tc.expected {
				t.Errorf("ExtractPublicIDFromURL(%q) = %q, want %q", tc.url, got, tc.expected)
			}
		})
	}

	t.Log("✓ Successfully extracted public IDs from URLs")
}

// TestMoxfieldFormatDeckE2E tests formatting a real deck.
func TestMoxfieldFormatDeckE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch a real deck
	publicID := "6HxHcsIL70W0wT0xGHbRqw"
	deck, err := GetMoxfieldDeck(ctx, publicID)
	if err != nil {
		t.Fatalf("GetMoxfieldDeck() failed: %v", err)
	}

	// Format the deck
	output := FormatDeckForDisplay(deck)
	if output == "" {
		t.Error("Expected non-empty formatted output")
	}

	// Verify output contains expected sections
	expectedStrings := []string{
		deck.Name,
		"Format:",
		"Mainboard",
	}

	for _, expected := range expectedStrings {
		if !contains(output, expected) {
			t.Errorf("Expected output to contain %q", expected)
		}
	}

	t.Logf("✓ Successfully formatted deck: %s", deck.Name)
}
