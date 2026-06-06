package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestSearchArchidektDecksE2E tests searching for real decks on Archidekt.
func TestSearchArchidektDecksE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := SearchArchidektDecks(ctx, "Atraxa, Praetors' Voice", 0, 5)
	if err != nil {
		t.Fatalf("SearchArchidektDecks() failed: %v", err)
	}

	if len(result.Results) == 0 {
		t.Error("Expected at least one deck result for Atraxa")
	}

	deck := result.Results[0]
	if deck.Name == "" {
		t.Error("Expected deck to have a name")
	}
	if deck.Owner.Username == "" {
		t.Error("Expected deck to have an owner")
	}
	if deck.ID == 0 {
		t.Error("Expected deck to have a non-zero ID")
	}

	t.Logf("✓ Found %d decks (total: %d), top: %q by %s (views: %d)",
		len(result.Results), result.Count, deck.Name, deck.Owner.Username, deck.ViewCount)
}

// TestSearchArchidektDecksByBracketE2E tests bracket filtering on the live API.
func TestSearchArchidektDecksByBracketE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := SearchArchidektDecks(ctx, "Atraxa, Praetors' Voice", 4, 5)
	if err != nil {
		t.Fatalf("SearchArchidektDecks() with bracket=4 failed: %v", err)
	}

	for _, deck := range result.Results {
		if deck.EdhBracket != nil && *deck.EdhBracket != 4 {
			t.Errorf("Expected all results to have bracket 4, got %d for deck %q", *deck.EdhBracket, deck.Name)
		}
	}

	t.Logf("✓ Bracket 4 search returned %d results", len(result.Results))
}

// TestGetArchidektUserDecksE2E tests fetching a real user's decks from Archidekt.
func TestGetArchidektUserDecksE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := GetArchidektUserDecks(ctx, "nathanmartins", 1)
	if err != nil {
		t.Fatalf("GetArchidektUserDecks() failed: %v", err)
	}

	if len(result.Results) == 0 {
		t.Error("Expected at least one deck for user nathanmartins")
	}

	t.Logf("✓ Found %d decks for nathanmartins (total: %d)", len(result.Results), result.Count)
}

// TestGetArchidektDeckE2E tests fetching a specific real deck from Archidekt.
func TestGetArchidektDeckE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Deathstar Hearthhull — nathanmartins' deck
	deck, err := GetArchidektDeck(ctx, 18257175)
	if err != nil {
		t.Fatalf("GetArchidektDeck() failed: %v", err)
	}

	if deck.Name == "" {
		t.Error("Expected deck to have a name")
	}
	if len(deck.Cards) == 0 {
		t.Error("Expected deck to have cards")
	}

	output := FormatArchidektDeckForDisplay(deck)
	if !strings.Contains(output, "https://archidekt.com/decks/18257175") {
		t.Error("Expected formatted output to contain deck URL")
	}

	t.Logf("✓ Fetched deck %q (%d cards)", deck.Name, len(deck.Cards))
}
