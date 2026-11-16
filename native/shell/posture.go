package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Posture returns the shell's target posture.
func (s *State) Posture() stage.Posture {
	if s == nil {
		return stage.PostureWriting
	}
	return s.posture
}

// StageScene returns the top-level stage placements from the most recent build.
func (s *State) StageScene() stage.Scene {
	if s == nil {
		return stage.Scene{}
	}
	return s.scene
}

// SetPosture moves the shell to posture, focusing the pill when entering the
// field and returning focus to the active sheet when returning to Writing.
func (s *State) SetPosture(p stage.Posture) {
	if s == nil || s.posture == p {
		return
	}
	was := s.posture
	s.posture = p
	if p == stage.PostureWriting {
		s.pill.focus = false
		s.pill.typing = false
		s.field.selectedID = ""
		if s.ActivePage != "" {
			s.desk.claim = s.ActivePage
		}
		return
	}
	if was == stage.PostureWriting {
		s.pill.pan = stage.Point{}
	}
	s.pill.focus = true
}

// CallField enters the field from Writing (Glance when sheets are open, Arrive
// when the desk is empty) or returns to Writing when already in the field.
func (s *State) CallField() {
	if s == nil {
		return
	}
	if s.posture != stage.PostureWriting {
		s.ReturnToDesk()
		return
	}
	if len(s.Open) == 0 {
		s.SetPosture(stage.PostureArrive)
		return
	}
	s.SetPosture(stage.PostureGlance)
}

// ToggleField toggles between Writing and Arrive (full field exploration).
func (s *State) ToggleField() {
	if s == nil {
		return
	}
	if s.posture == stage.PostureArrive {
		s.ReturnToDesk()
		return
	}
	s.SetPosture(stage.PostureArrive)
}

// ToggleGlance toggles between Writing and Glance (temporary half-window mix).
func (s *State) ToggleGlance() {
	if s == nil {
		return
	}
	if s.posture == stage.PostureGlance {
		s.ReturnToDesk()
		return
	}
	s.SetPosture(stage.PostureGlance)
}

// ReturnToDesk brings the desk back to Writing.
func (s *State) ReturnToDesk() {
	s.SetPosture(stage.PostureWriting)
}

func postureTarget(p stage.Posture) float32 {
	switch p {
	case stage.PostureGlance:
		return 1
	case stage.PostureArrive:
		return 2
	default:
		return 0
	}
}

func (s *State) computeStage(root ui.Element, width, height, head float32, animate bool) stage.Scene {
	target := postureTarget(s.posture)
	level := target
	if animate {
		level = components.Follow(root, "posture-level", target, components.MotionSettle)
	}
	level = max(0, min(level, 2))
	s.postureLevel = level
	vp := stage.Rect{W: width, H: height}
	hasDesk := len(s.Open) > 0
	var scene stage.Scene
	if level <= 1 {
		scene = stage.BlendScene(vp, stage.PostureWriting, stage.PostureGlance, level, hasDesk, head)
	} else {
		scene = stage.BlendScene(vp, stage.PostureGlance, stage.PostureArrive, level-1, hasDesk, head)
	}
	s.scene = scene
	return scene
}

// fieldView draws the field layer behind the desk when a field posture is
// active or in transition.
func (s *State) fieldView(c *ui.Context, semantic theme.Theme, scene stage.Scene, animate bool) {
	field := ui.Box(c).Key("field").Absolute().Fill().
		Background(semantic.Surface.Background).
		Opacity(scene.Field.Opacity).
		Role(ui.RoleGroup).Label("Field")
	if !scene.Field.Interactive {
		field.IgnoreInput(true)
	} else {
		k := s.Settings.UIFontSize / theme.DefaultUIFontSize
		field.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputScroll || (ev.DX == 0 && ev.DY == 0) {
				return false
			}
			s.panField(s.field.layout, -ev.DX, -ev.DY, k)
			return true
		})
	}
	field.Children(func() {
		canvas := ui.Box(c).Key("field-plane").Absolute().Fill().
			TransformOrigin(0.5, 0.5).Scale(scene.Field.Scale, scene.Field.Scale)
		if scene.Field.Interactive {
			if !s.pill.typing && s.pill.query == "" && !s.FindOpen() && c.Shortcut(0, ui.KeyEscape) {
				if s.pill.scopeGroup != "" {
					s.SetPillScope("")
				} else {
					s.ReturnToDesk()
				}
			}
		}
		canvas.Children(func() {
			s.fieldCardsView(c, canvas, semantic, scene, animate)
			s.pillView(c, semantic, scene, animate)
		})
	})
}
