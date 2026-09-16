package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// jsonServer starts a test server that replies to every request with the given value as JSON.
func jsonServer(t *testing.T, status int, body any) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestHandleGetMoxfieldDeckSuccess(t *testing.T) {
	deck := MoxfieldDeck{
		PublicID: "abc123",
		Name:     "Atraxa Superfriends",
		Format:   "commander",
		Commanders: map[string]MoxfieldCardEntry{
			"atraxa": {
				Quantity: 1,
				Card:     MoxfieldCardInfo{Name: "Atraxa, Praetors' Voice", TypeLine: "Legendary Creature"},
			},
		},
		Mainboard: map[string]MoxfieldCardEntry{
			"sol": {Quantity: 1, Card: MoxfieldCardInfo{Name: "Sol Ring", TypeLine: "Artifact"}},
		},
	}
	s := &MTGCommanderServer{moxfieldBaseURL: jsonServer(t, http.StatusOK, deck)}

	res, err := s.handleGetMoxfieldDeck(context.Background(), toolRequest(map[string]any{"deck_id": "abc123"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatal("unexpected error result")
	}
	if !strings.Contains(resultText(t, res), "Atraxa Superfriends") {
		t.Errorf("expected deck name in output:\n%s", resultText(t, res))
	}
}

func TestHandleGetMoxfieldDeckFailure(t *testing.T) {
	s := &MTGCommanderServer{moxfieldBaseURL: jsonServer(t, http.StatusInternalServerError, nil)}
	res, _ := s.handleGetMoxfieldDeck(context.Background(), toolRequest(map[string]any{"deck_id": "abc123"}))
	if !res.IsError {
		t.Error("expected error result on fetch failure")
	}
}

func TestHandleGetMoxfieldUserDecksSuccess(t *testing.T) {
	resp := MoxfieldUserDecksResponse{
		PageNumber:   1,
		PageSize:     20,
		TotalResults: 1,
		TotalPages:   1,
		Data: []MoxfieldDeckSummary{
			{
				PublicID:  "d1",
				Name:      "Deck One",
				Format:    "commander",
				PublicURL: "https://moxfield.com/d1",
				ViewCount: 10,
				LikeCount: 2,
			},
		},
	}
	s := &MTGCommanderServer{moxfieldBaseURL: jsonServer(t, http.StatusOK, resp)}

	res, err := s.handleGetMoxfieldUserDecks(context.Background(), toolRequest(map[string]any{
		"username":  "tester",
		"page_size": float64(200), // exercises the max clamp
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, res)
	for _, want := range []string{"Decks by tester", "Deck One", "Deck ID: d1"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\n%s", want, text)
		}
	}
}

func TestHandleGetMoxfieldUserDecksFailure(t *testing.T) {
	s := &MTGCommanderServer{moxfieldBaseURL: jsonServer(t, http.StatusInternalServerError, nil)}
	res, _ := s.handleGetMoxfieldUserDecks(context.Background(), toolRequest(map[string]any{"username": "tester"}))
	if !res.IsError {
		t.Error("expected error result on fetch failure")
	}
}

func TestHandleSearchMoxfieldDecks(t *testing.T) {
	t.Run("with results and overrides", func(t *testing.T) {
		resp := MoxfieldSearchResponse{
			PageNumber:   3,
			TotalResults: 1,
			TotalPages:   3,
			Data: []MoxfieldDeckSummary{
				{PublicID: "s1", Name: "Found Deck", Format: "commander", PublicURL: "https://moxfield.com/s1"},
			},
		}
		// Capture the outgoing request: the MCP argument names must map onto the
		// Moxfield parameters, and every value below differs from the handler default.
		var gotQuery url.Values
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			_ = json.NewEncoder(w).Encode(resp)
		}))
		t.Cleanup(ts.Close)

		// Verification fetches every candidate, so the deck read has to confirm the commander.
		deckServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
			_ = json.NewEncoder(w).Encode(commanderDeck(id, "Atraxa"))
		}))
		t.Cleanup(deckServer.Close)

		s := &MTGCommanderServer{moxfieldSearchURL: ts.URL, moxfieldBaseURL: deckServer.URL}
		res, err := s.handleSearchMoxfieldDecks(context.Background(), toolRequest(map[string]any{
			"commander":      "Atraxa",
			"format":         "commander",
			"sort":           "views",
			"sort_direction": sortDirectionAsc,
			"limit":          float64(5),
			"page":           float64(3),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// pageSize is the over-fetch (limit × moxfieldCandidateFactor) clamped to
		// moxfieldVerifyMaxChecks: 5 × 5 = 25 candidates, but verification can only read 20,
		// and asking for more would strand the surplus (page 2 resumes past them).
		want := map[string]string{
			"cardName":      "Atraxa",
			"fmt":           "commander",
			"sortType":      "views",
			"sortDirection": "Ascending",
			"pageSize":      "20",
			"pageNumber":    "3",
		}
		for key, value := range want {
			if gotQuery.Get(key) != value {
				t.Errorf("%s = %q, want %q", key, gotQuery.Get(key), value)
			}
		}
		if !strings.Contains(resultText(t, res), "Found Deck") {
			t.Errorf("expected deck in output:\n%s", resultText(t, res))
		}
	})

	t.Run("default limit never asks for more candidates than verification can read", func(t *testing.T) {
		// The default limit of 10 would over-fetch 50 candidates, but only the first 20 can
		// ever be verified and page 2 starts after candidate 50, so 21-50 would be lost.
		var gotQuery url.Values
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			_ = json.NewEncoder(w).Encode(MoxfieldSearchResponse{PageNumber: 1, TotalPages: 0})
		}))
		t.Cleanup(ts.Close)

		s := &MTGCommanderServer{moxfieldSearchURL: ts.URL}
		if _, err := s.handleSearchMoxfieldDecks(
			context.Background(), toolRequest(map[string]any{"commander": "Atraxa"}),
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := gotQuery.Get("pageSize"); got != "20" {
			t.Errorf("pageSize = %q, want %q (the verification budget)", got, "20")
		}
	})

	t.Run("no results", func(t *testing.T) {
		resp := MoxfieldSearchResponse{PageNumber: 1, TotalResults: 0, TotalPages: 0, Data: nil}
		s := &MTGCommanderServer{moxfieldSearchURL: jsonServer(t, http.StatusOK, resp)}
		res, _ := s.handleSearchMoxfieldDecks(context.Background(), toolRequest(map[string]any{"commander": "Nobody"}))
		if !strings.Contains(resultText(t, res), "No decks with this commander were found") {
			t.Error("expected no-decks message")
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &MTGCommanderServer{moxfieldSearchURL: jsonServer(t, http.StatusInternalServerError, nil)}
		res, _ := s.handleSearchMoxfieldDecks(context.Background(), toolRequest(map[string]any{"commander": "Atraxa"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})

	t.Run("invalid sort is rejected", func(t *testing.T) {
		// The stub answers 200 on any path, so only local validation can make this an error.
		s := &MTGCommanderServer{moxfieldSearchURL: jsonServer(t, http.StatusOK, MoxfieldSearchResponse{})}
		res, _ := s.handleSearchMoxfieldDecks(context.Background(), toolRequest(map[string]any{
			"commander": "Atraxa", "sort": "price",
		}))
		if !res.IsError {
			t.Error("expected an error result for a sort Moxfield rejects")
		}
		if !strings.Contains(resultText(t, res), moxfieldSortValues) {
			t.Errorf("error should list the accepted sorts:\n%s", resultText(t, res))
		}
	})
}

func TestHandleSearchMoxfieldDecksVerifiesCommander(t *testing.T) {
	matching := MoxfieldDeckSummary{PublicID: "yes1", Name: "Atraxa Superfriends", Format: "commander"}
	other := MoxfieldDeckSummary{PublicID: "no1", Name: "Winota Blink", Format: "commander"}

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(MoxfieldSearchResponse{
			PageNumber: 1, PageSize: 50, TotalResults: 2, TotalPages: 1,
			Data: []MoxfieldDeckSummary{matching, other},
		})
	}))
	defer searchServer.Close()

	deckServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
		commander := "Winota, Joiner of Forces"
		if id == "yes1" {
			commander = "Atraxa, Praetors' Voice"
		}
		_ = json.NewEncoder(w).Encode(commanderDeck(id, commander))
	}))
	defer deckServer.Close()

	s := &MTGCommanderServer{moxfieldSearchURL: searchServer.URL, moxfieldBaseURL: deckServer.URL}
	res, err := s.handleSearchMoxfieldDecks(context.Background(), toolRequest(map[string]any{
		"commander": "Atraxa, Praetors' Voice",
		"limit":     float64(5),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	if !strings.Contains(text, "Atraxa Superfriends") {
		t.Errorf("verified deck missing from output\n%s", text)
	}
	if strings.Contains(text, "Winota Blink") {
		t.Errorf("deck with a different commander must be filtered out\n%s", text)
	}
	if !strings.Contains(text, "Verified") {
		t.Errorf("output must state how many candidates were verified\n%s", text)
	}
}

func TestFormatMoxfieldCommanderSearchReportsUnfetchableCandidates(t *testing.T) {
	params := MoxfieldSearchParams{Format: "commander", SortType: "views", SortDirection: sortDirectionDesc}
	results := &MoxfieldSearchResponse{PageNumber: 1, TotalPages: 1, TotalResults: 3}

	t.Run("unreadable decks are named", func(t *testing.T) {
		text := formatMoxfieldCommanderSearch("Atraxa, Praetors' Voice", params, results, MoxfieldCommanderSearch{
			Candidates: 3,
			Checked:    3,
			Failed:     2,
			Incomplete: true,
			Reason:     "2 candidate deck(s) could not be fetched",
		})
		if !strings.Contains(text, "**Verified as commander:** 0 of 3 checked, 2 could not be fetched") {
			t.Errorf("checked count must disclose the unreadable decks\n%s", text)
		}
		if !strings.Contains(text, "Verification incomplete") {
			t.Errorf("unreadable decks make the verification incomplete\n%s", text)
		}
	})

	t.Run("a clean verification says nothing about fetch failures", func(t *testing.T) {
		text := formatMoxfieldCommanderSearch("Atraxa, Praetors' Voice", params, results, MoxfieldCommanderSearch{
			Candidates: 3,
			Checked:    3,
		})
		if !strings.Contains(text, "**Verified as commander:** 0 of 3 checked\n") {
			t.Errorf("clean verification must report the bare counts\n%s", text)
		}
		if strings.Contains(text, "could not be fetched") || strings.Contains(text, "incomplete") {
			t.Errorf("nothing failed, so nothing may be flagged\n%s", text)
		}
	})
}

func sampleArchidektDeck() ArchidektDeck {
	return ArchidektDeck{
		ID:         123,
		Name:       "Test Deck",
		DeckFormat: 3,
		Cards: []ArchidektCardEntry{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Atraxa, Praetors' Voice", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Land"},
				Card: ArchidektCard{
					OracleCard: ArchidektOracleCard{Name: "Command Tower", Types: []string{"Land"}},
				},
			},
		},
	}
}

func TestHandleGetArchidektDeckSuccess(t *testing.T) {
	url := jsonServer(t, http.StatusOK, sampleArchidektDeck())

	t.Run("full deck", func(t *testing.T) {
		s := &MTGCommanderServer{archidektBaseURL: url}
		res, err := s.handleGetArchidektDeck(context.Background(), toolRequest(map[string]any{"deck_id": "123"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Test Deck") {
			t.Errorf("expected deck name in output:\n%s", resultText(t, res))
		}
	})

	t.Run("lands only", func(t *testing.T) {
		s := &MTGCommanderServer{archidektBaseURL: url}
		res, err := s.handleGetArchidektDeck(context.Background(), toolRequest(map[string]any{
			"deck_id":    "123",
			"lands_only": true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Command Tower") {
			t.Errorf("expected land in lands-only output:\n%s", resultText(t, res))
		}
	})
}

func TestHandleGetArchidektDeckFailure(t *testing.T) {
	s := &MTGCommanderServer{archidektBaseURL: jsonServer(t, http.StatusInternalServerError, nil)}
	res, _ := s.handleGetArchidektDeck(context.Background(), toolRequest(map[string]any{"deck_id": "123"}))
	if !res.IsError {
		t.Error("expected error result on fetch failure")
	}
}

func TestHandleGetArchidektUserDecks(t *testing.T) {
	t.Run("with results and pagination", func(t *testing.T) {
		resp := ArchidektUserDecksResponse{
			Count: 2,
			Next:  "https://archidekt.com/api/decks/v3/?page=2",
			Results: []ArchidektDeckSummary{
				{ID: 1, Name: "Deck A", DeckFormat: 3, ViewCount: 5, UpdatedAt: "2024-01-01"},
			},
		}
		s := &MTGCommanderServer{archidektBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, err := s.handleGetArchidektUserDecks(context.Background(), toolRequest(map[string]any{
			"username": "tester",
			"page":     float64(1),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		for _, want := range []string{"Archidekt Decks by tester", "Deck A", "use page 2 to continue"} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
	})

	t.Run("no results", func(t *testing.T) {
		resp := ArchidektUserDecksResponse{Count: 0, Next: "", Results: nil}
		s := &MTGCommanderServer{archidektBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, _ := s.handleGetArchidektUserDecks(context.Background(), toolRequest(map[string]any{"username": "tester"}))
		if !strings.Contains(resultText(t, res), "No public decks found") {
			t.Error("expected no-decks message")
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &MTGCommanderServer{archidektBaseURL: jsonServer(t, http.StatusInternalServerError, nil)}
		res, _ := s.handleGetArchidektUserDecks(context.Background(), toolRequest(map[string]any{"username": "tester"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}

func TestHandleSearchArchidektDecks(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := ArchidektUserDecksResponse{
			Count: 1,
			Results: []ArchidektDeckSummary{
				{ID: 9, Name: "Atraxa Brew", DeckFormat: 3, ViewCount: 99, UpdatedAt: "2024-02-02"},
			},
		}
		s := &MTGCommanderServer{archidektBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, err := s.handleSearchArchidektDecks(context.Background(), toolRequest(map[string]any{
			"commander": "Atraxa, Praetors' Voice",
			"bracket":   float64(4),
			"limit":     float64(5),
			"page":      float64(1),
			"sort":      "updated",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Atraxa Brew") {
			t.Errorf("expected deck in output:\n%s", resultText(t, res))
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &MTGCommanderServer{archidektBaseURL: jsonServer(t, http.StatusInternalServerError, nil)}
		res, _ := s.handleSearchArchidektDecks(context.Background(), toolRequest(map[string]any{"commander": "Atraxa"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})

	t.Run("invalid sort is rejected", func(t *testing.T) {
		s := &MTGCommanderServer{archidektBaseURL: "http://127.0.0.1:1"}
		res, _ := s.handleSearchArchidektDecks(context.Background(), toolRequest(map[string]any{
			"commander": "Atraxa", "sort": "trending",
		}))
		if !res.IsError || !strings.Contains(resultText(t, res), "unsupported sort") {
			t.Errorf("expected an unsupported-sort error result, got IsError=%v: %s",
				res.IsError, resultText(t, res))
		}
	})

	t.Run("invalid colors are rejected", func(t *testing.T) {
		s := &MTGCommanderServer{archidektBaseURL: "http://127.0.0.1:1"}
		res, _ := s.handleSearchArchidektDecks(context.Background(), toolRequest(map[string]any{
			"commander": "Atraxa", "colors": "WX",
		}))
		if !res.IsError {
			t.Error("expected an error result for an invalid colors value")
		}
	})
}

func TestHandleGetEDHRECRecommendations(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := EDHRECResponse{
			Container: EDHRECContainer{
				JSONDict: EDHRECData{
					Card:     EDHRECCardInfo{Name: "Atraxa, Praetors' Voice", ColorID: []string{"W", "U", "B", "G"}},
					NumDecks: 1000,
					CardLists: []EDHRECCardList{
						{
							Header:    "High Synergy Cards",
							CardViews: []EDHRECCardView{{Name: "Doubling Season", Inclusion: 500}},
						},
					},
				},
			},
		}
		s := &MTGCommanderServer{edhrecBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, err := s.handleGetEDHRECRecommendations(context.Background(), toolRequest(map[string]any{
			"commander": "Atraxa, Praetors' Voice",
			"limit":     float64(5),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Doubling Season") {
			t.Errorf("expected recommendation in output:\n%s", resultText(t, res))
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &MTGCommanderServer{edhrecBaseURL: jsonServer(t, http.StatusNotFound, nil)}
		res, _ := s.handleGetEDHRECRecommendations(
			context.Background(),
			toolRequest(map[string]any{"commander": "Nobody"}),
		)
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}

func TestHandleGetEDHRECCombos(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := EDHRECComboResponse{
			Container: EDHRECComboContainer{
				JSONDict: EDHRECComboData{
					CardLists: []EDHRECComboList{
						{
							Header:    "Basalt Monolith + Forsaken Monument",
							CardViews: []EDHRECCardView{{Name: "Basalt Monolith"}, {Name: "Forsaken Monument"}},
							Combo:     &EDHRECCombo{ComboID: "c1", Results: []string{"Infinite mana"}},
						},
					},
				},
			},
		}
		s := &MTGCommanderServer{edhrecBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, err := s.handleGetEDHRECCombos(context.Background(), toolRequest(map[string]any{
			"colors": "colorless",
			"limit":  float64(5),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Infinite mana") {
			t.Errorf("expected combo in output:\n%s", resultText(t, res))
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &MTGCommanderServer{edhrecBaseURL: jsonServer(t, http.StatusNotFound, nil)}
		res, _ := s.handleGetEDHRECCombos(context.Background(), toolRequest(map[string]any{"colors": "zz"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}
