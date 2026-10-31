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
// it, and armedLabel, in the destructive style, carries out action. It lets
// go on its own after confirmWindow, on a press anywhere else, or on
// Escape. Without label it shows only once armed, as from a menu.
func (s *State) confirmButton(
	c *ui.Context,
	semantic theme.Theme,
	key, label, armedLabel string,
	animate bool,
	action func(),
) {
	if !s.armed(c, key) {
		if label != "" {
			components.Button{
				Key: key, Label: label, Style: components.SubtleButton, Size: components.CompactButton,
				Animate: animate, Action: func() { s.arm(key) },
			}.View(c, semantic)
		}
		return
	}
	button := components.Button{
		Key: key, Label: armedLabel, Tooltip: "Click to confirm. You can’t undo this.",
		Style: components.DestructiveButton, Size: components.CompactButton,
		Animate: animate, Action: func() {
			s.disarm()
			action()
		},
	}.View(c, semantic)
	if s.nav.armed == key && (button.PressedOutside() || c.Shortcut(0, ui.KeyEscape)) {
		s.disarm()
	}
}
