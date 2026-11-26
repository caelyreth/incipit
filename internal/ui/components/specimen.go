package components

import (
	"time"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// inkDraw is how long a ring of ink takes to draw itself round a choice.
const inkDraw = 640 * time.Millisecond

// Specimen is one answer to a choice, shown as what it does rather than
// named on a switch: a preview of its effect over its name and a line
// saying what it means. The answer chosen is circled in ink, as a pen
// rings a word on a proof; the others wait on the page, a little quieter.
// It is a radio button to the screen reader, Title its name.
type Specimen struct {
	Key     string
	Title   string
	Caption string
	// Selected is whether it is the answer chosen, Animate whether its
	// ring draws itself and its lift eases.
	Selected bool
	Animate  bool
	// Seed is the hand's: each card's ring wavers its own way.
	Seed float32
	// Ink is the ring's color, the accent's when unset.
	Ink    ui.Color
	Action func()
	// Menu, unless nil, is the card's context menu.
	Menu func(m *ui.Menu)
}

// View draws the card, its preview drawn by preview into the box at its
// top, the full width of the card.
func (s Specimen) View(c *ui.Context, semantic theme.Theme, preview func()) ui.Element {
	scale := UIFontSize(c)
	controls := theme.DefaultControls.Scaled(scale)
	pad := theme.Space08.DIPs(scale)
	ink := s.Ink
	if ink == (ui.Color{}) {
		ink = semantic.Accent.Foreground
	}
	card := ui.ButtonBase(c.Key(s.Key)).Role(ui.RoleRadio).Checked(s.Selected).Label(s.Title).
		Description(s.Caption).KeepFocus().Padding(pad).Radius(controls.PanelRadius).
		Grow(1).Shrink(1).MinWidth(0)
	if s.Menu != nil {
		card.ContextMenu(s.Menu)
	}
	hover := HoverAmount(card, card.Hovered(), s.Animate)
	ring := float32(0)
	if s.Selected {
		ring = 1
	}
	if s.Animate {
		ring = Follow(card, "ring", ring, inkDraw)
	}
	PressSink(card, s.Animate)
	rest := semantic.Surface.Page
	card.Background(Blend(rest, semantic.Surface.Elevated, 0.35*hover+0.5*ring)).
		Border(1, Blend(semantic.Border.Variant, semantic.Border.Default, hover)).
		Translate(0, -2*hover)
	if hover > 0 || ring > 0 {
		card.Shadow(0, 6*max(hover, ring*0.6), 18, 0, ui.RGBA(0, 0, 0, 0.12*max(hover, ring*0.6)))
	}
	card.Children(func() {
		ui.Column(c).WidthPercent(100).Gap(theme.Space08.DIPs(scale)).Children(func() {
			ui.Box(c).WidthPercent(100).Radius(controls.InnerRadius).Clip().Children(preview)
			ui.Column(c).WidthPercent(100).Padding(0, theme.Space04.DIPs(scale), theme.Space02.DIPs(scale)).Gap(theme.Space02.DIPs(scale)).Children(func() {
				title := semantic.Text.Default
				if s.Selected {
					title = semantic.Text.Heading
				}
				Label(c, s.Title, theme.TextDefault, title).FontWeight(600).SingleLine()
				if s.Caption != "" {
					Label(c, s.Caption, theme.TextSmall, semantic.Text.Muted)
				}
			})
		})
	})
	card.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if ring <= 0.001 {
			return
		}
		InkRing(p, ui.Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}, ink.Alpha(0.9), 2.2*scale/theme.DefaultUIFontSize, s.Seed, ring)
		mark := 18 * scale / theme.DefaultUIFontSize
		InkCheck(p, ui.Rect{X: r.X + r.W - mark - 1.6*pad, Y: r.Y + r.H - mark - 1.4*pad, W: mark, H: mark}, ink, 2.4*scale/theme.DefaultUIFontSize, (ring-0.4)/0.6)
	})
	if card.Clicked() && s.Action != nil {
		s.Action()
	}
	return card
}
