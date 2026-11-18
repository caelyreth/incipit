package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The arrows go by place, Tab in the field's order, and Left and Right
// only once the keys are on a row, the caret's until then.
func TestTheKeysGoByPlaceAndByOrder(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	tester.Key(0, ui.KeyRight)
	settle(tester)
	if state.field.chosen != "" {
		t.Fatalf("Right should be the caret's with no row chosen, not %q.", state.field.chosen)
	}
	tester.Key(0, ui.KeyDown)
	settle(tester)
	if state.field.chosen != state.field.order[0] {
		t.Fatalf("Down should put the keys on the first row, %q, not %q.", state.field.order[0], state.field.chosen)
	}
	tester.Key(0, ui.KeyDown)
	settle(tester)
	from := state.field.rowRects[state.field.chosen]
	if state.field.chosen != state.field.order[1] {
		t.Fatalf("Down again should go to the row under it, %q, not %q.", state.field.order[1], state.field.chosen)
	}
	// Across to the other card, whichever side it stands.
	across, sign := ui.KeyRight, float32(1)
	for _, r := range state.field.rowRects {
		if r.X+r.W < from.X {
			across, sign = ui.KeyLeft, -1
		}
	}
	tester.Key(0, across)
	settle(tester)
	to := state.field.rowRects[state.field.chosen]
	if sign*(to.X-from.X) <= from.W || to.Y+to.H < from.Y || to.Y > from.Y+from.H {
		t.Fatalf("The arrow should go to the row across from %v, not %q at %v.", from, state.field.chosen, to)
	}
	at := slices.Index(state.field.order, state.field.chosen)
	tester.Key(ui.Shift, ui.KeyTab)
	settle(tester)
	if want := state.field.order[max(at-1, 0)]; state.field.chosen != want {
		t.Fatalf("Shift-Tab should go back in the field's order to %q, not %q.", want, state.field.chosen)
	}
	tester.Key(0, ui.KeyTab)
	settle(tester)
	if want := state.field.order[at]; state.field.chosen != want {
		t.Fatalf("Tab should go on to %q, not %q.", want, state.field.chosen)
	}
}

// Space selects the row the keys are on, a group as a sheet, but the
// Inbox, which nothing moves; the pill's right counts the selection; and
// the query changing lets it go.
func TestSpaceSelectsUntilTheQueryChanges(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	state.field.chosen = "group:1"
	space(tester)
	if len(state.field.picked) != 0 {
		t.Fatalf("The Inbox should not be selected: %v.", state.field.picked)
	}
	state.field.chosen = "group:2"
	space(tester)
	tester.Key(0, ui.KeyDown)
	space(tester)
	if len(state.field.picked) != 2 || !strings.HasPrefix(state.field.picked[0], "group:") {
		t.Fatalf("Space should select the head's group and the row under it: %v.", state.field.picked)
	}
	if state.field.words != "" {
		t.Fatalf("Space on a row should not be written: %q.", state.field.words)
	}
	if _, ok := tester.Find("1 group and 1 sheet selected"); !ok {
		t.Fatalf("The pill's right should count the selection: %v.", tester.Texts())
	}
	space(tester)
	if len(state.field.picked) != 1 {
		t.Fatalf("Space again should let the row go: %v.", state.field.picked)
	}
	tester.Type("c")
	settle(tester)
	if len(state.field.picked) != 0 || state.field.chosen != "" {
		t.Fatalf("Writing should let the selection and the row go: %v, %q.", state.field.picked, state.field.chosen)
	}
}

// While something is selected, the pill's actions are the selection's:
// moved to the trash together, groups and sheets, they say so once, and
// one Undo brings them all back.
func TestTheSelectionsActionsAreOneUndo(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	group, sheet := "group:2", "sheet:8"
	state.pickToggle(group)
	state.pickToggle(sheet)
	settle(tester)
	var labels []string
	for _, a := range state.fieldActions() {
		labels = append(labels, a.label)
	}
	if !slices.Equal(labels, []string{"Open 1 Sheet", "Move To", "Move to Trash"}) {
		t.Fatalf("The selection's actions should stand in place of the place's: %v.", labels)
	}
	if err := tester.Click("Actions"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if _, ok := tester.Find("Move To…"); !ok {
		t.Fatal("Move To should stand, offering where.")
	}
	if err := tester.Click("Move to Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	g, sh := strings.TrimPrefix(group, "group:"), strings.TrimPrefix(sheet, "sheet:")
	if !slices.Contains(library.calls, "trash group "+g) || !slices.Contains(library.calls, "trash "+sh) {
		t.Fatalf("Both should go to the trash: %v.", library.calls)
	}
	if n := state.field.notice; n == nil || n.text != "Moved 1 Group and 1 Sheet to the Trash" {
		t.Fatalf("One notice should say what went: %+v.", state.field.notice)
	}
	if len(state.field.picked) != 0 {
		t.Fatal("Done, the selection should go.")
	}
	if err := tester.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Contains(library.calls, "restore group "+g) || !slices.Contains(library.calls, "restore "+sh) {
		t.Fatalf("One Undo should bring both back: %v.", library.calls)
	}
}

// space presses Space as typing it: the input writes it.
func space(tester *ui.Tester) {
	tester.TypeKey(0, ui.KeySpace, " ")
	settle(tester)
}
