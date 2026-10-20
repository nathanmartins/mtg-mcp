package mtgserver

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BlueMonday/go-scryfall"
)

// fixtureCard renders a minimal Scryfall card object.
func fixtureCard(name, typeLine, oracle string, identity []string, legality string) map[string]any {
	return map[string]any{
		"object": "card", "id": strings.ReplaceAll(strings.ToLower(name), " ", "-"),
		"name": name, "type_line": typeLine, "oracle_text": oracle,
		"color_identity": identity, "legalities": map[string]string{"commander": legality},
	}
}

// deckCheckFixture emulates the two Scryfall endpoints validate_deck uses: /cards/named for the
// commander and /cards/collection for the decklist. Cards are looked up by lowercase name.
type deckCheckFixture struct {
	commander        map[string]any
	cards            map[string]map[string]any
	collectionStatus int
	collectionCalls  atomic.Int32
}

func (f *deckCheckFixture) server(t *testing.T) *Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cards/named"):
			if f.commander == nil {
				scryfallError(w, http.StatusNotFound)
				return
			}
			writeJSON(t, w, f.commander)
		case strings.HasSuffix(r.URL.Path, "/cards/collection"):
			f.collectionCalls.Add(1)
			if f.collectionStatus != 0 {
				scryfallError(w, f.collectionStatus)
				return
			}
			var req struct {
				Identifiers []struct {
					Name string `json:"name"`
				} `json:"identifiers"`
			}
			if err := json.UnmarshalRead(r.Body, &req); err != nil {
				t.Errorf("bad collection request: %v", err)
			}
			if len(req.Identifiers) > collectionBatchSize {
				t.Errorf("collection request has %d identifiers, Scryfall accepts %d",
					len(req.Identifiers), collectionBatchSize)
			}
			data, notFound := []map[string]any{}, []map[string]string{}
			for _, id := range req.Identifiers {
				if card, ok := f.cards[strings.ToLower(id.Name)]; ok {
					data = append(data, card)
				} else {
					notFound = append(notFound, map[string]string{"name": id.Name})
				}
			}
			writeJSON(t, w, map[string]any{"object": "list", "data": data, "not_found": notFound})
		default:
			t.Errorf("unexpected Scryfall request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(ts.Close)

	client, err := scryfall.NewClient(scryfall.WithBaseURL(ts.URL), scryfall.WithLimiter(nil))
	if err != nil {
		t.Fatalf("failed to create scryfall test client: %v", err)
	}
	return &Server{scryfallClient: client}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, v); err != nil {
		t.Errorf("writing fixture: %v", err)
	}
}

func atraxa() map[string]any {
	return fixtureCard("Atraxa, Praetors' Voice", "Legendary Creature — Phyrexian Angel Horror",
		"Flying, vigilance", []string{"W", "U", "B", "G"}, "legal")
}

func validateDeck(t *testing.T, s *Server, commander, decklist string) string {
	t.Helper()
	res, err := s.handleValidateDeck(t.Context(), toolRequest(map[string]any{
		"commander": commander, "decklist": decklist,
	}))
	if err != nil {
		t.Fatalf("handleValidateDeck() error = %v", err)
	}
	return resultText(t, res)
}

func TestHandleValidateDeckArguments(t *testing.T) {
	fixture := &deckCheckFixture{commander: atraxa()}
	s := fixture.server(t)

	for name, args := range map[string]map[string]any{
		"missing commander": {"decklist": "1 Sol Ring"},
		"missing decklist":  {"commander": "Atraxa"},
	} {
		res, _ := s.handleValidateDeck(t.Context(), toolRequest(args))
		if !res.IsError {
			t.Errorf("%s: expected an error result", name)
		}
	}

	missing := (&deckCheckFixture{}).server(t)
	res, _ := missing.handleValidateDeck(t.Context(), toolRequest(map[string]any{
		"commander": "nope", "decklist": "1 Sol Ring",
	}))
	if !res.IsError {
		t.Error("expected an error result when the commander is not found")
	}
}

