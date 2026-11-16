package settings

import (
	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A shortcut is recorded in place: pressing a row's keys opens the recorder
// under the row, and a taken key is resolved there.

func shortcutLabel(shortcut appsettings.Shortcut, command keymap.CommandID) string {
	label := shortcut.Label
	if label == "" {
		label = string(command)
	}
	if len(shortcut.Bindings) == 0 {
		return "Add shortcut for " + label
	}
	return "Change shortcut for " + label
}

func (s *State) startRecording(command keymap.CommandID, host string) {
	s.keyEditing, s.keyHost, s.keyConflict = command, host, nil
	s.keyReturn = ""
	s.reportRecording()
}

// stopRecording ends a recording and gives the keyboard back to the key it
// was started from, so recording again, or moving on, goes on from there.
func (s *State) stopRecording() {
	if s.keyEditing != "" {
		s.keyReturn = triggerKey(s.keyHost, s.keyEditing)
	}
	s.keyEditing, s.keyHost, s.keyConflict = "", "", nil
	s.reportRecording()
}

// triggerKey names the keycaps that start recording command at host.
func triggerKey(host string, command keymap.CommandID) string {
	return "trigger-" + host + "-" + string(command)
}

// returnFocus gives trigger the keyboard back once, after its recording,
// quietly: the focus comes back for the keyboard to go on from, not as a
// place the person moved to, so it shows no ring until they move it.
func (s *State) returnFocus(trigger ui.Element, key string) {
	if s.keyReturn == key && s.keyEditing == "" {
		s.keyReturn, s.keyQuiet = "", key
		trigger.Focus(ui.FocusOptions{PreventScroll: true})
	}
}

// keyRing is how strongly a key is outlined, from 0 to 1, eased both ways:
// fully while it records, and while it holds a focus the keyboard brought.
// A focus given back quietly stays unringed until it leaves.
func (s *State) keyRing(trigger ui.Element, key string, recording, animate bool) float32 {
	if s.keyQuiet == key && !trigger.Focused() && s.keyReturn != key {
		s.keyQuiet = ""
	}
	target := float32(0)
	if recording || trigger.FocusVisible() && s.keyQuiet != key {
		target = 1
	}
	if !animate {
		return target
	}
	return components.Follow(trigger, "ring", target, components.MotionQuick)
}

// StopRecording ends a recording in progress, as the window it is in loses
// the keyboard or closes.
func (s *State) StopRecording() {
	if s != nil {
		s.keySearch = false
		s.stopRecording()
	}
}

// reportRecording tells the host when recording starts or ends: a shortcut
// for an action, or one pressed to search the keymap.
func (s *State) reportRecording() {
	recording := s.keyEditing != "" || s.keySearch
	if recording != s.reported {
		s.reported = recording
		if s.Recording != nil {
			s.Recording(recording)
		}
	}
}

// recorder opens under host while one of its keys is recording: a capture
// line, which answers a chord another action holds in place. It folds open
// and shut without moving what is below in a jump; closing, it shows what it
// last showed, inert. start aligns it with the host's own text.
func (s *State) recorder(c *ui.Context, semantic theme.Theme, host string, start float32) {
	open := s.keyEditing != "" && s.keyHost == host
	shown, ok := s.recorders[host]
	switch {
	case open:
		shown = recorderView{Command: s.keyEditing}
		if s.keyConflict != nil {
			conflict := *s.keyConflict
			shown.Conflict = &conflict
		}
		if s.recorders == nil {
			s.recorders = make(map[string]recorderView)
		}
		s.recorders[host] = shown
	case !ok:
		// Never opened here, or long shut: nothing to draw.
		return
	}
	reveal := components.Fold{
		Key: "recorder-" + host, Open: open, Animate: s.motion(c), Appear: true,
	}.View(c, func() {
		scale := s.Theme.UIFontSize
		inset := theme.Space16.DIPs(scale)
		command := shown.Command
		shortcut, _ := s.Data.Shortcut(command)
		line := captureLine{
			Key: "shortcut-capture-" + string(command), Prompt: "Press keys…",
			Live: open, Conflict: shown.Conflict,
			Chosen: func(chord keymap.Chord) { s.recordShortcut(command, chord) },
			Cancel: s.stopRecording,
			Clear: func() {
				s.stopRecording()
				s.apply(appsettings.Unbind(command))
			},
			Replace: func() {
				if conflict := s.keyConflict; conflict != nil {
					s.stopRecording()
					s.apply(appsettings.RemapReplacing(command, conflict.Target, conflict.Owner.Command))
				}
			},
		}
		if shortcut.Overridden {
			line.Reset = func() {
				s.stopRecording()
				s.apply(appsettings.ResetBinding(command))
			}
		}
		ui.Row(c).WidthPercent(100).MinWidth(0).
			Padding(0, inset, theme.Space12.DIPs(scale), start).Children(func() {
			s.captureLine(c, semantic, line)
		})
	})
	if !open && reveal <= 0 {
		delete(s.recorders, host)
	}
}

// recorderView is what a recorder shows: the command recording, and the
// answer to its last chord when another action holds it.
type recorderView struct {
	Command  keymap.CommandID
	Conflict *keyConflict
}
