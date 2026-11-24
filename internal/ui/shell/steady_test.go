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
	inbox, _ := tester.Find("Inbox")
	novel, _ := tester.Find("Novel")
	tester.Type("we")
	settle(tester)
	if nowInbox, _ := tester.Find("Inbox"); nowInbox != inbox {
		t.Fatalf("Narrowing the values should leave Inbox where it was: %v, now %v.", inbox, nowInbox)
	}
	if nowNovel, _ := tester.Find("Novel"); nowNovel != novel {
		t.Fatalf("Narrowing the values should leave Novel where it was: %v, now %v.", novel, nowNovel)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if nowInbox, _ := tester.Find("Inbox"); nowInbox.Y >= inbox.Y {
		t.Fatalf("The key closed, the field should close in about the pill again: %v -> %v.", inbox, nowInbox)
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
	card, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("The matching card should stand on the field.")
	}
	mid := pill.Y + pill.H/2
	if mid > 850/2-5 || card.Y < pill.Y+pill.H {
		t.Fatalf("With all of the field under it, the pill should rise over the middle (%v) above its card (%v).", mid, card)
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
		pillMid := pill.X + pill.W/2
		cardMid := card.X + card.W/2
		if abs(cardMid-pillMid) > 1.0 {
			t.Fatalf("%d wide: the lone card (mid %v, %v) should stand centered under the pill (mid %v, %v).", w, cardMid, card, pillMid, pill)
		}
	}
}

// The room a condition's ribbons took is given back as it goes, however
// it goes: the field comes back home as it first stood.
func TestTheBandGivesBackWhatAConditionTook(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.ClickAt(12, 12)
	settle(tester)
	homePill, _ := tester.Find("Search")
	homeInbox, _ := tester.Find("Inbox")
	homeNovel, _ := tester.Find("Novel")
	state.SetFieldQuery("$tag:work")
	settle(tester)
	state.goHome()
	settle(tester)
	if p, _ := tester.Find("Search"); p != homePill {
		t.Fatalf("Home again, the pill should stand as it did: %v, now %v.", homePill, p)
	}
	if i, _ := tester.Find("Inbox"); i != homeInbox {
		t.Fatalf("Home again, Inbox should stand as it did: %v, now %v.", homeInbox, i)
	}
	if n, _ := tester.Find("Novel"); n != homeNovel {
		t.Fatalf("Home again, Novel should stand as it did: %v, now %v.", homeNovel, n)
	}
}

// Written away in the pill, a field that had risen and gone across for
// little settles back once the writing pauses, the pill still held.
func TestTheFieldSettlesAsTheWritingPauses(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	tester.ClickAt(12, 12)
	settle(tester)
	homePill, _ := tester.Find("Search")
	library.order = func(string) []string { return []string{"7"} }
	state.SetFieldQuery("chapter")
	settle(tester)
	movedPill, _ := tester.Find("Search")
	if abs(movedPill.Y-homePill.Y) <= 1 {
		t.Fatalf("With one card, the pill should rise otherwise than at home: home=%v moved=%v.", homePill, movedPill)
	}
	state.Search()
	settle(tester)
	for range len("chapter") {
		tester.Key(0, ui.KeyBackspace)
	}
	tester.Frame()
	heldPill, _ := tester.Find("Search")
	tester.Advance(writingPause / 2)
	tester.Frame()
	if nowPill, _ := tester.Find("Search"); abs(nowPill.Y-heldPill.Y) > 0.5 {
		t.Fatalf("While the writing goes on, the pill should hold: %v, now %v.", heldPill, nowPill)
	}
	tester.Advance(writingPause)
	settle(tester)
	if backPill, _ := tester.Find("Search"); abs((backPill.X+backPill.W/2)-(homePill.X+homePill.W/2)) > 0.5 || abs(backPill.Y-homePill.Y) > 0.5 {
		t.Fatalf("The writing paused, the field should settle as at home: %v, not %v.", homePill, backPill)
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
	at := func(q string) float32 {
		state.SetFieldQuery(q)
		tester.ClickAt(12, 12)
		settle(tester)
		pill, _ := tester.Find("Search")
		return pill.Y + pill.H/2
	}
	// Nothing found, the field shows the card that says so alone.
	bare, ribboned := at("nothing"), at("$tag:work $tag:plan $updated:today nothing")
	if abs(bare-ribboned) > 0.5 {
		t.Fatalf("The pill should stand where it does without ribbons: %v, not %v.", bare, ribboned)
	}
}
