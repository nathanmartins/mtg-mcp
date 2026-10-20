package mtgserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/nathanmartins/mtg-mcp/internal/archidekt"
	"github.com/nathanmartins/mtg-mcp/internal/edhrec"
	"github.com/nathanmartins/mtg-mcp/internal/moxfield"
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
	deck := moxfield.Deck{
		PublicID: "abc123",
		Name:     "Atraxa Superfriends",
		Format:   "commander",
		Commanders: map[string]moxfield.CardEntry{
			"atraxa": {
				Quantity: 1,
				Card:     moxfield.CardInfo{Name: "Atraxa, Praetors' Voice", TypeLine: "Legendary Creature"},
			},
		},
		Mainboard: map[string]moxfield.CardEntry{
			"sol": {Quantity: 1, Card: moxfield.CardInfo{Name: "Sol Ring", TypeLine: "Artifact"}},
		},
	}
	s := &Server{moxfield: &moxfield.Client{BaseURL: jsonServer(t, http.StatusOK, deck)}}

	res, err := s.handleGetMoxfieldDeck(t.Context(), toolRequest(map[string]any{"deck_id": "abc123"}))
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
	s := &Server{moxfield: &moxfield.Client{BaseURL: jsonServer(t, http.StatusInternalServerError, nil)}}
	res, _ := s.handleGetMoxfieldDeck(t.Context(), toolRequest(map[string]any{"deck_id": "abc123"}))
	if !res.IsError {
		t.Error("expected error result on fetch failure")
	}
}

