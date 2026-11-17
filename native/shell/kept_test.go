package shell

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/native/components"

	"github.com/egoist/mygo/ui"
)

// keptState is the field with the bookmarks marks kept, the oldest first
// as the library holds them, the pointer on the kept box.
func keptState(t *testing.T, marks ...GroupItem) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := nestedGroupState()
	library.desk = state
	state.ShowField()
	library.groupTree[""] = append(library.groupTree[""], marks...)
	library.rev++
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	box, ok := tester.Find("Kept searches")
	if !ok {
		t.Fatal("There should be a kept box.")
	}
	tester.Move(4, box.Y+box.H/2)
	settle(tester)
	return state, library, tester
}

// The newest bookmark stands on top: each is kept after the others.
func TestBookmarksStandNewestFirst(t *testing.T) {
	_, _, tester := keptState(t,
		GroupItem{ID: "s1", Name: "Oldest", Query: "$tag:a", Color: TintHalo},
		GroupItem{ID: "s2", Name: "Newest", Query: "$tag:b", Color: TintHalo + 1})
	oldest, _ := tester.Find("Oldest")
	newest, _ := tester.Find("Newest")
	if newest.Y >= oldest.Y {
		t.Fatalf("The newest bookmark should stand over the oldest, but it is at %v and the oldest at %v.", newest.Y, oldest.Y)
	}
}

// The query held stands on top of the bookmarks as the one it would be;
// pressed, it is kept, dyed for good in the hue the others wear least.
func TestTheQueryHeldStandsOnTopAsADraftBookmark(t *testing.T) {
	state, library, tester := keptState(t,
		GroupItem{ID: "s1", Name: "Violet", Query: "$tag:a", Color: TintHalo},
		GroupItem{ID: "s2", Name: "Blue", Query: "$tag:b", Color: TintHalo + 1})
	state.SetFieldQuery("$tag:weekly")
	settle(tester)
	draft, ok := tester.Find("Keep this search")
	top, _ := tester.Find("Blue")
	if !ok || draft.Y >= top.Y {
		t.Fatalf("The search held should stand over the bookmarks, but it is at %v (found %v) and the top one at %v.", draft, ok, top)
	}
	if err := tester.Click("Keep this search"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(library.calls, "style 5  3") {
		t.Fatalf("The bookmark kept should be dyed the hue after the newest's, the third: %v.", library.calls)
	}
}

// A bookmark's hue is the one the others wear least, from the hue after
// the newest's on.
func TestABookmarkIsDyedTheHueWornLeast(t *testing.T) {
	for _, c := range []struct {
		marks []int
		want  int
	}{
		{nil, TintHalo},
		{[]int{TintHalo + 2}, TintHalo + 3},
		{[]int{TintHalo + 5}, TintHalo},
		// The newest is Blue, but Teal is worn twice: Amber, after it, is
		// the first worn least.
		{[]int{TintHalo + 1, TintHalo + 2, TintHalo + 2, TintHalo, TintHalo + 3, TintHalo + 4, TintHalo + 5}, TintHalo + 3},
		// One without a hue counts for none.
		{[]int{TintMoonveil}, TintHalo},
	} {
		var marks []GroupItem
		for _, tint := range c.marks {
			marks = append(marks, GroupItem{Color: tint})
		}
		if got := nextTint(marks); got != c.want {
			t.Errorf("After %v the next hue should be %d, but it is %d.", c.marks, c.want, got)
		}
	}
}

// A bookmark's × deletes it for good, as its menu's Delete does.
func TestABookmarkIsDeletedByItsCross(t *testing.T) {
	state, library, tester := keptState(t, GroupItem{ID: "s1", Name: "Mark", Query: "$tag:a", Color: TintHalo})
	mark, _ := tester.Find("Mark")
	tester.Move(mark.X+mark.W/2, mark.Y+mark.H/2)
	settle(tester)
	if err := tester.Click("Delete Mark"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(library.calls, "trash group s1") || !slices.Contains(library.calls, "delete group s1") {
		t.Fatalf("The × should delete the bookmark: %v.", library.calls)
	}
	if q := state.FieldQuery(); q == "$tag:a" {
		t.Fatal("The × should not press the bookmark too.")
	}
}

// Deleting the last bookmark, the slab takes the size of its hint on a
// spring, from its foot, and its icon goes with its top, neither jumping.
func TestTheKeptBoxResizesAsItsLastBookmarkGoes(t *testing.T) {
	state, library, tester := keptState(t, GroupItem{ID: "s1", Name: "Mark", Query: "$tag:a", Color: TintHalo})
	state.Appearance.Motion = true
	settle(tester)
	before, _ := tester.Find("Kept searches")
	icon, _ := tester.Find("Kept searches icon")
	library.groupTree[""] = library.groupTree[""][:len(library.groupTree[""])-1]
	library.rev++
	tester.Frame()
	tester.Advance(components.MotionPanel / 4)
	during, _ := tester.Find("Kept searches")
	iconDuring, _ := tester.Find("Kept searches icon")
	settle(tester)
	after, _ := tester.Find("Kept searches")
	iconAfter, _ := tester.Find("Kept searches icon")
	between := func(a, x, b float32) bool { return min(a, b)+1 < x && x < max(a, b)-1 }
	if !between(before.H, during.H, after.H) {
		t.Fatalf("The slab should resize on its way, but it is %v, then %v, then %v tall.", before.H, during.H, after.H)
	}
	if !between(icon.Y, iconDuring.Y, iconAfter.Y) || abs(iconDuring.Y-icon.Y-(during.Y-before.Y)) > 1 {
		t.Fatalf("The icon should move with the slab's top, but it is at %v, then %v, then %v.", icon.Y, iconDuring.Y, iconAfter.Y)
	}
	if abs(before.Y+before.H-(after.Y+after.H)) > 1 {
		t.Fatalf("The slab's foot should stay, but it went from %v to %v.", before.Y+before.H, after.Y+after.H)
	}
}

// Drawn out, a bookmark stands within the slab at the window's edge, clear
// of the slab's rounded edge there, its tail standing out of the other.
func TestABookmarkStandsWithinTheKeptBox(t *testing.T) {
	_, _, tester := keptState(t, GroupItem{ID: "s1", Name: "Mark", Query: "$tag:a", Color: TintHalo})
	box, _ := tester.Find("Kept searches")
	mark, _ := tester.Find("Mark")
	// The slab's rounded edge stays past the window's: the bookmark stands
	// as far in from the window's edge as the box's inner padding.
	if mark.X < 8 {
		t.Fatalf("The bookmark should stand clear of the window's edge, but it is at %v.", mark.X)
	}
	if mark.X+mark.W <= box.X+box.W-shelfTail {
		t.Fatalf("The bookmark's tail should stand out of the slab, but it ends at %v and the slab at %v.", mark.X+mark.W, box.X+box.W)
	}
}
