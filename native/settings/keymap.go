package settings

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// keymapPage lists every action by what it acts on: the page being written,
// the pages, the window, the app. A row names the feature that adds its
// action, when a feature does, and that name leads to the feature's Editor
// card, where the same key is shown and edited. A toolbar searches by name
// or by pressing a shortcut; each row records a new shortcut, warns before
// taking one another action holds, and resets or clears its own.
func (s *State) keymapPage(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	s.keymapToolbar(c, semantic)
	railQuery := strings.ToLower(strings.TrimSpace(s.Query))
	keyQuery := strings.ToLower(strings.TrimSpace(s.keyQuery))
	start := 0
	for start < len(s.Data.Shortcuts) {
		first := s.Data.Shortcuts[start]
		end := start + 1
		for end < len(s.Data.Shortcuts) && s.Data.Shortcuts[end].Group == first.Group {
			end++
		}
		filtered := make([]appsettings.Shortcut, 0, end-start)
		for _, shortcut := range s.Data.Shortcuts[start:end] {
			if s.shortcutMatches(shortcut, railQuery, keyQuery) {
				filtered = append(filtered, shortcut)
			}
		}
		if len(filtered) > 0 {
			s.section(c, semantic, first.Group, appsettings.ScopeDescription(first.Scope), func() {
				for index, shortcut := range filtered {
					s.shortcutRow(c, semantic, shortcut, index == len(filtered)-1, 0)
				}
			})
		}
		start = end
	}
	_ = metrics
}

func (s *State) shortcutMatches(shortcut appsettings.Shortcut, railQuery, keyQuery string) bool {
	if railQuery != "" && !strings.Contains(strings.ToLower(shortcutWords(shortcut)), railQuery) {
		return false
	}
	if s.keyFilter != nil {
		found := false
		for _, binding := range shortcut.Bindings {
			found = found || binding == *s.keyFilter
		}
		if !found {
			return false
		}
	}
	if keyQuery == "" {
		return true
	}
	haystack := shortcutWords(shortcut)
	for _, binding := range shortcut.Bindings {
		haystack += " " + keymap.Chord(binding, keymap.PlatformCurrent) +
			" " + keymap.ChordWords(binding, keymap.PlatformCurrent)
	}
	return strings.Contains(strings.ToLower(haystack), keyQuery)
}

// shortcutWords is what a search by name matches: the action, its scope and
// the feature that adds it.
func shortcutWords(shortcut appsettings.Shortcut) string {
	return shortcut.Label + " " + shortcut.Group + " " + shortcut.OriginLabel
}

// keymapToolbar searches the list by name, or by a recorded keystroke.
func (s *State) keymapToolbar(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	ui.Row(c).WidthPercent(100).AlignItems(ui.Center).
		Gap(theme.Space08.DIPs(scale)).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			switch {
			case s.keySearch:
				s.keyCapture(c, semantic, "keymap-search-capture", "Press a shortcut to find it", true,
					func(target kernel.BindingTarget) {
						s.keyFilter, s.keySearch = &target, false
					},
					func() { s.keySearch = false },
					nil,
				)
			case s.keyFilter != nil:
				s.keyFilterChip(c, semantic)
			default:
				components.TextField{
					Key: "keymap-search", Label: "Search shortcuts",
					Placeholder: "Search actions or keys…", Value: &s.keyQuery,
					Icon: components.IconSearch.SVG(), ClearIcon: components.IconClose.SVG(),
					Animate: animate,
				}.View(c, semantic)
			}
		})
		components.IconButton{
			Key: "keymap-record-search", Label: "Search by pressing a shortcut",
			Icon: components.IconKeyboard.SVG(), Style: components.OutlinedButton,
			Selected: s.keySearch || s.keyFilter != nil, Animate: animate,
			Action: func() {
				s.keySearch, s.keyFilter = !s.keySearch, nil
				s.keyQuery = ""
			},
		}.View(c, semantic)
		customized := make([]kernel.CommandID, 0)
		for _, shortcut := range s.Data.Shortcuts {
			if shortcut.Overridden {
				customized = append(customized, shortcut.Command)
			}
		}
		if len(customized) > 0 {
			components.Button{
				Key: "keymap-reset-all", Label: "Reset all",
				Style: components.OutlinedButton, Animate: animate,
				Action: func() {
					for _, command := range customized {
						s.apply(appsettings.ResetBinding(command))
					}
				},
			}.View(c, semantic)
		}
	})
}

