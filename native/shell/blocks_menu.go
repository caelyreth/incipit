package shell

import (
	"slices"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/locales"
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
			if m.Item(locales.PutBack()).Chosen() {
				s.restoreGroup(g)
			}
			m.Separator()
			if m.Item(deleteGroupLabel(g)).Chosen() {
				s.arm(deleteKey("group:" + g.ID))
			}
			return
		}
		if !g.Smart() {
			if m.Item(locales.NewSheet()).Chosen() {
				s.Library.NewSheet(g.ID, Last)
				s.ShowDesk()
			}
		}
		if g.Inbox {
			return
		}
		if !g.Smart() && m.Item(locales.NewGroupInside()).Chosen() {
			s.newGroup(g.ID)
		}
		m.Separator()
		s.pinItem(m, "group:"+g.ID, g.Pinned)
		s.keepItem(m, g)
		if !g.Smart() {
			s.verbItem(m, VerbRename, locales.RenameEllipsis(), true, func() { s.startRename(g.ID) })
		}
		m.Submenu(locales.Color(), func(m *ui.Menu) {
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
	s.verbItem(m, VerbPin, map[bool]string{true: locales.Unpin(), false: locales.Pin()}[pinned], true, func() { s.pin(id, !pinned) })
}

func (s *State) sheetMenu(sh SheetItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			s.verbItem(m, VerbTrash, locales.PutBack(), true, func() { s.restoreSheets([]string{sh.ID}) })
			m.Separator()
			if m.Item(locales.DeleteImmediately()).Chosen() {
				s.arm(deleteKey("sheet:" + sh.ID))
			}
			return
		}
		if m.Item(locales.Open()).Chosen() {
			s.Library.OpenSheet(sh.ID, false)
			s.ShowDesk()
		}
		if m.Item(locales.OpenAlongside()).Chosen() {
			s.Library.OpenSheet(sh.ID, true)
			s.ShowDesk()
		}
		if g := s.Library.SheetGroup(sh.ID); g != "" && g != s.scope() {
			if m.Item(locales.ShowInGroup()).Chosen() {
				s.showInGroup(sh.ID)
			}
		}
		m.Separator()
		s.verbItem(m, VerbDuplicate, locales.Duplicate(), true, func() { s.duplicate([]string{sh.ID}) })
		s.pinItem(m, "sheet:"+sh.ID, sh.Pinned)
		s.moveMenu(m, func(group string) { s.moveSheets([]string{sh.ID}, group) }, "")
		m.Separator()
		s.verbItem(m, VerbTrash, locales.MoveToTrash(), true, func() { s.trashSheets([]string{sh.ID}) })
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
		if key(m.Item(locales.SplitSheet()), "page.split").Chosen() {
			s.SplitColumn(page.Column)
		}
		m.Separator()
		if key(m.Item(locales.CloseSheet()), "page.close").Chosen() {
			s.CloseColumn(page.Column)
		}
		if key(m.Item(locales.CloseOtherSheets()), "page.close-others").Disabled(len(s.Open) < 2).Chosen() {
			s.closeOthers(page.Column)
		}
		m.Separator()
		if m.Item(locales.ShowInGroup()).Chosen() {
			s.showInGroup(page.ID)
		}
		if s.Library != nil {
			if m.Item(locales.Duplicate()).Chosen() {
				s.duplicate([]string{page.ID})
			}
			m.Separator()
			if m.Item(locales.MoveToTrash()).Chosen() {
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
