package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) header(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
	bar ui.TitleBar,
) {
	left := max(bar.Left, theme.Space12.DIPs(s.Settings.UIFontSize))
	right := max(bar.Right, theme.Space12.DIPs(s.Settings.UIFontSize))
	height := max(bar.Height, metrics.Height+theme.Space08.DIPs(s.Settings.UIFontSize))
	ui.Row(c).Height(height).Padding(0, right, 0, left).
		AlignItems(ui.Center).Gap(theme.Space08.DIPs(s.Settings.UIFontSize)).
		DragWindow().Children(func() {
		ui.Text(c, "Shoreward").Bold().FontSize(
			theme.TextDefault.Size(s.Settings.UIFontSize),
		).TextColor(semantic.Text.Default).MaxWidth(layout.Sidebar - left).SingleLine()
		if s.iconButton(c, "toggle-sidebar", "Toggle sidebar", IconSidebar, metrics, semantic).Clicked() &&
			s.ToggleSidebar != nil {
			s.ToggleSidebar()
		}
		ui.Box(c).Width(1).Height(metrics.Icon).Shrink(0).Background(semantic.Border.Default)
		if !s.Appearance.Zen {
			for _, page := range s.Pages {
				tab := ui.Button(c.Key("tab-"+page.ID), page.Title).
					Height(metrics.Height).Label(page.Title)
				if page.ID == s.ActivePage {
					tab.Background(semantic.Element.Selected)
				}
				if tab.Clicked() && s.SelectPage != nil {
					s.SelectPage(page.ID)
				}
			}
		} else {
			ui.Icon(c, glyph(IconDocument)).FontSize(metrics.Icon).
				TextColor(semantic.Icon.Muted)
			ui.Text(c, s.activeTitle()).MaxWidth(
				layout.Sidebar - theme.Space32.DIPs(s.Settings.UIFontSize),
			).MinWidth(0).FontSize(
				theme.TextDefault.Size(s.Settings.UIFontSize),
			).TextColor(semantic.Text.Default).SingleLine()
		}
		if s.iconButton(c, "close-page", "Close page", IconClose, metrics, semantic).
			Disabled(s.ActivePage == "").Clicked() && s.ClosePage != nil {
			s.ClosePage()
		}
		ui.Spacer(c)
		if s.iconButton(c, "commands", "Commands", IconToolbar, metrics, semantic).Clicked() &&
			s.OpenCommands != nil {
			s.OpenCommands()
		}
		if s.iconButton(c, "new-page", "New page", IconPlus, metrics, semantic).Clicked() &&
			s.NewPage != nil {
			s.NewPage()
		}
		ui.Box(c).Width(1).Height(metrics.Icon).Shrink(0).Background(semantic.Border.Default)
		if s.iconButton(c, "find-page", "Find pages", IconSearch, metrics, semantic).Clicked() &&
			s.FindPage != nil {
			s.FindPage()
		}
		if s.iconButton(c, "settings", "Settings", IconSettings, metrics, semantic).Clicked() &&
			s.OpenSettings != nil {
			s.OpenSettings()
		}
	})
	ui.Divider(c).BorderColor(semantic.Border.Default)
}
