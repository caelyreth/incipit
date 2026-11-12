package page

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/egoist/mygo/ui"
)

// Each kind of object the page draws is one renderer, looked up by its
// kind: the page's loops call it and nothing outside it names the kind or
// reads what it keeps on its unit or segment. A kind without a renderer
// of its own stacks its surfaces, or is a rule when it has none.

// objectKind lays out and draws one kind of block object.
type objectKind interface {
	// lay lays u out at width: its surfaces placed, its height, box,
	// drawn and frame, as in asks.
	lay(lc *layoutCache, u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, in objectIn)
	// paint draws the object beneath its surfaces' panels, and gives how
	// strongly its panel surfaces show.
	paint(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) float32
	// holds is what it holds open besides its surfaces, which its room
	// springs on.
	holds(u *unitLayout) int
	// caretX is where a caret in surface s stands across, in the page,
	// when the object places it itself.
	caretX(el *Element, u *unitLayout, s *surfaceLayout) (float32, bool)
	// over draws what the object shows over its surfaces' text.
	over(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32)
	// cover is the box surface s stands in, in its unit, when the object
	// sets its surfaces in boxes, as a table its cells: a point in it is
	// in the surface, and what covers all of the surface fills it.
	cover(u *unitLayout, s *surfaceLayout) (ui.Rect, bool)
	// controls are what it offers the pointer over it, bottom to top
	// (controls.go).
	controls(el *Element, u *unitLayout) []control
	// step moves what it moves on to the paint's time, before the page is
	// laid out, and sets what it asks of its own layout (springs.go).
	step(el *Element, u *unitLayout, in *objectIn)
	// anchor is how far the place it holds under the pointer moved from
	// was, drawn at wasY, to u, as the page is laid out again: the page
	// scrolls by it, so the place stays put.
	anchor(el *Element, was *unitLayout, wasY float32, u *unitLayout) (float32, bool)
}

// baseKind is what a kind does where it does nothing of its own: it holds
// nothing open, places no caret, draws nothing over its text, sets its
// surfaces in no boxes, offers no controls, moves nothing and holds no
// place.
type baseKind struct{}

func (baseKind) holds(*unitLayout) int                                        { return 0 }
func (baseKind) caretX(*Element, *unitLayout, *surfaceLayout) (float32, bool) { return 0, false }
func (baseKind) over(*Element, *ui.Painter, *unitLayout, float32, float32)    {}
func (baseKind) cover(*unitLayout, *surfaceLayout) (ui.Rect, bool)            { return ui.Rect{}, false }
func (baseKind) controls(*Element, *unitLayout) []control                     { return nil }
func (baseKind) step(*Element, *unitLayout, *objectIn)                        {}
func (baseKind) anchor(*Element, *unitLayout, float32, *unitLayout) (float32, bool) {
	return 0, false
}

// inlineKind lays out and draws one kind of inline object in its line.
type inlineKind interface {
	// lay completes the object's segment and gives the span its box
	// takes in the line.
	lay(lc *layoutCache, sg seg, style present.Style, m Metrics) (seg, ui.Span)
	// paint draws the object in its box o.
	paint(el *Element, p *ui.Painter, sg seg, o ui.Rect)
	// step moves what it moves on to the paint's time, before the page is
	// laid out.
	step(el *Element, sg seg)
}

var (
	objectKinds = map[doc.Kind]objectKind{profile.Figure: figureKind{}, profile.Math: mathKind{}, profile.Table: tableKind{}}
	inlineKinds = map[doc.Kind]inlineKind{profile.Formula: formulaKind{}}
)

// kindOf is the renderer of an object presentation.
func kindOf(p *present.Presentation) objectKind {
	if k, ok := objectKinds[p.Kind]; ok {
		return k
	}
	if len(p.Surfaces) > 0 {
		return stackKind{}
	}
	return ruleKind{}
}

