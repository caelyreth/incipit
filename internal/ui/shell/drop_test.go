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

// A group's card takes a sheet and a group dropped on it, each filed in
// the group.
func TestAGroupCardTakesSheetsAndGroups(t *testing.T) {
	for _, tc := range []struct {
		name, from, to, want string
	}{
		{"a sheet onto a group", "Chapter", "Arc", "move 7 to 3"},
		{"a group onto a group", "Coda", "Arc", "move group 6 to 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, library := libraryState()
			library.groupTree = map[string][]GroupItem{
				"":  {{ID: "1", Name: "Inbox", Inbox: true}, {ID: "2", Name: "Novel", HasGroups: true}},
				"2": {{ID: "3", Name: "Arc"}, {ID: "6", Name: "Coda"}},
			}
			state.ShowField()
			state.SetFieldQuery("$in:Novel")
			tester := ui.NewTester(state.View, 1260, 850)
			settle(tester)
			dragOnto(t, tester, tc.from, tc.to)
			if !slices.Contains(library.calls, tc.want) {
				t.Fatalf("The calls %v lack %q.", library.calls, tc.want)
			}
		})
	}
}
