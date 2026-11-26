package window

import (
	"testing"

	"github.com/caelyreth/neverland/internal/ui/shell"
	"github.com/egoist/mygo/ui"
)

// A task written in a sheet counts in its row and in its group's tree,
// and ⌘↩ toggles it, the counts following.
func TestTasksWrittenCountInTheSheetAndItsGroups(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	tester := atTheDesk(app)
	for _, r := range "- [ ] swim" {
		tester.Type(string(r))
		tester.Frame()
	}
	inbox := func() shell.GroupItem {
		for _, g := range app.Groups("") {
			if g.Inbox {
				return g
			}
		}
		t.Fatal("no Inbox")
		return shell.GroupItem{}
	}
	if got := inbox().Tasks; got != (shell.Tasks{Open: 1}) {
		t.Fatalf("The Inbox should count one open task: %+v.", got)
	}
	if got := app.Sheets(shell.Place{Kind: shell.PlaceAll})[0].Tasks; got != (shell.Tasks{Open: 1}) {
		t.Fatalf("The sheet should count one open task: %+v.", got)
	}
	tester.Key(ui.Cmd, ui.KeyEnter)
	tester.Frame()
	if got := inbox().Tasks; got != (shell.Tasks{Done: 1}) {
		t.Fatalf("⌘↩ should complete it, the Inbox counting it done: %+v.", got)
	}
}