func TestHandleGetMoxfieldUserDecksSuccess(t *testing.T) {
	resp := moxfield.UserDecksResponse{
		PageNumber:   1,
		PageSize:     20,
		TotalResults: 1,
		TotalPages:   1,
		Data: []moxfield.DeckSummary{
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
	s := &Server{moxfield: &moxfield.Client{BaseURL: jsonServer(t, http.StatusOK, resp)}}

	res, err := s.handleGetMoxfieldUserDecks(t.Context(), toolRequest(map[string]any{
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
	s := &Server{moxfield: &moxfield.Client{BaseURL: jsonServer(t, http.StatusInternalServerError, nil)}}
	res, _ := s.handleGetMoxfieldUserDecks(t.Context(), toolRequest(map[string]any{"username": "tester"}))
	if !res.IsError {
		t.Error("expected error result on fetch failure")
	}
}

func TestHandleSearchMoxfieldDecks(t *testing.T) {
	t.Run("with results and overrides", func(t *testing.T) {
		resp := moxfield.SearchResponse{
			PageNumber:   3,
			TotalResults: 1,
			TotalPages:   3,
			Data: []moxfield.DeckSummary{
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
			_ = json.NewEncoder(w).Encode(moxfieldCommanderDeck(id, "Atraxa"))
		}))
		t.Cleanup(deckServer.Close)

		s := &Server{moxfield: &moxfield.Client{SearchURL: ts.URL, BaseURL: deckServer.URL}}
		res, err := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{
			"commander":      "Atraxa",
			"format":         "modern",
			"sort":           "views",
			"sort_direction": sortDirectionAsc,
			"limit":          float64(5),
			"page":           float64(3),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// pageSize is the over-fetch (limit × moxfield.CandidateFactor) clamped to
		// moxfield.VerifyMaxChecks: 5 × 5 = 25 candidates, but verification can only read 20,
		// and asking for more would strand the surplus (page 2 resumes past them).
		want := map[string]string{
			"cardName":      "Atraxa",
			"fmt":           "modern",
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
			_ = json.NewEncoder(w).Encode(moxfield.SearchResponse{PageNumber: 1, TotalPages: 0})
		}))
		t.Cleanup(ts.Close)

		s := &Server{moxfield: &moxfield.Client{SearchURL: ts.URL}}
		if _, err := s.handleSearchMoxfieldDecks(
			t.Context(), toolRequest(map[string]any{"commander": "Atraxa"}),
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := gotQuery.Get("pageSize"); got != "20" {
			t.Errorf("pageSize = %q, want %q (the verification budget)", got, "20")
		}
		if got := gotQuery.Get("sortType"); got != searchSortDefault {
			t.Errorf("sortType = %q, want %q (the default shared with the Archidekt tool)",
				got, searchSortDefault)
		}
	})

	t.Run("no results", func(t *testing.T) {
		resp := moxfield.SearchResponse{PageNumber: 1, TotalResults: 0, TotalPages: 0, Data: nil}
		s := &Server{moxfield: &moxfield.Client{SearchURL: jsonServer(t, http.StatusOK, resp)}}
		res, _ := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{"commander": "Nobody"}))
		if !strings.Contains(resultText(t, res), "No decks with this commander were found") {
			t.Error("expected no-decks message")
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &Server{moxfield: &moxfield.Client{SearchURL: jsonServer(t, http.StatusInternalServerError, nil)}}
		res, _ := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{"commander": "Atraxa"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})

	t.Run("invalid sort is rejected", func(t *testing.T) {
		// The stub answers 200 on any path, so only local validation can make this an error.
		s := &Server{moxfield: &moxfield.Client{SearchURL: jsonServer(t, http.StatusOK, moxfield.SearchResponse{})}}
		res, _ := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "sort": "price",
		}))
		if !res.IsError {
			t.Error("expected an error result for a sort Moxfield rejects")
		}
		if !strings.Contains(resultText(t, res), moxfield.SortValues()) {
			t.Errorf("error should list the accepted sorts:\n%s", resultText(t, res))
		}
	})

	t.Run("limit beyond the verification budget is rejected", func(t *testing.T) {
		// Verification confirms at most moxfield.VerifyMaxChecks decks per call, so a
		// larger limit could never be honoured and must not be silently truncated.
		s := &Server{moxfield: &moxfield.Client{SearchURL: jsonServer(t, http.StatusOK, moxfield.SearchResponse{})}}
		res, _ := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "limit": float64(moxfield.VerifyMaxChecks + 1),
		}))
		if !res.IsError {
			t.Fatal("expected an error result for a limit beyond the verification budget")
		}
		want := fmt.Sprintf("invalid limit %d (accepted: 1-%d)", moxfield.VerifyMaxChecks+1, moxfield.VerifyMaxChecks)
		if !strings.Contains(resultText(t, res), want) {
			t.Errorf("error should read %q:\n%s", want, resultText(t, res))
		}
	})

	t.Run("limit at the verification budget is accepted", func(t *testing.T) {
		var gotQuery url.Values
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			_ = json.NewEncoder(w).Encode(moxfield.SearchResponse{PageNumber: 1, TotalPages: 1})
		}))
		t.Cleanup(ts.Close)

		s := &Server{moxfield: &moxfield.Client{SearchURL: ts.URL}}
		res, err := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "limit": float64(moxfield.VerifyMaxChecks),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("limit %d is the documented maximum and must be accepted:\n%s",
				moxfield.VerifyMaxChecks, resultText(t, res))
		}
		if got := gotQuery.Get("pageSize"); got != strconv.Itoa(moxfield.VerifyMaxChecks) {
			t.Errorf("pageSize = %q, want %q", got, strconv.Itoa(moxfield.VerifyMaxChecks))
		}
	})

	t.Run("wrong-typed page is rejected", func(t *testing.T) {
		s := &Server{moxfield: &moxfield.Client{SearchURL: "http://127.0.0.1:1"}}
		res, _ := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "page": true,
		}))
		if !res.IsError || !strings.Contains(resultText(t, res), "expected a number") {
			t.Errorf("a boolean page must be rejected, got: %s", resultText(t, res))
		}
	})
}

