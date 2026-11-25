package page

import (
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/egoist/mygo/ui"
)

// Look Up shows what a word means, as macOS does: the page's menu offers
// it (profile.LookUpOffers), and a force click looks up the word it is
// on. The words policy says which text, as an EffectLookUp; the page finds
// where it drew it, and the window shows its meaning over it.

// lookUpAt asks the rules to look up the word at (x, y) in the element, as
// a force click does, the selection left as it is; it reports whether
// there was one.
func (el *Element) lookUpAt(x, y float32) bool {
	ed := el.Editor
	if ed.layout == nil {
		return false
	}
	at, ok := ed.layout.hit(el.toPage(x, y))
	if !ok {
		return false
	}
	ed.Do(profile.LookUp{At: &at}, ed.now())
	return ed.lookUp != nil
}

// showLookUp shows the meaning of what the rules asked to be looked up,
// over where the page drew it, the element standing at bounds in the
// window.
func (el *Element) showLookUp(services ui.Services, bounds ui.Rect) {
	ed := el.Editor
	ask := ed.lookUp
	if ask == nil {
		return
	}
	ed.lookUp = nil
	if at, size, ok := el.lookUpPlace(*ask); ok {
		services.ShowDefinition(ask.Text, ui.Point{X: bounds.X + at.X, Y: bounds.Y + at.Y},
			ui.Font{Family: el.Prose.Family, Size: size, Weight: 400})
	}
}

// lookUpPlace is where the text ask names starts on its baseline, in the
// element, and the size of the type its surface sets.
func (el *Element) lookUpPlace(ask profile.LookUpAsk) (ui.Point, float32, bool) {
	pl := el.Editor.layout
	if pl == nil {
		return ui.Point{}, 0, false
	}
	pc, ok := pl.lookup(doc.CaretAt(doc.At(ask.Slot, ask.From)))
	if !ok || pc.surface == nil || pc.stop == nil {
		return ui.Point{}, 0, false
	}
	s, at := pc.surface, pc.stop.u
	caret := s.layout.Caret(at)
	baseline := caret.Y + caret.H
	for _, line := range s.layout.Lines() {
		if line.Start <= at && at < line.End {
			baseline = line.Baseline
			break
		}
	}
	x, y := pc.unit.origin(s)
	r := el.fromPage(ui.Rect{X: x + caret.X, Y: y + baseline})
	return ui.Point{X: r.X, Y: r.Y}, s.fontSize, true
}
