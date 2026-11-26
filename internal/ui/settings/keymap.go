package settings

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/app/keymap"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// The keymap lists every action by what it acts on: the pages, the
// window, the app, each a section. The query finds actions by name or by
// their keys, written or as $key names one; each row records a new
// shortcut in place, warns before taking one another action holds, and
// resets or clears its own. While any key is changed, a section puts
// every one back.

// changedKey names the section that puts every changed key back.
const changedKey = "changed"

// shortcutGroup is a group of actions, as many of them as a query finds:
// its scope's key names its section.
type shortcutGroup struct {
	key, title, description string
	shortcuts               []appsettings.Shortcut
}

// shortcutGroups are the groups of actions query finds, in order: every
// one, or the one keyed only.
func (s *State) shortcutGroups(query, only string) []shortcutGroup {
	var out []shortcutGroup
	for start := 0; start < len(s.Data.Shortcuts); {
		first := s.Data.Shortcuts[start]
		end := start + 1
		for end < len(s.Data.Shortcuts) && s.Data.Shortcuts[end].Scope == first.Scope {
			end++
		}
		if key := first.Scope.Key(); only == "" || key == only {
			group := shortcutGroup{key: key, title: first.Group, description: first.Scope.Description()}
			for _, shortcut := range s.Data.Shortcuts[start:end] {
				if s.shortcutMatches(shortcut, query) {
					group.shortcuts = append(group.shortcuts, shortcut)
				}
			}
			if len(group.shortcuts) > 0 {
				out = append(out, group)
			}
		}
		start = end
	}
	return out
}

// keymapSection draws the keymap's section sec, as query finds it.
func (s *State) keymapSection(c *ui.Context, semantic theme.Theme, sec Section, query string) {
	if sec.Key == changedKey {
		s.section(c, semantic, sec, func() { s.changedKeys(c, semantic) })
		return
	}
	for _, group := range s.shortcutGroups(query, sec.Key) {
		s.section(c, semantic, sec, func() {
			for index, shortcut := range group.shortcuts {
				s.shortcutRow(c, semantic, shortcut, index == len(group.shortcuts)-1, 0)
			}
		})
	}
}

// shortcutMatches reports whether query and the $key the query names find
// shortcut: by its name, its scope, or its keys spelled or said; the key,
// by one of its keys, spelled or said whole.
func (s *State) shortcutMatches(shortcut appsettings.Shortcut, query string) bool {
	if s.Key != "" && !slices.ContainsFunc(shortcut.Bindings, func(binding keymap.Chord) bool {
		return keyNames(binding, s.Key)
	}) {
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

// keyNames reports whether key, as a $key value, names binding: spelled
// as the Keymap spells it, or said in its words, case aside.
func keyNames(binding keymap.Chord, key string) bool {
	key = strings.TrimSpace(key)
	return strings.EqualFold(keymap.Spell(binding, keymap.PlatformCurrent), key) ||
		strings.EqualFold(keymap.Words(binding, keymap.PlatformCurrent), key)
}

// KeyNames are the keys the actions hold, spelled, each once, in the
// Keymap's order: the values $key takes.
func (s *State) KeyNames() []string {
	var out []string
	seen := map[string]bool{}
	for _, shortcut := range s.Data.Shortcuts {
		for _, binding := range shortcut.Bindings {
			if name := keymap.Spell(binding, keymap.PlatformCurrent); !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

// customized are the actions whose keys differ from the default.
func (s *State) customized() []keymap.CommandID {
	var out []keymap.CommandID
	for _, shortcut := range s.Data.Shortcuts {
		if shortcut.Overridden {
			out = append(out, shortcut.Command)
		}
	}
	return out
}

// changedKeys puts every changed key back as it was.
func (s *State) changedKeys(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	inset := theme.Space16.DIPs(scale)
	changed := s.customized()
	ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).Wrap().Gap(theme.Space08.DIPs(scale)).
		Padding(theme.Space12.DIPs(scale), inset, theme.Space12.DIPs(scale), inset).Children(func() {
		components.Label(c, locales.ResetKeysDescription(), theme.TextSmall, semantic.Text.Muted).
			Grow(1).MinWidth(180 * scale / theme.DefaultUIFontSize).MaxWidthPercent(100)
		components.Button{
			Key: "keymap-reset-all", Label: locales.ResetAll(),
			Style: components.OutlinedButton, Animate: s.motion(c),
			Action: func() {
				for _, command := range changed {
					s.apply(appsettings.ResetBinding(command))
				}
			},
		}.View(c, semantic)
	})
}

// shortcutRow is one action: its name, and its shortcut as keycaps that
// record a new one when clicked, with reset and clear actions. It records
// in place: the keys give way to the capture line in the row's own line,
// so nothing under it moves, and a key another action holds is answered
// there.
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
	recording := s.keyEditing == shortcut.Command && s.keyHost == host
	block := ui.Column(c).Key("shortcut-" + string(shortcut.Command)).
		Shrink(0).WidthPercent(100).MinWidth(0)
	hover := components.HoverAmount(block, block.Hovered() && !recording, animate)
	swap := func(key string, grow float32, content func()) {
		// The keys keep their width; the capture line takes the room the
		// name leaves it.
		part := ui.Row(c).Key(key).AlignItems(ui.Center).Gap(theme.Space12.DIPs(scale))
		if grow > 0 {
			part.Grow(grow).MinWidth(0)
		} else {
			part.Shrink(0)
		}
		if animate {
			part.Transition(keySwap)
		}
		part.Children(content)
	}
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
						s.tag(c, semantic, locales.Custom())
					}
					s.policyBadge(c, semantic, shortcut)
				})
				if recording {
					conflict := s.keyConflict
					swap("capture", 1.6, func() {
						// As tall as the keys it stands in for, so the row
						// keeps its height.
						line := s.shortcutCapture(shortcut.Command, true, conflict)
						line.Height = keysHeight(scale)
						s.captureLine(c, semantic, line)
					})
					return
				}
				swap("keys", 0, func() {
					s.shortcutActions(c, semantic, shortcut, hover)
					s.shortcutKeys(c, semantic, shortcut, host)
				})
			})
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
				Key: "clear-" + string(command), Label: locales.ClearShortcut(),
				Icon: components.IconClose.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
				IconColor: semantic.Icon.Muted,
				Action:    func() { s.apply(appsettings.Unbind(command)) },
			}.View(c, semantic).Opacity(hover)
		}
		if shortcut.Overridden {
			components.IconButton{
				Key: "reset-" + string(command), Label: locales.ResetShortcut(),
				Icon: components.IconUndo.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
				IconColor: semantic.Icon.Muted,
				Action:    func() { s.apply(appsettings.ResetBinding(command)) },
			}.View(c, semantic)
		}
	})
}

