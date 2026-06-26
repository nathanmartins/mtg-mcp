# Comprehensive Rules resource & tools — design

**Date:** 2026-06-26
**Status:** Approved (pending spec review)
**Branch:** `feat/comprehensive-rules`

## Goal

Give the MCP server autonomous access to the official Magic: The Gathering
**Comprehensive Rules**, fetched from the WotC site at runtime. Expose them both
as a full-text resource and as targeted lookup tools, so an LLM can cite a
specific rule or glossary term without ingesting the whole document.

## Source (verified 2026-06-26)

- Landing page `https://magic.wizards.com/en/rules` — reachable (HTTP 200, not
  bot-blocked).
- The page's embedded JSON contains the current rules file link, e.g.
  `https://media.wizards.com/2026/downloads/MagicCompRules 20260619.txt`
  (note the **space** in the filename → must be URL-encoded when fetched).
- The `.txt`: HTTP 200, `text/plain`, **~975 KB / ~9,367 lines**, starts with a
  BOM + header ("effective as of June 19, 2026"), numbered rules like
  `702.19. Trample`, subrules `702.19a`…, then a **Glossary** section.
- The dated filename changes ~quarterly, so the link must be **discovered**
  from the page, not hardcoded.

## Scope

In scope:
- Resource `rules://comprehensive` (full text, `text/plain`).
- Tools `get_rule`, `search_rules`, `get_glossary_term`.
- Runtime fetch + parse + in-memory cache.

Out of scope (YAGNI): disk cache, embedded snapshot/fallback file, PDF parsing,
non-English rules, format-specific rule filtering.

## Architecture

New file **`rules.go`** (one-file-per-concern, matching `archidekt.go` /
`moxfield.go` / `edhrec.go`). Handlers wired in `main.go`.

### Components (each independently testable)