// keyFilterChip shows the recorded search chord in place of the field.
func (s *State) keyFilterChip(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	ui.Row(c).Height(metrics.Height).WidthPercent(100).AlignItems(ui.Center).
		Gap(theme.Space06.DIPs(scale)).
		Padding(0, theme.Space04.DIPs(scale), 0, theme.Space08.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Inset).Children(func() {
		ui.Icon(c, components.IconKeyboard.SVG()).
			FontSize(16 * scale / theme.DefaultUIFontSize).TextColor(semantic.Icon.Muted)
		components.Label(c, "Shortcuts on", theme.TextSmall, semantic.Text.Muted).SingleLine()
		s.keycaps(c, semantic, *s.keyFilter, true)
		ui.Box(c).Grow(1)
		components.IconButton{
			Key: "keymap-clear-filter", Label: "Clear shortcut search",
			Icon: components.IconClose.SVG(), Style: components.SubtleButton,
			Size: components.CompactButton, IconColor: &semantic.Icon.Muted,
			Animate: s.motion(c),
			Action:  func() { s.keyFilter = nil },
		}.View(c, semantic)
	})
}

// keyCapture takes the next shortcut pressed. It focuses itself when it
// appears; Escape, or focus moving elsewhere, cancels; a bare Backspace or
// Delete clears, when clearing applies. A capture that is not live is the
// picture of one, as a closing recorder shows it: it takes no focus or keys.
func (s *State) keyCapture(
	c *ui.Context,
	semantic theme.Theme,
	key string,
	prompt string,
	live bool,
	chosen func(kernel.BindingTarget),
	cancel func(),
	clear func(),
) {
	scale := s.Theme.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	capture := ui.Row(c).Key(key).Role(ui.RoleTextField).Label(prompt).
		Height(metrics.Height).MinWidth(0).AlignItems(ui.Center).
		Gap(theme.Space06.DIPs(scale)).PaddingX(theme.Space08.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Focused).
		Background(semantic.Surface.Inset)
	if live {
		liveCapture(capture, chosen, cancel, clear)
	}
	capture.Children(func() {
		ui.Icon(c, components.IconKeyboard.SVG()).
			FontSize(16 * scale / theme.DefaultUIFontSize).TextColor(semantic.Icon.Accent)
		components.Label(c, prompt, theme.TextSmall, semantic.Text.Default).
			Grow(1).MinWidth(0).SingleLine()
		hint := "Esc to cancel"
		if clear != nil {
			hint = "Esc cancels · ⌫ clears"
		}
		components.Label(c, hint, theme.TextXSmall, semantic.Text.Muted).SingleLine()
	})
}

func liveCapture(
	capture ui.Element,
	chosen func(kernel.BindingTarget),
	cancel func(),
	clear func(),
) {
	capture.Focusable().FocusRing(false).AutoFocus()
	held := ui.Local(capture, "held", func() bool { return false })
	if *held && !capture.Focused() {
		cancel()
	}
	*held = capture.Focused()
	capture.HandleInput(func(event ui.InputEvent) bool {
		if event.Kind != ui.InputKeyDown {
			return false
		}
		if event.Mods == 0 && event.Key == ui.KeyEscape {
			cancel()
			return true
		}
		if event.Mods == 0 && clear != nil &&
			(event.Key == ui.KeyBackspace || event.Key == ui.KeyDelete) {
			clear()
			return true
		}
		if targets := keymap.BindingTargets(event, keymap.PlatformCurrent); len(targets) > 0 {
			chosen(targets[0])
		}
		// A bare modifier waits for the key it modifies.
		return true
	})
}

