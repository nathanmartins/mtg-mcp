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

	deck, err := getArchidektDeckWithURL(ctx, deckID, s.archidektBaseURL)
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

	result, err := getArchidektUserDecksWithURL(ctx, username, page, s.archidektBaseURL)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "get_archidekt_user_decks").
			Str("username", username).
			Msg("Failed to fetch Archidekt user decks")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch Archidekt user decks: %v", err)), nil
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# Archidekt Decks by %s\n\n", username)
	_, _ = fmt.Fprintf(&output, "**Total Decks:** %d\n", result.Count)
	_, _ = fmt.Fprintf(&output, "**Page:** %d\n\n", page)

	if len(result.Results) == 0 {
		output.WriteString("No public decks found for this user.\n")
	} else {
		for i, deck := range result.Results {
			_, _ = fmt.Fprintf(&output, "## %d. %s\n", i+1, deck.Name)
			_, _ = fmt.Fprintf(&output, "- **Format:** %s\n", archidektFormatName(deck.DeckFormat))
			_, _ = fmt.Fprintf(&output, "- **Views:** %d\n", deck.ViewCount)
			_, _ = fmt.Fprintf(&output, "- **Last Updated:** %s\n", deck.UpdatedAt)
			_, _ = fmt.Fprintf(&output, "- **URL:** https://archidekt.com/decks/%d\n\n", deck.ID)
		}
	}

	if result.Next != "" {
		_, _ = fmt.Fprintf(&output, "*More decks available — use page %d to continue.*\n", page+1)
	}

	return mcp.NewToolResultText(output.String()), nil
}

// archidektSearchParamsFromRequest validates MCP arguments into search parameters.
// Invalid values are rejected instead of being clamped, because Archidekt accepts
// bad orderBy values silently and would return a wrongly ordered page.
func archidektSearchParamsFromRequest(commander string, args map[string]any) (ArchidektSearchParams, error) {
	params := ArchidektSearchParams{
		Commander: commander,
		Colors:    stringArg(args, "colors", ""),
		Author:    stringArg(args, "author", ""),
		DeckName:  stringArg(args, paramName, ""),
	}

	bracket, err := intArg(args, "bracket", 0)
	if err != nil {
		return params, err
	}
	params.Bracket = bracket

	page, err := intArg(args, "page", 1)
	if err != nil {
		return params, err
	}
	params.Page = page

	limit, err := intArg(args, "limit", archidektSearchDefaultLimit)
	if err != nil {
		return params, err
	}
	params.Limit = limit

	deckSize, err := intArg(args, "deck_size", 0)
	if err != nil {
		return params, err
	}
	params.DeckSize = deckSize

	sortKey, err := enumArg(args, "sort", searchSortDefault)
	if err != nil {
		return params, err
	}
	params.Sort = sortKey

	direction, err := enumArg(args, "sort_direction", sortDirectionDesc)
	if err != nil {
		return params, err
	}
	switch direction {
	case sortDirectionDesc:
		params.Ascending = false
	case sortDirectionAsc:
		params.Ascending = true
	default:
		return params, fmt.Errorf("invalid sort_direction %q (accepted: %s, %s)",
			direction, sortDirectionAsc, sortDirectionDesc)
	}

	if params.Bracket != 0 && (params.Bracket < archidektMinBracket || params.Bracket > archidektMaxBracket) {
		return params, fmt.Errorf("invalid bracket %d (accepted: %d-%d, or omit for all brackets)",
			params.Bracket, archidektMinBracket, archidektMaxBracket)
	}
	if params.Page < 1 || params.Page > archidektSearchMaxPage {
		return params, fmt.Errorf("invalid page %d (accepted: 1-%d)", params.Page, archidektSearchMaxPage)
	}
	if params.Limit < 1 || params.Limit > archidektSearchMaxLimit {
		return params, fmt.Errorf("invalid limit %d (accepted: 1-%d)", params.Limit, archidektSearchMaxLimit)
	}
	if _, err = archidektOrderBy(params.Sort, params.Ascending); err != nil {
		return params, err
	}
	colors, err := normalizeArchidektColors(params.Colors)
	if err != nil {
		return params, err
	}
	params.Colors = colors
	if params.DeckSize < 0 || params.DeckSize > archidektMaxDeckSize {
		return params, fmt.Errorf("invalid deck_size %d (accepted: 1-%d, or omit)",
			params.DeckSize, archidektMaxDeckSize)
	}

	return params, nil
}

func (s *MTGCommanderServer) handleSearchArchidektDecks(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	commander, err := request.RequireString(paramCommander)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	params, err := archidektSearchParamsFromRequest(commander, request.GetArguments())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	GetLogger().Info().
		Str("tool", "search_archidekt_decks").
		Str(paramCommander, params.Commander).
		Int("bracket", params.Bracket).
		Str("sort", params.Sort).
		Bool("ascending", params.Ascending).
		Int("page", params.Page).
		Int("limit", params.Limit).
		Msg("Searching Archidekt decks")

	result, err := searchArchidektDecksWithURL(ctx, params, s.archidektBaseURL)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "search_archidekt_decks").
			Str(paramCommander, params.Commander).
			Msg("Failed to search Archidekt decks")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search Archidekt decks: %v", err)), nil
	}

	GetLogger().Info().
		Str("tool", "search_archidekt_decks").
		Str(paramCommander, params.Commander).
		Int("results_count", len(result.Decks)).
		Msg("Successfully searched Archidekt decks")

	return mcp.NewToolResultText(FormatArchidektSearchResultsForDisplay(params, result)), nil
}
