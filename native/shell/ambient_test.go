package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/native/theme"
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