func TestHandleSearchMoxfieldDecksDisclosesUnexaminedCandidates(t *testing.T) {
	// limit 1 is satisfied by the first candidate, so the other two are never fetched.
	// Page 2 resumes after this whole candidate page, so they are not merely deferred:
	// nothing will ever look at them, and the output must admit it.
	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(moxfield.SearchResponse{
			PageNumber: 1, TotalResults: 3, TotalPages: 1,
			Data: []moxfield.DeckSummary{
				{PublicID: "a", Name: "First Atraxa", Format: "commander"},
				{PublicID: "b", Name: "Second Atraxa", Format: "commander"},
				{PublicID: "c", Name: "Third Atraxa", Format: "commander"},
			},
		})
	}))
	defer searchServer.Close()

	deckServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
		_ = json.NewEncoder(w).Encode(moxfieldCommanderDeck(id, "Atraxa, Praetors' Voice"))
	}))
	defer deckServer.Close()

	s := &Server{moxfield: &moxfield.Client{SearchURL: searchServer.URL, BaseURL: deckServer.URL}}
	res, err := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{
		"commander": "Atraxa, Praetors' Voice",
		"limit":     float64(1),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	if !strings.Contains(text, "**Not examined on this page:** 2 candidate(s)") {
		t.Errorf("output must report the candidates paging will skip\n%s", text)
	}
	// Getting every requested deck is a success; only a 429, an exhausted budget or an
	// unreadable deck makes a run incomplete.
	if strings.Contains(text, "Verification incomplete") {
		t.Errorf("reaching the match limit must not be flagged as incomplete\n%s", text)
	}
}

