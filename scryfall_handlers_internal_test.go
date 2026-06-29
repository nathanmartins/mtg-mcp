package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	scryfall "github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
)

// newTestScryfallServer spins up an httptest server with the given handler and
// returns an MTGCommanderServer whose Scryfall client targets it.
func newTestScryfallServer(t *testing.T, handler http.HandlerFunc) *MTGCommanderServer {
	t.Helper()

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client, err := scryfall.NewClient(scryfall.WithBaseURL(ts.URL), scryfall.WithLimiter(nil))
	if err != nil {
		t.Fatalf("failed to create scryfall test client: %v", err)
	}

	return &MTGCommanderServer{scryfallClient: client}
}

// toolRequest builds a CallToolRequest carrying the supplied arguments.
func toolRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

// resultText extracts the text payload from a tool result.
func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()

	if res == nil {
		t.Fatal("result is nil")
	}
	if len(res.Content) == 0 {
		t.Fatal("result has no content")
	}

	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("result content is not text: %T", res.Content[0])
	}

	return tc.Text
}

// jsonResponse writes a 200 JSON body.
func jsonResponse(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

// scryfallError writes a Scryfall-style error response with the given status.
func scryfallError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `{"object":"error","code":"not_found","status":404,"details":"not found"}`)
}

const solRingJSON = `{
	"id": "sol-ring-id",
	"name": "Sol Ring",
	"mana_cost": "{1}",
	"type_line": "Artifact",
	"oracle_text": "{T}: Add {C}{C}.",
	"set": "cmr",
	"set_name": "Commander Legends",
	"collector_number": "472",
	"rarity": "uncommon",
	"color_identity": [],
	"scryfall_uri": "https://scryfall.com/card/cmr/472",
	"legalities": {
		"standard": "not_legal",
		"pioneer": "not_legal",
		"modern": "not_legal",
		"legacy": "legal",
		"vintage": "legal",
		"pauper": "not_legal",
		"commander": "legal"
	},
	"prices": {"usd": "1.50", "usd_foil": "5.00", "eur": "1.20", "eur_foil": "4.10", "tix": "0.05"}
}`

func TestHandleSearchCards(t *testing.T) {
	t.Run("missing query returns error", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {
			t.Error("scryfall should not be called when query is missing")
		})

		res, err := s.handleSearchCards(context.Background(), toolRequest(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Error("expected error result for missing query")
		}
	})

	t.Run("successful search renders cards", func(t *testing.T) {
		body := `{"object":"list","total_cards":1,"has_more":false,"data":[` + solRingJSON + `]}`
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/cards/search") {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
			jsonResponse(w, body)
		})

		res, err := s.handleSearchCards(context.Background(), toolRequest(map[string]any{"query": "sol ring"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatal("unexpected error result")
		}
		text := resultText(t, res)
		for _, want := range []string{"Sol Ring", "Found 1 cards", "Type: Artifact", "Commander Legal: legal"} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
	})

	t.Run("no results", func(t *testing.T) {
		body := `{"object":"list","total_cards":0,"has_more":false,"data":[]}`
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, body)
		})

		res, err := s.handleSearchCards(context.Background(), toolRequest(map[string]any{"query": "zzzz"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "No cards found") {
			t.Error("expected no-cards message")
		}
	})

	t.Run("limit is clamped and applied", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString(`{"object":"list","total_cards":3,"has_more":false,"data":[`)
		sb.WriteString(solRingJSON + "," + solRingJSON + "," + solRingJSON)
		sb.WriteString(`]}`)
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, sb.String())
		})

		// limit above maxSearchLimit gets clamped; only 1 card requested via small limit here.
		res, err := s.handleSearchCards(context.Background(), toolRequest(map[string]any{
			"query": "sol ring",
			"limit": float64(1),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		if strings.Count(text, "**Sol Ring**") != 1 {
			t.Errorf("expected limit to trim to 1 card, got:\n%s", text)
		}
		if !strings.Contains(text, "showing first 1") {
			t.Errorf("expected 'showing first 1', got:\n%s", text)
		}
	})

	t.Run("search failure returns error result", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusInternalServerError)
		})

		res, err := s.handleSearchCards(context.Background(), toolRequest(map[string]any{"query": "x"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Error("expected error result on search failure")
		}
	})
}

