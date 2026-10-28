package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) View(c *ui.Context) {
	semantic := s.Settings.Semantic()
	c.SetTheme(s.Settings.MyGoTheme())
	width, _ := c.Size()
	metrics := theme.DefaultControls.Scaled(s.Settings.UIFontSize)
	layout := theme.DefaultLayout
	sidebar := !s.Appearance.Zen && width >= layout.Narrow
	bar := c.TitleBar()

	ui.Column(c).Fill().Background(semantic.Surface.Background).Children(func() {
		s.header(c, semantic, metrics, layout, bar)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if sidebar {
				s.sidebar(c, semantic, metrics, layout)
			}
			ui.Column(c).Grow(1).Background(semantic.Surface.Page).Children(func() {
				s.document(c, semantic, metrics, layout)
			})
		})
		s.footer(c, semantic, metrics, layout)
	})
}

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

func (s *State) sidebar(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	ui.Column(c).Width(layout.Sidebar).HeightPercent(100).
		Background(semantic.Surface.Panel).Padding(theme.Space08.DIPs(s.Settings.UIFontSize)).
		Children(func() {
			s.pageSearch(c, semantic, metrics)
			ui.Box(c).Height(theme.Space04.DIPs(s.Settings.UIFontSize))
			selected := s.ActivePage
			ui.Sidebar(c, &selected, func() {
				open := true
				ui.SidebarSection(c, fmt.Sprintf("Pages · %d", len(s.filteredPages())), &open, func() {
					for _, page := range s.filteredPages() {
						ui.SidebarItem(c, page.ID, glyph(IconDocument), page.Title)
					}
				})
			}).Grow(1)
			if selected != s.ActivePage && s.SelectPage != nil {
				s.SelectPage(selected)
			}
		})
	ui.Divider(c).BorderColor(semantic.Border.Default)
}

func (s *State) document(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	prose := theme.NewProseStyle(s.Settings.ProseFontSize)
	column := prose.Column()
	ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
		ui.Row(c).Grow(1).WidthPercent(100).Justify(ui.Center).Padding(
			layout.ReadingTop,
			layout.ContentGutter,
			layout.ReadingTop,
			layout.ContentGutter,
		).Children(func() {
			ui.Column(c).FillHeight().Width(column).MaxWidthPercent(100).Children(func() {
				ui.Text(c, s.activeTitle()).FontSize(prose.TitleSize()).
					TextColor(semantic.Prose.Heading).Font(theme.ProseFontFamily)
				ui.Box(c).Height(layout.TitleBottom)
				if s.Content != nil {
					s.Content(c)
				}
				ui.Box(c).Height(layout.ReadingBottom)
			})
		})
		s.formatBar(c, semantic, metrics)
	})
}

func (s *State) pageSearch(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	ui.Row(c.Key("page-search-field")).Height(metrics.Height).
		Gap(theme.Space06.DIPs(s.Settings.UIFontSize)).
		Padding(0, theme.Space08.DIPs(s.Settings.UIFontSize)).
		AlignItems(ui.Center).Radius(metrics.Radius).
		Background(semantic.Element.Background).Children(func() {
		ui.Icon(c, glyph(IconSearch)).FontSize(metrics.Icon * 0.82).
			TextColor(semantic.Icon.Muted)
		ui.TextInputBase(c, &s.PageQuery).Grow(1).MinWidth(0).
			Placeholder("Find a page...").Label("Search pages").
			FontSize(theme.TextSmall.Size(s.Settings.UIFontSize)).
			TextColor(semantic.Text.Default)
	})
}