func TestHandleSearchMoxfieldDecksVerifiesCommander(t *testing.T) {
	matching := moxfield.DeckSummary{PublicID: "yes1", Name: "Atraxa Superfriends", Format: "commander"}
	other := moxfield.DeckSummary{PublicID: "no1", Name: "Winota Blink", Format: "commander"}

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(moxfield.SearchResponse{
			PageNumber: 1, PageSize: 50, TotalResults: 2, TotalPages: 1,
			Data: []moxfield.DeckSummary{matching, other},
		})
	}))
	defer searchServer.Close()

	deckServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/decks/all/")
		commander := "Winota, Joiner of Forces"
		if id == "yes1" {
			commander = "Atraxa, Praetors' Voice"
		}
		_ = json.NewEncoder(w).Encode(moxfieldCommanderDeck(id, commander))
	}))
	defer deckServer.Close()

	s := &Server{moxfield: &moxfield.Client{SearchURL: searchServer.URL, BaseURL: deckServer.URL}}
	res, err := s.handleSearchMoxfieldDecks(t.Context(), toolRequest(map[string]any{
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
	// SortDirection holds Moxfield's wire value, which the handler produces; the output
	// must be rendered back in the asc/desc vocabulary the caller actually passed.
	params := moxfield.SearchParams{
		Format: "commander", SortType: "views", SortDirection: moxfield.DirectionDescending,
	}
	results := &moxfield.SearchResponse{PageNumber: 1, TotalPages: 1, TotalResults: 3}

	t.Run("unreadable decks are named", func(t *testing.T) {
		text := formatMoxfieldCommanderSearch("Atraxa, Praetors' Voice", params, results, moxfield.CommanderSearch{
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
		text := formatMoxfieldCommanderSearch("Atraxa, Praetors' Voice", params, results, moxfield.CommanderSearch{
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

	t.Run("the sort direction is echoed in the caller's vocabulary", func(t *testing.T) {
		text := formatMoxfieldCommanderSearch("Atraxa, Praetors' Voice", params, results,
			moxfield.CommanderSearch{Candidates: 3, Checked: 3})
		if !strings.Contains(text, "**Sort:** views (desc)") {
			t.Errorf("direction must read desc, not Moxfield's wire value\n%s", text)
		}

		ascending := params
		ascending.SortDirection = moxfield.DirectionAscending
		text = formatMoxfieldCommanderSearch("Atraxa, Praetors' Voice", ascending, results,
			moxfield.CommanderSearch{Candidates: 3, Checked: 3})
		if !strings.Contains(text, "**Sort:** views (asc)") {
			t.Errorf("an ascending search must read asc\n%s", text)
		}
	})
}

func sampleArchidektDeck() archidekt.Deck {
	return archidekt.Deck{
		ID:         123,
		Name:       "Test Deck",
		DeckFormat: 3,
		Cards: []archidekt.CardEntry{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card: archidekt.Card{
					OracleCard: archidekt.OracleCard{Name: "Atraxa, Praetors' Voice", Types: []string{"Creature"}},
				},
			},
			{
				Quantity:   1,
				Categories: []string{"Land"},
				Card: archidekt.Card{
					OracleCard: archidekt.OracleCard{Name: "Command Tower", Types: []string{"Land"}},
				},
			},
		},
	}
}

func TestHandleGetArchidektDeckSuccess(t *testing.T) {
	url := jsonServer(t, http.StatusOK, sampleArchidektDeck())

	t.Run("full deck", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: url}}
		res, err := s.handleGetArchidektDeck(t.Context(), toolRequest(map[string]any{"deck_id": "123"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Test Deck") {
			t.Errorf("expected deck name in output:\n%s", resultText(t, res))
		}
	})

	t.Run("lands only", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: url}}
		res, err := s.handleGetArchidektDeck(t.Context(), toolRequest(map[string]any{
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
	s := &Server{archidekt: &archidekt.Client{BaseURL: jsonServer(t, http.StatusInternalServerError, nil)}}
	res, _ := s.handleGetArchidektDeck(t.Context(), toolRequest(map[string]any{"deck_id": "123"}))
	if !res.IsError {
		t.Error("expected error result on fetch failure")
	}
}

func TestHandleGetArchidektUserDecks(t *testing.T) {
	t.Run("with results and pagination", func(t *testing.T) {
		resp := archidekt.UserDecksResponse{
			Count: 2,
			Next:  "https://archidekt.com/api/decks/v3/?page=2",
			Results: []archidekt.DeckSummary{
				{ID: 1, Name: "Deck A", DeckFormat: 3, ViewCount: 5, UpdatedAt: "2024-01-01"},
			},
		}
		s := &Server{archidekt: &archidekt.Client{BaseURL: jsonServer(t, http.StatusOK, resp)}}
		res, err := s.handleGetArchidektUserDecks(t.Context(), toolRequest(map[string]any{
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
		resp := archidekt.UserDecksResponse{Count: 0, Next: "", Results: nil}
		s := &Server{archidekt: &archidekt.Client{BaseURL: jsonServer(t, http.StatusOK, resp)}}
		res, _ := s.handleGetArchidektUserDecks(t.Context(), toolRequest(map[string]any{"username": "tester"}))
		if !strings.Contains(resultText(t, res), "No public decks found") {
			t.Error("expected no-decks message")
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: jsonServer(t, http.StatusInternalServerError, nil)}}
		res, _ := s.handleGetArchidektUserDecks(t.Context(), toolRequest(map[string]any{"username": "tester"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}

func TestHandleSearchArchidektDecks(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := archidekt.UserDecksResponse{
			Count: 1,
			Results: []archidekt.DeckSummary{
				{ID: 9, Name: "Atraxa Brew", DeckFormat: 3, ViewCount: 99, UpdatedAt: "2024-02-02"},
			},
		}
		s := &Server{archidekt: &archidekt.Client{BaseURL: jsonServer(t, http.StatusOK, resp)}}
		res, err := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
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

	t.Run("colour filter is echoed as it was sent", func(t *testing.T) {
		var gotQuery url.Values
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			_ = json.NewEncoder(w).Encode(archidekt.UserDecksResponse{Count: 1, Results: []archidekt.DeckSummary{
				{ID: 7, Name: "Budget Atraxa", Owner: archidekt.Owner{Username: "player"}},
			}})
		}))
		t.Cleanup(ts.Close)

		s := &Server{archidekt: &archidekt.Client{BaseURL: ts.URL}}
		res, err := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "colors": " wu",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := gotQuery.Get("colors"); got != "W,U" {
			t.Errorf("colors = %q, want %q", got, "W,U")
		}
		if text := resultText(t, res); !strings.Contains(text, "colors WU") {
			t.Errorf("output must echo the normalised filter:\n%s", text)
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: jsonServer(t, http.StatusInternalServerError, nil)}}
		res, _ := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{"commander": "Atraxa"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})

	t.Run("invalid sort is rejected", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: "http://127.0.0.1:1"}}
		res, _ := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "sort": "trending",
		}))
		if !res.IsError || !strings.Contains(resultText(t, res), "unsupported sort") {
			t.Errorf("expected an unsupported-sort error result, got IsError=%v: %s",
				res.IsError, resultText(t, res))
		}
	})

	t.Run("invalid colors are rejected", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: "http://127.0.0.1:1"}}
		res, _ := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "colors": "WX",
		}))
		if !res.IsError {
			t.Error("expected an error result for an invalid colors value")
		}
	})

	t.Run("wrong-typed limit is rejected", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: "http://127.0.0.1:1"}}
		res, _ := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "limit": "5",
		}))
		if !res.IsError || !strings.Contains(resultText(t, res), "expected a number") {
			t.Errorf("a string limit must be rejected, got: %s", resultText(t, res))
		}
	})

	t.Run("empty sort is rejected", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: "http://127.0.0.1:1"}}
		res, _ := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "sort": "",
		}))
		if !res.IsError || !strings.Contains(resultText(t, res), "non-empty") {
			t.Errorf("an empty sort must be rejected, got: %s", resultText(t, res))
		}
	})

	t.Run("page beyond the reachable range is rejected", func(t *testing.T) {
		// (page-1)*limit used to overflow int here, producing a negative window offset
		// and panicking on collected[offset:end] after a perfectly successful fetch.
		s := &Server{
			archidekt: &archidekt.Client{BaseURL: jsonServer(t, http.StatusOK, archidekt.UserDecksResponse{
				Count:   5,
				Results: []archidekt.DeckSummary{{ID: 1, Name: "Deck"}},
			})},
		}
		res, err := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "page": float64(200000000000000000), "limit": float64(60),
		}))
		if err != nil {
			t.Fatalf("handler returned a transport error: %v", err)
		}
		if !res.IsError || !strings.Contains(resultText(t, res), "invalid page") {
			t.Errorf("an unreachable page must be rejected, got: %s", resultText(t, res))
		}
	})

	t.Run("fractional limit is rejected", func(t *testing.T) {
		s := &Server{archidekt: &archidekt.Client{BaseURL: "http://127.0.0.1:1"}}
		res, _ := s.handleSearchArchidektDecks(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa", "limit": 7.9,
		}))
		if !res.IsError || !strings.Contains(resultText(t, res), "whole number") {
			t.Errorf("a fractional limit must be rejected, got: %s", resultText(t, res))
		}
	})
}

