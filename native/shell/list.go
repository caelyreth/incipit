package shell

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A place is shown as a field or as a list (field-and-desk.md §4.7.6). The
// field stands by nearness and never reads the order things are filed in;
// the list is exact, and the one place that order is shown and set. It
// reads top down and left to right, ending at the pill, which rests at
// its foot as the ground it stands on: the place's own sheets, then each
// group inside it as a divider, its name across the list, its sheets in
// rows of the field's cards under it, and the groups inside it so in
// turn. As filed, a card dragged between two others goes there, into
// another group's sheets filing it there, and ⌥ and the arrows move the
// row the keys are on; a sort orders it instead, its steps the dividers
// and the groups divided again under each. A sort belongs to the list and
// bands to the field, so either written shows its view; the view is
// otherwise the place's, kept for the session.

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

// sortValue is the query's active $sort value, folded (for example
// "updated", "-updated", "title", "words"), or "" when unsorted.
func (s *State) sortValue() string {
	for _, text := range s.field.conditions {
		if c, ok := condition(text); ok && written(c).Key.Name == "sort" {
			return query.Fold(c.Value)
		}
	}
	return ""
}

// sorted reports whether the query states an order of its own.
func (s *State) sorted() bool {
	v := s.sortValue()
	return v != "" && strings.TrimPrefix(v, "-") != "relevance"
}

// listed reports whether the field shows its place as a list: sorted, or
// browsing a place shown as one. A search starts in the field. It is
// asked many times a frame, by every ribbon among them, and worked out
// again only as what it reads changes.
func (s *State) listed() bool {
	key := listedKey{conds: strings.Join(s.field.conditions, "\x00"), words: s.field.words, held: s.field.held, caret: s.field.caret,
		opened: s.field.opened != nil, rev: s.field.listsRev}
	if s.Library != nil {
		key.library = s.Library.Revision() + 1
	}
	if m := &s.field.listedMemo; m.ok && m.key == key {
		return m.value
	}
	v := s.isListed()
	s.field.listedMemo = listedMemo{key: key, value: v, ok: true}
	return v
}

// listedKey is what listed reads; listedMemo what it last found.
type listedKey struct {
	conds, words, held string
	caret              int
	opened             bool
	rev                int
	// library is the library's revision, one on, 0 for none.
	library uint64
}

type listedMemo struct {
	key       listedKey
	value, ok bool
}

func (s *State) isListed() bool {
	if s.Library == nil || s.InSettings() || strings.HasPrefix(s.field.words, ">") {
		return false
	}
	if s.sorted() {
		return true
	}
	if s.lens() != "" {
		return false
	}
	_, _, browse := s.browsing(query.Parse(s.wholeQuery()))
	return browse && s.field.lists[s.place()]
}

// toggleList shows the place as a list, taking its bands away, or as a
// field, taking its sort away: each belongs to its view.
func (s *State) toggleList() {
	if s.field.lists == nil {
		s.field.lists = map[string]bool{}
	}
	// What listed found stands no more once the places' views change.
	defer func() { s.field.listsRev++ }()
	if !s.listed() {
		s.field.lists[s.place()] = true
		s.field.conditions = slices.DeleteFunc(s.field.conditions, func(text string) bool { return conditionKey(text) == "bands" })
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

// listing fills f with the place as its list: as filed while browsing,
// or as the sort orders what the query finds.
func (s *State) listing(f *found, text, scope string, trash, browse bool) {
	if !browse {
		s.listSorted(f, text, scope, trash)
		return
	}
	if trash {
		s.listSheets(f, s.entries(s.Library.Sheets(Place{Kind: PlaceTrash})), true)
		for _, g := range s.Library.TrashedGroups() {
			s.listGroup(f, g, nil, true)
		}
		return
	}
	if scope != "" {
		s.listSheets(f, s.entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: scope})), false)
		if g, ok := s.Library.Group(scope); ok && !g.Smart() {
			f.add("new", tierResult, block{group: &g, create: true})
		}
	}
	for _, g := range s.groups(scope) {
		s.listGroup(f, g, nil, false)
	}
}

// listSheets adds entries to f, each a card of its own.
func (s *State) listSheets(f *found, entries []entry, trashed bool) {
	for i := range entries {
		f.add("sheet:"+entries[i].sheet.ID, tierResult, block{entries: entries[i : i+1], single: true, trashed: trashed})
	}
}

