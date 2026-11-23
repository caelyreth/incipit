package shell

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// FocusWithin reports whether the keyboard focus is anywhere in the shell,
// what floats over it among it: a press on what takes no focus, as a
// pane's background, leaves it nowhere.
func (s *State) FocusWithin(c *ui.Context) bool {
	return s.root.FocusWithin(c) || s.dockHandle.FocusWithin(c)
}

// Typing reports whether a field of the shell is open for typing, or about
// to take the focus as it opens: a sheet's find, the pill on the field, or
// a group's name being written.
func (s *State) Typing() bool {
	return s.FindRequested || s.finder.open || !s.OnDesk() || s.field.renaming != ""
}

// View draws the field and the desk where the window looks (scene.go),
// and the writing actions floating over them.
func (s *State) View(c *ui.Context) {
	semantic, changing := s.Colors.At(s.Settings, c.Now())
	c.SetTheme(s.Settings.MyGoThemeFor(semantic))
	components.Menus(c, semantic, s.Settings.DrawnMenus)
	if changing {
		c.AnimationFrame()
	}
	w, h := c.Size()
	layout := theme.DefaultLayout
	root := ui.Box(c).Fill().Background(semantic.Surface.Background).Clip().Bind(&s.root)
	animate := s.animate(c)
	s.deskEscape(c)
	s.field.drag, s.field.dragNext = s.field.dragNext, nil
	at := s.viewAt(root, w, animate)
	root.Children(func() {
		hidden := at >= 0.999
		fieldAnimate := animate && !s.field.hidden
		look := buildStage(w, h, at, stageRect{W: w, H: h}, aside).Field
		s.fieldView(c, semantic, look, s.laneRoom(), w-2*s.laneRoom(), h, hidden, fieldAnimate)
		s.fieldLaneView(c, semantic, look, w, h, hidden, fieldAnimate)
		s.laneView(c, semantic, w, h, hidden, fieldAnimate)
		scene := buildStage(w, h, at, s.deskRest(w, h), aside)
		if at > 0.001 {
			// The field darkens behind the desk coming up: a veil over it,
			// so the field is not painted anew each frame of the way.
			ui.Box(c).Key("veil").Absolute().Top(0).Left(0).Size(w, h).IgnoreInput(true).
				Background(semantic.Surface.Background.Alpha(1 - scene.Field.Shown))
		}
		s.deskView(c, semantic, layout, scene, w, h, animate)
		components.Floating(c, func() {
			if len(s.desk.columns) > 0 && s.view > 0.5 {
				s.dock(c, semantic)
			}
		})
	})
}
