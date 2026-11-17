package shell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// While a key's values are written, the room the ribbons take only grows:
// what they narrow to never pulls the field back.
func TestNarrowingAKeysValuesLeavesTheFieldBe(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	rest := slices.Clone(state.field.rest)
	tester.Type("we")
	settle(tester)
	if !slices.Equal(state.field.rest, rest) {
		t.Fatalf("Narrowing the values should leave the cards where they were: %v, now %v.", rest, state.field.rest)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if slices.Equal(state.field.rest, rest) {
		t.Fatal("The key closed, the field should close in about the pill again.")
	}
}

// What is written on keeps what it still finds where it stood, until the
// writing pauses; then it is ranked again.
func TestWritingKeepsTheRankUntilItPauses(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.homes = map[string]string{"8": "1"}
	library.order = func(text string) []string {
		if strings.HasSuffix(text, "ab") {
			return []string{"8", "7"}
		}
		return []string{"7", "8"}
	}
	state.Search()
	settle(tester)
	tester.Type("a")
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"group:2", "group:1"}) {
		t.Fatalf("Novel's block should lead: %v.", got)
	}
	tester.Type("b")
	tester.Frame()
	tester.Advance(100 * time.Millisecond)
	tester.Frame()
	if got := shownBlocks(state); !slices.Equal(got, []string{"group:2", "group:1"}) {
		t.Fatalf("Written on, the blocks should keep where they stood: %v.", got)
	}
	tester.Advance(writingPause)
	tester.Frame()
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"group:1", "group:2"}) {
		t.Fatalf("The writing paused, the blocks should be ranked again: %v.", got)
	}
}

// At rest, a field holding little stands even in the window, the pill
// rising into the room left empty over it; written in, the pill keeps
// still.
func TestTheHorizonRisesIntoEmptyRoomAndKeepsStillWhileWritten(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.order = func(string) []string { return []string{"7"} }
	state.SetFieldQuery("chapter")
	tester.ClickAt(12, 12)
	settle(tester)
	pill, _ := tester.Find("Search")
	mid := pill.Y + pill.H/2
	if mid > 850/2-5 {
		t.Fatalf("With all of the field under it, the pill should rise over the middle: %v.", mid)
	}
	// Its card, past the band under the pill, stands even about the
	// window's middle: the pill goes up half as far as the card reaches.
	below := float32(0)
	for _, fr := range state.field.rest {
		below = max(below, fr.Rect.Y+fr.Rect.H)
	}
	band := state.blockMetrics(1260-2*state.laneRoom(), 850).horizon.Below
	if want := 850/2 - (below-band)/2; abs(mid-want) > 1 {
		t.Fatalf("The pill should rise half as far as the card reaches: %v, not %v.", want, mid)
	}
	state.Search()
	settle(tester)
	held, _ := tester.Find("Search")
	tester.Type("x")
	settle(tester)
	if now, _ := tester.Find("Search"); now.Y != held.Y {
		t.Fatalf("Written in, the pill should keep still: %v, now %v.", held, now)
	}
}

// However many columns the window holds, a field of one card stands even
// about the pill at rest: in an even count, the card goes across to it.
func TestALoneCardStandsUnderThePillInAnyColumns(t *testing.T) {
	for _, w := range []int{900, 1260, 1600} {
		state, library, tester := onField(t, w, 850)
		library.order = func(string) []string { return []string{"7"} }
		state.SetFieldQuery("chapter")
		tester.ClickAt(12, 12)
		settle(tester)
		pill, _ := tester.Find("Search")
		card, ok := tester.Find("Chapter")
		if !ok {
			t.Fatalf("%d wide: the card should stand.", w)
		}
		cols := state.blockMetrics(float32(w)-2*state.laneRoom(), 850).horizon.Cols
		row := state.field.rest[0].Rect
		if mid := pill.X + pill.W/2; abs(row.X+row.W/2+state.field.poise.X) > 0.5 || card.X > mid {
			t.Fatalf("%d wide, %d columns: the card should stand under the pill at %v: %v, across %v.", w, cols, mid, row, state.field.poise.X)
		}
	}
}

// The room a condition's ribbons took is given back as it goes, however
// it goes: the field comes back home as it first stood.
func TestTheBandGivesBackWhatAConditionTook(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.ClickAt(12, 12)
	settle(tester)
	home := slices.Clone(state.field.rest)
	state.SetFieldQuery("$tag:work")
	settle(tester)
	state.goHome()
	settle(tester)
	if !slices.Equal(state.field.rest, home) {
		t.Fatalf("Home again, the field should stand as it did: %v, now %v.", home, state.field.rest)
	}
}

// Written away in the pill, a field that had risen and gone across for
// little settles back once the writing pauses, the pill still held.
func TestTheFieldSettlesAsTheWritingPauses(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	tester.ClickAt(12, 12)
	settle(tester)
	home := state.field.poise
	library.order = func(string) []string { return []string{"7"} }
	state.SetFieldQuery("chapter")
	settle(tester)
	if state.field.poise == home {
		t.Fatal("With one card, the field should stand otherwise than at home.")
	}
	state.Search()
	settle(tester)
	for range len("chapter") {
		tester.Key(0, ui.KeyBackspace)
	}
	tester.Frame()
	held := state.field.poiseTo
	tester.Advance(writingPause / 2)
	tester.Frame()
	if state.field.poiseTo != held {
		t.Fatalf("While the writing goes on, the field should hold: %v, now %v.", held, state.field.poiseTo)
	}
	tester.Advance(writingPause)
	settle(tester)
	if d := state.field.poise; abs(d.X-home.X) > 0.5 || abs(d.Y-home.Y) > 0.5 {
		t.Fatalf("The writing paused, the field should settle as at home: %v, not %v.", home, d)
	}
}

// Opening a key never moves the pill from under the pointer, though its
// values take room under it.
func TestOpeningAKeyKeepsThePillStill(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	pill, _ := tester.Find("Search")
	if err := tester.Click("Updated"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if now, _ := tester.Find("Search"); now.Y != pill.Y {
		t.Fatalf("Opening a key should leave the pill where it was: %v, now %v.", pill, now)
	}
}

// The ribbons are the pill's own: however many stand on one side of it,
// the pill stands at the window's middle while the cards stand even.
func TestRibbonsLeaveThePillAtTheMiddle(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.order = func(string) []string { return nil }
	state.SetFieldQuery("$tag:work $tag:plan $updated:today nothing")
	tester.ClickAt(12, 12)
	settle(tester)
	pill, _ := tester.Find("Search")
	if mid := pill.Y + pill.H/2; abs(mid-850/2) > 0.5 {
		t.Fatalf("With nothing found, the pill should stand at the middle whatever its ribbons: %v.", mid)
	}
}
