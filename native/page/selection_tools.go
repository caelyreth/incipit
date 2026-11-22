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

// The page's tools (field-and-desk.md §4.2) are commands offered for the
// selection, in two sets: the selection's own, a bar over a text
// selection come to rest or over the caret where Show Formatting calls it
// up, and the writing dock's under the page. Both are drawn by one tool
// bar (components.ToolBar) from what toolSpecs list: each a command
// previewed for whether it can run and whether its mark or kind is on,
// run as its chord runs it. The bar stays as one runs, so they chain.
// Show Formatting arms it: each tool shows a key of its own, a choice
// opens on its key and the next key picks.
//
// What is offered is listed here for now, the same for every selection:
// which tools suit which extent is the rule layer's to tell
// (before-release-2026-11-21.md, M2 notes).

// toolsRest is how long a selection stays put before its tools rise over
// it, so a selection being made by the keys does not show them at every
// step.
const toolsRest = 160 * time.Millisecond

// ToolSet is which of the page's tool sets is asked for.
type ToolSet uint8

const (
	// SelectionTools are the bar over the selection's.
	SelectionTools ToolSet = iota
	// DockTools are the writing dock's: what is inserted, and what the
	// selection is changed into.
	DockTools
)

// toolSpec is a command the tools offer: its icon, the key that runs it
// while armed, and the mark it toggles, if any.
type toolSpec struct {
	id   vocab.CommandID
	icon components.IconName
	key  string
	mark doc.MarkKind
}

// toolGroupSpec is a group of tools of one shape, and the sets it shows
// in.
type toolGroupSpec struct {
	key, label string
	shape      components.ToolShape
	openKey    string
	sets       []ToolSet
	tools      []toolSpec
}

var toolSpecs = []toolGroupSpec{
	{key: "insert", label: "Insert", shape: components.ToolActions, sets: []ToolSet{DockTools}, tools: []toolSpec{
		{id: "figures.insert", icon: components.IconImage},
		{id: "table.insert", icon: components.IconTable},
	}},
	{key: "kind", label: "Turn Into", shape: components.ToolChoice, openKey: "t",
		sets: []ToolSet{SelectionTools, DockTools}, tools: []toolSpec{
			{id: "blocks.paragraph", key: "p"},
			{id: "blocks.heading1", key: "1"},
			{id: "blocks.heading2", key: "2"},
			{id: "blocks.heading3", key: "3"},
			{id: "blocks.quote", key: "q"},
			{id: "lists.bullet", key: "l"},
			{id: "lists.numbered", key: "n"},
			{id: "blocks.codeBlock", key: "c"},
		}},
	{key: "marks", label: "Formatting", shape: components.ToolToggles,
		sets: []ToolSet{SelectionTools, DockTools}, tools: []toolSpec{
			{id: "formats.strong", icon: components.IconBold, key: "b", mark: profile.Strong},
			{id: "formats.emphasis", icon: components.IconItalic, key: "i", mark: profile.Emphasis},
			{id: "formats.code", icon: components.IconCode, key: "e", mark: profile.CodeMark},
			{id: "formats.strike", icon: components.IconStrike, key: "s", mark: profile.Strike},
		}},
	{key: "selection", label: "Selection", shape: components.ToolActions, sets: []ToolSet{SelectionTools}, tools: []toolSpec{
		{id: "scopes.shrink", icon: components.IconShrinkSelection, key: "left"},
		{id: "scopes.expand", icon: components.IconExpandSelection, key: "right"},
	}},
}

// toolsState is the tools' between frames.
type toolsState struct {
	// at is the selection they stand for, since when; summoned is whether
	// Show Formatting called them up, armed whether their keys act, and
	// dismissed whether Escape put them away for the selection. byTool
	// marks a selection a tool made, which keeps all three.
	at        doc.Selection
	since     time.Time
	summoned  bool
	armed     bool
	dismissed bool
	byTool    bool
	// open is the choice whose menu shows. took is whether the tools took
	// the last key, whose text they drop.
	open string
	took bool
	// shown is whether they showed the frame before.
	shown bool
	// states is how each tool stands, read at statesAt (states).
	states   map[vocab.CommandID]toolState
	statesAt toolsKey
}

// toolsKey is what the tools' states are read from.
type toolsKey struct {
	version uint64
	sel     doc.Selection
	program *policy.Program
}

// toolState is a tool's name, whether it can run now, and whether its
// mark or kind is on.
type toolState struct {
	label       string
	enabled, on bool
}

// ToggleSelectionTools is Show Formatting: it calls the tools up at the
// selection and arms them, or puts them away where they are armed.
func (el *Element) ToggleSelectionTools() {
	t := &el.tools
	if t.shown && t.armed {
		t.summoned, t.armed, t.dismissed, t.open = false, false, true, ""
		return
	}
	t.at, t.summoned, t.armed, t.dismissed = el.Editor.Selection(), true, true, false
	el.Focus()
}

