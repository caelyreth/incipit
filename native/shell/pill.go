package shell

import (
	"math"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The pill is where the field's query is written: the query's conditions
// as ribbons, each the color of what it names, and what is written after
// them. A condition comes in by a click on the
// field, or written whole and followed by a space; a ribbon pressed, or
// Backspace with nothing written, takes the last away. Return opens the
// row the keys are on, the arrows move them along the field's rows, and
// Escape takes the words away, then the conditions, then goes to the desk.
// Written after ">", what the pill holds names the app's commands, and
// Return runs the one the keys are on. Beside it on the horizon stand
// Neverland's mark, before it, and the settings, after it.

// horizonSide is how much room, at the reference type size, the mark and
// the settings each take beside the pill.
const horizonSide = 140

// horizonView is what stands on the horizon at r: the pill, Neverland's
// mark just before it, and the settings just after.
func (s *State) horizonView(c *ui.Context, semantic theme.Theme, r stage.Rect, w, k float32, animate bool) {
	gap := 12 * k
	ui.Row(c).Key("wordmark").Absolute().Top(r.Y).Right(w - r.X + gap).Height(r.H).
		AlignItems(ui.Center).IgnoreInput(true).Role(ui.RoleImage).Label("Neverland").
		Children(func() {
			ui.Row(c).Key("name").AlignItems(ui.End).Children(func() {
				size := 22 * k
				ui.Text(c, "Neverland").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
					LetterSpacing(-0.02 * size).TextColor(semantic.Text.Default).SingleLine()
				ui.Text(c, ".").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
					TextColor(semantic.Accent.Foreground).SingleLine()
			})
		})
	s.pillView(c, semantic, r, k, animate)
	settings := ui.ButtonBase(c.Key("settings")).Absolute().Left(r.X+r.W+gap).Top(r.Y).Size(r.H, r.H).
		Radius(r.H / 2).AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).
		Label("Open settings").Tooltip("Settings")
	hover := components.HoverAmount(settings, settings.Hovered(), animate)
	settings.Background(components.Blend(semantic.Surface.Page, semantic.GhostElement.Hover, hover)).
		Border(1, semantic.Border.Variant)
	if settings.Clicked() && s.OpenSettings != nil {
		s.OpenSettings()
	}
	settings.Children(func() {
		ui.Icon(c, components.IconSettings.SVG()).FontSize(16 * k).TextColor(semantic.Icon.Muted)
	})
}

func (s *State) pillView(c *ui.Context, semantic theme.Theme, r stage.Rect, k float32, animate bool) {
	pill := ui.Row(c).Key("pill").Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).
		Radius(r.H / 2).Background(semantic.Surface.Page).
		PaddingStart(r.H * 0.4).PaddingEnd(r.H * 0.2).Gap(8 * k).AlignItems(ui.Center).
		Cursor(ui.CursorText).Role(ui.RoleSearch).Label("Search")
	var input ui.Element
	pill.Children(func() {
		for i, text := range s.field.conditions {
			s.ribbon(c, semantic, i, text, r.H-14*k, k, animate)
		}
		input = ui.TextInputBase(c.Key("pill-input"), &s.field.words).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(theme.TextDefault.Size(s.Settings.UIFontSize)).
			TextColor(semantic.Text.Heading).Placeholder("Where to, today?").Label("Search")
		input.OnTextKey(s.pillKey)
		if edit := s.field.edit; edit != nil {
			s.field.edit = nil
			// The caret goes after what was set, as it would after typing it.
			input.ReplaceText(0, math.MaxInt, *edit).SetTextSelection(math.MaxInt, math.MaxInt)
		}
		if s.field.focus && !s.OnDesk() && !s.field.hidden {
			s.field.focus = false
			input.Focus()
		}
		// A condition written whole, a space after it, becomes a ribbon.
		if strings.HasSuffix(s.field.words, " ") && strings.TrimSpace(s.field.words) != "" {
			before := len(s.field.conditions)
			s.takeConditions(s.field.words)
			if len(s.field.conditions) > before {
				s.setWords(s.field.words)
			}
		}
		s.confirmButton(c, semantic, emptyTrashKey, "", "Empty Trash", animate, func() { s.Library.EmptyTrash() })
	})
	focused := input.Focused()
	if !focused && pill.Clicked() {
		s.field.focus = true
	}
	// Pressed with the field scrolled away, the pill brings it back.
	if (pill.Pressed() || input.Pressed()) && s.field.scroll != 0 && !s.field.home {
		s.field.home = true
	}
	target := float32(0)
	if focused {
		target = 1
	}
	active := target
	if animate {
		duration := components.HoverOut
		if focused {
			duration = components.HoverIn
		}
		active = components.Follow(pill, "focus", target, duration)
	}
	pill.Border(1, components.Blend(semantic.Border.Variant, semantic.Border.Focused, active))
}

