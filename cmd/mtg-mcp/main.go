// Command mtg-mcp serves Magic: The Gathering Commander tools over the MCP stdio transport.
package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
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

// logFileEnv overrides the default log file path, for setups such as Docker where passing a
// flag is awkward. The --log-file flag wins over it.
const logFileEnv = "MTG_MCP_LOG_FILE"

// exitUsage is the conventional exit status for invalid command-line usage, as the flag
// package itself uses.
const exitUsage = 2

// options are the command-line settings.
type options struct {
	showVersion bool
	debug       bool
	logFile     string
}

// parseOptions parses the command-line arguments (without the program name). Errors and
// usage are written to output, so the caller only has to pick the exit code.
func parseOptions(args []string, getenv func(string) string, output io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("mtg-mcp", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.BoolVar(&opts.showVersion, "version", false, "print the version and exit")
	fs.BoolVar(&opts.debug, "debug", false, "enable debug logging")
	fs.StringVar(&opts.logFile, "log-file", cmp.Or(getenv(logFileEnv), logging.DefaultFilePath()),
		"path of the log file (env "+logFileEnv+")")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() > 0 {
		err := fmt.Errorf("unexpected arguments: %v", fs.Args())
		_, _ = fmt.Fprintln(output, err)
		fs.Usage()
		return opts, err
	}
	return opts, nil
}

func main() {
	opts, err := parseOptions(os.Args[1:], os.Getenv, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	if err != nil {
		os.Exit(exitUsage)
	}

	// Handled before logger init so --version prints cleanly to stdout.
	if opts.showVersion {
		_, _ = fmt.Fprintf(os.Stdout, "mtg-mcp version %s (commit: %s, built: %s)\n", version, commit, buildDate)
		return
	}

	// A missing log file must not stop the server: Init falls back to stderr-only logging.
	logErr := logging.Init(opts.logFile)
	log := logging.Get()
	if logErr != nil {
		log.Warn().Err(logErr).Str("log_file", opts.logFile).Msg("Logging to stderr only")
	}
	if opts.debug {
		logging.SetLevel(logging.LevelDebug)
		log.Debug().Msg("Debug logging enabled")
	}

	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("build_date", buildDate).
		Msg("Initializing MTG Commander MCP Server")

	mtgServer, err := mtgserver.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create MTG Commander server")
	}

	mcpServer := server.NewMCPServer(
		"MTG Commander Assistant",
		version,               // the build version, so clients and bug reports see the real release
		server.WithRecovery(), // Add panic recovery middleware
	)
	mtgServer.Register(mcpServer)

	log.Info().
		Str("transport", "stdio").
		Str("log_file", opts.logFile).
		Msg("Starting MTG Commander MCP Server")

	if serveErr := server.ServeStdio(mcpServer); serveErr != nil {
		log.Fatal().Err(serveErr).Msg("Server error")
	}
}
