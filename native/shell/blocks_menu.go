package shell

import (
	"slices"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/theme"
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

func (s *State) groupMenu(g GroupItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			if m.Item("Put Back").Chosen() {
				s.restoreGroup(g)
			}
			m.Separator()
			if m.Item(deleteGroupLabel(g)).Chosen() {
				s.arm(deleteKey("group:" + g.ID))
			}
			return
		}
		if !g.Smart() {
			if m.Item("New Sheet").Chosen() {
				s.Library.NewSheet(g.ID, Last)
				s.ShowDesk()
			}
		}
		if g.Inbox {
			return
		}
		if !g.Smart() && m.Item("New Group Inside").Chosen() {
			s.newGroup(g.ID)
		}
		m.Separator()
		s.pinItem(m, "group:"+g.ID, g.Pinned)
		if !g.Smart() {
			s.verbItem(m, VerbRename, "Rename…", true, func() { s.startRename(g.ID) })
		}
		m.Submenu("Color", func(m *ui.Menu) {
			for color := TintMoonveil; color < TintHalo+len(theme.AllAccents); color++ {
				if m.Item(tintName(color)).Checked(g.Color == color).Chosen() {
					s.styleGroup(g, g.Icon, color)
				}
			}
		})
		s.moveMenu(m, func(target string) { s.moveGroup(g.ID, target) }, g.ID)
		m.Separator()
		s.verbItem(m, VerbTrash, trashGroupLabel(g), true, func() { s.trashGroup(g) })
	}
}

// pinItem is the menu's Pin, or Unpin, of the object id, which then
// stands where its pin puts it, brought into view.
func (s *State) pinItem(m *ui.Menu, id string, pinned bool) {
	s.verbItem(m, VerbPin, map[bool]string{true: "Unpin", false: "Pin"}[pinned], true, func() { s.pin(id, !pinned) })
}

func (s *State) sheetMenu(sh SheetItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			s.verbItem(m, VerbTrash, "Put Back", true, func() { s.restoreSheets([]string{sh.ID}) })
			m.Separator()
			if m.Item("Delete Immediately…").Chosen() {
				s.arm(deleteKey("sheet:" + sh.ID))
			}
			return
		}
		if m.Item("Open").Chosen() {
			s.Library.OpenSheet(sh.ID, false)
			s.ShowDesk()
		}
		if m.Item("Open Alongside").Chosen() {
			s.Library.OpenSheet(sh.ID, true)
			s.ShowDesk()
		}
		if m.Item("Edit Title").Chosen() {
			s.editTitle(sh.ID)
		}
		if g := s.Library.SheetGroup(sh.ID); g != "" && g != s.scope() {
			if m.Item("Show in Group").Chosen() {
				s.showInGroup(sh.ID)
			}
		}
		m.Separator()
		s.verbItem(m, VerbDuplicate, "Duplicate", true, func() { s.duplicate([]string{sh.ID}) })
		s.pinItem(m, "sheet:"+sh.ID, sh.Pinned)
		s.moveMenu(m, func(group string) { s.moveSheets([]string{sh.ID}, group) }, "")
		m.Separator()
		s.verbItem(m, VerbTrash, "Move to Trash", true, func() { s.trashSheets([]string{sh.ID}) })
	}
}

// editTitle opens the sheet on the desk, in front, its first line
// selected: a sheet is named by its first line, and this is where.
func (s *State) editTitle(sheet string) {
	if s.Library == nil {
		return
	}
	s.Library.OpenSheet(sheet, false)
	s.ShowDesk()
	if s.EditTitle != nil {
		s.EditTitle(sheet)
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
		if key(m.Item("Close Sheet"), "page.close").Chosen() {
			s.CloseSheet(page.ID)
		}
		if key(m.Item("Close Other Sheets"), "page.close-others").Disabled(len(s.Open) < 2).Chosen() {
			s.closeOthers(page.ID)
		}
		m.Separator()
		if m.Item("Edit Title").Chosen() {
			s.editTitle(page.ID)
		}
		if m.Item("Show in Group").Chosen() {
			s.showInGroup(page.ID)
		}
		if s.Library != nil {
			if m.Item("Duplicate").Chosen() {
				s.duplicate([]string{page.ID})
			}
			m.Separator()
			if m.Item("Move to Trash").Chosen() {
				s.trashSheets([]string{page.ID})
			}
		}
	}
}

// closeOthers closes every open sheet but keep.
func (s *State) closeOthers(keep string) {
	for _, page := range slices.Clone(s.Open) {
		if page.ID != keep {
			s.CloseSheet(page.ID)
		}
	}
}

// CloseOthers closes every open sheet but the one in front.
func (s *State) CloseOthers() { s.closeOthers(s.ActivePage) }
