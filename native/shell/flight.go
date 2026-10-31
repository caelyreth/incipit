package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The place's title in its two spots, in DIPs at the default UI size: the
// title bar's line, and the band at the top of the list.
const (
	headTitle, headLine, headGap, headWeight = float32(12), float32(16), float32(6), 400
	bandTitle, bandLine, bandGap, bandWeight = float32(16), float32(24), float32(8), 600
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
	// It leaves from where the title stands with its line at full length,
	// as the header lays it out with the title up (titleLines).
	up := s.titleLines(layout, reveal, listWidth, 0)
	library := layout.Sidebar * k * reveal
	pad := theme.Space12.DIPs(scale)
	button := theme.DefaultControls.Scaled(scale).Height
	headWidth := max(0, up.sheets-pad-theme.Space06.DIPs(scale)-button-listLineEnd.DIPs(scale))

	x := lerp(up.library+pad, library+pad)
	center := lerp(bar/2, bar+placeBandHeight(scale)/2)
	height := lerp(2*headLine, bandLine+headLine) * k
	width := max(0, lerp(headWidth, listWidth-2*pad))

	title, icon, tint, detail := s.placeInfo(semantic, s.listView().place)
	// The title keeps the two fonts it rests in and travels by transform:
	// each spot's copy is scaled to the size on the way and the two cross,
	// as a font of every size between would make a font a frame.
	line := lerp(headLine, bandLine) * k
	text := max(0, width-16*k-lerp(headGap, bandGap)*k)
	ui.Row(c).Key("place-flight").Absolute().Left(x).Top(center - height/2).
		Width(width).Height(height).Clip().Opacity(listReveal).
		AlignItems(ui.Center).Gap(lerp(headGap, bandGap) * k).Children(func() {
		s.rowIcon(c, semantic, icon, tint, animate)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Box(c).WidthPercent(100).Height(line).Shrink(0).Children(func() {
				for _, copy := range [...]struct {
					size     theme.TextSize
					font, at float32
					weight   int
					opacity  float32
				}{
					{theme.TextSmall, headTitle, headLine, headWeight, 1 - p},
					{theme.TextLarge, bandTitle, bandLine, bandWeight, p},
				} {
					grow := lerp(headTitle, bandTitle) / copy.font
					components.Label(c, title, copy.size, semantic.Text.Heading).SingleLine().
						FontWeight(copy.weight).Absolute().Left(0).Top(0).
						Width(text/grow).Height(copy.at*k).
						TransformOrigin(0, 0).Scale(grow, grow).Opacity(copy.opacity)
				}
			})
			components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
		})
	})
}
