package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPClient struct {
	client *client.Client
	tools  []mcp.Tool
}

func NewMCPClient() (*MCPClient, error) {
	// Get the path to the MCP server binary
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Server binary should be in the parent directory
	serverPath := filepath.Join(filepath.Dir(filepath.Dir(execPath)), "mtg-commander-server")

	// Check if server exists
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("MCP server not found at %s", serverPath)
	}

	// Create stdio client
	mcpClient, err := client.NewStdioMCPClient(serverPath, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Initialize the client
	ctx := context.Background()
	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "mtg-chat-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	// List available tools
	toolsList, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	return &MCPClient{
		client: mcpClient,
		tools:  toolsList.Tools,
	}, nil
}

func (m *MCPClient) GetTools(ctx context.Context) ([]Tool, error) {
	tools := make([]Tool, len(m.tools))
	for i, mcpTool := range m.tools {
		// Convert ToolInputSchema to map[string]interface{}
		schemaMap := map[string]interface{}{
			"type":       mcpTool.InputSchema.Type,
			"properties": mcpTool.InputSchema.Properties,
		}
		if len(mcpTool.InputSchema.Required) > 0 {
			schemaMap["required"] = mcpTool.InputSchema.Required
		}
		if len(mcpTool.InputSchema.Defs) > 0 {
			schemaMap["$defs"] = mcpTool.InputSchema.Defs
		}

		tools[i] = Tool{
			Name:        mcpTool.Name,
			Description: mcpTool.Description,
			InputSchema: schemaMap,
		}
	}
	return tools, nil
}

func (m *MCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	result, err := m.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	})

	if err != nil {
		return "", fmt.Errorf("tool call failed: %w", err)
	}

	// Check if there was an error in the result
	if result.IsError {
		return "", fmt.Errorf("tool returned error")
	}

	// Format the result content
	var output string
	for _, content := range result.Content {
		switch c := content.(type) {
		case mcp.TextContent:
			output += c.Text
		case mcp.ImageContent:
			output += fmt.Sprintf("[Image: %s]", c.Data)
		case mcp.EmbeddedResource:
			// Convert resource to JSON
			jsonData, _ := json.Marshal(c.Resource)
			output += string(jsonData)
		default:
			output += fmt.Sprintf("%v", content)
		}
	}

	return output, nil
}

func (m *MCPClient) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}
