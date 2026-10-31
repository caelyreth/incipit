package shell

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// confirmWindow is how long what cannot be undone waits for its second
// click.
const confirmWindow = 4 * time.Second

// arm readies the confirmation key names for its second click.
func (s *State) arm(key string) {
	s.nav.armed, s.nav.armedAt = key, time.Time{}
}

func (s *State) disarm() { s.nav.armed = "" }

// armed reports whether key waits for its second click, letting it go once
// confirmWindow has passed since it was first shown.
func (s *State) armed(c *ui.Context, key string) bool {
	if s.nav.armed != key {
		return false
	}
	now := c.Now()
	if s.nav.armedAt.IsZero() {
		s.nav.armedAt = now
	}
	left := confirmWindow - now.Sub(s.nav.armedAt)
	if left <= 0 {
		s.disarm()
		return false
	}
	c.After(left)
	return true
}

// confirmButton does what cannot be undone on a second click: label arms
// it, and it turns, in the danger color, to armedLabel, which carries out
// action. It lets go on its own after confirmWindow, on a press anywhere
// else, or on Escape. Without label it shows only once armed, as from a
// menu, growing in where it stands.
func (s *State) confirmButton(
	c *ui.Context,
	semantic theme.Theme,
	key, label, armedLabel string,
	animate bool,
	action func(),
) {
	armed := s.armed(c, key)
	if !armed && label == "" {
		return
	}
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	text := label
	if armed {
		text = armedLabel
	}
	button := ui.ButtonBase(c.Key(key)).Label(text).
		Height(components.CompactButton.Height(scale)).PaddingX(theme.Space08.DIPs(scale)).
		AlignItems(ui.Center).Justify(ui.Center).Radius(metrics.Radius).Shrink(0).FocusRing(false)
	if armed {
		button.Tooltip("Click again to delete. You can’t undo this.")
	}
	transition := popTransition(animate)
	if label != "" && animate {
		transition = ui.ElementTransition{
			Duration: motionList, Size: true,
			Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0},
		}
	}
	button.Transition(transition)
	amount := float32(0)
	if armed {
		amount = 1
	}
	hover := float32(0)
	if button.Hovered() {
		hover = 1
	}
	if animate {
		amount = components.Follow(button, "armed", amount, motionArm)
		hover = components.HoverAmount(button, button.Hovered(), animate)
	}
	rest := components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)
	danger := semantic.Status.Danger
	button.Background(components.Blend(rest, danger.Background, amount)).
		Border(1, components.Blend(ui.Transparent, danger.Border, amount))
	if button.FocusVisible() {
		button.Border(1, semantic.Border.Focused)
	}
	button.Children(func() {
		components.Label(c, text, theme.TextSmall,
			components.Blend(semantic.Text.Default, danger.Foreground, amount)).SingleLine()
	})
	if button.Clicked() {
		if armed {
			s.disarm()
			action()
			return
		}
		s.arm(key)
		return
	}
	if armed && (button.PressedOutside() || c.Shortcut(0, ui.KeyEscape)) {
		s.disarm()
	}
}
