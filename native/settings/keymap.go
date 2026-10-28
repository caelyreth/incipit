package settings

import (
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) keymapPage(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	start := 0
	for start < len(s.Data.Shortcuts) {
		first := s.Data.Shortcuts[start]
		end := start + 1
		for end < len(s.Data.Shortcuts) && s.Data.Shortcuts[end].Group == first.Group {
			end++
		}
		group := s.Data.Shortcuts[start:end]
		filtered := group[:0]
		for _, shortcut := range group {
			if query == "" ||
				strings.Contains(strings.ToLower(shortcut.Label+" "+shortcut.Group), query) {
				filtered = append(filtered, shortcut)
			}
		}
		if len(filtered) > 0 {
			s.section(c, semantic, first.Group, "", func() {
				for index, shortcut := range filtered {
					shortcut := shortcut
					components.SettingRow{
						Label: shortcut.Label, Last: index == len(filtered)-1,
						Control: func() { s.shortcutControl(c, semantic, shortcut) },
					}.View(c, semantic)
				}
			})
		}
		start = end
	}
	_ = metrics
}

func (s *State) shortcutControl(
	c *ui.Context,
	semantic theme.Theme,
	shortcut appsettings.Shortcut,
) {
	scale := s.Theme.UIFontSize
	display := "Unassigned"
	if len(shortcut.Bindings) > 0 {
		display = shortcut.Bindings[0].String()
	}
	if !shortcut.Enabled {
		display = "Policy off"
	}
	ui.Row(c).AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Children(func() {
		components.Label(c, display, theme.TextSmall, semantic.Text.Muted).
			SingleLine().FontFeatures("tnum")
		if shortcut.Enabled && len(shortcut.Bindings) > 0 {
			components.IconButton{
				Key: "clear-" + string(shortcut.Command), Label: "Clear Shortcut",
				Icon: components.IconClose.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: s.motion(c),
				IconColor: &semantic.Icon.Muted,
				Action: func() {
					s.apply(appsettings.Unbind(shortcut.Command))
				},
			}.View(c, semantic)
		}
		if shortcut.Overridden && shortcut.Enabled {
			components.IconButton{
				Key: "reset-" + string(shortcut.Command), Label: "Reset Shortcut",
				Icon: components.IconUndo.SVG(), Style: components.SubtleButton,
				Size: components.CompactButton, Animate: s.motion(c),
				IconColor: &semantic.Icon.Muted,
				Action: func() {
					s.apply(appsettings.ResetBinding(shortcut.Command))
				},
			}.View(c, semantic)
		}
	})
}