func TestHandleGetCardDetails(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {})
		res, _ := s.handleGetCardDetails(context.Background(), toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing name")
		}
	})

	t.Run("full card details", func(t *testing.T) {
		power, toughness, loyalty, artist := "2", "3", "4", "Some Artist"
		card := `{
			"id":"id1","name":"Test Walker","mana_cost":"{2}{G}","type_line":"Legendary Planeswalker",
			"oracle_text":"Do things.","set":"tst","set_name":"Test Set","collector_number":"1","rarity":"mythic",
			"power":"` + power + `","toughness":"` + toughness + `","loyalty":"` + loyalty + `",
			"color_identity":["G","U"],"artist":"` + artist + `","scryfall_uri":"https://scryfall.com/x",
			"legalities":{"commander":"legal","legacy":"legal","vintage":"legal","modern":"legal","standard":"legal"}
		}`
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/cards/named") {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
			jsonResponse(w, card)
		})

		res, err := s.handleGetCardDetails(context.Background(), toolRequest(map[string]any{"name": "Test Walker"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		for _, want := range []string{
			"# Test Walker", "Legendary Planeswalker", "Power/Toughness:** 2/3",
			"Loyalty:** 4", "Color Identity:** G, U", "Artist:** Some Artist", "Commander: legal",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
	})

	t.Run("card not found", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusNotFound)
		})
		res, _ := s.handleGetCardDetails(context.Background(), toolRequest(map[string]any{"name": "nope"}))
		if !res.IsError {
			t.Error("expected error result for missing card")
		}
	})
}

func TestHandleCheckLegality(t *testing.T) {
	cases := []struct {
		name      string
		legality  string
		wantInOut string
	}{
		{"legal", "legal", "LEGAL"},
		{"banned", "banned", "BANNED"},
		{"not legal", "not_legal", "NOT LEGAL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card := `{"id":"id","name":"Card X","type_line":"Creature","legalities":{"commander":"` +
				tc.legality + `","standard":"legal","pioneer":"legal","modern":"legal","legacy":"legal",` +
				`"vintage":"legal","pauper":"legal"}}`
			s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
				jsonResponse(w, card)
			})
			res, err := s.handleCheckLegality(context.Background(), toolRequest(map[string]any{"name": "Card X"}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			text := resultText(t, res)
			if !strings.Contains(text, tc.wantInOut) {
				t.Errorf("expected %q in output:\n%s", tc.wantInOut, text)
			}
		})
	}

	t.Run("missing name", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {})
		res, _ := s.handleCheckLegality(context.Background(), toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing name")
		}
	})

	t.Run("card not found", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusNotFound)
		})
		res, _ := s.handleCheckLegality(context.Background(), toolRequest(map[string]any{"name": "nope"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}

func TestHandleGetRulings(t *testing.T) {
	cardJSON := `{"id":"ruling-card","name":"Doubling Season","legalities":{"commander":"legal"}}`

	t.Run("missing name", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {})
		res, _ := s.handleGetRulings(context.Background(), toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing name")
		}
	})

	t.Run("rulings present", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/cards/named"):
				jsonResponse(w, cardJSON)
			case strings.HasSuffix(r.URL.Path, "/rulings"):
				jsonResponse(w, `{"object":"list","has_more":false,"data":[`+
					`{"source":"wotc","published_at":"2017-09-29","comment":"It works like this."}]}`)
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
			}
		})

		res, err := s.handleGetRulings(context.Background(), toolRequest(map[string]any{"name": "Doubling Season"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		for _, want := range []string{"Doubling Season", "Found 1 ruling", "It works like this."} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
	})

	t.Run("no rulings", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/cards/named") {
				jsonResponse(w, cardJSON)
				return
			}
			jsonResponse(w, `{"object":"list","has_more":false,"data":[]}`)
		})
		res, _ := s.handleGetRulings(context.Background(), toolRequest(map[string]any{"name": "Doubling Season"}))
		if !strings.Contains(resultText(t, res), "No official rulings") {
			t.Error("expected no-rulings message")
		}
	})

	t.Run("card not found", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusNotFound)
		})
		res, _ := s.handleGetRulings(context.Background(), toolRequest(map[string]any{"name": "nope"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})

	t.Run("rulings fetch fails", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/cards/named") {
				jsonResponse(w, cardJSON)
				return
			}
			scryfallError(w, http.StatusInternalServerError)
		})
		res, _ := s.handleGetRulings(context.Background(), toolRequest(map[string]any{"name": "Doubling Season"}))
		if !res.IsError {
			t.Error("expected error result when rulings fetch fails")
		}
	})
}

