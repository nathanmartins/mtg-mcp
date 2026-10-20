package mtgserver

// Tool argument keys without a production constant; the tools register them in server.go.
const (
	argLimit         = "limit"
	argPage          = "page"
	argPageSize      = "page_size"
	argDeckID        = "deck_id"
	argUsername      = "username"
	argColors        = "colors"
	argDecklist      = "decklist"
	argQuery         = "query"
	argKeyword       = "keyword"
	argSort          = "sort"
	argSortDirection = "sort_direction"
)

// Test fixture values shared across this package's tests.
const (
	testAtraxa              = "Atraxa, Praetors' Voice"
	testAtraxaShort         = "Atraxa"
	testBanned              = "banned"
	testDeckID              = "abc123"
	testDecklistSolRing     = "1 Sol Ring"
	testDelver              = "Delver of Secrets"
	testDoublingSeason      = "Doubling Season"
	testGibberish           = "zzz"
	testInsectileAberration = "Insectile Aberration"
	testIsland              = "Island"
	testLegal               = "legal"
	testLightningBolt       = "Lightning Bolt"
	testManaCrypt           = "Mana Crypt"
	testMountain            = "Mountain"
	testPlains              = "Plains"
	testRulesURI            = "commander://rules"
	testSetC21              = "c21"
	testSolRing             = "Sol Ring"
	testSolRingQuery        = "sol ring"
	testThemeInfect         = "infect"
	testTrample             = "Trample"
	testUnknownCard         = "nope"
	testUsername            = "tester"
	unreachableURL          = "http://127.0.0.1:1"
	testSortViews           = "views"
	testSol                 = "sol"
	testKeywordTrample      = "trample"
)
