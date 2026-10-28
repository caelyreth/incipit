package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type DividerWeight uint8

const (
	DefaultDivider DividerWeight = iota + 1
	VariantDivider
	FadedDivider
)

func DividerColor(semantic theme.Theme, weight DividerWeight) ui.Color {
	switch weight {
	case VariantDivider:
		return semantic.Border.Variant
	case FadedDivider:
		return semantic.Border.Default.Alpha(0.6)
	default:
		return semantic.Border.Default
	}
}

func HorizontalDivider(
	c *ui.Context,
	semantic theme.Theme,
	weight DividerWeight,
) ui.Element {
	return ui.Box(c).WidthPercent(100).Height(1).Shrink(0).
		Background(DividerColor(semantic, weight))
}

func VerticalDivider(
	c *ui.Context,
	semantic theme.Theme,
) ui.Element {
	return ui.Box(c).Width(1).Height(16 * UIFontSize(c) / theme.DefaultUIFontSize).
		Shrink(0).Background(DividerColor(semantic, DefaultDivider))
}
