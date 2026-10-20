// Package deckfmt renders decklists in the Markdown layout every deck tool shares.
package deckfmt

import (
	"fmt"
	"strings"
)

// CardGroups sorts deck cards into type buckets for display. The Moxfield and Archidekt deck
// formatters both use it, so the two render mainboards identically.
type CardGroups struct {
	Creatures     []string
	Instants      []string
	Sorceries     []string
	Artifacts     []string
	Enchantments  []string
	Planeswalkers []string
	Lands         []string
	Others        []string
	TotalCards    int
}

// Add files one card entry under the first matching type in typeLine. The order matters:
// an artifact creature is a creature, and a land creature is a creature too.
func (g *CardGroups) Add(typeLine, name string, quantity int) {
	cardLine := fmt.Sprintf("%dx %s", quantity, name)
	g.TotalCards += quantity

	typeLine = strings.ToLower(typeLine)
	switch {
	case strings.Contains(typeLine, "creature"):
		g.Creatures = append(g.Creatures, cardLine)
	case strings.Contains(typeLine, "instant"):
		g.Instants = append(g.Instants, cardLine)
	case strings.Contains(typeLine, "sorcery"):
		g.Sorceries = append(g.Sorceries, cardLine)
	case strings.Contains(typeLine, "artifact"):
		g.Artifacts = append(g.Artifacts, cardLine)
	case strings.Contains(typeLine, "enchantment"):
		g.Enchantments = append(g.Enchantments, cardLine)
	case strings.Contains(typeLine, "planeswalker"):
		g.Planeswalkers = append(g.Planeswalkers, cardLine)
	case strings.Contains(typeLine, "land"):
		g.Lands = append(g.Lands, cardLine)
	default:
		g.Others = append(g.Others, cardLine)
	}
}

// Render writes every non-empty group, in the fixed order the deck output uses.
func (g *CardGroups) Render(output *strings.Builder) {
	output.WriteString(formatGroup("Creatures", g.Creatures))
	output.WriteString(formatGroup("Instants", g.Instants))
	output.WriteString(formatGroup("Sorceries", g.Sorceries))
	output.WriteString(formatGroup("Artifacts", g.Artifacts))
	output.WriteString(formatGroup("Enchantments", g.Enchantments))
	output.WriteString(formatGroup("Planeswalkers", g.Planeswalkers))
	output.WriteString(formatGroup("Lands", g.Lands))
	output.WriteString(formatGroup("Other", g.Others))
}

// formatGroup formats a category of cards with a title.
func formatGroup(title string, cards []string) string {
	if len(cards) == 0 {
		return ""
	}

	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "**%s (%d):**\n", title, len(cards))
	for _, c := range cards {
		_, _ = fmt.Fprintf(&output, "- %s\n", c)
	}
	output.WriteString("\n")

	return output.String()
}
