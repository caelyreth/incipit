package overlay

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// View draws the palette as the app's lists are drawn: the query on top,
// the commands under their groups' labels, as the library heads its groups,
// each with its keys as the Keymap shows them, and the keys it takes along
// its foot. The chosen command is marked by the selection that glides
// between rows, as the sidebar's is.
func (p *CommandPalette) View(c *ui.Context, semantic theme.Theme, animate bool) {
	if p == nil || !p.Open {
		return
	}
	open := p.Open
	items := p.Filtered()
	if p.Selected >= len(items) {
		p.Selected = max(0, len(items)-1)
	}
	moved := false
	if c.Shortcut(0, ui.KeyDown) {
		p.Move(1)
		moved = true
	}
	if c.Shortcut(0, ui.KeyUp) {
		p.Move(-1)
		moved = true
	}
	scale := components.UIFontSize(c)
	unit := scale / theme.DefaultUIFontSize
	components.Dialog{
		Key: "command-palette", Label: "Commands", Width: 640 * unit, Animate: animate,
		Content: func(panel ui.Element) {
			panel.Children(func() {
				ui.Column(c).WidthPercent(100).Children(func() {
					ui.Box(c).WidthPercent(100).Padding(theme.Space16.DIPs(scale)).Children(func() {
						components.TextField{
							Key: "command-query", Label: "Search commands", Placeholder: "Search commands",
							Value: &p.Query, Icon: components.IconSearch.SVG(),
							ClearIcon: components.IconClose.SVG(), Animate: animate, AutoFocus: true,
							Prominent: true,
							Changed:   func() { p.Selected = 0 },
							Submit:    func() { p.Submit() },
						}.View(c, semantic)
					})
					components.HorizontalDivider(c, semantic, components.DefaultDivider)
					if len(items) == 0 {
						ui.Box(c).WidthPercent(100).Padding(theme.Space24.DIPs(scale)).
							AlignItems(ui.Center).Children(func() {
							components.Label(c, "No commands match", theme.TextDefault, semantic.Text.Muted)
						})
					} else {
						p.list(c, semantic, items, moved, animate)
					}
					components.HorizontalDivider(c, semantic, components.DefaultDivider)
					p.foot(c, semantic)
				})
			})
		},
	}.View(c, semantic, &open)
	p.Open = open && p.Open
}

// list draws the commands under their groups' labels, keeping the chosen
// one in view as the keys move it.
func (p *CommandPalette) list(c *ui.Context, semantic theme.Theme, items []Command, moved, animate bool) {
	scale := components.UIFontSize(c)
	scroll := ui.Scroll(c).MaxHeight(440 * scale / theme.DefaultUIFontSize).TrackScroll(&p.scroll)
	scroll = components.BindOverscroll(
		scroll,
		&p.scroll,
		&p.over,
		scrollfx.Vertical,
		components.OverscrollList,
		scroll.LayoutBounds().H,
		scale,
	)
	scroll.Children(func() {
		list := ui.Column(c).WidthPercent(100).Role(ui.RoleList).Label("Commands").
			Padding(theme.Space08.DIPs(scale)).Gap(theme.Space02.DIPs(scale))
		p.over.Apply(c, list, animate)
		// The dialog's surface is the sidebar's inset: the selection takes
		// the ghost's, to show on it.
		selection := components.NavigationSelection(c, semantic)
		selection.Fill = semantic.GhostElement.Selected
		track := components.TrackSelection(list, selection, animate)
		// The caps take the page's surface, to show on the dialog's.
		keys := components.KeycapStyle(semantic, scale, false)
		keys.Face = semantic.Surface.Page
		list.Children(func() {
			group := ""
			for index, command := range items {
				if command.Group != group {
					group = command.Group
					groupLabel(c, semantic, group, index == 0)
				}
				row := components.NavigationRow{
					Key: "command-" + string(command.ID), Label: command.Label,
					Selected: index == p.Selected, Track: track, FullWidth: true,
					Role: ui.RoleButton, Animate: animate, Prominent: true,
					Trailing: func() {
						if len(command.Keys) > 0 {
							components.Keycaps(c, keys, command.Keys, command.Binding)
						}
					},
					Action: func() {
						p.Selected = index
						p.Submit()
					},
				}.View(c, semantic)
				if moved && index == p.Selected {
					row.ScrollIntoView()
				}
			}
		})
	})
}

// groupLabel heads a group of commands, as the library heads its groups.
func groupLabel(c *ui.Context, semantic theme.Theme, group string, first bool) {
	scale := components.UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	label := ui.Row(c).Key("group-" + group).WidthPercent(100).Height(metrics.InlineHeight).
		PaddingStart(theme.Space08.DIPs(scale)).AlignItems(ui.End)
	if !first {
		label.Margin(theme.Space08.DIPs(scale), 0, 0, 0)
	}
	label.Children(func() {
		components.Label(c, group, theme.TextSmall, semantic.Text.Muted).SingleLine()
	})
}

// foot names the keys the palette takes, each on its caps.
func (p *CommandPalette) foot(c *ui.Context, semantic theme.Theme) {
	keyHints(c, semantic, []keyHint{
		{[]string{"↑", "↓"}, "Up or Down", "Move"},
		{[]string{"↩"}, "Return", "Run"},
		{[]string{"esc"}, "Escape", "Close"},
	})
}

// keyHint is a key an overlay takes: its caps, their spoken name, and
// what it does.
type keyHint struct {
	keys        []string
	label, does string
}

// keyHints names the keys an overlay takes along its foot, each on its
// caps.
func keyHints(c *ui.Context, semantic theme.Theme, hints []keyHint) {
	scale := components.UIFontSize(c)
	keys := components.KeycapStyle(semantic, scale, true)
	ui.Row(c).WidthPercent(100).Height(theme.DefaultControls.Scaled(scale).Row).
		PaddingX(theme.Space16.DIPs(scale)).AlignItems(ui.Center).
		Gap(theme.Space16.DIPs(scale)).Children(func() {
		for _, hint := range hints {
			ui.Row(c).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Children(func() {
				components.Keycaps(c, keys, hint.keys, hint.label)
				components.Label(c, hint.does, theme.TextSmall, semantic.Text.Muted).SingleLine()
			})
		}
	})
}
