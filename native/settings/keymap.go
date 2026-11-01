package settings

import (
	"strings"

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
				s.captureLine(c, semantic, captureLine{
					Key: "keymap-search-capture", Prompt: "Press a shortcut to find it",
					Live: true, Compact: true,
					Chosen: func(target kernel.BindingTarget) {
						s.keyFilter, s.keySearch = &target, false
					},
					Cancel: func() { s.keySearch = false },
				})
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
	button := ui.ButtonBase(c.Key("keys-" + string(shortcut.Command))).
		Label(shortcutLabel(shortcut, shortcut.Command)).
		Disabled(!shortcut.Enabled).FocusRing(false).
		Height(metrics.InlineHeight + theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		PaddingX(theme.Space04.DIPs(scale)).Radius(metrics.Radius)
	// The keys draw their own ring, the one a key in a sentence draws, so
	// it eases away when recording ends rather than lingering as focus.
	ring := s.keyRing(button, triggerKey(host, shortcut.Command), recording, s.motion(c))
	button.Border(1, semantic.Border.Focused.Alpha(ring))
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
	s.returnFocus(button, triggerKey(host, shortcut.Command))
}

// keycaps draws one binding as a run of keys.
func (s *State) keycaps(c *ui.Context, semantic theme.Theme, target kernel.BindingTarget, compact bool) {
	components.Keycaps(c,
		components.KeycapStyle(semantic, s.Theme.UIFontSize, compact),
		keymap.Keycaps(target, keymap.PlatformCurrent),
		keymap.Chord(target, keymap.PlatformCurrent))
}

var (
	keySpring = components.Spring(components.MotionNormal)
	// keyTransition moves a run of keys to its new width; their ring eases
	// on its own spring (keyRing).
	keyTransition = ui.ElementTransition{Animation: &keySpring, Size: true}
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
	if s.motion(c) {
		tag.Transition(tagFade)
	}
	tag.Shrink(0).Height(18 * scale / theme.DefaultUIFontSize).AlignItems(ui.Center).
		PaddingX(theme.Space06.DIPs(scale)).Radius(9 * scale / theme.DefaultUIFontSize).
		Background(semantic.Surface.Inset).Children(func() {
		components.Label(c, text, theme.TextXSmall, semantic.Text.Muted).SingleLine()
	})
}

// recordShortcut assigns target to command, unless another action holds it
// or the system keeps it: then the recording stays open and its line says
// so, for the person to replace, try another chord or give up.
func (s *State) recordShortcut(command kernel.CommandID, target kernel.BindingTarget) {
	if s.Reserved != nil {
		if label, reserved := s.Reserved(target); reserved {
			s.keyConflict = &keyConflict{
				Command: command, Target: target, Reserved: true,
				Owner: appsettings.Shortcut{Label: label},
			}
			return
		}
	}
	owner, taken := s.Data.Owner(target)
	if taken && owner.Command != command {
		s.keyConflict = &keyConflict{Command: command, Target: target, Owner: owner}
		return
	}
	s.stopRecording()
	if !taken {
		s.apply(appsettings.Remap(command, target))
	}
}
