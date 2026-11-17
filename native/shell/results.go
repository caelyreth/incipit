package shell

import (
	"strings"

	"github.com/caelyreth/neverland/internal/query"
)

// The library is the field's first source: what its query finds, in
// blocks, each a group with what it holds, its sheets in full as rows and
// the groups inside it as rows of their own. A query of no words, at most
// in a group or in the trash, browses: at the top every group, in a group
// that group and each group inside it, in the trash what is there. Any
// other query searches, and its sheets found stand in the blocks of the
// groups they are filed in, the blocks in the order of their best.
// Browsing is so a search with no words, and a group is what a click on
// its head adds to the query. A query written after ">" is the commands'.

type librarySource struct{}

func (librarySource) answer(s *State, text string, f *found) {
	if strings.HasPrefix(s.field.words, ">") {
		return
	}
	scope, trash, browse := s.browsing(query.Parse(text))
	switch {
	case !browse:
		s.searched(f, text, trash)
	case trash:
		if loose := s.Library.Sheets(Place{Kind: PlaceTrash}); len(loose) > 0 {
			f.add("trash", tierResult, block{entries: entries(loose), trashed: true})
		}
		for _, g := range s.Library.TrashedGroups() {
			f.add("group:"+g.ID, tierResult, s.groupBlock(g, true))
		}
	case scope != "" && s.lowest(scope):
		// Nothing but sheets in it: each sheet stands on its own.
		for _, e := range entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: scope})) {
			f.add("sheet:"+e.sheet.ID, tierResult, block{entries: []entry{e}, single: true})
		}
		if g, ok := s.Library.Group(scope); ok && !g.Smart() {
			f.add("new", tierResult, block{group: &g, create: true})
		}
	case scope != "":
		if g, ok := s.Library.Group(scope); ok {
			own := s.groupBlock(g, false)
			own.groups, own.scoped = nil, true
			f.add("group:"+g.ID, tierScope, own)
		}
		for _, g := range s.Library.Groups(scope) {
			f.add("group:"+g.ID, tierResult, s.groupBlock(g, false))
		}
	default:
		for _, g := range s.Library.Groups("") {
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
	// scoped is whether the block is the group the query is in; single
	// whether it is one sheet standing on its own, as a group's sheets do
	// where nothing is inside it but sheets; create is the block that makes
	// a sheet there.
	scoped, single, create bool
}

// entry is a sheet as a block shows it, and where its text matches what
// was searched.
type entry struct {
	sheet SheetItem
	line  string
}

// lowest reports whether the group holds no groups, only sheets.
func (s *State) lowest(id string) bool {
	g, ok := s.Library.Group(id)
	return ok && !g.HasGroups && !g.Smart()
}

// groupBlock is g's block: its sheets, and the groups inside it.
func (s *State) groupBlock(g GroupItem, trashed bool) block {
	b := block{group: &g, trashed: trashed}
	if g.Smart() {
		return b
	}
	b.entries = entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: g.ID}))
	if g.HasGroups && !trashed {
		b.groups = s.Library.Groups(g.ID)
	}
	return b
}

func entries(sheets []SheetItem) []entry {
	out := make([]entry, len(sheets))
	for i, sh := range sheets {
		out[i] = entry{sheet: sh}
	}
	return out
}

// searched fills f with what text finds, each sheet in its group's block.
func (s *State) searched(f *found, text string, trash bool) {
	hits := s.Library.Search(text)
	at := map[string]int{}
	var blocks []block
	var ids []string
	for _, hit := range hits.Sheets {
		id := "trash"
		var group *GroupItem
		if !trash {
			if g, ok := s.Library.Group(s.Library.SheetGroup(hit.ID)); ok {
				id, group = "group:"+g.ID, &g
			}
		}
		i, ok := at[id]
		if !ok {
			i = len(blocks)
			at[id] = i
			blocks = append(blocks, block{group: group, trashed: trash})
			ids = append(ids, id)
		}
		blocks[i].entries = append(blocks[i].entries, entry{sheet: hit.SheetItem, line: hit.Line})
	}
	for i, b := range blocks {
		f.add(ids[i], tierResult, b)
	}
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
	for _, c := range query.Conditions(q) {
		switch {
		case c.Not:
			browse = false
		case c.Key.Name == "trash":
			trash = true
		case c.Key.Name == "in":
			g, ok := s.groupNamed(c.Value)
			if !ok {
				browse = false
			}
			scope = g.ID
		default:
			browse = false
		}
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
