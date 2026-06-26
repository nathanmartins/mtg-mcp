package main

import (
	"regexp"
	"strings"
	"time"
)

const (
	rulesPageURL            = "https://magic.wizards.com/en/rules"
	rulesCacheTTL           = 24 * time.Hour
	rulesSearchDefaultLimit = 10
	rulesSearchMaxLimit     = 20
)

// rulesTxtLinkPattern discovers the comprehensive rules .txt URL embedded in the WotC rules page.
// The filename carries a date and a space (e.g. "MagicCompRules 20260619.txt"), so spaces are allowed.
var rulesTxtLinkPattern = regexp.MustCompile(`https?://[^"\s]*MagicCompRules[^"]*?\.txt`) //nolint:gochecknoglobals // compiled regex reused across calls

// ruleNumberPattern matches a rule/subrule number at the start of a line (e.g. "702.19." or "702.19a").
var ruleNumberPattern = regexp.MustCompile(`^(\d+\.\d+[a-z]*)\.?\s`) //nolint:gochecknoglobals // compiled regex reused across calls

// ComprehensiveRules holds the parsed Magic: The Gathering comprehensive rules.
type ComprehensiveRules struct {
	SourceURL string            // the .txt URL the rules were fetched from
	Raw       string            // full original text (served by the rules://comprehensive resource)
	ruleNums  []string          // rule numbers in document order
	rules     map[string]string // rule number -> full text of that rule/subrule
	glossary  map[string]string // lowercased term -> definition
}

// parseComprehensiveRules parses the raw rules .txt into numbered rules and a glossary.
func parseComprehensiveRules(raw string) *ComprehensiveRules {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	cr := &ComprehensiveRules{
		Raw:      raw,
		rules:    map[string]string{},
		glossary: map[string]string{},
	}
	lines := strings.Split(raw, "\n")

	// The word "Glossary" can appear in the table of contents; the real section is the last occurrence.
	glossaryStart := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "Glossary" {
			glossaryStart = i
		}
	}

	parseNumberedRules(cr, lines[:glossaryStart])
	if glossaryStart < len(lines) {
		parseGlossary(cr, lines[glossaryStart+1:])
	}
	return cr
}

// parseNumberedRules groups lines into entries keyed by rule number.
func parseNumberedRules(cr *ComprehensiveRules, lines []string) {
	var curNum string
	var buf []string
	flush := func() {
		if curNum != "" {
			cr.ruleNums = append(cr.ruleNums, curNum)
			cr.rules[curNum] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
		buf = nil
	}
	for _, line := range lines {
		if m := ruleNumberPattern.FindStringSubmatch(line); m != nil {
			flush()
			curNum = m[1]
		}
		if curNum != "" {
			buf = append(buf, line)
		}
	}
	flush()
}

// parseGlossary parses "term\n definition\n\n" blocks into the glossary map.
func parseGlossary(cr *ComprehensiveRules, lines []string) {
	var term string
	var buf []string
	flush := func() {
		if term != "" {
			cr.glossary[strings.ToLower(term)] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
		buf = nil
		term = ""
	}
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			flush()
			continue
		}
		if term == "" {
			term = t
		} else {
			buf = append(buf, line)
		}
	}
	flush()
}
