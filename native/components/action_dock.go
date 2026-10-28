package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type ActionGroup struct {
	Label   string
	Actions []func()
}

// ActionDock is the floating source-material boundary around grouped actions.
type ActionDock struct {
	Key    string
	Label  string
	Groups []ActionGroup
}

func (d ActionDock) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	return ui.Row(c).Key(d.Key).Role(ui.RoleToolbar).Label(d.Label).
		MaxWidthPercent(100).Wrap().AlignItems(ui.Center).Justify(ui.Center).
		Gap(theme.Space06.DIPs(scale)).Padding(theme.Space04.DIPs(scale)).
		KeepFocus().
		Radius(metrics.DockRadius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Panel).Children(func() {
		for index, group := range d.Groups {
			if index > 0 {
				VerticalDivider(c, semantic)
			}
			label := group.Label
			if label == "" {
				label = d.Label
			}
			ui.Row(c).Role(ui.RoleGroup).Label(label).
				AlignItems(ui.Center).Gap(theme.Space02.DIPs(scale)).
				Children(func() {
					for _, action := range group.Actions {
						action()
					}
				})
		}
	})
}
