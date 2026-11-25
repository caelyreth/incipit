package page

import (
	"time"

	"github.com/caelyreth/neverland/internal/app/clipboard"
	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// The page translates input into intents, sending each to the engine with
// the host answers it carries (§9.1). What an intent does is the engine's.
// Copy is the one edit-menu command that changes nothing: the page writes
// the engine's copy to the clipboard instead.

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
	"bold":          profile.Format{Mark: profile.Strong},
	"italic":        profile.Format{Mark: profile.Emphasis},
	"strikethrough": profile.Format{Mark: profile.Strike},
}

// keyIntent is the intent a key press asks for. A key holding several
// commands runs the first the engine finds available, previewing each
// before the last: Tab leaves a scope or, with none to leave, nests.
func (el *Element) keyIntent(ev ui.InputEvent) (vocab.Intent, bool) {
	keys := el.keys()
	ed := el.Editor
	prog := ed.Engine.Program()
	at := ev.Time
	if at.IsZero() {
		at = ed.now()
	}
	intent := func(chord keymap.Chord) (vocab.Intent, bool) {
		var found []vocab.Intent
		for _, id := range keys.CommandsFor(chord) {
			if cmd, ok := prog.Command(vocab.CommandID(id)); ok {
				found = append(found, cmd.Intent)
			}
		}
		for i, in := range found {
			if i == len(found)-1 {
				return in, true
			}
			if out, err := ed.Engine.Preview(ed.View, vocab.Input{Intent: in, At: at}); err == nil && out.Available {
				return in, true
			}
		}
		return nil, false
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
	case vocab.SelectLine:
		resolved, ok := ed.lineSpan(v)
		if !ok {
			return false
		}
		in = resolved
	case vocab.Paste:
		if v.Text != "" || !v.Slice.Empty() {
			break
		}
		formats := services.ClipboardFormats()
		ours := holds(formats, transfer.Format(clipboard.Format))
		if ed.Keep != nil && !ours {
			if pictures := picturesOf(services.ReadClipboardData(pictureFormats...), formats); len(pictures) > 0 {
				ed.PlacePictures(pictures, nil)
				return true
			}
		}
		clip := ed.pasted(services)
		if len(clip.Pictures) > 0 {
			ed.pasteKept(clip)
			return true
		}
		in = clip.Paste
	case vocab.Cut:
		ed.writeClipboard(services)
	}
	return ed.Do(in, at)
}

// holds reports whether formats include f.
func holds(formats []transfer.Format, f transfer.Format) bool {
	for _, o := range formats {
		if o == f {
			return true
		}
	}
	return false
}
