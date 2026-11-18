package shell

import (
	"github.com/caelyreth/neverland/native/components"
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
	preview stageRect
	// tray is the tray's shelf, under it; held the sheets set aside in it,
	// which the field leaves out while they are, and heldRev counts its
	// changes.
	tray    shelfState
	held    []SheetItem
	heldRev uint64
}

// laneWidth is each lane's width at the reference type size: the part
// of what rests in it that shows, and the field gap beside it. The field
// lane at the left is as wide, so what rests in the two lanes shows alike
// at both edges (kept.go).
const laneWidth = 52

// laneRoom is the room the field leaves for each lane.
func (s *State) laneRoom() float32 {
	return laneWidth * s.Settings.UIFontSize / theme.DefaultUIFontSize
}

// laneShown is how much of what rests in a lane stands in the window.
func (s *State) laneShown() float32 {
	return (laneWidth - fieldGap) * s.Settings.UIFontSize / theme.DefaultUIFontSize
}

// fieldMargin is how far from the window's left edge the field's normal
// columns begin, the window being w by h: the lane, and half of what the
// columns leave of the field's width, steady across query modes.
func (s *State) fieldMargin(w, h float32) float32 {
	room := s.laneRoom()
	z := s.fieldMetrics(w-2*room, h).horizon
	return room + (w-2*room-(float32(z.Cols)*z.Col+float32(z.Cols-1)*z.Gap))/2
}

// laneRest is where the desk rests in its lane, the window being w by h:
// in the window by laneShown and past its right edge, keeping the
// window's proportion so its frame stays tall even when narrow.
func (s *State) laneRest(w, h float32) stageRect {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	visible := s.laneShown()
	ph := max(2*visible*h/max(w, 1), min(h*0.38, max(0, h-120*k)))
	pw := ph * w / max(h, 1)
	return stageRect{X: w - visible, Y: h/2 - ph/2, W: pw, H: ph}
}

// laneIcon, keyed key, marks what rests in a lane with icon and count
// beside it where there is one, standing above rest at the window's edge:
// at its left where left is set, else at its right.
func laneIcon(c *ui.Context, semantic theme.Theme, key string, icon components.IconName, label string, count int, rest stageRect, left bool, k float32) ui.Element {
	row := ui.Row(c).Key(key).Absolute().Top(rest.Y - 24*k).Gap(4 * k).AlignItems(ui.Center).Label(label)
	if left {
		row.Left(10 * k)
	} else {
		row.Right(10 * k)
	}
	// Its count faces the window's middle: after the icon at the left,
	// before it at the right.
	if !left {
		row.Reverse()
	}
	row.Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(14 * k).TextColor(semantic.Icon.Muted).Shrink(0)
		if count > 0 {
			components.CenteredLabel(c, thousands(count), theme.TextXSmall, semantic.Text.Muted)
		}
	})
	return row
}

// laneView draws the lane at the right of the window w by h, noting where
// the desk rests in it, and the tray under the desk.
func (s *State) laneView(c *ui.Context, semantic theme.Theme, w, h float32, hidden, animate bool) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	lane := ui.Box(c).Key("lane").Absolute().Top(0).Right(0).Size(s.laneRoom(), h).Hidden(hidden).Label("Desk lane")
	s.lane.preview = s.laneRest(w, h)
	lane.Children(func() {
		// The lane's top band drags the window above what rests in it.
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		laneIcon(c, semantic, "desk-label", components.IconDesk, "Desk icon", len(s.Open), s.lane.preview, false, k)
	})
	s.trayView(c, semantic, h, k, hidden, animate)
}
