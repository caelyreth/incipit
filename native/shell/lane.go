package shell

import (
	"fmt"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The desk lane is where the desk rests while the field is in front: a
// strip at the window's right edge, always there, apart from the field,
// which never moves with it. The desk stands in it, in the window's
// proportion, half in the window and half past its right edge; above and below it the lane is kept free, for what will be
// dropped on the desk. With nothing open, the desk's place keeps its room
// and makes a sheet there.

type laneState struct {
	// preview is where the desk rests in the lane this frame, in the
	// window.
	preview stage.Rect
}

// laneWidth is the lane's width at the reference type size: the half of
// the desk that stands in the window, and the room beside it.
const laneWidth = 150

// laneRoom is the room the field leaves for the lane.
func (s *State) laneRoom() float32 {
	return laneWidth * s.Settings.UIFontSize / theme.DefaultUIFontSize
}

// laneView draws the lane at the right of the window w by h, and notes
// where the desk rests in it.
func (s *State) laneView(c *ui.Context, semantic theme.Theme, w, h float32, animate bool) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	lane := ui.Box(c).Key("lane").Absolute().Top(0).Right(0).Height(h).Label("Desk lane")
	width := s.laneRoom()
	lane.Width(width)
	pad := 16 * k
	// The desk stands half in the window and half past its edge, as
	// something to step into rather than a picture of one.
	pw := 2 * (width - pad)
	ph := pw * h / max(w, 1)
	s.lane.preview = stage.Rect{X: w - width + pad, Y: h/2 - ph/2, W: pw, H: ph}
	open := len(s.Open)
	lane.Children(func() {
		label := "Desk"
		if open > 0 {
			label = fmt.Sprintf("Desk · %d open", open)
		}
		components.Label(c, label, theme.TextSmall, semantic.Text.Muted).SingleLine().
			Absolute().Left(pad + 4*k).Top(h/2 - ph/2 - 24*k)
		if open > 0 {
			return
		}
		// With nothing open, the desk's place keeps its room, and makes a
		// sheet to open there.
		radius := theme.DefaultControls.Scaled(s.Settings.UIFontSize).Radius
		place := ui.ButtonBase(c.Key("place")).Absolute().Left(pad).Top(h/2-ph/2).Size(pw, ph).
			// What it says stands in its half in the window.
			PaddingEnd(pw / 2).Justify(ui.Center).AlignItems(ui.Center).Gap(6 * k).FocusRing(false).Label("New sheet on the desk")
		hover := components.HoverAmount(place, place.Hovered(), animate)
		line := components.Blend(semantic.Border.Default, semantic.Accent.Foreground, hover)
		place.Draw(func(p *ui.Painter, r ui.Rect) { components.DashedOutline(p, r, radius, 1, line) })
		if place.Clicked() && s.Library != nil {
			s.Library.NewSheet(s.CurrentGroup(), Last)
			s.ShowDesk()
		}
		place.Children(func() {
			ui.Icon(c, components.IconNewSheet.SVG()).FontSize(16 * k).TextColor(semantic.Icon.Muted)
			components.Label(c, "Nothing open", theme.TextSmall, semantic.Text.Muted).SingleLine()
		})
	})
}
