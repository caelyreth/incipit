package shell

import (
	"time"

	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/ui"
)

// The window looks at the field or at the desk (buildStage). The field
// is where the window opens and where it comes back to: a sheet opened
// goes to the desk, and the desk left, or its last sheet closed, comes
// back to the field. On the field the desk rests in its lane at the
// window's right edge (lane.go), there with nothing open too; going to it,
// the field goes aside to the left and the desk comes up out of the lane,
// and leaving, it goes back into it.

// aside is how the field stands aside as the desk comes up.
var aside = stageAside{Shift: 0.18, Dim: 0.5}

// OnDesk reports whether the window looks at the desk, or is going there.
func (s *State) OnDesk() bool { return s.onDesk && len(s.Open) > 0 }

// ShowDesk has the window go to the desk, its sheet in front taking the
// keyboard, where there is one open.
func (s *State) ShowDesk() {
	if len(s.Open) == 0 {
		return
	}
	s.onDesk = true
	s.field.focus = false
	s.desk.claim = s.ActiveColumn
}

// ShowField has the window go to the field, its pill taking the keyboard.
func (s *State) ShowField() {
	s.onDesk, s.field.rerank = false, true
	s.field.focus = true
}

// SearchCommands has the window go to the field, the pill set to name the
// app's commands.
func (s *State) SearchCommands() {
	s.ShowField()
	s.setWords(">")
}

// escapeTwice is how soon after one Escape on the desk a second goes to
// the field.
const escapeTwice = 400 * time.Millisecond

// deskEscape hears Escape on the desk: pressed twice in quick succession,
// and not taken by what has the keyboard, it goes back to the field.
func (s *State) deskEscape(c *ui.Context) {
	if !s.OnDesk() || !c.Shortcut(0, ui.KeyEscape) {
		return
	}
	if s.desk.switcher.open {
		// The switcher open, Escape puts it away, and no more.
		s.escapedAt = time.Time{}
		s.closeSwitcher()
		return
	}
	if now := c.Now(); now.Sub(s.escapedAt) > escapeTwice {
		s.escapedAt = now
		return
	}
	s.escapedAt = time.Time{}
	s.returnToField()
}

// returnToField goes back from the desk to the field, the keyboard with
// the field, not its pill: going back is not setting out to search.
func (s *State) returnToField() {
	s.onDesk, s.field.rerank = false, true
	s.field.quiet = true
}

// ToggleField goes to the field from the desk, and back: arriving at the
// field as every way back does (returnToField), the keyboard with the
// field.
func (s *State) ToggleField() {
	if s.OnDesk() {
		s.returnToField()
		return
	}
	s.ShowDesk()
}

// view is how far the window is on its way to the desk this frame, and
// notes, as it sets out, whether the desk comes from its block. The way
// is the window's width w across, so the spring rests within a quarter
// point of it: resting a step short, the desk would jump that step.
func (s *State) viewAt(root ui.Element, w float32, animate bool) float32 {
	target := float32(0)
	if s.OnDesk() {
		target = 1
	}
	at := target
	if animate {
		at = root.AnimateTo("view", target, ui.Animation{Spring: ui.SpringOptions{
			Duration: components.MotionScene, Precision: 0.25 / float64(max(w, 1)),
		}})
	}
	s.view = at
	return at
}

// deskRest is where the desk rests this frame: in the lane.
func (s *State) deskRest(w, h float32) stageRect {
	if !s.lane.preview.Empty() {
		return s.lane.preview
	}
	return stageRect{X: w, W: w, H: h}
}

// leanDesk stands the desk where the scene says. On the way what it holds is only scaled and moved as one image
// (CacheScaled), never laid out or painted anew, so it keeps its lines and
// costs a frame no more than an image does: drawn again only as it
// shrinks under half the scale it was drawn at, or grows past it, and
// never for the scale it comes to rest at, so it settles in the lane as it
// moved there. Its corners and edge are the desk's own, around the image,
// so their changing on the way never draws it again.
// Away from the desk, the desk takes no input but a press, which brings
// it up, or opens a sheet when empty.
func (s *State) leanDesk(area ui.Element, semantic theme.Theme, look stageLook, upright float32) {
	if upright < 1 {
		// Its corners and edge are drawn as the tray's and the kept box's
		// are, in the window's points however small it stands; empty, its
		// dashed outline is its edge (deskEmpty).
		k := s.Settings.UIFontSize / theme.DefaultUIFontSize
		away, inv := 1-upright, 1/max(look.Scale, 0.01)
		area.TransformOrigin(0.5, 0.5).Scale(look.Scale, look.Scale).
			Translate(look.Offset.X, look.Offset.Y).Radius(deskCorner(k, away) * inv).Clip()
		if edge := deskEdge(away); edge > 0 && len(s.Open) > 0 {
			line := semantic.Border.Default
			area.Border(inv, line.Alpha(float32(line.A)/255*edge))
		}
	}
	if s.OnDesk() {
		return
	}
	area.Cursor(ui.CursorPointer).Role(ui.RoleButton)
	if len(s.Open) == 0 {
		area.Label(locales.NewSheetOnDesk())
		if area.Clicked() && s.Library != nil {
			s.Library.NewSheet(s.CurrentGroup(), Last)
			s.ShowDesk()
		}
		return
	}
	area.Label(locales.GoToDesk())
	if area.Clicked() {
		s.ShowDesk()
	}
	area.ContextMenu(s.openSheetsMenu)
}

// deskCorner is how round the desk's corners are, in the window's points,
// at away from filling it (1 resting in its lane, 0 filling the window):
// as round as the shelves in the lane, squaring over the last of the way
// up, so they come to the window's own corners without a seam of the
// field showing between, nor a jump as the desk arrives.
func deskCorner(k, away float32) float32 {
	return laneRadius * k * components.Smoothstep(away/deskSquaring)
}

// deskEdge is how much of its line the desk shows at away from filling
// the window: gone early on the way up, before it nears the window's own
// edges, and coming back late on the way down.
func deskEdge(away float32) float32 {
	return components.Smoothstep((away - deskEdgeGone) / (1 - deskEdgeGone - deskEdgeFull))
}

// deskSquaring is how much of the way up the desk's corners square over;
// deskEdgeGone how far from the window its line is gone, and deskEdgeFull
// how near its lane it is whole.
const deskSquaring, deskEdgeGone, deskEdgeFull = 0.35, 0.6, 0.1

// openSheetsMenu lists the sheets open on the desk, in their order, the
// one in front checked: chosen, the desk comes up on it. It says what the
// desk resting in its lane holds, which is too small to read.
func (s *State) openSheetsMenu(m *ui.Menu) {
	for _, page := range s.Open {
		title := page.Title
		if title == "" {
			title = locales.Untitled()
		}
		if m.Item(title).ID("open-" + page.Column).Checked(page.Column == s.ActiveColumn).Chosen() {
			if s.SelectColumn != nil {
				s.SelectColumn(page.Column)
			}
			s.ShowDesk()
		}
	}
	m.Separator()
	s.verbItem(m, "page.new", locales.NewSheet(), components.IconNewSheet, s.Library != nil, func() {
		s.Library.NewSheet(s.CurrentGroup(), Last)
		s.ShowDesk()
	})
}