func TestHandleGetPrice(t *testing.T) {
	// Stub the exchange rate so the handler never touches the live API.
	prev := exchangeRateFetcher
	exchangeRateFetcher = func(context.Context) (float64, error) { return 5.0, nil }
	t.Cleanup(func() { exchangeRateFetcher = prev })

	t.Run("missing name", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {})
		res, _ := s.handleGetPrice(context.Background(), toolRequest(nil))
		if !res.IsError {
			t.Error("expected error for missing name")
		}
	})

	t.Run("price by name with all currencies", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/cards/named") {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
			jsonResponse(w, solRingJSON)
		})
		res, err := s.handleGetPrice(context.Background(), toolRequest(map[string]any{"name": "Sol Ring"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		for _, want := range []string{
			"Pricing for Sol Ring", "USD:** $1.50", "BRL:** R$ 7.50", "USD (Foil):** $5.00",
			"EUR:** €1.20", "EUR (Foil):** €4.10", "MTGO Tix:** 0.05", "1 USD = 5.0000 BRL",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
	})

	t.Run("price by set uses search", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/cards/search") {
				t.Errorf("expected search path, got %q", r.URL.Path)
			}
			jsonResponse(w, `{"object":"list","total_cards":1,"has_more":false,"data":[`+solRingJSON+`]}`)
		})
		res, err := s.handleGetPrice(context.Background(), toolRequest(map[string]any{
			"name": "Sol Ring",
			"set":  "cmr",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resultText(t, res), "Pricing for Sol Ring") {
			t.Error("expected pricing output for set lookup")
		}
	})

	t.Run("card not found in set", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, `{"object":"list","total_cards":0,"has_more":false,"data":[]}`)
		})
		res, _ := s.handleGetPrice(context.Background(), toolRequest(map[string]any{
			"name": "Sol Ring",
			"set":  "zzz",
		}))
		if !res.IsError {
			t.Error("expected error result for missing card in set")
		}
	})

	t.Run("no pricing data", func(t *testing.T) {
		card := `{"id":"id","name":"No Price","set":"x","set_name":"X","collector_number":"1",` +
			`"legalities":{"commander":"legal"},"prices":{}}`
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, card)
		})
		res, _ := s.handleGetPrice(context.Background(), toolRequest(map[string]any{"name": "No Price"}))
		if !strings.Contains(resultText(t, res), "No pricing data available") {
			t.Error("expected no-pricing message")
		}
	})

	t.Run("card not found by name", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusNotFound)
		})
		res, _ := s.handleGetPrice(context.Background(), toolRequest(map[string]any{"name": "nope"}))
		if !res.IsError {
			t.Error("expected error result")
		}
	})

	t.Run("falls back when rate fetch fails", func(t *testing.T) {
		exchangeRateFetcher = func(context.Context) (float64, error) {
			return 0, io.ErrUnexpectedEOF
		}
		defer func() {
			exchangeRateFetcher = func(context.Context) (float64, error) { return 5.0, nil }
		}()

		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, solRingJSON)
		})
		res, err := s.handleGetPrice(context.Background(), toolRequest(map[string]any{"name": "Sol Ring"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Fallback rate is 5.40 -> 1.50 * 5.40 = 8.10
		if !strings.Contains(resultText(t, res), "BRL:** R$ 8.10") {
			t.Errorf("expected fallback rate conversion, got:\n%s", resultText(t, res))
		}
	})
}

func TestHandleGetBannedList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		body := `{"object":"list","total_cards":2,"has_more":false,"data":[` +
			`{"id":"1","name":"Mana Crypt","legalities":{"commander":"banned"}},` +
			`{"id":"2","name":"Channel","legalities":{"commander":"banned"}}]}`
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, body)
		})
		res, err := s.handleGetBannedList(context.Background(), toolRequest(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		for _, want := range []string{"Banned List", "Total banned cards: 2", "Mana Crypt", "Channel"} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
	})

	t.Run("fetch fails", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusInternalServerError)
		})
		res, _ := s.handleGetBannedList(context.Background(), toolRequest(nil))
		if !res.IsError {
			t.Error("expected error result")
		}
	})
}

