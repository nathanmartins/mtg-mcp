package mtgserver

import (
	"testing"
	"testing/synctest"
	"time"
)

// TestScryfallLimiterSpacesRequests checks the limiter configuration: requests are spaced at
// the two-per-second rate even after idle time, with no banked burst. It runs in a synctest
// bubble, so the sleeps advance a fake clock instead of taking real time, and the spacing is
// exact rather than approximate.
func TestScryfallLimiterSpacesRequests(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		limiter := newScryfallLimiter()
		limiter.Take()
		time.Sleep(time.Second) // idle time that a slack-enabled limiter would bank as a burst
		start := limiter.Take()
		next := limiter.Take()
		if gap, want := next.Sub(start), time.Second/scryfallRequestsPerSecond; gap != want {
			t.Errorf("consecutive requests %v apart, want exactly %v", gap, want)
		}
	})
}
