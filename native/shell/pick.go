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
func (s listing) noteOrder(entries []GroupEntry) {
	p := &s.browser.picking
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
func (s listing) setPicked(set []string) {
	p := &s.browser.picking
	p.ids = p.ids[:0]
	for _, id := range p.order {
		if slices.Contains(set, id) {
			p.ids = append(p.ids, id)
		}
	}
}

// picked reports whether the sheet is picked.
func (s listing) picked(id string) bool { return slices.Contains(s.browser.picking.ids, id) }

// pickedSheets is the picked sheets in the list's order.
func (s listing) pickedSheets() []string { return slices.Clone(s.browser.picking.ids) }

// unpick lets the picked sheets go, keeping the anchor.
func (s listing) unpick() { s.browser.picking.ids, s.browser.picking.base = nil, nil }

// pickTo picks every sheet from the anchor to id, on top of what was picked
// as the anchor was set. Without an anchor in the list it runs from the
// open sheet, or else picks id alone.
func (s listing) pickTo(id string) {
	p := &s.browser.picking
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
func (s listing) pickToggle(id string) {
	p := &s.browser.picking
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
func (s listing) unpickOnPressElsewhere(sheets []ui.Element) {
	if len(s.browser.picking.ids) == 0 || len(sheets) == 0 {
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
func (s listing) pickAll() {
	p := &s.browser.picking
	p.ids, p.base = slices.Clone(p.order), nil
	if p.anchor == "" && len(p.order) > 0 {
		p.anchor = p.order[0]
	}
}

// pickAmount is how far the sheet's row is picked, following its picking
// on a spring, for what of the row's look goes with it.
func (s listing) pickAmount(row ui.Element, id string, animate bool) float32 {
	target := float32(0)
	if s.picked(id) {
		target = 1
	}
	if !animate {
		return target
	}
	return components.Follow(row, "picked", target, components.MotionQuick)
}

// menuRing is what a menu open on the sheet list acts on, which the list
// rings while the menu shows, as menus elsewhere show what they are for:
// the picked sheets for theirs, a sheet alone for its own. MyGo builds the
// menu as it opens and draws the frame before showing it, but says nothing
// when it closes unchosen: the ring goes with the first move of the
// pointer after it opened, or as an item is chosen.
type menuRing struct {
	ids []string
	// fresh is a ring set this frame, whose pointer is yet to be noted.
	fresh bool
	// pointer is where the pointer was over the list as the menu opened.
	pointer [2]float32
	over    bool
	// chose is set as the menu reports an item chosen.
	chose bool
}

// ringedMenu is build as the menu acting on ids, ringing them while it is
// open.
func (s listing) ringedMenu(ids []string, build func(*ui.Menu)) func(*ui.Menu) {
	return func(m *ui.Menu) {
		ring := &s.browser.menuRing
		ring.chose = false
		build(m)
		if ring.chose {
			*ring = menuRing{}
			return
		}
		*ring = menuRing{ids: ids, fresh: true}
	}
}

// chose reports whether item was chosen, noting it for the ring.
func (s listing) chose(item *ui.MenuItem) bool {
	if item.Chosen() {
		s.browser.menuRing.chose = true
		return true
	}
	return false
}

// ringMenu rings on track what a menu open on the list acts on, and lets
// the ring go once the pointer moves after the menu opened, or presses
// away from the list. The menu takes the press that opened it, so the list
// may still count that press as held while the menu shows: it is no sign
// the menu closed.
func (s listing) ringMenu(list ui.Element, track *components.SelectionTrack) {
	ring := &s.browser.menuRing
	if len(ring.ids) == 0 {
		return
	}
	x, y, over := list.PointerPosition()
	if ring.fresh {
		ring.fresh, ring.pointer, ring.over = false, [2]float32{x, y}, over
	} else if over != ring.over || [2]float32{x, y} != ring.pointer || list.PressedOutside() {
		*ring = menuRing{}
		return
	}
	track.Ring(ring.ids)
}

// batchMenu is the menu of what the picked sheets take together: as the
// single sheet's menu has it, each item saying how many it does.
func (s listing) batchMenu(m *ui.Menu, row string, trashed bool) {
	ids := s.pickedSheets()
	n := len(ids)
	each := func(do func(id string)) {
		for _, id := range ids {
			do(id)
		}
		s.unpick()
	}
	if trashed {
		if s.chose(m.Item(fmt.Sprintf("Put Back %d Sheets", n))) {
			each(s.Library.RestoreSheet)
		}
		m.Separator()
		if s.chose(m.Item(fmt.Sprintf("Delete %d Sheets Immediately…", n))) {
			// The row the menu opened on asks to be sure, for all of them.
			s.arm(deletePickedKey(row))
		}
		return
	}
	if s.chose(m.Item(fmt.Sprintf("Open %d Sheets Alongside", n))) {
		each(func(id string) { s.open(id, "", true) })
	}
	m.Separator()
	if s.chose(m.Item(fmt.Sprintf("Duplicate %d Sheets", n))) {
		each(s.Library.DuplicateSheet)
	}
	s.moveMenu(m, func(group string) {
		s.browser.menuRing.chose = true
		// In their order, each after the last.
		each(func(id string) { s.Library.MoveSheet(id, group, Last) })
	}, "")
	m.Separator()
	if s.chose(m.Item(fmt.Sprintf("Move %d Sheets to Trash", n))) {
		each(s.Library.TrashSheet)
	}
}

// deletePickedKey is the confirmation that deletes the picked sheets, shown
// on the row whose menu asked for it.
func deletePickedKey(row string) string { return "delete-picked-" + row }

// deletePicked deletes the picked sheets for good.
func (s listing) deletePicked() {
	for _, id := range s.pickedSheets() {
		s.Library.DeleteSheet(id)
	}
	s.unpick()
}

// pickedDetail is what the place's title says of the picked sheets among sheets,
// with any picked: how many, and their words.
func (s listing) pickedDetail(sheets []SheetItem) (string, bool) {
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
func (s listing) pickClick(id string, modifiers ui.Modifiers) bool {
	switch {
	case modifiers&ui.Shift != 0:
		s.pickTo(id)
		return true
	case modifiers&ui.Cmd != 0:
		s.pickToggle(id)
		return true
	}
	s.unpick()
	s.browser.picking.anchor = id
	return false
}
