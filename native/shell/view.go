package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// FocusWithin reports whether the keyboard focus is anywhere in the shell,
// what floats over it among it: a press on what takes no focus, as a
// pane's background, leaves it nowhere.
func (s *State) FocusWithin(c *ui.Context) bool {
	return s.root.FocusWithin(c) || s.floating[0].FocusWithin(c) || s.floating[1].FocusWithin(c)
}

// Typing reports whether a field of the shell is open for typing, or about
// to take the focus as it opens: the search, a sheet's find, or a name
// being edited.
func (s *State) Typing() bool {
	return s.FindRequested || s.finder.open ||
		s.pill.focus || s.pill.typing || s.field.renamingGroup != ""
}

func (s *State) View(c *ui.Context) {
	semantic, changing := s.Colors.At(s.Settings, c.Now())
	c.SetTheme(s.Settings.MyGoThemeFor(semantic))
	if changing {
		c.AnimationFrame()
	}
	width, height := c.Size()
	scale := s.Settings.UIFontSize
	layout := theme.DefaultLayout

	chromeTarget := float32(1)
	if s.Appearance.Zen {
		chromeTarget = 0
	}
	root := ui.Column(c).Fill().Background(semantic.Surface.Background).Bind(&s.root)
	animate := s.animate(c)
	chrome := chromeTarget
	if animate {
		chrome = components.Follow(root, "shell-chrome", chromeTarget, components.MotionSettle)
	}
	chrome = max(0, min(chrome, 1))
	s.lastChrome = chrome
	head := layout.Titlebar * scale / theme.DefaultUIFontSize
	scene := s.computeStage(root, width, height, head, animate)

	root.Children(func() {
		if scene.Field.Opacity > 0.001 {
			s.fieldView(c, semantic, scene, width, height, head, animate)
		}
		ui.Row(c).Key("body").Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().
			AlignItems(ui.Stretch).Children(func() {
			s.deskView(c, semantic, layout, chrome, scene, animate)
		})
		components.Floating(c, func() {
			s.controls(c, semantic, chrome, animate)
			if len(s.desk.columns) > 0 && s.posture == stage.PostureWriting {
				s.dock(c, semantic, 0)
			}
		})
	})
}
