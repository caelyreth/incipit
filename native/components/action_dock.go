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
// divider with it, and the groups beside it move over, so the dock reads
// as one that changes, not one swapped for another.
type ActionDock struct {
	Key     string
	Label   string
	Groups  []ActionGroup
	Animate bool
}

func (d ActionDock) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	gap := theme.Space06.DIPs(scale)
	return ui.Row(c).Key(d.Key).Role(ui.RoleToolbar).Label(d.Label).
		MaxWidthPercent(100).Wrap().AlignItems(ui.Center).Justify(ui.Center).
		Padding(theme.Space04.DIPs(scale)).
		KeepFocus().FocusGroup(ui.Horizontal).
		Radius(metrics.DockRadius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Panel).Children(func() {
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
// actions growing in, and closes it the same way.
func groupTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	gone := &ui.Motion{Opacity: 0, Scale: 0.8, Collapse: true}
	return ui.ElementTransition{Duration: MotionPanel, Position: true, Colors: true, Enter: gone, Exit: gone}
}
