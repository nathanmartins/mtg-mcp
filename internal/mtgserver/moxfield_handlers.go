package mtgserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nathanmartins/mtg-mcp/internal/logging"
	"github.com/nathanmartins/mtg-mcp/internal/moxfield"
)

func (s *Server) handleGetMoxfieldDeck(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	deckID, err := request.RequireString("deck_id")
	if err != nil {
		logging.Get().Error().Err(err).Str("tool", "get_moxfield_deck").Msg("Missing deck_id parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	publicID := moxfield.ExtractPublicID(deckID)

	logging.Get().Info().
		Str("tool", "get_moxfield_deck").
		Str("deck_id", publicID).
		Msg("Fetching Moxfield deck")

	deck, err := s.moxfield.GetDeck(ctx, publicID)
	if err != nil {
		logging.Get().Error().
			Err(err).
			Str("tool", "get_moxfield_deck").
			Str("deck_id", publicID).
			Msg("Failed to fetch Moxfield deck")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch Moxfield deck: %v", err)), nil
	}

	logging.Get().Info().
		Str("tool", "get_moxfield_deck").
		Str("deck_id", publicID).
		Str("deck_name", deck.Name).
		Str("format", deck.Format).
		Int("mainboard_cards", len(deck.Mainboard)).
		Msg("Successfully fetched Moxfield deck")

	output := moxfield.FormatDeck(deck)
	return mcp.NewToolResultText(output), nil
}

func (s *Server) handleGetMoxfieldUserDecks(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	username, err := request.RequireString("username")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	const defaultPageSize = 20
	pageSize, err := cappedIntArg(request.GetArguments(), "page_size", defaultPageSize, 1, maxPageSize)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	decks, err := s.moxfield.GetUserDecks(ctx, username, pageSize)
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
func moxfieldSearchParamsFromRequest(commander string, args map[string]any) (moxfield.SearchParams, error) {
	params := moxfield.SearchParams{
		CardName: commander,
		Format:   stringArg(args, "format", defaultFormat),
	}

	page, err := intArg(args, "page", 1)
	if err != nil {
		return params, err
	}
	params.PageNumber = page

	limit, err := intArg(args, "limit", moxfield.SearchDefaultLimit)
	if err != nil {
		return params, err
	}
	params.PageSize = limit

	sortKey, err := enumArg(args, "sort", searchSortDefault)
	if err != nil {
		return params, err
	}
	sortType, err := moxfield.ParseSortType(sortKey)
	if err != nil {
		return params, err
	}
	params.SortType = sortType

	ascending, err := sortAscendingArg(args)
	if err != nil {
		return params, err
	}
	params.SortDirection = moxfield.SortDirection(ascending)

	if params.PageNumber < 1 {
		return params, fmt.Errorf("%w page %d (must be 1 or greater)", errInvalid, params.PageNumber)
	}
	// The limit is bounded by what verification can actually deliver, not by Moxfield's
	// page size: at most moxfield.VerifyMaxChecks decks are ever fetched and confirmed,
	// so a larger limit could never be honoured.
	if params.PageSize < 1 || params.PageSize > moxfield.VerifyMaxChecks {
		return params, fmt.Errorf("%w limit %d (accepted: 1-%d)", errInvalid, params.PageSize, moxfield.VerifyMaxChecks)
	}

	return params, nil
}

func (s *Server) handleSearchMoxfieldDecks(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	commander, err := request.RequireString(paramCommander)
	if err != nil {
		logging.Get().Error().Err(err).Str("tool", "search_moxfield_decks").Msg("Missing commander parameter")
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := request.GetArguments()
	params, err := moxfieldSearchParamsFromRequest(commander, args)
	if err != nil {
		logging.Get().Error().
			Err(err).
			Str("tool", "search_moxfield_decks").
			Str(paramCommander, commander).
			Str("sort", stringArg(args, "sort", searchSortDefault)).
			Str("sort_direction", stringArg(args, "sort_direction", sortDirectionDesc)).
			Int("page", params.PageNumber).
			Int("limit", params.PageSize).
			Msg("Rejected Moxfield search arguments")
		return mcp.NewToolResultError(err.Error()), nil
	}

	logging.Get().Info().
		Str("tool", "search_moxfield_decks").
		Str(paramCommander, commander).
		Str("format", params.Format).
		Str("sort", params.SortType).
		Int("page", params.PageNumber).
		Int("limit", params.PageSize).
		Msg("Searching Moxfield decks")

	wanted := params.PageSize
	// The over-fetch is capped by the verification budget: candidates beyond
	// moxfield.VerifyMaxChecks could never be checked. Verification also stops as soon as
	// it has `wanted` matches, so any candidate left over on this page is skipped for
	// good — page 2 resumes after the whole requested page. The output discloses how
	// many candidates were left unexamined.
	params.PageSize = min(wanted*moxfield.CandidateFactor, moxfield.VerifyMaxChecks)

	results, err := s.moxfield.SearchDecks(ctx, params)
	if err != nil {
		logging.Get().Error().
			Err(err).
			Str("tool", "search_moxfield_decks").
			Str(paramCommander, commander).
			Msg("Failed to search Moxfield decks")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search Moxfield decks: %v", err)), nil
	}

	verifyCtx, cancel := context.WithTimeout(ctx, moxfield.VerifyBudget)
	defer cancel()
	outcome := s.moxfield.VerifyCommanderDecks(
		verifyCtx, results.Data, commander,
		wanted, moxfield.VerifyMaxChecks,
	)

	logging.Get().Info().
		Str("tool", "search_moxfield_decks").
		Str(paramCommander, commander).
		Int("candidates", outcome.Candidates).
		Int("checked", outcome.Checked).
		Int("verified", len(outcome.Decks)).
		Int("unexamined", outcome.Unexamined).
		Bool("incomplete", outcome.Incomplete).
		Msg("Successfully searched Moxfield decks")

	return mcp.NewToolResultText(formatMoxfieldCommanderSearch(commander, params, results, outcome)), nil
}

// formatMoxfieldCommanderSearch renders verified commander decks. Moxfield's search API
// cannot filter by commander, so the output states exactly how many candidates were
// checked, how many were never looked at, and whether verification was cut short.
func formatMoxfieldCommanderSearch(
	commander string,
	params moxfield.SearchParams,
	results *moxfield.SearchResponse,
	outcome moxfield.CommanderSearch,
) string {
	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "# Moxfield Decks for %s\n\n", commander)
	_, _ = fmt.Fprintf(&output, "**Format:** %s\n", params.Format)
	_, _ = fmt.Fprintf(&output, "**Sort:** %s (%s)\n",
		params.SortType, sortDirectionLabel(params.SortDirection == moxfield.DirectionAscending))
	_, _ = fmt.Fprintf(&output, "**Candidates containing the card:** %d (page %d, %s)\n",
		outcome.Candidates, results.PageNumber, formatMoxfieldTotal(results))
	_, _ = fmt.Fprintf(&output, "**Verified as commander:** %d of %d checked%s\n",
		len(outcome.Decks), outcome.Checked, unfetchableSuffix(outcome.Failed))
	if outcome.Unexamined > 0 {
		// Page N+1 resumes after this whole candidate page, so these candidates are not
		// merely deferred — nothing will ever look at them.
		_, _ = fmt.Fprintf(&output,
			"**Not examined on this page:** %d candidate(s) (match limit reached; paging skips them)\n",
			outcome.Unexamined)
	}
	output.WriteString("\n")

	if outcome.Incomplete {
		_, _ = fmt.Fprintf(&output,
			"> ⚠️ Verification incomplete — %s. More matching decks may exist on this page.\n\n",
			outcome.Reason)
	}

	if len(outcome.Decks) == 0 {
		output.WriteString("No decks with this commander were found among the checked candidates.\n")
		return output.String()
	}

	for i, deck := range outcome.Decks {
		_, _ = fmt.Fprintf(&output, "## %d. %s\n", i+1, deck.Name)
		_, _ = fmt.Fprintf(&output, "- **Format:** %s\n", deck.Format)
		_, _ = fmt.Fprintf(&output, "- **Deck ID:** %s\n", deck.PublicID)
		_, _ = fmt.Fprintf(&output, "- **Views:** %d | **Likes:** %d\n", deck.ViewCount, deck.LikeCount)
		_, _ = fmt.Fprintf(&output, "- **URL:** %s\n\n", deck.PublicURL)
	}

	return output.String()
}

// unfetchableSuffix names the candidates whose deck could not be read. Without it the
// checked count would read as a verdict on every candidate, when an unreadable deck is
// neither a match nor a non-match.
func unfetchableSuffix(failed int) string {
	if failed == 0 {
		return ""
	}

	return fmt.Sprintf(", %d could not be fetched", failed)
}

// formatMoxfieldTotal renders Moxfield's search total without presenting a saturated
// value as a real count. Moxfield answers moxfield.TotalCap for any broad card-name
// search and derives totalPages from it, so both numbers would otherwise advertise a
// result depth that does not exist.
func formatMoxfieldTotal(results *moxfield.SearchResponse) string {
	if results.TotalResults >= moxfield.TotalCap {
		return fmt.Sprintf("%d+ card-name matches — upstream cap, real total unknown", moxfield.TotalCap)
	}

	return fmt.Sprintf("%d card-name matches over %d page(s)", results.TotalResults, results.TotalPages)
}