// stackKind stacks an object's surfaces, as text units stand.
type stackKind struct{ baseKind }

func (stackKind) lay(lc *layoutCache, u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, _ objectIn) {
	lc.layStack(u, p, comp, th, m, width, false)
	u.box = ui.Rect{W: width, H: u.h}
}
func (stackKind) paint(*Element, *ui.Painter, *unitLayout, float32, float32) float32 { return 0 }

// ruleKind draws an object with no text as a rule across its column.
type ruleKind struct{ baseKind }

func (ruleKind) lay(_ *layoutCache, u *unitLayout, _ *present.Presentation, _ *policy.Composition, _ present.Theme, m Metrics, width float32, _ objectIn) {
	u.h = lineHeight(m, m.Body)
	u.box = ui.Rect{W: width, H: u.h}
}
func (ruleKind) paint(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	mid := uy + u.h/2
	p.Line(ux, mid, ux+u.box.W, mid, 1, el.Theme.Prose.Subtle)
	return 0
}

// figureKind draws a figure: its picture over its caption.
type figureKind struct{ baseKind }

func (figureKind) lay(lc *layoutCache, u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, in objectIn) {
	lc.layFigure(u, p, comp, th, m, width, in.span)
}
func (figureKind) paint(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	el.paintFigure(p, u, ux, uy, el.paintNow, el.animate)
	return 0
}

// caretX stands a caret in an empty caption at its hint's start.
func (figureKind) caretX(el *Element, u *unitLayout, s *surfaceLayout) (float32, bool) {
	if !el.hinted(u) {
		return 0, false
	}
	_, x, _ := hintOf(u, el.Editor.theme, el.Editor.metrics)
	return u.x + s.x + x, true
}

// mathKind draws a math block: its display over its source, a panel.
type mathKind struct{ baseKind }

func (mathKind) lay(lc *layoutCache, u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, _ objectIn) {
	lc.layStack(u, p, comp, th, m, width, true)
	layMath(u, lc, m, width)
}
func (mathKind) paint(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	return el.paintMath(p, u, ux, uy)
}

// holds is its panel open and its display set.

func (mathKind) step(el *Element, u *unitLayout, _ *objectIn) {
	if u.math != nil {
		el.stepMath(u.pres.Unit, u.math.mathSet, u.math.open)
	}
}

func (mathKind) holds(u *unitLayout) int {
	n := 0
	if u.math.open {
		n++
	}
	if u.math.box != nil {
		n += 2
	}
	return n
}

// formulaKind draws an inline formula as its math, on the baseline.
type formulaKind struct{}

func (formulaKind) lay(lc *layoutCache, sg seg, style present.Style, m Metrics) (seg, ui.Span) {
	ms := lc.typeset(sg.node.ID(), sg.node.Text(), false, m.Body*style.Size, 0)
	sp := span("￼", style, m.Body)
	sg.text, sg.math = sg.node.Text(), ms
	sg.face = span(formulaHint, style, m.Body)
	sg.face.Italic = true
	if ms.box != nil {
		sp.ObjectWidth = ms.to - ms.from + mathPad*2
		sp.ObjectHeight, sp.ObjectDescent = ms.ascent, ms.descent
	} else {
		// No math yet: the formula stands as its hint.
		lay := ui.ShapeRichTextWith([]ui.Span{sg.face}, ui.Font{Family: sg.face.Font, Size: sg.face.Size, Italic: true}, ui.TextShape{})
		w, _ := lay.Size()
		sp.ObjectWidth = w + mathPad*2
		sp.ObjectHeight, sp.ObjectDescent = sg.face.Size*0.75, sg.face.Size*0.25
	}
	return sg, sp
}
func (formulaKind) paint(el *Element, p *ui.Painter, sg seg, o ui.Rect) { el.paintInline(p, sg, o) }
func (formulaKind) step(el *Element, sg seg)                            { el.stepMath(sg.node.ID(), sg.math, sg.open) }
