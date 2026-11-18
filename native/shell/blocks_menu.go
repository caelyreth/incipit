package shell

import (
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
		if m.Item("Rename…").Chosen() {
			s.startRename(g.ID)
		}
		m.Submenu("Color", func(m *ui.Menu) {
			for color := TintMoonveil; color < TintHalo+len(theme.AllAccents); color++ {
				if m.Item(tintName(color)).Checked(g.Color == color).Chosen() {
					s.Library.StyleGroup(g.ID, g.Icon, color)
				}
			}
		})
		s.moveMenu(m, func(target string) { s.moveGroup(g.ID, target) }, g.ID)
		m.Separator()
		if m.Item(trashGroupLabel(g)).Chosen() {
			s.trashGroup(g)
		}
	}
}

// pinItem is the menu's Pin, or Unpin, of the object id, which then
// stands where its pin puts it, brought into view.
func (s *State) pinItem(m *ui.Menu, id string, pinned bool) {
	if m.Item(map[bool]string{true: "Unpin", false: "Pin"}[pinned]).Chosen() {
		s.Library.Pin(id, !pinned)
		s.arrive(id)
	}
}

func (s *State) sheetMenu(sh SheetItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			if m.Item("Put Back").Chosen() {
				s.restoreSheets([]string{sh.ID})
			}
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
		m.Separator()
		if m.Item("Duplicate").Chosen() {
			s.Library.DuplicateSheet(sh.ID)
		}
		s.pinItem(m, "sheet:"+sh.ID, sh.Pinned)
		s.moveMenu(m, func(group string) { s.moveSheets([]string{sh.ID}, group) }, "")
		m.Separator()
		if m.Item("Move to Trash").Chosen() {
			s.trashSheets([]string{sh.ID})
		}
	}
}
