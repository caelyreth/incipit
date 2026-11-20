package shell

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
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
	s.desk.claim = s.ActivePage
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

// leanDesk stands the desk where the scene says. On the way it is only
// scaled and moved as one image (Cache), never laid out or painted anew,
// so it keeps its lines and costs a frame no more than an image does.
// Away from the desk, the desk takes no input but a press, which brings
// it up, or opens a sheet when empty.
func (s *State) leanDesk(area ui.Element, look stageLook, upright float32) {
	if upright < 1 {
		k := s.Settings.UIFontSize / theme.DefaultUIFontSize
		area.Cache().TransformOrigin(0.5, 0.5).Scale(look.Scale, look.Scale).
			Translate(look.Offset.X, look.Offset.Y).Radius(10 * k).Clip()
	}
	if s.OnDesk() {
		return
	}
	area.Cursor(ui.CursorPointer).Role(ui.RoleButton)
	if len(s.Open) == 0 {
		area.Label("New sheet on the desk")
		if area.Clicked() && s.Library != nil {
			s.Library.NewSheet(s.CurrentGroup(), Last)
			s.ShowDesk()
		}
		return
	}
	area.Label("Go to the desk")
	if area.Clicked() {
		s.ShowDesk()
	}
}
