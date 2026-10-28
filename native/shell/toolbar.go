package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

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
						&s.Toolbar.Italic, s.FormatItalic, metrics, semantic)
					s.formatToggle(c, "format-bold", "Bold", IconBold,
						&s.Toolbar.Bold, s.FormatBold, metrics, semantic)
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
					s.formatToggle(c, "mode-typewriter", "Typewriter", IconTypewriter,
						&s.Appearance.Typewriter, s.Typewriter, metrics, semantic)
					s.formatToggle(c, "mode-zen", "Zen", IconZen,
						&s.Appearance.Zen, s.Zen, metrics, semantic)
					s.formatToggle(c, "mode-cjk", "CJK", IconCJK,
						&s.Toolbar.CJK, s.ToggleCJK, metrics, semantic)
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
	if *on {
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
