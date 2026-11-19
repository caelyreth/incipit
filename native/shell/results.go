package shell

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
)

// The library is the field's first source: what its query finds, in
// blocks, each a group with what it holds, its sheets in full as rows and
// the groups inside it as rows of their own. A block is for telling
// groups apart (field-and-desk.md §4.1): the group the query is in heads
// none, its ribbon names it, and its sheets each stand on their own; so do
// a search's where it finds them all in one group, each naming it. A query
// of no words, at most in a group or in the trash, browses: at the top
// every group, in a group each group inside it and then its sheets, in
// the trash the same of what is there. Any other query searches, and its sheets found
// stand in the blocks of the groups they are filed in, in the order of
// their best, each block's head saying where its group is from the one
// the query is in. Browsing is so a search with no words, and a group is
// what a click on its head adds to the query. A query written after ">"
// is the commands'.

type librarySource struct{}

func (librarySource) answer(s *State, text string, f *found) {
	if s.modeFor(text) != modeLibrary {
		return
	}
	scope, trash, browse := s.browsing(query.Parse(text))
	if f.listed {
		s.listing(f, text, scope, trash, browse)
		return
	}
	switch {
	case !browse:
		s.searched(f, text, scope, trash)
	case trash:
		// The trash heads no block, as a group the query is in: the groups
		// in it, then its sheets, each on its own; what is done to it whole
		// is the pill's action (actions.go).
		s.filed(f, s.Library.TrashedGroups(), s.entries(s.Library.Sheets(Place{Kind: PlaceTrash})), true)
	case scope != "":
		// As a block's rows go: the groups inside it, then its sheets, then
		// the area that makes one.
		s.filed(f, s.groups(scope), s.entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: scope})), false)
		if g, ok := s.Library.Group(scope); ok && !g.Smart() {
			f.add("new", tierResult, block{group: &g, create: true})
		}
	default:
		s.filed(f, s.groups(""), nil, false)
	}
}

// block is the library's card: a group, the trash's loose sheets, a sheet
// standing on its own, or the area that makes a sheet.
type block struct {
	// group is the group the block is, nil for the trash's own.
	group *GroupItem
	// groups are the groups inside it it shows as rows, entries its sheets.
	groups  []GroupItem
	entries []entry
	trashed bool
	// single is whether it is one sheet standing on its own, as the sheets
	// of the group the query is in do; create is the block that makes a
	// sheet there.
	single, create bool
	// path is where the block's group is from the group the query is in,
	// the groups between, outermost first; none where it is inside it.
	path []string
	// divider is a group standing as its head alone, across a list, its
	// sheets under it; section is a sort's step standing so instead.
	divider bool
	section string
}

// entry is a sheet as a block shows it, where its text matches what was
// searched, and, standing on its own, the group it is in, where the query
// does not say.
type entry struct {
	sheet SheetItem
	line  string
	in    string
	// score is how well a search found it.
	score float64
}

// groups are the groups in parent the field shows: smart folders are
// saved queries, and stand as strips in the field lane instead (strips.go).
func (s *State) groups(parent string) []GroupItem {
	var out []GroupItem
	for _, g := range s.Library.Groups(parent) {
		if !g.Smart() {
			out = append(out, g)
		}
	}
	return out
}

// pinnedFirst calls group and sheet with the index of each of a place's
// groups and sheets in the order they stand: whatever is pinned first, of
// either kind, then the rest, groups before sheets on either side, each in
// its order. A pin is the hand's one say in the order of a place
// (field-and-desk.md §4.7.5), and anything filed can be pinned.
func pinnedFirst(groups []GroupItem, entries []entry, group, sheet func(int)) {
	for _, pinned := range []bool{true, false} {
		for i := range groups {
			if groups[i].Pinned == pinned {
				group(i)
			}
		}
		for i := range entries {
			if entries[i].sheet.Pinned == pinned {
				sheet(i)
			}
		}
	}
}

// made stands groups and entries as the field browses them, by when each
// was made: the groups oldest first, the Inbox among them, the sheets
// newest first. The field never reads the order they are filed in, which
// is the list's to show and set (field-and-desk.md §4.7.6), so arranging
// a list moves nothing in it.
func made(groups []GroupItem, entries []entry) ([]GroupItem, []entry) {
	groups, entries = slices.Clone(groups), slices.Clone(entries)
	slices.SortStableFunc(groups, func(a, b GroupItem) int { return cmp.Compare(idNumber(a.ID), idNumber(b.ID)) })
	slices.SortStableFunc(entries, func(a, b entry) int {
		return cmp.Or(b.sheet.Created.Compare(a.sheet.Created), cmp.Compare(idNumber(b.sheet.ID), idNumber(a.sheet.ID)))
	})
	return groups, entries
}

// idNumber is an identity as the number it is, made in order.
func idNumber(id string) int64 {
	n, _ := strconv.ParseInt(id, 10, 64)
	return n
}

// filed fills f with a place's groups, each a block, and its sheets, each
// on its own, as the field browses them.
func (s *State) filed(f *found, groups []GroupItem, entries []entry, trashed bool) {
	groups, entries = made(groups, entries)
	pinnedFirst(groups, entries, func(i int) {
		f.add("group:"+groups[i].ID, tierResult, s.groupBlock(groups[i], trashed))
	}, func(i int) {
		f.add("sheet:"+entries[i].sheet.ID, tierResult, block{entries: entries[i : i+1], single: true, trashed: trashed})
	})
}