// shortcutRow is one action: its name, the feature that adds it, and its
// shortcut as keycaps that record a new one when clicked, with reset and
// clear actions. The recorder opens under the row, as it does under a
// sentence on the Editor page. Indent nests the row under a policy card's
// header, level with the card's options, where the feature is the card's own
// and goes unnamed.
func (s *State) shortcutRow(
	c *ui.Context,
	semantic theme.Theme,
	shortcut appsettings.Shortcut,
	last bool,
	indent float32,
) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	inset := theme.Space16.DIPs(scale)
	start := inset + indent
	host := "row-" + string(shortcut.Command)
	block := ui.Column(c).Key("shortcut-" + string(shortcut.Command)).
		Shrink(0).WidthPercent(100).MinWidth(0)
	if !shortcut.Enabled {
		block.Opacity(0.55)
	}
	hover := components.HoverAmount(block, block.Hovered() && shortcut.Enabled, animate)
	block.Children(func() {
		ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).
			MinHeight(theme.DefaultControls.Scaled(scale).Row+theme.Space08.DIPs(scale)).
			Gap(theme.Space12.DIPs(scale)).
			Padding(theme.Space08.DIPs(scale), inset, theme.Space08.DIPs(scale), start).
			Children(func() {
				ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).
					Gap(theme.Space08.DIPs(scale)).Children(func() {
					components.Label(c, shortcut.Label, theme.TextDefault, semantic.Text.Default).
						MinWidth(0).SingleLine()
					if shortcut.Origin != "" && indent == 0 {
						s.originTag(c, semantic, shortcut)
					}
					switch {
					case !shortcut.Enabled:
						s.tag(c, semantic, "Off")
					case shortcut.Overridden:
						s.tag(c, semantic, "Custom")
					}
				})
				s.shortcutActions(c, semantic, shortcut, hover)
				s.shortcutKeys(c, semantic, shortcut, host)
			})
		s.recorder(c, semantic, host, start)
		if !last {
			ui.Row(c).WidthPercent(100).PaddingStart(start).Children(func() {
				components.HorizontalDivider(c, semantic, components.VariantDivider)
			})
		}
	})
}

// originTag names the feature that adds an action and leads to its card on
// the Editor page. It stays legible while the feature is off, as that is
// where it is turned back on.
func (s *State) originTag(c *ui.Context, semantic theme.Theme, shortcut appsettings.Shortcut) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	origin := shortcut.OriginLabel
	tag := ui.ButtonBase(c.Key("origin-"+string(shortcut.Command))).
		Label("From "+origin+": show in Editor settings").Tooltip("Added by "+origin).
		Shrink(0).Height(18*scale/theme.DefaultUIFontSize).AlignItems(ui.Center).
		Gap(theme.Space02.DIPs(scale)).
		PaddingX(theme.Space06.DIPs(scale)).Radius(9*scale/theme.DefaultUIFontSize).
		Border(1, semantic.Border.Variant)
	hover := components.HoverAmount(tag, tag.Hovered(), animate)
	tag.Background(semantic.GhostElement.Hover.Alpha(hover))
	tag.Children(func() {
		components.Label(c, origin, theme.TextXSmall, semantic.Text.Muted).SingleLine()
		ui.Icon(c, components.IconChevronRight.SVG()).
			FontSize(11 * scale / theme.DefaultUIFontSize).TextColor(semantic.Icon.Muted)
	})
	if tag.Clicked() {
		s.chooseSection(appsettings.Editor, origin)
	}
}