// Tools are the page's tools of a set as the selection stands.
func (el *Element) Tools(set ToolSet) []components.ToolGroup {
	states := el.states()
	var out []components.ToolGroup
	for _, g := range toolSpecs {
		if !inSet(g.sets, set) {
			continue
		}
		group := components.ToolGroup{Key: g.key, Label: g.label, Shape: g.shape,
			OpenKey: g.openKey, OpenCap: capOf(g.openKey)}
		for _, spec := range g.tools {
			st := states[spec.id]
			group.Tools = append(group.Tools, components.Tool{
				ID: string(spec.id), Label: st.label, Icon: spec.icon,
				Binding: el.binding(spec.id), Key: spec.key, Cap: capOf(spec.key),
				Enabled: st.enabled, On: st.on,
			})
		}
		out = append(out, group)
	}
	return out
}

// RunTool runs a tool's command as its chord would, the tools staying.
func (el *Element) RunTool(id string) {
	el.tools.byTool = true
	el.run(vocab.Command{ID: vocab.CommandID(id)}, ui.Services{}, time.Now())
}

func inSet(sets []ToolSet, set ToolSet) bool {
	for _, s := range sets {
		if s == set {
			return true
		}
	}
	return false
}

// capOf is a tool key's cap.
func capOf(key string) string {
	if key == "" {
		return ""
	}
	caps := keymap.Keycaps(keymap.NewChord(key, 0), keymap.PlatformCurrent)
	return caps[len(caps)-1]
}

// binding is a tool's chord as the keymap has it now, which a remap may
// change under the same states.
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

// states is how every tool stands, read again only as the page or the
// selection changes.
func (el *Element) states() map[vocab.CommandID]toolState {
	ed := el.Editor
	key := toolsKey{version: ed.Engine.Version(), sel: ed.Selection(), program: ed.Engine.Program()}
	if key == el.tools.statesAt && el.tools.states != nil {
		return el.tools.states
	}
	labels := map[vocab.CommandID]string{}
	for _, c := range keymap.Declared(key.program.Catalog()) {
		labels[vocab.CommandID(c.ID)] = c.Label
	}
	kind := blockKindAt(ed.Engine.Snapshot(), key.sel.Head.Pos.Node)
	states := map[vocab.CommandID]toolState{}
	for _, g := range toolSpecs {
		for _, spec := range g.tools {
			out, err := ed.Engine.Preview(ed.View, vocab.Input{Intent: vocab.Command{ID: spec.id}, At: ed.active})
			st := toolState{label: labels[spec.id], enabled: err == nil && out.Available}
			switch {
			case spec.mark != 0:
				st.on = st.enabled && el.marked(spec.mark)
			case g.shape == components.ToolChoice:
				st.on = spec.id == kind
			}
			states[spec.id] = st
		}
	}
	el.tools.states, el.tools.statesAt = states, key
	return states
}

// toolsAnchor is what the tools stand over, in the page, and whether they
// show now: over a text selection at rest, or the selection they were
// called up at, while the page has the keyboard and nothing else holds it.
func (el *Element) toolsAnchor(c *ui.Context, pl *pageLayout, focused bool) (ui.Rect, bool) {
	t := &el.tools
	ed := el.Editor
	now := c.Now()
	sel := ed.Selection()
	if sel != t.at {
		// Another selection: one a tool made keeps the tools as they are;
		// any other puts away what was called up for the one before.
		t.at, t.since, t.dismissed = sel, now, false
		if !t.byTool {
			t.summoned, t.armed, t.open = false, false, ""
		}
	}
	t.byTool = false
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

// toolKey answers a key the tools take while they show: Escape closes an
// open menu, else puts them away; with a menu open, the key of one of its
// tools picks it and any other key closes it; armed, a tool's key runs it
// and a choice's opens its menu. Another key disarms them and goes on to
// the page, as do keys held with any modifier.
func (el *Element) toolKey(ev ui.InputEvent) bool {
	t := &el.tools
	if !t.keyed() || ev.Mods != 0 {
		return false
	}
	t.took = true
	if ev.Key == ui.KeyEscape {
		if t.open != "" {
			t.open = ""
		} else {
			t.summoned, t.armed, t.dismissed = false, false, true
		}
		return true
	}
	name, _ := keymap.KeyName(ev.Key)
	for _, g := range el.Tools(SelectionTools) {
		if t.open == "" && g.Shape == components.ToolChoice {
			if name == g.OpenKey {
				t.open = g.Key
				return true
			}
			continue
		}
		if t.open != "" && g.Key != t.open {
			continue
		}
		for _, tool := range g.Tools {
			if tool.Key == name && tool.Enabled {
				t.open = ""
				el.RunTool(tool.ID)
				return true
			}
		}
	}
	if t.open != "" {
		t.open = ""
		return true
	}
	t.armed, t.took = false, false
	return false
}

// keyed reports whether the tools read the keys: armed, or with a menu
// open.
func (t *toolsState) keyed() bool { return t.shown && (t.armed || t.open != "") }

// toolText is the text of a key while the tools read the keys: theirs
// where they took the key, else typed, as the page takes text.
func (el *Element) toolText(text string) bool {
	if el.tools.took {
		el.tools.took = false
		return true
	}
	el.client.ReplaceText(nil, text)
	return true
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

// blockKindAt is the block command naming the kind of the text block at:
// the list or quote holding it nearest, else its own kind.
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

// union is the least rectangle holding a and b.
func union(a, b ui.Rect) ui.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
