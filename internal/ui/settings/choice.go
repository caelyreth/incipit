package settings

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// choiceOption is one option of a list of them, of which one is chosen,
// as the glyph forms and the languages are: a radio, its label and a note
// under it, pressed it is chosen. mark stands in place of the radio,
// actions after the option, and footer under it, where they are not nil.
type choiceOption struct {
	key, label, note string
	chosen           bool
	choose           func()
	mark             func()
	actions          func()
	footer           func()
}

// choiceView draws o as wide as its list, the option inset as a setting's
// row is.
func (s *State) choiceView(c *ui.Context, semantic theme.Theme, o choiceOption) {
	scale := s.Theme.UIFontSize
	k := scale / theme.DefaultUIFontSize
	pad := theme.Space16.DIPs(scale)
	inner := theme.Space08.DIPs(scale)
	ui.Column(c.Key(o.key)).WidthPercent(100).Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(inner).
			Padding(0, pad-inner, 0, pad-inner).Children(func() {
			option := ui.ButtonBase(c.Key("choose")).Role(ui.RoleRadio).Label(o.label).Checked(o.chosen).
				Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(theme.Space12.DIPs(scale)).
				Padding(inner, inner, inner, inner).
				Radius(theme.DefaultControls.Scaled(scale).Radius).FocusRing(true)
			// The row eases its fill in and out under the pointer and gives
			// when pressed, as the field's rows do.
			hovered := option.Hovered() && !o.chosen
			hover := components.HoverAmount(option, hovered, s.motion(c))
			option.Background(semantic.GhostElement.Hover.Alpha(hover))
			components.PressSink(option, s.motion(c))
			option.Children(func() {
				if o.mark != nil {
					o.mark()
				} else {
					radio(c, semantic, o.chosen, hovered, k)
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
					components.Label(c, o.label, theme.TextDefault, semantic.Text.Default).SingleLine()
					if o.note != "" {
						components.Label(c, o.note, theme.TextSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
					}
				})
			})
			if option.Clicked() && !o.chosen && o.choose != nil {
				o.choose()
			}
			if o.actions != nil {
				ui.Row(c).Key("actions").Gap(inner).AlignItems(ui.Center).Shrink(0).Children(o.actions)
			}
		})
		if o.footer != nil {
			o.footer()
		}
	})
}

// radio is an option's mark: a ring, filled with the accent's dot when
// chosen. The ring is a control's edge, in the muted ink, so it reads on
// either ground; over a hovered row it takes the text's own.
func radio(c *ui.Context, semantic theme.Theme, on, hovered bool, k float32) {
	size := 16 * k
	ring := ui.Box(c.Key("radio")).Size(size, size).Shrink(0).Radius(size / 2).AlignItems(ui.Center).Justify(ui.Center)
	if !on {
		ink := semantic.Text.Muted
		if hovered {
			ink = semantic.Text.Default
		}
		ring.Border(1.5*k, ink)
		return
	}
	ring.Background(semantic.Accent.Foreground).Children(func() {
		ui.Box(c.Key("dot")).Size(6*k, 6*k).Radius(3 * k).Background(semantic.Accent.OnAccent)
	})
}
