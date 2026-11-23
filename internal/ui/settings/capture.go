package settings

import (
	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// captureLine is the one control that takes a shortcut, wherever one is
// pressed: under a Keymap row or in the Keymap's search. It
// asks for keys, and when the keys are taken it says so in place, in the
// same line, turning from the accent to a warning while its words cross
// over, so a conflict reads as the recording's answer, not as a new row. It
// stays live meanwhile: another chord tries again, Return takes the key
// anyway, Escape gives up.
type captureLine struct {
	Key    string
	Prompt string
	// Compact sets the line at a field's height, as in a toolbar beside
	// other controls; otherwise it is a little taller, to hold a sentence
	// and its buttons with room.
	Compact bool
	// Live is false for the picture of a closing line: it takes no focus
	// or keys, and its buttons do nothing.
	Live bool
	// Conflict, when set, is the answer to the last chord pressed.
	Conflict *keyConflict
	Chosen   func(keymap.Chord)
	Cancel   func()
	// Clear, Reset and Replace are offered when set: Backspace clears,
	// Use default resets, and Replace takes a key another action holds.
	Clear   func()
	Reset   func()
	Replace func()
}

const (
	captureRecording = "recording"
	captureTaken     = "taken"
	captureReserved  = "reserved"
)

// captureSwap crosses one state's words over to the next in place.
var captureSwap = ui.ElementTransition{
	Animation: &keySpring,
	Enter:     &ui.Motion{Y: 3, Opacity: 0},
	Exit:      &ui.Motion{Y: -3, Opacity: 0},
}

func (s *State) captureLine(c *ui.Context, semantic theme.Theme, line captureLine) {
	scale := s.Theme.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	animate := s.motion(c)
	state, message, edge := captureRecording, line.Prompt, semantic.Border.Focused
	if conflict := line.Conflict; conflict != nil {
		chord := keymap.Spell(conflict.Target, keymap.PlatformCurrent)
		state, edge = captureTaken, semantic.Status.Warning.Foreground
		message = locales.AssignedTo(chord, conflict.Owner.Label)
		if conflict.Reserved {
			state = captureReserved
			message = locales.KeptInAppMenu(chord, conflict.Owner.Label)
		}
	}
	height := metrics.Height
	if !line.Compact {
		height += theme.Space08.DIPs(scale)
	}
	capture := ui.Row(c).Key(line.Key).Role(ui.RoleTextField).Label(message).
		Height(height).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).
		Padding(0, theme.Space06.DIPs(scale), 0, theme.Space12.DIPs(scale)).Radius(metrics.Radius).
		Background(semantic.Surface.Inset).Clip()
	if animate {
		edge = capture.AnimateColor("edge", edge, components.Spring(components.MotionNormal))
	}
	capture.Border(1, edge)
	// The same line may close and open again before its fold shuts: each
	// recording tracks the keyboard afresh.
	wasLive := ui.Local(capture, "live", func() bool { return false })
	if line.Live {
		s.holdKeyboard(capture, !*wasLive, line)
	}
	*wasLive = line.Live

	capture.Children(func() {
		// The words of each state, and of each chord found taken, are their
		// own element, so a change crosses over rather than cutting.
		content := ui.Row(c).Key(state + message).Grow(1).Shrink(1).MinWidth(0).
			AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale))
		if animate {
			content.Transition(captureSwap)
		}
		content.Children(func() {
			icon, tint := components.IconKeyboard, semantic.Icon.Accent
			if state != captureRecording {
				icon, tint = components.IconWarning, semantic.Status.Warning.Foreground
			}
			ui.Icon(c, icon.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).TextColor(tint)
			components.CenteredLabel(c, message, theme.TextSmall, semantic.Text.Default).
				Grow(1).Shrink(1).MinWidth(0)
			s.captureTrailing(c, semantic, state, line)
		})
	})
}

// captureTrailing is the end of the line: what the keys can do now, and the
// buttons that do the rest.
func (s *State) captureTrailing(c *ui.Context, semantic theme.Theme, state string, line captureLine) {
	// The end keeps its size; the message before it is what gives way.
	hint := func(text string) {
		// Set in from the edge as far as a button's words are.
		components.CenteredLabel(c, text, theme.TextXSmall, semantic.Text.Muted).Shrink(0).
			Padding(0, theme.Space06.DIPs(s.Theme.UIFontSize), 0, 0)
	}
	// Pressed, a button leaves the keyboard with the line, which would
	// otherwise take its losing the focus for giving up.
	button := func(key, label string, style components.ButtonStyle, action func()) {
		components.Button{
			Key: key, Label: label, Style: style, Size: components.CompactButton,
			Animate: s.motion(c),
			Action: func() {
				if line.Live && action != nil {
					action()
				}
			},
		}.View(c, semantic).KeepFocus().Shrink(0)
	}
	switch state {
	case captureTaken:
		button("capture-cancel", locales.Cancel(), components.SubtleButton, line.Cancel)
		button("capture-replace", locales.Replace(), components.WarningButton, line.Replace)
	case captureReserved:
		hint(locales.PressAnotherKey())
		button("capture-cancel", locales.Cancel(), components.SubtleButton, line.Cancel)
	default:
		switch {
		case line.Clear != nil:
			hint(locales.EscCancelsBackspaceClears())
		default:
			hint(locales.EscToCancel())
		}
		if line.Reset != nil {
			button("capture-reset", locales.UseDefault(), components.SubtleButton, line.Reset)
		}
	}
}

// holdKeyboard keeps the keyboard with a live line and takes the next chord.
// It keeps the focus every frame, not only as it appears: the key or
// keycaps pressed to start recording take the focus as they are clicked, and
// a line that let them keep it would leave Tab to move the focus and other
// chords to whatever holds it. A press elsewhere, which leaves the focus
// where it is when it lands on a button (as a Mac's do), or the focus moved
// away, ends the recording.
//
// The host stands its menu down meanwhile, so chords such as ⌘N reach the
// line as keys. A menu role's command arriving meanwhile is taken and
// dropped, so it never acts while a key is being recorded.
func (s *State) holdKeyboard(capture ui.Element, fresh bool, line captureLine) {
	held := ui.Local(capture, "held", func() bool { return false })
	if fresh {
		*held = false
	}
	if *held && (!capture.Focused() || capture.PressedOutside()) {
		*held = false
		line.Cancel()
		return
	}
	capture.Focusable().FocusRing(false).Focus()
	*held = capture.Focused()
	capture.HandleInput(func(event ui.InputEvent) bool {
		switch event.Kind {
		case ui.InputCommand:
			return true
		case ui.InputKeyDown:
		default:
			return false
		}
		switch {
		case event.Mods == 0 && event.Key == ui.KeyEscape:
			line.Cancel()
		case event.Mods == 0 && event.Key == ui.KeyEnter && line.Conflict != nil &&
			!line.Conflict.Reserved && line.Replace != nil:
			line.Replace()
		case event.Mods == 0 && line.Clear != nil &&
			(event.Key == ui.KeyBackspace || event.Key == ui.KeyDelete):
			line.Clear()
		default:
			// A bare modifier waits for the key it modifies.
			if chords := keymap.EventChords(event, keymap.PlatformCurrent); len(chords) > 0 {
				line.Chosen(chords[0])
			}
		}
		return true
	})
}
