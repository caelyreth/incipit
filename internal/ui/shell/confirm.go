package shell

import (
	"time"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// confirmWindow is how long what cannot be undone waits for its second
// click.
const confirmWindow = 4 * time.Second

// dangerTone drives a control's danger-arming spring and hover fade together,
// painting its background, border, and focused ring, and returns the arming
// progress (0..1) and danger foreground color for its contents.
func dangerTone(
	btn ui.Element,
	semantic theme.Theme,
	armed, hovered, animate bool,
	restGround, restBorder ui.Color,
) (float32, ui.Color) {
	var amount float32
	if armed {
		amount = 1
	}
	if animate {
		amount = max(0, min(components.Follow(btn, "armed", amount, motionArm), 1))
	}
	hover := components.HoverAmount(btn, hovered, animate)
	danger := semantic.Status.Danger
	rest := components.Blend(restGround, semantic.GhostElement.Hover, hover)
	btn.Background(components.Blend(rest, danger.Background, amount)).
		Border(1, components.Blend(restBorder, danger.Border, amount))
	if btn.FocusVisible() {
		btn.Border(1, semantic.Border.Focused)
	}
	return amount, danger.Foreground
}

// arm readies the confirmation key names for its second click.
func (s *State) arm(key string) {
	s.armedKey, s.armedAt = key, time.Time{}
}

func (s *State) disarm() { s.armedKey = "" }

// armed reports whether key waits for its second click, letting it go once
// confirmWindow has passed since it was first shown.
func (s *State) armed(c *ui.Context, key string) bool {
	if s.armedKey != key {
		return false
	}
	now := c.Now()
	if s.armedAt.IsZero() {
		s.armedAt = now
	}
	left := confirmWindow - now.Sub(s.armedAt)
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
		AlignItems(ui.Center).Justify(ui.Center).Radius(metrics.Radius).Shrink(0).FocusRing(false).KeepFocus()
	// The fill and border follow the arming spring below; a transition that
	// also eased the colors would chase each frame's, and lag behind.
	transition := popTransition(animate)
	transition.Colors, transition.Position = false, true
	button.Transition(transition)
	amount, dangerInk := dangerTone(button, semantic, armed, button.Hovered(), animate,
		semantic.GhostElement.Background, ui.Transparent)
	// Both labels stand in one cell, so the button is as wide as the longer
	// from the first and keeps its width as it turns: the turn crosses the
	// labels in place, and nothing beside it moves.
	ink := components.Blend(semantic.Text.Default, dangerInk, amount)
	button.Children(func() {
		ui.Grid(c).ColumnTracks(ui.FitContent()).Children(func() {
			for _, face := range [...]struct {
				text    string
				opacity float32
				// The label going leaves before the one coming arrives, each
				// shifting a little, so the two are never read over each other.
				shift float32
			}{
				{label, 1 - min(1, amount/0.5), -amount},
				{armedLabel, max(0, amount-0.5) / 0.5, 1 - amount},
			} {
				if face.text == "" {
					continue
				}
				shown := components.CenteredLabel(c, face.text, theme.TextSmall, ink).
					ColumnStart(1).RowStart(1).JustifySelf(ui.Center).Opacity(face.opacity).
					Translate(0, face.shift*4*scale/theme.DefaultUIFontSize-components.UIOpticalLift(scale))
				if face.opacity < 0.01 {
					// Out of sight, it keeps its room, and is not read out.
					shown.Invisible()
				}
			}
		})
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

// lingering keeps what goes on standing where it stood while it fades,
// as a button that empties what it stands for: an exit copy goes from
// where its row lays it out, and a row that changes as the button goes
// takes the copy across its neighbors. The wrapper is built every frame,
// under key, so the fade knows where it was.
func (s *State) lingering(c *ui.Context, key string, shown, animate bool, fn func()) {
	box := ui.Row(c).Key(key).Shrink(0).AlignItems(ui.Center)
	target := float32(0)
	if shown {
		target = 1
	}
	p := target
	if animate {
		p = max(0, min(components.Follow(box, "shown", target, motionList), 1))
	}
	if p < 0.01 {
		box.Hidden(true)
		return
	}
	box.Opacity(p).Children(fn)
}
