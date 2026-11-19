package shell

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A sort is the list's: written, the place stands as a list, and the
// field is not offered one; the list is the place's, kept as it goes and
// comes back, and leaving it takes its sort away.
func TestASortShowsTheList(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	offered := func() bool {
		state.field.open = true
		defer func() { state.field.open = false }()
		return slices.ContainsFunc(state.keyRibbons(), func(p ribbonPlace) bool { return p.key != nil && p.key.Name == "sort" })
	}
	state.ShowField()
	settle(tester)
	if state.listed() || offered() {
		t.Fatal("The field should not stand as a list, nor offer a sort.")
	}
	state.SetFieldQuery("$sort:title")
	settle(tester)
	if !state.listed() || !offered() {
		t.Fatal("A sort written should show the list, which offers its sort.")
	}
	state.toggleList()
	if state.listed() || state.FieldQuery() != "" {
		t.Fatalf("Leaving the list should take its sort away: %q.", state.FieldQuery())
	}
	state.SetFieldQuery("$in:Novel")
	state.toggleList()
	state.SetFieldQuery("")
	if state.listed() {
		t.Fatal("The top should keep its own view.")
	}
	state.SetFieldQuery("$in:Novel")
	if !state.listed() {
		t.Fatal("Novel should come back as its list.")
	}
	state.SetFieldQuery("$in:Novel chapter")
	if state.listed() {
		t.Fatal("A search should start in the field.")
	}
}

// The list divides a place by its groups, their sheets under them; ⌥ and
// the arrows move a sheet along its group, and one dropped among another
// group's sheets is filed there, beside the one it was dropped on.
func TestAListIsArrangedByHand(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 3
	state.toggleList()
	settle(tester)
	ids := shownBlocks(state)
	if !slices.Contains(ids, "group:1") || !slices.Contains(ids, "group:2") || slices.Index(ids, "group:2") > slices.Index(ids, "sheet:7") {
		t.Fatalf("The groups should divide the list, their sheets under them: %v.", ids)
	}
	if !state.field.found.cards[slices.Index(ids, "group:2")].kind.(block).divider {
		t.Fatal("A group in a list should stand as its divider.")
	}
	state.field.chosen = "sheet:7"
	if !state.fieldKey(ui.TextKey{Key: ui.KeyRight, Mods: ui.Alt}) || !slices.Equal(library.moved, []string{"7@1"}) {
		t.Fatalf("⌥→ should move the sheet one on: %v.", library.moved)
	}
	dragOnto(t, tester, "Draft", "Sheet 2")
	if !slices.Contains(library.calls, "move 8 to 2") {
		t.Fatalf("A sheet dropped among another group's should be filed there: %v.", library.calls)
	}
	state.toggleList()
	settle(tester)
	state.field.chosen = "sheet:7"
	if state.fieldKey(ui.TextKey{Key: ui.KeyRight, Mods: ui.Alt}) {
		t.Fatal("The field has no order to arrange.")
	}
}

// The field reads no filed order: groups by when they were made, sheets
// newest made first, however they are filed.
func TestTheFieldReadsNoFiledOrder(t *testing.T) {
	now := time.Now()
	groups := []GroupItem{{ID: "12"}, {ID: "3"}}
	entries := []entry{{sheet: SheetItem{ID: "1", Created: now.Add(-time.Hour)}}, {sheet: SheetItem{ID: "2", Created: now}}}
	g, e := made(groups, entries)
	if g[0].ID != "3" || e[0].sheet.ID != "2" {
		t.Fatalf("made: %v %v", g, e)
	}
	g2, e2 := made([]GroupItem{groups[1], groups[0]}, []entry{entries[1], entries[0]})
	if !slices.Equal(g, g2) || e2[0].sheet.ID != e[0].sheet.ID {
		t.Fatal("Filed otherwise, the field should stand the same.")
	}
}

// The order of itself stands as nothing, as filed or the best match; a
// sort chosen stands as its ribbon.
func TestOnlyAChosenOrderStands(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	for _, label := range []string{"As filed", "Best match"} {
		if _, ok := tester.Find(label); ok {
			t.Fatalf("The order of itself should not stand: %q.", label)
		}
	}
	state.SetFieldQuery("$sort:updated")
	settle(tester)
	if !slices.ContainsFunc(state.ribbons(nil, 1, 630, 400, 40).places, func(p ribbonPlace) bool { return p.text == "$sort:updated" }) {
		t.Fatal("A sort chosen should stand as its ribbon.")
	}
}

// What arrives is brought into view and marked a moment.
func TestWhatArrivesIsShownAndMarked(t *testing.T) {
	state, library, tester := onField(t, 900, 460)
	var groups []GroupItem
	for i := range 24 {
		groups = append(groups, GroupItem{ID: fmt.Sprint("g", i), Name: fmt.Sprint("Group ", i), Count: 1})
	}
	library.groupTree = map[string][]GroupItem{"": groups}
	library.rev++
	settle(tester)
	inView := func(r ui.Rect, ok bool) bool { return ok && r.H > 0 && r.Y >= 0 && r.Y+r.H <= 460 }
	if r, ok := tester.Find("Group 23"); inView(r, ok) {
		t.Fatalf("Group 23 should stand out of view to begin with: %v.", r)
	}
	state.arrive("group:g23")
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	if state.arrivalAt("group:g23", tester.Now()) <= 0 {
		t.Fatal("What arrives should be marked as it is first shown.")
	}
	settle(tester)
	r, ok := tester.Find("Group 23")
	if !inView(r, ok) {
		t.Fatalf("What arrives should be brought into view: %v.", r)
	}
	if _, marked := state.field.arrivals["group:g23"]; marked {
		t.Fatal("The mark should go after its while.")
	}
}

// Anything filed can be pinned: what is pinned stands first, of either
// kind, then the rest, groups before sheets on either side; and a pinned
// sheet wears its badge.
func TestWhatIsPinnedStandsFirst(t *testing.T) {
	b := block{
		groups:  []GroupItem{{ID: "a"}, {ID: "b", Pinned: true}},
		entries: []entry{{sheet: SheetItem{ID: "x"}}, {sheet: SheetItem{ID: "y", Pinned: true}}},
	}
	if got := b.rows(); !slices.Equal(got, []string{"group:b", "sheet:y", "group:a", "sheet:x"}) {
		t.Fatalf("What is pinned should stand first, of either kind: %v.", got)
	}
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 2
	library.Pin("sheet:extra-1", true)
	settle(tester)
	rows := state.groupBlock(GroupItem{ID: "2", Name: "Novel"}, false).rows()
	if !slices.Equal(rows, []string{"group:2", "sheet:extra-1", "sheet:7", "sheet:extra-0"}) {
		t.Fatalf("The pinned sheet should stand first, the rest in their order: %v.", rows)
	}
	if _, ok := tester.Find("Pinned"); !ok {
		t.Fatal("A pinned sheet should wear its badge.")
	}
}
