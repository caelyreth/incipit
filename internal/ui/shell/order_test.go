package shell

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A sort is the list's: written, the place stands as a list, and the
// field is not offered one; the list is the place's, kept as it goes and
// comes back, and leaving it takes its sort away.
func TestASortShowsTheList(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	offered := func() bool {
		state.field.open = true
		defer func() { state.field.open = false }()
		return slices.ContainsFunc(state.keyRibbons(), func(p ribbonPlace) bool { return p.key != nil && p.key.Name == "sort" })
	}
	state.ShowField()
	settle(tester)
	if state.listed() || offered() {
		t.Fatal("The field should not stand as a list, nor offer a sort.")
	}
	state.SetFieldQuery("$sort:title")
	settle(tester)
	if !state.listed() || !offered() {
		t.Fatal("A sort written should show the list, which offers its sort.")
	}
	state.toggleList()
	if state.listed() || state.FieldQuery() != "" {
		t.Fatalf("Leaving the list should take its sort away: %q.", state.FieldQuery())
	}
	state.SetFieldQuery("$in:Novel")
	state.toggleList()
	state.SetFieldQuery("")
	if state.listed() {
		t.Fatal("The top should keep its own view.")
	}
	state.SetFieldQuery("$in:Novel")
	if !state.listed() {
		t.Fatal("Novel should come back as its list.")
	}
	state.SetFieldQuery("$in:Novel chapter")
	if state.listed() {
		t.Fatal("A search should start in the field.")
	}
}

// The list divides a place by its groups, their sheets under them; ⌥ and
// the arrows move a sheet along its group, and one dropped among another
// group's sheets is filed there, beside the one it was dropped on.
func TestAListIsArrangedByHand(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 3
	state.toggleList()
	settle(tester)
	ids := shownBlocks(state)
	if !slices.Contains(ids, "group:1") || !slices.Contains(ids, "group:2") || slices.Index(ids, "group:2") > slices.Index(ids, "sheet:7") {
		t.Fatalf("The groups should divide the list, their sheets under them: %v.", ids)
	}
	if !state.field.found.cards[slices.Index(ids, "group:2")].kind.(block).divider {
		t.Fatal("A group in a list should stand as its divider.")
	}
	state.field.chosen = "sheet:7"
	if !state.fieldKey(ui.TextKey{Key: ui.KeyRight, Mods: ui.Alt}) || !slices.Equal(library.moved, []string{"7@1"}) {
		t.Fatalf("⌥→ should move the sheet one on: %v.", library.moved)
	}
	dragOnto(t, tester, "Draft", "Sheet 2")
	if !slices.Contains(library.calls, "move 8 to 2") {
		t.Fatalf("A sheet dropped among another group's should be filed there: %v.", library.calls)
	}
	state.toggleList()
	settle(tester)
	state.field.chosen = "sheet:7"
	if state.fieldKey(ui.TextKey{Key: ui.KeyRight, Mods: ui.Alt}) {
		t.Fatal("The field has no order to arrange.")
	}
}

// The field reads no filed order: groups by when they were made, sheets
// newest made first, however they are filed.
func TestTheFieldReadsNoFiledOrder(t *testing.T) {
	now := time.Now()
	groups := []GroupItem{{ID: "12"}, {ID: "3"}}
	entries := []entry{{sheet: SheetItem{ID: "1", Created: now.Add(-time.Hour)}}, {sheet: SheetItem{ID: "2", Created: now}}}
	g, e := made(groups, entries)
	if g[0].ID != "3" || e[0].sheet.ID != "2" {
		t.Fatalf("made: %v %v", g, e)
	}
	g2, e2 := made([]GroupItem{groups[1], groups[0]}, []entry{entries[1], entries[0]})
	if !slices.Equal(g, g2) || e2[0].sheet.ID != e[0].sheet.ID {
		t.Fatal("Filed otherwise, the field should stand the same.")
	}
}

