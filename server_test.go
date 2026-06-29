package main

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestNewMTGCommanderServer(t *testing.T) {
	s, err := NewMTGCommanderServer()
	if err != nil {
		t.Fatalf("NewMTGCommanderServer() error = %v", err)
	}
	if s == nil || s.scryfallClient == nil {
		t.Fatal("expected a server with an initialized Scryfall client")
	}
}

// TestRegisterToolsAndResources exercises the registration wiring. It must not
// panic and should register every tool and resource without error.
func TestRegisterToolsAndResources(t *testing.T) {
	s, err := NewMTGCommanderServer()
	if err != nil {
		t.Fatalf("NewMTGCommanderServer() error = %v", err)
	}

	mcpServer := server.NewMCPServer("test", "0.0.0", server.WithRecovery())

	// registerTools also calls registerArchidektTools internally.
	s.registerTools(mcpServer)
	s.registerResources(mcpServer)
}
