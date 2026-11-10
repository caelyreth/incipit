package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// gesture is a pointer press the renderer owns until release. The engine
// sees one Place intent for it; the selection drawn while it is held is
// only a renderer preview.
type gesture struct {
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
		stop: stop, node: node, clicks: max(clicks, 1),
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
	el.gesture = g
	el.previewGesture(pl)
	return g.valid
}

// moveGesture records the newest pointer place. It does not publish it;
// the next paint resolves one preview for the frame.
func (el *Element) moveGesture(x, y float32, at time.Time) bool {
	if el.gesture == nil {
		return false
	}
	el.gesture.x, el.gesture.y, el.gesture.at = x, y, at
	el.gesture.moved = true
	el.gesture.dirty = true
	return true
}

// previewGesture resolves the latest pointer place through the same Place
// intent the release will publish. It runs at most once per paint.
func (el *Element) previewGesture(pl *pageLayout) {
	g := el.gesture
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

// finishGesture publishes one Place intent, or drops it when the system
// canceled the press.
func (el *Element) finishGesture(canceled bool, at time.Time) bool {
	g := el.gesture
	if g == nil {
		return false
	}
	if !canceled {
		el.previewGesture(el.Editor.layout)
	}
	el.gesture = nil
	if canceled {
		return true
	}
	in := vocab.Place{Stop: g.stop, Node: g.node, Clicks: g.clicks}
	if g.moved || g.extend {
		in.Extend = true
		in.Anchor = &g.anchor
	}
	return el.Editor.DoInput(vocab.Input{Intent: in, At: at})
}

// shownSelection is the selection the renderer draws: the gesture's
// preview while a press is held, otherwise the engine's published view.
func (el *Element) shownSelection() doc.Selection {
	if g := el.gesture; g != nil && g.valid {
		return g.selection
	}
	return el.Editor.Selection()
}