// The order of itself stands as nothing, as filed or the best match; a
// sort chosen stands as its ribbon.
func TestOnlyAChosenOrderStands(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	for _, label := range []string{"As filed", "Best match"} {
		if _, ok := tester.Find(label); ok {
			t.Fatalf("The order of itself should not stand: %q.", label)
		}
	}
	state.SetFieldQuery("$sort:updated")
	settle(tester)
	if !slices.ContainsFunc(state.ribbons(nil, 1, 630, 400, 40).places, func(p ribbonPlace) bool { return p.text == "$sort:updated" }) {
		t.Fatal("A sort chosen should stand as its ribbon.")
	}
}

// What arrives is brought into view and marked a moment.
func TestWhatArrivesIsShownAndMarked(t *testing.T) {
	state, library, tester := onField(t, 900, 460)
	var groups []GroupItem
	for i := range 24 {
		groups = append(groups, GroupItem{ID: fmt.Sprint("g", i), Name: fmt.Sprint("Group ", i), Count: 1})
	}
	library.groupTree = map[string][]GroupItem{"": groups}
	library.rev++
	settle(tester)
	inView := func(r ui.Rect, ok bool) bool { return ok && r.H > 0 && r.Y >= 0 && r.Y+r.H <= 460 }
	if r, ok := tester.Find("Group 23"); inView(r, ok) {
		t.Fatalf("Group 23 should stand out of view to begin with: %v.", r)
	}
	state.arrive("group:g23")
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	if state.arrivalAt("group:g23", tester.Now()) <= 0 {
		t.Fatal("What arrives should be marked as it is first shown.")
	}
	settle(tester)
	r, ok := tester.Find("Group 23")
	if !inView(r, ok) {
		t.Fatalf("What arrives should be brought into view: %v.", r)
	}
	if _, marked := state.field.arrivals["group:g23"]; marked {
		t.Fatal("The mark should go after its while.")
	}
}

// Anything filed can be pinned: what is pinned stands first, of either
// kind, then the rest, groups before sheets on either side; and a pinned
// sheet wears its badge.
func TestWhatIsPinnedStandsFirst(t *testing.T) {
	b := block{
		groups:  []GroupItem{{ID: "a"}, {ID: "b", Pinned: true}},
		entries: []entry{{sheet: SheetItem{ID: "x"}}, {sheet: SheetItem{ID: "y", Pinned: true}}},
	}
	if got := b.rows(); !slices.Equal(got, []string{"group:b", "sheet:y", "group:a", "sheet:x"}) {
		t.Fatalf("What is pinned should stand first, of either kind: %v.", got)
	}
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 2
	library.Pin("sheet:extra-1", true)
	settle(tester)
	rows := state.groupBlock(GroupItem{ID: "2", Name: "Novel"}, false).rows()
	if !slices.Equal(rows, []string{"group:2", "sheet:extra-1", "sheet:7", "sheet:extra-0"}) {
		t.Fatalf("The pinned sheet should stand first, the rest in their order: %v.", rows)
	}
	if _, ok := tester.Find("Pinned"); !ok {
		t.Fatal("A pinned sheet should wear its badge.")
	}
}

// A divider is pressed as a person presses: held a moment, the hand not
// quite still. It goes into its group, never taken for a drag however
// wide it stands.
func TestADividerHeldAMomentGoesIn(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 3
	state.toggleList()
	settle(tester)
	r, ok := tester.Find("Novel")
	if !ok {
		t.Fatal("Novel's divider should show.")
	}
	x, y := r.X+40, r.Y+r.H/2
	tester.Move(x, y)
	tester.Frame()
	tester.Press(x, y)
	for range 9 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	tester.Move(x+1, y)
	tester.Frame()
	tester.Release(x+1, y)
	settle(tester)
	if got := state.place(); got != "group:2" {
		t.Fatalf("Held a moment, the divider should go into its group, not %q.", got)
	}
}

