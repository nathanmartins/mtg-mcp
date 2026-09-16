package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// commanderDeck builds a deck payload whose commander zone holds the given cards.
func commanderDeck(publicID string, commanders ...string) MoxfieldDeck {
	zone := make(map[string]MoxfieldCardEntry, len(commanders))
	for _, name := range commanders {
		zone[name] = MoxfieldCardEntry{Quantity: 1, Card: MoxfieldCardInfo{Name: name}}
	}
	return MoxfieldDeck{PublicID: publicID, Name: "deck-" + publicID, Format: "commander", Commanders: zone}
}

func TestDeckHasCommander(t *testing.T) {
	deck := commanderDeck("a", "Atraxa, Praetors' Voice")
	partners := commanderDeck("b", "Thrasios, Triton Hero", "Tymna the Weaver")

	tests := []struct {
		name      string
		deck      MoxfieldDeck
		commander string
		want      bool
	}{
		{"exact match", deck, "Atraxa, Praetors' Voice", true},
		{"case and space insensitive", deck, "  atraxa, praetors' voice ", true},
		{"second partner matches", partners, "Tymna the Weaver", true},
		{"card present but not commander", deck, "Sol Ring", false},
		{"empty commander zone", MoxfieldDeck{PublicID: "c"}, "Atraxa, Praetors' Voice", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deckHasCommander(&tt.deck, tt.commander); got != tt.want {
				t.Errorf("deckHasCommander(%q) = %v, want %v", tt.commander, got, tt.want)
			}
		})
	}
}

func TestVerifyMoxfieldCommanderDecksKeepsOnlyRealCommanderDecks(t *testing.T) {
	decks := map[string]MoxfieldDeck{
		"yes1": commanderDeck("yes1", "Atraxa, Praetors' Voice"),
		"no1":  commanderDeck("no1", "Winota, Joiner of Forces"),
		"yes2": commanderDeck("yes2", "Atraxa, Praetors' Voice"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
		deck, ok := decks[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(deck)
	}))
	defer server.Close()

	candidates := []MoxfieldDeckSummary{{PublicID: "yes1"}, {PublicID: "no1"}, {PublicID: "yes2"}}
	got := verifyMoxfieldCommanderDecks(
		context.Background(), candidates, "Atraxa, Praetors' Voice", server.URL, 10, 20, 0,
	)

	if len(got.Decks) != 2 {
		t.Fatalf("kept %d decks, want 2", len(got.Decks))
	}
	if got.Decks[0].PublicID != "yes1" || got.Decks[1].PublicID != "yes2" {
		t.Errorf("kept the wrong decks: %+v", got.Decks)
	}
	if got.Checked != 3 {
		t.Errorf("Checked = %d, want 3", got.Checked)
	}
	if got.Incomplete {
		t.Errorf("must not be marked incomplete: %q", got.Reason)
	}
}

func TestVerifyMoxfieldCommanderDecksStopsAtLimit(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
		_ = json.NewEncoder(w).Encode(commanderDeck(id, "Atraxa, Praetors' Voice"))
	}))
	defer server.Close()

	candidates := make([]MoxfieldDeckSummary, 0, 30)
	for i := range 30 {
		candidates = append(candidates, MoxfieldDeckSummary{PublicID: fmt.Sprintf("d%d", i)})
	}

	got := verifyMoxfieldCommanderDecks(
		context.Background(), candidates, "Atraxa, Praetors' Voice", server.URL, 3, 20, 0,
	)
	if len(got.Decks) != 3 {
		t.Fatalf("returned %d decks, want 3", len(got.Decks))
	}
	if requests != 3 {
		t.Errorf("made %d upstream requests, want 3 — verification must stop at the limit", requests)
	}
}

func TestVerifyMoxfieldCommanderDecksRespectsCheckBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
		_ = json.NewEncoder(w).Encode(commanderDeck(id, "Winota, Joiner of Forces"))
	}))
	defer server.Close()

	candidates := make([]MoxfieldDeckSummary, 0, 30)
	for i := range 30 {
		candidates = append(candidates, MoxfieldDeckSummary{PublicID: fmt.Sprintf("d%d", i)})
	}

	got := verifyMoxfieldCommanderDecks(
		context.Background(), candidates, "Atraxa, Praetors' Voice", server.URL, 10, 4, 0,
	)
	if got.Checked != 4 {
		t.Errorf("Checked = %d, want 4 (the budget)", got.Checked)
	}
	if !got.Incomplete || !strings.Contains(got.Reason, "budget") {
		t.Errorf("exhausted budget must be reported, got Incomplete=%v Reason=%q", got.Incomplete, got.Reason)
	}
}

func TestVerifyMoxfieldCommanderDecksStopsOnRateLimit(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
		_ = json.NewEncoder(w).Encode(commanderDeck(id, "Atraxa, Praetors' Voice"))
	}))
	defer server.Close()

	candidates := []MoxfieldDeckSummary{{PublicID: "a"}, {PublicID: "b"}, {PublicID: "c"}}
	got := verifyMoxfieldCommanderDecks(
		context.Background(), candidates, "Atraxa, Praetors' Voice", server.URL, 10, 20, 0,
	)

	if len(got.Decks) != 1 {
		t.Fatalf("kept %d decks, want the 1 verified before the 429", len(got.Decks))
	}
	if !got.Incomplete || !strings.Contains(got.Reason, "429") {
		t.Errorf("rate limit must be reported, got Incomplete=%v Reason=%q", got.Incomplete, got.Reason)
	}
	if requests != 2 {
		t.Errorf("made %d requests, want 2 — verification must stop at the first 429", requests)
	}
}

func TestIsMoxfieldRateLimited(t *testing.T) {
	if !isMoxfieldRateLimited(&moxfieldStatusError{StatusCode: http.StatusTooManyRequests}) {
		t.Error("429 must be detected as a rate limit")
	}
	if isMoxfieldRateLimited(&moxfieldStatusError{StatusCode: http.StatusNotFound}) {
		t.Error("404 must not be detected as a rate limit")
	}
	if isMoxfieldRateLimited(errors.New("network down")) {
		t.Error("a plain error must not be detected as a rate limit")
	}
}
