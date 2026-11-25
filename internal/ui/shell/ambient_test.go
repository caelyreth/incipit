package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// quietCard is an ambient card for the tests, shown as on says.
type quietCard struct {
	name string
	on   bool
}

func (q quietCard) id() string           { return q.name }
func (q quietCard) shows(*State) bool    { return q.on }
func (q quietCard) card(*State) cardKind { return q }
func (quietCard) rows() []string         { return nil }
func (quietCard) ground() bool           { return true }
func (quietCard) drawsLive(*State) bool  { return false }
func (quietCard) shape(_ *State, _ *ui.Context, m blockMetrics) shape {
	return shape{size: stageSize{W: m.col + 2*m.pad, H: m.head + 2*m.pad}}
}
func (q quietCard) view(_ *State, c *ui.Context, semantic theme.Theme, box ui.Element, _ stageFrame, _ shape, _ bool, m blockMetrics, _ bool) {
	box.Children(func() { cardHead(c, semantic, "", q.name, m) })
}

// Ambient cards answer only an empty query, after everything else, never
// more than two, and only those that show.
func TestAmbientCardsStandQuietlyAfterAnEmptyQuery(t *testing.T) {
	defer func(was []ambient) { ambients = was }(ambients)
	ambients = []ambient{quietCard{"today", true}, quietCard{"off", false}, quietCard{"week", true}, quietCard{"line", true}}
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	var shown []string
	for _, id := range shownBlocks(state) {
		if name, ok := strings.CutPrefix(id, "ambient:"); ok {
			shown = append(shown, name)
		}
	}
	if !slices.Equal(shown, []string{"today", "week"}) {
		t.Fatalf("The first two that show should stand, and no more: %v.", shown)
	}
	if got := shownBlocks(state); !strings.HasPrefix(got[len(got)-1], "ambient:") || !strings.HasPrefix(got[len(got)-2], "ambient:") {
		t.Fatalf("Ambient cards should stand after everything else: %v.", got)
	}
	state.SetFieldQuery("chapter")
	settle(tester)
	if slices.ContainsFunc(shownBlocks(state), func(id string) bool { return strings.HasPrefix(id, "ambient:") }) {
		t.Fatalf("Once anything is asked, no ambient card should stand: %v.", shownBlocks(state))
	}
}

// ambientShown are the ambient cards the field shows at rest, by id.
func ambientShown(state *State) []string {
	var shown []string
	for _, id := range shownBlocks(state) {
		if name, ok := strings.CutPrefix(id, "ambient:"); ok {
			shown = append(shown, name)
		}
	}
	return shown
}

// A notebook with nothing written says how to begin, with New Sheet's
// key, and stops once a word is written.
func TestANewNotebookSaysHowToBegin(t *testing.T) {
	state, library := libraryState()
	library.unwritten = true
	state.Commands = func() []Command { return []Command{{ID: "page.new", Label: "New Sheet", Keys: []string{"⌘", "N"}}} }
	state.ShowField()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if got := ambientShown(state); !slices.Equal(got, []string{"first-run"}) {
		t.Fatalf("A notebook with nothing written should say how to begin: %v.", got)
	}
	first, ok := state.field.found.cards[len(state.field.found.cards)-1].kind.(note)
	if !ok || len(first.lines) != 3 || !slices.Equal(first.lines[1].keys, []string{"⌘", "N"}) {
		t.Fatalf("It should tell finding, writing with New Sheet's key, and filing: %+v.", first)
	}
	library.unwritten = false
	library.rev++
	settle(tester)
	if got := ambientShown(state); len(got) != 0 {
		t.Fatalf("Once anything is written, it should be gone: %v.", got)
	}
}

// A search that finds nothing says so under the pill, and Clear Search
// on its card goes home.
func TestNothingFoundOffersClearSearch(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.order = func(string) []string { return nil }
	state.SetFieldQuery("qqqqzzzz")
	tester.ClickAt(12, 12)
	settle(tester)
	if _, ok := tester.Find(locales.NothingHolds("qqqqzzzz")); !ok {
		t.Fatal("The field should say nothing holds what was searched.")
	}
	if err := tester.Click(locales.ClearSearch()); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "" {
		t.Fatalf("Clear Search should empty the query, not leave %q.", q)
	}
}
