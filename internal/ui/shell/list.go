package shell

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
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
	if s.Library == nil || s.mode() != modeLibrary {
		return false
	}
	if s.sorted() {
		return true
	}
	if s.lens() != "" {
		return false
	}
	_, _, browse := s.browsing(query.Parse(s.wholeQuery()))
	return browse && s.placeListed()
}

// placeListed is how the place is shown: as turned in it this session,
// else as its group keeps (keepAsList), else as a field.
func (s *State) placeListed() bool {
	place := s.place()
	if listed, turned := s.field.lists[place]; turned {
		return listed
	}
	if id, ok := strings.CutPrefix(place, "group:"); ok && s.Library != nil {
		if g, ok := s.Library.Group(id); ok {
			return g.Listed
		}
	}
	return false
}

// keepAsList keeps g shown as a list, or lets that go: a choice of its
// own, marked on the group (listBadge), where turning the view passes.
// What was turned in it this session gives way to what it now keeps.
func (s *State) keepAsList(g GroupItem, listed bool) {
	set := func(on bool) func() {
		return func() {
			s.Library.KeepAsList(g.ID, on)
			delete(s.field.lists, "group:"+g.ID)
			s.field.listsRev++
		}
	}
	set(listed)()
	text := locales.AlwaysShowingAsList(g.Name)
	if !listed {
		text = locales.StoppedShowingAsList(g.Name)
	}
	s.tell(text, set(!listed), set(listed))
}

// keepItem is a menu's Always Show as List for g, checked while it does.
func (s *State) keepItem(m *ui.Menu, g GroupItem) {
	if components.MenuItem(m.Item(locales.AlwaysShowAsList()), components.IconList).Checked(g.Listed).Chosen() {
		s.keepAsList(g, !g.Listed)
	}
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
		return locales.ViewAsField()
	}
	return locales.ViewAsList()
}

// viewSwitch is how the place is shown, as two choices standing together,
// the field over the list, the one shown filled: both always in sight,
// so the other view is never a guess. size is a choice's width.
func (s *State) viewSwitch(c *ui.Context, semantic theme.Theme, size float32, animate bool) ui.Element {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	pad := 2 * k
	listed := s.listed()
	// A group kept as a list says so on the switch too: its List choice
	// is marked, whichever view stands now, so turning to the field reads
	// as passing.
	kept, keeps := s.scopeGroup()
	keeps = keeps && kept.Listed && !s.inTrash()
	box := ui.Column(c).Key("view-switch").Width(size).Padding(pad).Gap(pad).Radius(size/2).
		Border(1, semantic.Border.Variant).Background(semantic.Surface.Page).Role(ui.RoleRadioGroup).Label(locales.View())
	face := size - 2*pad
	// The fill is one circle standing behind the choice shown, sliding to
	// the other as the view turns, so the switch reads as one thing moving.
	at := map[bool]float32{true: 1}[listed]
	if animate {
		at = components.Follow(box, "thumb", at, components.MotionQuick)
	}
	box.Children(func() {
		ui.Box(c).Key("view-thumb").Absolute().Left(pad).Top(pad+at*(face+pad)).Size(face, face).Radius(face / 2).
			Background(semantic.Surface.Inverse).IgnoreInput(true).ZIndex(-1)
		for i, choice := range [...]struct {
			key, label string
			icon       components.IconName
			on         bool
		}{
			{"view-field", locales.ViewAsField(), components.IconMap, !listed},
			{"view-list", locales.ViewAsList(), components.IconList, listed},
		} {
			btn := ui.ButtonBase(c.Key(choice.key)).Size(face, face).Radius(face / 2).Role(ui.RoleRadio).
				AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).KeepFocus().Label(choice.label).Selected(choice.on)
			binding := ""
			if !choice.on {
				binding = s.binding(VerbViewList)
			}
			hint := choice.label
			marked := keeps && choice.key == "view-list"
			if marked {
				hint = locales.AlwaysIn(hint, kept.Name)
			}
			components.Hint(c, semantic, btn, hint, binding, components.HintAfter)
			// How far the fill stands behind this choice.
			on := max(0, 1-abs32(at-float32(i)))
			hover := components.HoverAmount(btn, btn.Hovered() && !choice.on, animate)
			btn.Background(components.Blend(semantic.GhostElement.Hover.Alpha(0), semantic.GhostElement.Hover, hover*(1-on)))
			if btn.Clicked() && !choice.on {
				s.toggleList()
			}
			btn.Children(func() {
				ui.Icon(c, choice.icon.SVG()).FontSize(face / 2).
					TextColor(components.Blend(semantic.Icon.Muted, semantic.Text.OnInverse, on))
				if marked {
					dot := 6 * k
					ui.Box(c).Key("kept-mark").Absolute().Top(0).Right(0).Size(dot, dot).Radius(dot/2).
						Background(semantic.Families[theme.Unripe].Foreground).Border(1, semantic.Surface.Page).
						IgnoreInput(true).Label(locales.AlwaysShownAsAList())
				}
			})
		}
	})
	return box
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
		s.listGroup(f, inner, append(slices.Clone(path), g.Title()), trashed)
	}
}

