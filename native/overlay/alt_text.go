package overlay

import (
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// AltText asks for the words that describe a picture, for whoever cannot
// see it, as the palette asks for a command: one field, its keys along the
// foot. Return keeps the words, Escape leaves them as they were.
type AltText struct {
	Open   bool
	Value  string
	node   model.NodeID
	chosen bool
}

// Edit opens the dialog for an image block's words.
func (a *AltText) Edit(node model.NodeID, alt string) {
	a.Open, a.Value, a.node, a.chosen = true, alt, node, false
}

// ConsumeChoice returns the words kept for an image block, once.
func (a *AltText) ConsumeChoice() (model.NodeID, string, bool) {
	if a == nil || !a.chosen {
		return 0, "", false
	}
	a.chosen = false
	return a.node, a.Value, true
}

func (a *AltText) View(c *ui.Context, semantic theme.Theme, animate bool) {
	if a == nil || !a.Open {
		return
	}
	open := a.Open
	scale := components.UIFontSize(c)
	unit := scale / theme.DefaultUIFontSize
	components.Dialog{
		Key: "alt-text", Label: "Alt text", Width: 480 * unit, Animate: animate,
		Content: func(panel ui.Element) {
			panel.Children(func() {
				ui.Column(c).WidthPercent(100).Children(func() {
					ui.Column(c).WidthPercent(100).Padding(theme.Space16.DIPs(scale)).
						Gap(theme.Space08.DIPs(scale)).Children(func() {
						components.Label(c, "Alt text", theme.TextDefault, semantic.Text.Heading).SingleLine()
						components.Label(c, "Describe the picture for whoever cannot see it.",
							theme.TextSmall, semantic.Text.Muted)
						components.TextField{
							Key: "alt-text-field", Label: "Alt text", Placeholder: "What the picture shows",
							Value: &a.Value, Icon: components.IconImage.SVG(),
							ClearIcon: components.IconClose.SVG(), Animate: animate, AutoFocus: true,
							Prominent: true,
							Submit: func() {
								a.chosen = true
								a.Open = false
							},
						}.View(c, semantic)
					})
					components.HorizontalDivider(c, semantic, components.DefaultDivider)
					keyHints(c, semantic, []keyHint{
						{[]string{"↩"}, "Return", "Save"},
						{[]string{"esc"}, "Escape", "Cancel"},
					})
				})
			})
		},
	}.View(c, semantic, &open)
	a.Open = open && a.Open
}