1. **Data acquisition** — `GetComprehensiveRules` / `getComprehensiveRulesWithURL`
   (the project's `GetX`/`getXWithURL` testability split):
   - `GetComprehensiveRules(ctx) (*ComprehensiveRules, error)` hardcodes the prod
     page URL and delegates to
     `getComprehensiveRulesWithURL(ctx, pageURL string)`.
   - Steps: GET `pageURL` → regex-discover the rules `.txt` URL
     (case-insensitive `MagicCompRules…\.txt`, robust to the date) → GET that
     `.txt` → `parseComprehensiveRules`.
   - Builds its own `&http.Client{}` (as `archidekt.go` does) so http
     `httptest` servers work. In tests the fake page links to the test server's
     own `.txt` path.

2. **Parser** — `parseComprehensiveRules(raw string) *ComprehensiveRules` (pure):
   - Splits numbered rules via a line-anchored pattern (`^\d+\.\d+…`), building an
     ordered slice + a `map[string]string` index keyed by rule number (parent
     rule text plus its subrules grouped under the parent).
   - Detects the `Glossary` section and parses it into `map[string]string`
     (term → definition).

3. **Lookup** — pure methods on `*ComprehensiveRules`:
   - `Rule(number string) (text string, ok bool)` — a parent number (e.g.
     `702.19`) returns the rule plus all its subrules; a subrule number (e.g.
     `702.19a`) returns just that subrule.
   - `Search(keyword string, limit int) []RuleMatch` — case-insensitive substring
     match across rule text, capped at `limit`. The `search_rules` tool defaults
     `limit` to **10** and clamps to **20** (mirroring `searchArchidektDecks`).
   - `GlossaryTerm(term string) (def string, ok bool)` — case-insensitive exact
     term match.

4. **Cache** — on `MTGCommanderServer`:
   - New mutex-guarded field holding `*ComprehensiveRules` + fetch timestamp.
   - `s.comprehensiveRules(ctx)` returns the cached value, refetching when empty
     or older than `rulesCacheTTL` (**default 24h**). In-memory, per-process.

5. **Handlers** (`main.go`), each formatting via a `Format…ForDisplay` helper:
   - Resource `rules://comprehensive` → full text.
   - Tool `get_rule` (param: rule number) → `FormatRuleForDisplay`.
   - Tool `search_rules` (params: keyword, optional limit) →
     `FormatRuleSearchForDisplay`.
   - Tool `get_glossary_term` (param: term) → `FormatGlossaryTermForDisplay`.

## Data flow

read/tool call → handler → `s.comprehensiveRules(ctx)` →
[cache fresh? return it] | [fetch page → discover `.txt` → fetch + parse → store]
→ `Rule` / `Search` / `GlossaryTerm` → `Format…ForDisplay` → result text.

## Error handling

- **Fetch/discovery/parse failure:** if a previous cache exists, serve it (stale)
  and log a warning; otherwise return an error — consistent with
  `handleBannedListResource`, which returns an error when Scryfall fails.
- **Unknown rule number / glossary term / no search matches:** return friendly
  "not found" **text** (not a Go error), so the tool call still succeeds.

## Wiring & constants

- `totalToolCount` 14 → **17**; `totalResourceCount` 2 → **3** (in `main.go`).
- New `registerRulesTools(mcpServer)` to keep `registerTools` within `funlen`
  (mirrors the existing `registerArchidektTools` split). The resource is added in
  `registerResources`.
- New constants in `rules.go`: `rulesPageURL`, `rulesCacheTTL`, and the discovery
  regex.

## Testing

- **Unit** (`rules_test.go`, `httptest`-backed, via the `WithURL` split):
  - Fake rules page linking to the test server's `.txt`; small fixture `.txt`
    with a couple of numbered rules + subrules + a glossary entry.
  - Assert discovery + fetch + parse; `Rule` (incl. subrules), `Search`,
    `GlossaryTerm`; **cache** behaviour (2nd call does not refetch — verify with a
    request counter on the test server); failure → error and stale-serve.
  - Pure parser tests with inline fixtures.
- **E2E** (`rules_e2e_test.go`, `Test…E2E`, gated by `testing.Short()`):
  hit the real WotC page; assert `702.19`/"Trample" retrievable and the glossary
  contains "Trample".

## Conventions to follow (creator's patterns — must adhere)

- **`GetX` / `getXWithURL` split** for every network call; tests inject an
  `httptest` URL through the `WithURL` variant. No HTTP-mock framework.
- **One file per concern** (`rules.go`); handlers stay in `main.go` with the
  signature `func (s *MTGCommanderServer) handleX(ctx, mcp.CallToolRequest)
  (*mcp.CallToolResult, error)`.
- **Handler → fetch → `Format…ForDisplay`** pipeline; titled lists via a small
  string-returning helper with an empty-guard (cf. `formatCardGroup`).
- Build a plain `&http.Client{}` in the fetch (as `archidekt.go`), since the
  shared `HTTPGet` enforces HTTPS and would reject http test servers.
- Map-as-set / index helpers built in a simple loop (cf.
  `archidektPremierCategories`).
- Diagnostics through the global zerolog logger (`GetLogger`), never `fmt.Print`
  to stdout (owned by the MCP protocol). Globals carry `//nolint:gochecknoglobals`.
- Keep functions within `golangci-lint` limits (`funlen` 200 lines/70 stmts,
  `golines` 120, `cyclop`, `exhaustive`, `goimports`); run `make check`.
- Bump `totalToolCount` / `totalResourceCount` when adding tools/resources.
- Update ADR (new ADR-007) and re-index after merge.

## Risks

- WotC could later bot-block the page (cf. Moxfield's Cloudflare wall) or change
  the embedded-link format → discovery regex breaks. Mitigated by stale-serve and
  a clear error; E2E test surfaces breakage.
- ~975 KB full-text resource is heavy for clients that read it; documented, and
  the lookup tools are the intended path.
