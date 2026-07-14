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

	data := structuredCardData(t, res)
	if len(data.Images) != 1 || !strings.HasPrefix(data.Images[0].Src, "data:image/jpeg;base64,") {
		t.Fatalf("expected one base64 jpeg data URI in structuredContent, got %+v", data.Images)
	}

	// The widget resource is fixed and served as an MCP Apps document.
	contents, err := s.handleCardImageUIResource(ctx, readResourceRequest(uiResourceURI(t, res)))
	if err != nil {
		t.Fatalf("handleCardImageUIResource() failed: %v", err)
	}
	trc := firstTextResource(t, contents)
	if trc.MIMEType != mimeMCPAppHTML {
		t.Errorf("resource mime = %q, want %q", trc.MIMEType, mimeMCPAppHTML)
	}
	t.Logf("✓ Sol Ring: %d-byte data URI, %d-byte widget HTML", len(data.Images[0].Src), len(trc.Text))
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
	data := structuredCardData(t, res)
	if data.Language != "it" {
		t.Errorf("structured language = %q, want it", data.Language)
	}
	if len(data.Images) == 0 || !strings.HasPrefix(data.Images[0].Src, "data:image/") {
		t.Errorf("expected a data URI image in structuredContent, got %+v", data.Images)
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

	if n := len(structuredCardData(t, res).Images); n != 2 {
		t.Fatalf("expected 2 images in structuredContent for a double-faced card, got %d", n)
	}
}
