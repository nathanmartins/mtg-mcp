//go:build e2e

package httpx_test

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/nathanmartins/mtg-mcp/internal/archidekt"
	"github.com/nathanmartins/mtg-mcp/internal/edhrec"
	"github.com/nathanmartins/mtg-mcp/internal/httpx"
	"github.com/nathanmartins/mtg-mcp/internal/moxfield"
)

// TestJSONv2DecodesLikeV1E2E decodes real responses from every deck-site API with both
// encoding/json and encoding/json/v2 (default options) and requires identical results. v2
// matches field names case-sensitively and rejects invalid UTF-8 and duplicate names, so an
// upstream payload v1 accepts could fail or silently lose fields under v2.
func TestJSONv2DecodesLikeV1E2E(t *testing.T) {
	var search moxfield.SearchResponse
	compareDecoders(t, "moxfield search",
		moxfield.DefaultSearchURL+"?pageSize=5&pageNumber=1&cardName=Sol+Ring&fmt=commander", &search)
	if len(search.Data) > 0 {
		compareDecoders(t, "moxfield deck",
			moxfield.DefaultBaseURL+"/decks/all/"+search.Data[0].PublicID, &moxfield.Deck{})
	}

	compareDecoders(t, "archidekt deck", archidekt.DefaultBaseURL+"/decks/18257175/", &archidekt.Deck{})
	compareDecoders(t, "archidekt user decks",
		archidekt.DefaultBaseURL+"/decks/v3/?ownerUsername=nathanmartins&page=1", &archidekt.UserDecksResponse{})
	compareDecoders(t, "archidekt search",
		archidekt.DefaultBaseURL+"/decks/v3/?commanderName=Atraxa%2C+Praetors%27+Voice&deckFormat=3&orderBy=-viewCount",
		&archidekt.UserDecksResponse{})

	compareDecoders(t, "edhrec commander",
		edhrec.DefaultBaseURL+"/commanders/atraxa-praetors-voice.json", &edhrec.Response{})
	compareDecoders(t, "edhrec combos", edhrec.DefaultBaseURL+"/combos/azorius.json", &edhrec.ComboResponse{})
	compareDecoders(t, "edhrec set", edhrec.DefaultBaseURL+"/sets/blb.json", &edhrec.Response{})
}

// compareDecoders fetches rawURL, decodes the body into a fresh *T with each package, and
// fails unless both succeed with equal values. The v1 result is stored in target so callers
// can chain requests.
func compareDecoders[T any](t *testing.T, name, rawURL string, target *T) {
	t.Helper()

	body, err := fetchBody(t, rawURL)
	if err != nil {
		t.Errorf("%s: fetch failed: %v", name, err)
		return
	}

	var fromV1, fromV2 T
	if err = jsonv1.Unmarshal(body, &fromV1); err != nil {
		t.Errorf("%s: v1 decode failed: %v", name, err)
		return
	}
	*target = fromV1
	if err = json.Unmarshal(body, &fromV2); err != nil {
		t.Errorf("%s: v2 decode failed where v1 succeeded: %v", name, err)
		return
	}
	if !reflect.DeepEqual(fromV1, fromV2) {
		t.Errorf("%s: v2 decoded a different value than v1:\n%s", name, firstDifference(t, fromV1, fromV2))
	}
}

func fetchBody(t *testing.T, rawURL string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	resp, err := httpx.GetWithClient(ctx, rawURL, httpx.APIClient())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpx.StatusError{Source: "e2e fetch", StatusCode: resp.StatusCode}
	}
	return io.ReadAll(resp.Body)
}

// firstDifference renders both values as JSON and shows where they first diverge.
func firstDifference(t *testing.T, a, b any) string {
	t.Helper()
	left, err := jsonv1.Marshal(a)
	if err != nil {
		return fmt.Sprintf("cannot render v1 value: %v", err)
	}
	right, err := jsonv1.Marshal(b)
	if err != nil {
		return fmt.Sprintf("cannot render v2 value: %v", err)
	}
	i := 0
	for i < min(len(left), len(right)) && left[i] == right[i] {
		i++
	}
	start := max(0, i-80)
	return fmt.Sprintf("v1: …%s…\nv2: …%s…",
		left[start:min(len(left), i+80)], right[start:min(len(right), i+80)])
}
