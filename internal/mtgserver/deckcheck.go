package mtgserver

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/BlueMonday/go-scryfall"
)

// collectionBatchSize is the most identifiers Scryfall's /cards/collection accepts per request.
const collectionBatchSize = 75

// unlimitedCopies marks a card whose own text lets a deck run any number of copies.
const unlimitedCopies = -1

// copiesRulePattern matches the deck-construction text on cards that override the singleton
// rule: "A deck can have any number of cards named X" (Relentless Rats) or "A deck can have up
// to seven cards named X" (Seven Dwarves).
var copiesRulePattern = regexp.MustCompile(`(?i)a deck can have (any number of|up to (\w+)) cards named`)

// basicLandNames lets the singleton check exempt basics by name when card data could not be
// fetched; with card data, the "Basic" supertype decides instead.
func basicLandNames() []string {
	return []string{
		"plains", "island", "swamp", "mountain", "forest", "wastes",
		"snow-covered plains", "snow-covered island", "snow-covered swamp",
		"snow-covered mountain", "snow-covered forest", "snow-covered wastes",
	}
}

// numberWords lists spelled-out counts by value, for "up to N cards named" texts.
func numberWords() []string {
	return []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
}

// lookupCards resolves card names with batched /cards/collection requests (75 names each, so a
// 99-card deck costs at most two). Cards are keyed by lowercase full name and by each face name,
// because decklists name double-faced cards by their front face. Scryfall does not guarantee
// response order, so results are never matched up by position, and a name counts as unknown
// whenever no returned card answers to it, whether or not Scryfall listed it as not found.
func (s *Server) lookupCards(ctx context.Context, names []string) (map[string]scryfall.Card, []string, error) {
	seen := make(map[string]bool, len(names))
	var unique []string
	for _, name := range names {
		key := cardKey(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, strings.TrimSpace(name))
	}

	found := make(map[string]scryfall.Card, len(unique))
	for batch := range slices.Chunk(unique, collectionBatchSize) {
		identifiers := make([]scryfall.CardIdentifier, len(batch))
		for i, name := range batch {
			identifiers[i] = scryfall.CardIdentifier{Name: name}
		}
		resp, err := s.scryfallClient.GetCardsByIdentifiers(ctx, identifiers)
		if err != nil {
			return nil, nil, fmt.Errorf("look up %d cards on Scryfall: %w", len(identifiers), err)
		}
		for _, card := range resp.Data {
			found[cardKey(card.Name)] = card
			for _, face := range card.CardFaces {
				found[cardKey(face.Name)] = card
			}
		}
	}

	var notFound []string
	for _, name := range unique {
		if _, ok := found[cardKey(name)]; !ok {
			notFound = append(notFound, name)
		}
	}
	slices.Sort(notFound)
	return found, notFound, nil
}

// cardKey normalizes a card name for case-insensitive matching.
func cardKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// face is the part of a card that deck-construction rules are judged on.
type face struct {
	typeLine   string
	oracleText string
	power      *string
}

// frontFace returns the front face for double-faced cards, otherwise the card itself.
func frontFace(card scryfall.Card) face {
	if len(card.CardFaces) == 0 {
		return face{typeLine: card.TypeLine, oracleText: card.OracleText, power: card.Power}
	}
	front := face{typeLine: card.CardFaces[0].TypeLine, power: card.CardFaces[0].Power}
	if card.CardFaces[0].OracleText != nil {
		front.oracleText = *card.CardFaces[0].OracleText
	}
	return front
}

// commanderEligible reports whether card can be a commander under rule 903.3: a legendary
// creature, Vehicle, or Spacecraft with power/toughness, or a card whose text says it can be
// your commander. Being legendary alone is not enough (a Legendary Sorcery cannot lead a deck).
func commanderEligible(card scryfall.Card) bool {
	front := frontFace(card)
	if strings.Contains(strings.ToLower(front.oracleText), "can be your commander") {
		return true
	}
	typeLine := strings.ToLower(front.typeLine)
	if !strings.Contains(typeLine, "legendary") {
		return false
	}
	switch {
	case strings.Contains(typeLine, "creature"), strings.Contains(typeLine, "vehicle"):
		return true
	case strings.Contains(typeLine, "spacecraft"):
		return front.power != nil
	default:
		return false
	}
}

// isBasicLand reports whether card has the Basic supertype, which covers Wastes and the
// Snow-Covered basics as well as the five classic basic lands.
func isBasicLand(card scryfall.Card) bool {
	return strings.Contains(strings.ToLower(frontFace(card).typeLine), "basic")
}

// allowedCopies returns how many copies of card a Commander deck may run: 1 normally, any number
// for basic lands and cards that say so, or the "up to N" a card's own text sets.
func allowedCopies(card scryfall.Card) int {
	if isBasicLand(card) {
		return unlimitedCopies
	}
	match := copiesRulePattern.FindStringSubmatch(frontFace(card).oracleText)
	switch {
	case match == nil:
		return 1
	case strings.EqualFold(match[1], "any number of"):
		return unlimitedCopies
	default:
		if n := slices.Index(numberWords(), strings.ToLower(match[2])); n > 0 {
			return n
		}
		return 1
	}
}

// colorIdentityWithin reports whether every color in card's identity is in allowed.
func colorIdentityWithin(card scryfall.Card, allowed []scryfall.Color) bool {
	for _, color := range card.ColorIdentity {
		if !slices.Contains(allowed, color) {
			return false
		}
	}
	return true
}

// formatColorIdentity renders an identity as "W, U, B" or "Colorless".
func formatColorIdentity(identity []scryfall.Color) string {
	if len(identity) == 0 {
		return "Colorless"
	}
	letters := make([]string, len(identity))
	for i, color := range identity {
		letters[i] = string(color)
	}
	return strings.Join(letters, ", ")
}
