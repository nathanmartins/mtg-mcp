package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MTGCommanderServer) handleComprehensiveRulesResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load comprehensive rules: %w", err)
	}
	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: mimeTypeTextPlain,
			Text:     rules.Raw,
		},
	}, nil
}

func (s *MTGCommanderServer) handleGetRule(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	number, err := request.RequireString("rule")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	text, ok := rules.Rule(number)
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("Rule %s not found.", number)), nil
	}
	return mcp.NewToolResultText(FormatRuleForDisplay(number, text)), nil
}

func (s *MTGCommanderServer) handleSearchRules(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	keyword, err := request.RequireString("keyword")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := rulesSearchDefaultLimit
	if limitVal, ok := request.GetArguments()["limit"]; ok {
		if limitFloat, isNum := limitVal.(float64); isNum && limitFloat >= 1 {
			limit = int(limitFloat)
		}
	}
	if limit > rulesSearchMaxLimit {
		limit = rulesSearchMaxLimit
	}
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	return mcp.NewToolResultText(FormatRuleSearchForDisplay(keyword, rules.Search(keyword, limit))), nil
}

func (s *MTGCommanderServer) handleGetGlossaryTerm(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	term, err := request.RequireString("term")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rules, err := s.comprehensiveRules(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load comprehensive rules: %v", err)), nil
	}
	def, ok := rules.GlossaryTerm(term)
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("Glossary term %q not found.", term)), nil
	}
	return mcp.NewToolResultText(FormatGlossaryTermForDisplay(term, def)), nil
}
