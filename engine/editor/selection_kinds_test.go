package editor

import (
	"testing"
	"time"
)

// Cells picked whole are the engine's selection: copying reads them as a
// spreadsheet does, Delete empties them and keeps them picked, Dismiss
// lets them go, and any other intent lets them go before it runs.
func TestCellsPickedWholeResolveIntents(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	if _, err := session.InsertTable([][]string{{"a", "b"}, {"c", "d"}}); err != nil {
		t.Fatal(err)
	}
	table, _ := TableOf(session.Runtime().Snapshot().Document(), DocumentBlocks(session.Runtime().Snapshot().Document())[0].ID())
	if err := session.SelectCells(CellRect(table.First(), [2]int{0, 0}, [2]int{1, 0})); err != nil {
		t.Fatal(err)
	}
	if session.SelectionKind() != SelectionCells {
		t.Fatalf("kind = %d, want cells", session.SelectionKind())
	}
	if text, err := session.SelectedText(); err != nil || text != "a\nc\n" {
		t.Fatalf("copied %q, %v", text, err)
	}
	if _, err := session.Do(Delete(ByGrapheme, Backward), time.Time{}); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"", "b"}, []string{"", "d"})
	if session.SelectionKind() != SelectionCells {
		t.Fatal("Delete let the cells go")
	}
	if _, err := session.Do(Dismiss, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if session.SelectionKind() != SelectionText {
		t.Fatal("Dismiss kept the cells")
	}
}

// A picture selected whole is an object: Dismiss leaves it with the caret
// at its caption's end.
func TestDismissLeavesASelectedPictureAtItsCaption(t *testing.T) {
	session, line := pictureBetween(t, "Dusk")
	if err := session.SelectProjected(line.Start, line.End); err != nil {
		t.Fatal(err)
	}
	if session.SelectionKind() != SelectionObject {
		t.Fatalf("kind = %d, want object", session.SelectionKind())
	}
	if _, err := session.Do(Dismiss, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if got := session.Caret().RangeUTF16(); !got.Empty() || got.Start != line.End {
		t.Fatalf("caret = %v, want at %d", got, line.End)
	}
}
