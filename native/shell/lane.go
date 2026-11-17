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

// laneWidth is the lane's width at the reference type size: the visible
// strip of the desk and the field gap beside it.
const laneWidth = 94

// laneRoom is the room the field leaves for the lane.
func (s *State) laneRoom() float32 {
	return laneWidth * s.Settings.UIFontSize / theme.DefaultUIFontSize
}

// laneView draws the lane at the right of the window w by h, and notes
// where the desk rests in it.
func (s *State) laneView(c *ui.Context, semantic theme.Theme, w, h float32, hidden bool) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	lane := ui.Box(c).Key("lane").Absolute().Top(0).Right(0).Height(h).Hidden(hidden).Label("Desk lane")
	width := s.laneRoom()
	lane.Width(width)
	pad := fieldGap * k
	// The desk stands in the window and past its right edge, keeping the
	// window's proportion so its frame stays tall even when narrow.
	visible := width - pad
	ph := max(2*visible*h/max(w, 1), min(h*0.38, max(0, h-120*k)))
	pw := ph * w / max(h, 1)
	s.lane.preview = stage.Rect{X: w - visible, Y: h/2 - ph/2, W: pw, H: ph}
	open := len(s.Open)
	lane.Children(func() {
		// The lane's top band drags the window above what rests in it.
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		label := "Desk"
		if open > 0 {
			label = fmt.Sprintf("Desk · %d open", open)
		}
		components.Label(c, label, theme.TextSmall, semantic.Text.Muted).SingleLine().
			Absolute().Left(pad + 4*k).Top(h/2 - ph/2 - 24*k)
	})
}
