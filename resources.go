package main

import (
	"context"
	"encoding/json"
	"fmt"

	scryfall "github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MTGCommanderServer) handleCommanderRules(
	_ context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	rules := `# Commander Format Rules

## Overview
Commander is a multiplayer format for Magic: The Gathering, emphasizing social interactions, interesting games, and creative deck-building.

## Deck Construction
- **100 cards total**: 1 commander + 99 other cards
- **Singleton**: No more than 1 copy of any card (except basic lands)
- **Commander**: Must be a legendary creature or eligible legendary permanent
- **Color Identity**: All cards must match commander's color identity (mana symbols in cost and rules text)
- **Banned List**: 85 cards currently banned (use get_banned_list tool for current list)

## Gameplay Rules
- **Starting Life**: 40 life points (instead of 20)
- **Command Zone**: Commanders start in the Command Zone
- **Commander Tax**: Pay 2 additional generic mana for each previous casting from command zone
- **Commander Damage**: 21 combat damage from a single commander causes player loss
- **Mulligan**: Partial Paris mulligan (draw 7, then any number of mulligans drawing 1 less each time)

## Winning Conditions
- Reduce all opponents to 0 life
- 21 combat damage from a single commander
- Opponents deck out (draw from empty library)
- Alternate win conditions (as printed on cards)

## Official Resources
- Format managed by Wizards of the Coast (as of September 2024)
- Official website: https://mtgcommander.net
- Rules updates: Follow WeeklyMTG on Twitch/YouTube

*Last updated: November 2025*
`
	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     rules,
		},
	}, nil
}

func (s *MTGCommanderServer) handleBannedListResource(
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

	data, err := json.MarshalIndent(map[string]interface{}{
		"format":       defaultFormat,
		"total_banned": result.TotalCards,
		"cards":        bannedCards,
		"last_updated": "real-time",
	}, "", "  ")
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
