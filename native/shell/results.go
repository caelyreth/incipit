package shell

import (
	"slices"
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
	if strings.HasPrefix(s.field.words, ">") || inSettings(text) {
		return
	}
	scope, trash, browse := s.browsing(query.Parse(text))
	switch {
	case !browse:
		s.searched(f, text, scope, trash)
	case trash:
		// The trash heads no block, as a group the query is in: the groups
		// in it, then its sheets, each on its own; what is done to it whole
		// is the pill's action (actions.go).
		for _, g := range s.Library.TrashedGroups() {
			f.add("group:"+g.ID, tierResult, s.groupBlock(g, true))
		}
		for _, e := range s.entries(s.Library.Sheets(Place{Kind: PlaceTrash})) {
			f.add("sheet:"+e.sheet.ID, tierResult, block{entries: []entry{e}, single: true, trashed: true})
		}
	case scope != "":
		// As a block's rows go: the groups inside it, then its sheets, then
		// the area that makes one.
		for _, g := range s.groups(scope) {
			f.add("group:"+g.ID, tierResult, s.groupBlock(g, false))
		}
		for _, e := range s.entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: scope})) {
			f.add("sheet:"+e.sheet.ID, tierResult, block{entries: []entry{e}, single: true})
		}
		if g, ok := s.Library.Group(scope); ok && !g.Smart() {
			f.add("new", tierResult, block{group: &g, create: true})
		}
	default:
		for _, g := range s.groups("") {
			f.add("group:"+g.ID, tierResult, s.groupBlock(g, false))
		}
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
}

// entry is a sheet as a block shows it, where its text matches what was
// searched, and, standing on its own, the group it is in, where the query
// does not say.
type entry struct {
	sheet SheetItem
	line  string
	in    string
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
	return pinnedFirst(out, func(g GroupItem) bool { return g.Pinned })
}

// pinnedFirst is items with those pinned first, each kind in its order:
// a pin is the hand's one say in the order of a place (field-and-desk.md
// §4.7.5).
func pinnedFirst[T any](items []T, pinned func(T) bool) []T {
	slices.SortStableFunc(items, func(a, b T) int {
		switch pa, pb := pinned(a), pinned(b); {
		case pa && !pb:
			return -1
		case pb && !pa:
			return 1
		}
		return 0
	})
	return items
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
	return pinnedFirst(out, func(e entry) bool { return e.sheet.Pinned })
}

// searched fills f with what text finds in scope: each sheet in its
// group's block, the blocks in the order of their best; the scope's own,
// and all of them where one group holds them all, on their own.
func (s *State) searched(f *found, text, scope string, trash bool) {
	paths := s.paths(scope)
	type part struct {
		id    string
		group *GroupItem
		hits  []entry
	}
	var parts []*part
	at := map[string]*part{}
	for _, hit := range s.Library.Search(text).Sheets {
		if s.inTray(hit.ID) {
			continue
		}
		id := "trash"
		var group *GroupItem
		if !trash {
			if g, ok := s.Library.Group(s.Library.SheetGroup(hit.ID)); ok {
				id, group = "group:"+g.ID, &g
			}
		}
		p, ok := at[id]
		if !ok {
			p = &part{id: id, group: group}
			at[id] = p
			parts = append(parts, p)
		}
		p.hits = append(p.hits, entry{sheet: hit.SheetItem, line: hit.Line})
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
