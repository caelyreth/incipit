package shell

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A sorted field is read in rows, left to right; a ranked one stands by
// nearness to the pill.
func TestASortedFieldIsLaidOutInReadingOrder(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	var groups []GroupItem
	for i := range 6 {
		groups = append(groups, GroupItem{ID: fmt.Sprint("g", i), Name: fmt.Sprint("Group ", i), Count: 1})
	}
	library.groupTree = map[string][]GroupItem{"": groups}
	library.rev++
	settle(tester)
	if state.sortedField() {
		t.Fatal("Browsing as filed, the field should not be read in rows.")
	}
	state.SetFieldQuery("$sort:title")
	if !state.sortedField() {
		t.Fatal("A sort written should be read in rows.")
	}
	state.SetFieldQuery("$sort:relevance")
	if state.sortedField() {
		t.Fatal("The best match first is ranked, not read.")
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
