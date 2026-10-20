package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nathanmartins/mtg-mcp/internal/edhrec"
	"github.com/nathanmartins/mtg-mcp/internal/logging"
)

// optionalStringArg returns the trimmed value of an optional string argument. A missing argument
// yields "", a present non-string argument is an error rather than a silent default.
func optionalStringArg(args map[string]any, key string) (string, error) {
	raw, present := args[key]
	if !present || raw == nil {
		return "", nil
	}

	str, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", key)
	}

	return strings.TrimSpace(str), nil
}

// recFilterFromArgs builds a recommendation filter from raw MCP arguments, rejecting wrong-typed
// values and unknown price tiers instead of silently ignoring them.
func recFilterFromArgs(args map[string]any) (edhrec.RecFilter, error) {
	theme, err := optionalStringArg(args, paramTheme)
	if err != nil {
		return edhrec.RecFilter{}, err
	}
	theme = strings.ToLower(theme)

	tier, err := optionalStringArg(args, paramPriceTier)
	if err != nil {
		return edhrec.RecFilter{}, err
	}
	tier = strings.ToLower(tier)

	if tier != "" && tier != edhrec.PriceTierBudget && tier != edhrec.PriceTierExpensive {
		return edhrec.RecFilter{}, fmt.Errorf(
			"argument %q must be %q or %q, got %q",
			paramPriceTier,
			edhrec.PriceTierBudget,
			edhrec.PriceTierExpensive,
			tier,
		)
	}

	return edhrec.RecFilter{Theme: theme, PriceTier: tier}, nil
}

func (s *MTGCommanderServer) handleGetEDHRECRecommendations(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	commander, err := request.RequireString(paramCommander)
	if err != nil {
		logging.Get().Error().Err(err).Str("tool", "get_edhrec_recommendations").Msg("Missing commander parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	filter, err := recFilterFromArgs(request.GetArguments())
	if err != nil {
		logging.Get().Error().Err(err).Str("tool", "get_edhrec_recommendations").Msg("Invalid filter arguments")
		return mcp.NewToolResultError(err.Error()), nil
	}

	limit := request.GetInt("limit", edhrec.DefaultLimit)
	if limit < 0 {
		return mcp.NewToolResultError("argument \"limit\" must be zero or greater"), nil
	}

	logging.Get().Info().
		Str("tool", "get_edhrec_recommendations").
		Str(paramCommander, commander).
		Str(paramTheme, filter.Theme).
		Str(paramPriceTier, filter.PriceTier).
		Int("limit", limit).
		Msg("Fetching EDHREC recommendations")

	page, err := edhrec.GetCommanderPageWithURL(ctx, commander, filter, s.edhrecBaseURL)
	if err != nil {
		logging.Get().Error().
			Err(err).
			Str("tool", "get_edhrec_recommendations").
			Str(paramCommander, commander).
			Str(paramTheme, filter.Theme).
			Str(paramPriceTier, filter.PriceTier).
			Msg("Failed to fetch EDHREC recommendations")

		if filter.Theme != "" {
			if hint := edhrec.CommanderThemeHint(ctx, commander, filter.Theme, s.edhrecBaseURL); hint != "" {
				return mcp.NewToolResultError(hint), nil
			}
		}

		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch EDHREC recommendations: %v", err)), nil
	}

	logging.Get().Info().
		Str("tool", "get_edhrec_recommendations").
		Str(paramCommander, commander).
		Str(paramTheme, filter.Theme).
		Str(paramPriceTier, filter.PriceTier).
		Int("num_decks", page.Container.JSONDict.Card.NumDecks).
		Int("card_lists", len(page.Container.JSONDict.CardLists)).
		Msg("Successfully fetched EDHREC recommendations")

	output := edhrec.FormatCommanderRecs(page, filter, limit)
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

	limit := request.GetInt("limit", edhrec.DefaultLimit)

	data, err := edhrec.GetCombosForColorsWithURL(ctx, colors, s.edhrecBaseURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch EDHREC combos: %v", err)), nil
	}

	output := edhrec.FormatCombos(data, limit)
	return mcp.NewToolResultText(output), nil
}

func (s *MTGCommanderServer) handleGetEDHRECSetCards(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	setCode, err := request.RequireString(paramSet)
	if err != nil {
		logging.Get().Error().Err(err).Str("tool", "get_edhrec_set_cards").Msg("Missing set parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	code := strings.ToLower(strings.TrimSpace(setCode))
	if code == "" {
		return mcp.NewToolResultError(fmt.Sprintf("argument %q must not be empty", paramSet)), nil
	}

	limit := request.GetInt("limit", edhrec.DefaultSetCardsLimit)
	if limit < 0 {
		return mcp.NewToolResultError("argument \"limit\" must be zero or greater"), nil
	}

	logging.Get().Info().
		Str("tool", "get_edhrec_set_cards").
		Str(paramSet, code).
		Int("limit", limit).
		Msg("Fetching EDHREC set cards")

	page, err := edhrec.GetSetCardsWithURL(ctx, code, s.edhrecBaseURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Failed to fetch EDHREC data for set %q: %v. "+
				"EDHREC keys set pages by lowercase short code (e.g. 'rna', 'c21'); unknown codes answer HTTP 403.",
			code, err,
		)), nil
	}

	logging.Get().Info().
		Str("tool", "get_edhrec_set_cards").
		Str(paramSet, code).
		Int("card_lists", len(page.Container.JSONDict.CardLists)).
		Msg("Successfully fetched EDHREC set cards")

	return mcp.NewToolResultText(edhrec.FormatSetCards(page, code, limit)), nil
}
