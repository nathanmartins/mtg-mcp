package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// moxfieldVerifyMaxChecks bounds how many decks one tool call may fetch while
	// verifying commanders. Moxfield answers HTTP 429 after roughly a dozen rapid
	// requests, so this is a politeness cap as much as a latency cap.
	moxfieldVerifyMaxChecks = 20
	// moxfieldVerifyDelay spaces verification requests.
	moxfieldVerifyDelay = 400 * time.Millisecond
	// moxfieldVerifyBudget bounds the total time spent verifying one page.
	moxfieldVerifyBudget = 30 * time.Second
	// moxfieldCandidateFactor decides how many search candidates to request per
	// wanted deck, since the search filter matches any deck containing the card.
	moxfieldCandidateFactor = 5
)

// MoxfieldCommanderSearch is the outcome of a commander search. Moxfield's search API
// ignores every commander parameter, so Decks holds only candidates whose commander
// zone was actually fetched and matched. Incomplete marks a truncated verification —
// the caller must not present the list as exhaustive.
type MoxfieldCommanderSearch struct {
	Decks      []MoxfieldDeckSummary
	Candidates int
	Checked    int
	Incomplete bool
	Reason     string
}

// deckHasCommander reports whether the deck's commander zone contains the named card.
// A partner or background pairing matches when either commander matches.
func deckHasCommander(deck *MoxfieldDeck, commander string) bool {
	want := strings.ToLower(strings.TrimSpace(commander))
	for name, entry := range deck.Commanders {
		if strings.ToLower(strings.TrimSpace(name)) == want {
			return true
		}
		if strings.ToLower(strings.TrimSpace(entry.Card.Name)) == want {
			return true
		}
	}
	return false
}

// verifyMoxfieldCommanderDecks keeps the candidates whose commander is the requested
// card, fetching each deck in sequence. It stops at limit matches, at maxChecks
// fetches, when ctx expires, or at the first HTTP 429, reporting why.
func verifyMoxfieldCommanderDecks(
	ctx context.Context,
	candidates []MoxfieldDeckSummary,
	commander string,
	deckBaseURL string,
	limit int,
	maxChecks int,
	delay time.Duration,
) MoxfieldCommanderSearch {
	outcome := MoxfieldCommanderSearch{Candidates: len(candidates)}

	for i, candidate := range candidates {
		if len(outcome.Decks) >= limit {
			break
		}
		if outcome.Checked >= maxChecks {
			outcome.Incomplete = true
			outcome.Reason = fmt.Sprintf("verification budget reached after %d decks", outcome.Checked)
			break
		}
		if i > 0 && delay > 0 {
			select {
			case <-ctx.Done():
				outcome.Incomplete = true
				outcome.Reason = "verification stopped: " + ctx.Err().Error()
				return outcome
			case <-time.After(delay):
			}
		}

		deck, err := getMoxfieldDeckWithURL(ctx, candidate.PublicID, deckBaseURL)
		outcome.Checked++
		if err != nil {
			if isMoxfieldRateLimited(err) {
				outcome.Incomplete = true
				outcome.Reason = "verification stopped: Moxfield rate limit (HTTP 429)"
				break
			}
			GetLogger().Warn().
				Err(err).
				Str("deck_id", candidate.PublicID).
				Msg("Skipping deck that could not be verified")
			continue
		}

		if deckHasCommander(deck, commander) {
			outcome.Decks = append(outcome.Decks, candidate)
		}
	}

	return outcome
}
