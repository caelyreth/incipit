package shell

import (
	"fmt"
	"slices"

	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

// picking is the sheets picked together in the sheet list, for what is
// done to several at once, as lists elsewhere pick them: a Cmd-click picks
// a sheet or lets it go, a shift-click picks every sheet from the anchor to
// the one clicked, and a plain click picks none and opens its sheet. Each
// moves the anchor to the sheet clicked but the shift-click, which keeps
// it, so shift-clicks in turn reshape one run.
type picking struct {
	// ids is the picked sheets in the list's order.
	ids []string
	// anchor is where a shift-click's run begins.
	anchor string
	// base is what was picked as the anchor was set: a shift-click's run
	// adds to it, so a run reshaped leaves the sheets picked apart alone.
	base []string
	// order is the sheets the list shows, in order, as last built.
	order []string
}

// noteOrder keeps the sheets the list shows, and lets go of picked ones it
// no longer shows, as one moved to the trash.
func (s *State) noteOrder(entries []GroupEntry) {
	p := &s.nav.picking
	p.order = p.order[:0]
	for _, entry := range entries {
		if entry.Kind == EntrySheet {
			p.order = append(p.order, entry.ID)
		}
	}
	gone := func(id string) bool { return !slices.Contains(p.order, id) }
	p.ids = slices.DeleteFunc(p.ids, gone)
	p.base = slices.DeleteFunc(p.base, gone)
}

// setPicked picks the sheets in set, in the list's order.
func (s *State) setPicked(set []string) {
	p := &s.nav.picking
	p.ids = p.ids[:0]
	for _, id := range p.order {
		if slices.Contains(set, id) {
			p.ids = append(p.ids, id)
		}
	}
}

// picked reports whether the sheet is picked.
func (s *State) picked(id string) bool { return slices.Contains(s.nav.picking.ids, id) }

// pickedSheets is the picked sheets in the list's order.
func (s *State) pickedSheets() []string { return slices.Clone(s.nav.picking.ids) }

// unpick lets the picked sheets go, keeping the anchor.
func (s *State) unpick() { s.nav.picking.ids, s.nav.picking.base = nil, nil }

// pickTo picks every sheet from the anchor to id, on top of what was picked
// as the anchor was set. Without an anchor in the list it runs from the
// open sheet, or else picks id alone.
func (s *State) pickTo(id string) {
	p := &s.nav.picking
	from := slices.Index(p.order, p.anchor)
	if from < 0 {
		from = slices.Index(p.order, s.ActivePage)
		p.base = nil
	}
	to := slices.Index(p.order, id)
	if to < 0 {
		return
	}
	if from < 0 {
		from = to
	}
	p.anchor = p.order[from]
	s.setPicked(append(slices.Clone(p.base), p.order[min(from, to):max(from, to)+1]...))
}

// pickToggle picks id or lets it go, leaving the rest as they are. With
// none picked, the open sheet is the one the list already marks, so it is
// picked with id, as a selection a Cmd-click adds to.
func (s *State) pickToggle(id string) {
	p := &s.nav.picking
	if !slices.Contains(p.order, id) {
		return
	}
	set := slices.Clone(p.ids)
	if len(set) == 0 && s.ActivePage != id && slices.Contains(p.order, s.ActivePage) {
		set = append(set, s.ActivePage)
	}
	if index := slices.Index(set, id); index >= 0 {
		set = slices.Delete(set, index, index+1)
	} else {
		set = append(set, id)
	}
	s.setPicked(set)
	p.anchor, p.base = id, slices.Clone(p.ids)
}

// unpickOnPressElsewhere lets the picked go on a press on none of the
// sheets: in the list's empty space, on a chapter, in the library or the
// page. A press on a sheet is its click's to decide.
func (s *State) unpickOnPressElsewhere(sheets []ui.Element) {
	if len(s.nav.picking.ids) == 0 || len(sheets) == 0 {
		return
	}
	for _, row := range sheets {
		if !row.PressedOutside() {
			return
		}
	}
	s.unpick()
}

// pickAll picks every sheet the list shows.
func (s *State) pickAll() {
	p := &s.nav.picking
	p.ids, p.base = slices.Clone(p.order), nil
	if p.anchor == "" && len(p.order) > 0 {
		p.anchor = p.order[0]
	}
}

// pickAmount is how far the sheet's row is picked, following its picking
// on a spring so the run's fill grows to it and gives it up smoothly.
func (s *State) pickAmount(row ui.Element, id string, animate bool) float32 {
	target := float32(0)
	if s.picked(id) {
		target = 1
	}
	if !animate {
		return target
	}
	return components.Follow(row, "picked", target, components.MotionQuick)
}

// batchMenu is the menu of what the picked sheets take together: as the
// single sheet's menu has it, each item saying how many it does.
func (s *State) batchMenu(m *ui.Menu, row string, trashed bool) {
	ids := s.pickedSheets()
	n := len(ids)
	each := func(do func(id string)) {
		for _, id := range ids {
			do(id)
		}
		s.unpick()
	}
	if trashed {
		if m.Item(fmt.Sprintf("Put Back %d Sheets", n)).Chosen() {
			each(s.Library.RestoreSheet)
		}
		m.Separator()
		if m.Item(fmt.Sprintf("Delete %d Sheets Immediately…", n)).Chosen() {
			// The row the menu opened on asks to be sure, for all of them.
			s.arm(deletePickedKey(row))
		}
		return
	}
	if m.Item(fmt.Sprintf("Open %d Sheets in Tabs", n)).Chosen() {
		each(func(id string) { s.Library.OpenSheet(id, true) })
	}
	m.Separator()
	if m.Item(fmt.Sprintf("Duplicate %d Sheets", n)).Chosen() {
		each(s.Library.DuplicateSheet)
	}
	s.moveMenu(m, func(group string) {
		// In their order, each after the last.
		each(func(id string) { s.Library.MoveSheet(id, group, Last) })
	}, "")
	m.Separator()
	if m.Item(fmt.Sprintf("Move %d Sheets to Trash", n)).Chosen() {
		each(s.Library.TrashSheet)
	}
}

// deletePickedKey is the confirmation that deletes the picked sheets, shown
// on the row whose menu asked for it.
func deletePickedKey(row string) string { return "delete-picked-" + row }

// deletePicked deletes the picked sheets for good.
func (s *State) deletePicked() {
	for _, id := range s.pickedSheets() {
		s.Library.DeleteSheet(id)
	}
	s.unpick()
}

// pickedDetail is what the header says of the picked sheets among sheets,
// with any picked: how many, and their words.
func (s *State) pickedDetail(sheets []SheetItem) (string, bool) {
	n, total := 0, 0
	for _, sheet := range sheets {
		if s.picked(sheet.ID) {
			n++
			total += sheet.Words
		}
	}
	if n == 0 {
		return "", false
	}
	return fmt.Sprintf("%d selected · %s", n, words(total)), true
}

// pickClick does what a click on a sheet's row does to the picking: with
// Shift it picks the run up to the sheet, with Cmd it picks the sheet or
// lets it go, and either way reports that the sheet is not to open;
// without, it lets the picked go and starts the next run there.
func (s *State) pickClick(id string, modifiers ui.Modifiers) bool {
	switch {
	case modifiers&ui.Shift != 0:
		s.pickTo(id)
		return true
	case modifiers&ui.Cmd != 0:
		s.pickToggle(id)
		return true
	}
	s.unpick()
	s.nav.picking.anchor = id
	return false
}
