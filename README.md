# MTG Commander MCP Server

[![CI](https://github.com/nathanmartins/mtg-mcp/actions/workflows/ci.yaml/badge.svg)](https://github.com/nathanmartins/mtg-mcp/actions/workflows/ci.yaml)


[![Lint](https://github.com/nathanmartins/mtg-mcp/actions/workflows/lint.yaml/badge.svg)](https://github.com/nathanmartins/mtg-mcp/actions/workflows/lint.yaml)

[![CD](https://github.com/nathanmartins/mtg-mcp/actions/workflows/cd.yaml/badge.svg)](https://github.com/nathanmartins/mtg-mcp/actions/workflows/cd.yaml)

[![Dependabot Updates](https://github.com/nathanmartins/mtg-mcp/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/nathanmartins/mtg-mcp/actions/workflows/dependabot/dependabot-updates)

A Model Context Protocol (MCP) server for Magic: The Gathering Commander format, providing comprehensive card
information, rulings, pricing, and deck validation tools.

## Features

### Tools (AI-Callable Functions)

#### Scryfall Card Data (7 tools)

1. **search_cards** - Search for MTG cards using Scryfall search syntax
   - Supports advanced queries (colors, types, abilities, etc.)
   - Returns up to 50 results with full card details
   - Includes Commander legality status

2. **get_card_details** - Get detailed information about a specific card
   - Oracle text and rules
   - Mana cost, type, power/toughness
   - Color identity
   - Format legalities across all formats
   - Artist and set information

3. **check_commander_legality** - Check if a card is legal in Commander
   - Shows legality status across all formats
   - Clear indication of banned/legal/not legal status
   - Quick format validation

4. **get_card_rulings** - Get official card rulings and clarifications
   - Official WotC rulings
   - Dates and sources for each ruling
   - Comprehensive rules clarifications

5. **get_card_price** - Get current card pricing
   - USD and EUR prices from Scryfall
   - **BRL (Brazilian Real) pricing** via real-time currency conversion
   - Supports both regular and foil versions
   - Optional set-specific pricing

6. **get_banned_list** - Get current Commander banned list
   - Real-time data from Scryfall
   - 85+ banned cards (updated automatically)
   - Complete list with card names

7. **validate_deck** - Validate a Commander deck
   - 100-card deck size check
   - Singleton rule verification (no duplicates except basics)
   - Commander legality check
   - Color identity validation
   - Supports JSON array or text format decklists

#### Moxfield Integration (3 tools)

1. **get_moxfield_deck** - Fetch complete deck from Moxfield
   - Accepts deck URL or public ID
   - Full decklist with card types organized
   - Deck metadata (views, likes, comments, author)
   - Commanders, mainboard, sideboard, maybeboard
   - Last updated timestamp

2. **get_moxfield_user_decks** - Get user's deck list from Moxfield
   - List all decks for a Moxfield user
   - Paginated results (up to 100 per page)
   - Deck summaries with views and likes
   - Format and public URL for each deck

3. **search_moxfield_decks** - Search for decks on Moxfield by commander
   - Search by commander name
   - Filter by format (commander, standard, modern, etc.)
   - Sort by updated, views, or likes
   - Paginated results (up to 100 per page)
   - Returns deck metadata with views, likes, and URLs

#### EDHREC Meta Data (2 tools)

1. **get_edhrec_recommendations** - Get EDHREC recommendations for a commander
   - High synergy cards with synergy scores
   - Most popular cards by inclusion rate
   - New cards trending for the commander
   - Card categories (creatures, instants, artifacts, etc.)
   - Deck count and meta statistics
   - Salt scores for controversial cards

2. **get_edhrec_combos** - Get popular combos for color combinations
   - Combo cards and prerequisites
   - Combo results (e.g., "Infinite mana", "Win the game")
   - Usage statistics and percentages
   - Ranked by popularity
   - Color identity filtering (w/u/b/r/g)

### Resources (Data Sources)

1. **commander://rules** - Complete Commander format rules
   - Deck construction guidelines
   - Gameplay rules
   - Winning conditions
   - Official sources

2. **commander://banned-list** - JSON-formatted banned list
   - Real-time data
   - Card names, types, and mana costs
   - Total count of banned cards

## Installation

### Prerequisites

- Go 1.21 or later
- Internet connection (for Scryfall API and currency conversion)

### Building from Source

```bash
# Clone or navigate to the project directory
cd mtg-mcp

# Install dependencies
go mod tidy

# Build the MCP server
go build -o mtg-commander-server
```

The compiled binary `mtg-commander-server` is an MCP server for use with Claude Desktop or other MCP clients.

## Usage

### Running as MCP Server

The server uses stdio transport for communication with MCP clients like Claude Desktop:

```bash
./mtg-commander-server
```

#### Connecting to Claude Desktop

To use this server with Claude Desktop, add the following configuration to your `claude_desktop_config.json`:

**Location:**

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

**Configuration:**

```json
{
  "mcpServers": {
    "mtg-commander": {
      "command": "/absolute/path/to/mtg-mcp/mtg-commander-server"
    }
  }
}
```

Replace `/absolute/path/to/mtg-mcp/` with the actual path to the binary.

After adding the configuration, restart Claude Desktop.

#### Connecting to Claude Code (Global Configuration)

To make this server available globally in Claude Code across all projects:

#### Option 1: Using the CLI (Recommended)

```bash
claude mcp add --transport stdio mtg-commander /absolute/path/to/mtg-mcp/mtg-commander-server --scope user
```

#### Option 2: Manual Configuration

Edit the global MCP configuration file:

**File location:**

- macOS: `~/.claude/servers.json`
- Windows: `%APPDATA%\ClaudeCode\servers.json`
- Linux: `~/.config/ClaudeCode/servers.json`

**Configuration:**

```json
{
  "mcpServers": {
    "mtg-commander": {
      "command": "/absolute/path/to/mtg-mcp/mtg-commander-server",
      "args": [],
      "transport": "stdio"
    }
  }
}
```

**Verify installation:**

```bash
claude mcp list
```

You should see `mtg-commander` in the list of available servers.

## Example Queries

Once connected to Claude Desktop, you can ask questions like:

**Card Data:**

- "Search for blue counterspells in Commander"
- "Is Mana Crypt legal in Commander?"
- "What are the official rulings for Doubling Season?"
- "How much does Sol Ring cost in BRL?"
- "Show me the current Commander banned list"
- "Validate my Commander deck with Atraxa as commander"

**Moxfield:**

- "Fetch this Moxfield deck: <https://www.moxfield.com/decks/abc123>"
- "Show me decks by user JohnDoe on Moxfield"
- "What's in the mainboard of Moxfield deck xyz789?"
- "Search Moxfield for top Atraxa, Praetors' Voice decks"
- "Find the most popular Thrasios decks on Moxfield sorted by views"

**EDHREC:**

- "What are the best cards for Atraxa, Praetors' Voice according to EDHREC?"
- "Show me popular combos in Dimir colors (ub)"
- "What are high synergy cards for Meren of Clan Nel Toth?"
- "Get me the top 5-color combos for WUBRG"

## Architecture

### Technology Stack

- **Language:** Go 1.21+
- **MCP Framework:** [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
- **Card Data API:** [Scryfall API](https://scryfall.com/docs/api) via [go-scryfall](https://github.com/BlueMonday/go-scryfall)
- **Currency Conversion:** [Frankfurter API](https://www.frankfurter.app/) (free, no API key)
- **Logging:** [zerolog](https://github.com/rs/zerolog) for structured JSON logging
- **Transport:** stdio (Model Context Protocol)

### Data Sources

1. **Card Data:** Scryfall API
   - Updated daily
   - Complete MTG card database
   - Includes rulings, legalities, and pricing
   - Rate-limited to 10 requests/second (built into client)

2. **Commander Rules:** Official format rules embedded in server
   - Source: <https://mtgcommander.net>
   - Format managed by Wizards of the Coast

3. **Pricing:**
   - Base prices: Scryfall (USD/EUR)
   - BRL conversion: Real-time exchange rates via Frankfurter API
   - Note: Prices are indicative and may not reflect Brazilian market conditions

4. **Moxfield:** Unofficial API (<https://api.moxfield.com>)
   - Deck data and user profiles
   - Metadata including views, likes, comments
   - **Note:** No official public API; be respectful of rate limits
   - Contact <support@moxfield.com> for authorized access

5. **EDHREC:** Unofficial JSON endpoints (<https://json.edhrec.com>)
   - Card recommendations and synergies
   - Meta statistics and popularity data
   - Combo database
   - **Rate limit:** Recommend 1 request/second
   - Cached data (may not be real-time)

## Project Structure

```text
mtg-mcp/
├── main.go                 # Core MCP server implementation
├── logger.go               # Structured logging configuration (zerolog)
├── edhrec.go               # EDHREC API integration
├── moxfield.go             # Moxfield API integration
├── http.go                 # HTTP utilities for API calls
├── *_test.go               # Unit test files
│   ├── edhrec_test.go      # Tests for EDHREC functionality
│   ├── moxfield_test.go    # Tests for Moxfield functionality
│   ├── http_test.go        # Tests for HTTP utilities
│   └── logger_test.go      # Tests for logger
├── .github/                # GitHub Actions workflows
│   └── workflows/
│       └── ci.yaml         # CI pipeline with tests and linting
├── go.mod                  # Go module dependencies
├── go.sum                  # Dependency checksums
├── mtg-commander-server    # Compiled MCP server binary
├── mtg-commander-server.log # Server log file (JSON)
└── README.md               # This file
```

## Dependencies

Key dependencies (automatically managed by `go mod`):

- `github.com/mark3labs/mcp-go` v0.43.0 - MCP server and client framework
- `github.com/BlueMonday/go-scryfall` v0.9.1 - Scryfall API client
- `github.com/rs/zerolog` v1.34.0 - Structured JSON logging
- `go.uber.org/ratelimit` v0.2.0 - Rate limiting (via scryfall client)

## Development

### Running Tests

The project includes comprehensive unit tests with good coverage:

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# View coverage report
go tool cover -html=coverage.out
```

### Linting

The project uses golangci-lint for code quality checks:

```bash
# Run linter
golangci-lint run

# Run linter with auto-fix
golangci-lint run --fix
```

### Adding New Tools

1. Define the tool in `registerTools()` using `mcp.NewTool()`
2. Create a handler function with signature:

   ```go
   func (s *MTGCommanderServer) handleToolName(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
   ```

3. Register with `mcpServer.AddTool()`
4. Write unit tests in a corresponding `*_test.go` file

### Adding New Resources

1. Define the resource in `registerResources()` using `mcp.NewResource()`
2. Create a handler function with signature:

   ```go
   func (s *MTGCommanderServer) handleResourceName(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)
   ```

3. Register with `mcpServer.AddResource()`
4. Write unit tests for the resource handler

### Continuous Integration

The project uses GitHub Actions for CI:

- **Build**: Compiles the project for Linux/amd64
- **Test**: Runs all tests with race detection and coverage
- **Lint**: Runs golangci-lint for code quality checks
- **Coverage**: Uploads coverage reports to Codecov (optional)

## Limitations

1. **Brazilian Pricing:** Converted from USD using exchange rates. Does not account for:
   - Import taxes and fees
   - Brazilian market supply/demand
   - Local marketplace pricing (LigaMagic, etc.)

2. **Rate Limiting:** Scryfall API has a 10 req/sec limit (automatically handled)

3. **Deck Validation:** Basic validation only. Full color identity and individual card legality checks require
   manual implementation for large decks.

## Future Enhancements

Potential improvements:

- [x] Moxfield deck fetching and user deck lists
- [x] Moxfield deck search by commander
- [x] EDHREC card recommendations and combo database
- [x] Comprehensive unit tests with CI/CD
- [ ] Direct LigaMagic integration for accurate BRL pricing
- [ ] Caching layer for frequently accessed cards
- [ ] Bulk deck validation with full color identity checking
- [ ] Support for other formats (Modern, Standard, etc.)
- [ ] Card image retrieval
- [ ] Price history tracking
- [ ] Deck building suggestions based on EDHREC data
- [ ] Commander power level estimation (EDH brackets)
- [ ] Archidekt integration for additional deck sources

## Contributing

This is a personal project for Commander format assistance. Contributions, suggestions, and bug reports are welcome!

## License

This project is provided as-is for personal use.

**Note:** This project uses data from Scryfall, which is provided under Wizards of the Coast's Fan Content Policy.
Card data and imagery are property of Wizards of the Coast.

## Acknowledgments

- [Scryfall](https://scryfall.com) for comprehensive MTG card data
- [Wizards of the Coast](https://magic.wizards.com) for Magic: The Gathering
- [Anthropic](https://anthropic.com) for the Model Context Protocol
- The Commander Rules Committee and Commander Format Panel

## Support

For issues or questions:

- Check Scryfall API status: <https://scryfall.com/docs/api>
- Verify MCP server is running: Check Claude Desktop logs
- Review configuration: Ensure correct binary path in `claude_desktop_config.json`

---

**Generated by Claude Code** 🤖