// listGroup adds g to f as a divider, path the groups it is in from the
// place, its sheets under it, then the groups inside it so in turn.
func (s *State) listGroup(f *found, g GroupItem, path []string, trashed bool) {
	f.add("group:"+g.ID, tierResult, block{group: &g, path: path, divider: true, trashed: trashed})
	if g.Smart() {
		return
	}
	s.listSheets(f, s.entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: g.ID})), trashed)
	// Its sheets end with the area that makes one, as its card's do.
	if !trashed {
		f.add("new:"+g.ID, tierResult, block{group: &g, create: true})
	}
	for _, inner := range s.groups(g.ID) {
		s.listGroup(f, inner, append(slices.Clone(path), g.Name), trashed)
	}
}

// listSorted adds what text finds in scope to f as its sort orders it:
// the sort's steps, if it has any, each a divider, and under each the
// groups the sheets are in, each a divider in turn, as first found.
func (s *State) listSorted(f *found, text, scope string, trash bool) {
	paths := s.paths(scope)
	type part struct {
		group *GroupItem
		hits  []entry
	}
	var steps []string
	parts := map[string][]*part{}
	at := map[string]*part{}
	for _, hit := range s.Library.Search(text).Sheets {
		if s.inTray(hit.ID) {
			continue
		}
		step := s.sortStep(hit.SheetItem, f.now)
		if _, ok := parts[step]; !ok {
			steps = append(steps, step)
			parts[step] = nil
		}
		var group *GroupItem
		gid := ""
		if !trash {
			if g, ok := s.Library.Group(s.Library.SheetGroup(hit.ID)); ok && g.ID != scope {
				group, gid = &g, g.ID
			}
		}
		p, ok := at[step+"/"+gid]
		if !ok {
			p = &part{group: group}
			at[step+"/"+gid] = p
			// The place's own sheets stand first in a step, under no group.
			if gid == "" {
				parts[step] = append([]*part{p}, parts[step]...)
			} else {
				parts[step] = append(parts[step], p)
			}
		}
		p.hits = append(p.hits, entry{sheet: hit.SheetItem, score: hit.Score})
	}
	for _, step := range steps {
		if step != "" {
			f.add("step:"+step, tierResult, block{section: step})
		}
		for _, p := range parts[step] {
			if p.group != nil {
				path := paths[p.group.ID]
				f.add("group:"+p.group.ID+"@"+step, tierResult, block{group: p.group, path: path[:max(0, len(path)-1)], divider: true, trashed: trash})
			}
			s.listSheets(f, p.hits, trash)
		}
	}
}

// sortStep is the step of the query's sort a sheet stands in: when it was
// changed or made, how long it is; none for an order with no steps.
func (s *State) sortStep(sh SheetItem, now time.Time) string {
	by := strings.TrimPrefix(s.sortValue(), "-")
	switch by {
	case "updated", "created":
		t := sh.Updated
		if by == "created" {
			t = sh.Created
		}
		return dateStep(t, now)
	case "words":
		for _, v := range query.CountPresets {
			if n, ok := query.ParseCounts(v); ok && n.Contains(sh.Words) {
				d := query.Describe(mustKey("words"), v)
				return strings.ToUpper(d[:1]) + d[1:] + " words"
			}
		}
	}
	return ""
}

// listWide are the cards of a list that stand across it, its dividers,
// alone in their rows.
func listWide(cards []card) []bool {
	out := make([]bool, len(cards))
	for i, cd := range cards {
		b, ok := cd.kind.(block)
		out[i] = ok && (b.divider || b.section != "")
	}
	return out
}

// arranging reports whether the list is set by hand: listed as filed,
// browsing a place out of the trash that no query fills.
func (s *State) arranging() bool {
	if !s.listed() || s.sorted() {
		return false
	}
	scope, trash, browse := s.browsing(query.Parse(s.wholeQuery()))
	if !browse || trash {
		return false
	}
	g, ok := s.Library.Group(scope)
	return scope == "" || ok && !g.Smart()
}

// sheetIDs are the sheets filed in group, those in the trash left out.
func (s *State) sheetIDs(group string) []string {
	var out []string
	for _, sh := range s.Library.Sheets(Place{Kind: PlaceGroup, Group: group}) {
		out = append(out, sh.ID)
	}
	return out
}

// arrange files the sheet id in group at index among its sheets, its own
// group or another's, to be undone.
func (s *State) arrange(id, group string, index int) {
	from := s.Library.SheetGroup(id)
	at := slices.Index(s.sheetIDs(from), id)
	if from == group && index == at {
		return
	}
	move := func(group string, index int) {
		s.Library.MoveSheet(id, group, index)
		s.arrive("sheet:" + id)
	}
	move(group, index)
	text := "Moved " + s.sheetsNamed([]string{id})
	if from != group {
		text += " to " + s.groupNamedByID(group)
	}
	s.tell(text, func() { move(from, at) })
}

