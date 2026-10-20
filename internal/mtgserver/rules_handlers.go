package mtgserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nathanmartins/mtg-mcp/internal/rules"
)

func (s *Server) handleComprehensiveRulesResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	cr, err := s.rules.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load comprehensive rules: %w", err)
	}
	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: mimeTypeTextPlain,
			Text:     cr.Raw,
		},
	}, nil
}

func (s *Server) handleGetRule(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	number, err := request.RequireString("rule")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	cr, err := s.rules.Get(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	text, ok := cr.Rule(number)
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("Rule %s not found.", number)), nil
	}
	return mcp.NewToolResultText(rules.FormatRule(number, text)), nil
}

func (s *Server) handleSearchRules(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	keyword, err := request.RequireString("keyword")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit, err := cappedIntArg(request.GetArguments(), "limit", rules.SearchDefaultLimit, 1, rules.SearchMaxLimit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	cr, err := s.rules.Get(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	return mcp.NewToolResultText(rules.FormatRuleSearch(keyword, cr.Search(keyword, limit))), nil
}

func (s *Server) handleGetGlossaryTerm(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	term, err := request.RequireString("term")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	cr, err := s.rules.Get(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	def, ok := cr.GlossaryTerm(term)
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("Glossary term %q not found.", term)), nil
	}
	return mcp.NewToolResultText(rules.FormatGlossaryTerm(term, def)), nil
}
