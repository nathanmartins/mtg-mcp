package main

import (
	"context"
	"strings"
	"testing"
	"time"

	scryfall "github.com/BlueMonday/go-scryfall"
)

// newE2EImageServer builds a server backed by the real Scryfall API.
func newE2EImageServer(t *testing.T) *MTGCommanderServer {
	t.Helper()

	client, err := scryfall.NewClient()
	if err != nil {
		t.Fatalf("Failed to create Scryfall client: %v", err)
	}

	return &MTGCommanderServer{scryfallClient: client}
}

// TestGetCardImageEnglishE2E fetches a plain English card image end-to-end.
func TestGetCardImageEnglishE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := newE2EImageServer(t)
	res, err := s.handleGetCardImage(ctx, toolRequest(map[string]any{"name": "Sol Ring"}))
	if err != nil {
		t.Fatalf("handleGetCardImage() failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if !strings.Contains(text, "**Language:** en") {
		t.Errorf("expected English language marker, got:\n%s", text)
	}
	if !strings.Contains(text, "data:image/jpeg;base64,") {
		t.Fatal("expected an embedded base64 data URI in the result")
	}
	t.Logf("✓ Embedded Sol Ring image (%d chars of result text)", len(text))
}

// TestGetCardImageItalianE2E fetches an Italian printing (Sol Ring -> Anello Solare).
func TestGetCardImageItalianE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := newE2EImageServer(t)
	res, err := s.handleGetCardImage(ctx, toolRequest(map[string]any{
		"name":     "Sol Ring",
		"language": "it",
	}))
	if err != nil {
		t.Fatalf("handleGetCardImage() failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if !strings.Contains(text, "**Language:** it") {
		t.Errorf("expected Italian printing, got:\n%s", text)
	}
	if !strings.Contains(text, "data:image/") {
		t.Error("expected an embedded data URI for the Italian printing")
	}
}

// TestGetCardImageFallbackE2E requests a language with no printing and expects English.
func TestGetCardImageFallbackE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := newE2EImageServer(t)
	// Universes Beyond cards have no Italian printing.
	res, err := s.handleGetCardImage(ctx, toolRequest(map[string]any{
		"name":     "Abaddon the Despoiler",
		"language": "it",
	}))
	if err != nil {
		t.Fatalf("handleGetCardImage() failed: %v", err)
	}
	text := resultText(t, res)
	if !strings.Contains(text, "showing English") || !strings.Contains(text, "**Language:** en") {
		t.Errorf("expected English fallback, got:\n%s", text)
	}
}

// TestGetCardImageDoubleFacedE2E fetches a transform card and expects two images.
func TestGetCardImageDoubleFacedE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := newE2EImageServer(t)
	res, err := s.handleGetCardImage(ctx, toolRequest(map[string]any{"name": "Delver of Secrets"}))
	if err != nil {
		t.Fatalf("handleGetCardImage() failed: %v", err)
	}

	if n := strings.Count(resultText(t, res), "data:image/jpeg;base64,"); n != 2 {
		t.Fatalf("expected 2 embedded images for a double-faced card, got %d", n)
	}
}
