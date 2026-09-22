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
// zone was actually fetched and matched. Failed counts candidates whose deck could not
// be read at all — those are neither matches nor non-matches, merely unknown.
// Incomplete marks a truncated or partially blind verification, and is always paired
// with a Reason; the caller must not present the list as exhaustive.
// Unexamined counts the candidates left untouched because the requested number of
// matches was already found. That is an ordinary success rather than an incomplete
// verification, but the next page resumes after the whole candidate page, so those
// candidates are skipped for good and the count has to be reported.
type MoxfieldCommanderSearch struct {
	Decks      []MoxfieldDeckSummary
	Candidates int
	Checked    int
	Failed     int
	Unexamined int
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
// fetches, when ctx expires, or at the first HTTP 429, reporting why. Candidates whose
// deck read fails are counted in Failed and make the outcome incomplete, because a deck
// that could not be read cannot be ruled out either. Candidates never reached because
// limit matches were already found are counted in Unexamined instead: the run
// succeeded, but those decks are unreachable by paging.
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
	stopReason := ""

	for i, candidate := range candidates {
		if len(outcome.Decks) >= limit {
			outcome.Unexamined = len(candidates) - i
			break
		}
		if outcome.Checked >= maxChecks {
			stopReason = fmt.Sprintf("verification budget reached after %d decks", outcome.Checked)
			break
		}
		waitBeforeCheck(ctx, i, delay)

		deck, err := getMoxfieldDeckWithURL(ctx, candidate.PublicID, deckBaseURL)
		if err != nil {
			// An expired budget and a rate limit both end verification, and neither says
			// anything about this candidate, so they are ruled out before the failed read
			// is recorded as a check or blamed on the deck.
			// Untested: the window between a successful read and this guard cannot be
			// forced through the http.Client seam without a flaky timing hack.
			if stop := verificationStopReason(ctx, err); stop != "" {
				stopReason = stop
				break
			}
			outcome.Checked++
			outcome.Failed++
			GetLogger().Warn().
				Err(err).
				Str("deck_id", candidate.PublicID).
				Msg("Skipping deck that could not be verified")
			continue
		}

		outcome.Checked++
		if deckHasCommander(deck, commander) {
			outcome.Decks = append(outcome.Decks, candidate)
		}
	}

	outcome.Reason = verificationReason(stopReason, outcome.Failed)
	outcome.Incomplete = outcome.Reason != ""

	return outcome
}

// waitBeforeCheck spaces verification requests, abandoning the pause as soon as the
// context expires so a dead budget is not spent sleeping.
func waitBeforeCheck(ctx context.Context, index int, delay time.Duration) {
	if index == 0 || delay <= 0 {
		return
	}

	select {
	case <-ctx.Done():
	case <-time.After(delay):
	}
}

// verificationStopReason reports why the remaining candidates must be abandoned. It is
// consulted only for a failed read: an expired verification budget surfaces there as an
// ordinary fetch error, so a deck that was fully fetched before the budget expired is
// never blamed on a cancelled context. Cancellation itself still always stops the loop —
// the next iteration's read fails immediately with the context error — and
// waitBeforeCheck already abandons its pause on ctx.Done().
func verificationStopReason(ctx context.Context, err error) string {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "verification stopped: " + ctxErr.Error()
	}
	if isMoxfieldRateLimited(err) {
		return "verification stopped: Moxfield rate limit (HTTP 429)"
	}

	return ""
}

// verificationReason combines the reason the loop stopped early with the count of decks
// that could not be read. Either makes the result non-exhaustive, so an empty reason is
// the only signal that the deck list is the complete answer for this page.
func verificationReason(stopReason string, failed int) string {
	unfetchable := ""
	if failed > 0 {
		unfetchable = fmt.Sprintf("%d candidate deck(s) could not be fetched", failed)
	}

	switch {
	case stopReason != "" && unfetchable != "":
		return stopReason + "; " + unfetchable
	case stopReason != "":
		return stopReason
	default:
		return unfetchable
	}
}
