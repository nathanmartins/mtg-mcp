package main

// TEMPORARY PROBE — not a numbered production tool.
//
// Registers a tool (`mcp_ui_render_test`) that references a *registered* ui:// HTML
// resource via `_meta.ui.resourceUri` (the spec-correct MCP Apps pattern, where the
// host fetches the resource via resources/read). Its only purpose is to check whether
// claude.ai web renders an MCP Apps UI resource from a remote connector. The HTML shows
// a visible banner plus an inlined data: card image so we can tell apart three outcomes:
// widget+image renders, widget renders but image blocked, or nothing renders.
// Remove this file (and the registerUIRenderTest call in registerResources) once the
// question is answered.

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/BlueMonday/go-scryfall"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	uiRenderTestURI = "ui://render-test"

	uiRenderTestHTMLHead = `<!doctype html><html><head><meta charset="utf-8"><style>` +
		`body{margin:0;font-family:system-ui,sans-serif;background:#0d0d0d;color:#e6e6e6;text-align:center;padding:16px}` +
		`.ok{font-size:20px;font-weight:700;color:#4ade80;margin-bottom:12px}` +
		`img{max-width:100%;height:auto;border-radius:12px}` +
		`</style></head><body><div class="ok">MCP-UI RENDER OK</div>`
	uiRenderTestHTMLTail = `<p>If you see this box, the widget renders (data: image = images allowed).</p>` +
		`<script>(function(){var n=1,done=false;` +
		`function send(m,p){parent.postMessage({jsonrpc:"2.0",id:n++,method:m,params:p||{}},"*");}` +
		`function note(m,p){parent.postMessage({jsonrpc:"2.0",method:m,params:p||{}},"*");}` +
		`function size(){note("ui/notifications/size-changed",` +
		`{width:document.body.scrollWidth,height:document.body.scrollHeight});}` +
		`function ready(){if(done)return;done=true;note("ui/notifications/initialized",{});size();}` +
		`addEventListener("message",function(e){var d=e.data||{};` +
		`if((d.id===1&&d.result)||(typeof d.method==="string"&&d.method.lastIndexOf("ui/",0)===0))ready();});` +
		`send("ui/initialize",{appCapabilities:{},clientInfo:{name:"mtg-mcp",version:"1.0.0"},` +
		`protocolVersion:"2026-01-26"});` +
		`setTimeout(ready,400);setTimeout(size,900);addEventListener("load",size);` +
		`if(window.ResizeObserver){new ResizeObserver(size).observe(document.body);}` +
		`})();</script></body></html>`
)

// registerUIRenderTest wires the temporary UI-render probe (tool + ui:// resource).
func (s *MTGCommanderServer) registerUIRenderTest(mcpServer *server.MCPServer) {
	resource := mcp.NewResource(
		uiRenderTestURI,
		"MTG UI Render Test",
		mcp.WithResourceDescription("Temporary probe: static HTML widget with an inlined card image"),
		mcp.WithMIMEType(mimeMCPAppHTML),
	)
	mcpServer.AddResource(resource, s.handleUIRenderTestResource)

	tool := mcp.NewTool(
		"mcp_ui_render_test",
		mcp.WithDescription("Temporary probe: checks whether claude.ai renders an MCP Apps ui:// UI resource."),
	)
	tool.Meta = mcp.NewMetaFromMap(map[string]any{
		metaKeyUI: map[string]any{metaKeyResourceURI: uiRenderTestURI},
	})
	mcpServer.AddTool(tool, s.handleUIRenderTest)
}

// handleUIRenderTest returns a text result that references the registered ui:// resource
// via _meta.ui.resourceUri, so an MCP Apps host renders that resource as a widget.
func (s *MTGCommanderServer) handleUIRenderTest(
	_ context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	result := mcp.NewToolResultText(
		"UI render test: a widget with a green 'MCP-UI RENDER OK' banner and a card image should appear. " +
			"If you only see this text, the host did not render the ui:// resource.",
	)
	result.Meta = mcp.NewMetaFromMap(map[string]any{
		metaKeyUI: map[string]any{metaKeyResourceURI: uiRenderTestURI},
	})

	return result, nil
}

// handleUIRenderTestResource serves the probe HTML: a visible banner plus, when the
// download succeeds, an inlined data: URI card image (Llanowar Elves, small).
func (s *MTGCommanderServer) handleUIRenderTestResource(
	ctx context.Context,
	_ mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	img := ""

	card, err := s.scryfallClient.GetCardByName(ctx, "Llanowar Elves", false, scryfall.GetCardByNameOptions{})
	if err == nil && card.ImageURIs != nil {
		if url, ok := imageURLForSize(*card.ImageURIs, imageSizeSmall); ok && url != "" {
			if data, dlErr := cardImageBytesFetcher(ctx, url); dlErr == nil {
				encoded := base64.StdEncoding.EncodeToString(data)
				img = fmt.Sprintf(`<img src="data:%s;base64,%s" alt="render test">`,
					imageMIMEType(imageSizeSmall), encoded)
			}
		}
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      uiRenderTestURI,
			MIMEType: mimeMCPAppHTML,
			Text:     uiRenderTestHTMLHead + img + uiRenderTestHTMLTail,
		},
	}, nil
}
