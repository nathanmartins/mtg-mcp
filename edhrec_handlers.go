package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MTGCommanderServer) handleGetEDHRECRecommendations(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	commander, err := request.RequireString(paramCommander)
	if err != nil {
		GetLogger().Error().Err(err).Str("tool", "get_edhrec_recommendations").Msg("Missing commander parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	limit := 10
	args := request.GetArguments()
	if limitVal, hasLimit := args["limit"]; hasLimit {
		if limitFloat, ok := limitVal.(float64); ok {
			limit = int(limitFloat)
		}
	}

	GetLogger().Info().
		Str("tool", "get_edhrec_recommendations").
		Str(paramCommander, commander).
		Int("limit", limit).
		Msg("Fetching EDHREC recommendations")

	data, err := GetCommanderRecommendations(ctx, commander)
	if err != nil {
		GetLogger().Error().
			Err(err).
			Str("tool", "get_edhrec_recommendations").
			Str(paramCommander, commander).
			Msg("Failed to fetch EDHREC recommendations")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch EDHREC recommendations: %v", err)), nil
	}

	GetLogger().Info().
		Str("tool", "get_edhrec_recommendations").
		Str(paramCommander, commander).
		Int("num_decks", data.NumDecks).
		Int("card_lists", len(data.CardLists)).
		Msg("Successfully fetched EDHREC recommendations")

	output := FormatCommanderRecsForDisplay(data, limit)
	return mcp.NewToolResultText(output), nil
}

func (s *MTGCommanderServer) handleGetEDHRECCombos(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	colors, err := request.RequireString("colors")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	const defaultLimit = 10
	limit := defaultLimit
	args := request.GetArguments()
	if limitVal, hasLimit := args["limit"]; hasLimit {
		if limitFloat, ok := limitVal.(float64); ok {
			limit = int(limitFloat)
		}
	}

	data, err := GetCombosForColors(ctx, colors)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch EDHREC combos: %v", err)), nil
	}

	output := FormatCombosForDisplay(data, limit)
	return mcp.NewToolResultText(output), nil
}
