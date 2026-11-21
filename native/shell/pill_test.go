package shell

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Words written search: the field shows the sheets found, and Return opens
// the first on the desk.
func TestWordsWrittenSearchAndReturnOpensTheFirst(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	tester.Type("chap")
	settle(tester)

	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.Contains(library.calls, "open found 7 chap") || !state.OnDesk() {
		t.Fatalf("Return should open the first found on the desk, but the calls are %v.", library.calls)
	}
}

// A condition written whole, a space after it, becomes a ribbon, and
// Backspace with nothing written takes it away.
func TestAConditionWrittenBecomesARibbon(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.Type("$trash ")
	settle(tester)
	if q := state.FieldQuery(); q != "$trash" || state.field.words != "" {
		t.Fatalf("$trash and a space should become a ribbon, but the query is %q with %q written.", q, state.field.words)
	}

	tester.Key(0, ui.KeyBackspace)
	settle(tester)
	if q := state.FieldQuery(); q != "" {
		t.Fatalf("Backspace with nothing written should take the ribbon away, but the query is %q.", q)
	}
}

// Escape lets go of the pill, the query kept; pressed on and on, it
// never takes the words or the conditions away and never goes anywhere.
func TestEscapeComesBackOneStepAtATime(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.SetFieldQuery("$in:Novel chap")
	settle(tester)
	if !state.field.open {
		t.Fatal("The pill should hold the keyboard on the field.")
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.field.open || state.FieldQuery() != "$in:Novel chap" {
		t.Fatalf("Escape in the pill should let go of it, the query kept, but it is open %v with %q.", state.field.open, state.FieldQuery())
	}
	for range 3 {
		tester.Key(0, ui.KeyEscape)
		settle(tester)
		if q := state.FieldQuery(); q != "$in:Novel chap" || state.OnDesk() {
			t.Fatalf("Escape should keep the query and the field, but the query is %q, on the desk %v.", q, state.OnDesk())
		}
	}
}

// Back and Forward walk the places the field has been, each with what was
// written in it.
func TestBackAndForwardWalkThePlaces(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.SetFieldQuery("chap")
	settle(tester)
	state.SetFieldQuery("$in:Novel")
	settle(tester)
	state.DoVerb(VerbBack)
	settle(tester)
	if q := state.FieldQuery(); q != "chap" {
		t.Fatalf("Back should come to where the field was, its words with it, but the query is %q.", q)
	}
	state.DoVerb(VerbForward)
	settle(tester)
	if q := state.FieldQuery(); q != "$in:Novel" {
		t.Fatalf("Forward should go again, but the query is %q.", q)
	}
	// The trash is a side trip: leaving it comes back to where it was
	// gone to from.
	state.toggleTrash()
	settle(tester)
	state.toggleTrash()
	settle(tester)
	if q := state.FieldQuery(); q != "$in:Novel" {
		t.Fatalf("Leaving the trash should come back to the group, but the query is %q.", q)
	}
}

// What is being written is suggested around the pill, as idle ribbons:
// the keys that start it, then its key's values, each taken with Tab; a
// value taken becomes a ribbon, and the field answers the query without
// what is half written, showing no card for the suggestions.
func TestSuggestionsStandAroundThePillAndTabTakesThem(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.Type("$up")
	settle(tester)
	if _, ok := tester.Find("Updated"); !ok {
		t.Fatalf("The key being written should be suggested around the pill: %v", tester.Texts())
	}
	if got := shownBlocks(state); !slices.Equal(got, []string{"group:1", "group:2"}) {
		t.Fatalf("The field should answer the query without what is half written, but shows %v.", got)
	}
	tester.Key(0, ui.KeyTab)
	settle(tester)
	if state.field.opened == nil || state.field.opened.Name != "updated" || state.field.words != "" {
		t.Fatalf("Tab should open the key, nothing of it written, but the pill holds %q.", state.field.words)
	}
	today, ok := tester.Find("today")
	if !ok {
		t.Fatalf("The key's values should stand around the pill: %v", tester.Texts())
	}
	tag, ok := tester.Find("Tag")
	if !ok || today.Y <= tag.Y {
		t.Fatalf("The keys should stay, the values under them: Tag %v, today %v.", tag, today)
	}
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyTab)
	settle(tester)
	if !slices.Equal(state.field.conditions, []string{"$updated:yesterday"}) || state.field.words != "" {
		t.Fatalf("Down twice then Tab should take the second value as a ribbon, but the query is %v %q.", state.field.conditions, state.field.words)
	}
}

