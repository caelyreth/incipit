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

// The order in force always stands: as filed while browsing, the best
// match while searching, and a sort written in its place.
func TestTheOrderInForceAlwaysStands(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	if _, ok := tester.Find("As filed"); !ok {
		t.Fatalf("Browsing, the order should stand as filed: %v.", tester.Texts())
	}
	state.SetFieldQuery("chapter")
	settle(tester)
	if _, ok := tester.Find("Best match"); !ok {
		t.Fatalf("Searching, the order should be the best match: %v.", tester.Texts())
	}
	state.SetFieldQuery("$sort:updated")
	settle(tester)
	for _, label := range []string{"As filed", "Best match"} {
		if _, ok := tester.Find(label); ok {
			t.Fatalf("A sort written should stand in place of %q.", label)
		}
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

// A pinned sheet stands first in its group, marked; pinning is in its menu.
func TestAPinnedSheetStandsFirst(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 2
	library.PinSheet("extra-1", true)
	settle(tester)
	b := state.groupBlock(GroupItem{ID: "2", Name: "Novel"}, false)
	ids := make([]string, len(b.entries))
	for i, e := range b.entries {
		ids[i] = e.sheet.ID
	}
	if !slices.Equal(ids, []string{"extra-1", "7", "extra-0"}) {
		t.Fatalf("The pinned sheet should stand first, the rest in their order: %v.", ids)
	}
	if _, ok := tester.Find("Pinned"); !ok {
		t.Fatal("A pinned sheet should be marked.")
	}
}
