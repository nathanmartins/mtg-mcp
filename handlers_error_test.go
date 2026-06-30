package main

import (
	"context"
	"testing"
)

// These tests exercise the request-validation branches of the deck-source handlers,
// which return before any network call and are therefore hermetic.

func TestMoxfieldHandlerValidation(t *testing.T) {
	s := &MTGCommanderServer{}
	ctx := context.Background()

	t.Run("get deck missing id", func(t *testing.T) {
		res, _ := s.handleGetMoxfieldDeck(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing deck_id")
		}
	})

	t.Run("user decks missing username", func(t *testing.T) {
		res, _ := s.handleGetMoxfieldUserDecks(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing username")
		}
	})

	t.Run("search missing commander", func(t *testing.T) {
		res, _ := s.handleSearchMoxfieldDecks(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing commander")
		}
	})
}

func TestArchidektHandlerValidation(t *testing.T) {
	s := &MTGCommanderServer{}
	ctx := context.Background()

	t.Run("get deck missing id", func(t *testing.T) {
		res, _ := s.handleGetArchidektDeck(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing deck_id")
		}
	})

	t.Run("get deck invalid id", func(t *testing.T) {
		res, _ := s.handleGetArchidektDeck(ctx, toolRequest(map[string]any{"deck_id": "not-a-number"}))
		if !res.IsError {
			t.Error("expected error for invalid deck_id")
		}
	})

	t.Run("user decks missing username", func(t *testing.T) {
		res, _ := s.handleGetArchidektUserDecks(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing username")
		}
	})

	t.Run("search missing commander", func(t *testing.T) {
		res, _ := s.handleSearchArchidektDecks(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing commander")
		}
	})
}

func TestEDHRECHandlerValidation(t *testing.T) {
	s := &MTGCommanderServer{}
	ctx := context.Background()

	t.Run("recommendations missing commander", func(t *testing.T) {
		res, _ := s.handleGetEDHRECRecommendations(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing commander")
		}
	})

	t.Run("combos missing colors", func(t *testing.T) {
		res, _ := s.handleGetEDHRECCombos(ctx, toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing colors")
		}
	})
}
