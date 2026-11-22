package shell

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Every change to the library is done once, as an entry on the field's
// undo stack, which Edit › Undo and Redo, ⌘Z and ⇧⌘Z walk while the field
// is in front, however long ago it was and wherever the field has looked
// since. The last change also says so on the pill's right, in place of
// the stats, with Undo (field-and-desk.md §4.7.1): for noticeFor, or until
// the query changes, or another takes its place. The notice is a view of
// the stack's top, never the stack. What cannot be undone asks before it
// runs instead, and says nothing after.

// undoEntry is a change done, and how to take it back and do it again.
type undoEntry struct {
	text       string
	undo, redo func()
}

// undoKept is how many changes the stack keeps.
const undoKept = 100

// notice is what was just done, as the stack's top says it.
type notice struct {
	text string
	// query is the query it was done under; at is when it was first shown.
	query string
	at    time.Time
}

// noticeFor is how long a notice stands.
const noticeFor = 8 * time.Second

// tell records a change just done, called text, undone by undo and done
// again by redo, and puts up its notice; while things are done together,
// it only gathers it with theirs.
func (s *State) tell(text string, undo, redo func()) {
	entry := undoEntry{text: text, undo: undo, redo: redo}
	if s.field.gathering != nil {
		*s.field.gathering = append(*s.field.gathering, entry)
		return
	}
	s.field.undos = append(s.field.undos, entry)
	if n := len(s.field.undos); n > undoKept {
		s.field.undos = slices.Delete(s.field.undos, 0, n-undoKept)
	}
	s.field.redos = nil
	s.field.notice = &notice{text: text, query: s.wholeQuery()}
}

// together does do, whose changes each would tell of their own, as one
// change called text: one notice, one entry, whose undo takes them all
// back, the last first.
func (s *State) together(text string, do func()) {
	var entries []undoEntry
	s.field.gathering = &entries
	do()
	s.field.gathering = nil
	if len(entries) == 0 {
		return
	}
	s.tell(text, func() {
		for i := len(entries) - 1; i >= 0; i-- {
			entries[i].undo()
		}
	}, func() {
		for _, e := range entries {
			e.redo()
		}
	})
}

// CanUndo is what Undo would take back, and whether there is anything.
func (s *State) CanUndo() (string, bool) {
	if n := len(s.field.undos); n > 0 {
		return s.field.undos[n-1].text, true
	}
	return "", false
}

// CanRedo is what Redo would do again, and whether there is anything.
func (s *State) CanRedo() (string, bool) {
	if n := len(s.field.redos); n > 0 {
		return s.field.redos[n-1].text, true
	}
	return "", false
}

// Undo takes back the last change, reporting whether there was one.
func (s *State) Undo() bool {
	n := len(s.field.undos)
	if n == 0 {
		return false
	}
	e := s.field.undos[n-1]
	s.field.undos = s.field.undos[:n-1]
	s.field.redos = append(s.field.redos, e)
	s.field.notice = nil
	e.undo()
	return true
}

// Redo does again the last change taken back, reporting whether there
// was one.
func (s *State) Redo() bool {
	n := len(s.field.redos)
	if n == 0 {
		return false
	}
	e := s.field.redos[n-1]
	s.field.redos = s.field.redos[:n-1]
	s.field.undos = append(s.field.undos, e)
	e.redo()
	return true
}

// undoNotice takes back what the notice says was done: the stack's top.
func (s *State) undoNotice() bool {
	if s.field.notice == nil {
		return false
	}
	return s.Undo()
}

// noticeView stands the notice, if one stands, on the pill's right at r,
// in the room to the field's edge w, its words giving way before its
// Undo; otherwise, while something is selected, how much. It keeps how
// far it is shown, for the stats to give way.
func (s *State) noticeView(c *ui.Context, semantic theme.Theme, r stageRect, w, gap, k float32, animate bool) {
	if n := s.field.notice; n != nil {
		if n.at.IsZero() {
			n.at = c.Now()
		}
		if left := noticeFor - c.Now().Sub(n.at); left <= 0 || n.query != s.wholeQuery() {
			s.field.notice = nil
		} else {
			c.After(left)
		}
	}
	n := s.field.notice
	text := ""
	switch {
	case n != nil:
		text = n.text
	case len(s.field.picked) > 0:
		text = locales.Selected(objectsCount(len(s.pickedOf("group:")), len(s.pickedOf("sheet:")), locales.GroupsCount, locales.SheetsCount))
	}
	shown := map[bool]float32{true: 1}[text != ""]
	row := ui.Row(c).Key("notice").Absolute().Top(0).Left(r.X+r.W+gap).Height(r.H).Translate(0, r.Y).
		MaxWidth(max(0, w-(r.X+r.W+gap)-gap)).AlignItems(ui.Center).Gap(6 * k)
	if animate {
		shown = components.Follow(row, "shown", shown, components.MotionQuick)
	}
	s.field.told = shown
	row.Opacity(shown * (1 - s.field.actingShown))
	if text == "" {
		row.IgnoreInput(true)
		return
	}
	s.field.lastNotice = text
	row.Children(func() {
		components.CenteredLabel(c, s.field.lastNotice, theme.TextXSmall, semantic.Text.Muted).SingleLine().Shrink(1).MinWidth(0)
		if n == nil {
			return
		}
		h := 24 * k
		btn := ui.ButtonBase(c.Key("undo")).Height(h).PaddingX(8 * k).Radius(h / 2).AlignItems(ui.Center).Shrink(0).
			FocusRing(false).KeepFocus().Label(locales.Undo())
		hover := components.HoverAmount(btn, btn.Hovered(), animate)
		btn.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
		if btn.Clicked() {
			s.undoNotice()
		}
		btn.Children(func() {
			components.CenteredLabel(c, locales.Undo(), theme.TextXSmall, semantic.Accent.Foreground).SingleLine()
		})
	})
}

