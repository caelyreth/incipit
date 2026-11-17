package shell

import (
	"slices"
	"testing"
)

// The group the query is in heads no block: its sheets stand on their own,
// browsed or searched, so narrowing in it keeps them where they are.
func TestTheScopeHeadsNoBlock(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.SetFieldQuery("$in:Novel")
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"sheet:7", "new"}) {
		t.Fatalf("In Novel, its sheet should stand on its own: %v.", got)
	}
	state.SetFieldQuery("$in:Novel $tag:work")
	settle(tester)
	if got := shownBlocks(state); !slices.Contains(got, "sheet:7") || slices.Contains(got, "group:2") {
		t.Fatalf("Narrowed in Novel, its sheets should still stand on their own: %v.", got)
	}
}

// What one group holds all of stands on its own, each sheet naming it;
// found in several, the sheets stand in their groups' blocks, each saying
// where its group is from the query's.
func TestBlocksTellGroupsApart(t *testing.T) {
	state, library := nestedGroupState()
	state.ShowField()
	state.SetFieldQuery("chapter")
	f := state.results()
	if ids := cardIDs(f); !slices.Equal(ids, []string{"sheet:7", "sheet:8"}) {
		t.Fatalf("All in Novel, the sheets should stand on their own: %v.", ids)
	}
	if b := f.cards[0].kind.(block); b.entries[0].in != "Novel" {
		t.Fatalf("A sheet on its own should name its group: %q.", b.entries[0].in)
	}
	library.homes = map[string]string{"7": "4", "8": "1"}
	library.rev++
	f = state.results()
	if ids := cardIDs(f); !slices.Equal(ids, []string{"group:4", "group:1"}) {
		t.Fatalf("Found in two groups, each should have its block: %v.", ids)
	}
	if b := f.cards[0].kind.(block); !slices.Equal(b.path, []string{"Novel", "Arc"}) {
		t.Fatalf("Scene's block should say where it is: %v.", b.path)
	}
	library.homes = map[string]string{"7": "2", "8": "4"}
	library.rev++
	state.SetFieldQuery("$in:Novel chapter")
	f = state.results()
	if ids := cardIDs(f); !slices.Equal(ids, []string{"sheet:7", "group:4"}) {
		t.Fatalf("In Novel, its own sheet should stand alone, Scene's in its block: %v.", ids)
	}
	if b := f.cards[0].kind.(block); b.entries[0].in != "" {
		t.Fatalf("A sheet of the group the query is in should not name it: %q.", b.entries[0].in)
	}
	if b := f.cards[1].kind.(block); !slices.Equal(b.path, []string{"Arc"}) {
		t.Fatalf("Scene's block should say where it is from Novel: %v.", b.path)
	}
}

func cardIDs(f found) []string {
	ids := make([]string, len(f.cards))
	for i, cd := range f.cards {
		ids[i] = cd.id
	}
	return ids
}

// In a group, as in a block's rows, the groups inside it come first, then
// its sheets, then the area that makes one.
func TestAGroupShowsItsGroupsThenItsSheets(t *testing.T) {
	state, _ := nestedGroupState()
	state.ShowField()
	state.SetFieldQuery("$in:Novel")
	if ids := cardIDs(state.results()); !slices.Equal(ids, []string{"group:3", "sheet:7", "new"}) {
		t.Fatalf("Novel should show Arc, then its sheet, then the area that makes one: %v.", ids)
	}
}
