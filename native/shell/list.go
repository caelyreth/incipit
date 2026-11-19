package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A place is shown as a field or as a list (field-and-desk.md §4.7.6). The
// field stands by nearness and never claims an exact order; the list is
// exact: the field's cards in rows under the pill, read left to right and
// top down, the place's groups first, then its sheets, as filed or as a
// sort says. A sort belongs to the list, so one written shows the list;
// the view is otherwise the place's, kept for the session. As filed, in a
// group, the list is where its order is set by hand: a card dragged
// between two others, or moved along by ⌥ and the arrows.

// place is the place the field's query is in, for its view: a group, the
// trash, or the top.
func (s *State) place() string {
	scope, trash, _ := s.browsing(query.Parse(s.wholeQuery()))
	switch {
	case trash:
		return "trash"
	case scope != "":
		return "group:" + scope
	}
	return ""
}

// sorted reports whether the query states an order of its own.
func (s *State) sorted() bool {
	for _, text := range s.field.conditions {
		if c, ok := condition(text); ok && written(c).Key.Name == "sort" {
			return strings.TrimPrefix(query.Fold(c.Value), "-") != "relevance"
		}
	}
	return false
}

// listed reports whether the field shows its place as a list: sorted, or
// browsing a place shown as one. A search starts in the field.
func (s *State) listed() bool {
	if s.Library == nil || s.InSettings() || strings.HasPrefix(s.field.words, ">") {
		return false
	}
	if s.sorted() {
		return true
	}
	_, _, browse := s.browsing(query.Parse(s.wholeQuery()))
	return browse && s.field.lists[s.place()]
}

// toggleList shows the place as a list, or as a field, taking its sort
// away, which belongs to the list.
func (s *State) toggleList() {
	if s.field.lists == nil {
		s.field.lists = map[string]bool{}
	}
	if !s.listed() {
		s.field.lists[s.place()] = true
		return
	}
	s.field.lists[s.place()] = false
	s.field.conditions = slices.DeleteFunc(s.field.conditions, func(text string) bool { return conditionKey(text) == "sort" })
}

// listLabel names what turning the place does.
func (s *State) listLabel() string {
	if s.listed() {
		return "Show as Field"
	}
	return "Show as List"
}

// listButton turns the place between the field and the list, filled while
// it stands as a list.
func (s *State) listButton(c *ui.Context, semantic theme.Theme, size float32, animate bool) ui.Element {
	label := s.listLabel()
	btn := s.laneCircleButton(c, semantic, "list", components.IconList, label, label, s.listed(), size, animate)
	if btn.Clicked() {
		s.toggleList()
	}
	return btn
}

// listRows are where a list's cards start a row: its sheets after its
// groups.
func listRows(cards []card) []bool {
	out := make([]bool, len(cards))
	for i := 1; i < len(cards); i++ {
		out[i] = isGroupCard(cards[i-1]) && !isGroupCard(cards[i])
	}
	return out
}

func isGroupCard(cd card) bool {
	b, ok := cd.kind.(block)
	return ok && b.group != nil && !b.single && !b.create
}

// arranging is the group whose list is set by hand, if the field shows
// one: listed as filed, browsing a group.
func (s *State) arranging() (string, bool) {
	if !s.listed() || s.sorted() {
		return "", false
	}
	g, ok := strings.CutPrefix(s.place(), "group:")
	if !ok {
		return "", false
	}
	group, ok := s.Library.Group(g)
	return g, ok && !group.Smart()
}

// filedIDs are the rows of kind, "sheet:" or "group:", in group as filed,
// those in the trash left out.
func (s *State) filedIDs(group, kind string) []string {
	var out []string
	if kind == "group:" {
		for _, g := range s.Library.Groups(group) {
			out = append(out, kind+g.ID)
		}
		return out
	}
	for _, sh := range s.Library.Sheets(Place{Kind: PlaceGroup, Group: group}) {
		out = append(out, kind+sh.ID)
	}
	return out
}

// arrange puts the row id, a sheet or a group in group, at index among
// those of its kind, to be undone.
func (s *State) arrange(group, id string, index int) {
	kind, bare, _ := strings.Cut(id, ":")
	kind += ":"
	from := slices.Index(s.filedIDs(group, kind), id)
	if from < 0 || index == from {
		return
	}
	move := func(at int) {
		if kind == "group:" {
			s.Library.MoveGroup(bare, group, at)
		} else {
			s.Library.MoveSheet(bare, group, at)
		}
		s.arrive(id)
	}
	move(index)
	s.tell("Moved "+s.objectsNamed([]string{id}), func() { move(from) })
}

// nudge moves the row the keys are on by along its group's order.
func (s *State) nudge(by int) bool {
	group, ok := s.arranging()
	kind, _, _ := strings.Cut(s.field.chosen, ":")
	if !ok || kind != "sheet" && kind != "group" {
		return false
	}
	ids := s.filedIDs(group, kind+":")
	at := slices.Index(ids, s.field.chosen)
	if at < 0 {
		return false
	}
	s.arrange(group, s.field.chosen, max(0, min(len(ids)-1, at+by)))
	return true
}

// arrangeTargets are where, while a row of the group arranged is dragged,
// it can be let go of among the others of its kind: the half of each card
// before it or after it, a line in the gap there showing where it goes.
func (s *State) arrangeTargets(c *ui.Context, semantic theme.Theme, drawn []stageFrame, at stagePoint, m blockMetrics, animate bool) {
	group, ok := s.arranging()
	if !ok {
		return
	}
	var dragged string
	switch d := s.field.drag.(type) {
	case sheetDrag:
		dragged = "sheet:" + d.id
	case groupDrag:
		dragged = "group:" + d.id
	default:
		return
	}
	kind, _, _ := strings.Cut(dragged, ":")
	ids := s.filedIDs(group, kind+":")
	if !slices.Contains(ids, dragged) {
		return
	}
	t := takes{sheets: kind == "sheet", groups: kind == "group"}
	gap := m.horizon.Gap
	for _, fr := range drawn {
		index := slices.Index(ids, fr.ID)
		if index < 0 || fr.ID == dragged {
			continue
		}
		// A group's card takes what is dropped on its head: its sides alone
		// arrange.
		side := fr.Rect.W / 2
		if kind == "group" {
			side = fr.Rect.W / 4
		}
		for _, after := range []bool{false, true} {
			x, end := fr.Rect.X, "-before"
			if after {
				x, end = x+fr.Rect.W-side, "-after"
			}
			zone := ui.Box(c.Key("arrange-"+fr.ID+end)).
				Absolute().Left(at.X+x).Top(at.Y+fr.Rect.Y).Size(side, fr.Rect.H)
			over, _, dropped := dragOn(zone, t)
			if dropped {
				to := index
				if after {
					to++
				}
				if from := slices.Index(ids, dragged); from < to {
					to--
				}
				s.arrange(group, dragged, to)
			}
			shown := float32(0)
			if over {
				shown = 1
			}
			if animate {
				shown = components.Follow(zone, "line", shown, components.MotionQuick)
			}
			if shown <= 0.001 {
				continue
			}
			lx := fr.Rect.X - gap/2
			if after {
				lx = fr.Rect.X + fr.Rect.W + gap/2
			}
			thin := 3 * m.k
			ui.Box(c.Key("arrange-line-"+fr.ID+end)).
				Absolute().Left(at.X+lx-thin/2).Top(at.Y+fr.Rect.Y).Size(thin, fr.Rect.H).Radius(thin / 2).
				Background(semantic.Accent.Foreground).Opacity(shown).IgnoreInput(true)
		}
	}
}
