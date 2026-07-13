package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	scryfall "github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
)

// stubImageFetcher replaces the download seam with one that echoes the URL back
// as fake image bytes, restoring the original when the test ends.
func stubImageFetcher(t *testing.T) {
	t.Helper()

	prev := cardImageBytesFetcher
	cardImageBytesFetcher = func(_ context.Context, rawURL string) ([]byte, error) {
		return []byte("IMG:" + rawURL), nil
	}
	t.Cleanup(func() { cardImageBytesFetcher = prev })
}

// embeddedHTML returns the text/html EmbeddedResource carried by a tool result.
func embeddedHTML(t *testing.T, res *mcp.CallToolResult) mcp.TextResourceContents {
	t.Helper()

	for _, c := range res.Content {
		if er, ok := mcp.AsEmbeddedResource(c); ok {
			if trc, isText := er.Resource.(mcp.TextResourceContents); isText {
				return trc
			}
		}
	}
	t.Fatal("result has no text embedded resource")

	return mcp.TextResourceContents{}
}

// uiResourceURI returns _meta.ui.resourceUri from a tool result.
func uiResourceURI(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()

	if res.Meta == nil {
		t.Fatal("result has no _meta")
	}
	ui, ok := res.Meta.AdditionalFields["ui"].(map[string]any)
	if !ok {
		t.Fatal("_meta.ui missing")
	}
	uri, _ := ui["resourceUri"].(string)

	return uri
}

// assertContainsAll fails the test for each wanted substring missing from text.
func assertContainsAll(t *testing.T, text string, want ...string) {
	t.Helper()

	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("text missing %q\n%s", w, text)
		}
	}
}

const (
	solRingImageJSON = `{
		"id":"sol","name":"Sol Ring","lang":"en",
		"image_uris":{"small":"https://img.test/small.jpg","normal":"https://img.test/en-normal.jpg",
			"large":"https://img.test/large.jpg","png":"https://img.test/en.png",
			"art_crop":"https://img.test/art.jpg","border_crop":"https://img.test/border.jpg"},
		"legalities":{"commander":"legal"}
	}`

	solRingItalianListJSON = `{"object":"list","total_cards":1,"has_more":false,"data":[{
		"id":"sol-it","name":"Sol Ring","lang":"it","printed_name":"Anello Solare",
		"image_uris":{"normal":"https://img.test/it-normal.jpg","png":"https://img.test/it.png"},
		"legalities":{"commander":"legal"}
	}]}`

	delverImageJSON = `{
		"id":"delver","name":"Delver of Secrets // Insectile Aberration","lang":"en",
		"card_faces":[
			{"name":"Delver of Secrets","image_uris":{"normal":"https://img.test/front.jpg"}},
			{"name":"Insectile Aberration","image_uris":{"normal":"https://img.test/back.jpg"}}
		],
		"legalities":{"commander":"legal"}
	}`
)

func TestHandleGetCardImageValidation(t *testing.T) {
	t.Run("missing name returns error", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {
			t.Error("scryfall should not be called when name is missing")
		})
		res, err := s.handleGetCardImage(context.Background(), toolRequest(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Error("expected error result for missing name")
		}
	})

	t.Run("invalid size returns error", func(t *testing.T) {
		s := newTestScryfallServer(t, func(http.ResponseWriter, *http.Request) {
			t.Error("scryfall should not be called when size is invalid")
		})
		res, _ := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{
			"name": "Sol Ring",
			"size": "gigantic",
		}))
		if !res.IsError {
			t.Error("expected error result for invalid size")
		}
	})

	t.Run("card not found returns error", func(t *testing.T) {
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			scryfallError(w, http.StatusNotFound)
		})
		res, _ := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{"name": "nope"}))
		if !res.IsError {
			t.Error("expected error result for missing card")
		}
	})

	t.Run("download failure returns error", func(t *testing.T) {
		prev := cardImageBytesFetcher
		cardImageBytesFetcher = func(context.Context, string) ([]byte, error) {
			return nil, http.ErrHandlerTimeout
		}
		t.Cleanup(func() { cardImageBytesFetcher = prev })

		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, solRingImageJSON)
		})
		res, _ := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{"name": "Sol Ring"}))
		if !res.IsError {
			t.Error("expected error result when the image download fails")
		}
	})
}