func TestHandleGetEDHRECRecommendations(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := edhrec.Response{
			Container: edhrec.Container{
				JSONDict: edhrec.Data{
					Card: edhrec.CardInfo{
						Name: "Atraxa, Praetors' Voice", ColorID: []string{"W", "U", "B", "G"}, NumDecks: 1000,
					},
					CardLists: []edhrec.CardList{
						{
							Header:    "High Synergy Cards",
							CardViews: []edhrec.CardView{{Name: "Doubling Season", NumDecks: 500}},
						},
					},
				},
			},
		}
		s := &Server{edhrecBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, err := s.handleGetEDHRECRecommendations(t.Context(), toolRequest(map[string]any{
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
		s := &Server{edhrecBaseURL: jsonServer(t, http.StatusNotFound, nil)}
		res, _ := s.handleGetEDHRECRecommendations(
			t.Context(),
			toolRequest(map[string]any{"commander": "Nobody"}),
		)
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}

func TestHandleGetEDHRECCombos(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := edhrec.ComboResponse{
			Container: edhrec.ComboContainer{
				JSONDict: edhrec.ComboData{
					CardLists: []edhrec.ComboList{
						{
							Header:    "Basalt Monolith + Forsaken Monument",
							CardViews: []edhrec.CardView{{Name: "Basalt Monolith"}, {Name: "Forsaken Monument"}},
							Combo:     &edhrec.Combo{ComboID: "c1", Results: []string{"Infinite mana"}},
						},
					},
				},
			},
		}
		s := &Server{edhrecBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, err := s.handleGetEDHRECCombos(t.Context(), toolRequest(map[string]any{
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
		s := &Server{edhrecBaseURL: jsonServer(t, http.StatusNotFound, nil)}
		res, _ := s.handleGetEDHRECCombos(t.Context(), toolRequest(map[string]any{"colors": "zz"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}

func TestHandleGetEDHRECSetCards(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := edhrec.Response{
			Header: "Ravnica Allegiance",
			Container: edhrec.Container{
				JSONDict: edhrec.Data{
					CardLists: []edhrec.CardList{
						{
							Header:    "Commanders",
							CardViews: []edhrec.CardView{{Name: "Teysa Karlov", NumDecks: 21469}},
						},
					},
				},
			},
		}
		s := &Server{edhrecBaseURL: jsonServer(t, http.StatusOK, resp)}
		res, err := s.handleGetEDHRECSetCards(t.Context(), toolRequest(map[string]any{"set": "rna"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Teysa Karlov") {
			t.Errorf("expected card name in output:\n%s", resultText(t, res))
		}
	})

	t.Run("failure", func(t *testing.T) {
		s := &Server{edhrecBaseURL: jsonServer(t, http.StatusForbidden, nil)}
		res, _ := s.handleGetEDHRECSetCards(t.Context(), toolRequest(map[string]any{"set": "notaset"}))
		if !res.IsError {
			t.Error("expected error result")
		}
		if !strings.Contains(resultText(t, res), "notaset") {
			t.Errorf("expected set code in error message:\n%s", resultText(t, res))
		}
	})
}

func TestHandleGetEDHRECRecommendationsThemeHint(t *testing.T) {
	t.Run("invalid theme suggests the real slug", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/commanders/atraxa-praetors-voice/infekt.json"):
				w.WriteHeader(http.StatusForbidden)
			case strings.HasSuffix(r.URL.Path, "/commanders/atraxa-praetors-voice.json"):
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(edhrec.Response{
					TagCounts: []edhrec.TagCount{{Slug: "infect", Value: "Infect", Count: 4066}},
				})
			default:
				w.WriteHeader(http.StatusForbidden)
			}
		}))
		defer ts.Close()

		s := &Server{edhrecBaseURL: ts.URL}
		res, err := s.handleGetEDHRECRecommendations(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa, Praetors' Voice",
			"theme":     "infekt",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Fatal("expected error result")
		}
		text := resultText(t, res)
		if !strings.Contains(text, "infect") || !strings.Contains(text, "4066") {
			t.Errorf("expected theme hint naming infect (4066 decks), got:\n%s", text)
		}
	})

	t.Run("valid-looking theme that still fails does not misreport an outage", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer ts.Close()

		s := &Server{edhrecBaseURL: ts.URL}
		res, err := s.handleGetEDHRECRecommendations(t.Context(), toolRequest(map[string]any{
			"commander": "Atraxa, Praetors' Voice",
			"theme":     "infect",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Fatal("expected error result")
		}
		if !strings.Contains(resultText(t, res), "Failed to fetch EDHREC recommendations") {
			t.Errorf("expected generic failure message, got:\n%s", resultText(t, res))
		}
	})
}

func TestHandleGetEDHRECRecommendationsBadPriceTier(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("no HTTP request should be made for an invalid price tier")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := &Server{edhrecBaseURL: ts.URL}
	res, err := s.handleGetEDHRECRecommendations(t.Context(), toolRequest(map[string]any{
		"commander":  "Atraxa, Praetors' Voice",
		"price_tier": "middle",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

// moxfieldCommanderDeck builds a Moxfield deck whose commander zone holds the given names.
func moxfieldCommanderDeck(publicID string, commanders ...string) moxfield.Deck {
	zone := make(map[string]moxfield.CardEntry, len(commanders))
	for _, name := range commanders {
		zone[name] = moxfield.CardEntry{Quantity: 1, Card: moxfield.CardInfo{Name: name}}
	}
	return moxfield.Deck{PublicID: publicID, Name: "deck-" + publicID, Format: "commander", Commanders: zone}
}
