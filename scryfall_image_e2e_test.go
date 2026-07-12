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

	imgs := imageContents(t, res)
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	if imgs[0].MIMEType != "image/jpeg" || len(imgs[0].Data) == 0 {
		t.Errorf("expected non-empty jpeg image, got mime=%q len=%d", imgs[0].MIMEType, len(imgs[0].Data))
	}
	t.Logf("✓ Downloaded Sol Ring image (%d base64 chars)", len(imgs[0].Data))
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

	if !strings.Contains(resultText(t, res), "**Language:** it") {
		t.Errorf("expected Italian printing, got:\n%s", resultText(t, res))
	}
	if len(imageContents(t, res)) == 0 {
		t.Error("expected at least one image for the Italian printing")
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

	if imgs := imageContents(t, res); len(imgs) != 2 {
		t.Fatalf("expected 2 images for a double-faced card, got %d", len(imgs))
	}
}
