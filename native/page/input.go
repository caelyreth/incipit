package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// The page translates input into intents and nothing else: every key and
// menu command is an intent in one table, sent to the engine with the host
// answers it carries (§9.1). What an intent does is the engine's.

type mods uint8

const (
	modShift mods = 1 << iota
	modAlt
	modSuper
)

// chord is a key, named or by its text, and the modifiers held with it.
type chord struct {
	key  ui.Key
	text string
	mods mods
}

func chordsOf(ev ui.InputEvent) []chord {
	var m mods
	if ev.Mods&ui.Shift != 0 {
		m |= modShift
	}
	if ev.Mods&ui.Alt != 0 {
		m |= modAlt
	}
	if ev.Mods&(ui.Super|ui.Ctrl) != 0 {
		m |= modSuper
	}
	return []chord{{key: ev.Key, mods: m}, {text: ev.Text, mods: m}}
}

func move(unit vocab.Unit, dir vocab.Direction) vocab.Intent {
	return vocab.Move{Unit: unit, Dir: dir}
}

func del(unit vocab.Unit, dir vocab.Direction) vocab.Intent {
	return vocab.Delete{Unit: unit, Dir: dir}
}

// keyIntents are the page's editing keys. A motion also answers its chord
// with Shift, extending the selection.
// TODO(keymap): route editing keys through app/keymap so they can be
// rebound; these are the defaults.
var keyIntents = map[chord]vocab.Intent{
	{key: ui.KeyLeft}:                  move(vocab.ByGrapheme, vocab.Backward),
	{key: ui.KeyLeft, mods: modAlt}:    move(vocab.ByWord, vocab.Backward),
	{key: ui.KeyLeft, mods: modSuper}:  vocab.VisualMove{Dir: vocab.LineStart},
	{key: ui.KeyRight}:                 move(vocab.ByGrapheme, vocab.Forward),
	{key: ui.KeyRight, mods: modAlt}:   move(vocab.ByWord, vocab.Forward),
	{key: ui.KeyRight, mods: modSuper}: vocab.VisualMove{Dir: vocab.LineEnd},
	{key: ui.KeyUp}:                    vocab.VisualMove{Dir: vocab.Up},
	{key: ui.KeyUp, mods: modAlt}:      move(vocab.ByParagraph, vocab.Backward),
	{key: ui.KeyUp, mods: modSuper}:    move(vocab.ToPageEdge, vocab.Backward),
	{key: ui.KeyDown}:                  vocab.VisualMove{Dir: vocab.Down},
	{key: ui.KeyDown, mods: modAlt}:    move(vocab.ByParagraph, vocab.Forward),
	{key: ui.KeyDown, mods: modSuper}:  move(vocab.ToPageEdge, vocab.Forward),
	{key: ui.KeyHome}:                  vocab.VisualMove{Dir: vocab.LineStart},
	{key: ui.KeyEnd}:                   vocab.VisualMove{Dir: vocab.LineEnd},
	{key: ui.KeyPageUp}:                vocab.VisualMove{Dir: vocab.PageUp},
	{key: ui.KeyPageDown}:              vocab.VisualMove{Dir: vocab.PageDown},

	{key: ui.KeyBackspace}:                 del(vocab.ByGrapheme, vocab.Backward),
	{key: ui.KeyBackspace, mods: modAlt}:   del(vocab.ByWord, vocab.Backward),
	{key: ui.KeyBackspace, mods: modSuper}: del(vocab.ToSlotEdge, vocab.Backward),
	{key: ui.KeyDelete}:                    del(vocab.ByGrapheme, vocab.Forward),
	{key: ui.KeyDelete, mods: modAlt}:      del(vocab.ByWord, vocab.Forward),
	{key: ui.KeyDelete, mods: modSuper}:    del(vocab.ToSlotEdge, vocab.Forward),
	{key: ui.KeyEnter}:                     vocab.Break{},
	{key: ui.KeyEnter, mods: modShift}:     vocab.SoftBreak{},
	{key: ui.KeyTab}:                       vocab.Nest{Dir: vocab.Forward},
	{key: ui.KeyTab, mods: modShift}:       vocab.Nest{Dir: vocab.Backward},
	{key: ui.KeyEscape}:                    vocab.Leave{},
	{text: "z", mods: modSuper}:            vocab.Undo{},
	{text: "z", mods: modSuper | modShift}: vocab.Redo{},
	{text: "Z", mods: modSuper | modShift}: vocab.Redo{},
	{text: "b", mods: modSuper}:            vocab.Format{Mark: vocab.Strong},
	{text: "i", mods: modSuper}:            vocab.Format{Mark: vocab.Emphasis},
	{text: "e", mods: modSuper}:            vocab.Format{Mark: vocab.CodeMark},
}

// menuIntents are the edit menu's commands. Copy changes nothing, so it is
// the one command that is no intent: the page writes the engine's copy to
// the clipboard.
var menuIntents = map[string]vocab.Intent{
	"cut":           vocab.Cut{},
	"paste":         vocab.Paste{},
	"selectAll":     vocab.SelectScope{},
	"undo":          vocab.Undo{},
	"redo":          vocab.Redo{},
	"delete":        del(vocab.ByGrapheme, vocab.Backward),
	"bold":          vocab.Format{Mark: vocab.Strong},
	"italic":        vocab.Format{Mark: vocab.Emphasis},
	"strikethrough": vocab.Format{Mark: vocab.Strike},
}

// keyIntent is the intent a key press asks for.
func keyIntent(ev ui.InputEvent) (vocab.Intent, bool) {
	for _, c := range chordsOf(ev) {
		if c.key == 0 && c.text == "" {
			continue
		}
		if in, ok := keyIntents[c]; ok {
			return in, true
		}
		if c.mods&modShift == 0 {
			continue
		}
		c.mods &^= modShift
		switch in := keyIntents[c].(type) {
		case vocab.Move:
			in.Extend = true
			return in, true
		case vocab.VisualMove:
			in.Extend = true
			return in, true
		}
	}
	return nil, false
}

// run sends an intent with the host answers it carries: the geometry of a
// visual move and the clipboard of a paste or a cut.
func (el *Element) run(in vocab.Intent, services ui.Services, at time.Time) bool {
	ed := el.Editor
	switch v := in.(type) {
	case vocab.VisualMove:
		resolved, ok := ed.resolve(v, el.viewport.H)
		if !ok {
			return false
		}
		in = resolved
	case vocab.Paste:
		in = ed.pasted(services.ReadClipboard())
	case vocab.Cut:
		if text := ed.copy(); text != "" {
			services.WriteClipboard(text)
		}
	}
	return ed.Do(in, at)
}
