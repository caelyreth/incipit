package page

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// Select Line takes the caret's whole line as laid out, and pressed again
// over whole lines, the next line with them.
func TestSelectLineTakesTheLineThenTheNext(t *testing.T) {
	el := newElement(t)
	typeText(el, strings.Repeat("wrapping words ", 30))
	pl := el.Editor.layout
	sl := pl.units[0].surfaces[0]
	lines := sl.layout.Lines()
	if len(lines) < 3 {
		t.Fatalf("the text lays out in %d lines, want at least 3", len(lines))
	}
	el.Editor.Do(vocab.Place{Stop: sl.stops[3].caret}, time.Unix(5000, 0))
	el.relayout()

	clock := int64(6000)
	selectLine := func() (int, int) {
		t.Helper()
		clock++
		if !el.run(vocab.SelectLine{}, ui.Services{}, time.Unix(clock, 0)) {
			t.Fatal("Select Line did not run")
		}
		el.relayout()
		f, to := el.Editor.Selection().Ordered(el.Editor.Engine.Snapshot())
		return f.Pos.Offset, to.Pos.Offset
	}
	want := func(n int) (int, int) {
		start, _ := sl.lineBounds(lines[0])
		_, end := sl.lineBounds(lines[n-1])
		return sl.nearest(start).caret.Pos.Offset, sl.nearest(end).caret.Pos.Offset
	}
	for n := 1; n <= 2; n++ {
		f, to := selectLine()
		wf, wt := want(n)
		if f != wf || to != wt {
			t.Fatalf("pressed %d times, Select Line took %d..%d, want lines 1..%d at %d..%d", n, f, to, n, wf, wt)
		}
	}
}
