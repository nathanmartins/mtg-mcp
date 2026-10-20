package mtgserver

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleCommanderRules(
	_ context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	// Summarises Comprehensive Rules 903 (Commander) and 103.5 (mulligans). Counts and dates
	// that go stale, such as the size of the banned list, belong to the live tools instead.
	rules := `# Commander Format Rules

## Overview
Commander is a multiplayer format for Magic: The Gathering, emphasizing social interactions, interesting games, and creative deck-building.

## Deck Construction
- **100 cards total**: exactly 100 cards, including the commander (rule 903.5a)
- **Singleton**: every card except basic lands must have a different English name (903.5b)
- **Commander**: a legendary creature, Vehicle, or Spacecraft with power/toughness, or a card whose text says it can be your commander (903.3)
- **Color Identity**: every card's color identity must be within the commander's color identity (903.5c)
- **Banned List**: use the get_banned_list tool or the commander://banned-list resource for the current list

## Gameplay Rules
- **Starting Life**: 40 life points (903.7)
- **Command Zone**: commanders start the game face up in the command zone (903.6)
- **Commander Tax**: casting a commander from the command zone costs {2} more for each previous time it was cast from there (903.8)
- **Commander Damage**: a player dealt 21 or more combat damage by the same commander loses the game (903.10a)
- **Mulligan**: London mulligan; in multiplayer games the first mulligan is free (103.5, 103.5c)

## Winning Conditions
- Reduce all opponents to 0 life
- 21 or more combat damage from a single commander
- Opponents drawing from an empty library
- Alternate win conditions (as printed on cards)

## Official Rules
- Commander is defined in section 903 of the Comprehensive Rules.
- Read the current text with the rules://comprehensive resource or the get_rule tool (e.g. rule "903").
`
	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: mimeTypeTextPlain,
			Text:     rules,
		},
	}, nil
}

func (s *Server) handleBannedListResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	result, err := s.scryfallClient.SearchCards(ctx, "banned:commander", scryfall.SearchCardsOptions{
		Order: cardSortOrderName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch banned list: %w", err)
	}

	bannedCards := make([]map[string]string, len(result.Cards))
	for i, card := range result.Cards {
		bannedCards[i] = map[string]string{
			paramName:   card.Name,
			"type":      card.TypeLine,
			"mana_cost": card.ManaCost,
		}
	}

	// Deterministic sorts the map keys, matching the v1 MarshalIndent output this replaced.
	data, err := json.Marshal(map[string]any{
		"format":       defaultFormat,
		"total_banned": result.TotalCards,
		"cards":        bannedCards,
		"last_updated": "real-time",
	}, json.Deterministic(true), jsontext.WithIndent("  "))
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