// A list keeps its place as a browser's history does: opened in filed order,
// a new place starts at its end; scrolled up and gone into a group, the group
// opens at its own end; come back, the list stands as it was left.
func TestAListComesBackAsItWasLeft(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 40
	state.toggleList()
	settle(tester)
	if state.field.hi <= 0 || abs(state.field.scroll-state.field.hi) > 0.5 {
		t.Fatalf("A long filed list should open at its end: scroll=%v hi=%v.", state.field.scroll, state.field.hi)
	}
	visible := func(r ui.Rect, ok bool) bool {
		return ok && r.H > 0 && r.Y >= state.field.viewTop && r.Y+r.H <= state.field.viewBottom
	}
	for range 30 {
		if r, ok := tester.Find("Novel"); visible(r, ok) {
			break
		}
		tester.Scroll(600, 400, 0, -80)
		tester.Frame()
	}
	tester.Scroll(600, 400, 0, 0)
	settle(tester)
	watch := "Chapter"
	rWatch, ok := tester.Find(watch)
	if !visible(rWatch, ok) {
		t.Fatalf("%s should be in view near Novel: %+v.", watch, rWatch)
	}
	was, scroll := rWatch.Y, state.field.scroll
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.place() != "group:2" {
		t.Fatalf("Clicking Novel should enter group:2, got %q.", state.place())
	}
	if state.field.hi <= 0 || abs(state.field.scroll-state.field.hi) > 0.5 {
		t.Fatalf("A group gone into should open at its end: scroll=%v hi=%v.", state.field.scroll, state.field.hi)
	}
	state.broaden(0)
	settle(tester)
	if r, ok := tester.Find(watch); !ok || abs(state.field.scroll-scroll) > 0.5 || abs(r.Y-was) > 0.5 {
		t.Fatalf("Come back, the list should stand as it was left: scrolled %v, %s at %v, not %v and %v (ok=%v).", scroll, watch, was, state.field.scroll, r.Y, ok)
	}
}

// A list shorter than the window stands on the foot just above the pill, and
// ribbons appearing there push it up on a spring.
func TestAShortListStandsOnTheFootAndRisesWithRibbons(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 2
	state.toggleList()
	state.field.quiet = true
	settle(tester)
	if state.field.open {
		t.Fatal("Pill should be closed at rest before opening ribbons.")
	}
	if state.field.lo >= 0 || state.field.lo != state.field.hi || abs(state.field.scroll-state.field.hi) > 0.5 {
		t.Fatalf("A short list should pin to the foot: lo=%v hi=%v scroll=%v.", state.field.lo, state.field.hi, state.field.scroll)
	}
	var end float32
	for _, fr := range state.field.rest {
		end = max(end, fr.Rect.Y+fr.Rect.H)
	}
	bottom := state.field.origin.Y + end
	if abs(bottom-state.field.viewBottom) > 1 {
		t.Fatalf("A short list should rest on viewBottom (%v), not %v.", state.field.viewBottom, bottom)
	}
	wasBottom := bottom
	state.ShowField()
	settle(tester)
	afterBottom := state.field.origin.Y + end
	if afterBottom >= wasBottom-10 || abs(afterBottom-state.field.viewBottom) > 1 {
		t.Fatalf("Opening the pill should push a short list up with viewBottom: was=%v now=%v viewBottom=%v.", wasBottom, afterBottom, state.field.viewBottom)
	}
}

