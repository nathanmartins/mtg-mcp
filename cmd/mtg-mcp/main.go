// Command mtg-mcp serves Magic: The Gathering Commander tools over the MCP stdio transport.
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanmartins/mtg-mcp/internal/logging"
	"github.com/nathanmartins/mtg-mcp/internal/mtgserver"
)

// Build-time variables injected via -ldflags.
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	// Handle flags before logger init so --version can print cleanly to stdout
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version":
			_, _ = fmt.Fprintf(os.Stdout, "mtg-mcp version %s (commit: %s, built: %s)\n", version, commit, buildDate)
			os.Exit(0)
		}
	}

	// Initialize logger
	logFilePath := "mtg-commander-server.log"
	if err := logging.Init(logFilePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log := logging.Get()
	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("build_date", buildDate).
		Msg("Initializing MTG Commander MCP Server")

	// Check for log level flag
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		logging.SetLevel(logging.LevelDebug)
		log.Debug().Msg("Debug logging enabled")
	}

	// Create MTG Commander server instance
	log.Info().Msg("Creating MTG Commander server instance")
	mtgServer, err := mtgserver.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create MTG Commander server")
	}
	log.Info().Msg("MTG Commander server instance created successfully")

	// Create MCP server
	log.Info().Msg("Creating MCP server")
	mcpServer := server.NewMCPServer(
		"MTG Commander Assistant",
		"1.0.0",
		server.WithRecovery(), // Add panic recovery middleware
	)

	// Register all tools and resources
	log.Info().Msg("Registering MCP tools and resources")
	mtgServer.Register(mcpServer)
	log.Info().Msg("All tools and resources registered successfully")

	// Start server with stdio transport
	log.Info().
		Str("transport", "stdio").
		Str("log_file", logFilePath).
		Msg("Starting MTG Commander MCP Server")

	if serveErr := server.ServeStdio(mcpServer); serveErr != nil {
		log.Fatal().Err(serveErr).Msg("Server error")
	}
}