// ribbon is the query's condition at i, as text writes it: the group it
// is in, in the group's color, or what it holds in words. Pressed, it goes.
func (s *State) ribbon(c *ui.Context, semantic theme.Theme, i int, text string, h, k float32, animate bool) {
	label, ink := text, semantic.Text.Default
	var icon components.IconName
	if cond, ok := condition(text); ok {
		label = cond.Sentence()
		switch cond.Key.Name {
		case "in":
			if g, ok := s.groupNamed(cond.Value); ok && s.Library != nil {
				label, icon = g.Name, groupIcon(g)
				if tint := groupTint(semantic, g); tint != nil {
					ink = *tint
				}
			}
		case "trash":
			label, icon = "Trash", components.IconTrash
		}
	}
	btn := ui.ButtonBase(c.Key("ribbon-" + text)).Height(h).PaddingX(10 * k).Gap(6 * k).Shrink(0).
		AlignItems(ui.Center).Radius(h / 2).FocusRing(false).KeepFocus().
		Label(label + ", remove").Tooltip("Remove " + label).
		Transition(ribbonTransition(animate))
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	btn.Background(components.Blend(ink.Alpha(0.14), ink.Alpha(0.24), hover))
	if btn.Clicked() {
		s.broaden(i)
	}
	btn.Children(func() {
		if icon != "" {
			ui.Icon(c, icon.SVG()).FontSize(14 * k).TextColor(ink)
		}
		components.Label(c, label, theme.TextSmall, semantic.Text.Heading).SingleLine()
	})
}

// pillKey is what a key does in the pill's input before the input takes it.
func (s *State) pillKey(key ui.TextKey) bool {
	plain := key.Mods == 0
	switch key.Key {
	case ui.KeyEscape:
		s.escapeField()
		return true
	case ui.KeyBackspace:
		if s.field.words == "" && len(s.field.conditions) > 0 {
			s.broaden(len(s.field.conditions) - 1)
			return true
		}
	case ui.KeyEnter:
		id := s.field.chosen
		if id == "" {
			for _, row := range s.field.order {
				if strings.HasPrefix(row, "sheet:") || strings.HasPrefix(row, "command:") {
					id = row
					break
				}
			}
		}
		s.activate(id, key.Mods&(ui.Cmd|ui.Alt) != 0)
		return true
	case ui.KeyDown, ui.KeyUp:
		if plain {
			s.choose(map[ui.Key]int{ui.KeyUp: -1, ui.KeyDown: 1}[key.Key])
			return true
		}
	}
	return false
}

// ribbonTransition brings a ribbon in as it opens out, what is written
// after it moving aside with it, and takes it away as it closes up.
func ribbonTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	spring := components.Spring(components.MotionScene)
	gone := &ui.Motion{Collapse: true, Opacity: 0, Scale: 0.9}
	return ui.ElementTransition{Animation: &spring, Position: true, Colors: true, Enter: gone, Exit: gone}
}
