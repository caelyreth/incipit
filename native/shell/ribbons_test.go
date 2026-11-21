package shell

import (
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/egoist/mygo/ui"
)

// pressKey opens the key name from its idle ribbon around the open pill.
func pressKey(t *testing.T, state *State, tester *ui.Tester, label string) {
	t.Helper()
	// A key past the first few stands behind More Filters.
	if _, ok := tester.Find(label); !ok {
		if err := tester.Click("More Filters"); err == nil {
			settle(tester)
		}
	}
	if err := tester.Click(label); err != nil {
		t.Fatalf("No %q ribbon: %v", label, tester.Texts())
	}
	settle(tester)
	if state.field.opened == nil {
		t.Fatalf("%q pressed should open in the pill.", label)
	}
}

// Every key the pill offers to narrow by offers what it takes: its values
// as they stand, or, for a key taking words, what is written; none opens
// empty once something is written.
func TestEveryKeyOffersItsValues(t *testing.T) {
	for _, name := range filterKeys {
		k := mustKey(name)
		if k.Takes == query.TakesNothing {
			continue
		}
		t.Run(name, func(t *testing.T) {
			state, _, tester := onField(t, 1260, 850)
			if name == "sort" {
				// A sort is the list's.
				state.toggleList()
				settle(tester)
			}
			pressKey(t, state, tester, k.Label)
			if k.Takes != query.TakesText && k.Takes != query.TakesName || name == "tag" {
				if len(state.valueRibbons()) == 0 {
					t.Fatalf("%s should offer its values at once.", name)
				}
			}
			// What is written, where the key takes it, is a value too.
			written := map[query.Takes]string{
				query.TakesText: "plan", query.TakesName: "github.com", query.TakesDate: "7d",
				query.TakesNumber: "500", query.TakesChoice: "",
			}[k.Takes]
			if name == "tag" {
				written = "plan"
			}
			if written == "" {
				return
			}
			tester.Type(written)
			settle(tester)
			label := query.Describe(k, written)
			if _, ok := tester.Find(label); !ok {
				t.Fatalf("What is written should stand as %q: %v", label, tester.Texts())
			}
			tester.Key(0, ui.KeyEnter)
			settle(tester)
			want := (&query.Condition{Key: k, Value: written}).String()
			if !slices.Contains(state.field.conditions, want) {
				t.Fatalf("Return should take what is written as %q, but the query is %v.", want, state.field.conditions)
			}
		})
	}
}

// What is written stands after the values it starts, so Return takes the
// first of them, not the letters written.
func TestWhatIsWrittenStandsAfterTheValuesItStarts(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	pressKey(t, state, tester, "Tag")
	tester.Type("wo")
	settle(tester)
	items := state.field.completion.Items
	if len(items) != 2 || items[0].Label != "work" || !items[1].Written {
		t.Fatalf("The tags written should be offered, what is written after: %v.", items)
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.Contains(state.field.conditions, "$tag:work") {
		t.Fatalf("Return should take work: %v.", state.field.conditions)
	}
}

// A key holding one value takes a new one in place of the old; tags
// gather.
func TestADateTakesThePlaceOfTheOneTaken(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	pressKey(t, state, tester, "Updated")
	if err := tester.Click("today"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	pressKey(t, state, tester, "Updated")
	if err := tester.Click("yesterday"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Equal(state.field.conditions, []string{"$updated:yesterday"}) {
		t.Fatalf("Yesterday should take today's place, but the query is %v.", state.field.conditions)
	}
	pressKey(t, state, tester, "Tag")
	for _, tag := range []string{"work", "weekly"} {
		if err := tester.Click(tag); err != nil {
			t.Fatal(err)
		}
		settle(tester)
	}
	if !slices.Contains(state.field.conditions, "$tag:work") || !slices.Contains(state.field.conditions, "$tag:weekly") {
		t.Fatalf("Tags should gather: %v.", state.field.conditions)
	}
}

// Each value says how many sheets it would find, counted against the
// query without the key's own value where it holds one; one that would
// find none is faded.
func TestValuesSayHowManyTheyWouldFind(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	pressKey(t, state, tester, "Tag")
	for _, p := range state.valueRibbons() {
		if want := len(rawValue(p.item)); p.count != want {
			t.Fatalf("%s should count %d, not %d.", p.item.Label, want, p.count)
		}
	}
	if _, ok := tester.Find("weekly"); !ok || !tester.HasText("6") {
		t.Fatalf("weekly's count should stand beside it: %v", tester.Texts())
	}
	tester.Type("none")
	settle(tester)
	for _, p := range state.valueRibbons() {
		if p.item.Written && state.look(state.Settings.Semantic(), p).shown >= 1 {
			t.Fatal("A value that would find nothing should be faded.")
		}
	}
	state.field.conditions = []string{"$updated:today", "$tag:work"}
	if got := state.countedWith(mustKey("updated")); got != "$tag:work" {
		t.Fatalf("A date should be counted without the date taken: %q.", got)
	}
	if got := state.countedWith(mustKey("tag")); got != "$updated:today $tag:work" {
		t.Fatalf("A tag should be counted with the tags taken: %q.", got)
	}
}

// The order is offered both ways, and the pill says what the key says of
// its value.
func TestTheOrderIsOfferedBothWays(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.toggleList()
	settle(tester)
	pressKey(t, state, tester, "Sorted by")
	if _, ok := tester.Find("updated, oldest first"); !ok {
		t.Fatalf("Oldest first should be offered: %v", tester.Texts())
	}
	if got := state.pillPlaceholder(); got != "Sorted by…" {
		t.Fatalf("placeholder %q", got)
	}
	state.closeKey()
	pressKey(t, state, tester, "Title")
	if got := state.pillPlaceholder(); got != "Title has…" {
		t.Fatalf("placeholder %q", got)
	}
}

// What the pill offers comes in as it opens, at a list's top as in the
// field: the pill making room for it never puts it away on the way.
func TestRibbonsComeInAtOnceWhereverThePillStands(t *testing.T) {
	for _, list := range []bool{false, true} {
		state, library, tester := onField(t, 1260, 850)
		library.extraSheets = 30
		state.SetFieldQuery("$in:Novel")
		if list {
			state.toggleList()
		}
		settle(tester)
		for i := 0; i < 3 && state.field.open; i++ {
			tester.Key(0, ui.KeyEscape)
			settle(tester)
		}
		if err := tester.Click("Search"); err != nil {
			t.Fatal(err)
		}
		for range 12 {
			tester.Advance(25 * time.Millisecond)
			tester.Frame()
			if state.field.away > 0 {
				t.Fatalf("Opening the pill (list %v) should not put its ribbons away: %v.", list, state.field.away)
			}
		}
	}
}
