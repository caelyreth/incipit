package shell

import (
	"slices"

	"github.com/caelyreth/incipit/internal/text/query"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/ui"
)

func filing(g GroupItem) takes {
	return takes{sheets: true, groups: !g.Inbox}
}

// fileInto files what was dropped on g in it: the sheets it carries, out
// of the tray if they came from it, or a group.
func (s *State) fileInto(g GroupItem, dropped shellDrag) {
	switch d := dropped.(type) {
	case sheetDrag, trayDrag:
		s.moveSheets(s.carried(d), g.ID)
		s.delivered(d)
	case groupDrag:
		if d.id != g.ID {
			s.moveGroup(d.id, g.ID)
		}
	}
}

// entryRow is a sheet in a block, in full, as the sheet list showed it:
// pressed it opens on the desk, with Option alongside the sheets open;
// Cmd or Shift selects it (picks.go), and a menu on it, selected with
// others, is for the selection.

func deleteKey(id string) string { return "delete-" + id }

// groupMenu is the menu of the group g, its colors in semantic's hues.
func (s *State) groupMenu(semantic theme.Theme, g GroupItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			if components.MenuItem(m.Item(locales.PutBack()), components.IconUndo).Chosen() {
				s.restoreGroup(g)
			}
			m.Separator()
			if components.MenuItem(m.Item(deleteGroupLabel(g)), components.IconTrash).Danger().Chosen() {
				s.arm(deleteKey("group:" + g.ID))
			}
			return
		}
		if !g.Smart() {
			if components.MenuItem(m.Item(locales.NewSheet()), components.IconNewSheet).Chosen() {
				s.Library.NewSheet(g.ID, Last)
				s.ShowDesk()
			}
		}
		s.verbItem(m, verbExport, locales.ExportEllipsis(), components.IconExport, true, func() { s.Library.Export("group:" + g.ID) })
		if g.Inbox {
			return
		}
		if !g.Smart() && components.MenuItem(m.Item(locales.NewGroupInside()), components.IconNewGroup).Chosen() {
			s.newGroup(g.ID)
		}
		m.Separator()
		s.pinItem(m, "group:"+g.ID, g.Pinned)
		s.keepItem(m, g)
		if !g.Smart() {
			s.verbItem(m, VerbRename, locales.RenameEllipsis(), components.IconPen, true, func() { s.startRename(g.ID) })
		}
		// Its colors stand in a row of swatches, each a disc of its hue.
		components.MenuItem(m.Palette(locales.Color(), func(m *ui.Menu) {
			for color := TintMoonveil; color < TintHalo+len(theme.AllAccents); color++ {
				if m.Item(tintName(color)).Swatch(tintInk(semantic, color)).Radio(g.Color == color).Chosen() {
					s.styleGroup(g, g.Icon, color)
				}
			}
		}), components.IconPalette)
		s.moveMenu(m, func(target string) { s.moveGroup(g.ID, target) }, g.ID)
		m.Separator()
		s.verbItem(m, VerbTrash, trashGroupLabel(g), components.IconTrash, true, func() { s.trashGroup(g) })
	}
}

// pinItem is the menu's Pin, or Unpin, of the object id, which then
// stands where its pin puts it, brought into view.
func (s *State) pinItem(m *ui.Menu, id string, pinned bool) {
	s.verbItem(m, VerbPin, map[bool]string{true: locales.Unpin(), false: locales.Pin()}[pinned], components.IconPin, true, func() { s.pin(id, !pinned) })
}

