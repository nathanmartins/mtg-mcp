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
