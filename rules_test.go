package main

import (
	"strings"
	"testing"
)

const sampleRulesTxt = `Magic: The Gathering Comprehensive Rules

These rules are effective as of June 19, 2026.

100. General

100.1. These Magic rules apply to any Magic game with two or more players.

702.19. Trample

702.19a Trample is a static ability that modifies the rules for assigning combat damage.

702.19b The controller of an attacking creature with trample first assigns damage to the creatures blocking it.

Glossary

Trample
A keyword ability that allows excess combat damage to be assigned to the defending player. See rule 702.19.

Deathtouch
A keyword ability that causes damage dealt by an object to be lethal. See rule 702.2.
`

func TestParseComprehensiveRules(t *testing.T) {
	cr := parseComprehensiveRules(sampleRulesTxt)

	for _, num := range []string{"100.1", "702.19", "702.19a", "702.19b"} {
		if _, ok := cr.rules[num]; !ok {
			t.Errorf("expected rule %q to be parsed", num)
		}
	}
	if !strings.Contains(cr.rules["702.19a"], "static ability") {
		t.Errorf("rule 702.19a text wrong: %q", cr.rules["702.19a"])
	}
	if _, ok := cr.glossary["trample"]; !ok {
		t.Error("expected glossary term 'trample'")
	}
	if _, ok := cr.glossary["deathtouch"]; !ok {
		t.Error("expected glossary term 'deathtouch'")
	}
	if cr.Raw != strings.ReplaceAll(sampleRulesTxt, "\r\n", "\n") {
		t.Error("Raw should hold the full normalized text")
	}
}

func TestComprehensiveRulesLookups(t *testing.T) {
	cr := parseComprehensiveRules(sampleRulesTxt)

	// Rule with subrules
	got, ok := cr.Rule("702.19")
	if !ok {
		t.Fatal("expected rule 702.19")
	}
	if !strings.Contains(got, "Trample") || !strings.Contains(got, "static ability") ||
		!strings.Contains(got, "first assigns damage") {
		t.Errorf("Rule(702.19) should include parent + subrules; got:\n%s", got)
	}

	// Subrule alone
	sub, ok := cr.Rule("702.19a")
	if !ok || strings.Contains(sub, "first assigns damage") {
		t.Errorf("Rule(702.19a) should return only that subrule; got:\n%s", sub)
	}

	// Unknown rule
	if _, ok := cr.Rule("999.99"); ok {
		t.Error("unknown rule should return ok=false")
	}

	// Search
	matches := cr.Search("trample", 0)
	if len(matches) == 0 || matches[0].Number != "702.19" {
		t.Errorf("Search(trample) first match should be 702.19; got %+v", matches)
	}
	if len(cr.Search("trample", 1)) != 1 {
		t.Error("Search limit should cap results")
	}

	// Glossary (case-insensitive)
	def, ok := cr.GlossaryTerm("Trample")
	if !ok || !strings.Contains(def, "keyword ability") {
		t.Errorf("GlossaryTerm(Trample) wrong; ok=%v def=%q", ok, def)
	}
	if _, ok := cr.GlossaryTerm("nope"); ok {
		t.Error("unknown glossary term should return ok=false")
	}
}
