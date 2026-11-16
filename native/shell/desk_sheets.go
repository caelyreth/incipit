package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The strip holds a column for each open sheet, each a page written in
// where it stands. The sheet in front is the one the writing actions act
// on; the pointer coming onto another, or writing in it, brings it in
// front (desk_front.go), and the strip goes as far as brings one that
// comes in front otherwise into view.

// sheetPad is the room beside a sheet's page in its column and below it
// past the writing dock, at the reference type size.
const sheetPad = 16

// openColumn is an open sheet's column, w wide: its page, set at reading
// measure in it, and its head (sheetHead) over it.
func (s *State) openColumn(c *ui.Context, semantic theme.Theme, page Page, w, origin float32, animate bool) {
	size := components.UIFontSize(c)
	k := size / theme.DefaultUIFontSize
	col, front, shown := s.column(c, deskColumn{sheet: page.ID}, w, origin, animate)
	col.Role(ui.RoleGroup).Label(page.Title).Clip().Background(semantic.Surface.Page)
	// The keyboard comes to it as the pointer does (desk_front.go).
	claim := s.desk.claim == page.ID
	if claim {
		s.desk.claim = ""
	}
	// Its head floats over the page's reading margin, shown while the
	// pointer is on it, as nothing else needs it.
	lead, trail := s.clearOfChrome(c, shown)
	head := float32(0)
	if col.Hovered() {
		head = 1
	}
	if animate {
		head = components.Follow(col, "head-shown", head, components.MotionNormal)
	}
	side := sheetPad * k
	prose := s.Settings.Prose()
	width := max(0, w-2*side)
	dock := theme.DefaultLayout.DockClearance() * k
	frame := SheetFrame{
		Prose: prose, UIFontSize: s.Settings.UIFontSize, LayoutWidth: width, Inset: prose.Inset(width),
		Top:    theme.DefaultLayout.ReadingTop * k * 3 / 2,
		Bottom: dock + side, Dock: dock,
		Focus: page.ID == s.ActivePage, Claim: claim,
	}
	col.Children(func() {
		ui.Column(c).Key("sheet-page").WidthPercent(100).Grow(1).MinHeight(0).PaddingX(side).
			Opacity(s.columnShows(col, front, animate)).Children(func() {
			if s.Sheet != nil {
				s.Sheet(c, page.ID, frame)
			}
		})
		s.sheetHead(c, semantic, page, k, lead, trail, head, animate)
	})
}

// sheetHead is a sheet's head (columnHead): its group's mark and its
// title, and what closes it, at the type's size k.
func (s *State) sheetHead(c *ui.Context, semantic theme.Theme, page Page, k, lead, trail, shown float32, animate bool) ui.Element {
	info := func() {
		ui.Row(c).Key("sheet-info").AlignItems(ui.Center).Gap(6 * k).PaddingX(12 * k).Children(func() {
			if group, ok := s.groupOf(page.ID); ok {
				ink := semantic.Icon.Muted
				if tint := groupTint(semantic, group); tint != nil {
					ink = *tint
				}
				ui.Icon(c, groupIcon(group).SVG()).FontSize(14 * k).Shrink(0).TextColor(ink)
			}
			components.Label(c, page.Title, theme.TextSmall, semantic.Text.Muted).SingleLine().MaxWidth(240 * k)
		})
	}
	// It names its sheet, which its tooltip need not.
	return s.columnHead(c, semantic, "sheet-head", lead, trail, shown, info, chromeButton{
		key: "sheet-close-" + page.ID, label: "Close " + page.Title, tooltip: "Close sheet",
		action: func() { s.CloseSheet(page.ID) },
	}, animate)
}

// sheetHint stands where the desk's columns would, while there are none:
// how to bring one. It is the desk's empty room, as the room around
// columns is.
func (s *State) sheetHint(c *ui.Context, semantic theme.Theme, w float32, animate bool) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	hint := ui.Column(c).Key("sheet-hint").Width(w).Shrink(0).FillHeight().Transition(columnTransition(animate)).
		AlignItems(ui.Center).Justify(ui.Center).Gap(12 * k)
	hint.ContextMenu(func(m *ui.Menu) {
		if m.Item("New Picker").Chosen() {
			s.newPickerAt(0)
		}
	})
	hint.Children(func() {
		ui.Icon(c, components.IconDocument.SVG()).FontSize(72 * k).TextColor(semantic.Icon.Muted).Opacity(0.35)
		components.Label(c, "Open a sheet from the library, or press ⌘N for a new one", theme.TextXSmall, semantic.Text.Muted).
			Opacity(0.8).TextAlign(ui.Center)
	})
}

// groupOf is the group a sheet is filed in.
func (s *State) groupOf(sheet string) (GroupItem, bool) {
	if s.Library == nil {
		return GroupItem{}, false
	}
	id := s.Library.SheetGroup(sheet)
	if id == "" {
		return GroupItem{}, false
	}
	return s.Library.Group(id)
}

// openPage is the open sheet id, or one of that id alone.
func (s *State) openPage(id string) Page {
	for _, page := range s.Open {
		if page.ID == id {
			return page
		}
	}
	return Page{ID: id}
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
