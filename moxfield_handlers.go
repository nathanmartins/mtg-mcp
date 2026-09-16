package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MTGCommanderServer) handleGetMoxfieldDeck(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	deckID, err := request.RequireString("deck_id")
	if err != nil {
		GetLogger().Error().Err(err).Str("tool", "get_moxfield_deck").Msg("Missing deck_id parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	publicID := ExtractPublicIDFromURL(deckID)

	GetLogger().Info().
		Str("tool", "get_moxfield_deck").
		Str("deck_id", publicID).
		Msg("Fetching Moxfield deck")

	deck, err := getMoxfieldDeckWithURL(ctx, publicID, s.moxfieldBaseURL)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "get_moxfield_deck").
			Str("deck_id", publicID).
			Msg("Failed to fetch Moxfield deck")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch Moxfield deck: %v", err)), nil
	}

	GetLogger().Info().
		Str("tool", "get_moxfield_deck").
		Str("deck_id", publicID).
		Str("deck_name", deck.Name).
		Str("format", deck.Format).
		Int("mainboard_cards", len(deck.Mainboard)).
		Msg("Successfully fetched Moxfield deck")

	output := FormatDeckForDisplay(deck)
	return mcp.NewToolResultText(output), nil
}

func (s *MTGCommanderServer) handleGetMoxfieldUserDecks(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	username, err := request.RequireString("username")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	const defaultPageSize = 20
	pageSize := defaultPageSize
	args := request.GetArguments()
	if pageSizeVal, hasPageSize := args["page_size"]; hasPageSize {
		if pageSizeFloat, ok := pageSizeVal.(float64); ok {
			pageSize = int(pageSizeFloat)
			if pageSize > maxPageSize {
				pageSize = maxPageSize
			}
		}
	}

	decks, err := getUserDecksWithURL(ctx, username, pageSize, s.moxfieldBaseURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch user decks: %v", err)), nil
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# Decks by %s\n\n", username)
	_, _ = fmt.Fprintf(&output, "**Total Decks:** %d\n", decks.TotalResults)
	_, _ = fmt.Fprintf(&output, "**Showing:** %d decks (Page %d of %d)\n\n",
		len(decks.Data), decks.PageNumber, decks.TotalPages)

	for i, deck := range decks.Data {
		_, _ = fmt.Fprintf(&output, "%d. **%s** (%s)\n", i+1, deck.Name, deck.Format)
		_, _ = fmt.Fprintf(&output, "   - Deck ID: %s\n", deck.PublicID)
		_, _ = fmt.Fprintf(&output, "   - Views: %d | Likes: %d\n", deck.ViewCount, deck.LikeCount)
		_, _ = fmt.Fprintf(&output, "   - URL: %s\n\n", deck.PublicURL)
	}

	return mcp.NewToolResultText(output.String()), nil
}

// moxfieldSearchParamsFromRequest validates MCP arguments into Moxfield search parameters.
func moxfieldSearchParamsFromRequest(commander string, args map[string]any) (MoxfieldSearchParams, error) {
	params := MoxfieldSearchParams{
		CardName:   commander,
		Format:     stringArg(args, "format", defaultFormat),
		PageNumber: intArg(args, "page", 1),
		PageSize:   intArg(args, "limit", moxfieldSearchDefaultLimit),
	}

	sortType, err := moxfieldSortType(stringArg(args, "sort", "updated"))
	if err != nil {
		return params, err
	}
	params.SortType = sortType

	direction, err := moxfieldSortDirection(stringArg(args, "sort_direction", sortDirectionDesc))
	if err != nil {
		return params, err
	}
	params.SortDirection = direction

	if params.PageNumber < 1 {
		return params, fmt.Errorf("invalid page %d (must be 1 or greater)", params.PageNumber)
	}
	if params.PageSize < 1 || params.PageSize > maxPageSize {
		return params, fmt.Errorf("invalid limit %d (accepted: 1-%d)", params.PageSize, maxPageSize)
	}

	return params, nil
}

func (s *MTGCommanderServer) handleSearchMoxfieldDecks(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	commander, err := request.RequireString(paramCommander)
	if err != nil {
		GetLogger().Error().Err(err).Str("tool", "search_moxfield_decks").Msg("Missing commander parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := request.GetArguments()
	params, err := moxfieldSearchParamsFromRequest(commander, args)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "search_moxfield_decks").
			Str(paramCommander, commander).
			Str("sort", stringArg(args, "sort", "updated")).
			Str("sort_direction", stringArg(args, "sort_direction", sortDirectionDesc)).
			Int("page", params.PageNumber).
			Int("limit", params.PageSize).
			Msg("Rejected Moxfield search arguments")
		return mcp.NewToolResultError(err.Error()), nil
	}

	GetLogger().Info().
		Str("tool", "search_moxfield_decks").
		Str(paramCommander, commander).
		Str("format", params.Format).
		Str("sort_type", params.SortType).
		Int("page", params.PageNumber).
		Int("limit", params.PageSize).
		Msg("Searching Moxfield decks")

	results, err := searchMoxfieldDecksWithURL(ctx, params, s.moxfieldSearchURL)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "search_moxfield_decks").
			Str(paramCommander, commander).
			Msg("Failed to search Moxfield decks")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search Moxfield decks: %v", err)), nil
	}

	GetLogger().Info().
		Str("tool", "search_moxfield_decks").
		Str(paramCommander, commander).
		Int("results_count", len(results.Data)).
		Int("page", params.PageNumber).
		Int("limit", params.PageSize).
		Msg("Successfully searched Moxfield decks")

	return mcp.NewToolResultText(formatMoxfieldSearchResults(commander, params, results)), nil
}

// formatMoxfieldSearchResults renders a page of Moxfield search results.
func formatMoxfieldSearchResults(
	commander string,
	params MoxfieldSearchParams,
	results *MoxfieldSearchResponse,
) string {
	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# Moxfield Decks for %s\n\n", commander)
	_, _ = fmt.Fprintf(&output, "**Format:** %s\n", params.Format)
	_, _ = fmt.Fprintf(&output, "**Sort:** %s (%s)\n", params.SortType, params.SortDirection)
	_, _ = fmt.Fprintf(&output, "**Total Results:** %d\n", results.TotalResults)
	_, _ = fmt.Fprintf(&output, "**Showing:** %d decks (Page %d of %d)\n\n",
		len(results.Data), results.PageNumber, results.TotalPages)

	if len(results.Data) == 0 {
		output.WriteString("No decks found for this commander.\n")
		return output.String()
	}

	for i, deck := range results.Data {
		_, _ = fmt.Fprintf(&output, "## %d. %s\n", i+1, deck.Name)
		_, _ = fmt.Fprintf(&output, "- **Format:** %s\n", deck.Format)
		_, _ = fmt.Fprintf(&output, "- **Deck ID:** %s\n", deck.PublicID)
		_, _ = fmt.Fprintf(&output, "- **Views:** %d | **Likes:** %d\n", deck.ViewCount, deck.LikeCount)
		_, _ = fmt.Fprintf(&output, "- **URL:** %s\n\n", deck.PublicURL)
	}

	return output.String()
}
