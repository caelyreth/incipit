package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The strip holds a column for each open sheet, each a page written in
// where it stands. The sheet in front is the one the writing actions act
// on; the pointer coming onto another, clicking its head, or writing in
// it brings it in front (canvas_front.go), and the strip goes as far as
// brings one that comes in front otherwise into view.

// sheetPad is the room beside a sheet's page in its column, and above it
// under the title line, at the reference type size.
const sheetPad = 16

// openColumn is an open sheet's column, w wide: its surface (sheetSurface),
// its head (sheetHead) over its page.
// The sheet in front stands here while the canvas is laid out, and on the
// page otherwise, its column keeping its place meanwhile.
func (s *State) openColumn(c *ui.Context, semantic theme.Theme, page Page, w, origin float32, animate bool) {
	size := components.UIFontSize(c)
	k := size / theme.DefaultUIFontSize
	prose := s.Settings.Prose()
	col, front, shown := s.column(c, canvasColumn{sheet: page.ID}, w, origin, animate)
	col.Role(ui.RoleGroup).Label(page.Title)
	// The keyboard comes to it as the pointer does (canvas_front.go).
	claim := s.canvas.claim == page.ID
	if claim {
		s.canvas.claim = ""
	}
	col.Children(func() {
		if page.ID == s.ActivePage && s.laid != ModeCanvas {
			return
		}
		// Its page begins under the title line, its head over it shown
		// while the pointer is on it, as nothing else needs it.
		lead, trail := s.clearOfChrome(c, shown)
		head := float32(0)
		if col.Hovered() {
			head = 1
		}
		s.sheetSurface(c, semantic, page.ID, sheetLook{
			top: titleLine(c) + sheetPad*k, side: sheetPad * k, shows: columnShows(col, front, animate),
			head:      func(shown float32) { s.sheetHead(c, semantic, page, k, lead, trail, shown, animate) },
			headShown: head,
		}, func(top float32) {
			if s.Sheet != nil {
				// Its page scrolls on under the writing actions floating
				// over the canvas's foot, and fades out there.
				dock := theme.DefaultLayout.DockClearance() * k
				s.Sheet(c, page.ID, SheetFrame{
					Prose: prose, UIFontSize: size, Top: top, Bottom: dock + sheetPad*k, Dock: dock,
					Focus: page.ID == s.ActivePage, Claim: claim,
				})
			}
		})
	})
}

// sheetHead is a sheet's head (columnHead): its group's mark and its
// title, and what closes it, at the type's size k.
func (s *State) sheetHead(c *ui.Context, semantic theme.Theme, page Page, k, lead, trail, shown float32, animate bool) ui.Element {
	info := func() {
		ui.Row(c).Key("sheet-info").AlignItems(ui.Center).Gap(6 * k).PaddingX(8 * k).Children(func() {
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

// sheetHint stands where the strip's columns would, while there are none:
// how to bring one. It is the canvas's empty room, as the room around
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
		ui.Icon(c, components.IconDocument.SVG()).FontSize(40 * k).TextColor(semantic.Icon.Muted).Opacity(0.35)
		components.Label(c, "Right-click the canvas, or use New picker, to choose a sheet", theme.TextXSmall, semantic.Text.Muted).
			Opacity(0.8).TextAlign(ui.Center)
	})
}

// groupOf is the group a sheet is filed in.
func (s *State) groupOf(sheet string) (GroupItem, bool) {
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
