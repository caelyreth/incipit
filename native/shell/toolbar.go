package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) formatBar(c *ui.Context, semantic theme.Theme, room float32) {
	animate := s.animate(c)
	Icon := func(
		key, label, binding string,
		icon components.IconName,
		selected, enabled bool,
		action func(),
	) func() {
		return func() {
			components.IconButton{
				Key: key, Label: label, Tooltip: components.CommandTooltip(label, binding),
				Icon:  icon.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				Selected: selected, Disabled: !enabled, Animate: animate,
				IconColor: &semantic.Icon.Muted, Action: action,
			}.View(c, semantic)
		}
	}
	groups := []components.ActionGroup{
		{
			Key: "formatting", Label: "Formatting",
			Actions: []func(){
				Icon("strong", "Strong", s.Toolbar.Bindings.Strong, components.IconBold,
					s.Toolbar.Formatting.Strong.On, s.Toolbar.Formatting.Strong.Enabled,
					func() {
						if s.Strong != nil {
							s.Strong()
						}
					}),
				Icon("emphasis", "Emphasis", s.Toolbar.Bindings.Emphasis, components.IconItalic,
					s.Toolbar.Formatting.Emphasis.On, s.Toolbar.Formatting.Emphasis.Enabled,
					func() {
						if s.Emphasis != nil {
							s.Emphasis()
						}
					}),
				Icon("code", "Code", s.Toolbar.Bindings.Code, components.IconCode,
					s.Toolbar.Formatting.Code.On, s.Toolbar.Formatting.Code.Enabled,
					func() {
						if s.Code != nil {
							s.Code()
						}
					}),
				Icon("strike", "Strike", s.Toolbar.Bindings.Strike, components.IconStrike,
					s.Toolbar.Formatting.Strike.On, s.Toolbar.Formatting.Strike.Enabled,
					func() {
						if s.Strike != nil {
							s.Strike()
						}
					}),
			},
		},
		{
			Key: "presentation", Label: "Presentation",
			Actions: []func(){
				Icon("typewriter", "Typewriter", s.Toolbar.Bindings.Typewriter, components.IconTypewriter,
					s.Toolbar.Typewriter, s.Toolbar.TypewriterEnabled,
					func() {
						if s.Typewriter != nil {
							s.Typewriter(!s.Toolbar.Typewriter)
						}
					}),
				Icon("zen", "Zen", s.Toolbar.Bindings.Zen, components.IconZen,
					s.Toolbar.Zen, s.Toolbar.ZenEnabled,
					func() {
						if s.Zen != nil {
							s.Zen(!s.Toolbar.Zen)
						}
					}),
			},
		},
	}
	// The desk's own: what makes its columns.
	groups = append(groups, components.ActionGroup{Key: "desk", Label: "Desk actions", Actions: []func(){
		Icon("new-picker", "New picker", "", components.IconFolderSearch, false, true, s.NewPicker),
	}})
	// It goes across on its own motion when it stands elsewhere for other
	// groups, which it does at once; following the panes as they open and
	// close, it is laid out where they leave it, frame by frame.
	at := ""
	for _, group := range groups {
		at += "/" + group.Key
	}
	now := c.Now()
	if at != s.dockAt {
		if s.dockAt != "" {
			s.dockMoving = now.Add(components.MotionDock + components.MotionQuick)
		}
		s.dockAt = at
	}
	components.ActionDock{
		Key: "writing-actions", Label: "Writing actions", Groups: groups, Room: room,
		Animate: animate, Move: now.Before(s.dockMoving),
	}.View(c, semantic)
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
	ui.Row(c).Key("dock").Absolute().Left(0).Right(0).Bind(&s.floating[1]).
		Bottom(theme.DefaultLayout.DockOffset * scale / theme.DefaultUIFontSize).
		PaddingStart(left + pad).PaddingEnd(pad).
		Justify(ui.Center).PassThrough().Children(func() {
		if s.finder.open {
			s.findBarView(c, semantic, animate)
			return
		}
		s.formatBar(c, semantic, room)
	})
}