func TestHandleValidateDeck(t *testing.T) {
	commanderJSON := `{"id":"c","name":"Atraxa, Praetors' Voice","type_line":"Legendary Creature — Phyrexian Angel Horror",` +
		`"oracle_text":"Flying, vigilance","color_identity":["W","U","B","G"],"legalities":{"commander":"legal"}}`

	t.Run("missing commander", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {})
		res, _ := s.handleValidateDeck(context.Background(), toolRequest(map[string]any{"decklist": "1 Sol Ring"}))
		if !res.IsError {
			t.Error("expected error for missing commander")
		}
	})

	t.Run("missing decklist", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {})
		res, _ := s.handleValidateDeck(context.Background(), toolRequest(map[string]any{"commander": "Atraxa"}))
		if !res.IsError {
			t.Error("expected error for missing decklist")
		}
	})

	t.Run("commander not found", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusNotFound)
		})
		res, _ := s.handleValidateDeck(context.Background(), toolRequest(map[string]any{
			"commander": "nope",
			"decklist":  "1 Sol Ring",
		}))
		if !res.IsError {
			t.Error("expected error result for missing commander")
		}
	})

	t.Run("valid 99-card deck with duplicates flagged", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, commanderJSON)
		})
		// Build a decklist: 97 unique + a duplicated non-basic = 99 entries total.
		var sb strings.Builder
		sb.WriteString("2 Sol Ring\n") // duplicate non-basic -> flagged
		sb.WriteString("10 Forest\n")  // basic land duplicates -> allowed
		for i := 0; i < 87; i++ {      // pad to 99 total entries
			sb.WriteString("1 Card")
			sb.WriteByte(byte('A' + (i % 26)))
			sb.WriteString("\n")
		}
		res, err := s.handleValidateDeck(context.Background(), toolRequest(map[string]any{
			"commander": "Atraxa, Praetors' Voice",
			"decklist":  sb.String(),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		for _, want := range []string{"Atraxa", "Color Identity:** W, U, B, G", "Found duplicates", "sol ring (x2)"} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
		if strings.Contains(text, "forest (x") {
			t.Error("basic lands should not be flagged as duplicates")
		}
	})

	t.Run("wrong deck size and no duplicates", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, commanderJSON)
		})
		res, _ := s.handleValidateDeck(context.Background(), toolRequest(map[string]any{
			"commander": "Atraxa, Praetors' Voice",
			"decklist":  "1 Sol Ring\n1 Arcane Signet",
		}))
		text := resultText(t, res)
		if !strings.Contains(text, "No duplicates") {
			t.Errorf("expected no-duplicates message, got:\n%s", text)
		}
		if !strings.Contains(text, "should be 99 cards") {
			t.Errorf("expected deck-size warning, got:\n%s", text)
		}
	})

	t.Run("banned non-legendary commander", func(t *testing.T) {
		badCommander := `{"id":"c","name":"Black Lotus","type_line":"Artifact","oracle_text":"",` +
			`"color_identity":[],"legalities":{"commander":"banned"}}`
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, badCommander)
		})
		res, _ := s.handleValidateDeck(context.Background(), toolRequest(map[string]any{
			"commander": "Black Lotus",
			"decklist":  "1 Sol Ring",
		}))
		text := resultText(t, res)
		if !strings.Contains(text, "banned in Commander") {
			t.Errorf("expected banned warning, got:\n%s", text)
		}
		if !strings.Contains(text, "cannot be a commander") {
			t.Errorf("expected non-commander warning, got:\n%s", text)
		}
	})
}

func TestGetExchangeRate(t *testing.T) {
	t.Run("success decodes BRL", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, `{"amount":1,"base":"USD","rates":{"BRL":5.25}}`)
		}))
		defer ts.Close()

		get := func(ctx context.Context, rawURL string) (*http.Response, error) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
			return http.DefaultClient.Do(req)
		}
		rate, err := getExchangeRate(context.Background(), ts.URL, get)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate != 5.25 {
			t.Errorf("rate = %v, want 5.25", rate)
		}
	})

	t.Run("getter error propagates", func(t *testing.T) {
		get := func(context.Context, string) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		}
		if _, err := getExchangeRate(context.Background(), "x", get); err == nil {
			t.Error("expected error from getter")
		}
	})

	t.Run("decode error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, `not json`)
		}))
		defer ts.Close()
		get := func(ctx context.Context, rawURL string) (*http.Response, error) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
			return http.DefaultClient.Do(req)
		}
		if _, err := getExchangeRate(context.Background(), ts.URL, get); err == nil {
			t.Error("expected decode error")
		}
	})
}