// listSorted adds what text finds in scope to f as its sort orders it:
// the sort's steps, if it has any, each a divider, and under each the
// groups the sheets are in, each a divider in turn, as first found.
func (s *State) listSorted(f *found, text, scope string, trash bool) {
	paths := s.paths(scope)
	by := strings.TrimPrefix(s.sortValue(), "-")
	var steps []string
	parts := map[string][]*hitBucket{}
	at := map[string]*hitBucket{}
	for _, hit := range s.Library.Search(text).Sheets {
		if s.inTray(hit.ID) {
			continue
		}
		step := sortStep(by, hit.SheetItem, f.now)
		if _, ok := parts[step]; !ok {
			steps = append(steps, step)
			parts[step] = nil
		}
		var group *GroupItem
		gid := ""
		if g, ok := s.hitGroup(hit.ID, trash); ok && g.ID != scope {
			group, gid = g, g.ID
		}
		p, ok := at[step+"/"+gid]
		if !ok {
			p = &hitBucket{group: group}
			at[step+"/"+gid] = p
			// The place's own sheets stand first in a step, under no group.
			if gid == "" {
				parts[step] = append([]*hitBucket{p}, parts[step]...)
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

// sortStep is the step of the sort by, as the query's $sort names it
// without its direction, a sheet stands in: when it was changed or made,
// how long it is; none for an order with no steps.
func sortStep(by string, sh SheetItem, now time.Time) string {
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
				return lengthStep(n)
			}
		}
	}
	return ""
}

// lengthStep says a span of lengths, as $words offers them.
func lengthStep(n query.Counts) string {
	switch {
	case !n.HasMin:
		return locales.LengthUnder(thousands(n.Max + 1))
	case !n.HasMax:
		return locales.LengthOver(thousands(n.Min))
	}
	return locales.LengthBetween(thousands(n.Min), thousands(n.Max))
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

// arrange files the sheets ids in group, together and in their order,
// before the sheet before among its sheets, or at their end for "", to be
// undone.
func (s *State) arrange(ids []string, group, before string) {
	order := s.sheetIDs(group)
	want := slices.DeleteFunc(slices.Clone(order), func(id string) bool { return slices.Contains(ids, id) })
	at := len(want)
	if i := slices.Index(want, before); i >= 0 {
		at = i
	}
	want = slices.Insert(want, at, ids...)
	if slices.Equal(want, order) {
		return
	}
	from := s.sheetPlaces(ids)
	do := func() { s.placeSheets(group, want, at, len(ids)) }
	do()
	text := locales.Moved(s.sheetsNamed(ids))
	if slices.ContainsFunc(from, func(p sheetPlace) bool { return p.group != group }) {
		text = locales.MovedTo(s.sheetsNamed(ids), s.groupNamedByID(group))
	}
	s.tell(text, func() {
		for _, p := range from {
			s.Library.MoveSheet(p.id, p.group, p.index)
		}
		s.arrive(rowIDs("sheet:", ids)...)
	}, do)
}

// placeSheets moves the n sheets at in want so group's sheets stand as
// want does: each goes just after the one before it there, which already
// stands where it will, so wherever each comes from, the rest keep their
// order around it.
func (s *State) placeSheets(group string, want []string, at, n int) {
	cur := s.sheetIDs(group)
	for _, id := range want[at : at+n] {
		cur = slices.DeleteFunc(cur, func(c string) bool { return c == id })
		to := 0
		if i := slices.Index(want, id); i > 0 {
			to = slices.Index(cur, want[i-1]) + 1
		}
		s.Library.MoveSheet(id, group, to)
		cur = slices.Insert(cur, to, id)
	}
	s.arrive(rowIDs("sheet:", want[at:at+n])...)
}

// arrangeGroup puts the group id in parent at index among its groups, as
// the library counts them (groupPlace), to be undone.
func (s *State) arrangeGroup(id, parent string, index int) {
	from, at := s.groupPlace(id)
	if from == parent && index == at {
		return
	}
	move := func(parent string, index int) {
		s.Library.MoveGroup(id, parent, index)
		s.arrive("group:" + id)
	}
	move(parent, index)
	s.tell(locales.Moved(s.groupNamedByID(id)), func() { move(from, at) }, func() { move(parent, index) })
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
			rest := slices.Delete(slices.Clone(ids), at, at+1)
			to, before := max(0, min(len(rest), at+by)), ""
			if to < len(rest) {
				before = rest[to]
			}
			s.arrange([]string{id}, group, before)
			return true
		}
	case "group":
		// Along the groups the field shows, which leave smart folders out:
		// to where the one it passes stands.
		parent, _ := s.groupPlace(id)
		shown := s.groups(parent)
		at := slices.IndexFunc(shown, func(g GroupItem) bool { return g.ID == id })
		if at < 0 {
			return false
		}
		_, to := s.groupPlace(shown[max(0, min(len(shown)-1, at+by))].ID)
		s.arrangeGroup(id, parent, to)
		return true
	}
	return false
}