// shortcutActions resets a customized shortcut, always shown while it
// differs from the default, and clears one, shown as the row is hovered.
func (s *State) shortcutActions(
	c *ui.Context,
	semantic theme.Theme,
	shortcut appsettings.Shortcut,
	hover float32,
) {
	if !shortcut.Enabled {
		return
	}
	animate := s.motion(c)
	command := shortcut.Command
	ui.Row(c).AlignItems(ui.Center).Gap(theme.Space02.DIPs(s.Theme.UIFontSize)).Children(func() {
		if len(shortcut.Bindings) > 0 {
			components.IconButton{
				Key: "clear-" + string(command), Label: "Clear Shortcut",
				Icon: components.IconClose.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
				IconColor: &semantic.Icon.Muted,
				Action:    func() { s.apply(appsettings.Unbind(command)) },
			}.View(c, semantic).Opacity(hover)
		}
		if shortcut.Overridden {
			components.IconButton{
				Key: "reset-" + string(command), Label: "Reset Shortcut",
				Icon: components.IconUndo.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
				IconColor: &semantic.Icon.Muted,
				Action:    func() { s.apply(appsettings.ResetBinding(command)) },
			}.View(c, semantic)
		}
	})
}

// shortcutKeys shows the bindings as keycaps; clicking them records anew,
// and they stay marked while their recorder is open.
func (s *State) shortcutKeys(c *ui.Context, semantic theme.Theme, shortcut appsettings.Shortcut, host string) {
	scale := s.Theme.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	recording := s.keyEditing == shortcut.Command && s.keyHost == host
	edge := ui.Transparent
	if recording {
		edge = semantic.Border.Focused
	}
	button := ui.ButtonBase(c.Key("keys-"+string(shortcut.Command))).
		Label(shortcutLabel(shortcut, shortcut.Command)).
		Disabled(!shortcut.Enabled).
		Height(metrics.InlineHeight+theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		PaddingX(theme.Space04.DIPs(scale)).Radius(metrics.Radius).
		Border(1, edge)
	hover := components.HoverAmount(button, button.Hovered() && shortcut.Enabled, s.motion(c))
	button.Background(semantic.GhostElement.Hover.Alpha(hover))
	animate := s.motion(c)
	if animate {
		// The button eases to the width of what it shows while the old
		// keys lift out and the new ones settle in.
		button.Transition(keyTransition)
	}
	swap := func(key string, content func()) {
		keys := ui.Row(c).Key(key).AlignItems(ui.Center)
		if animate {
			keys.Transition(keySwap)
		}
		keys.Children(content)
	}
	button.Children(func() {
		if len(shortcut.Bindings) == 0 {
			swap("unbound", func() {
				components.Label(c, "Add shortcut", theme.TextSmall, semantic.Text.Muted).SingleLine()
			})
			return
		}
		for _, binding := range shortcut.Bindings {
			swap(binding.String(), func() { s.keycaps(c, semantic, binding, false) })
		}
	})
	if button.Clicked() && shortcut.Enabled {
		s.startRecording(shortcut.Command, host)
	}
}

// keycaps draws one binding as a run of keys.
func (s *State) keycaps(c *ui.Context, semantic theme.Theme, target kernel.BindingTarget, compact bool) {
	scale := s.Theme.UIFontSize
	ui.Row(c).AlignItems(ui.Center).Gap(theme.Space02.DIPs(scale)).
		Label(keymap.Chord(target, keymap.PlatformCurrent)).Children(func() {
		for _, key := range keymap.Keycaps(target, keymap.PlatformCurrent) {
			height := 22 * scale / theme.DefaultUIFontSize
			face := semantic.Surface.Inset
			if compact {
				// Inside an inset field the caps sit on the page surface.
				height = 20 * scale / theme.DefaultUIFontSize
				face = semantic.Surface.Page
			}
			cap := ui.Row(c).Height(height).Justify(ui.Center).AlignItems(ui.Center).
				Radius(5*scale/theme.DefaultUIFontSize).
				Border(1, semantic.Border.Variant).Background(face)
			if utf8.RuneCountInString(key) == 1 {
				// One glyph: a square cap, so a row of caps is even.
				cap.Width(height)
			} else {
				cap.MinWidth(height).PaddingX(theme.Space06.DIPs(scale))
			}
			cap.Children(func() {
				// The UI face's ink sits half a DIP below its line box;
				// lift it so glyphs center optically in the cap.
				components.Label(c, key, theme.TextXSmall, semantic.Text.Default).
					SingleLine().FontFeatures("tnum").
					Translate(0, -0.5*scale/theme.DefaultUIFontSize)
			})
		}
	})
}

var (
	keySpring = components.Spring(components.MotionNormal)
	// keyTransition moves a run of keys to its new width and colors.
	keyTransition = ui.ElementTransition{Animation: &keySpring, Size: true, Colors: true}
	// keySwap replaces one key with another: the old one lifts out as the
	// new one rises into its place.
	keySwap = ui.ElementTransition{
		Animation: &keySpring,
		Enter:     &ui.Motion{Y: 4, Opacity: 0},
		Exit:      &ui.Motion{Y: -4, Opacity: 0},
	}
	// tagFade brings a state tag in and out in place.
	tagFade = ui.ElementTransition{
		Animation: &keySpring,
		Enter:     &ui.Motion{Scale: 0.9, Opacity: 0},
		Exit:      &ui.Motion{Scale: 0.9, Opacity: 0},
	}
)

func (s *State) tag(c *ui.Context, semantic theme.Theme, text string) {
	scale := s.Theme.UIFontSize
	tag := ui.Row(c).Key("tag-" + text)
	if s.Animate {
		tag.Transition(tagFade)
	}
	tag.Shrink(0).Height(18 * scale / theme.DefaultUIFontSize).AlignItems(ui.Center).
		PaddingX(theme.Space06.DIPs(scale)).Radius(9 * scale / theme.DefaultUIFontSize).
		Background(semantic.Surface.Inset).Children(func() {
		components.Label(c, text, theme.TextXSmall, semantic.Text.Muted).SingleLine()
	})
}

// recordShortcut assigns target to command, or holds it for a decision, at
// host, when another action already uses it.
func (s *State) recordShortcut(command kernel.CommandID, target kernel.BindingTarget, host string) {
	s.stopRecording()
	if owner, taken := s.Data.Owner(target); taken {
		if owner.Command != command {
			s.keyConflict = &keyConflict{Command: command, Target: target, Owner: owner, Host: host}
		}
		return
	}
	s.keyConflict = nil
	s.apply(appsettings.Remap(command, target))
}

// conflictRow asks before taking a shortcut another action holds.
func (s *State) conflictRow(c *ui.Context, semantic theme.Theme, conflict keyConflict, start float32) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	inset := theme.Space16.DIPs(scale)
	ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).Wrap().
		Gap(theme.Space08.DIPs(scale)).
		Padding(0, inset, theme.Space12.DIPs(scale), start).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).
			Gap(theme.Space06.DIPs(scale)).Children(func() {
			ui.Icon(c, components.IconWarning.SVG()).
				FontSize(14 * scale / theme.DefaultUIFontSize).TextColor(semantic.Status.Warning.Foreground)
			components.Label(c, fmt.Sprintf("%s is assigned to “%s”.",
				keymap.Chord(conflict.Target, keymap.PlatformCurrent), conflict.Owner.Label),
				theme.TextSmall, semantic.Text.Default).MinWidth(0)
		})
		components.Button{
			Key: "conflict-cancel", Label: "Cancel", Style: components.SubtleButton,
			Size: components.CompactButton, Animate: animate,
			Action: func() {
				if s.keyConflict != nil {
					s.keyConflict = nil
				}
			},
		}.View(c, semantic)
		components.Button{
			Key: "conflict-replace", Label: "Replace", Style: components.PrimaryButton,
			Size: components.CompactButton, Animate: animate,
			Action: func() {
				// A closing prompt still shows its buttons, but its
				// decision has been made.
				if s.keyConflict == nil {
					return
				}
				s.keyConflict = nil
				s.apply(appsettings.RemapReplacing(conflict.Command, conflict.Target, conflict.Owner.Command))
			},
		}.View(c, semantic)
	})
}
