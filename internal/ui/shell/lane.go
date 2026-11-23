package shell

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
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

// laneIcon, keyed key, marks what rests in a lane with its icon and
// name, and its count beside where there is one, standing above rest at
// the window's edge: at its left where left is set, else at its right.
// The name stands quiet at rest, so the lane says what it holds without
// being drawn out; it gives way only where it would reach the field's
// columns, the icon and count staying.
func (s *State) laneIcon(c *ui.Context, semantic theme.Theme, key string, icon components.IconName, name string, count int, rest stageRect, left bool, k float32) ui.Element {
	inset, gap, iconSize := 10*k, 4*k, 14*k
	w, h := c.Size()
	size := theme.TextXSmall.Size(components.UIFontSize(c))
	words := name
	if count > 0 {
		words += " " + thousands(count)
	}
	wordsW, _ := c.MeasureText(0, ui.Span{Text: words, Font: theme.UIFontFamily, Size: size, Features: "tnum"})
	named := inset+iconSize+gap+wordsW+8*k <= s.fieldMargin(w, h)
	row := ui.Row(c).Key(key).Absolute().Top(rest.Y - 24*k).Gap(gap).AlignItems(ui.Center).Label(name)
	if left {
		row.Left(inset)
	} else {
		row.Right(inset)
	}
	// Its words face the window's middle: after the icon at the left,
	// before it at the right, the name read first either way.
	row.Children(func() {
		mark := func() { ui.Icon(c, icon.SVG()).FontSize(iconSize).TextColor(semantic.Icon.Muted).Shrink(0) }
		if left {
			mark()
		}
		if named {
			components.CenteredLabel(c, name, theme.TextXSmall, semantic.Text.Muted).Shrink(0)
		}
		if count > 0 {
			components.CenteredLabel(c, thousands(count), theme.TextXSmall, semantic.Text.Muted).FontFeatures("tnum").Shrink(0)
		}
		if !left {
			mark()
		}
	})
	return row
}

// laneView draws the lane at the right of the window w by h, noting where
// the desk rests in it, and the tray under the desk.
func (s *State) laneView(c *ui.Context, semantic theme.Theme, w, h float32, hidden, animate bool) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	lane := ui.Box(c).Key("lane").Absolute().Top(0).Right(0).Size(s.laneRoom(), h).Hidden(hidden).Label(locales.DeskLane())
	s.lane.preview = s.laneRest(w, h)
	if hidden {
		return
	}
	lane.Children(func() {
		// The lane's top band drags the window above what rests in it.
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		s.laneIcon(c, semantic, "desk-label", components.IconDesk, locales.Desk(), len(s.Open), s.lane.preview, false, k)
	})
	s.trayView(c, semantic, h, k, hidden, animate)
}
