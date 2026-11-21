package window

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// What writing adds to the pages is written today, less what it takes
// away; a copy writes nothing; and the day's words come back with the
// notebook.
func TestTheWordsWrittenTodayAreWhatWritingChanged(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	tester := atTheDesk(app)
	for _, r := range "one two three" {
		tester.Type(string(r))
		tester.Frame()
	}
	if got := app.Shell.WrittenToday(); got != 3 {
		t.Fatalf("Three words were written today: %d.", got)
	}
	tester.Key(0, ui.KeyBackspace)
	tester.Key(0, ui.KeyBackspace)
	tester.Key(0, ui.KeyBackspace)
	tester.Key(0, ui.KeyBackspace)
	tester.Key(0, ui.KeyBackspace)
	tester.Key(0, ui.KeyBackspace)
	tester.Frame()
	if got := app.Shell.WrittenToday(); got != 2 {
		t.Fatalf("A word taken away is no longer written: %d.", got)
	}
	id, _ := app.Workspace.Active()
	copied := app.DuplicateSheet(sheetName(id))
	if copied == "" {
		t.Fatal("The sheet should be duplicated.")
	}
	app.OpenSheet(copied, true)
	for range 3 {
		tester.Frame()
	}
	if session, ok := app.sessionOf(copied); !ok || session.summarized == nil {
		t.Fatal("The copy should be open and read.")
	}
	if got := app.Shell.WrittenToday(); got != 2 {
		t.Fatalf("A copy writes nothing: %d.", got)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	again := openNotebookApp(t, root)
	again.selectMounted()
	if got := again.Shell.WrittenToday(); got != 2 {
		t.Fatalf("Today's words should come back with the notebook: %d.", got)
	}
}