// arrangeTargets are where, while a sheet is dragged in a list set by
// hand, it can be let go of: the half of each other sheet's card before it
// or after it, filing what it carries there in that sheet's group. The
// halves reach across the gaps around the cards to meet, so the drag is
// always over one and the place it lands is never lost between cards.
// Over one, the two cards either side of that gap ease apart and one line
// stands in it, gliding from gap to gap as the drag goes, where the sheets
// will go. A divider's head takes them whole, into its group, as a
// group's head does.
func (s *State) arrangeTargets(c *ui.Context, semantic theme.Theme, drawn []stageFrame, at stagePoint, m blockMetrics, animate bool) {
	d, ok := s.field.drag.(sheetDrag)
	if !ok || !s.arranging() {
		return
	}
	carried := s.carried(d)
	gap := m.horizon.Gap
	apart := 6 * m.k
	parts := map[string]float32{}
	var line stageRect
	landing := false
	for _, fr := range drawn {
		id, ok := strings.CutPrefix(fr.ID, "sheet:")
		if !ok || slices.Contains(carried, id) {
			continue
		}
		half := fr.Rect.W / 2
		for _, after := range []bool{false, true} {
			x, end := fr.Rect.X-gap/2, "-before"
			if after {
				x, end = fr.Rect.X+half, "-after"
			}
			zone := ui.Box(c.Key("arrange-"+fr.ID+end)).
				Absolute().Left(at.X+x).Top(at.Y+fr.Rect.Y-gap/2).Size(half+gap/2, fr.Rect.H+gap)
			over, _, dropped := dragOn(zone, takes{sheets: true})
			if dropped {
				s.arrange(carried, s.Library.SheetGroup(id), s.landingBefore(id, after, carried))
			}
			if !over {
				continue
			}
			// The gap: the card's side, and the card beside it across it.
			landing = true
			line = stageRect{X: fr.Rect.X - gap/2, Y: fr.Rect.Y, H: fr.Rect.H}
			parts[fr.ID] = apart
			if after {
				line.X = fr.Rect.X + fr.Rect.W + gap/2
				parts[fr.ID] = -apart
			}
			for _, o := range drawn {
				if o.Rect.Y == fr.Rect.Y && (after && o.Rect.X > fr.Rect.X || !after && o.Rect.X < fr.Rect.X) &&
					abs32(o.Rect.X-fr.Rect.X) < fr.Rect.W+gap+1 {
					parts[o.ID] = -parts[fr.ID]
				}
			}
		}
	}
	s.field.parts = parts
	s.landingLine(c, semantic, line, landing, at, m, animate)
}

// landingBefore is the sheet what is dropped by id, on its after side or
// before it, goes before among its group's sheets, those carried left
// out: "" for their end.
func (s *State) landingBefore(id string, after bool, carried []string) string {
	if !after {
		return id
	}
	ids := slices.DeleteFunc(s.sheetIDs(s.Library.SheetGroup(id)), func(o string) bool { return slices.Contains(carried, o) })
	if i := slices.Index(ids, id); i >= 0 && i+1 < len(ids) {
		return ids[i+1]
	}
	return ""
}

// landingLine is the one line in the gap a drag would land in, at r's
// left, as tall: it glides to the next gap as the drag moves on, growing
// as it comes, and fades where it last stood once no gap is under it.
func (s *State) landingLine(c *ui.Context, semantic theme.Theme, r stageRect, landing bool, at stagePoint, m blockMetrics, animate bool) {
	place := ui.Box(c.Key("arrange-line")).Absolute().Left(0).Top(0).IgnoreInput(true)
	shown := map[bool]float32{true: 1}[landing]
	x, y, h := r.X, r.Y, r.H
	if !landing {
		// Gone, it fades where it last stood.
		x, y, h = s.field.landing.X, s.field.landing.Y, s.field.landing.H
	}
	s.field.landing = stageRect{X: x, Y: y, H: h}
	grow := shown
	if animate {
		glide := components.Spring(components.MotionQuick)
		if s.field.landingShown < 0.01 {
			// Coming in, it stands where it lands at once, and grows there.
			place.Jump("x", x)
			place.Jump("y", y)
			place.Jump("h", h)
		}
		x, y, h = place.AnimateTo("x", x, glide), place.AnimateTo("y", y, glide), place.AnimateTo("h", h, glide)
		shown = components.Follow(place, "shown", shown, components.MotionQuick)
		grow = place.AnimateTo("grow", grow, components.Bouncy(components.MotionQuick))
	}
	s.field.landingShown = shown
	if shown <= 0.001 {
		return
	}
	thin, tall := 4*m.k, h*(0.5+0.5*grow)
	place.Children(func() {
		ui.Box(c.Key("arrange-line-bar")).Absolute().Left(at.X+x-thin/2).Top(at.Y+y+(h-tall)/2).Size(thin, tall).
			Radius(thin / 2).Background(semantic.Accent.Foreground).Opacity(shown)
	})
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
