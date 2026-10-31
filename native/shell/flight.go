package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The place's title in its two spots, in DIPs at the default UI size: the
// title bar's line, and the band at the top of the list.
const (
	headTitle, headLine, headGap, headWeight = 12, 16, 6, 400
	bandTitle, bandLine, bandGap, bandWeight = 16, 24, 8, 600
)

// placeBandHeight is the band's height at the top of the list.
func placeBandHeight(scale float32) float32 {
	return (bandLine + headLine + 2*float32(theme.Space08)) * scale / theme.DefaultUIFontSize
}

// takeFlight notes whether the place's title is on its way between the
// title bar and the top of the list this frame. While it is, both spots
// stand empty and placeFlight draws the one title that moves.
func (s *State) takeFlight(listReveal, chrome float32, animate bool) {
	s.flight = animate && s.Library != nil && !s.nav.searching &&
		s.listDock > 0.001 && s.listDock < 0.999 &&
		listReveal > 0.01 && chrome > 0.999
}

func (s *State) flying() bool { return s.flight }

// placeFlight draws the place's title moving from the title bar's line to
// the top of the list, or back, growing to the band's size as it goes, so
// it reads as one title going down rather than one leaving as another
// comes. Both ends are worked out from the panes' openness this frame, so
// it tracks them as they slide.
func (s *State) placeFlight(
	c *ui.Context,
	semantic theme.Theme,
	layout theme.Layout,
	reveal, listWidth, listReveal float32,
	animate bool,
) {
	if !s.flight {
		return
	}
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	p := s.listDock
	lerp := func(a, b float32) float32 { return a + (b-a)*p }

	bar := layout.Titlebar * k
	leading := ((layout.Sidebar-140)*reveal + 140) * k
	library := layout.Sidebar * k * reveal
	pad := theme.Space12.DIPs(scale)
	button := theme.DefaultControls.Scaled(scale).Height
	headWidth := max(0, listWidth-(leading-library)) - pad - 2*theme.Space06.DIPs(scale) - button

	x := lerp(leading+pad, library+pad)
	center := lerp(bar/2, bar+placeBandHeight(scale)/2)
	height := lerp(2*headLine, bandLine+headLine) * k
	width := max(0, lerp(headWidth, listWidth-2*pad))

	title, icon, tint, detail := s.placeInfo(semantic, s.listView().place)
	ui.Row(c).Key("place-flight").Absolute().Left(x).Top(center - height/2).
		Width(width).Height(height).Clip().Opacity(listReveal).
		AlignItems(ui.Center).Gap(lerp(headGap, bandGap) * k).Children(func() {
		s.rowIcon(c, semantic, icon, tint, animate)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			components.Label(c, title, theme.TextSmall, semantic.Text.Heading).SingleLine().
				FontSize(lerp(headTitle, bandTitle) * k).FixedLineHeight(lerp(headLine, bandLine) * k).
				FontWeight(int(lerp(headWeight, bandWeight) + 0.5))
			components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
		})
	})
}
