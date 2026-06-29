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

	deck, err := GetMoxfieldDeck(ctx, publicID)
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

	decks, err := GetUserDecks(ctx, username, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch user decks: %v", err)), nil
	}

	var output strings.Builder
	fmt.Fprintf(&output, "# Decks by %s\n\n", username)
	fmt.Fprintf(&output, "**Total Decks:** %d\n", decks.TotalResults)
	fmt.Fprintf(&output, "**Showing:** %d decks (Page %d of %d)\n\n",
		len(decks.Data), decks.PageNumber, decks.TotalPages)

	for i, deck := range decks.Data {
		fmt.Fprintf(&output, "%d. **%s** (%s)\n", i+1, deck.Name, deck.Format)
		fmt.Fprintf(&output, "   - Deck ID: %s\n", deck.PublicID)
		fmt.Fprintf(&output, "   - Views: %d | Likes: %d\n", deck.ViewCount, deck.LikeCount)
		fmt.Fprintf(&output, "   - URL: %s\n\n", deck.PublicURL)
	}

	return mcp.NewToolResultText(output.String()), nil
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

	format := defaultFormat
	if formatVal, hasFormat := args["format"]; hasFormat {
		if formatStr, ok := formatVal.(string); ok {
			format = formatStr
		}
	}

	sortType := "updated"
	if sortTypeVal, hasSortType := args["sort_type"]; hasSortType {
		if sortTypeStr, ok := sortTypeVal.(string); ok {
			sortType = sortTypeStr
		}
	}

	sortDirection := defaultSortDirection
	if sortDirVal, hasSortDir := args["sort_direction"]; hasSortDir {
		if sortDirStr, ok := sortDirVal.(string); ok {
			sortDirection = sortDirStr
		}
	}

	const defaultPageSize = 20
	pageSize := defaultPageSize
	if pageSizeVal, hasPageSize := args["page_size"]; hasPageSize {
		if pageSizeFloat, ok := pageSizeVal.(float64); ok {
			pageSize = int(pageSizeFloat)
			if pageSize > maxPageSize {
				pageSize = maxPageSize
			}
		}
	}

	GetLogger().Info().
		Str("tool", "search_moxfield_decks").
		Str(paramCommander, commander).
		Str("format", format).
		Str("sort_type", sortType).
		Int("page_size", pageSize).
		Msg("Searching Moxfield decks")

	params := MoxfieldSearchParams{
		Query:         commander,
		Format:        format,
		SortType:      sortType,
		SortDirection: sortDirection,
		PageSize:      pageSize,
		PageNumber:    1,
	}

	results, err := SearchMoxfieldDecks(ctx, params)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "search_moxfield_decks").
			Str(paramCommander, commander).
			Msg("Failed to search Moxfield decks")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search Moxfield decks: %v", err)), nil
	}

	var output strings.Builder
	fmt.Fprintf(&output, "# Moxfield Decks for %s\n\n", commander)
	fmt.Fprintf(&output, "**Format:** %s\n", format)
	fmt.Fprintf(&output, "**Total Results:** %d\n", results.TotalResults)
	fmt.Fprintf(&output, "**Showing:** %d decks (Page %d of %d)\n\n",
		len(results.Data), results.PageNumber, results.TotalPages)

	if len(results.Data) == 0 {
		output.WriteString("No decks found for this commander.\n")
	} else {
		for i, deck := range results.Data {
			fmt.Fprintf(&output, "## %d. %s\n", i+1, deck.Name)
			fmt.Fprintf(&output, "- **Format:** %s\n", deck.Format)
			fmt.Fprintf(&output, "- **Deck ID:** %s\n", deck.PublicID)
			fmt.Fprintf(&output, "- **Views:** %d | **Likes:** %d\n", deck.ViewCount, deck.LikeCount)
			fmt.Fprintf(&output, "- **URL:** %s\n\n", deck.PublicURL)
		}
	}

	GetLogger().Info().
		Str("tool", "search_moxfield_decks").
		Str(paramCommander, commander).
		Int("results_count", len(results.Data)).
		Msg("Successfully searched Moxfield decks")

	return mcp.NewToolResultText(output.String()), nil
}
