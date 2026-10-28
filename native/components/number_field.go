package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type NumberField struct {
	Key         string
	Label       string
	Value       string
	CanDecrease bool
	CanIncrease bool
	Animate     bool
	Decrease    func()
	Increase    func()
}

func (n NumberField) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	return ui.Row(c).Height(metrics.Height).Shrink(0).AlignItems(ui.Center).
		Gap(theme.Space04.DIPs(scale)).PaddingX(theme.Space04.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Inset).Children(func() {
		IconButton{
			Key: "decrease", Label: "Decrease " + n.Label, Icon: IconMinus.SVG(),
			Style: SubtleButton, Size: CompactButton, Disabled: !n.CanDecrease,
			Animate: n.Animate, IconColor: &semantic.Icon.Muted, Action: n.Decrease,
		}.View(c, semantic)
		ui.Box(c).MinWidth(58 * scale / theme.DefaultUIFontSize).
			AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			Label(c, n.Value, theme.TextSmall, semantic.Text.Default).
				SingleLine().FontFeatures("tnum")
		})
		IconButton{
			Key: "increase", Label: "Increase " + n.Label, Icon: IconPlus.SVG(),
			Style: SubtleButton, Size: CompactButton, Disabled: !n.CanIncrease,
			Animate: n.Animate, IconColor: &semantic.Icon.Muted, Action: n.Increase,
		}.View(c, semantic)
	})
}
