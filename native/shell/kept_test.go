package shell

import (
	"fmt"
	"slices"
	"testing"
	"time"

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
		GroupItem{ID: "s1", Name: "Halo", Query: "$tag:a", Color: TintHalo},
		GroupItem{ID: "s2", Name: "Afterglow", Query: "$tag:b", Color: TintHalo + 1})
	state.SetFieldQuery("$tag:weekly")
	settle(tester)
	draft, ok := tester.Find("Keep This Search")
	top, _ := tester.Find("Afterglow")
	if !ok || draft.Y >= top.Y {
		t.Fatalf("The search held should stand over the bookmarks, but it is at %v (found %v) and the top one at %v.", draft, ok, top)
	}
	if err := tester.Click("Keep This Search"); err != nil {
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
		// The newest is Afterglow, but Remnant is worn twice: Unripe,
		// after it, is the first worn least.
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
	icon, _ := tester.Find("Kept")
	library.groupTree[""] = library.groupTree[""][:len(library.groupTree[""])-1]
	library.rev++
	tester.Frame()
	tester.Advance(components.MotionPanel / 4)
	during, _ := tester.Find("Kept searches")
	iconDuring, _ := tester.Find("Kept")
	settle(tester)
	after, _ := tester.Find("Kept searches")
	iconAfter, _ := tester.Find("Kept")
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

// A bookmark kept stands on top, the box growing upward from its foot:
// those kept before stay where they stood, frame by frame, never jumping
// up as the new one comes in.
func TestBookmarksKeptBeforeStayAsOneIsKept(t *testing.T) {
	state, library, tester := keptState(t, GroupItem{ID: "s1", Name: "Mark", Query: "$tag:a", Color: TintHalo})
	state.Appearance.Motion = true
	settle(tester)
	before, _ := tester.Find("Mark")
	library.groupTree[""] = append(library.groupTree[""], GroupItem{ID: "s2", Name: "Newer", Query: "$tag:b", Color: TintHalo + 1})
	library.rev++
	for range 40 {
		tester.Frame()
		tester.Advance(16 * time.Millisecond)
		if now, _ := tester.Find("Mark"); abs(now.Y-before.Y) > 0.5 {
			t.Fatalf("The bookmark kept before should stay at %v, but it went to %v.", before.Y, now.Y)
		}
	}
	newer, _ := tester.Find("Newer")
	if newer.Y >= before.Y {
		t.Fatalf("The newer bookmark should stand over the older, at %v, not %v.", before.Y, newer.Y)
	}
}

// A bookmark deleted from among the others folds shut where it stood:
// those over it come down as it does, frame by frame, never at once, and
// those under it stay; shut, it is gone.
func TestABookmarkDeletedFromTheMiddleFoldsShut(t *testing.T) {
	state, library, tester := keptState(t,
		GroupItem{ID: "s1", Name: "Oldest", Query: "$tag:a", Color: TintHalo},
		GroupItem{ID: "s2", Name: "Middle", Query: "$tag:b", Color: TintHalo + 1},
		GroupItem{ID: "s3", Name: "Newest", Query: "$tag:c", Color: TintHalo + 2})
	state.Appearance.Motion = true
	settle(tester)
	top, _ := tester.Find("Newest")
	under, _ := tester.Find("Oldest")
	marks := library.groupTree[""]
	library.groupTree[""] = slices.DeleteFunc(slices.Clone(marks), func(g GroupItem) bool { return g.ID == "s2" })
	library.rev++
	last := top.Y
	for range 40 {
		tester.Frame()
		tester.Advance(16 * time.Millisecond)
		now, _ := tester.Find("Newest")
		if now.Y < last-0.5 || now.Y-last > 12 {
			t.Fatalf("The bookmark over the one going should come down smoothly, but it went from %v to %v.", last, now.Y)
		}
		last = now.Y
		if still, _ := tester.Find("Oldest"); abs(still.Y-under.Y) > 0.5 {
			t.Fatalf("The bookmark under the one going should stay at %v, but it went to %v.", under.Y, still.Y)
		}
	}
	settle(tester)
	if end, _ := tester.Find("Newest"); end.Y <= top.Y+20 {
		t.Fatalf("The bookmark over the one gone should have come down a row, but it is at %v from %v.", end.Y, top.Y)
	}
	if _, ok := tester.Find("Middle"); ok || len(state.field.keptRows) != 2 {
		t.Fatalf("Shut, the bookmark should be gone: %v.", state.field.keptRows)
	}
}

// Under Home stands the trash: pressed, the field shows the trash, the
// button filled; pressed again, it leaves it.
func TestTheTrashStandsUnderHome(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	home, _ := tester.Find("Home")
	trash, ok := tester.Find("Show Trash")
	if !ok || abs(trash.X-home.X) > 0.5 || trash.Y <= home.Y+home.H {
		t.Fatalf("The trash should stand under Home, at %v, Home at %v.", trash, home)
	}
	if err := tester.Click("Show Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !state.inTrash() {
		t.Fatalf("The trash button should show the trash: %q.", state.FieldQuery())
	}
	if err := tester.Click("Leave Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.inTrash() {
		t.Fatal("Pressed again, it should leave the trash.")
	}
}

// The lanes' marks are named at rest, their words facing the window's
// middle: at the right the name, then the count, then the icon at the
// edge; at the left the icon, then the name and count.
func TestTheLanesAreNamedAtRest(t *testing.T) {
	_, _, tester := keptState(t, GroupItem{ID: "s1", Name: "Mark", Query: "$tag:a", Color: TintHalo})
	words := func(row ui.Rect, text string) ui.Rect {
		for _, a := range tester.AccessAll() {
			if a.Name == text && a.Bounds != row && a.Bounds.X >= row.X && a.Bounds.X+a.Bounds.W <= row.X+row.W+1 && a.Bounds.Y < row.Y+row.H && a.Bounds.Y+a.Bounds.H > row.Y {
				return a.Bounds
			}
		}
		return ui.Rect{}
	}
	desk, _ := tester.Find("Desk")
	name, count := words(desk, "Desk"), words(desk, "1")
	if name.W == 0 || count.W == 0 || !(name.X < count.X && count.X+count.W < desk.X+desk.W-10) {
		t.Fatalf("At the right, the name and count should stand before the icon: %v, %v in %v.", name, count, desk)
	}
	kept, _ := tester.Find("Kept")
	name, count = words(kept, "Kept"), words(kept, "1")
	if name.W == 0 || count.W == 0 || !(kept.X+10 < name.X && name.X < count.X) {
		t.Fatalf("At the left, the name and count should stand after the icon: %v, %v in %v.", name, count, kept)
	}
}

// keptNames are the bookmarks' names as the box shows them, top first.
func keptNames(state *State) []string {
	var names []string
	for _, g := range state.bookmarks() {
		names = append(names, g.Name)
	}
	return names
}

// A bookmark dragged onto another stands over or under it, as the half
// it is let go on says, to be undone.
func TestABookmarkDraggedStandsWhereItIsLetGo(t *testing.T) {
	state, _, tester := keptState(t,
		GroupItem{ID: "s1", Name: "Oldest", Query: "$tag:a", Color: TintHalo},
		GroupItem{ID: "s2", Name: "Middle", Query: "$tag:b", Color: TintHalo},
		GroupItem{ID: "s3", Name: "Newest", Query: "$tag:c", Color: TintHalo})
	if got := keptNames(state); !slices.Equal(got, []string{"Newest", "Middle", "Oldest"}) {
		t.Fatalf("The newest should stand on top: %v.", got)
	}
	from, _ := tester.Find("Oldest")
	onto, _ := tester.Find("Newest")
	dragTo(tester, from.X+from.W/3, from.Y+from.H/2, onto.X+onto.W/3, onto.Y+onto.H/4)
	if got := keptNames(state); !slices.Equal(got, []string{"Oldest", "Newest", "Middle"}) {
		t.Fatalf("Let go on the top half of Newest, Oldest should stand over it: %v.", got)
	}
	from, _ = tester.Find("Oldest")
	onto, _ = tester.Find("Middle")
	dragTo(tester, from.X+from.W/3, from.Y+from.H/2, onto.X+onto.W/3, onto.Y+onto.H*3/4)
	if got := keptNames(state); !slices.Equal(got, []string{"Newest", "Middle", "Oldest"}) {
		t.Fatalf("Let go on the bottom half of Middle, Oldest should stand under it: %v.", got)
	}
	if !state.Undo() {
		t.Fatal("The move should be undone.")
	}
	if got := keptNames(state); !slices.Equal(got, []string{"Oldest", "Newest", "Middle"}) {
		t.Fatalf("Undone, Oldest should stand where it stood: %v.", got)
	}
}

// A box fuller than its room scrolls what it holds.
func TestAFullKeptBoxScrolls(t *testing.T) {
	var marks []GroupItem
	for i := range 20 {
		marks = append(marks, GroupItem{ID: fmt.Sprint("s", i), Name: fmt.Sprint("Mark ", i), Query: fmt.Sprint("$tag:m", i), Color: TintHalo})
	}
	state, _, tester := keptState(t, marks...)
	box, _ := tester.Find("Kept searches")
	if state.field.kept.scroll.MaxY <= 0 {
		t.Fatalf("Twenty bookmarks should be more than the box holds: %+v.", state.field.kept.scroll)
	}
	tester.Scroll(box.X+box.W/3, box.Y+box.H/2, 0, 200)
	settle(tester)
	if state.field.kept.scroll.Y <= 0 {
		t.Fatalf("Scrolled over, the box should scroll what it holds: %+v.", state.field.kept.scroll)
	}
}

// Where a group stands is counted as the library counts it, smart
// folders among its siblings, so that a move of it is undone to there.
func TestAGroupsPlaceCountsTheSmartFoldersBesideIt(t *testing.T) {
	state, library := nestedGroupState()
	library.groupTree[""] = []GroupItem{
		{ID: "1", Name: "Inbox", Inbox: true},
		{ID: "s1", Name: "Mark", Query: "$tag:a"},
		{ID: "2", Name: "Novel", HasGroups: true},
	}
	if parent, at := state.groupPlace("2"); parent != "" || at != 2 {
		t.Fatalf("Novel stands third at the top: %q %d.", parent, at)
	}
	if parent, at := state.groupPlace("4"); parent != "3" || at != 0 {
		t.Fatalf("Scene stands first in Arc: %q %d.", parent, at)
	}
}
