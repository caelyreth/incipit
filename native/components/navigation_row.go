package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// NavigationRow is the compact 32 DIP row shared by the sidebar and settings
// tree. The caller supplies leading/trailing content through children.
type NavigationRow struct {
	// Base, when set, supplies the row's behavior, as a TreeBase item does.
	Base      ui.Element
	Key       string
	Label     string
	Selected  bool
	Role      ui.Role
	TextColor *ui.Color
	FullWidth bool
	Indent    float32
	Animate   bool
	Leading   func()
	Trailing  func()
	Action    func()
	// Track, when set, is the list's shared selection highlight: a selected
	// row marks itself on it instead of painting its own background.
	Track *SelectionTrack
	// Prominent is a row of a list a surface is for, as the command
	// palette's: a list row's height and the default text.
	Prominent bool
}

// NavigationSelection is the shared highlight style for navigation lists.
func NavigationSelection(c *ui.Context, semantic theme.Theme) SelectionStyle {
	return SelectionStyle{
		Fill:        semantic.Surface.Chosen,
		Marker:      semantic.Accent.Foreground,
		Radius:      theme.DefaultControls.Scaled(UIFontSize(c)).Radius,
		Picked:      semantic.Accent.Muted,
		PickedHover: semantic.Accent.Foreground.Alpha(0.1),
		Ring:        semantic.Accent.Foreground.Alpha(0.85),
	}
}

func (r NavigationRow) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	height, text := metrics.Height, theme.TextSmall
	if r.Prominent {
		height, text = metrics.Row, theme.TextDefault
	}
	row := r.Base
	if !row.Valid() {
		row = ui.Row(c).Key(r.Key).Role(r.Role)
	}
	row.Label(r.Label).Selected(r.Selected).
		Height(height).Shrink(0).
		PaddingStart(theme.Space08.DIPs(scale)+r.Indent).
		PaddingEnd(theme.Space08.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).Border(1, ui.Transparent).
		Focusable().FocusRing(false)
	if r.FullWidth {
		row.WidthPercent(100)
	}
	textColor := semantic.Text.Default
	if r.Selected {
		textColor = semantic.Text.Heading
	}
	if r.TextColor != nil {
		textColor = *r.TextColor
	}
	rest := semantic.GhostElement.Background
	switch {
	case r.Selected && r.Track != nil:
		r.Track.Mark(row)
	case r.Selected:
		rest = semantic.Surface.Inset
	}
	// The selected row shows the selection; hovering it adds nothing.
	hover := HoverAmount(row, !r.Selected && row.Hovered(), r.Animate)
	row.Background(blend(rest, semantic.GhostElement.Hover, hover))
	if row.FocusVisible() {
		row.Border(1, semantic.Border.Focused)
	}
	row.Children(func() {
		if r.Leading != nil {
			r.Leading()
		}
		CenteredLabel(c, r.Label, text, textColor).Grow(1).MinWidth(0)
		if r.Trailing != nil {
			r.Trailing()
		}
	})
	clicked := row.Clicked()
	activated := row.Shortcut(0, ui.KeyEnter) || row.Shortcut(0, ui.KeySpace)
	if (clicked || activated) && r.Action != nil {
		r.Action()
	}
	return row
}