// Open, the pill shows what the query can be narrowed by around it, idle;
// let go of, only what the query holds. An idle key pressed is written in
// the pill, for its value to be chosen.
func TestIdleFiltersStandAroundTheOpenPill(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if _, ok := tester.Find("Tag"); !ok {
		t.Fatalf("The open pill should offer Tag around it: %v", tester.Texts())
	}
	if err := tester.Click("Updated"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.field.opened == nil || state.field.words != "" {
		t.Fatalf("An idle key pressed should open in the pill, nothing written, but it holds %q.", state.field.words)
	}
	tester.ClickAt(12, 12)
	settle(tester)
	if _, ok := tester.Find("Tag"); ok {
		t.Fatal("Let go of, the pill should show only what the query holds.")
	}
}

// A tag pressed is taken and stays for more, pressed again let go; and
// the pill let go of, the field never answers the half written key.
func TestTagsToggleAndHalfWrittenKeysNeverReachTheField(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.field.suggestion != -1 {
		t.Fatalf("Nothing should be chosen until the keys go to it, but %d is.", state.field.suggestion)
	}
	if err := tester.Click("work"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Equal(state.field.conditions, []string{"$tag:work"}) || state.field.opened == nil || state.field.words != "" {
		t.Fatalf("A tag pressed should be taken, its key staying: %v %q.", state.field.conditions, state.field.words)
	}
	if err := tester.Click("work"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if len(state.field.conditions) != 0 {
		t.Fatalf("A tag taken, pressed again, should be let go: %v.", state.field.conditions)
	}
	tester.ClickAt(12, 12)
	settle(tester)
	if q := state.wholeQuery(); q != "" {
		t.Fatalf("The field should not answer the half written key, but answers %q.", q)
	}
}

// What is written in an open key narrows its values, and Return takes the
// first; a key whose value is written, as a title's, takes the words; a
// condition's × takes it away, whatever stands after it.
func TestOpenKeysNarrowTakeAndLetGo(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("we")
	settle(tester)
	if _, ok := tester.Find("work"); ok {
		t.Fatal("What is written should narrow the values.")
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.Equal(state.field.conditions, []string{"$tag:weekly"}) {
		t.Fatalf("Return should take the first value: %v.", state.field.conditions)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if err := tester.Click("More Filters"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("Title"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("plan")
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if len(state.field.conditions) != 2 || state.field.opened != nil || state.field.words != "" {
		t.Fatalf("Return should take the title as written and close: %v %q.", state.field.conditions, state.field.words)
	}
	if err := tester.Click("Remove Tag is weekly"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if len(state.field.conditions) != 1 {
		t.Fatalf("The × should take its condition away: %v.", state.field.conditions)
	}
}

// The pill let go of, what was under way in it settles: an open key comes
// back closed.
func TestAnOpenKeyClosesAsThePillIsLetGo(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.ClickAt(12, 12)
	settle(tester)
	state.Search()
	settle(tester)
	if state.field.opened != nil {
		t.Fatal("The key should come back closed.")
	}
	if _, ok := tester.Find("work"); ok {
		t.Fatal("Its values should be gone.")
	}
}

func TestHorizonStatsUpdateAndAnimate(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	search, ok := tester.Find("Search")
	if !ok {
		t.Fatal("Search pill should be present.")
	}
	countRect, ok := tester.Find("2")
	if !ok || countRect.X < search.X+search.W {
		t.Fatalf("Initial sheet count %v not to the right of search pill %v (ok=%v).", countRect, search, ok)
	}
	if !tester.HasText("sheets") || !tester.HasText("1,204") || !tester.HasText("words") {
		t.Fatalf("Initial library stats missing: %v", tester.Texts())
	}
	wordUnit0, _ := tester.Find("words")

	// Narrow to Inbox (1 sheet, 0 words): rolling counts update without
	// ellipsis truncation and shrink smoothly.
	if err := tester.Click("Inbox"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(40 * time.Millisecond)
	tester.Frame()
	if tester.HasText("…") || tester.HasText("...") {
		t.Fatal("Downward stats transition should never show an ellipsis placeholder.")
	}
	wordUnitMid, ok := tester.Find("words")
	if !ok {
		t.Fatal("Mid-roll frame missing word unit.")
	}

	settle(tester)
	if _, stillOld := tester.Find("1,204"); stillOld {
		t.Fatal("Old word count 1,204 should exit once settled.")
	}
	wordUnitSettled, ok := tester.Find("words")
	if !ok || !(wordUnitSettled.X < wordUnitMid.X && wordUnitMid.X < wordUnit0.X) {
		t.Fatalf("Stats width did not animate smoothly when shrinking: settledX=%v midX=%v initialX=%v",
			wordUnitSettled.X, wordUnitMid.X, wordUnit0.X)
	}
	if _, ok := tester.Find("sheet"); !ok {
		t.Fatal("Single-sheet filter should use singular unit 'sheet'.")
	}

	// Clearing the filter rolls back upward cleanly; interrupting mid-flight settles to 0.
	tester.Key(0, ui.KeyBackspace)
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	tester.Frame()
	if tester.HasText("…") || tester.HasText("...") {
		t.Fatal("Upward transition emitted an ellipsis placeholder.")
	}
	if err := tester.Click("Inbox"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !tester.HasText("0") || !tester.HasText("sheet") {
		t.Fatalf("Interrupted reversal did not settle to Inbox stats: %v", tester.Texts())
	}

	// Clear filter, then switch between commands (>) and settings ($settings).
	tester.Key(0, ui.KeyBackspace)
	settle(tester)
	if !tester.HasText("1,204") {
		t.Fatalf("Clearing filter did not restore full library stats: %v", tester.Texts())
	}
	state.Search()
	tester.Type(">")
	settle(tester)
	if !tester.HasText("commands") && !tester.HasText("command") {
		t.Fatal("Commands mode should display command count stats.")
	}
	if tester.HasText("words") {
		t.Fatal("Commands mode should collapse the words stats section.")
	}

	tester.Key(0, ui.KeyBackspace)
	tester.Type("$settings")
	settle(tester)
	if !tester.HasText("sections") && !tester.HasText("section") {
		t.Fatal("Settings mode should display section count stats.")
	}
	if tester.HasText("words") {
		t.Fatal("Settings mode should collapse the words stats section.")
	}
}

func TestRibbonTransitionsMirrorVerticallyAroundPill(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.ClickAt(12, 12)
	settle(tester)

	state.Search()
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	tester.Frame()

	bottomIn, okB := tester.Find("Tag")
	topIn, okT := tester.Find("Group")
	if !okB || !okT {
		t.Fatalf("Expected entering Tag (%v) and Group (%v) ribbons.", okB, okT)
	}

	settle(tester)
	pill, _ := tester.Find("Search")
	bottomRest, _ := tester.Find("Tag")
	topRest, _ := tester.Find("Group")
	if !(topRest.Y < pill.Y && bottomRest.Y > pill.Y) {
		t.Fatalf("Expected Group (%v) above and Tag (%v) below pill (%v).", topRest, bottomRest, pill)
	}

	midY := func(r ui.Rect) float32 { return r.Y + r.H/2 }
	if !(midY(bottomIn) < midY(bottomRest) && midY(topIn) > midY(topRest)) {
		t.Fatalf("Entering ribbons should mirror outward from pill: bottom (%v -> %v) down, top (%v -> %v) up.",
			midY(bottomIn), midY(bottomRest), midY(topIn), midY(topRest))
	}

	tester.ClickAt(12, 12)
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	tester.Frame()

	if !tester.FramePending() {
		t.Fatal("Blurring should animate exiting ribbons.")
	}
	settle(tester)
	if _, still := tester.Find("Tag"); still {
		t.Fatal("Tag ribbon should be gone once blurred and settled.")
	}
	if _, still := tester.Find("Group"); still {
		t.Fatal("Group ribbon should be gone once blurred and settled.")
	}
}

// Window unfocus runs dismiss animations smoothly for the focused search pill
// and its ribbons rather than snapping in a single frame.
func TestWindowBlurAnimatesFocusedPillAndRibbonsSmoothly(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	if !tester.Focused("Search") || !state.field.open {
		t.Fatal("Search pill should start focused and open.")
	}
	if _, ok := tester.Find("Tag"); !ok {
		t.Fatal("Key ribbons should be visible while the window and pill are focused.")
	}
	focusedPill, _ := tester.Find("Search")

	tester.SetFocused(false)
	tester.Frame()
	tester.Advance(40 * time.Millisecond)
	tester.Frame()

	if !tester.FramePending() {
		t.Fatal("Window blur should animate pill contraction and ribbon exit.")
	}
	midPill, _ := tester.Find("Search")
	settle(tester)
	settledPill, _ := tester.Find("Search")
	if !(settledPill.W < midPill.W && midPill.W < focusedPill.W-5) {
		t.Fatalf("Blurred window pill width did not animate down smoothly: focused=%v mid=%v settled=%v.",
			focusedPill.W, midPill.W, settledPill.W)
	}
	if _, still := tester.Find("Tag"); still {
		t.Fatal("Key ribbons should dismiss cleanly once window blur settles.")
	}

	tester.SetFocused(true)
	tester.Frame()
	tester.Advance(40 * time.Millisecond)
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("Window refocus should animate pill expansion and ribbons back in.")
	}
	settle(tester)
	if refocused, _ := tester.Find("Search"); abs(refocused.W-focusedPill.W) > 0.5 {
		t.Fatalf("Refocused window pill width = %v, want %v.", refocused.W, focusedPill.W)
	}
	if _, ok := tester.Find("Tag"); !ok {
		t.Fatal("Refocusing window should restore key ribbons.")
	}
}

// What is written and narrowed by is cleared beside the count of what it
// finds, not in the pill; with nothing written, there is no Clear.
func TestClearStandsBesideTheCount(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	if _, ok := tester.Find("Clear"); ok {
		t.Fatal("Clear stands with nothing written.")
	}
	state.SetFieldQuery("$in:Novel chap")
	settle(tester)
	if err := tester.Click("Clear Search"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "" {
		t.Fatalf("Cleared, the query is %q.", q)
	}
}

// A press on the field's controls leaves the keyboard with the pill, as a
// Mac's buttons do: it stays open, not closing as the press lands and
// opening again as the place it goes to asks for it.
func TestPressingTheLaneLeavesThePillOpen(t *testing.T) {
	state, _, tester := settingsState(t)
	for _, label := range []string{"Show settings", "Hide settings", "Show Trash", "Leave Trash", "View as List", "Home"} {
		if err := tester.Click("Search"); err != nil {
			t.Fatal(err)
		}
		settle(tester)
		r, ok := tester.Find(label)
		if !ok {
			t.Fatalf("No %q.", label)
		}
		tester.Press(r.X+r.W/2, r.Y+r.H/2)
		tester.Frame()
		if !tester.Focused("Search") || !state.field.open {
			t.Errorf("Pressing %q took the keyboard from the pill.", label)
		}
		tester.Release(r.X+r.W/2, r.Y+r.H/2)
		settle(tester)
	}
}

// The actions standing about the pill's end take room as the ribbons do:
// what stands near moves aside for them rather than under them.
func TestOpenActionsPushWhatStandsNear(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	// The pill let go of, no keys it offers hold room.
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.field.open {
		t.Fatal("Escape left the pill open.")
	}
	state.toggleActions()
	settle(tester)
	want := actionsReach(len(state.fieldActions()), 40, ribbonH, ribbonGap)
	if a, b := state.field.ribbonHold.above, state.field.ribbonHold.below; !state.field.acting || want <= 0 || a < want || b < want {
		t.Fatalf("Open, the actions kept %v above and %v below; they reach %v.", a, b, want)
	}
}
