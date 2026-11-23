package window

import (
	"time"

	"github.com/caelyreth/neverland/internal/app/workspace"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/page"
)

// captureViews records every open column's selection and scroll in the
// workspace, so a switch, a quit, or a column behind another never loses
// where its caret was.
func (a *AppState) captureViews() {
	if a == nil || a.Workspace == nil {
		return
	}
	for _, session := range a.sessions {
		for col, el := range session.columns {
			if view, ok := savedView(el.Editor.Selection()); ok {
				a.Workspace.SetView(col, view)
			}
			a.Workspace.SetScroll(col, workspace.ScrollPosition{Y: el.ScrollY()})
		}
	}
}

// restoreView puts a column's caret and selection back as its editor
// opens. A stored place that no longer exists is dropped by the engine's
// selection normalisation.
func restoreView(view workspace.ViewState, el *page.Element) {
	if !view.Valid() {
		return
	}
	snap := el.Editor.Engine.Snapshot()
	at := time.Now()
	switch view.Kind {
	case workspace.SelectNode:
		if _, ok := snap.Node(doc.ID(view.Node)); !ok {
			return
		}
		el.Editor.Do(vocab.Place{Node: doc.ID(view.Node)}, at)
	case workspace.SelectText:
		anchor, ok1 := restoreCaret(snap, view.Anchor)
		head, ok2 := restoreCaret(snap, view.Head)
		if !ok1 || !ok2 {
			return
		}
		if !el.Editor.Do(vocab.Place{Stop: anchor}, at) {
			return
		}
		if anchor != head {
			el.Editor.Do(vocab.Place{Stop: head, Extend: true}, at)
		}
	}
}

func savedView(selection doc.Selection) (workspace.ViewState, bool) {
	switch selection.Kind {
	case doc.SelectText:
		return workspace.ViewState{
			Kind:   workspace.SelectText,
			Anchor: savedCaret(selection.Anchor),
			Head:   savedCaret(selection.Head),
		}, true
	case doc.SelectNode:
		return workspace.ViewState{Kind: workspace.SelectNode, Node: uint64(selection.Node)}, true
	default:
		// A cell range's head is a table placeholder in this build; it is
		// not yet a place the engine can put back.
		return workspace.ViewState{}, false
	}
}

func savedCaret(caret doc.Caret) workspace.Caret {
	return workspace.Caret{
		Node:   uint64(caret.Pos.Node),
		Slot:   caret.Pos.Slot,
		Offset: caret.Pos.Offset,
		Stop:   caret.Stop,
	}
}

func restoreCaret(snap *doc.Snapshot, saved workspace.Caret) (doc.Caret, bool) {
	node, ok := snap.Node(doc.ID(saved.Node))
	if !ok || saved.Slot < 0 || saved.Slot >= node.NumSlots() {
		return doc.Caret{}, false
	}
	offset := max(0, min(saved.Offset, node.Slot(saved.Slot).Len()))
	return doc.Caret{Pos: doc.Pos{Node: doc.ID(saved.Node), Slot: saved.Slot, Offset: offset}, Stop: saved.Stop}, true
}