// arrangeGroup puts the group id at index among its parent's groups, to
// be undone.
func (s *State) arrangeGroup(id string, index int) {
	parent, at := s.groupPlace(id)
	if index == at {
		return
	}
	move := func(index int) {
		s.Library.MoveGroup(id, parent, index)
		s.arrive("group:" + id)
	}
	move(index)
	s.tell("Moved "+s.groupNamedByID(id), func() { move(at) })
}

// nudge moves the row the keys are on by along its order: a sheet among
// its group's sheets, a group among its parent's groups.
func (s *State) nudge(by int) bool {
	if !s.arranging() {
		return false
	}
	kind, id, _ := strings.Cut(s.field.chosen, ":")
	switch kind {
	case "sheet":
		group := s.Library.SheetGroup(id)
		ids := s.sheetIDs(group)
		if at := slices.Index(ids, id); at >= 0 {
			s.arrange(id, group, max(0, min(len(ids)-1, at+by)))
			return true
		}
	case "group":
		parent, at := s.groupPlace(id)
		if at == Last {
			return false
		}
		s.arrangeGroup(id, max(0, min(len(s.groups(parent))-1, at+by)))
		return true
	}
	return false
}

// arrangeTargets are where, while a sheet is dragged in a list set by
// hand, it can be let go of: the half of each other sheet's card before it
// or after it, filing it there in that sheet's group. Over one, the two
// cards either side of that gap ease apart and a line grows in it, where
// the sheet will go. A divider's head takes it whole, into its group, as
// a group's head does.
func (s *State) arrangeTargets(c *ui.Context, semantic theme.Theme, drawn []stageFrame, at stagePoint, m blockMetrics, animate bool) {
	d, ok := s.field.drag.(sheetDrag)
	if !ok || !s.arranging() {
		return
	}
	gap := m.horizon.Gap
	apart := 6 * m.k
	parts := map[string]float32{}
	for _, fr := range drawn {
		id, ok := strings.CutPrefix(fr.ID, "sheet:")
		if !ok || id == d.id {
			continue
		}
		side := fr.Rect.W / 2
		for _, after := range []bool{false, true} {
			x, end := fr.Rect.X, "-before"
			if after {
				x, end = x+side, "-after"
			}
			zone := ui.Box(c.Key("arrange-"+fr.ID+end)).
				Absolute().Left(at.X+x).Top(at.Y+fr.Rect.Y).Size(side, fr.Rect.H)
			over, _, dropped := dragOn(zone, takes{sheets: true})
			if dropped {
				group := s.Library.SheetGroup(id)
				ids := s.sheetIDs(group)
				to := slices.Index(ids, id)
				if after {
					to++
				}
				if from := slices.Index(ids, d.id); from >= 0 && from < to {
					to--
				}
				s.arrange(d.id, group, to)
			}
			// The gap: the card's side, and the card beside it across it.
			lx := fr.Rect.X - gap/2
			if after {
				lx = fr.Rect.X + fr.Rect.W + gap/2
			}
			if over {
				if after {
					parts[fr.ID] = -apart
				} else {
					parts[fr.ID] = apart
				}
				for _, o := range drawn {
					if o.Rect.Y == fr.Rect.Y && (after && o.Rect.X > fr.Rect.X || !after && o.Rect.X < fr.Rect.X) &&
						abs32(o.Rect.X-fr.Rect.X) < fr.Rect.W+gap+1 {
						parts[o.ID] = -parts[fr.ID]
					}
				}
			}
			shown := float32(0)
			if over {
				shown = 1
			}
			grow := shown
			if animate {
				shown = components.Follow(zone, "line", shown, components.MotionQuick)
				grow = zone.AnimateTo("grow", grow, components.Bouncy(components.MotionQuick))
			}
			if shown <= 0.001 {
				continue
			}
			thin, tall := 4*m.k, fr.Rect.H*(0.5+0.5*grow)
			ui.Box(c.Key("arrange-line-"+fr.ID+end)).
				Absolute().Left(at.X+lx-thin/2).Top(at.Y+fr.Rect.Y+(fr.Rect.H-tall)/2).Size(thin, tall).Radius(thin / 2).
				Background(semantic.Accent.Foreground).Opacity(shown).IgnoreInput(true)
		}
	}
	s.field.parts = parts
}

// partOf is how far aside the card id stands, easing, to open the gap a
// sheet dragged in a list would go into.
func (s *State) partOf(el ui.Element, id string, animate bool) float32 {
	to := s.field.parts[id]
	if !animate {
		return to
	}
	v := el.AnimateTo("part-"+id, to, components.Spring(components.MotionQuick))
	if v != 0 {
		s.field.parted = true
	}
	return v
}
