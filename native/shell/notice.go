package shell

import (
	"fmt"
	"slices"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// What moves or removes things says so on the pill's right, in place of
// the stats, with Undo (field-and-desk.md §4.7.1): for noticeFor, or until
// the query changes, or another takes its place. ⌘Z does the same while
// the field, not the pill's words, has the keys. What cannot be undone
// asks before it runs instead, and says nothing after.

// notice is what was just done, and how to take it back.
type notice struct {
	text string
	undo func()
	// query is the query it was done under; at is when it was first shown.
	query string
	at    time.Time
}

// noticeFor is how long a notice stands.
const noticeFor = 8 * time.Second

// tell puts up a notice of text, undone by undo.
func (s *State) tell(text string, undo func()) {
	s.field.notice = &notice{text: text, undo: undo, query: s.wholeQuery()}
}

// undoNotice takes back what the notice says was done, reporting whether
// there was one.
func (s *State) undoNotice() bool {
	n := s.field.notice
	if n == nil {
		return false
	}
	s.field.notice = nil
	n.undo()
	return true
}

// noticeView stands the notice, if one stands, on the pill's right at r,
// in the room to the field's edge w, its words giving way before its
// Undo; it keeps how far it is shown, for the stats to give way.
func (s *State) noticeView(c *ui.Context, semantic theme.Theme, r stage.Rect, w, gap, k float32, animate bool) {
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
	shown := map[bool]float32{true: 1}[s.field.notice != nil]
	row := ui.Row(c).Key("notice").Absolute().Top(0).Left(r.X+r.W+gap).Height(r.H).Translate(0, r.Y).
		MaxWidth(max(0, w-(r.X+r.W+gap)-gap)).AlignItems(ui.Center).Gap(6 * k)
	if animate {
		shown = components.Follow(row, "shown", shown, components.MotionQuick)
	}
	s.field.told = shown
	row.Opacity(shown * (1 - s.field.actingShown))
	n := s.field.notice
	if n == nil {
		row.IgnoreInput(true)
		return
	}
	s.field.lastNotice = n.text
	row.Children(func() {
		components.CenteredLabel(c, s.field.lastNotice, theme.TextXSmall, semantic.Text.Muted).SingleLine().Shrink(1).MinWidth(0)
		h := 24 * k
		btn := ui.ButtonBase(c.Key("undo")).Height(h).PaddingX(8 * k).Radius(h / 2).AlignItems(ui.Center).Shrink(0).
			FocusRing(false).KeepFocus().Label("Undo").Tooltip("Undo  ⌘Z")
		hover := components.HoverAmount(btn, btn.Hovered(), animate)
		btn.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
		if btn.Clicked() {
			s.undoNotice()
		}
		btn.Children(func() {
			components.CenteredLabel(c, "Undo", theme.TextXSmall, semantic.Accent.Foreground).SingleLine()
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
						return "“" + sheetTitle(e.sheet) + "”"
					}
				}
			}
		}
	}
	return fmt.Sprintf("%d %s", len(ids), sheetsWord(len(ids)))
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
		return "“" + g.Name + "”"
	}
	return "the group"
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
	text := "Moved " + s.sheetsNamed(ids) + " to " + s.groupNamedByID(group)
	from := s.sheetPlaces(ids)
	for _, id := range ids {
		s.Library.MoveSheet(id, group, Last)
	}
	s.arrive(rowIDs("sheet:", ids)...)
	s.tell(text, func() {
		for _, p := range from {
			s.Library.MoveSheet(p.id, p.group, p.index)
		}
		s.arrive(rowIDs("sheet:", ids)...)
	})
}

// trashSheets puts the sheets ids in the trash, to be undone.
func (s *State) trashSheets(ids []string) {
	if len(ids) == 0 {
		return
	}
	text := "Moved " + s.sheetsNamed(ids) + " to the Trash"
	from := s.sheetPlaces(ids)
	for _, id := range ids {
		s.Library.TrashSheet(id)
	}
	s.tell(text, func() {
		for _, p := range from {
			s.Library.RestoreSheet(p.id)
			s.Library.MoveSheet(p.id, p.group, p.index)
		}
		s.arrive(rowIDs("sheet:", ids)...)
	})
}

// restoreSheets puts the sheets ids back out of the trash, to be undone.
func (s *State) restoreSheets(ids []string) {
	if len(ids) == 0 {
		return
	}
	text := "Put back " + s.sheetsNamed(ids)
	for _, id := range ids {
		s.Library.RestoreSheet(id)
	}
	s.arrive(rowIDs("sheet:", ids)...)
	s.tell(text, func() {
		for _, id := range ids {
			s.Library.TrashSheet(id)
		}
	})
}

// groupPlace is where a group stands: its parent, and where among its
// parent's groups.
func (s *State) groupPlace(id string) (parent string, index int) {
	var path []string
	at := map[string]int{}
	for _, t := range s.Library.Targets() {
		path = append(path[:min(t.Depth, len(path))], t.ID)
		p := ""
		if t.Depth > 0 && t.Depth-1 < len(path) {
			p = path[t.Depth-1]
		}
		if t.ID == id {
			return p, at[p]
		}
		at[p]++
	}
	return "", Last
}

// moveGroup files the group id in parent, to be undone.
func (s *State) moveGroup(id, parent string) {
	text := "Moved " + s.groupNamedByID(id) + " to the top"
	if parent != "" {
		text = "Moved " + s.groupNamedByID(id) + " into " + s.groupNamedByID(parent)
	}
	from, at := s.groupPlace(id)
	s.Library.MoveGroup(id, parent, Last)
	s.arrive("group:" + id)
	s.tell(text, func() {
		s.Library.MoveGroup(id, from, at)
		s.arrive("group:" + id)
	})
}

// trashGroup puts the group g in the trash, to be undone.
func (s *State) trashGroup(g GroupItem) {
	s.Library.TrashGroup(g.ID)
	s.tell("Moved “"+g.Name+"” to the Trash", func() {
		s.Library.RestoreGroup(g.ID)
		s.arrive("group:" + g.ID)
	})
}

// restoreGroup puts the group g back out of the trash, to be undone.
func (s *State) restoreGroup(g GroupItem) {
	s.Library.RestoreGroup(g.ID)
	s.arrive("group:" + g.ID)
	s.tell("Put back “"+g.Name+"”", func() { s.Library.TrashGroup(g.ID) })
}
