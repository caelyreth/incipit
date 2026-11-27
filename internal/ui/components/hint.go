package components

import (
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// HintSide is where a hint stands by its control: toward the open space
// beside it, so it never covers a neighbour.
type HintSide uint8

const (
	// HintAbove stands over the control, centred, or under it where there
	// is no room above.
	HintAbove HintSide = iota
	// HintBelow stands under it, as for what stands on the title line.
	HintBelow
	// HintAfter stands past its trailing edge, as for the field lane's.
	HintAfter
	// HintBefore stands before its leading edge, as for the desk lane's.
	HintBefore
)

// Hint names a control that has no words of its own, and shows its keys,
// once the pointer rests on it: never instructions, warnings or anything
// the person needs, which belong on the screen, and never on a control
// whose words already say it. It is drawn in the shell's own material,
// raised, hairline-rimmed and the controls' size, not the system's
// inverse block. binding is its command's key as the keymap spells it,
// "" for none.
func Hint(c *ui.Context, semantic theme.Theme, el ui.Element, label, binding string, side HintSide) ui.Element {
	if label == "" {
		return el
	}
	ui.TooltipBase(c, el, func(tip ui.Element) {
		scale := UIFontSize(c)
		k := scale / theme.DefaultUIFontSize
		gap := 6 * k
		switch side {
		case HintBelow:
			tip.AttachTo(el, ui.AnchorBottom, ui.AnchorTop).Margin(gap, 0, 0, 0)
		case HintAfter:
			tip.AttachTo(el, ui.AnchorEnd, ui.AnchorStart).MarginStart(gap)
		case HintBefore:
			tip.AttachTo(el, ui.AnchorStart, ui.AnchorEnd).MarginEnd(gap)
		default:
			tip.AttachTo(el, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, gap, 0)
		}
		tip.Padding(4*k, 8*k).Radius(8*k).Background(semantic.Surface.Elevated).
			Border(1, semantic.Border.Variant).Shadow(0, 2*k, 10*k, 0, ui.RGBA(0, 0, 0, 0.14))
		tip.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8 * k).Children(func() {
				CenteredLabel(c, label, theme.TextSmall, semantic.Text.Default).SingleLine()
				if binding != "" {
					CenteredLabel(c, binding, theme.TextSmall, semantic.Text.Muted).SingleLine()
				}
			})
		})
	})
	return el
}