// Scrolled up, ribbons at the foot and new sheets at the end keep what is
// being read still; at the end, ribbons push it up cleanly.
func TestAListScrolledUpHoldsStillWhileTheEndFollowsTheFoot(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 40
	state.toggleList()
	state.field.quiet = true
	settle(tester)
	wasScroll := state.field.scroll
	// At the end, opening the pill grows ribbons above it and pushes the end up.
	state.ShowField()
	settle(tester)
	if state.field.scroll <= wasScroll+10 || abs(state.field.scroll-state.field.hi) > 1 {
		t.Fatalf("At the end, opening the pill should push the end up: was=%v scroll=%v hi=%v.", wasScroll, state.field.scroll, state.field.hi)
	}
	state.field.quiet = true
	settle(tester)
	// Scroll well up from the end so Chapter is visible inside the viewport.
	visible := func(r ui.Rect, ok bool) bool {
		return ok && r.H > 0 && r.Y >= state.field.viewTop && r.Y+r.H <= state.field.viewBottom
	}
	for range 30 {
		if r, ok := tester.Find("Chapter"); visible(r, ok) {
			break
		}
		tester.Scroll(600, 400, 0, -80)
		tester.Frame()
	}
	tester.Scroll(600, 400, 0, 0)
	settle(tester)
	r, ok := tester.Find("Chapter")
	if !visible(r, ok) {
		t.Fatalf("Chapter should be visible once scrolled up: %+v.", r)
	}
	wasY, wasScroll := r.Y, state.field.scroll
	// Opening the pill while scrolled up moves nothing you are reading.
	state.ShowField()
	settle(tester)
	if r2, _ := tester.Find("Chapter"); abs(state.field.scroll-wasScroll) > 0.5 || abs(r2.Y-wasY) > 0.5 {
		t.Fatalf("Scrolled up, opening ribbons should not move Chapter: scroll %v->%v, Y %v->%v.", wasScroll, state.field.scroll, wasY, r2.Y)
	}
	// Adding a sheet at the end while scrolled up leaves rows above untouched.
	library.extraSheets++
	library.rev++
	settle(tester)
	if r3, _ := tester.Find("Chapter"); abs(state.field.scroll-wasScroll) > 0.5 || abs(r3.Y-wasY) > 0.5 {
		t.Fatalf("Scrolled up, adding a sheet at the end should not move Chapter: scroll %v->%v, Y %v->%v.", wasScroll, state.field.scroll, wasY, r3.Y)
	}
}

type sortedTestLibrary struct {
	*fakeLibrary
	hits []FoundSheet
}

func (l *sortedTestLibrary) Search(text string) Found {
	out := slices.Clone(l.hits)
	if strings.Contains(text, "$sort:-updated") {
		slices.SortFunc(out, func(a, b FoundSheet) int { return b.Updated.Compare(a.Updated) })
	} else if strings.Contains(text, "$sort:updated") || strings.Contains(text, "$sort:created") {
		slices.SortFunc(out, func(a, b FoundSheet) int { return a.Updated.Compare(b.Updated) })
	}
	return Found{Sheets: out, Total: len(out)}
}

// Forward time sorts open at the end (showing Today by the pill, oldest at the
// top), while title, word-count, and reversed time sorts open at the top.
func TestListSortsPickTheirHome(t *testing.T) {
	now := time.Now()
	base := &fakeLibrary{}
	var hits []FoundSheet
	for i := range 45 {
		ts := now.AddDate(-2, 0, i)
		if i >= 41 {
			ts = now.Add(-time.Duration(45-i) * time.Minute)
		}
		hits = append(hits, FoundSheet{SheetItem: SheetItem{
			ID:      fmt.Sprint("s", i),
			Title:   fmt.Sprintf("Note %02d", i),
			Updated: ts,
			Created: ts,
			Words:   (i + 1) * 100,
		}})
	}
	lib := &sortedTestLibrary{fakeLibrary: base, hits: hits}
	state := New()
	state.Library = lib
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)

	state.SetFieldQuery("$sort:updated")
	settle(tester)
	if state.field.hi <= 0 || abs(state.field.scroll-state.field.hi) > 0.5 {
		t.Fatalf("$sort:updated should open at the end (Today by the pill): scroll=%v hi=%v.", state.field.scroll, state.field.hi)
	}
	if iEarlier, iToday := slices.Index(shownBlocks(state), "step:Earlier"), slices.Index(shownBlocks(state), "step:Today"); iEarlier < 0 || iToday < 0 || iEarlier > iToday {
		t.Fatalf("$sort:updated should read oldest-to-newest top-down: Earlier=%d Today=%d (%v).", iEarlier, iToday, shownBlocks(state))
	}

	for _, q := range []string{"$sort:title", "$sort:words", "$sort:-updated"} {
		state.SetFieldQuery(q)
		settle(tester)
		if state.field.hi <= 0 || state.field.scroll != 0 {
			t.Fatalf("%s should open at the top when long: scroll=%v hi=%v.", q, state.field.scroll, state.field.hi)
		}
	}
}

