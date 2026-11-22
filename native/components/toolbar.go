package components

import (
	"strings"
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A tool bar draws groups of commands as a dock, whoever offers them: the
// selection's tools over the page and the writing dock under it draw
// their tools this one way, so they look and act alike. It knows nothing
// of what its tools do: each is an id its holder runs.

// ToolShape is how a group offers its tools.
type ToolShape uint8

const (
	// ToolToggles are buttons each turning something on or off, lit while
	// on.
	ToolToggles ToolShape = iota
	// ToolChoice is one of its tools, the one on named on its face, which
	// opens a menu of them all.
	ToolChoice
	// ToolActions are buttons each doing something once.
	ToolActions
)

// Tool is one command a bar offers.
type Tool struct {
	ID    string
	Label string
	Icon  IconName
	// Binding is its chord as spelled; Key the key that runs it while the
	// bar is armed or its menu open, Cap that key as its cap reads.
	Binding  string
	Key, Cap string
	Enabled  bool
	On       bool
}

// ToolGroup is a group of tools of one shape. A choice opens its menu on
// its own Key and Cap.
type ToolGroup struct {
	Key, Label string
	Shape      ToolShape
	Tools      []Tool
	OpenKey    string
	OpenCap    string
}

// ToolBar draws tool groups, then its holder's own groups (Extra).
// Keyed, its holder reads keys for it: its menus show each tool's key, and
// Armed, each tool shows the key that runs it on a small cap. Open names
// the choice whose menu shows, kept by the holder so its keys may open and
// close it too. Settle, where set, holds the groups as they were while a
// new set of them has not yet stood a moment (ToolSettle). Size is its
// buttons' (MediumButton unless set).
type ToolBar struct {
	Key, Label string
	Groups     []ToolGroup
	Extra      []ActionGroup
	Room       float32
	Animate    bool
	Side       HintSide
	Keyed      bool
	Armed      bool
	Open       *string
	Settle     *ToolSettle
	Size       ButtonSize
	// Move moves it to a new place on the motion of its width, as
	// ActionDock.Move: for a holder placing it at once.
	Move bool
	Run  func(id string)
}

// toolSettle is how long a new set of a bar's groups stands before the
// bar takes it.
const toolSettle = 200 * time.Millisecond

// ToolSettle keeps a bar's groups from coming and going as fast as what
// they are offered for changes, as a selection being dragged: the groups
// shown stay those they were until a new set of them has stood
// toolSettle, each shown group taking its tools' states as they are now
// wherever it is still offered alike.
type ToolSettle struct {
	shown       []ToolGroup
	shape, next string
	since       time.Time
}

// settled is the groups to show at now, and how long until a new set
// will have stood, if one waits.
func (s *ToolSettle) settled(groups []ToolGroup, now time.Time) ([]ToolGroup, time.Duration) {
	shape := toolShapeOf(groups)
	if s.shown == nil || shape == s.shape {
		s.shown, s.shape, s.next = groups, shape, ""
		return groups, 0
	}
	if shape != s.next {
		s.next, s.since = shape, now
	}
	if wait := toolSettle - now.Sub(s.since); wait > 0 {
		out := make([]ToolGroup, len(s.shown))
		for i, g := range s.shown {
			out[i] = g
			for _, current := range groups {
				if current.Key == g.Key && toolShapeOf([]ToolGroup{current}) == toolShapeOf([]ToolGroup{g}) {
					out[i] = current
				}
			}
		}
		return out, wait
	}
	s.shown, s.shape, s.next = groups, shape, ""
	return groups, 0
}

// toolShapeOf names which groups and tools a set holds.
func toolShapeOf(groups []ToolGroup) string {
	var b strings.Builder
	for _, g := range groups {
		b.WriteString(g.Key)
		b.WriteByte('(')
		for _, t := range g.Tools {
			b.WriteString(t.ID)
			b.WriteByte(' ')
		}
		b.WriteByte(')')
	}
	return b.String()
}

func (b ToolBar) View(c *ui.Context, semantic theme.Theme) ui.Element {
	tools := b.Groups
	if b.Settle != nil {
		var wait time.Duration
		if tools, wait = b.Settle.settled(tools, c.Now()); wait > 0 {
			c.After(wait)
		}
	}
	if b.Size == 0 {
		b.Size = MediumButton
	}
	groups := make([]ActionGroup, 0, len(tools)+len(b.Extra))
	for _, g := range tools {
		if len(g.Tools) == 0 {
			continue
		}
		var actions []func()
		if g.Shape == ToolChoice {
			actions = append(actions, func() { b.choice(c, semantic, g) })
		} else {
			for _, t := range g.Tools {
				actions = append(actions, func() { b.button(c, semantic, t) })
			}
		}
		groups = append(groups, ActionGroup{Key: g.Key, Label: g.Label, Actions: actions})
	}
	groups = append(groups, b.Extra...)
	return ActionDock{Key: b.Key, Label: b.Label, Groups: groups, Room: b.Room, Animate: b.Animate, Move: b.Move}.View(c, semantic)
}

func (b ToolBar) run(id string) {
	if b.Run != nil {
		b.Run(id)
	}
}

// button is a toggle's or an action's button, its cap over its corner
// while armed.
func (b ToolBar) button(c *ui.Context, semantic theme.Theme, t Tool) {
	ui.Box(c.Key("tool-" + t.ID)).Children(func() {
		IconButton{
			Key: t.ID, Label: t.Label, Binding: t.Binding, HintSide: b.Side,
			Icon: t.Icon.SVG(), Style: SubtleButton, Size: b.Size,
			Selected: t.On, Disabled: !t.Enabled, Animate: b.Animate,
			IconColor: semantic.Icon.Muted,
			Action:    func() { b.run(t.ID) },
		}.View(c, semantic)
		b.cap(c, semantic, t.Cap, b.Keyed && b.Armed && t.Enabled)
	})
}

// cap is a tool's key on a small cap at its button's lower corner, coming
// as the bar is armed.
func (b ToolBar) cap(c *ui.Context, semantic theme.Theme, key string, shown bool) {
	if key == "" || !b.Keyed {
		return
	}
	scale := UIFontSize(c)
	unit := scale / theme.DefaultUIFontSize
	style := KeycapStyle(semantic, scale, true)
	style.Height, style.PadX, style.Radius, style.Face = 14*unit, 3*unit, 4*unit, semantic.Surface.Inset
	amount := map[bool]float32{true: 1}[shown]
	el := Keycaps(c, style, []string{key}, "").Absolute().Right(-3 * unit).Bottom(-3 * unit).IgnoreInput(true)
	if b.Animate {
		amount = Follow(el, "cap", amount, MotionQuick)
	}
	el.Opacity(amount).Scale(0.8+0.2*amount, 0.8+0.2*amount)
}

// choice is a choice's button, naming the tool on, which opens the menu
// of its tools below it: each with its icon, its chord and its key, the
// one on checked.
func (b ToolBar) choice(c *ui.Context, semantic theme.Theme, g ToolGroup) {
	label := g.Label
	enabled := false
	for _, t := range g.Tools {
		if t.On {
			label = t.Label
		}
		enabled = enabled || t.Enabled
	}
	open := b.Open != nil && *b.Open == g.Key
	var face ui.Element
	ui.Box(c.Key("choice-" + g.Key)).Children(func() {
		face = ChoiceButton{
			Key: g.Key, Label: label, Tooltip: g.Label, HintSide: b.Side, Size: b.Size,
			Disabled: !enabled, Animate: b.Animate, Selected: open,
			Action: func() {
				if b.Open != nil {
					*b.Open = map[bool]string{false: g.Key}[open]
				}
			},
		}.View(c, semantic)
		b.cap(c, semantic, g.OpenCap, b.Keyed && b.Armed && enabled && !open)
	})
	if b.Open == nil {
		return
	}
	shown := open
	ui.PopoverBase(c, face, &shown, func(panel ui.Element) {
		b.menu(c, semantic, panel, g)
	})
	if open && !shown {
		*b.Open = ""
	}
}

// menu is a choice's open menu: a row for each of its tools.
func (b ToolBar) menu(c *ui.Context, semantic theme.Theme, panel ui.Element, g ToolGroup) {
	scale := UIFontSize(c)
	pad := theme.Space04.DIPs(scale)
	row := theme.DefaultControls.Scaled(scale).Height
	OutlinedSurface(panel, semantic, scale)
	panel.Padding(pad, pad, pad, pad).MinWidth(220 * scale / theme.DefaultUIFontSize).
		Role(ui.RoleMenu).Label(g.Label)
	if b.Animate {
		spring := Spring(MotionNormal)
		rise := -theme.Space04.DIPs(scale)
		panel.Transition(ui.ElementTransition{
			Animation: &spring, Enter: &ui.Motion{Y: rise, Opacity: 0},
			Exit: &ui.Motion{Y: rise / 2, Opacity: 0}, ExitDuration: MotionQuick,
		})
	}
	style := KeycapStyle(semantic, scale, true)
	panel.Children(func() {
		for _, t := range g.Tools {
			item := ui.ButtonBase(c.Key("item-" + t.ID)).Role(ui.RoleMenuItem).Label(t.Label).
				Checked(t.On).Disabled(!t.Enabled).KeepFocus().FocusRing(false).
				Height(row).PaddingX(theme.Space08.DIPs(scale)).Gap(theme.Space08.DIPs(scale)).
				AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(scale).Radius)
			if !t.Enabled {
				item.Opacity(DisabledOpacity)
			} else if item.Hovered() {
				item.Background(semantic.GhostElement.Hover)
			}
			if t.Enabled && item.Clicked() {
				if b.Open != nil {
					*b.Open = ""
				}
				b.run(t.ID)
			}
			item.Children(func() {
				mark := ui.Box(c).Size(14*scale/theme.DefaultUIFontSize, 14*scale/theme.DefaultUIFontSize)
				if t.On {
					mark.Children(func() {
						ui.Icon(c, IconCheck.SVG()).FontSize(14 * scale / theme.DefaultUIFontSize).
							TextColor(semantic.Accent.Foreground)
					})
				}
				label := CenteredLabel(c, t.Label, theme.TextSmall, semantic.Text.Default)
				label.Grow(1)
				if t.Binding != "" {
					CenteredLabel(c, t.Binding, theme.TextXSmall, semantic.Text.Muted)
				}
				if b.Keyed && t.Cap != "" {
					Keycaps(c, style, []string{t.Cap}, "")
				}
			})
		}
	})
}
