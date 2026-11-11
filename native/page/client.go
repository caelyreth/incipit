package page

import (
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// client is the page's text-input client: the system's input methods see
// the surface the caret is in as their text, in UTF-16 indices of its
// laid-out text, and edit it through intents.
type client struct{ el *Element }

// surface is the laid-out surface holding the caret.
func (c *client) surface() (*surfaceLayout, *unitLayout, *stopAt) {
	ed := c.el.Editor
	if ed.layout == nil {
		return nil, nil, nil
	}
	pc, ok := ed.layout.lookup(ed.Selection().Head)
	if !ok || pc.surface == nil {
		return nil, nil, nil
	}
	return pc.surface, pc.unit, pc.stop
}

func clampRange(r ui.TextInputRange, n int) ui.TextInputRange {
	r.Start = min(max(r.Start, 0), n)
	r.End = min(max(r.End, r.Start), n)
	return r
}

func (c *client) TextForRange(r ui.TextInputRange) (string, ui.TextInputRange) {
	s, _, _ := c.surface()
	if s == nil {
		return "", ui.TextInputRange{}
	}
	r = clampRange(r, utf16Len(s.text))
	return s.text[byteIndex(s.text, r.Start):byteIndex(s.text, r.End)], r
}

func (c *client) Selection() ui.TextInputSelection {
	s, _, head := c.surface()
	if s == nil {
		return ui.TextInputSelection{}
	}
	if s.hasMarked {
		return ui.TextInputSelection{Range: ui.TextInputRange{Start: s.compSel, End: s.compSel}}
	}
	ed := c.el.Editor
	sel := ed.Selection()
	anchor := head.u
	if pc, ok := ed.layout.lookup(sel.Anchor); ok && pc.surface == s {
		anchor = pc.stop.u
	}
	if anchor > head.u {
		return ui.TextInputSelection{Range: ui.TextInputRange{Start: head.u, End: anchor}, Reversed: true}
	}
	return ui.TextInputSelection{Range: ui.TextInputRange{Start: anchor, End: head.u}}
}

func (c *client) MarkedRange() (ui.TextInputRange, bool) {
	s, _, _ := c.surface()
	if s == nil || !s.hasMarked {
		return ui.TextInputRange{}, false
	}
	return s.marked, true
}

// selectRange selects a range of the caret's surface, as an input method
// replacing text other than the selection asks.
func (c *client) selectRange(r ui.TextInputRange) {
	s, _, _ := c.surface()
	if s == nil {
		return
	}
	cur := c.Selection().Range
	if cur == r {
		return
	}
	a, b := s.nearest(r.Start), s.nearest(r.End)
	if a == nil || b == nil {
		return
	}
	ed := c.el.Editor
	ed.Do(vocab.Place{Stop: a.caret, Clicks: 1}, ed.now())
	if b.caret != a.caret {
		ed.Do(vocab.Place{Stop: b.caret, Clicks: 1, Extend: true}, ed.now())
	}
}

func (c *client) ReplaceText(r *ui.TextInputRange, text string) {
	ed := c.el.Editor
	composing := ed.deco.Composition != nil
	if r != nil && !composing {
		c.selectRange(*r)
	}
	switch {
	case composing:
		ed.Do(vocab.Type{Text: text}, ed.now())
	case text == "\n":
		ed.Do(vocab.Break{}, ed.now())
	default:
		// Text replaces the selection; empty text deletes it.
		ed.Do(vocab.Type{Text: text}, ed.now())
	}
	ed.Relayout()
}

func (c *client) SetMarkedText(r *ui.TextInputRange, text string, sel ui.TextInputRange) {
	ed := c.el.Editor
	if text == "" {
		if ed.deco.Composition != nil {
			ed.Do(vocab.CancelComposition{}, ed.now())
		}
		ed.Relayout()
		return
	}
	if r != nil && ed.deco.Composition == nil {
		c.selectRange(*r)
	}
	sel = clampRange(sel, utf16Len(text))
	ed.Do(vocab.Compose{Preedit: text, Selected: [2]int{byteIndex(text, sel.Start), byteIndex(text, sel.End)}}, ed.now())
	ed.Relayout()
}

func (c *client) UnmarkText() {
	ed := c.el.Editor
	if comp := ed.deco.Composition; comp != nil {
		ed.Do(vocab.Type{Text: comp.Text}, ed.now())
		ed.Relayout()
	}
}

func (c *client) BoundsForRange(r ui.TextInputRange) (ui.Rect, ui.TextInputRange, bool) {
	s, u, _ := c.surface()
	if s == nil {
		return ui.Rect{}, r, false
	}
	r = clampRange(r, utf16Len(s.text))
	cr := s.caretRect(r.Start)
	if r.End > r.Start {
		if rects := s.layout.SelectionRects(r); len(rects) > 0 {
			cr = rects[0]
		}
	}
	x, y := u.origin(s)
	cr.X += x
	cr.Y += y
	return c.el.fromPage(cr), r, true
}

func (c *client) IndexForPoint(pt ui.Point) (int, bool) {
	s, u, _ := c.surface()
	if s == nil {
		return 0, false
	}
	px, py := c.el.toPage(pt.X, pt.Y)
	x, y := u.origin(s)
	if py < y || py >= y+s.h {
		return 0, false
	}
	return s.layout.IndexAt(ui.Point{X: px - x, Y: py - y}), true
}