// Turning from the field to a list carries the pill directly from its screen
// place to the foot—never jumping to the top on a long list or straight to
// the foot on a short one—and coming back eases monotonically from the foot.
func TestFieldToListAnimatesPillFromItsPlace(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra int
		click bool
	}{
		{"long", 40, false},
		{"short-click", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, library, tester := onField(t, 1260, 850)
			library.extraSheets = tc.extra
			settle(tester)

			startPill, ok := tester.Find("Search")
			if !ok {
				t.Fatal("missing Search pill at rest")
			}

			if tc.click {
				if err := tester.Click("View as List"); err != nil {
					t.Fatal(err)
				}
			} else {
				state.toggleList()
			}

			var ys []float32
			for step := range 25 {
				if step > 0 {
					tester.Advance(16 * time.Millisecond)
				}
				tester.Frame()
				r, ok := tester.Find("Search")
				if !ok {
					t.Fatalf("step %d: missing Search pill", step)
				}
				ys = append(ys, r.Y)
			}
			settle(tester)
			endPill, _ := tester.Find("Search")

			if abs(ys[0]-startPill.Y) > 1 {
				t.Fatalf("frame 0 pill Y jumped from %v to %v (expected start at Field place)", startPill.Y, ys[0])
			}
			if ys[1] >= endPill.Y-1 || ys[4] >= endPill.Y-1 {
				t.Fatalf("pill should animate gradually rather than snapping straight to foot (%v): sample=%v", endPill.Y, ys[:5])
			}
			for i := 1; i < len(ys); i++ {
				if ys[i] < ys[i-1]-0.5 || ys[i] < startPill.Y-1 || ys[i] > endPill.Y+1 {
					t.Fatalf("pill left monotonic Field->List path [%v..%v] at step %d: %v -> %v",
						startPill.Y, endPill.Y, i, ys[i-1], ys[i])
				}
			}

			// List -> Field starts cleanly at the foot and moves monotonically upward.
			if tc.click {
				if err := tester.Click("View as Field"); err != nil {
					t.Fatal(err)
				}
			} else {
				state.toggleList()
			}
			var backYs []float32
			for step := range 25 {
				if step > 0 {
					tester.Advance(16 * time.Millisecond)
				}
				tester.Frame()
				r, ok := tester.Find("Search")
				if !ok {
					t.Fatalf("back step %d: missing Search pill", step)
				}
				backYs = append(backYs, r.Y)
			}
			if abs(backYs[0]-endPill.Y) > 1 {
				t.Fatalf("list->field frame 0 jumped from foot %v to %v", endPill.Y, backYs[0])
			}
			if backYs[len(backYs)-1] >= endPill.Y-20 {
				t.Fatalf("list->field did not ease up from foot %v: %v", endPill.Y, backYs[:8])
			}
			for i := 1; i < len(backYs); i++ {
				if backYs[i] > backYs[i-1]+0.5 {
					t.Fatalf("list->field moved downward at step %d: %v -> %v", i, backYs[i-1], backYs[i])
				}
			}
		})
	}
}

