package shell

import (
	"testing"

	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// Move To follows the groups' levels: a group holding groups opens a
// submenu of itself and of them, and an excluded group goes with them.
func TestMoveToFollowsTheGroupsLevels(t *testing.T) {
	state, _ := nestedGroupState()
	var moved string
	var exclude []string
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Padding(20).Children(func() { ui.Text(c, "Here") }).ContextMenu(func(m *ui.Menu) {
			state.moveMenu(m, func(group string) { moved = group }, exclude...)
		})
	}, 400, 300)
	for _, c := range []struct {
		path []string
		want string
	}{
		{[]string{locales.MoveTo(), "Novel", "Arc", "Scene"}, "4"},
		{[]string{locales.MoveTo(), "Novel", "Arc", "Arc"}, "3"},
		{[]string{locales.MoveTo(), "Novel", "Novel"}, "2"},
		{[]string{locales.MoveTo(), "Inbox"}, "1"},
	} {
		moved = ""
		tester.RightClick("Here")
		if err := tester.ChooseMenuItem(c.path...); err != nil || moved != c.want {
			t.Fatalf("%q: moved to %q, want %q: %v", c.path, moved, c.want, err)
		}
	}
	// Moving Arc, it and what it holds are no place to move to: Novel is
	// left holding none, an item of its own.
	exclude = []string{"3"}
	tester.RightClick("Here")
	if err := tester.ChooseMenuItem(locales.MoveTo(), "Novel"); err != nil || moved != "2" {
		t.Fatalf("Novel without Arc: moved to %q: %v", moved, err)
	}
}
