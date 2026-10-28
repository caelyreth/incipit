package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) toolbar(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	gap := theme.Space08.DIPs(s.Settings.UIFontSize)
	ui.Row(c).Width(420).Height(metrics.Row+theme.Space12.DIPs(s.Settings.UIFontSize)).
		Padding(theme.Space04.DIPs(s.Settings.UIFontSize)).
		AlignItems(ui.Center).Justify(ui.Center).Gap(gap).
		Radius(metrics.PanelRadius).
		Background(semantic.Surface.Elevated).
		Border(1, semantic.Border.Variant).
		Children(func() {
			toolbarButton(c, "toolbar-italic", "Italic", s.FormatItalic, true)
			toolbarButton(c, "toolbar-bold", "Bold", s.FormatBold, true)
			ui.Divider(c).BorderColor(semantic.Border.Variant)
			toolbarButton(c, "toolbar-undo", "Undo", s.Undo, s.CanUndo)
			toolbarButton(c, "toolbar-redo", "Redo", s.Redo, s.CanRedo)
			ui.Divider(c).BorderColor(semantic.Border.Variant)
			toolbarButton(c, "toolbar-background", "Background", s.Background, true)
			toolbarButton(c, "toolbar-center", "Center writing", s.CenterText, true)
			toolbarButton(c, "toolbar-clear", "Clear formatting", s.ClearFormat, true)
		})
}

func toolbarButton(
	c *ui.Context,
	key string,
	label string,
	action func(),
	enabled bool,
) {
	button := ui.Button(c.Key(key), label).Label(label).Disabled(!enabled).
		Background(ui.Transparent)
	if enabled && action != nil && button.Clicked() {
		action()
	}
}