// sheetsNamed names the sheets ids: one by its title, more by how many.
func (s *State) sheetsNamed(ids []string) string {
	if len(ids) == 1 {
		for _, cd := range s.field.found.cards {
			if b, ok := cd.kind.(block); ok {
				for _, e := range b.entries {
					if e.sheet.ID == ids[0] {
						return locales.Quoted(sheetTitle(e.sheet))
					}
				}
			}
		}
	}
	return locales.CountSheets(len(ids))
}

// rowIDs are ids as the field names their rows, each after kind.
func rowIDs(kind string, ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = kind + id
	}
	return out
}

// groupNamedByID is the group id's name, quoted.
func (s *State) groupNamedByID(id string) string {
	if g, ok := s.Library.Group(id); ok {
		return locales.Quoted(g.Name)
	}
	return locales.TheGroup()
}

// sheetPlace is where a sheet is filed: its group, and where in it.
type sheetPlace struct {
	id, group string
	index     int
}

// sheetPlaces are where the sheets ids are filed, the first in a group
// first, so that each put back in turn lands where it was.
func (s *State) sheetPlaces(ids []string) []sheetPlace {
	out := make([]sheetPlace, 0, len(ids))
	for _, id := range ids {
		g := s.Library.SheetGroup(id)
		at := slices.IndexFunc(s.Library.Sheets(Place{Kind: PlaceGroup, Group: g}), func(sh SheetItem) bool { return sh.ID == id })
		out = append(out, sheetPlace{id: id, group: g, index: max(at, 0)})
	}
	slices.SortStableFunc(out, func(a, b sheetPlace) int { return a.index - b.index })
	return out
}

// moveSheets files the sheets ids in group, to be undone.
func (s *State) moveSheets(ids []string, group string) {
	if len(ids) == 0 {
		return
	}
	text := locales.MovedTo(s.sheetsNamed(ids), s.groupNamedByID(group))
	from := s.sheetPlaces(ids)
	do := func() {
		for _, id := range ids {
			s.Library.MoveSheet(id, group, Last)
		}
		s.arrive(rowIDs("sheet:", ids)...)
	}
	do()
	s.tell(text, func() {
		for _, p := range from {
			s.Library.MoveSheet(p.id, p.group, p.index)
		}
		s.arrive(rowIDs("sheet:", ids)...)
	}, do)
}

// trashSheets puts the sheets ids in the trash, to be undone.
func (s *State) trashSheets(ids []string) {
	if len(ids) == 0 {
		return
	}
	text := locales.MovedToTrash(s.sheetsNamed(ids))
	from := s.sheetPlaces(ids)
	// A sheet open on the desk closes as it goes; taken back, it comes
	// back to its place in the field, marked there, and stays closed: the
	// undo is had on the field, and leaves the window looking at it.
	do := func() {
		for _, id := range ids {
			s.Library.TrashSheet(id)
		}
	}
	do()
	s.tell(text, func() {
		for _, p := range from {
			s.Library.RestoreSheet(p.id)
			s.Library.MoveSheet(p.id, p.group, p.index)
		}
		s.arrive(rowIDs("sheet:", ids)...)
	}, do)
}

// restoreSheets puts the sheets ids back out of the trash, to be undone.
func (s *State) restoreSheets(ids []string) {
	if len(ids) == 0 {
		return
	}
	text := locales.PutBackWhat(s.sheetsNamed(ids))
	do := func() {
		for _, id := range ids {
			s.Library.RestoreSheet(id)
		}
		s.arrive(rowIDs("sheet:", ids)...)
	}
	do()
	s.tell(text, func() {
		for _, id := range ids {
			s.Library.TrashSheet(id)
		}
	}, do)
}

// groupPlace is where a group stands: its parent, and where among its
// parent's groups, smart folders among them, as the library counts them
// (MoveGroup); Last for one not found.
func (s *State) groupPlace(id string) (parent string, index int) {
	var find func(parent string) (string, int, bool)
	find = func(parent string) (string, int, bool) {
		for i, g := range s.Library.Groups(parent) {
			if g.ID == id {
				return parent, i, true
			}
			if g.HasGroups {
				if p, at, ok := find(g.ID); ok {
					return p, at, true
				}
			}
		}
		return "", 0, false
	}
	if p, at, ok := find(""); ok {
		return p, at
	}
	return "", Last
}

