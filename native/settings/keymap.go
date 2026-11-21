package settings

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The keymap lists every action by what it acts on: the pages, the
// window, the app, each a section. The words written find actions by name
// or by their keys, and a section of its own finds them by pressing a
// shortcut, or puts every key back; each row records a new shortcut, warns
// before taking one another action holds, and resets or clears its own.

// findShortcut is the title of the section that searches by keys.
const findShortcut = "Find a Shortcut"

// shortcutGroup is a group of actions, as many of them as a query finds.
type shortcutGroup struct {
	title, description string
	shortcuts          []appsettings.Shortcut
}

// shortcutGroups are the groups of actions query finds, in order.
func (s *State) shortcutGroups(query string) []shortcutGroup {
	var out []shortcutGroup
	for start := 0; start < len(s.Data.Shortcuts); {
		first := s.Data.Shortcuts[start]
		end := start + 1
		for end < len(s.Data.Shortcuts) && s.Data.Shortcuts[end].Group == first.Group {
			end++
		}
		group := shortcutGroup{title: first.Group, description: first.Scope.Description()}
		for _, shortcut := range s.Data.Shortcuts[start:end] {
			if s.shortcutMatches(shortcut, query) {
				group.shortcuts = append(group.shortcuts, shortcut)
			}
		}
		if len(group.shortcuts) > 0 {
			out = append(out, group)
		}
		start = end
	}
	return out
}

// keymapSection draws the keymap's section sec, as query finds it.
func (s *State) keymapSection(c *ui.Context, semantic theme.Theme, sec Section, query string) {
	if sec.Title == findShortcut {
		s.section(c, semantic, sec, func() { s.keymapFinder(c, semantic) })
		return
	}
	for _, group := range s.shortcutGroups(query) {
		if group.title == sec.Title {
			s.section(c, semantic, sec, func() {
				for index, shortcut := range group.shortcuts {
					s.shortcutRow(c, semantic, shortcut, index == len(group.shortcuts)-1, 0)
				}
			})
			return
		}
	}
}

// shortcutMatches reports whether query and the chord searched by find
// shortcut: by its name, its scope, or its keys spelled or said.
func (s *State) shortcutMatches(shortcut appsettings.Shortcut, query string) bool {
	if s.keyFilter != nil && !slices.Contains(shortcut.Bindings, *s.keyFilter) {
		return false
	}
	if query == "" {
		return true
	}
	haystack := shortcutWords(shortcut)
	for _, binding := range shortcut.Bindings {
		haystack += " " + keymap.Spell(binding, keymap.PlatformCurrent) +
			" " + keymap.Words(binding, keymap.PlatformCurrent)
	}
	return strings.Contains(strings.ToLower(haystack), query)
}

// shortcutWords is what a search by name matches: the action and its scope.
func shortcutWords(shortcut appsettings.Shortcut) string {
	return shortcut.Label + " " + shortcut.Group
}

// keymapFinder searches the keymap by a keystroke pressed, and puts every
// key changed back.
func (s *State) keymapFinder(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	inset := theme.Space16.DIPs(scale)
	ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
		Padding(theme.Space12.DIPs(scale), inset, theme.Space12.DIPs(scale), inset).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			switch {
			case s.keySearch:
				s.captureLine(c, semantic, captureLine{
					Key: "keymap-search-capture", Prompt: "Press a shortcut to find it",
					Live: true, Compact: true,
					Chosen: func(chord keymap.Chord) {
						s.keyFilter, s.keySearch = &chord, false
					},
					Cancel: func() { s.keySearch = false },
				})
			case s.keyFilter != nil:
				s.keyFilterChip(c, semantic)
			default:
				components.Label(c, "Press a shortcut to find its action.", theme.TextSmall, semantic.Text.Muted)
			}
		})
		components.IconButton{
			Key: "keymap-record-search", Label: "Search by pressing a shortcut",
			Icon: components.IconKeyboard.SVG(), Style: components.OutlinedButton,
			Selected: s.keySearch || s.keyFilter != nil, Animate: animate,
			Action: func() {
				s.keySearch, s.keyFilter = !s.keySearch, nil
			},
		}.View(c, semantic)
		customized := make([]keymap.CommandID, 0)
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
			Size: components.CompactButton, IconColor: semantic.Icon.Muted,
			Animate: s.motion(c),
			Action:  func() { s.keyFilter = nil },
		}.View(c, semantic)
	})
}

// shortcutRow is one action: its name, and its shortcut as keycaps that
// record a new one when clicked, with reset and clear actions. The recorder
// opens under the row.
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
	hover := components.HoverAmount(block, block.Hovered(), animate)
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
					if shortcut.Overridden {
						s.tag(c, semantic, "Custom")
					}
					s.policyBadge(c, semantic, shortcut)
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

