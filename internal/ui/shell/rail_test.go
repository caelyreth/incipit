package shell

import (
	"strings"
	"testing"
)

// A long list's rail holds a stop for each divider; pressed, a stop
// brings its divider to the list's top.
func TestTheRailTakesTheListToADivider(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 40
	state.toggleList()
	settle(tester)
	var stops []string
	for _, n := range tester.AccessAll() {
		if name, ok := strings.CutPrefix(n.Name, "Go to "); ok {
			stops = append(stops, name)
		}
	}
	if len(stops) < 2 {
		t.Fatalf("A long list should have a stop for each divider, not %v.", stops)
	}
	if err := tester.Click("Go to " + stops[0]); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.field.scroll != max(state.field.lo, 0) {
		t.Fatalf("Pressed, the first stop should bring the list to its top, not %v (lo %v).", state.field.scroll, state.field.lo)
	}
}