// moveGroup files the group id in parent, to be undone.
func (s *State) moveGroup(id, parent string) {
	text := locales.MovedToTop(s.groupNamedByID(id))
	if parent != "" {
		text = locales.MovedInto(s.groupNamedByID(id), s.groupNamedByID(parent))
	}
	from, at := s.groupPlace(id)
	do := func() {
		s.Library.MoveGroup(id, parent, Last)
		s.arrive("group:" + id)
	}
	do()
	s.tell(text, func() {
		s.Library.MoveGroup(id, from, at)
		s.arrive("group:" + id)
	}, do)
}

// trashGroup puts the group g in the trash, to be undone.
func (s *State) trashGroup(g GroupItem) {
	do := func() { s.Library.TrashGroup(g.ID) }
	do()
	s.tell(locales.MovedToTrash(locales.Quoted(g.Name)), func() {
		s.Library.RestoreGroup(g.ID)
		s.arrive("group:" + g.ID)
	}, do)
}

// restoreGroup puts the group g back out of the trash, to be undone.
func (s *State) restoreGroup(g GroupItem) {
	do := func() {
		s.Library.RestoreGroup(g.ID)
		s.arrive("group:" + g.ID)
	}
	do()
	s.tell(locales.PutBackWhat(locales.Quoted(g.Name)), func() { s.Library.TrashGroup(g.ID) }, do)
}

// renameGroup names the group id name, to be undone.
func (s *State) renameGroup(id, name string) {
	g, ok := s.Library.Group(id)
	if !ok || g.Name == name {
		return
	}
	old := g.Name
	set := func(from, to string) func() {
		return func() {
			// The query going on in the group follows its name.
			for i, text := range s.field.conditions {
				if c, ok := condition(text); ok && c.Key.Name == "in" && strings.EqualFold(c.Value, from) {
					c.Value = to
					s.field.conditions[i] = c.String()
				}
			}
			s.field.conditions = ordered(s.field.conditions)
			s.Library.RenameGroup(id, to)
		}
	}
	set(old, name)()
	s.tell(locales.Renamed(old, name), set(name, old), set(old, name))
}

// deleteBookmark lets the bookmark id go; taken back, it is kept again
// as it was, its name and color with it.
func (s *State) deleteBookmark(id string) {
	g, ok := s.Library.Group(id)
	if !ok {
		return
	}
	current := id
	remove := func() {
		s.Library.TrashGroup(current)
		s.Library.DeleteGroup(current)
	}
	remove()
	s.tell(locales.DeletedBookmark(g.Name), func() {
		current = s.Library.NewSmartGroup(g.Name, g.Query)
		s.Library.StyleGroup(current, g.Icon, g.Color)
	}, remove)
}

// styleGroup gives the group g icon and color, to be undone.
func (s *State) styleGroup(g GroupItem, icon string, color int) {
	if g.Icon == icon && g.Color == color {
		return
	}
	do := func() { s.Library.StyleGroup(g.ID, icon, color) }
	do()
	s.tell(locales.Changed(g.Name), func() { s.Library.StyleGroup(g.ID, g.Icon, g.Color) }, do)
}

// pin pins the object id, or lets its pin go, to be undone.
func (s *State) pin(id string, pinned bool) {
	set := func(on bool) func() {
		return func() {
			s.Library.Pin(id, on)
			s.arrive(id)
		}
	}
	set(pinned)()
	text := map[bool]func(string) string{true: locales.PinnedWhat, false: locales.UnpinnedWhat}[pinned](s.objectsNamed([]string{id}))
	s.tell(text, set(!pinned), set(pinned))
}

// newGroup makes a group in parent and starts naming it; taken back, it
// goes to the trash, as a new folder does in the Finder.
func (s *State) newGroup(parent string) {
	id := s.Library.NewGroup(parent, Last)
	s.arrive("group:" + id)
	s.startRename(id)
	s.tell(locales.NewGroupTold(), func() { s.Library.TrashGroup(id) }, func() {
		s.Library.RestoreGroup(id)
		s.arrive("group:" + id)
	})
}

// duplicate copies the sheets, each filed after itself: one notice, one
// Undo, which puts the copies in the trash, as Finder's does.
func (s *State) duplicate(sheets []string) {
	ids := make([]string, len(sheets))
	for i, sheet := range sheets {
		ids[i] = "sheet:" + sheet
	}
	s.together(locales.DuplicatedWhat(s.objectsNamed(ids)), func() {
		for _, sheet := range sheets {
			copied := s.Library.DuplicateSheet(sheet)
			if copied == "" {
				continue
			}
			s.arrive("sheet:" + copied)
			s.tell(locales.Duplicated(), func() { s.Library.TrashSheet(copied) }, func() {
				s.Library.RestoreSheet(copied)
				s.arrive("sheet:" + copied)
			})
		}
	})
}

// FieldUndoes reports whether Undo and Redo are the field's: while it is
// in front and no name is being written, whose text undoes its own.
func (s *State) FieldUndoes() bool {
	return !s.OnDesk() && s.field.renaming == ""
}
