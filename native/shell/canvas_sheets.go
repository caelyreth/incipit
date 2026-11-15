package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The strip holds a column for each open sheet, each a page written in
// where it stands. The one in front has the accent's edge, and the writing
// actions act on it; clicking another's head, or writing in it, brings it
// in front, and the strip goes as far as brings whichever comes in front
// into view.

// sheetPad is the room beside a sheet's page in its column, at the
// reference type size.
const sheetPad = 16

// openColumn is an open sheet's column: a head with its group's mark, its
// title, and what takes it back to the columns or closes it, over its
// page. A drag on the head moves the strip.
func (s *State) openColumn(c *ui.Context, semantic theme.Theme, page Page, origin float32, animate bool) {
	size := components.UIFontSize(c)
	k := size / theme.DefaultUIFontSize
	prose := s.Settings.Prose()
	front := page.ID == s.ActivePage
	edge := semantic.Border.Variant
	if front {
		edge = semantic.Accent.Foreground
	}
	col := s.column(c, "sheet-"+page.ID, origin, animate).
		Role(ui.RoleGroup).Label(page.Title).Clip().Radius(10*k).
		Background(semantic.Surface.Page).Border(1, edge)
	// The sheet that comes in front comes into view; as the canvas comes,
	// the picker before it brings it.
	switch {
	case front && s.canvas.follow && !s.canvas.entered, page.ID == s.canvas.reveal:
		spring := components.Spring(components.MotionScene)
		col.ScrollIntoView(ui.ScrollIntoViewOptions{Inline: ui.ScrollNearest, Animation: &spring})
	}
	col.Children(func() {
		head := ui.Row(c).Key("sheet-head").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
			Gap(6 * k).PaddingStart(sheetPad * k).PaddingEnd(6 * k).PaddingY(6 * k).
			Role(ui.RoleButton).Label("Bring " + page.Title + " in front")
		head.Children(func() {
			if group, ok := s.groupOf(page.ID); ok {
				ink := semantic.Icon.Muted
				if tint := groupTint(semantic, group); tint != nil {
					ink = *tint
				}
				ui.Icon(c, groupIcon(group).SVG()).FontSize(14 * k).Shrink(0).TextColor(ink)
			}
			title := semantic.Text.Muted
			if front {
				title = semantic.Text.Heading
			}
			components.Label(c, page.Title, theme.TextSmall, title).SingleLine().MinWidth(0).Grow(1).
				Transition(appearing(motionList, nil, nil))
			// Each names its sheet, which its tooltip need not.
			chromeButton{
				key: "sheet-columns-" + page.ID, label: "Write " + page.Title + " in columns",
				tooltip: "Write in columns", icon: components.IconSidebar, compact: true,
				action: func() { s.writeInColumns(page.ID) },
			}.view(c, semantic, animate)
			chromeButton{
				key: "sheet-close-" + page.ID, label: "Close " + page.Title, tooltip: "Close sheet",
				icon: components.IconClose, compact: true,
				action: func() {
					if s.ClosePageID != nil {
						s.ClosePageID(page.ID)
					}
				},
			}.view(c, semantic, animate)
		})
		if !s.pan(head) && head.Clicked() && !front && s.SelectPage != nil {
			s.SelectPage(page.ID)
		}
		ui.Box(c).WidthPercent(100).Height(1).Shrink(0).Background(semantic.Border.Variant)
		ui.Column(c).Key("sheet-page").WidthPercent(100).Grow(1).MinHeight(0).PaddingX(sheetPad * k).
			Children(func() {
				if s.Sheet != nil {
					s.Sheet(c, page.ID, SheetFrame{
						Prose: prose, UIFontSize: size, Top: 16 * k, Bottom: 48 * k, Focus: front,
					})
				}
			})
	})
}

// sheetHint stands where the strip's columns would, while there are none:
// how to bring one.
func (s *State) sheetHint(c *ui.Context, semantic theme.Theme, origin float32, animate bool) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	s.column(c, "sheet-hint", origin, animate).AlignItems(ui.Center).Justify(ui.Center).
		Gap(12 * k).Children(func() {
		ui.Icon(c, components.IconDocument.SVG()).FontSize(40 * k).TextColor(semantic.Icon.Muted).Opacity(0.35)
		components.Label(c, "Make a picker to choose a sheet", theme.TextXSmall, semantic.Text.Muted).
			Opacity(0.8).TextAlign(ui.Center)
	})
}

// writeInColumns leaves the canvas for the columns at a sheet, in front,
// at its group.
func (s *State) writeInColumns(id string) {
	s.closeCanvas()
	if s.SelectPage != nil {
		s.SelectPage(id)
	}
	s.reveal(id)
}

// openOnCanvas opens a sheet in a column of its own, after the others,
// in front; the strip goes to it.
func (s *State) openOnCanvas(id string) {
	if s.isOpen(id) {
		if s.SelectPage != nil {
			s.SelectPage(id)
		}
		s.canvas.reveal = id
		return
	}
	s.Library.OpenSheet(id, true)
}

// groupOf is the group a sheet is filed in.
func (s *State) groupOf(sheet string) (GroupItem, bool) {
	id := s.Library.SheetGroup(sheet)
	if id == "" {
		return GroupItem{}, false
	}
	return s.Library.Group(id)
}

// isOpen reports whether a sheet is open.
func (s *State) isOpen(id string) bool {
	for _, page := range s.Open {
		if page.ID == id {
			return true
		}
	}
	return false
}