// groupBlock is g's block: its sheets, and the groups inside it.
func (s *State) groupBlock(g GroupItem, trashed bool) block {
	b := block{group: &g, trashed: trashed}
	if g.Smart() {
		return b
	}
	b.entries = s.entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: g.ID}))
	if g.HasGroups && !trashed {
		b.groups = s.groups(g.ID)
	}
	b.groups, b.entries = made(b.groups, b.entries)
	return b
}

// entries are sheets as a block shows them, those set aside in the tray
// left out.
func (s *State) entries(sheets []SheetItem) []entry {
	out := make([]entry, 0, len(sheets))
	for _, sh := range sheets {
		if !s.inTray(sh.ID) {
			out = append(out, entry{sheet: sh})
		}
	}
	return out
}

type hitBucket struct {
	id    string
	group *GroupItem
	hits  []entry
}

// hitGroup resolves a search hit's owning group unless viewing the trash.
func (s *State) hitGroup(sheetID string, trash bool) (*GroupItem, bool) {
	if trash {
		return nil, false
	}
	g, ok := s.Library.Group(s.Library.SheetGroup(sheetID))
	if !ok {
		return nil, false
	}
	return &g, true
}

// searched fills f with what text finds in scope: each sheet in its
// group's block, the blocks in the order of their best; the scope's own,
// and all of them where one group holds them all, on their own.
func (s *State) searched(f *found, text, scope string, trash bool) {
	paths := s.paths(scope)
	var parts []*hitBucket
	at := map[string]*hitBucket{}
	for _, hit := range s.Library.Search(text).Sheets {
		if s.inTray(hit.ID) {
			continue
		}
		id, group := "trash", (*GroupItem)(nil)
		if g, ok := s.hitGroup(hit.ID, trash); ok {
			id, group = "group:"+g.ID, g
		}
		p, ok := at[id]
		if !ok {
			p = &hitBucket{id: id, group: group}
			at[id] = p
			parts = append(parts, p)
		}
		p.hits = append(p.hits, entry{sheet: hit.SheetItem, line: hit.Line, score: hit.Score})
	}
	for _, p := range parts {
		own := p.group != nil && p.group.ID == scope
		if !own && len(parts) > 1 {
			b := block{group: p.group, entries: p.hits, trashed: trash}
			if p.group != nil {
				path := paths[p.group.ID]
				b.path = path[:max(0, len(path)-1)]
			}
			f.add(p.id, tierResult, b)
			continue
		}
		for _, e := range p.hits {
			if !own && p.group != nil {
				e.in = strings.Join(paths[p.group.ID], " › ")
			}
			f.add("sheet:"+e.sheet.ID, tierResult, block{entries: []entry{e}, single: true, trashed: trash})
		}
	}
}

// paths are where each group inside scope is from it, by id: the names of
// the groups from the one inside scope to it, outermost first.
func (s *State) paths(scope string) map[string][]string {
	out := map[string][]string{}
	var walk func(parent string, path []string)
	walk = func(parent string, path []string) {
		for _, g := range s.Library.Groups(parent) {
			p := append(slices.Clone(path), g.Name)
			out[g.ID] = p
			if g.HasGroups {
				walk(g.ID, p)
			}
		}
	}
	walk(scope, nil)
	return out
}

// browsing reads q: the group it is in and whether it is in the trash, and
// whether it only browses there.
func (s *State) browsing(q query.Query) (scope string, trash, browse bool) {
	words := false
	query.Walk(q.Root, func(n query.Node) {
		if _, ok := n.(*query.Term); ok {
			words = true
		}
	})
	browse = !words
	groups := 0
	for _, c := range query.Conditions(q) {
		switch {
		case c.Not:
			browse = false
		case c.Key.Name == "trash":
			trash = true
		case c.Key.Name == "bands":
			// How the field stands, not what it finds.
		case c.Key.Name == "in":
			groups++
			g, ok := s.groupNamed(c.Value)
			if !ok {
				browse = false
			}
			scope = g.ID
		default:
			browse = false
		}
	}
	// In two groups at once is in neither alone: the field finds the
	// sheets in both.
	if groups > 1 {
		scope, browse = "", false
	}
	if scope != "" && trash {
		browse = false
	}
	return scope, trash, browse
}

// groupNamed is the group a query names, the first of that name.
func (s *State) groupNamed(name string) (GroupItem, bool) {
	var found GroupItem
	ok := false
	var walk func(parent string)
	walk = func(parent string) {
		for _, g := range s.Library.Groups(parent) {
			if ok {
				return
			}
			if strings.EqualFold(g.Name, name) {
				found, ok = g, true
				return
			}
			if g.HasGroups {
				walk(g.ID)
			}
		}
	}
	walk("")
	return found, ok
}

// openKey names the sheets open, in order.
func openKey(open []Page) string {
	var b strings.Builder
	for _, p := range open {
		b.WriteString(p.ID)
		b.WriteByte(',')
	}
	return b.String()
}
