package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleUIRenderTest(t *testing.T) {
	s := &MTGCommanderServer{}

	res, err := s.handleUIRenderTest(context.Background(), toolRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := uiResourceURI(t, res); got != uiRenderTestURI {
		t.Errorf("_meta.ui.resourceUri = %q, want %q", got, uiRenderTestURI)
	}
}

func TestHandleUIRenderTestResource(t *testing.T) {
	stubImageFetcher(t)
	s := newTestScryfallServer(t, func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, solRingImageJSON)
	})

	contents, err := s.handleUIRenderTestResource(context.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 resource content, got %d", len(contents))
	}

	trc, ok := contents[0].(*mcp.TextResourceContents)
	if !ok {
		t.Fatalf("content is not *TextResourceContents: %T", contents[0])
	}
	if trc.MIMEType != mimeMCPAppHTML {
		t.Errorf("mime = %q, want %q", trc.MIMEType, mimeMCPAppHTML)
	}
	assertContainsAll(t, trc.Text,
		"MCP-UI RENDER OK", "<img ", "data:image/jpeg;base64,",
		"ui/initialize", "ui/notifications/initialized", "ui/notifications/size-changed")
}
