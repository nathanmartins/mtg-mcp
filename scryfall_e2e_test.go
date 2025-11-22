package main

import (
	"context"
	"testing"
	"time"

	scryfall "github.com/BlueMonday/go-scryfall"
)

// TestScryfallSearchCardsE2E tests real Scryfall API card search.
func TestScryfallSearchCardsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := scryfall.NewClient()
	if err != nil {
		t.Fatalf("Failed to create Scryfall client: %v", err)
	}

	// Search for a well-known card
	searchOpts := scryfall.SearchCardsOptions{
		Unique: scryfall.UniqueModePrints,
		Order:  scryfall.OrderName,
		Dir:    scryfall.DirAuto,
	}

	result, err := client.SearchCards(ctx, "Lightning Bolt", searchOpts)
	if err != nil {
		t.Fatalf("SearchCards() failed: %v", err)
	}

	// Verify we got results
	if len(result.Cards) == 0 {
		t.Error("Expected to find Lightning Bolt cards")
	}

	// Verify card structure
	card := result.Cards[0]
	if card.Name == "" {
		t.Error("Expected card to have a name")
	}

	if card.TypeLine == "" {
		t.Error("Expected card to have a type line")
	}

	t.Logf("✓ Successfully found %d results for Lightning Bolt", len(result.Cards))
}

// TestScryfallGetCardByNameE2E tests fetching a specific card by exact name.
func TestScryfallGetCardByNameE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := scryfall.NewClient()
	if err != nil {
		t.Fatalf("Failed to create Scryfall client: %v", err)
	}

	// Get a specific card by name
	card, err := client.GetCardByName(ctx, "Sol Ring", true, scryfall.GetCardByNameOptions{})
	if err != nil {
		t.Fatalf("GetCardByName() failed: %v", err)
	}

	// Verify card details
	if card.Name != "Sol Ring" {
		t.Errorf("Expected card name 'Sol Ring', got %q", card.Name)
	}

	if card.TypeLine == "" {
		t.Error("Expected card to have a type line")
	}

	if card.ManaCost == "" {
		t.Error("Expected Sol Ring to have a mana cost")
	}

	t.Logf("✓ Successfully fetched card: %s (%s)", card.Name, card.TypeLine)
}

// TestScryfallGetRandomCardE2E tests fetching a random card.
func TestScryfallGetRandomCardE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := scryfall.NewClient()
	if err != nil {
		t.Fatalf("Failed to create Scryfall client: %v", err)
	}

	// Get a random card (API doesn't support query filters)
	card, err := client.GetRandomCard(ctx)
	if err != nil {
		t.Fatalf("GetRandomCard() failed: %v", err)
	}

	// Verify we got a card
	if card.Name == "" {
		t.Error("Expected random card to have a name")
	}

	if card.TypeLine == "" {
		t.Error("Expected random card to have a type line")
	}

	t.Logf("✓ Successfully fetched random card: %s (%s)", card.Name, card.TypeLine)
}

// TestScryfallCommanderLegalityE2E tests checking commander legality.
func TestScryfallCommanderLegalityE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := scryfall.NewClient()
	if err != nil {
		t.Fatalf("Failed to create Scryfall client: %v", err)
	}

	testCases := []struct {
		name           string
		cardName       string
		expectedLegal  bool
		expectedBanned bool
	}{
		{
			name:           "legal card",
			cardName:       "Sol Ring",
			expectedLegal:  true,
			expectedBanned: false,
		},
		{
			name:           "banned card",
			cardName:       "Mana Crypt",
			expectedLegal:  false,
			expectedBanned: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			card, cardErr := client.GetCardByName(ctx, tc.cardName, true, scryfall.GetCardByNameOptions{})
			if cardErr != nil {
				t.Fatalf("GetCardByName() failed: %v", cardErr)
			}

			// Check commander legality
			legality := card.Legalities.Commander

			isLegal := legality == "legal"
			isBanned := legality == "banned"

			if tc.expectedLegal && !isLegal {
				t.Errorf("Expected %s to be legal in Commander, got: %s", tc.cardName, legality)
			}

			if tc.expectedBanned && !isBanned {
				t.Errorf("Expected %s to be banned in Commander, got: %s", tc.cardName, legality)
			}

			t.Logf("✓ %s legality in Commander: %s", tc.cardName, legality)
		})
	}
}

// TestScryfallCardRulingsE2E tests fetching card rulings.
func TestScryfallCardRulingsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := scryfall.NewClient()
	if err != nil {
		t.Fatalf("Failed to create Scryfall client: %v", err)
	}

	// Get a card with known rulings
	card, err := client.GetCardByName(ctx, "Doubling Season", true, scryfall.GetCardByNameOptions{})
	if err != nil {
		t.Fatalf("GetCardByName() failed: %v", err)
	}

	// Fetch rulings
	rulings, err := client.GetRulings(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetRulings() failed: %v", err)
	}

	// Verify rulings structure (this card should have rulings)
	if len(rulings) == 0 {
		t.Log("Note: Doubling Season has no rulings (this may be expected)")
	} else {
		ruling := rulings[0]
		if ruling.Comment == "" {
			t.Error("Expected ruling to have a comment")
		}

		t.Logf("✓ Successfully fetched %d rulings for %s", len(rulings), card.Name)
	}
}

// TestScryfallCardPricingE2E tests fetching card pricing information.
func TestScryfallCardPricingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := scryfall.NewClient()
	if err != nil {
		t.Fatalf("Failed to create Scryfall client: %v", err)
	}

	// Get a card with pricing
	card, err := client.GetCardByName(ctx, "Lightning Bolt", true, scryfall.GetCardByNameOptions{})
	if err != nil {
		t.Fatalf("GetCardByName() failed: %v", err)
	}

	// Check if pricing information is available
	if card.Prices.USD == "" && card.Prices.USDFoil == "" && card.Prices.EUR == "" {
		t.Error("Expected at least one price to be available")
	}

	t.Logf("✓ Successfully fetched pricing for %s (USD: %s, EUR: %s)",
		card.Name, card.Prices.USD, card.Prices.EUR)
}
