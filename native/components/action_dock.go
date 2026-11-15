package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// ActionGroup is one group of a dock's actions. Key names it among the
// dock's groups, so a group that comes or goes is known as itself, not by
// its place.
type ActionGroup struct {
	Key     string
	Label   string
	Actions []func()
}

// ActionDock is the floating source-material boundary around grouped
// actions. A group that comes or goes opens or closes up in the dock, its
// divider with it, the groups beside it moving over and the dock growing
// or shrinking about them, so it reads as one that changes, not one
// swapped for another.
//
// Room is how wide it may be. It keeps to one line, and wraps only when
// that line no longer fits the room: laid out on its way between two
// widths, it would otherwise wrap its last group onto a second line.
type ActionDock struct {
	Key     string
	Label   string
	Groups  []ActionGroup
	Room    float32
	Animate bool
	// Bare is how far its pill gives way, from none of it to all: its
	// room about its buttons and its edge going, its corners closing in to
	// theirs, its buttons standing on its ground alone, as tall as they.
	Bare float32
	// Move animates where it stands as well, as it goes from one place to
	// another, on the motion of its width. It is for a move made at once:
	// one where its holder moves frame by frame would trail it.
	Move bool
}

func (d ActionDock) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	gap := theme.Space06.DIPs(scale)
	// Its corners keep round about its buttons' as its room about them.
	room := theme.Space04.DIPs(scale)
	keeps := 1 - max(0, min(d.Bare, 1))
	pad := room * keeps
	dock := ui.Row(c).Key(d.Key).Role(ui.RoleToolbar).Label(d.Label).
		AlignItems(ui.Center).Justify(ui.Start).Clip().
		Padding(pad).
		KeepFocus().FocusGroup(ui.Horizontal).
		Radius(metrics.DockRadius-(room-pad)).Border(keeps, semantic.Border.Variant.Alpha(keeps)).
		Background(semantic.Surface.Panel).Transition(dockTransition(d.Animate, d.Move))
	// How wide its one line was when last laid out on one line.
	line := ui.Local(dock, "line", func() float32 { return 0 })
	wrapped := ui.Local(dock, "wrapped", func() bool { return false })
	if b := dock.LayoutBounds(); !*wrapped && b.W > 0 {
		*line = b.W
	}
	*wrapped = d.Room > 0 && *line > d.Room
	if *wrapped {
		dock.MaxWidthPercent(100).Wrap().Justify(ui.Center)
	}
	return dock.Children(func() {
		for index, group := range d.Groups {
			label := group.Label
			if label == "" {
				label = d.Label
			}
			// The room before a group, and its divider, are the group's,
			// so they close up with it.
			row := ui.Row(c).Key("group-" + group.Key).Shrink(0).AlignItems(ui.Center).Gap(gap).
				Transition(groupTransition(d.Animate))
			if index > 0 {
				row.PaddingStart(gap)
			}
			row.Children(func() {
				if index > 0 {
					VerticalDivider(c, semantic)
				}
				ui.Row(c).Role(ui.RoleGroup).Label(label).
					AlignItems(ui.Center).Gap(theme.Space02.DIPs(scale)).
					Children(func() {
						for _, action := range group.Actions {
							action()
						}
					})
			})
		}
	})
}

// groupTransition opens a dock's group in from nothing across, its
// actions growing in, and closes it the same way, the groups after it
// moving over as the dock grows or shrinks about them (dockTransition).
func groupTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	gone := &ui.Motion{Opacity: 0, Scale: 0.8, Collapse: true}
	spring := Bouncy(MotionDock)
	return ui.ElementTransition{Animation: &spring, Position: true, Colors: true, Enter: gone, Exit: gone}
}

// dockTransition grows and shrinks the dock about its groups as they come
// and go, on their motion, its content showing as far as it has grown:
// MyGo collapses a group alone, its parent laid out at once at its new
// size. With move it goes to its new place on the same motion, so a dock
// held by its middle stays about it. The dock pops in and out where it
// stands. Where it stands is its holder's to move: a transition of its
// place would trail behind every move of what holds it.
func dockTransition(animate, move bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	pop := &ui.Motion{Scale: 0.9, Opacity: 0}
	spring := Bouncy(MotionDock)
	return ui.ElementTransition{
		Animation: &spring, Position: move, Size: true, Colors: true,
		Enter: pop, Exit: pop,
	}
}