// shortcutActions resets a customized shortcut, always shown while it
// differs from the default, and clears one, shown as the row is hovered.
func (s *State) shortcutActions(
	c *ui.Context,
	semantic theme.Theme,
	shortcut appsettings.Shortcut,
	hover float32,
) {
	animate := s.motion(c)
	command := shortcut.Command
	ui.Row(c).AlignItems(ui.Center).Gap(theme.Space02.DIPs(s.Theme.UIFontSize)).Children(func() {
		if len(shortcut.Bindings) > 0 {
			components.IconButton{
				Key: "clear-" + string(command), Label: "Clear Shortcut",
				Icon: components.IconClose.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
				IconColor: semantic.Icon.Muted,
				Action:    func() { s.apply(appsettings.Unbind(command)) },
			}.View(c, semantic).Opacity(hover)
		}
		if shortcut.Overridden {
			components.IconButton{
				Key: "reset-" + string(command), Label: "Reset Shortcut",
				Icon: components.IconUndo.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
				IconColor: semantic.Icon.Muted,
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
		FocusRing(false).
		Height(metrics.InlineHeight + theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		PaddingX(theme.Space04.DIPs(scale)).Radius(metrics.Radius)
	// The keys draw their own ring, the one a key in a sentence draws, so
	// it eases away when recording ends rather than lingering as focus.
	ring := s.keyRing(button, triggerKey(host, shortcut.Command), recording, s.motion(c))
	button.Border(1, semantic.Border.Focused.Alpha(ring))
	hover := components.HoverAmount(button, button.Hovered(), s.motion(c))
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
				components.CenteredLabel(c, "Add shortcut", theme.TextSmall, semantic.Text.Muted)
			})
			return
		}
		for _, binding := range shortcut.Bindings {
			swap(binding.String(), func() { s.keycaps(c, semantic, binding, false) })
		}
	})
	if button.Clicked() {
		s.startRecording(shortcut.Command, host)
	}
	s.returnFocus(button, triggerKey(host, shortcut.Command))
}

// keycaps draws one binding as a run of keys.
func (s *State) keycaps(c *ui.Context, semantic theme.Theme, chord keymap.Chord, compact bool) {
	components.Keycaps(c,
		components.KeycapStyle(semantic, s.Theme.UIFontSize, compact),
		keymap.Keycaps(chord, keymap.PlatformCurrent),
		keymap.Spell(chord, keymap.PlatformCurrent))
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
	settingsTagFrame(tag, scale).
		Background(semantic.Surface.Inset).Children(func() {
		components.CenteredLabel(c, text, theme.TextXSmall, semantic.Text.Muted)
	})
}

// policyBadge names the policy declaring an editing command, and finds
// its card.
func (s *State) policyBadge(c *ui.Context, semantic theme.Theme, shortcut appsettings.Shortcut) {
	title := ""
	for _, card := range s.Data.Policies {
		if card.ID == shortcut.Policy {
			title = card.Title
		}
	}
	if title == "" {
		return
	}
	scale := s.Theme.UIFontSize
	badge := ui.Row(c).Key("policy-" + string(shortcut.Command)).
		Role(ui.RoleButton).Label("Open the " + title + " policy").
		Focusable().FocusRing(false).Cursor(ui.CursorPointer)
	hover := components.HoverAmount(badge, badge.Hovered(), s.motion(c))
	settingsTagFrame(badge, scale).
		Background(components.Blend(semantic.Surface.Inset, semantic.Accent.Hint, hover)).Children(func() {
		components.CenteredLabel(c, title, theme.TextXSmall, semantic.Text.Muted.Mix(semantic.Accent.Foreground, hover))
	})
	if badge.Clicked() && s.Find != nil {
		s.Find(title)
	}
}

// settingsTagFrame is a state tag's compact geometry.
func settingsTagFrame(tag ui.Element, scale float32) ui.Element {
	return tag.Shrink(0).Height(18 * scale / theme.DefaultUIFontSize).AlignItems(ui.Center).
		PaddingX(theme.Space06.DIPs(scale)).Radius(9 * scale / theme.DefaultUIFontSize)
}

// recordShortcut assigns chord to command, unless another action holds it
// or the system keeps it: then the recording stays open and its line says
// so, for the person to replace, try another chord or give up.
func (s *State) recordShortcut(command keymap.CommandID, chord keymap.Chord) {
	if s.Reserved != nil {
		if label, reserved := s.Reserved(chord); reserved {
			s.keyConflict = &keyConflict{
				Command: command, Target: chord, Reserved: true,
				Owner: appsettings.Shortcut{Label: label},
			}
			return
		}
	}
	owner, taken := s.Data.Owner(chord)
	if taken && owner.Command != command {
		s.keyConflict = &keyConflict{Command: command, Target: chord, Owner: owner}
		return
	}
	s.stopRecording()
	if !taken {
		s.apply(appsettings.Remap(command, chord))
	}
}
