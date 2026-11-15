package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// placeTitleStyle is shared by each resting title and the copy in flight.
type placeTitleStyle struct {
	text   theme.TextSize
	gap    theme.Spacing
	weight int
}

var (
	headTitle = placeTitleStyle{text: theme.TextSmall, gap: theme.Space06, weight: 400}
	bandTitle = placeTitleStyle{text: theme.TextLarge, gap: theme.Space08, weight: 600}
)

// placeBandHeight is the band's height at the top of the list.
func placeBandHeight(scale float32) float32 {
	return bandTitle.text.LineHeight(scale) + theme.TextXSmall.LineHeight(scale) + 2*theme.Space08.DIPs(scale)
}

// takeFlight notes whether the place's title is on its way between the
// panes' title line and the top of the list this frame. While it is, both spots
// stand empty and placeFlight draws the one title that moves.
func (s *State) takeFlight(listReveal, chrome float32, animate bool) {
	s.flight = animate && s.Library != nil && !s.nav.searching &&
		s.listDock > 0.001 && s.listDock < 0.999 &&
		listReveal > 0.01 && chrome > 0.999
}

func (s *State) flying() bool { return s.flight }

// placeFlight draws the place's title moving from the title line to the
// top of the list, or back, growing to the band's size as it goes, so
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
	p := s.listDock
	lerp := func(a, b float32) float32 { return a + (b-a)*p }

	bar := layout.Titlebar * scale / theme.DefaultUIFontSize
	// It leaves from where the title stands with its line at full length,
	// as the title line lays it out with the title up (titleLines).
	up := s.titleLines(layout, reveal, listWidth, 0)
	library := layout.Sidebar * scale / theme.DefaultUIFontSize * reveal
	pad := theme.Space12.DIPs(scale)
	button := theme.DefaultControls.Scaled(scale).Height
	headWidth := max(0, up.sheets-pad-headTitle.gap.DIPs(scale)-button-listLineEnd.DIPs(scale))

	x := lerp(up.library+pad, library+pad)
	center := lerp(bar/2, bar+placeBandHeight(scale)/2)
	detailLine := theme.TextXSmall.LineHeight(scale)
	height := lerp(headTitle.text.LineHeight(scale)+detailLine, bandTitle.text.LineHeight(scale)+detailLine)
	width := max(0, lerp(headWidth, listWidth-2*pad))

	title, icon, tint, detail := s.placeInfo(semantic, s.listView().place)
	// The title keeps the two fonts it rests in and travels by transform:
	// each spot's copy is scaled to the size on the way and the two cross,
	// as a font of every size between would make a font a frame.
	line := lerp(headTitle.text.LineHeight(scale), bandTitle.text.LineHeight(scale))
	gap := lerp(headTitle.gap.DIPs(scale), bandTitle.gap.DIPs(scale))
	text := max(0, width-16*scale/theme.DefaultUIFontSize-gap)
	ui.Row(c).Key("place-flight").Absolute().Left(x).Top(center - height/2).
		Width(width).Height(height).Clip().Opacity(listReveal).
		AlignItems(ui.Center).Gap(gap).Children(func() {
		s.rowIcon(c, semantic, icon, tint, animate)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Box(c).WidthPercent(100).Height(line).Shrink(0).Children(func() {
				for _, copy := range [...]struct {
					style   placeTitleStyle
					opacity float32
				}{
					{headTitle, 1 - p},
					{bandTitle, p},
				} {
					grow := lerp(headTitle.text.Size(scale), bandTitle.text.Size(scale)) / copy.style.text.Size(scale)
					components.Label(c, title, copy.style.text, semantic.Text.Heading).SingleLine().
						FontWeight(copy.style.weight).Absolute().Left(0).Top(0).
						Width(text/grow).Height(copy.style.text.LineHeight(scale)).
						TransformOrigin(0, 0).Scale(grow, grow).Opacity(copy.opacity)
				}
			})
			components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
		})
	})
}
