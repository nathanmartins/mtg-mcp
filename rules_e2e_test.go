package main

import (
	"context"
	"strings"
	"testing"
)

func TestComprehensiveRulesE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	rules, err := GetComprehensiveRules(context.Background())
	if err != nil {
		t.Fatalf("failed to fetch comprehensive rules: %v", err)
	}

	text, ok := rules.Rule("702.19")
	if !ok || !strings.Contains(text, "Trample") {
		t.Errorf("expected rule 702.19 to mention Trample; ok=%v got=%q", ok, text)
	}
	if _, okGlossary := rules.GlossaryTerm("Trample"); !okGlossary {
		t.Error("expected glossary to contain 'Trample'")
	}
	if !strings.Contains(rules.SourceURL, "MagicCompRules") {
		t.Errorf("unexpected SourceURL: %q", rules.SourceURL)
	}
}
