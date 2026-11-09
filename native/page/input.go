package page

import (
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// The page translates input into intents and nothing else: every key and
// menu command is an intent in one table, sent to the engine with the host
// answers it carries (§9.1). What an intent does is the engine's.

// Keys come through the keymap: a chord names a command, and the program
// resolves the command to its intent, so a disabled policy's keys do
// nothing and a remapped key moves with its command. A motion also
// answers its chord with Shift, extending the selection.

// menuIntents are the edit menu's commands. Copy changes nothing, so it is
// the one command that is no intent: the page writes the engine's copy to
// the clipboard.
var menuIntents = map[string]vocab.Intent{
	"cut":           vocab.Cut{},
	"paste":         vocab.Paste{},
	"selectAll":     vocab.SelectScope{},
	"undo":          vocab.Undo{},
	"redo":          vocab.Redo{},
	"delete":        vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward},
	"bold":          vocab.Format{Mark: vocab.Strong},
	"italic":        vocab.Format{Mark: vocab.Emphasis},
	"strikethrough": vocab.Format{Mark: vocab.Strike},
}

// keyIntent is the intent a key press asks for.
func (el *Element) keyIntent(ev ui.InputEvent) (vocab.Intent, bool) {
	keys := el.Keys
	if keys == nil {
		keys = keymap.Default()
	}
	prog := el.Editor.Engine.Program()
	intent := func(chord keymap.Chord) (vocab.Intent, bool) {
		id, ok := keys.CommandFor(chord)
		if !ok {
			return nil, false
		}
		cmd, ok := prog.Command(vocab.CommandID(id))
		return cmd.Intent, ok
	}
	for _, chord := range keymap.EventChords(ev, keymap.PlatformCurrent) {
		if in, ok := intent(chord); ok {
			return in, true
		}
		if !chord.Modifiers.Has(keymap.ModifierShift) {
			continue
		}
		chord.Modifiers &^= keymap.ModifiersShift
		in, _ := intent(chord)
		switch in := in.(type) {
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
