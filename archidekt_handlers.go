package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MTGCommanderServer) handleGetArchidektDeck(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	deckIDStr, err := request.RequireString("deck_id")
	if err != nil {
		GetLogger().Error().Err(err).Str("tool", "get_archidekt_deck").Msg("Missing deck_id parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	deckID, err := ExtractArchidektDeckID(deckIDStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid deck ID: %v", err)), nil
	}

	GetLogger().Info().
		Str("tool", "get_archidekt_deck").
		Int("deck_id", deckID).
		Msg("Fetching Archidekt deck")

	deck, err := GetArchidektDeck(ctx, deckID)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "get_archidekt_deck").
			Int("deck_id", deckID).
			Msg("Failed to fetch Archidekt deck")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch Archidekt deck: %v", err)), nil
	}

	landsOnly := false
	args := request.GetArguments()
	if v, hasLandsOnly := args["lands_only"]; hasLandsOnly {
		if b, isBool := v.(bool); isBool {
			landsOnly = b
		}
	}

	GetLogger().Info().
		Str("tool", "get_archidekt_deck").
		Int("deck_id", deckID).
		Str("deck_name", deck.Name).
		Int("card_count", len(deck.Cards)).
		Bool("lands_only", landsOnly).
		Msg("Successfully fetched Archidekt deck")

	var output string
	if landsOnly {
		output = FormatArchidektLandsForDisplay(deck)
	} else {
		output = FormatArchidektDeckForDisplay(deck)
	}
	return mcp.NewToolResultText(output), nil
}

func (s *MTGCommanderServer) handleGetArchidektUserDecks(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	username, err := request.RequireString("username")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	page := 1
	args := request.GetArguments()
	if pageVal, hasPage := args["page"]; hasPage {
		if pageFloat, ok := pageVal.(float64); ok && pageFloat >= 1 {
			page = int(pageFloat)
		}
	}

	GetLogger().Info().
		Str("tool", "get_archidekt_user_decks").
		Str("username", username).
		Int("page", page).
		Msg("Fetching Archidekt user decks")

	result, err := GetArchidektUserDecks(ctx, username, page)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "get_archidekt_user_decks").
			Str("username", username).
			Msg("Failed to fetch Archidekt user decks")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch Archidekt user decks: %v", err)), nil
	}

	var output strings.Builder
	fmt.Fprintf(&output, "# Archidekt Decks by %s\n\n", username)
	fmt.Fprintf(&output, "**Total Decks:** %d\n", result.Count)
	fmt.Fprintf(&output, "**Page:** %d\n\n", page)

	if len(result.Results) == 0 {
		output.WriteString("No public decks found for this user.\n")
	} else {
		for i, deck := range result.Results {
			fmt.Fprintf(&output, "## %d. %s\n", i+1, deck.Name)
			fmt.Fprintf(&output, "- **Format:** %s\n", archidektFormatName(deck.DeckFormat))
			fmt.Fprintf(&output, "- **Views:** %d\n", deck.ViewCount)
			fmt.Fprintf(&output, "- **Last Updated:** %s\n", deck.UpdatedAt)
			fmt.Fprintf(&output, "- **URL:** https://archidekt.com/decks/%d\n\n", deck.ID)
		}
	}

	if result.Next != "" {
		fmt.Fprintf(&output, "*More decks available — use page %d to continue.*\n", page+1)
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (s *MTGCommanderServer) handleSearchArchidektDecks(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	commander, err := request.RequireString("commander")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bracket := 0
	limit := 10
	args := request.GetArguments()
	if bracketVal, hasBracket := args["bracket"]; hasBracket {
		if bracketFloat, ok := bracketVal.(float64); ok {
			bracket = int(bracketFloat)
		}
	}
	if limitVal, hasLimit := args["limit"]; hasLimit {
		if limitFloat, ok := limitVal.(float64); ok && limitFloat >= 1 {
			limit = int(limitFloat)
		}
	}

	GetLogger().Info().
		Str("tool", "search_archidekt_decks").
		Str("commander", commander).
		Int("bracket", bracket).
		Int("limit", limit).
		Msg("Searching Archidekt decks")

	result, err := SearchArchidektDecks(ctx, commander, bracket, limit)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "search_archidekt_decks").
			Str("commander", commander).
			Msg("Failed to search Archidekt decks")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search Archidekt decks: %v", err)), nil
	}

	GetLogger().Info().
		Str("tool", "search_archidekt_decks").
		Str("commander", commander).
		Int("result_count", len(result.Results)).
		Msg("Successfully searched Archidekt decks")

	output := FormatArchidektSearchResultsForDisplay(commander, bracket, result)
	return mcp.NewToolResultText(output), nil
}
