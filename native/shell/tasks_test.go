package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// taskLibrary is the fake library with tasks: Novel's tree holding five,
// three done, and Chapter two, one done.
type taskLibrary struct{ *fakeLibrary }

func (f taskLibrary) Groups(parent string) []GroupItem {
	groups := f.fakeLibrary.Groups(parent)
	for i := range groups {
		if groups[i].Name == "Novel" {
			groups[i].Tasks = Tasks{Open: 2, Done: 3}
		}
	}
	return groups
}

func (f taskLibrary) Sheets(place Place) []SheetItem {
	sheets := f.fakeLibrary.Sheets(place)
	for i := range sheets {
		if sheets[i].Title == "Chapter" {
			sheets[i].Tasks = Tasks{Open: 1, Done: 1}
		}
	}
	return sheets
}

// A sheet or group holding tasks shows how far they are done as a ring;
// one holding none shows none.
func TestTasksShowAsARingWhereThereAreAny(t *testing.T) {
	state, library := libraryState()
	state.Library = taskLibrary{library}
	state.ShowField()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if _, ok := tester.Find("3 of 5 tasks done"); !ok {
		t.Fatal("Novel's head should show its tree's ring.")
	}
	if _, ok := tester.Find("1 of 2 tasks done"); !ok {
		t.Fatal("Chapter's row should show its ring.")
	}
	if _, ok := tester.Find("0 of 0 tasks done"); ok {
		t.Fatal("Nothing without tasks should show a ring.")
	}
	state.Library = library
	library.rev++
	settle(tester)
	if _, ok := tester.Find("3 of 5 tasks done"); ok {
		t.Fatal("With its tasks gone, Novel should show no ring.")
	}
}