func TestHandleGetCardImageRendering(t *testing.T) {
	t.Run("english single-faced card", func(t *testing.T) {
		stubImageFetcher(t)
		searched := false
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/cards/search") {
				searched = true
			}
			jsonResponse(w, solRingImageJSON)
		})

		res, err := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{"name": "Sol Ring"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("unexpected error result: %s", resultText(t, res))
		}
		if searched {
			t.Error("English lookup should not hit the search endpoint")
		}

		assertContainsAll(t, resultText(t, res),
			"# Sol Ring", "**Language:** en", "**Size:** normal",
			"view on Scryfall](https://img.test/en-normal.jpg)")

		er := embeddedHTML(t, res)
		if er.MIMEType != "text/html" {
			t.Errorf("resource mime = %q, want text/html", er.MIMEType)
		}
		enData := base64.StdEncoding.EncodeToString([]byte("IMG:https://img.test/en-normal.jpg"))
		assertContainsAll(t, er.Text, "<img ", "data:image/jpeg;base64,"+enData)
		if got := uiResourceURI(t, res); got != er.URI {
			t.Errorf("_meta.ui.resourceUri %q != embedded resource URI %q", got, er.URI)
		}
	})

	t.Run("size parameter selects png and mime", func(t *testing.T) {
		stubImageFetcher(t)
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, solRingImageJSON)
		})
		res, _ := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{
			"name": "Sol Ring",
			"size": "png",
		}))
		pngData := base64.StdEncoding.EncodeToString([]byte("IMG:https://img.test/en.png"))
		assertContainsAll(t, embeddedHTML(t, res).Text, "data:image/png;base64,"+pngData)
		if !strings.Contains(resultText(t, res), "view on Scryfall](https://img.test/en.png)") {
			t.Errorf("expected Scryfall link in text:\n%s", resultText(t, res))
		}
	})

	t.Run("double-faced card yields one image per face", func(t *testing.T) {
		stubImageFetcher(t)
		s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, delverImageJSON)
		})

		res, err := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{"name": "Delver of Secrets"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		front := base64.StdEncoding.EncodeToString([]byte("IMG:https://img.test/front.jpg"))
		back := base64.StdEncoding.EncodeToString([]byte("IMG:https://img.test/back.jpg"))
		assertContainsAll(t, embeddedHTML(t, res).Text,
			"data:image/jpeg;base64,"+front,
			"data:image/jpeg;base64,"+back)
	})
}

func TestHandleGetCardImageLanguage(t *testing.T) {
	t.Run("localized italian printing", func(t *testing.T) {
		stubImageFetcher(t)
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/cards/named"):
				jsonResponse(w, solRingImageJSON)
			case strings.HasPrefix(r.URL.Path, "/cards/search"):
				jsonResponse(w, solRingItalianListJSON)
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
			}
		})

		res, err := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{
			"name":     "Sol Ring",
			"language": "it",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := resultText(t, res)
		assertContainsAll(t, text, "**Language:** it", "https://img.test/it-normal.jpg")
		if strings.Contains(text, "showing English") {
			t.Errorf("should not report a fallback when the localized printing exists:\n%s", text)
		}
	})

	t.Run("falls back to english when no localized printing", func(t *testing.T) {
		stubImageFetcher(t)
		s := newTestScryfallServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/cards/search") {
				scryfallError(w, http.StatusNotFound)
				return
			}
			jsonResponse(w, solRingImageJSON)
		})

		res, err := s.handleGetCardImage(context.Background(), toolRequest(map[string]any{
			"name":     "Sol Ring",
			"language": "it",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertContainsAll(t, resultText(t, res),
			"No 'it' printing found; showing English.", "**Language:** en", "https://img.test/en-normal.jpg")
	})
}

func TestImageURLForSize(t *testing.T) {
	iu := scryfall.ImageURIs{
		Small:      "s",
		Normal:     "n",
		Large:      "l",
		PNG:        "p",
		ArtCrop:    "a",
		BorderCrop: "b",
	}
	cases := []struct {
		size string
		want string
		ok   bool
	}{
		{imageSizeSmall, "s", true},
		{imageSizeNormal, "n", true},
		{imageSizeLarge, "l", true},
		{imageSizePNG, "p", true},
		{imageSizeArtCrop, "a", true},
		{imageSizeBorderCrop, "b", true},
		{"unknown", "", false},
	}
	for _, c := range cases {
		got, ok := imageURLForSize(iu, c.size)
		if got != c.want || ok != c.ok {
			t.Errorf("imageURLForSize(%q) = (%q, %t), want (%q, %t)", c.size, got, ok, c.want, c.ok)
		}
	}
}

func TestGetImageBytes(t *testing.T) {
	get := func(ctx context.Context, rawURL string) (*http.Response, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		return http.DefaultClient.Do(req)
	}

	t.Run("downloads body", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("BYTES"))
		}))
		defer ts.Close()

		data, err := getImageBytes(context.Background(), ts.URL, get)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "BYTES" {
			t.Errorf("data = %q, want BYTES", data)
		}
	})

	t.Run("non-200 status is an error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		if _, err := getImageBytes(context.Background(), ts.URL, get); err == nil {
			t.Error("expected error for non-200 status")
		}
	})

	t.Run("getter error propagates", func(t *testing.T) {
		getErr := func(context.Context, string) (*http.Response, error) {
			return nil, http.ErrHandlerTimeout
		}
		if _, err := getImageBytes(context.Background(), "x", getErr); err == nil {
			t.Error("expected getter error to propagate")
		}
	})
}
