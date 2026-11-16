package shell

import (
	"strings"

	"github.com/caelyreth/neverland/internal/query"
)

// What the field shows is what its query finds, in blocks: each a group
// with what it holds, its sheets in full as rows and the groups inside it
// as rows of their own. A query of no words, at most in a group or in the
// trash, browses: at the top every group, in a group that group and each
// group inside it, in the trash what is there. Any other query searches,
// and its sheets found stand in the blocks of the groups they are filed
// in, the blocks in the order of their best. Browsing is so a search with
// no words, and a group is what a click on its head adds to the query.

// block is one of the field's blocks.
type block struct {
	// id is the block's identity as it moves between layouts: "group:"
	// and a group's ID, or "trash".
	id string
	// group is the group the block is, nil for the trash's own.
	group *GroupItem
	// groups are the groups inside it it shows as rows, entries its sheets.
	groups  []GroupItem
	entries []entry
	// commands are the app's commands it lists, in a block of their own.
	commands []Command
	trashed  bool
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

// found is what a query found, for the library as it stood.
type found struct {
	query  string
	rev    uint64
	open   string
	blocks []block
	// total is how many sheets the query finds, those shown and those not.
	total int
}

// results is what the field's query finds, worked out again only when the
// query, the library or the sheets open change.
func (s *State) results() found {
	if s.Library == nil {
		return found{}
	}
	text := s.FieldQuery()
	open := openKey(s.Open)
	rev := s.Library.Revision()
	if f := s.field.found; f.query == text && f.rev == rev && f.open == open && f.blocks != nil {
		return f
	}
	f := found{query: text, rev: rev, open: open, blocks: []block{}}
	scope, trash, browse := s.browsing(query.Parse(text))
	words, commanding := strings.CutPrefix(s.field.words, ">")
	switch {
	case commanding:
		s.commanded(&f, words)
	case !browse:
		s.searched(&f, text, trash)
	case trash:
		if loose := s.Library.Sheets(Place{Kind: PlaceTrash}); len(loose) > 0 {
			f.blocks = append(f.blocks, block{id: "trash", entries: entries(loose), trashed: true})
		}
		for _, g := range s.Library.TrashedGroups() {
			f.blocks = append(f.blocks, s.groupBlock(g, true))
		}
	case scope != "" && s.lowest(scope):
		// Nothing but sheets in it: each sheet stands on its own.
		for _, e := range entries(s.Library.Sheets(Place{Kind: PlaceGroup, Group: scope})) {
			f.blocks = append(f.blocks, block{id: "sheet:" + e.sheet.ID, entries: []entry{e}, single: true})
		}
		if g, ok := s.Library.Group(scope); ok && !g.Smart() {
			f.blocks = append(f.blocks, block{id: "new", group: &g, create: true})
		}
	case scope != "":
		if g, ok := s.Library.Group(scope); ok {
			own := s.groupBlock(g, false)
			own.groups, own.scoped = nil, true
			f.blocks = append(f.blocks, own)
		}
		for _, g := range s.Library.Groups(scope) {
			f.blocks = append(f.blocks, s.groupBlock(g, false))
		}
	default:
		for _, g := range s.Library.Groups("") {
			f.blocks = append(f.blocks, s.groupBlock(g, false))
		}
	}
	if f.total == 0 {
		for _, b := range f.blocks {
			f.total += len(b.entries)
		}
	}
	s.field.found = f
	return f
}

// lowest reports whether the group holds no groups, only sheets.
func (s *State) lowest(id string) bool {
	g, ok := s.Library.Group(id)
	return ok && !g.HasGroups && !g.Smart()
}

// groupBlock is g's block: its sheets, and the groups inside it.
func (s *State) groupBlock(g GroupItem, trashed bool) block {
	b := block{id: "group:" + g.ID, group: &g, trashed: trashed}
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
			i = len(f.blocks)
			at[id] = i
			f.blocks = append(f.blocks, block{id: id, group: group, trashed: trash})
		}
		f.blocks[i].entries = append(f.blocks[i].entries, entry{sheet: hit.SheetItem, line: hit.Line})
	}
	f.total = max(hits.Total, len(hits.Sheets))
}

// commanded fills f with the commands whose names hold every word
// written, a block for each of their groups.
func (s *State) commanded(f *found, text string) {
	if s.Commands == nil {
		return
	}
	words := strings.Fields(strings.ToLower(text))
	at := map[string]int{}
	for _, cmd := range s.Commands() {
		label := strings.ToLower(cmd.Label)
		if !all(words, func(w string) bool { return strings.Contains(label, w) }) {
			continue
		}
		i, ok := at[cmd.Group]
		if !ok {
			i = len(f.blocks)
			at[cmd.Group] = i
			f.blocks = append(f.blocks, block{id: "commands:" + cmd.Group})
		}
		f.blocks[i].commands = append(f.blocks[i].commands, cmd)
	}
}

func all[T any](items []T, ok func(T) bool) bool {
	for _, it := range items {
		if !ok(it) {
			return false
		}
	}
	return true
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
