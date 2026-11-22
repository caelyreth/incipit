package page

import (
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

// The selection's tools (field-and-desk.md §4.2): a small floating bar
// over a text selection come to rest, or over the caret where Show
// Formatting calls it up, holding what is done to what is selected: its
// marks, its blocks' kind, and the selection's own rungs (Expand and
// Shrink Selection). Each tool is a command, run as its key runs it and
// showing that key, and the bar stays as one runs, so they follow one
// another from the pointer or the keys alike. It is the page's, as the
// tag picker is: it stands in the page's own coordinates, moving with the
// text as it scrolls. Typing, a drag, Escape or the selection collapsing
// puts it away.

// toolsRest is how long a selection stays put before its tools rise over
// it, so a selection being made by the keys does not show them at every
// step.
const toolsRest = 160 * time.Millisecond

// toolsState is the tools' between frames.
type toolsState struct {
	// at is the selection they stand for, since when; summoned is whether
	// Show Formatting called them up at it, and dismissed whether Escape
	// put them away for it.
	at        doc.Selection
	since     time.Time
	summoned  bool
	dismissed bool
	// shown is whether they showed the frame before.
	shown bool
	// model is what they offer, read at the engine's version for the
	// selection at (modelOf).
	model   toolsModel
	modelAt toolsKey
}

// toolsKey is what the tools' model is read from.
type toolsKey struct {
	version uint64
	sel     doc.Selection
	program *policy.Program
}

// toolsModel is what the tools offer as the selection stands.
type toolsModel struct {
	marks  []tool
	blocks []tool
	// block is the kind of the blocks the selection is in, one of blocks.
	block          vocab.CommandID
	expand, shrink tool
}

// tool is one command the tools run: whether it can run now, and whether
// what it toggles is on.
type tool struct {
	id      vocab.CommandID
	label   string
	icon    components.IconName
	enabled bool
	on      bool
}

// The commands the tools hold, as the policies declare them.
var (
	markTools = []struct {
		id   vocab.CommandID
		mark doc.MarkKind
		icon components.IconName
	}{
		{"formats.strong", profile.Strong, components.IconBold},
		{"formats.emphasis", profile.Emphasis, components.IconItalic},
		{"formats.code", profile.CodeMark, components.IconCode},
		{"formats.strike", profile.Strike, components.IconStrike},
	}
	blockTools = []vocab.CommandID{
		"blocks.paragraph", "blocks.heading1", "blocks.heading2", "blocks.heading3",
		"blocks.quote", "lists.bullet", "lists.numbered", "blocks.codeBlock",
	}
	expandTool = vocab.CommandID("scopes.expand")
	shrinkTool = vocab.CommandID("scopes.shrink")
)

// ToggleSelectionTools calls the tools up at the selection, or puts them
// away where they show: Show Formatting.
func (el *Element) ToggleSelectionTools() {
	t := &el.tools
	if t.shown {
		t.summoned, t.dismissed = false, true
		return
	}
	t.at, t.summoned, t.dismissed = el.Editor.Selection(), true, false
	el.Focus()
}

// toolsAnchor is what the tools stand over, in the page, and whether they
// show now: over a text selection at rest, or the caret they were called
// up at, while the page has the keyboard and nothing else holds it.
func (el *Element) toolsAnchor(c *ui.Context, pl *pageLayout, focused bool) (ui.Rect, bool) {
	t := &el.tools
	ed := el.Editor
	now := c.Now()
	sel := ed.Selection()
	if sel != t.at {
		// Another selection: tools called up stay only while it is one a
		// tool made from theirs.
		t.at, t.since, t.dismissed = sel, now, false
		t.summoned = t.summoned && t.shown && !sel.IsCollapsed()
	}
	ranged := sel.Kind == doc.SelectText && !sel.IsCollapsed()
	if !focused || t.dismissed || !(ranged || t.summoned) || pl == nil ||
		ed.deco.Composition != nil || el.drag != nil || el.textGesture() != nil || el.tags.open() {
		return ui.Rect{}, false
	}
	if wait := toolsRest - now.Sub(t.since); !t.shown && !t.summoned && wait > 0 {
		c.After(wait)
		return ui.Rect{}, false
	}
	var r ui.Rect
	if ranged {
		for i, row := range selectionRows(pl, el.shownPaints(pl)) {
			if i == 0 {
				r = row
			} else {
				r = union(r, row)
			}
		}
	} else if box, ok := pl.caretBox(sel.Head, ed.metrics); ok {
		r = box
	}
	if r.W <= 0 && r.H <= 0 {
		return ui.Rect{}, false
	}
	// Scrolled out of the page's view, they go with it.
	if shown := el.fromPage(r); shown.Y+shown.H < 0 || shown.Y > el.viewport.H {
		return ui.Rect{}, false
	}
	return r, true
}

// modelOf is what the tools offer for the selection, read again only as
// the page or the selection changes.
func (el *Element) modelOf() toolsModel {
	ed := el.Editor
	key := toolsKey{version: ed.Engine.Version(), sel: ed.Selection(), program: ed.Engine.Program()}
	if key == el.tools.modelAt {
		return el.tools.model
	}
	labels := map[vocab.CommandID]string{}
	for _, c := range keymap.Declared(key.program.Catalog()) {
		labels[vocab.CommandID(c.ID)] = c.Label
	}
	toolFor := func(id vocab.CommandID, icon components.IconName) tool {
		out, err := ed.Engine.Preview(ed.View, vocab.Input{Intent: vocab.Command{ID: id}, At: ed.active})
		return tool{id: id, label: labels[id], icon: icon, enabled: err == nil && out.Available}
	}
	var m toolsModel
	for _, mt := range markTools {
		t := toolFor(mt.id, mt.icon)
		t.on = t.enabled && el.marked(mt.mark)
		m.marks = append(m.marks, t)
	}
	m.block = blockKindAt(ed.Engine.Snapshot(), key.sel.Head.Pos.Node)
	for _, id := range blockTools {
		t := toolFor(id, "")
		t.on = id == m.block
		m.blocks = append(m.blocks, t)
	}
	m.expand = toolFor(expandTool, components.IconExpandSelection)
	m.shrink = toolFor(shrinkTool, components.IconShrinkSelection)
	el.tools.model, el.tools.modelAt = m, key
	return m
}

// marked reports whether what the selection holds is marked mark, as
// toggling it would take the mark away: over a range, where its toggle
// removes it; at a caret, where typing would take it.
func (el *Element) marked(mark doc.MarkKind) bool {
	ed := el.Editor
	view, _ := ed.Engine.View(ed.View)
	if view.Selection.Kind == doc.SelectText && !view.Selection.IsCollapsed() {
		out, err := ed.Engine.Preview(ed.View, vocab.Input{Intent: profile.Format{Mark: mark}, At: ed.active})
		if err != nil {
			return false
		}
		for _, step := range out.Steps {
			if op, ok := step.Op().(doc.MarkText); ok {
				for _, removed := range op.Remove {
					if removed == mark {
						return true
					}
				}
			}
		}
		return false
	}
	caret := view.Selection.Head
	if p := view.Pending; p != nil && p.At == caret {
		return p.Marks.Has(mark)
	}
	snap := ed.Engine.Snapshot()
	node, ok := snap.Node(caret.Pos.Node)
	if !ok || caret.Pos.Slot >= node.NumSlots() || !node.Slot(caret.Pos.Slot).IsInline() {
		return false
	}
	marks := vocab.InheritedMarks(snap.Schema(), node.Slot(caret.Pos.Slot).Content(), caret.Pos.Offset)
	return marks != nil && marks.Has(mark)
}

// blockKindAt is the block command naming the kind of the text block at,
// as the tools show it: the list or quote holding it nearest, else its
// own kind.
func blockKindAt(snap *doc.Snapshot, at doc.ID) vocab.CommandID {
	for id := at; ; {
		parent, _, ok := snap.Parent(id)
		if !ok {
			break
		}
		if n, ok := snap.Node(parent); ok {
			switch n.Kind() {
			case profile.List:
				if profile.ListStyleOf(n) == profile.Ordered {
					return "lists.numbered"
				}
				return "lists.bullet"
			case profile.Quote:
				return "blocks.quote"
			}
		}
		id = parent
	}
	n, ok := snap.Node(at)
	if !ok {
		return "blocks.paragraph"
	}
	switch n.Kind() {
	case profile.Heading:
		switch profile.HeadingLevel(n) {
		case 1:
			return "blocks.heading1"
		case 2:
			return "blocks.heading2"
		default:
			return "blocks.heading3"
		}
	case profile.Code:
		return "blocks.codeBlock"
	}
	return "blocks.paragraph"
}

// binding is a tool's key as the keymap has it now, which a remap may
// change under the same model.
func (el *Element) binding(id vocab.CommandID) string {
	if el.Keys == nil {
		return ""
	}
	chord, ok := el.Keys.ChordFor(keymap.CommandID(id))
	if !ok {
		return ""
	}
	return keymap.Spell(chord, keymap.PlatformCurrent)
}

// runTool runs a tool's command as its key would, the tools staying.
func (el *Element) runTool(id vocab.CommandID, services ui.Services) {
	el.tools.summoned = el.tools.summoned || el.tools.shown
	el.run(vocab.Command{ID: id}, services, time.Now())
	services.Invalidate()
}
