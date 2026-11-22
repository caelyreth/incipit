package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

// What is filed is selected on the field, a group or a sheet alike, for
// what is done to several at once (field-and-desk.md §4.7.7): a Cmd-click,
// or Space on the row the keys are on, adds one or lets it go; a
// Shift-click takes the run from the anchor in the field's order; a plain
// click selects none and opens. A selection is of what is shown: it goes
// as the query changes. While it stands, the pill's actions are its own
// and the pill's right counts it; Escape lets it go first.

func (s *State) isPicked(id string) bool { return slices.Contains(s.field.picked, id) }

// pickable reports whether id is a row that can be selected: a group or a
// sheet, but the Inbox, which nothing moves.
func (s *State) pickable(id string) bool {
	if g, ok := strings.CutPrefix(id, "group:"); ok {
		group, ok := s.shownGroup(g)
		return ok && !group.Inbox
	}
	return strings.HasPrefix(id, "sheet:")
}

// pickOrder is what the field shows that can be selected, in its order.
func (s *State) pickOrder() []string {
	return slices.DeleteFunc(slices.Clone(s.field.order), func(id string) bool { return !s.pickable(id) })
}

// pickToggle selects id or lets it go.
func (s *State) pickToggle(id string) {
	if !s.pickable(id) {
		return
	}
	if i := slices.Index(s.field.picked, id); i >= 0 {
		s.field.picked = slices.Delete(slices.Clone(s.field.picked), i, i+1)
	} else {
		s.field.picked = append(slices.Clone(s.field.picked), id)
	}
	s.field.anchor = id
}

// pickTo selects every row from the anchor to id.
func (s *State) pickTo(id string) {
	order := s.pickOrder()
	from, to := slices.Index(order, s.field.anchor), slices.Index(order, id)
	if to < 0 {
		return
	}
	if from < 0 {
		from = to
	}
	s.field.picked = slices.Clone(order[min(from, to) : max(from, to)+1])
}

// keepPicked lets the selection go as the query changes, and what the
// field no longer shows meanwhile.
func (s *State) keepPicked() {
	if q := s.wholeQuery(); q != s.field.pickedFor {
		s.field.pickedFor = q
		s.field.picked, s.field.chosen = nil, ""
		return
	}
	order := s.pickOrder()
	s.field.picked = slices.DeleteFunc(s.field.picked, func(id string) bool { return !slices.Contains(order, id) })
}

// picked is the bare ids of the selection's rows of kind, "group:" or
// "sheet:".
func (s *State) pickedOf(kind string) []string {
	var out []string
	for _, id := range s.field.picked {
		if bare, ok := strings.CutPrefix(id, kind); ok {
			out = append(out, bare)
		}
	}
	return out
}

// pickedGroups are the groups selected, as the field shows them.
func (s *State) pickedGroups() []GroupItem {
	var out []GroupItem
	for _, id := range s.pickedOf("group:") {
		if g, ok := s.shownGroup(id); ok {
			out = append(out, g)
		}
	}
	return out
}

// shownGroup is the group id as the field shows it, in the trash too.
func (s *State) shownGroup(id string) (GroupItem, bool) {
	for _, cd := range s.field.found.cards {
		if b, ok := cd.kind.(block); ok {
			if b.group != nil && b.group.ID == id {
				return *b.group, true
			}
			for _, g := range b.groups {
				if g.ID == id {
					return g, true
				}
			}
		}
	}
	return s.Library.Group(id)
}

