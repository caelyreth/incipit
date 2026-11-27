package shell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/text/query"
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
			pressKey(t, state, tester, k.Label())
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

// In list view, the bottom blur strip and its expanded ribbons area catch
// pointer hits so clicks over underlying cards do not pass through:
// clicking an actual ribbon still works, while clicking elsewhere on the
// blur dismisses the open pill just as clicking blank field does.
func TestListBackdropBlocksCardPassthroughAndDismissesPill(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 30
	library.rev++
	state.toggleList()
	settle(tester)
	// List view homes to the bottom when sorted by filed order; scroll to
	// the top so cards pass under the bottom blur backdrop (stuck > 0).
	state.field.scroll = max(state.field.lo, 0)
	settle(tester)
	if state.field.stuck <= 0 {
		t.Fatalf("Expected list view to scroll under the pill backdrop, stuck=%v", state.field.stuck)
	}

	// Pick a point on the blurred strip beside the pill/wordmark that overlaps
	// an underlying sheet row (if one sits there) or check expanded area.
	if err := tester.Click("Search"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !state.field.open || !tester.Focused("Search") {
		t.Fatal("Search pill should be open and focused")
	}

	// Find an expanded ribbon above the pill (e.g. "Sorted by") to locate the
	// expanded blur band, then click an empty horizontal spot on that same row
	// over the underlying list content.
	sortBtn, ok := tester.Find("Sorted by")
	if !ok {
		t.Fatalf("Expected Sorted by ribbon when open in list view, texts: %v", tester.Texts())
	}
	pillRect, _ := tester.Find("Search")
	emptyX := pillRect.X - 80
	clickY := sortBtn.Y + sortBtn.H/2
	// Verify an underlying sheet row actually overlaps emptyX, clickY? Or test
	// at a spot inside the blur edge where a sheet row lies horizontally.
	// Clicking near the center-right/left of an open sheet row inside the blur
	// strip must NOT open any sheet onto the desk and MUST dismiss the pill.
	tester.ClickAt(emptyX, clickY)
	settle(tester)
	if state.OnDesk() {
		t.Fatal("Clicking empty space on the expanded ribbons blur passed through to open a sheet")
	}
	if state.field.open || tester.Focused("Search") {
		t.Fatalf("Clicking empty space on the expanded ribbons blur should dismiss pill: open=%v focused=%v", state.field.open, tester.Focused("Search"))
	}

	// Also test directly over a visible sheet row clipped/overlapping the
	// bottom blur strip: clicking an actual ribbon still opens its key without
	// passing through, while clicking over a card inside the blur area does not.
	if err := tester.Click("Search"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("Sorted by"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.field.opened == nil || state.field.opened.Name != "sort" {
		t.Fatalf("Clicking ribbon on the blur should open sort key, got %v", state.field.opened)
	}
	if state.OnDesk() {
		t.Fatal("Clicking a ribbon should not pass through to an underlying sheet")
	}

	// Find a sheet whose bounds fall inside the blur area (Y >= 850-state.field.backdrop)
	// and click it at an X clear of the pill/ribbons: must not open the sheet.
	blurTop := float32(850) - state.field.backdrop
	var covered ui.Rect
	var coveredTitle string
	for _, a := range tester.AccessAll() {
		if strings.HasPrefix(a.Name, "Sheet ") && a.Bounds.H > 0 && a.Bounds.Y+a.Bounds.H > blurTop+4 {
			covered = a.Bounds
			coveredTitle = a.Name
			break
		}
	}
	if covered.W == 0 {
		t.Fatalf("Expected at least one sheet row to overlap the blur area (blurTop=%v)", blurTop)
	}
	// Click inside the overlapped card row on the right edge (under blur, clear
	// of pill/ribbons/wordmark).
	hitX := covered.X + covered.W - 8
	hitY := max(blurTop+4, covered.Y+covered.H/2)
	if hitY >= 850 {
		hitY = 850 - 8
	}
	tester.ClickAt(hitX, hitY)
	settle(tester)
	if state.OnDesk() {
		t.Fatalf("Clicking %q inside the blur area at (%v,%v) passed through to open it", coveredTitle, hitX, hitY)
	}
	if state.field.open || state.field.opened != nil {
		t.Fatalf("Clicking empty blur over %q should dismiss pill & opened key, got open=%v opened=%v", coveredTitle, state.field.open, state.field.opened)
	}

	// Verify clicking the same sheet above the blur area still opens it normally.
	var clearRow ui.Rect
	for _, a := range tester.AccessAll() {
		if strings.HasPrefix(a.Name, "Sheet ") && a.Bounds.H > 0 && a.Bounds.Y > 80 && a.Bounds.Y+a.Bounds.H < blurTop-4 {
			clearRow = a.Bounds
			break
		}
	}
	if clearRow.W == 0 {
		t.Fatalf("Expected a sheet row above the blur area (blurTop=%v)", blurTop)
	}
	tester.ClickAt(clearRow.X+clearRow.W/2, clearRow.Y+clearRow.H/2)
	settle(tester)
	if !state.OnDesk() {
		t.Fatal("Clicking a sheet row above the blur strip should open it on the desk")
	}
}
