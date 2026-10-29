package overlay

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (p *CommandPalette) View(c *ui.Context, semantic theme.Theme, animate bool) {
	if p == nil || !p.Open {
		return
	}
	open := p.Open
	items := p.Filtered()
	if p.Selected >= len(items) {
		p.Selected = max(0, len(items)-1)
	}
	if c.Shortcut(0, ui.KeyDown) {
		p.Move(1)
	}
	if c.Shortcut(0, ui.KeyUp) {
		p.Move(-1)
	}
	scale := semanticScale(c)
	components.Dialog{
		Key: "command-palette", Label: "Commands", Width: 560, Animate: animate,
		Content: func(panel ui.Element) {
			panel.Padding(16 * scale).Children(func() {
				ui.Column(c).Gap(8 * scale).Children(func() {
					components.TextField{
						Key: "command-query", Label: "Search commands", Placeholder: "Search commands…",
						Value: &p.Query, Icon: components.IconSearch.SVG(),
						ClearIcon: components.IconClose.SVG(), Animate: animate, AutoFocus: true,
						Changed: func() { p.Selected = 0 },
						Submit:  func() { p.Submit() },
					}.View(c, semantic)
					components.HorizontalDivider(c, semantic, components.VariantDivider)
					if len(items) == 0 {
						components.Label(c, "No commands match", theme.TextSmall,
							semantic.Text.Muted)
						return
					}
					ui.Scroll(c).MaxHeight(420 * scale).Children(func() {
						ui.Column(c).Gap(2 * scale).Children(func() {
							for index, command := range items {
								index, command := index, command
								components.NavigationRow{
									Key: "command-" + string(command.ID), Label: command.Label,
									Selected: index == p.Selected, FullWidth: true,
									Role: ui.RoleButton,
									Trailing: func() {
										if command.Binding != "" {
											components.Label(c, command.Binding, theme.TextXSmall,
												semantic.Text.Muted).SingleLine().FontFeatures("tnum")
										}
									},
									Action: func() {
										p.Selected = index
										p.Submit()
									},
								}.View(c, semantic)
							}
						})
					})
					components.Label(c,
						"↑↓ Navigate  ·  Enter Run  ·  Esc Close",
						theme.TextXSmall, semantic.Text.Muted,
					)
				})
			})
		},
	}.View(c, semantic, &open)
	p.Open = open && p.Open
}
