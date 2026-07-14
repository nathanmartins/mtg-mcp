# MTG Commander MCP Server

<!-- markdownlint-disable-next-line MD013 -->
[![CI](https://github.com/nathanmartins/mtg-mcp/actions/workflows/ci.yaml/badge.svg)](https://github.com/nathanmartins/mtg-mcp/actions/workflows/ci.yaml) [![CD](https://github.com/nathanmartins/mtg-mcp/actions/workflows/cd.yaml/badge.svg)](https://github.com/nathanmartins/mtg-mcp/actions/workflows/cd.yaml) [![Lint](https://github.com/nathanmartins/mtg-mcp/actions/workflows/lint.yaml/badge.svg)](https://github.com/nathanmartins/mtg-mcp/actions/workflows/lint.yaml) ![Coverage](https://img.shields.io/badge/Coverage-93.2%25-green)

A Model Context Protocol (MCP) server for Magic: The Gathering Commander format, providing comprehensive card
information, rulings, pricing, deck validation tools, and multi-platform deck importing.

<!-- markdownlint-disable-next-line MD013 -->
[![Go Report Card](https://goreportcard.com/badge/github.com/nathanmartins/mtg-mcp)](https://goreportcard.com/report/github.com/nathanmartins/mtg-mcp) [![License](https://img.shields.io/github/license/nathanmartins/mtg-mcp)](https://github.com/nathanmartins/mtg-mcp/blob/main/LICENSE) [![Release](https://img.shields.io/github/v/release/nathanmartins/mtg-mcp)](https://github.com/nathanmartins/mtg-mcp/releases/latest) [![Go Version](https://img.shields.io/github/go-mod/go-version/nathanmartins/mtg-mcp)](https://github.com/nathanmartins/mtg-mcp/blob/main/go.mod)

## Features

### Tools (AI-Callable Functions)

#### Scryfall Card Data (8 tools)

1. **search_cards** – Search for MTG cards using Scryfall search syntax
    - Supports advanced queries (colors, types, abilities, etc.)
    - Returns up to 50 results with full card details
    - Includes Commander legality status

2. **get_card_details** – Get detailed information about a specific card
    - Oracle text and rules
    - Mana cost, type, power/toughness
    - Color identity
    - Format legalities across all formats
    - Artist and set information

3. **check_commander_legality** – Check if a card is legal in Commander
    - Shows legality status across all formats
    - Clear indication of banned/legal/not legal status
    - Quick format validation

4. **get_card_rulings** – Get official card rulings and clarifications
    - Official WotC rulings
    - Dates and sources for each ruling
    - Comprehensive rules clarifications

5. **get_card_price** – Get current card pricing
    - USD and EUR prices from Scryfall
    - **BRL (Brazilian Real) pricing** via real-time currency conversion
    - Supports both regular and foil versions
    - Optional set-specific pricing

6. **get_banned_list** – Get the current Commander banned list
    - Real-time data from Scryfall
    - 85+ banned cards (updated automatically)
    - Complete list with card names

7. **validate_deck** – Validate a Commander deck
    - 100-card deck size check
    - Singleton rule verification (no duplicates except basics)
    - Commander legality check
    - Color identity validation
    - Supports JSON array or text format decklists

8. **get_card_image** – Get a card's image, localized by language
    - Language-specific printing with English fallback
    - Selectable size (small, normal, large, png, art_crop, border_crop)
    - Renders inline in MCP Apps hosts via a `ui://mtg-card` widget
    - Text summary with a Scryfall link as a fallback

#### Moxfield Integration (3 tools)

1. **get_moxfield_deck** – Fetch the complete deck from Moxfield
    - Accepts deck URL or public ID
    - Full decklist with card types organized
    - Deck metadata (views, likes, comments, author)
    - Commanders, mainboard, sideboard, maybeboard
    - Last updated timestamp

2. **get_moxfield_user_decks** – Get user's deck list from Moxfield
    - List all decks for a Moxfield user
    - Paginated results (up to 100 per page)
    - Deck summaries with views and likes
    - Format and public URL for each deck

3. **search_moxfield_decks** – Search for decks on Moxfield by commander
    - Search by commander name
    - Filter by format (commander, standard, modern, etc.)
    - Sort by updated, views, or likes
    - Paginated results (up to 100 per page)
    - Returns deck metadata with views, likes, and URLs

#### Archidekt Integration (3 tools)

1. **get_archidekt_deck** – Fetch a complete deck from Archidekt
    - Accepts deck URL or numeric ID (e.g., `https://archidekt.com/decks/12345` or `12345`)
    - Full decklist organised by a card type
    - Commander(s) identified via the premier category flag
    - Deck metadata (format, owner, views, EDH bracket, last updated)
    - Direct link back to the Archidekt deck page
    - Optional `lands_only` flag — returns only land cards, ideal for landbase comparisons
      without the token overhead of a full decklist

2. **get_archidekt_user_decks** – Get a user's public decks from Archidekt
    - List all public decks for any Archidekt username
    - Paginated results with direct deck URLs
    - Format name, view count, and last-updated date for each deck

3. **search_archidekt_decks** – Search public Commander decks by commander name
    - Required `commander` parameter (card name, e.g. `"Atraxa, Praetors' Voice"`)
    - Optional `bracket` filter (1–4; omit searching all brackets)
    - Optional `limit` (default: 10, max: 20)
    - Results sorted by view count descending
    - Each result includes deck name, author, view count, EDH bracket, last updated, and direct URL

#### Comprehensive Rules (3 tools)

1. **get_rule** - Get a specific Comprehensive Rule by number (e.g. `702.19`), including its subrules.

2. **search_rules** - Search the Comprehensive Rules text by keyword (default 10 results, max 20).

3. **get_glossary_term** - Get a Comprehensive Rules glossary definition by term.

#### EDHREC Meta Data (2 tools)

1. **get_edhrec_recommendations** – Get EDHREC recommendations for a commander
    - High-synergy cards with synergy scores
    - Most popular cards by inclusion rate
    - New cards trending for the commander
    - Card categories (creatures, instants, artifacts, etc.)
    - Deck count and meta-statistics
    - Salt scores for controversial cards

2. **get_edhrec_combos** – Get popular combos for color combinations
    - Combo cards and prerequisites
    - Combo results (e.g., "Infinite mana", "Win the game")
    - Usage statistics and percentages
    - Ranked by popularity
    - Color identity filtering (w/u/b/r/g)

### Resources (Data Sources)

1. **commander://rules** – Complete Commander format rules
    - Deck construction guidelines
    - Gameplay rules
    - Winning conditions
    - Official sources

2. **commander://banned-list** - JSON-formatted banned list
    - Real-time data
    - Card names, types, and mana costs
    - Total count of banned cards

3. **rules://comprehensive** - Full official Magic: The Gathering Comprehensive Rules (fetched from WotC, large)
   - Fetched at runtime from the official WotC page
   - Cached for repeated use
   - Includes all rules sections and glossary

## Installation

### Prerequisites

- Go 1.26 or later
- Internet connection (for Scryfall API and currency conversion)

### Building from Source

```bash
# Clone or navigate to the project directory
cd mtg-mcp

# Install dependencies
go mod tidy

# Build the MCP server
go build -o mtg-mcp
```

The compiled binary `mtg-mcp` is an MCP server for use with Claude Desktop or other MCP clients.

## Usage

### Running as an MCP Server

The server uses stdio transport for communication with MCP clients like Claude Desktop:

```bash
./mtg-mcp
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
      "command": "/absolute/path/to/mtg-mcp/mtg-mcp"
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
claude mcp add --transport stdio mtg-commander /absolute/path/to/mtg-mcp/mtg-mcp --scope user
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
      "command": "/absolute/path/to/mtg-mcp/mtg-mcp",
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

**Archidekt:**

- "Fetch this Archidekt deck: <https://archidekt.com/decks/12345>"
- "Show me decks by user NorwegianWhaler on Archidekt"
- "What's the commander and decklist for Archidekt deck 67890?"

**EDHREC:**

- "What are the best cards for Atraxa, Praetors' Voice, according to EDHREC?"
- "Show me popular combos in Dimir colors (ub)"
- "What are high-synergy cards for Meren of Clan Nel Toth?"
- "Get me the top 5-color combos for WUBRG"

## Architecture

### Technology Stack

- **Language:** Go 1.26+
- **MCP Framework:** [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
- **Card Data API:** [Scryfall API](https://scryfall.com/docs/api)
  via [go-scryfall](https://github.com/BlueMonday/go-scryfall)
- **Currency Conversion:** [Frankfurter API](https://www.frankfurter.app/) (free, no API key)
- **Logging:** [zerolog](https://github.com/rs/zerolog) for structured JSON logging
- **Transport:** stdio (Model Context Protocol)

### Data Sources

1. **Card Data:** Scryfall API
    - Updated daily
    - Complete MTG card database
    - Includes rulings, legalities, and pricing
    - Rate-limited to 10 requests/second (built into a client)

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

5. **Archidekt:** Open public API (<https://archidekt.com/api>)
    - Deck data including full card lists and categories
    - Owner, format, EDH bracket, and view count metadata
    - **Note:** API is open for read access; credit deck creators when publishing data
    - No API key required

6. **EDHREC:** Unofficial JSON endpoints (<https://json.edhrec.com>)
    - Card recommendations and synergies
    - Meta-statistics and popularity data
    - Combo database
    - **Rate limit:** Recommend 1 request/second
    - Cached data (may not be real-time)

## Project Structure

```text
mtg-mcp/
├── main.go                  # Core MCP server, tool/resource registration
├── logger.go                # Structured logging configuration (zerolog)
├── http.go                  # HTTP utilities for API calls
├── resources.go             # MCP resource handlers (rules, banned list)
├── archidekt.go             # Archidekt API integration
├── edhrec.go                # EDHREC API integration
├── moxfield.go              # Moxfield API integration
├── *_handlers.go            # MCP tool handlers per source
│   ├── scryfall_handlers.go  # Scryfall tools (search, details, price, …)
│   ├── archidekt_handlers.go # Archidekt tools
│   ├── edhrec_handlers.go    # EDHREC tools
│   └── moxfield_handlers.go  # Moxfield tools
├── *_test.go                # Unit test files (with httptest mocks)
│   ├── archidekt_test.go    # Tests for Archidekt functionality
│   ├── edhrec_test.go       # Tests for EDHREC functionality
│   ├── moxfield_test.go     # Tests for Moxfield functionality
│   ├── http_test.go         # Tests for HTTP utilities
│   ├── logger_test.go       # Tests for logger
│   ├── server_test.go       # Tests for server construction & registration
│   ├── resources_test.go    # Tests for MCP resource handlers
│   ├── scryfall_handlers_test.go          # Tests for decklist parsing helpers
│   ├── scryfall_handlers_internal_test.go # Tests for Scryfall tool handlers
│   ├── handlers_success_test.go           # Deck-handler success-path tests
│   └── handlers_error_test.go             # Handler request-validation tests
├── *_e2e_test.go            # E2E test files (real API calls)
│   ├── archidekt_e2e_test.go # E2E tests for Archidekt API
│   ├── edhrec_e2e_test.go    # E2E tests for EDHREC API
│   ├── moxfield_e2e_test.go  # E2E tests for Moxfield API
│   └── scryfall_e2e_test.go  # E2E tests for Scryfall API
├── .github/                 # GitHub Actions workflows
│   └── workflows/
│       ├── ci.yaml          # CI pipeline (unit tests + linting)
│       ├── e2e.yaml         # Manual E2E tests + coverage workflow
│       ├── cd.yaml          # Continuous deployment
│       └── lint.yaml        # Code quality checks
├── go.mod                   # Go module dependencies
├── go.sum                   # Dependency checksums
├── mtg-mcp                  # Compiled MCP server binary
├── mtg-commander-server.log # Server log file (JSON)
└── README.md                # This file
```

## Development

### Quick Start with Makefile

The project includes a Makefile with common development tasks (compatible with macOS and Linux):

```bash
# View all available commands
make help

# Run all checks (format, lint, unit tests)
make check

# Build the binary
make build

# Run unit tests (fast)
make test-unit

# Run all tests including E2E
make test

# Run tests with coverage report
make test-coverage

# Format and lint code
make fmt lint

# Run CI pipeline locally
make ci
```

**Common Makefile Commands:**

| Command                      | Description                                        |
|------------------------------|----------------------------------------------------|
| `make help`                  | Show all available commands                        |
| `make build`                 | Build the binary                                   |
| `make test-unit`             | Run unit tests only (fast, skips E2E)              |
| `make test-e2e`              | Run E2E tests only                                 |
| `make test`                  | Run all tests (unit + E2E)                         |
| `make test-coverage`         | Generate HTML coverage report                      |
| `make test-coverage-cli`     | Show coverage in terminal                          |
| `make update-coverage-badge` | Update coverage badge in README with proper colors |
| `make test-race`             | Run tests with race detector                       |
| `make bench`                 | Run benchmarks                                     |
| `make fmt`                   | Format Go code                                     |
| `make lint`                  | Run all linters                                    |
| `make check`                 | Run fmt + lint + test-unit                         |
| `make ci`                    | Run full CI pipeline locally                       |
| `make clean`                 | Clean build artifacts                              |
| `make deps`                  | Download dependencies                              |
| `make tidy`                  | Tidy go.mod                                        |

### Running Tests

The project includes comprehensive unit tests with good coverage:

```bash
# Run all tests (unit tests only, skips E2E)
go test -short -v ./...

# Run tests with coverage
go test -short -v -race -coverprofile=coverage.out -covermode=atomic ./...

# View coverage report
go tool cover -html=coverage.out
```

### End-to-End (E2E) Tests

E2E tests validate the integration with real external APIs (Scryfall, EDHREC, Moxfield) and are **not run
automatically** in CI. They test:

- Real API connectivity and response formats
- Data structure validation with actual API responses
- Card name sanitization with real EDHREC endpoints
- Moxfield deck fetching and search
- Scryfall card queries and pricing

**Running E2E Tests Locally:**

```bash
# Run all tests including E2E (no -short flag)
go test -v ./...

# Run only E2E tests
go test -v -run E2E ./...

# Run with timeout for slower APIs
go test -v -timeout 5m ./...
```

**Running E2E Tests via GitHub Actions:**

E2E tests can be triggered manually through the GitHub Actions workflow:

1. Go to the "Actions" tab in the repository
2. Select "E2E Tests & Coverage" workflow
3. Click "Run workflow"
4. View test results, coverage, and logs

The workflow:

- Runs unit tests first for quick validation
- Runs all tests including E2E (without `-short` flag)
- Generates coverage badge and auto-commits to README
- Has a 15-minute timeout to accommodate API calls
- Uploads test logs as artifacts for debugging
- Only runs when manually triggered (not on push/PR)

**E2E Test Files:**

- `edhrec_e2e_test.go` - EDHREC API integration tests
- `moxfield_e2e_test.go` - Moxfield API integration tests
- `scryfall_e2e_test.go` - Scryfall API integration tests

**Note:** E2E tests make real API calls and are subject to:

- API rate limits
- Network latency
- External API availability
- Potential API changes

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
2. Create a handler function with a signature:

   ```go
   func (s *MTGCommanderServer) handleToolName(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
   ```

3. Register with `mcpServer.AddTool()`
4. Write unit tests in a corresponding `*_test.go` file

### Adding New Resources

1. Define the resource in `registerResources()` using `mcp.NewResource()`
2. Create a handler function with a signature:

   ```go
   func (s *MTGCommanderServer) handleResourceName(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)
   ```

3. Register with `mcpServer.AddResource()`
4. Write unit tests for the resource handler

### Continuous Integration

The project uses GitHub Actions with two main workflows:

**CI Workflow** (runs on push/PR):

- **Build**: Compiles the project for Linux/amd64
- **Test**: Runs unit tests (with `-short` flag) with race detection
- **Lint**: Runs golangci-lint for code quality checks
- Fast feedback loop for development (skips E2E tests)

**E2E Tests & Coverage Workflow** (manual trigger only):

- **Unit Tests**: Runs all unit tests first
- **E2E Tests**: Runs full test suite including real API calls to Scryfall, EDHREC, and Moxfield
- **Coverage**: Generates a coverage report from the full test suite
- **Badge Generation**: Updates README with current coverage percentage
- **Auto-commit**: Commits badge changes automatically
- **Artifacts**: Uploads test logs for debugging (7-day retention)
- **Timeout**: 15 minutes to accommodate external API calls
- Uses [tj-actions/coverage-badge-go](https://github.com/tj-actions/coverage-badge-go)
- Thresholds: Green ≥80%, Yellow ≥60%, Red <60%
- Triggered via GitHub Actions UI → "E2E Tests & Coverage" → "Run workflow"

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
- [x] Archidekt deck fetching and user deck lists
- [ ] Direct LigaMagic integration for accurate BRL pricing
- [ ] Caching layer for frequently accessed cards
- [ ] Bulk deck validation with full color identity checking
- [ ] Card image retrieval
- [ ] Price history tracking
- [ ] Deck building suggestions based on EDHREC data
- [x] EDH bracket filtering and display for Archidekt decks
- [ ] Commander power level estimation (compute a bracket from a decklist)

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
- Review configuration: Ensure a correct binary path (`mtg-mcp`) in `claude_desktop_config.json`
