package mtgserver

import (
	"testing"
	"time"
)

// TestScryfallLimiterSpacesRequests checks the limiter configuration: requests are spaced at
// the two-per-second rate even after idle time, with no banked burst.
func TestScryfallLimiterSpacesRequests(t *testing.T) {
	t.Parallel()
	limiter := newScryfallLimiter()
	limiter.Take()
	time.Sleep(time.Second) // idle time that a slack-enabled limiter would bank as a burst
	start := limiter.Take()
	next := limiter.Take()
	if gap := next.Sub(start); gap < 450*time.Millisecond {
		t.Errorf("consecutive requests %v apart, want about %v", gap, time.Second/scryfallRequestsPerSecond)
	}
}
