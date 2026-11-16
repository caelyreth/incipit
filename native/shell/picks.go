package shell

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Sheets are picked together on the field, for what is done to several at
// once, as the sheet list picked them: a Cmd-click picks a sheet or lets
// it go, a Shift-click picks every sheet from the anchor to the one
// clicked, and a plain click picks none and opens its sheet. Escape lets
// the picked go first.

func (s *State) isPicked(id string) bool { return slices.Contains(s.field.picked, id) }

// sheetOrder is the sheets the field shows, in order.
func (s *State) sheetOrder() []string {
	out := make([]string, 0, len(s.field.order))
	for _, id := range s.field.order {
		if strings.HasPrefix(id, "sheet:") {
			out = append(out, id)
		}
	}
	return out
}

// pickToggle picks id or lets it go; with none picked, the sheet in front
// is picked with it, as a selection a Cmd-click adds to.
func (s *State) pickToggle(id string) {
	set := slices.Clone(s.field.picked)
	front := "sheet:" + s.ActivePage
	if len(set) == 0 && front != id && slices.Contains(s.sheetOrder(), front) {
		set = append(set, front)
	}
	if i := slices.Index(set, id); i >= 0 {
		set = slices.Delete(set, i, i+1)
	} else {
		set = append(set, id)
	}
	s.field.picked, s.field.anchor = set, id
}

// pickTo picks every sheet from the anchor to id.
func (s *State) pickTo(id string) {
	order := s.sheetOrder()
	from, to := slices.Index(order, s.field.anchor), slices.Index(order, id)
	if to < 0 {
		return
	}
	if from < 0 {
		from = to
	}
	s.field.picked = slices.Clone(order[min(from, to) : max(from, to)+1])
}

// keepPicked lets go of picked sheets the field no longer shows.
func (s *State) keepPicked() {
	order := s.sheetOrder()
	s.field.picked = slices.DeleteFunc(s.field.picked, func(id string) bool { return !slices.Contains(order, id) })
}

func (s *State) pickedSheets() []string {
	out := make([]string, len(s.field.picked))
	for i, id := range s.field.picked {
		out[i] = strings.TrimPrefix(id, "sheet:")
	}
	return out
}

func deletePickedKey(id string) string { return "delete-picked-" + id }

func (s *State) deletePicked() {
	for _, id := range s.pickedSheets() {
		s.Library.DeleteSheet(id)
	}
	s.field.picked = nil
}

// pickedMenu is what a menu on a picked sheet does to all the picked.
func (s *State) pickedMenu(id string, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		ids := s.pickedSheets()
		n := len(ids)
		if trashed {
			if m.Item(fmt.Sprintf("Put Back %d Sheets", n)).Chosen() {
				for _, sh := range ids {
					s.Library.RestoreSheet(sh)
				}
				s.field.picked = nil
			}
			m.Separator()
			if m.Item(fmt.Sprintf("Delete %d Sheets Immediately…", n)).Chosen() {
				s.arm(deletePickedKey(id))
			}
			return
		}
		if m.Item(fmt.Sprintf("Open %d Sheets", n)).Chosen() {
			for _, sh := range ids {
				s.Library.OpenSheet(sh, true)
			}
			s.field.picked = nil
			s.ShowDesk()
		}
		m.Separator()
		s.moveMenu(m, func(group string) {
			for _, sh := range ids {
				s.Library.MoveSheet(sh, group, Last)
			}
		}, "")
		m.Separator()
		if m.Item(fmt.Sprintf("Move %d Sheets to Trash", n)).Chosen() {
			for _, sh := range ids {
				s.Library.TrashSheet(sh)
			}
			s.field.picked = nil
		}
	}
}