// selectionActions are what the selection's actions do, in the trash or
// out of it: the arc, and the menu of a row selected, hold them.
func (s *State) selectionActions() []fieldAction {
	n := len(s.field.picked)
	if n == 0 || s.Library == nil {
		return nil
	}
	done := func() { s.field.picked = nil }
	if s.inTrash() {
		return []fieldAction{
			{id: "put-back", label: locales.PutBack(), icon: components.IconUndo, run: func() {
				s.together(locales.PutBackWhat(s.objectsNamed(s.field.picked)), func() {
					s.restoreSheets(s.pickedOf("sheet:"))
					for _, g := range s.pickedGroups() {
						s.restoreGroup(g)
					}
				})
				done()
			}},
			{id: deletePickedKey, label: locales.DeleteImmediatelyAction(), icon: components.IconTrash, danger: true, run: func() {
				for _, id := range s.pickedOf("sheet:") {
					s.Library.DeleteSheet(id)
				}
				for _, id := range s.pickedOf("group:") {
					s.Library.DeleteGroup(id)
				}
				done()
			}},
		}
	}
	var out []fieldAction
	if sheets := s.pickedOf("sheet:"); len(sheets) > 0 {
		out = append(out, fieldAction{id: "open-picked", label: locales.OpenWhat(objectsCount(0, len(sheets), locales.CountGroups, locales.CountSheets)), icon: components.IconDesk,
			run: func() {
				for _, id := range sheets {
					s.Library.OpenSheet(id, true)
				}
				done()
				s.ShowDesk()
			}})
	}
	out = append(out,
		fieldAction{id: "move-picked", label: locales.MoveTo(), icon: components.IconFolder, menu: s.moveTargets(func(group string) {
			s.field.acting = false
			s.moveObjects(s.field.picked, group)
			done()
		}, s.pickedOf("group:")...)},
		fieldAction{id: "trash-picked", label: locales.MoveToTrash(), icon: components.IconTrash, run: func() {
			s.together(locales.MovedToTrash(s.objectsNamed(s.field.picked)), func() {
				s.trashSheets(s.pickedOf("sheet:"))
				for _, g := range s.pickedGroups() {
					s.trashGroup(g)
				}
			})
			done()
		}},
	)
	return out
}

// deletePickedKey names the selection's Delete Immediately, for arming.
const deletePickedKey = "delete-picked"

// moveObjects files the rows ids, groups and sheets, in group, as one
// thing to be undone.
func (s *State) moveObjects(ids []string, group string) {
	sheets, groups := bare(ids, "sheet:"), bare(ids, "group:")
	told := locales.MovedToTop(s.objectsNamed(ids))
	if group != "" {
		told = locales.MovedTo(s.objectsNamed(ids), s.groupNamedByID(group))
	}
	s.together(told, func() {
		s.moveSheets(sheets, group)
		for _, g := range groups {
			if g != group {
				s.moveGroup(g, group)
			}
		}
	})
}

// bare is the bare ids of the rows ids of kind.
func bare(ids []string, kind string) []string {
	var out []string
	for _, id := range ids {
		if b, ok := strings.CutPrefix(id, kind); ok {
			out = append(out, b)
		}
	}
	return out
}

// objectsNamed names the rows ids: one by its name, more by how many of
// each kind.
func (s *State) objectsNamed(ids []string) string {
	groups, sheets := bare(ids, "group:"), bare(ids, "sheet:")
	switch {
	case len(ids) == 1 && len(sheets) == 1:
		return s.sheetsNamed(sheets)
	case len(ids) == 1 && len(groups) == 1:
		if g, ok := s.shownGroup(groups[0]); ok {
			return locales.Quoted(g.Name)
		}
	}
	return objectsCount(len(groups), len(sheets), locales.CountGroups, locales.CountSheets)
}

// objectsCount says how many groups and sheets, each counted as group and
// sheet count them: "2 Groups and 3 Sheets", leaving out a kind there is
// none of.
func objectsCount(groups, sheets int, group, sheet func(int) string) string {
	switch {
	case groups == 0:
		return sheet(sheets)
	case sheets == 0:
		return group(groups)
	}
	return locales.GroupsAndSheets(group(groups), sheet(sheets))
}

// pickedMenu is what a menu on a row selected does to all the selection:
// the selection's actions.
func (s *State) pickedMenu() func(*ui.Menu) {
	return func(m *ui.Menu) { s.actionsMenu(m, s.selectionActions()) }
}

// rowMenu is the menu of the row id: the selection's, where it is one of
// several selected, or its own.
func (s *State) rowMenu(id string, own func(*ui.Menu)) func(*ui.Menu) {
	if s.isPicked(id) && len(s.field.picked) > 1 {
		return s.pickedMenu()
	}
	return own
}

// pickRow does what a click on the pickable row id does with mods,
// reporting whether it selected rather than went on.
func (s *State) pickRow(id string, mods ui.Modifiers) bool {
	switch {
	case mods&ui.Cmd != 0:
		s.pickToggle(id)
	case mods&ui.Shift != 0:
		s.pickTo(id)
	default:
		s.field.picked, s.field.anchor = nil, id
		return false
	}
	return true
}
