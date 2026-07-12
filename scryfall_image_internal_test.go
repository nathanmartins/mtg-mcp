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

// imageContents extracts the ImageContent blocks from a tool result.
func imageContents(t *testing.T, res *mcp.CallToolResult) []mcp.ImageContent {
	t.Helper()

	var imgs []mcp.ImageContent
	for _, c := range res.Content {
		if ic, ok := mcp.AsImageContent(c); ok {
			imgs = append(imgs, *ic)
		}
	}

	return imgs
}

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
			"# Sol Ring", "**Language:** en", "**Size:** normal", "https://img.test/en-normal.jpg")

		imgs := imageContents(t, res)
		if len(imgs) != 1 {
			t.Fatalf("expected 1 image, got %d", len(imgs))
		}
		if imgs[0].MIMEType != mimeJPEG {
			t.Errorf("mime = %q, want %q", imgs[0].MIMEType, mimeJPEG)
		}
		decoded, _ := base64.StdEncoding.DecodeString(imgs[0].Data)
		if !strings.Contains(string(decoded), "https://img.test/en-normal.jpg") {
			t.Errorf("image bytes should echo the normal URL, got %q", decoded)
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
		imgs := imageContents(t, res)
		if len(imgs) != 1 || imgs[0].MIMEType != mimePNG {
			t.Fatalf("expected 1 png image, got %+v", imgs)
		}
		if !strings.Contains(resultText(t, res), "https://img.test/en.png") {
			t.Errorf("expected png URL in text:\n%s", resultText(t, res))
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
		assertContainsAll(t, resultText(t, res),
			"Delver of Secrets:** https://img.test/front.jpg",
			"Insectile Aberration:** https://img.test/back.jpg")
		if imgs := imageContents(t, res); len(imgs) != 2 {
			t.Fatalf("expected 2 images for a double-faced card, got %d", len(imgs))
		}
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
