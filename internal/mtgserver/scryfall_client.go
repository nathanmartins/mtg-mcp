package mtgserver

import (
	"time"

	"github.com/BlueMonday/go-scryfall"
	"go.uber.org/ratelimit"
)

const (
	// scryfallRequestsPerSecond follows Scryfall's published limits: the search, named,
	// random and collection endpoints this server mostly uses allow two requests per second
	// (other endpoints allow ten). Exceeding them earns HTTP 429 and, if repeated, a network
	// block, so the stricter rate applies to every request.
	scryfallRequestsPerSecond = 2
	// scryfallUserAgent identifies this application, as Scryfall asks API clients to do.
	scryfallUserAgent = "MTG-Commander-MCP-Server/1.0"
)

// scryfallLimiter is shared by every Scryfall client in the process: Scryfall limits by client
// IP, so separate per-client limiters would each allow the full rate and together exceed it.
// WithoutSlack stops idle time from banking a burst of back-to-back requests.
var scryfallLimiter = newScryfallLimiter() //nolint:gochecknoglobals // one limiter per process, see above

// newScryfallLimiter builds the limiter configuration scryfallLimiter uses.
func newScryfallLimiter() ratelimit.Limiter {
	return ratelimit.New(scryfallRequestsPerSecond, ratelimit.Per(time.Second), ratelimit.WithoutSlack)
}

// newScryfallClient returns a Scryfall client that respects the shared rate limit.
func newScryfallClient() (*scryfall.Client, error) {
	return scryfall.NewClient(scryfall.WithLimiter(scryfallLimiter), scryfall.WithUserAgent(scryfallUserAgent))
}