// shortcutKeys shows the bindings as keycaps; clicking them records anew,
// in their place (shortcutRow).
func (s *State) shortcutKeys(c *ui.Context, semantic theme.Theme, shortcut appsettings.Shortcut, host string) {
	scale := s.Theme.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	button := ui.ButtonBase(c.Key("keys-" + string(shortcut.Command))).
		Label(shortcutLabel(shortcut, shortcut.Command)).
		FocusRing(false).
		Height(keysHeight(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		PaddingX(theme.Space04.DIPs(scale)).Radius(metrics.Radius)
	// The keys draw their own ring, the one a key in a sentence draws, for
	// a focus the keyboard brought; given back after recording, quietly.
	ring := s.keyRing(button, triggerKey(host, shortcut.Command), false, s.motion(c))
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
				components.CenteredLabel(c, locales.AddShortcut(), theme.TextSmall, semantic.Text.Muted)
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

// keysHeight is how tall a row's keys stand, and the capture line that
// records in their place.
func keysHeight(scale float32) float32 {
	return theme.DefaultControls.Scaled(scale).InlineHeight + theme.Space04.DIPs(scale)
}

// keycaps draws one binding as a run of keys.
func (s *State) keycaps(c *ui.Context, semantic theme.Theme, chord keymap.Chord, compact bool) {
	components.Keycaps(c,
		components.KeycapStyle(semantic, s.Theme.UIFontSize, compact),
		keymap.Keycaps(chord, keymap.PlatformCurrent),
		keymap.Spell(chord, keymap.PlatformCurrent))
}

var (
	keySpring = components.Settle(components.MotionNormal)
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
	components.BadgeFrame(tag, scale).
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
		Role(ui.RoleButton).Label(locales.OpenPolicy(title)).
		Focusable().FocusRing(false).Cursor(ui.CursorPointer)
	hover := components.HoverAmount(badge, badge.Hovered(), s.motion(c))
	components.BadgeFrame(badge, scale).
		Background(components.Blend(semantic.Surface.Inset, semantic.Accent.Hint, hover)).Children(func() {
		components.CenteredLabel(c, title, theme.TextXSmall, semantic.Text.Muted.Mix(semantic.Accent.Foreground, hover))
	})
	if badge.Clicked() && s.Find != nil {
		s.Find(title)
	}
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