func TestHandleValidateDeckBatchesLookupsAndPassesALegalDeck(t *testing.T) {
	fixture := &deckCheckFixture{commander: atraxa(), cards: map[string]map[string]any{
		"forest": fixtureCard("Forest", "Basic Land — Forest", "({T}: Add {G}.)", nil, "legal"),
	}}
	var deck strings.Builder
	deck.WriteString("19 Forest\n")
	for i := range 80 {
		name := fmt.Sprintf("Test Card %d", i)
		fixture.cards[strings.ToLower(name)] = fixtureCard(name, "Creature — Elf", "", []string{"G"}, "legal")
		_, _ = fmt.Fprintf(&deck, "1 %s\n", name)
	}

	text := validateDeck(t, fixture.server(t), "Atraxa, Praetors' Voice", deck.String())

	// 81 distinct names: one batch of 75 and one of 6.
	if got := fixture.collectionCalls.Load(); got != 2 {
		t.Errorf("collection requests = %d, want 2", got)
	}
	for _, want := range []string{"**Deck Size:** 99 cards ✅", "No duplicates", "within the commander's color identity",
		"No banned or illegal cards", "Every card was found", "The deck passes every check"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\n%s", want, text)
		}
	}
}

func TestHandleValidateDeckReportsEveryProblem(t *testing.T) {
	fixture := &deckCheckFixture{commander: atraxa(), cards: map[string]map[string]any{
		"sol ring":            fixtureCard("Sol Ring", "Artifact", "{T}: Add {C}{C}.", nil, "legal"),
		"lightning bolt":      fixtureCard("Lightning Bolt", "Instant", "Deal 3 damage.", []string{"R"}, "legal"),
		"mana crypt":          fixtureCard("Mana Crypt", "Artifact", "", nil, "banned"),
		"ach! hans, run!":     fixtureCard("Ach! Hans, Run!", "Enchantment", "", []string{"R", "G"}, "not_legal"),
		"snow-covered forest": fixtureCard("Snow-Covered Forest", "Basic Snow Land — Forest", "", nil, "legal"),
		"relentless rats": fixtureCard("Relentless Rats", "Creature — Rat",
			"A deck can have any number of cards named Relentless Rats.", []string{"B"}, "legal"),
		"seven dwarves": fixtureCard("Seven Dwarves", "Creature — Dwarf",
			"A deck can have up to seven cards named Seven Dwarves.", []string{"R"}, "legal"),
	}}
	decklist := "2 Sol Ring\n1 Lightning Bolt\n1 Mana Crypt\n1 Ach! Hans, Run!\n1 Not A Real Card\n" +
		"3 Snow-Covered Forest\n12 Relentless Rats\n8 Seven Dwarves\n"

	text := validateDeck(t, fixture.server(t), "Atraxa, Praetors' Voice", decklist)

	for _, want := range []string{
		"Sol Ring (x2)", "Seven Dwarves (x8)", // over the singleton / "up to seven" limits
		"Lightning Bolt (R)", "Seven Dwarves (R)", // outside W/U/B/G
		"Mana Crypt (banned)", "Ach! Hans, Run! (not legal in Commander)",
		"Not A Real Card",
		"should be 99 cards",
		"**Result:** ❌",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Snow-Covered Forest (x", "Relentless Rats (x"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("output wrongly flags %q\n%s", unwanted, text)
		}
	}
	// Lists are sorted, so the output is stable between calls.
	if strings.Index(text, "Lightning Bolt (R)") > strings.Index(text, "Seven Dwarves (R)") {
		t.Errorf("color identity list is not sorted\n%s", text)
	}
}

func TestHandleValidateDeckMatchesDoubleFacedCardsByFrontFace(t *testing.T) {
	delver := fixtureCard("Delver of Secrets // Insectile Aberration", "Creature — Human Wizard // Creature — Insect",
		"", []string{"U"}, "legal")
	delver["card_faces"] = []map[string]any{
		{"name": "Delver of Secrets", "type_line": "Creature — Human Wizard"},
		{"name": "Insectile Aberration", "type_line": "Creature — Human Insect"},
	}
	fixture := &deckCheckFixture{commander: atraxa(), cards: map[string]map[string]any{"delver of secrets": delver}}

	text := validateDeck(t, fixture.server(t), "Atraxa, Praetors' Voice", "1 Delver of Secrets")

	if !strings.Contains(text, "Every card was found") {
		t.Errorf("a double-faced card named by its front face should be found\n%s", text)
	}
}

func TestHandleValidateDeckRejectsIneligibleCommander(t *testing.T) {
	fixture := &deckCheckFixture{commander: fixtureCard("Urza's Ruinous Blast", "Legendary Sorcery",
		"(You may cast a legendary sorcery only if you control a legendary creature or planeswalker.)",
		[]string{"W"}, "legal")}

	text := validateDeck(t, fixture.server(t), "Urza's Ruinous Blast", "1 Sol Ring")

	if !strings.Contains(text, "cannot be a commander") {
		t.Errorf("a Legendary Sorcery must not pass as a commander\n%s", text)
	}
}

func TestHandleValidateDeckStillChecksSingletonWhenLookupFails(t *testing.T) {
	fixture := &deckCheckFixture{commander: atraxa(), collectionStatus: http.StatusInternalServerError}

	text := validateDeck(t, fixture.server(t), "Atraxa, Praetors' Voice", "2 Sol Ring\n4 Snow-Covered Forest")

	for _, want := range []string{"Could not check color identity or card legality", "Sol Ring (x2)"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Snow-Covered Forest (x") {
		t.Errorf("snow-covered basics are exempt even without card data\n%s", text)
	}
}

func TestCommanderEligible(t *testing.T) {
	power := "4"
	tests := []struct {
		name string
		card scryfall.Card
		want bool
	}{
		{"legendary creature", scryfall.Card{TypeLine: "Legendary Creature — Elf"}, true},
		{"legendary vehicle", scryfall.Card{TypeLine: "Legendary Artifact — Vehicle"}, true},
		{"spacecraft with power", scryfall.Card{TypeLine: "Legendary Artifact — Spacecraft", Power: &power}, true},
		{"spacecraft without power", scryfall.Card{TypeLine: "Legendary Artifact — Spacecraft"}, false},
		{"legendary sorcery", scryfall.Card{TypeLine: "Legendary Sorcery"}, false},
		{"legendary land", scryfall.Card{TypeLine: "Legendary Land"}, false},
		{"nonlegendary creature", scryfall.Card{TypeLine: "Creature — Elf"}, false},
		{"planeswalker that says so", scryfall.Card{
			TypeLine: "Legendary Planeswalker — Teferi", OracleText: "Teferi can be your commander.",
		}, true},
		{
			"double-faced, judged by front face",
			scryfall.Card{
				TypeLine: "Legendary Creature — Human // Legendary Planeswalker — Nissa",
				CardFaces: []scryfall.CardFace{
					{TypeLine: "Legendary Creature — Elf"},
					{TypeLine: "Legendary Planeswalker"},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		if got := commanderEligible(tt.card); got != tt.want {
			t.Errorf("%s: commanderEligible() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAllowedCopies(t *testing.T) {
	tests := []struct {
		name string
		card scryfall.Card
		want int
	}{
		{"ordinary card", scryfall.Card{TypeLine: "Artifact"}, 1},
		{"basic land", scryfall.Card{TypeLine: "Basic Land — Forest"}, unlimitedCopies},
		{"snow basic", scryfall.Card{TypeLine: "Basic Snow Land — Island"}, unlimitedCopies},
		{"any number", scryfall.Card{OracleText: "A deck can have any number of cards named Relentless Rats."},
			unlimitedCopies},
		{"up to seven", scryfall.Card{OracleText: "A deck can have up to seven cards named Seven Dwarves."}, 7},
		{"up to nine", scryfall.Card{OracleText: "A deck can have up to nine cards named Nazgûl."}, 9},
	}
	for _, tt := range tests {
		if got := allowedCopies(tt.card); got != tt.want {
			t.Errorf("%s: allowedCopies() = %d, want %d", tt.name, got, tt.want)
		}
	}
}