func (s *State) formatBar(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	ui.Row(c).Absolute().Left(0).Right(0).
		Bottom(theme.Space16.DIPs(s.Settings.UIFontSize)).
		Justify(ui.Center).PassThrough().Children(func() {
		ui.Row(c).Height(metrics.Height+theme.Space08.DIPs(s.Settings.UIFontSize)).
			AlignItems(ui.Center).PaddingX(theme.Space04.DIPs(s.Settings.UIFontSize)).
			Radius(metrics.DockRadius).
			Background(semantic.Surface.Elevated).
			Border(1, semantic.Border.Default).
			Shadow(0, theme.Space08.DIPs(s.Settings.UIFontSize),
				theme.Space16.DIPs(s.Settings.UIFontSize), 0, ui.RGBA(0, 0, 0, 0.24)).
			Children(func() {
				ui.Toolbar(c, func() {
					s.formatToggle(c, "format-italic", "Italic", IconItalic,
						&s.Formatting.Italic, s.FormatItalic, metrics, semantic)
					s.formatToggle(c, "format-bold", "Bold", IconBold,
						&s.Formatting.Bold, s.FormatBold, metrics, semantic)
					formatSeparator(c, metrics, semantic)
					if s.iconButton(c, "undo", "Undo", IconUndo, metrics, semantic).
						Disabled(!s.CanUndo).Clicked() && s.Undo != nil {
						s.Undo()
					}
					if s.iconButton(c, "redo", "Redo", IconRedo, metrics, semantic).
						Disabled(!s.CanRedo).Clicked() && s.Redo != nil {
						s.Redo()
					}
					formatSeparator(c, metrics, semantic)
					s.formatToggle(c, "format-background", "Background", IconBackground,
						&s.Formatting.Background, s.Background, metrics, semantic)
					if s.iconButton(c, "focus-document", "Focus document", IconFocus,
						metrics, semantic).Clicked() && s.FocusDocument != nil {
						s.FocusDocument()
					}
					if s.iconButton(c, "clear-formatting", "Clear formatting", IconClearFormatting,
						metrics, semantic).Clicked() && s.ClearFormat != nil {
						s.ClearFormat()
					}
				}).Label("Formatting")
			})
	})
}

func (s *State) formatToggle(
	c *ui.Context,
	key string,
	label string,
	icon Icon,
	on *bool,
	changed func(bool),
	metrics theme.ControlMetrics,
	semantic theme.Theme,
) {
	color := semantic.Icon.Default
	background := ui.Transparent
	switch {
	case *on:
		color = semantic.Icon.Accent
		background = semantic.Element.Selected
	}
	toggle := ui.ToggleBase(c.Key(key), on).Label(label).Tooltip(label).
		Size(metrics.Height, metrics.Height).Padding(0).Radius(metrics.Radius).
		Background(background)
	if !*on && toggle.Hovered() {
		toggle.Background(semantic.Element.Hover)
	}
	toggle.Children(func() {
		ui.Icon(c, glyph(icon)).FontSize(metrics.Icon).TextColor(color)
	})
	if toggle.Changed() && changed != nil {
		changed(*on)
	}
}

func formatSeparator(c *ui.Context, metrics theme.ControlMetrics, semantic theme.Theme) {
	ui.Box(c).Width(1).Height(metrics.Icon).Shrink(0).Background(semantic.Border.Default)
}

func (s *State) iconButton(
	c *ui.Context,
	key string,
	label string,
	icon Icon,
	metrics theme.ControlMetrics,
	semantic theme.Theme,
) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).
		Size(metrics.Height, metrics.Height).Padding(0).Radius(metrics.Radius).
		Background(ui.Transparent)
	if button.Hovered() {
		if button.Pressed() {
			button.Background(semantic.Element.Selected)
		} else {
			button.Background(semantic.Element.Hover)
		}
	}
	button.Children(func() {
		ui.Icon(c, glyph(icon)).FontSize(metrics.Icon).TextColor(semantic.Icon.Default)
	})
	return button
}

func (s *State) footer(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	ui.Divider(c).BorderColor(semantic.Border.Default)
	ui.Row(c).Height(layout.Status).Padding(0, theme.Space12.DIPs(s.Settings.UIFontSize)).
		AlignItems(ui.Center).Gap(theme.Space12.DIPs(s.Settings.UIFontSize)).Children(func() {
		ui.Text(c, s.Status).TextColor(semantic.Text.Muted)
		ui.Spacer(c)
		if s.Statistics != "" {
			ui.Text(c, s.Statistics).TextColor(semantic.Text.Muted)
		}
		if s.Appearance.Typewriter {
			ui.Text(c, "Typewriter").TextColor(semantic.Text.Muted)
		}
	})
}

func (s *State) activeTitle() string {
	for _, page := range s.Pages {
		if page.ID == s.ActivePage {
			return page.Title
		}
	}
	return "Notebook"
}

func (s *State) filteredPages() []Page {
	query := strings.TrimSpace(strings.ToLower(s.PageQuery))
	if query == "" {
		return s.Pages
	}
	filtered := make([]Page, 0, len(s.Pages))
	for _, page := range s.Pages {
		if strings.Contains(strings.ToLower(page.Title), query) {
			filtered = append(filtered, page)
		}
	}
	return filtered
}
