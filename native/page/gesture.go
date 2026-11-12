package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// gesture is a press on the text, which no control took: the page's drag
// until release. The engine sees one Place intent for it; the selection
// drawn while it is held is only a renderer preview.
type gesture struct {
	el           *Element
	stop, anchor doc.Caret
	node         doc.ID
	clicks       int
	extend       bool
	moved        bool
	x, y         float32
	at           time.Time
	dirty        bool
	selection    doc.Selection
	valid        bool
}

// beginGesture records a press and computes its first preview.
func (el *Element) beginGesture(x, y float32, clicks int, extend bool, at time.Time) bool {
	pl := el.Editor.layout
	if pl == nil {
		return false
	}
	stop, node, ok := pl.hit(x, y)
	if !ok {
		return false
	}
	g := &gesture{
		el: el, stop: stop, node: node, clicks: max(clicks, 1),
		extend: extend, x: x, y: y, at: at, dirty: true,
	}
	if extend {
		if sel := el.Editor.Selection(); sel.Kind == doc.SelectText {
			g.anchor = sel.Anchor
		} else {
			g.anchor = stop
		}
	} else {
		g.anchor = stop
	}
	el.drag = g
	el.previewGesture(pl)
	if !g.valid {
		el.drag = nil
	}
	return g.valid
}

// textGesture is the press on the text the page holds, if any.
func (el *Element) textGesture() *gesture {
	g, _ := el.drag.(*gesture)
	return g
}

func (g *gesture) key() controlKey   { return controlKey{} }
func (g *gesture) cursor() ui.Cursor { return 0 }

// move records the newest pointer place. It does not publish it; the
// next paint resolves one preview for the frame.
func (g *gesture) move(x, y float32, at time.Time) {
	g.x, g.y, g.at = x, y, at
	g.moved = true
	g.dirty = true
}

// previewGesture resolves the latest pointer place through the same Place
// intent the release will publish. It runs at most once per paint.
func (el *Element) previewGesture(pl *pageLayout) {
	g := el.textGesture()
	if g == nil || pl == nil || !g.dirty {
		return
	}
	g.dirty = false
	stop, node, ok := pl.hit(g.x, g.y)
	if !ok {
		return
	}
	g.stop, g.node = stop, node
	in := vocab.Place{Stop: stop, Node: node, Clicks: g.clicks}
	if g.moved || g.extend {
		in.Extend = true
		in.Anchor = &g.anchor
	}
	out, err := el.Editor.Engine.Preview(el.Editor.View, vocab.Input{Intent: in, At: g.at})
	if err != nil {
		return
	}
	g.selection, g.valid = out.Selection, true
}

// finish publishes one Place intent, or drops it when the system canceled
// the press.
func (g *gesture) finish(canceled bool, at time.Time) {
	el := g.el
	if canceled {
		return
	}
	if g.dirty {
		el.drag = g
		el.previewGesture(el.Editor.layout)
		el.drag = nil
	}
	in := vocab.Place{Stop: g.stop, Node: g.node, Clicks: g.clicks}
	if g.moved || g.extend {
		in.Extend = true
		in.Anchor = &g.anchor
	}
	el.Editor.DoInput(vocab.Input{Intent: in, At: at})
}

// shownSelection is the selection the renderer draws: the gesture's
// preview while a press is held, otherwise the engine's published view.
func (el *Element) shownSelection() doc.Selection {
	if g := el.textGesture(); g != nil && g.valid {
		return g.selection
	}
	return el.Editor.Selection()
}
