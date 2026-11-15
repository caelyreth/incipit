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

// openColumn is an open sheet's column: its surface (sheetSurface), a head
// with its group's mark, its title, and what closes it, over its page, w wide. A drag on the head moves the strip.
// The sheet in front stands here while the canvas is laid out, and on the
// page otherwise, its column keeping its place meanwhile.
func (s *State) openColumn(c *ui.Context, semantic theme.Theme, page Page, w, origin float32, animate bool) {
	size := components.UIFontSize(c)
	k := size / theme.DefaultUIFontSize
	prose := s.Settings.Prose()
	front := page.ID == s.ActivePage
	edge := semantic.Border.Variant
	if front {
		edge = semantic.Accent.Foreground
	}
	col := s.column(c, canvasColumn{sheet: page.ID}.key(), w, origin, animate).
		Role(ui.RoleGroup).Label(page.Title)
	col.Children(func() {
		// The sheet in front comes into view as it comes in front, and at
		// once as the canvas comes, for it to fly to.
		if front && s.canvas.follow || front && s.canvas.entered || page.ID == s.canvas.reveal {
			s.revealColumn(c, col, !s.canvas.entered && animate)
		}
		if front && s.laid != ModeCanvas {
			return
		}
		var head ui.Element
		s.sheetSurface(c, semantic, page.ID, sheetLook{radius: 10 * k, edge: edge, head: func() {
			head = ui.Row(c).Key("sheet-head").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
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
				// It names its sheet, which its tooltip need not.
				chromeButton{
					key: "sheet-close-" + page.ID, label: "Close " + page.Title, tooltip: "Close sheet",
					icon: components.IconClose, compact: true,
					action: func() { s.CloseSheet(page.ID) },
				}.view(c, semantic, animate)
			})
			ui.Box(c).WidthPercent(100).Height(1).Shrink(0).Background(semantic.Border.Variant)
		}}, func() {
			ui.Column(c).Key("sheet-page").WidthPercent(100).FillHeight().PaddingX(sheetPad * k).
				Children(func() {
					if s.Sheet != nil {
						s.Sheet(c, page.ID, SheetFrame{
							Prose: prose, UIFontSize: size, Top: 16 * k, Bottom: 48 * k, Focus: front,
						})
					}
				})
		})
		if !s.pan(head) && head.Clicked() && !front && s.SelectPage != nil {
			s.SelectPage(page.ID)
		}
	})
}

// sheetHint stands where the strip's columns would, while there are none:
// how to bring one. It is the canvas's empty room, as the room around
// columns is.
func (s *State) sheetHint(c *ui.Context, semantic theme.Theme, w, origin float32, animate bool) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	hint := s.column(c, "sheet-hint", w, origin, animate).AlignItems(ui.Center).Justify(ui.Center).Gap(12 * k)
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

// isOpen reports whether a sheet is open.
func (s *State) isOpen(id string) bool {
	for _, page := range s.Open {
		if page.ID == id {
			return true
		}
	}
	return false
}
