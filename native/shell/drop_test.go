package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// dragOnto drags what shows from onto what shows to, the pointer moving
// there in steps, a frame each, as a hand does.
func dragOnto(t *testing.T, tester *ui.Tester, from, to string) {
	t.Helper()
	a, ok := tester.Find(from)
	if !ok {
		t.Fatalf("%q is not shown", from)
	}
	b, ok := tester.Find(to)
	if !ok {
		t.Fatalf("%q is not shown", to)
	}
	x, y := a.X+a.W/2, a.Y+a.H/2
	tx, ty := b.X+b.W/2, b.Y+b.H/2
	tester.Press(x, y)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		tester.Move(x+(tx-x)*f, y+(ty-y)*f)
		tester.Frame()
	}
	tester.Release(tx, ty)
	tester.Frame()
	settle(tester)
}

// A target that takes several kinds of drag takes each of them, not only
// the kind it asks about last.
func TestDropTargetsTakeEveryKindTheyAccept(t *testing.T) {
	for _, tc := range []struct {
		name      string
		allSheets []SheetItem
		from, to  string
		want      string
	}{
		{"a sheet onto a group board", nil, "Draft", "Novel", "move 8 to 2"},
		{"a sheet onto the trash token", nil, "Draft", "Show trash", "trash 8"},
		{"a sheet onto Inbox board", []SheetItem{{ID: "7", Title: "Chapter"}}, "Chapter", "Inbox", "move 7 to 1"},
		{"a group onto the trash token", nil, "Novel", "Show trash", "trash group 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, library := libraryState()
			library.allSheets = tc.allSheets
			state.ToggleField()
			tester := ui.NewTester(state.View, 1260, 850)
			settle(tester)
			dragOnto(t, tester, tc.from, tc.to)
			if !slices.Contains(library.calls, tc.want) {
				t.Fatalf("calls %v lack %q", library.calls, tc.want)
			}
		})
	}
}
