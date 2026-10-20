package deckfmt

import (
	"slices"
	"strings"
	"testing"
)

func TestCardGroupsAdd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		typeLine string
		group    func(*CardGroups) []string
	}{
		{typeLine: "Creature — Elf Druid", group: func(g *CardGroups) []string { return g.Creatures }},
		// Type precedence: multi-typed cards land in the first matching bucket.
		{typeLine: "Artifact Creature — Golem", group: func(g *CardGroups) []string { return g.Creatures }},
		{typeLine: "Land Creature — Forest Dryad", group: func(g *CardGroups) []string { return g.Creatures }},
		{typeLine: "Tribal Instant — Faerie", group: func(g *CardGroups) []string { return g.Instants }},
		{typeLine: "Sorcery", group: func(g *CardGroups) []string { return g.Sorceries }},
		{typeLine: "Artifact Land", group: func(g *CardGroups) []string { return g.Artifacts }},
		{typeLine: "Enchantment — Aura", group: func(g *CardGroups) []string { return g.Enchantments }},
		{typeLine: "Legendary Planeswalker — Nissa", group: func(g *CardGroups) []string { return g.Planeswalkers }},
		{typeLine: "Basic Land — Island", group: func(g *CardGroups) []string { return g.Lands }},
		{typeLine: "CREATURE", group: func(g *CardGroups) []string { return g.Creatures }},
		{typeLine: "Battle — Siege", group: func(g *CardGroups) []string { return g.Others }},
		{typeLine: "", group: func(g *CardGroups) []string { return g.Others }},
	}
	for _, tt := range tests {
		var g CardGroups
		g.Add(tt.typeLine, "Card", 2)
		if got := tt.group(&g); !slices.Equal(got, []string{"2x Card"}) {
			t.Errorf("Add(%q) put the card in the wrong group: %+v", tt.typeLine, g)
		}
	}
}

func TestCardGroupsTotalCountsQuantities(t *testing.T) {
	t.Parallel()
	var g CardGroups
	g.Add("Basic Land — Island", "Island", 30)
	g.Add("Artifact", "Sol Ring", 1)
	if g.TotalCards != 31 {
		t.Errorf("TotalCards = %d, want 31", g.TotalCards)
	}
}

func TestCardGroupsRender(t *testing.T) {
	t.Parallel()
	var g CardGroups
	g.Add("Basic Land — Island", "Island", 9)
	g.Add("Creature — Bird", "Birds of Paradise", 1)
	g.Add("Token", "Treasure", 1)
	g.Add("Creature — Elf", "Llanowar Elves", 1)

	var output strings.Builder
	g.Render(&output)

	// Groups appear in fixed order, empty ones are omitted, and each header counts
	// entries (not copies), keeping insertion order within a group.
	want := "**Creatures (2):**\n- 1x Birds of Paradise\n- 1x Llanowar Elves\n\n" +
		"**Lands (1):**\n- 9x Island\n\n" +
		"**Other (1):**\n- 1x Treasure\n\n"
	if output.String() != want {
		t.Errorf("Render() =\n%q\nwant\n%q", output.String(), want)
	}
}

func TestCardGroupsRenderEmpty(t *testing.T) {
	t.Parallel()
	var g CardGroups
	var output strings.Builder
	g.Render(&output)
	if output.Len() != 0 {
		t.Errorf("Render() of empty groups wrote %q", output.String())
	}
}