// Sorting the list another way lays none of its sheets out again: a
// sheet's card shows the same whatever the order, so the shapes it was
// given stand, and only where the cards go changes.
func TestResortingLaysNoCardOutAgain(t *testing.T) {
	now := time.Now()
	var hits []FoundSheet
	for i := range 30 {
		ts := now.Add(-time.Duration(i*29) * time.Hour)
		hits = append(hits, FoundSheet{SheetItem: SheetItem{ID: fmt.Sprint("s", i), Title: fmt.Sprintf("Note %02d", i),
			Excerpt: "An opening.", Updated: ts, Created: ts, Words: i * 90}})
	}
	state := New()
	state.Library = &sortedTestLibrary{fakeLibrary: &fakeLibrary{}, hits: hits}
	tester := ui.NewTester(state.View, 1260, 850)
	state.SetFieldQuery("$sort:updated")
	settle(tester)
	shaped := maps.Clone(state.field.shapes)
	for _, q := range []string{"$sort:-updated", "$sort:words", "$sort:title"} {
		state.SetFieldQuery(q)
		settle(tester)
	}
	for id := range state.field.shapes {
		if _, ok := shaped[id]; !ok && strings.HasPrefix(id.id, "sheet:") {
			t.Fatalf("Sorting another way laid %s out again.", id.id)
		}
	}
}

// orderedLibrary files sheets as MoveSheet says, each group's in order.
type orderedLibrary struct {
	*fakeLibrary
	groups map[string][]string
}

func (l *orderedLibrary) Sheets(place Place) []SheetItem {
	var out []SheetItem
	for _, id := range l.groups[place.Group] {
		out = append(out, SheetItem{ID: id, Title: id})
	}
	return out
}

func (l *orderedLibrary) SheetGroup(id string) string {
	for g, ids := range l.groups {
		if slices.Contains(ids, id) {
			return g
		}
	}
	return ""
}

func (l *orderedLibrary) MoveSheet(id, group string, index int) bool {
	for g := range l.groups {
		l.groups[g] = slices.DeleteFunc(l.groups[g], func(o string) bool { return o == id })
	}
	l.groups[group] = slices.Insert(l.groups[group], min(index, len(l.groups[group])), id)
	return true
}

// Sheets arranged together land together, in their order, before the one
// they were dropped by, wherever each stood, and are taken back as one.
func TestSheetsArrangedTogetherLandTogether(t *testing.T) {
	library := &orderedLibrary{fakeLibrary: &fakeLibrary{}, groups: map[string][]string{
		"2": {"y", "a", "x", "b"}, "1": {"z"},
	}}
	state := New()
	state.Library = library
	state.arrange([]string{"x", "y", "z"}, "2", "b")
	if got := library.groups["2"]; !slices.Equal(got, []string{"a", "x", "y", "z", "b"}) || len(library.groups["1"]) != 0 {
		t.Fatalf("Arranged before b: %v, %v.", got, library.groups["1"])
	}
	if !state.Undo() {
		t.Fatal("The arranging should be undone.")
	}
	if got := library.groups["2"]; !slices.Equal(got, []string{"y", "a", "x", "b"}) || !slices.Equal(library.groups["1"], []string{"z"}) {
		t.Fatalf("Taken back: %v, %v.", got, library.groups["1"])
	}
	state.arrange([]string{"a"}, "2", "")
	if got := library.groups["2"]; !slices.Equal(got, []string{"y", "x", "b", "a"}) {
		t.Fatalf("Arranged at the end: %v.", got)
	}
}

// In a list set by hand, the gap between two cards takes a sheet as their
// halves do: let go there, it is filed between them.
func TestAListsGapsTakeWhatIsDropped(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 3
	state.toggleList()
	settle(tester)
	a, _ := tester.Find("Sheet 1")
	b, _ := tester.Find("Sheet 2")
	if a.Y != b.Y || b.X <= a.X+a.W {
		t.Fatalf("Sheet 1 and Sheet 2 should stand side by side: %v, %v.", a, b)
	}
	draft, _ := tester.Find("Draft")
	dragTo(tester, draft.X+draft.W/2, draft.Y+draft.H/2, (a.X+a.W+b.X)/2, a.Y+a.H/2)
	if !slices.Contains(library.calls, "move 8 to 2") {
		t.Fatalf("Let go between two cards, the sheet should be filed with them: %v.", library.calls)
	}
}
