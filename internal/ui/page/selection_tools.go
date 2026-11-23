package page

import (
	"github.com/egoist/mygo/i18n"
	"time"

	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/app/phrases"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/egoist/mygo/ui"
)

// The page's tools (field-and-desk.md §4.2) are the commands the rules
// offer for the selection (policy.Offer, Engine.Offers), drawn by one tool
// bar (components.ToolBar) in two surfaces: the bar over a text selection
// come to rest, or over the caret where Show Formatting calls it up
// (vocab.SurfaceTools), and the writing dock under the page
// (vocab.SurfaceDock). What is offered, whether it can run and whether it
// is on are the rules' answers; the page only draws them, run as their
// chords run them, an icon each from its table (toolIcon). The bar stays
// as one runs, so they chain. Show Formatting arms it: each tool shows the
// key its offer names, a choice opens on its group's key and the next key
// picks.

// toolsRest is how long a selection stays put before its tools rise over
// it, so a selection being made by the keys does not show them at every
// step.
const toolsRest = 160 * time.Millisecond

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
	// shown is whether they showed the frame before; settle holds their
	// groups while what they are offered for keeps changing.
	shown  bool
	settle components.ToolSettle
	// rise is the way they rose where they last stood, which they go
	// back along.
	rise float32
	// drawn are each surface's tools as last drawn, and labels the
	// commands' names, for the program and the language they were read in.
	drawn    map[vocab.Surface]drawnTools
	labels   map[vocab.CommandID]string
	labelsOf *policy.Program
	labelsIn string
}

// drawnTools are a surface's tools as drawn from the offers at a version,
// with a keymap, in a language.
type drawnTools struct {
	version uint64
	keys    *keymap.Keymap
	lang    string
	groups  []components.ToolGroup
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

// Tools are the commands offered for the selection on surface, as tool
// groups, drawn again only as the engine, the keymap or the language
// changes.
func (el *Element) Tools(surface vocab.Surface) []components.ToolGroup {
	ed := el.Editor
	t := &el.tools
	version, lang := ed.Engine.Version(), i18n.Locale()
	if d, ok := t.drawn[surface]; ok && d.version == version && d.keys == el.Keys && d.lang == lang {
		return d.groups
	}
	labels := el.commandLabels()
	var groups []components.ToolGroup
	for _, g := range ed.Engine.Offers(ed.View, surface) {
		group := components.ToolGroup{Key: g.Group.ID, Label: phrases.Text(g.Group.Label), Shape: toolShape(g.Group.Shape),
			OpenKey: g.Group.Key, OpenCap: capOf(g.Group.Key)}
		for _, o := range g.Offers {
			group.Tools = append(group.Tools, components.Tool{
				ID: string(o.Command), Label: labels[o.Command], Icon: toolIcon(o.Command),
				Binding: el.binding(o.Command), Key: o.Key, Cap: capOf(o.Key),
				Enabled: o.Available, On: o.On,
			})
		}
		groups = append(groups, group)
	}
	if t.drawn == nil {
		t.drawn = map[vocab.Surface]drawnTools{}
	}
	t.drawn[surface] = drawnTools{version: version, keys: el.Keys, lang: lang, groups: groups}
	return groups
}

// RunTool runs a tool's command as its chord would, the tools staying.
func (el *Element) RunTool(id string) {
	el.tools.byTool = true
	el.run(vocab.Command{ID: vocab.CommandID(id)}, ui.Services{}, time.Now())
}

// toolShape is how the bar draws a group of a shape.
func toolShape(s vocab.OfferShape) components.ToolShape {
	switch s {
	case vocab.OfferToggles:
		return components.ToolToggles
	case vocab.OfferChoice:
		return components.ToolChoice
	}
	return components.ToolActions
}

// commandLabels are the commands' names, as the keymap lists them, in the
// language the app shows: kept while the program and the language stand.
func (el *Element) commandLabels() map[vocab.CommandID]string {
	t := &el.tools
	prog, lang := el.Editor.Engine.Program(), i18n.Locale()
	if t.labels == nil || t.labelsOf != prog || t.labelsIn != lang {
		t.labels, t.labelsOf, t.labelsIn = map[vocab.CommandID]string{}, prog, lang
		for _, c := range keymap.Declared(prog.Catalog()) {
			t.labels[vocab.CommandID(c.ID)] = c.Label()
		}
	}
	return t.labels
}

// capOf is a tool key's cap.
func capOf(key string) string {
	if key == "" {
		return ""
	}
	caps := keymap.Keycaps(keymap.NewChord(key, 0), keymap.PlatformCurrent)
	return caps[len(caps)-1]
}

// binding is a tool's chord as the keymap has it now.
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
	for _, g := range el.Tools(vocab.SurfaceTools) {
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

// union is the least rectangle holding a and b.
func union(a, b ui.Rect) ui.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
