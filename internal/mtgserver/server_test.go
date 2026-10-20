package mtgserver

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanmartins/mtg-mcp/internal/moxfield"
)

func TestNewMTGCommanderServer(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s == nil || s.scryfallClient == nil {
		t.Fatal("expected a server with an initialized Scryfall client")
	}
}

// TestNewMTGCommanderServerMoxfieldWiring pins the production Moxfield settings: the search
// URL is a different host from deck reads, and verification requests must be spaced.
func TestNewMTGCommanderServerMoxfieldWiring(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s.moxfieldSearchURL != moxfield.DefaultSearchURL {
		t.Errorf("moxfieldSearchURL = %q, want %q", s.moxfieldSearchURL, moxfield.DefaultSearchURL)
	}
	if s.verifyDelay != moxfield.VerifyDelay {
		t.Errorf("verifyDelay = %v, want %v — production must space verification requests",
			s.verifyDelay, moxfield.VerifyDelay)
	}
}

// TestRegisterToolsAndResources exercises the registration wiring. It must not
// panic and should register every tool and resource without error.
func TestRegisterToolsAndResources(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	mcpServer := server.NewMCPServer("test", "0.0.0", server.WithRecovery())

	// registerTools also calls registerArchidektTools internally.
	s.registerTools(mcpServer)
	s.registerResources(mcpServer)
}
