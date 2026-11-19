package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type dockState struct {
	ctx      *ui.Context
	semantic theme.Theme
	animate  bool
	groups   []components.ActionGroup
}

func (s *State) dockIcon(
	key, label, binding string,
	icon components.IconName,
	selected, enabled bool,
	action func(),
) {
	c := s.actionDock.ctx
	semantic := s.actionDock.semantic
	components.IconButton{
		Key: key, Label: label, Tooltip: components.CommandTooltip(label, binding),
		Icon:  icon.SVG(),
		Style: components.SubtleButton, Size: components.MediumButton,
		Selected: selected, Disabled: !enabled, Animate: s.actionDock.animate,
		IconColor: &s.actionDock.semantic.Icon.Muted, Action: action,
	}.View(c, semantic)
}

func (s *State) ensureDockGroups() {
	if len(s.actionDock.groups) != 0 {
		return
	}
	onStrong := func() {
		if s.Strong != nil {
			s.Strong()
		}
	}
	onEmphasis := func() {
		if s.Emphasis != nil {
			s.Emphasis()
		}
	}
	onCode := func() {
		if s.Code != nil {
			s.Code()
		}
	}
	onStrike := func() {
		if s.Strike != nil {
			s.Strike()
		}
	}
	onTypewriter := func() {
		if s.Typewriter != nil {
			s.Typewriter(!s.Toolbar.Typewriter)
		}
	}
	onZen := func() {
		if s.Zen != nil {
			s.Zen(!s.Toolbar.Zen)
		}
	}
	s.actionDock.groups = []components.ActionGroup{
		{
			Key: "formatting", Label: "Formatting",
			Actions: []func(){
				func() {
					s.dockIcon("strong", "Strong", s.Toolbar.Bindings.Strong, components.IconBold,
						s.Toolbar.Formatting.Strong.On, s.Toolbar.Formatting.Strong.Enabled, onStrong)
				},
				func() {
					s.dockIcon("emphasis", "Emphasis", s.Toolbar.Bindings.Emphasis, components.IconItalic,
						s.Toolbar.Formatting.Emphasis.On, s.Toolbar.Formatting.Emphasis.Enabled, onEmphasis)
				},
				func() {
					s.dockIcon("code", "Code", s.Toolbar.Bindings.Code, components.IconCode,
						s.Toolbar.Formatting.Code.On, s.Toolbar.Formatting.Code.Enabled, onCode)
				},
				func() {
					s.dockIcon("strike", "Strike", s.Toolbar.Bindings.Strike, components.IconStrike,
						s.Toolbar.Formatting.Strike.On, s.Toolbar.Formatting.Strike.Enabled, onStrike)
				},
			},
		},
		{
			Key: "presentation", Label: "Presentation",
			Actions: []func(){
				func() {
					s.dockIcon("typewriter", "Typewriter", s.Toolbar.Bindings.Typewriter, components.IconTypewriter,
						s.Toolbar.Typewriter, s.Toolbar.TypewriterEnabled, onTypewriter)
				},
				func() {
					s.dockIcon("zen", "Zen", s.Toolbar.Bindings.Zen, components.IconZen,
						s.Toolbar.Zen, s.Toolbar.ZenEnabled, onZen)
				},
			},
		},
	}
}

func (s *State) formatBar(c *ui.Context, semantic theme.Theme, room float32) {
	animate := s.animate(c)
	s.ensureDockGroups()
	s.actionDock.ctx = c
	s.actionDock.semantic = semantic
	s.actionDock.animate = animate
	// It goes across on its own motion when it stands elsewhere for other
	// groups, which it does at once; following the panes as they open and
	// close, it is laid out where they leave it, frame by frame.
	const dockPath = "/formatting/presentation"
	now := c.Now()
	if dockPath != s.dockAt {
		if s.dockAt != "" {
			s.dockMoving = now.Add(components.MotionDock + components.MotionQuick)
		}
		s.dockAt = dockPath
	}
	components.ActionDock{
		Key: "writing-actions", Label: "Writing actions", Groups: s.actionDock.groups, Room: room,
		Animate: animate, Move: now.Before(s.dockMoving),
	}.View(c, semantic)
	s.actionDock.ctx = nil
}

// dock is what floats over the desk's foot, left in from the window's
// left edge: the writing actions, or a sheet's find in their place while
// it is open, moving with the desk as the panes open and close.
func (s *State) dock(c *ui.Context, semantic theme.Theme, left float32) {
	scale := s.Settings.UIFontSize
	animate := s.animate(c)
	pad := theme.Space16.DIPs(scale)
	width, _ := c.Size()
	room := width - left - 2*pad
	// The row spans the window, and keeps clear of the panes by its
	// padding: a dock moving with it (formatBar) moves from where it was
	// in it, which is where it was in the window.
	ui.Row(c).Key("dock").Absolute().Left(0).Right(0).Bind(&s.dockHandle).
		Bottom(theme.DefaultLayout.DockOffset * scale / theme.DefaultUIFontSize).
		PaddingStart(left + pad).PaddingEnd(pad).
		Justify(ui.Center).PassThrough().
		// It comes in over the last half of the way to the desk, and goes in
		// the first half of the way back.
		Opacity(max(0, min(1, s.view*2-1))).Children(func() {
		if s.finder.open {
			s.findBarView(c, semantic, animate)
			return
		}
		s.formatBar(c, semantic, room)
	})
}
