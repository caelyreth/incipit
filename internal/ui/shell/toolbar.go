package shell

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// dockState is the dock's between frames.
type dockState struct {
	// open is the tool choice whose menu shows, and settle holds its
	// groups while what they are offered for keeps changing.
	open   string
	settle components.ToolSettle
}

// dockGroups are the dock's own actions, after the page's tools: the
// writing modes, drawn in c.
func (s *State) dockGroups(c *ui.Context, semantic theme.Theme, animate bool) []components.ActionGroup {
	typewriter := func() {
		components.IconButton{
			Key: "typewriter", Label: locales.Typewriter(), Binding: s.Toolbar.Bindings.Typewriter,
			Icon:  components.IconTypewriter.SVG(),
			Style: components.SubtleButton, Size: components.MediumButton,
			Selected: s.Toolbar.Typewriter, Disabled: !s.Toolbar.TypewriterEnabled, Animate: animate,
			IconColor: semantic.Icon.Muted, Action: func() {
				if s.Typewriter != nil {
					s.Typewriter(!s.Toolbar.Typewriter)
				}
			},
		}.View(c, semantic)
	}
	return []components.ActionGroup{{Key: "presentation", Label: locales.Presentation(), Actions: []func(){typewriter}}}
}

func (s *State) formatBar(c *ui.Context, semantic theme.Theme, room float32) {
	animate := s.animate(c)
	var tools []components.ToolGroup
	if s.Tools != nil {
		tools = s.Tools()
	}
	components.ToolBar{
		Key: "writing-actions", Label: locales.WritingActions(), Groups: tools, Extra: s.dockGroups(c, semantic, animate),
		Room: room, Animate: animate, Side: components.HintAbove, Open: &s.actionDock.open,
		Settle: &s.actionDock.settle, Move: true,
		Run: func(id string) {
			if s.RunTool != nil {
				s.RunTool(id)
			}
		},
	}.View(c, semantic)
}

// dock floats over the desk's foot: the writing actions, or a sheet's find
// in their place while it is open.
func (s *State) dock(c *ui.Context, semantic theme.Theme) {
	scale := s.Settings.UIFontSize
	animate := s.animate(c)
	pad := theme.Space16.DIPs(scale)
	width, _ := c.Size()
	room := width - 2*pad
	ui.Row(c).Key("dock").Absolute().Left(0).Right(0).Bind(&s.dockHandle).
		Bottom(theme.DefaultLayout.DockOffset * scale / theme.DefaultUIFontSize).
		PaddingX(pad).
		Justify(ui.Center).PassThrough().
		// It comes in over the last half of the way to the desk, and goes in
		// the first half of the way back.
		// Under the desk's switcher it fades with the desk behind the
		// guard, and takes no pointer.
		Opacity(max(0, min(1, s.view*2-1)) * (1 - guardDim*s.desk.switcher.grown)).
		IgnoreInput(s.desk.switcher.open).Children(func() {
		if s.finder.open {
			s.findBarView(c, semantic, animate)
			return
		}
		s.formatBar(c, semantic, room)
	})
}
