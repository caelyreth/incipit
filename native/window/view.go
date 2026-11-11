package window

import (
	"time"

	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/page"
)

// captureViews records every open page's selection in the workspace, so a
// page switch, a quit, or a parked page never loses where its caret was.
func (a *AppState) captureViews() {
	if a == nil || a.Workspace == nil {
		return
	}
	for id, session := range a.sessions {
		if session.element == nil {
			continue
		}
		if view, ok := savedView(session.element.Editor.Selection()); ok {
			a.Workspace.SetView(id, view)
		}
		a.Workspace.SetScroll(id, workspace.ScrollPosition{Y: session.element.ScrollY()})
	}
}

// restoreView puts a page's caret and selection back after its document
// loads. A stored place that no longer exists is dropped by the engine's
// selection normalisation.
func (a *AppState) restoreView(session *sheetSession, el *page.Element) {
	view, ok := a.Workspace.View(session.page)
	if !ok {
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