// sheetMenu is the menu of the sheet id, pinned or not. It holds the sheet
// by its id alone: a row's menu is made at every frame it is drawn, and
// what the menu does reads the sheet as it is once chosen.
func (s *State) sheetMenu(id string, pinned, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			s.verbItem(m, VerbTrash, locales.PutBack(), components.IconUndo, true, func() { s.restoreSheets([]string{id}) })
			m.Separator()
			if components.MenuItem(m.Item(locales.DeleteImmediately()), components.IconTrash).Danger().Chosen() {
				s.arm(deleteKey("sheet:" + id))
			}
			return
		}
		if components.MenuItem(m.Item(locales.Open()), components.IconFileText).Chosen() {
			s.Library.OpenSheet(id, false)
			s.ShowDesk()
		}
		if components.MenuItem(m.Item(locales.OpenAlongside()), components.IconColumns).Chosen() {
			s.Library.OpenSheet(id, true)
			s.ShowDesk()
		}
		if g := s.Library.SheetGroup(id); g != "" && g != s.scope() {
			if components.MenuItem(m.Item(locales.ShowInGroup()), components.IconFolderSearch).Chosen() {
				s.showInGroup(id)
			}
		}
		m.Separator()
		s.verbItem(m, VerbDuplicate, locales.Duplicate(), components.IconDuplicate, true, func() { s.duplicate([]string{id}) })
		s.pinItem(m, "sheet:"+id, pinned)
		s.moveMenu(m, func(group string) { s.moveSheets([]string{id}, group) }, "")
		s.verbItem(m, verbExport, locales.ExportEllipsis(), components.IconExport, true, func() { s.Library.Export("sheet:" + id) })
		m.Separator()
		s.verbItem(m, VerbTrash, locales.MoveToTrash(), components.IconTrash, true, func() { s.trashSheets([]string{id}) })
	}
}

// showInGroup goes to the field in the group the sheet is filed in, the
// sheet brought into view and marked: where a sheet found is kept.
func (s *State) showInGroup(sheet string) {
	g, ok := s.groupOf(sheet)
	if !ok {
		return
	}
	s.ShowField()
	s.field.picked = nil
	s.SetFieldQuery((&query.Condition{Key: mustKey("in"), Value: g.Name}).String())
	if g.Inbox {
		s.SetFieldQuery("")
	}
	s.arrive("sheet:" + sheet)
}

// deskMenu is the menu of an open sheet's head: what is done to it as it
// stands on the desk.
func (s *State) deskMenu(page Page) func(*ui.Menu) {
	return func(m *ui.Menu) {
		key := func(it *ui.MenuItem, id string) *ui.MenuItem {
			if s.KeyOf != nil {
				if mods, k, ok := s.KeyOf(id); ok {
					it.Shortcut(mods, k)
				}
			}
			return it
		}
		if key(components.MenuItem(m.Item(locales.SplitSheet()), components.IconColumns), "page.split").Chosen() {
			s.SplitColumn(page.Column)
		}
		m.Separator()
		if key(components.MenuItem(m.Item(locales.CloseSheet()), components.IconClose), "page.close").Chosen() {
			s.CloseColumn(page.Column)
		}
		if key(m.Item(locales.CloseOtherSheets()), "page.close-others").Disabled(len(s.Open) < 2).Chosen() {
			s.closeOthers(page.Column)
		}
		m.Separator()
		if components.MenuItem(m.Item(locales.ShowInGroup()), components.IconFolderSearch).Chosen() {
			s.showInGroup(page.ID)
		}
		if s.Library != nil {
			if components.MenuItem(m.Item(locales.Duplicate()), components.IconDuplicate).Chosen() {
				s.duplicate([]string{page.ID})
			}
			// Its key exports the sheet in front, so only that one's
			// menu shows it.
			export := components.MenuItem(m.Item(locales.ExportEllipsis()), components.IconExport)
			if page.Column == s.ActiveColumn {
				key(export, verbExport)
			}
			if export.Chosen() {
				s.Library.Export("sheet:" + page.ID)
			}
			m.Separator()
			if components.MenuItem(m.Item(locales.MoveToTrash()), components.IconTrash).Chosen() {
				s.trashSheets([]string{page.ID})
			}
		}
	}
}

// closeOthers closes every column but keep.
func (s *State) closeOthers(keep string) {
	for _, page := range slices.Clone(s.Open) {
		if page.Column != keep {
			s.CloseColumn(page.Column)
		}
	}
}

// CloseOthers closes every column but the one in front.
func (s *State) CloseOthers() { s.closeOthers(s.ActiveColumn) }
