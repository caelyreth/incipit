package page

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// TestFarUnitsLetTheirShapingGo keeps the text near the view shaped and
// lets the rest go, sizes kept; a unit read while it is far is held.
func TestFarUnitsLetTheirShapingGo(t *testing.T) {
	el := newElement(t)
	el.Editor.Do(vocab.Paste{Text: strings.Repeat("Plain words here, enough of them to wrap a line or two.\n\n", 200)}, time.Unix(100, 0))
	el.relayout()
	pl := el.Editor.layout
	w, h := el.textSize()
	releaseFar(pl, 0, 0, 600)
	first, last := pl.units[0], pl.units[len(pl.units)-1]
	if first.far || !first.surfaces[0].layout.Shaped() {
		t.Fatal("the unit in view let its shaping go")
	}
	if !last.far || last.surfaces[0].layout.Shaped() {
		t.Fatal("the last unit, far below, is still shaped")
	}
	if rw, rh := el.textSize(); rw != w || rh != h {
		t.Errorf("measured with its far text let go, the page is %vx%v, want %vx%v", rw, rh, w, h)
	}
	// Read while far, it is shaped again, and held.
	if _, _, n := last.surfaces[0].lineOf(0); n == 0 {
		t.Fatal("the last unit has no lines")
	}
	releaseFar(pl, 0, 0, 600)
	if !last.held || !last.surfaces[0].layout.Shaped() {
		t.Error("read while far, the last unit was let go again")
	}
	// Scrolled to, it is near again.
	releaseFar(pl, -last.y, 0, 600)
	if last.far || last.held || !first.far {
		t.Errorf("scrolled to the end: the last unit far %v, held %v; the first far %v", last.far, last.held, first.far)
	}
}
