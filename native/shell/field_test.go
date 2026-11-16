package shell

import (
	"testing"

	"github.com/caelyreth/neverland/native/stage"
	"github.com/egoist/mygo/ui"
)

func fieldLibraryState(t *testing.T) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	library.desk = state
	state.Open = []Page{{ID: "1", Title: "Draft"}}
	state.ActivePage = "1"
	state.Sheet = func(c *ui.Context, id string, _ SheetFrame) {
		ui.Text(c, "Sheet body "+id)
	}
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	return state, library, tester
}

func TestFieldOverviewDisplaysGroupBoardsAndScopesIntoGroup(t *testing.T) {
	state, _, tester := fieldLibraryState(t)

	state.ToggleField()
	settle(tester)

	pill, ok := tester.Find("Neverland pill")
	if !ok {
		t.Fatal("Missing Neverland pill in Arrive")
	}
	for _, label := range []string{"Inbox", "Novel", "· Chapter", "New sheet"} {
		card, ok := tester.Find(label)
		if !ok {
			t.Fatalf("Missing field card %q in %#v", label, tester.Texts())
		}
		if card.X < pill.X+pill.W && card.X+card.W > pill.X &&
			card.Y < pill.Y+pill.H && card.Y+card.H > pill.Y {
			t.Fatalf("Field card %q (%+v) overlaps pill (%+v)", label, card, pill)
		}
	}

	// Clicking a group board scopes the pill into that group.
	if err := tester.Click("Novel"); err != nil {
		t.Fatalf("Click Novel board: %v", err)
	}
	settle(tester)
	if state.PillScope() != "2" {
		t.Fatalf("PillScope = %q, want %q", state.PillScope(), "2")
	}
	if _, ok := tester.Find("Clear scope Novel"); !ok {
		t.Fatal("Pill should display active scope chip for Novel")
	}
	if _, ok := tester.Find("Chapter"); !ok {
		t.Fatal("Scoped Novel group should display Chapter card")
	}

	// Clearing the scope returns to the library overview.
	if err := tester.Click("Clear scope Novel"); err != nil {
		t.Fatalf("Click Clear scope Novel: %v", err)
	}
	settle(tester)
	if state.PillScope() != "" {
		t.Fatalf("PillScope after clear = %q, want empty", state.PillScope())
	}
}

func TestFieldCardOpensInPlaceAndBesideOnDesk(t *testing.T) {
	state, library, tester := fieldLibraryState(t)

	// In Glance, searching and pressing Cmd+Enter opens the matched sheet
	// beside the active one on the desk while keeping Glance open.
	state.ToggleGlance()
	settle(tester)
	tester.Type("chap")
	settle(tester)
	tester.Key(ui.Cmd, ui.KeyEnter)
	settle(tester)
	if state.Posture() != stage.PostureGlance {
		t.Fatalf("Opening beside in Glance should stay in Glance, got %v", state.Posture())
	}
	if len(state.Open) != 2 || state.Open[1].ID != "7" {
		t.Fatalf("Cmd+Enter should add Chapter (7) beside 1 on desk, got open=%+v", state.Open)
	}

	// Clearing the query and clicking Draft opens it in place and returns to Writing.
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if err := tester.Click("Draft"); err != nil {
		t.Fatalf("Click Draft card: %v", err)
	}
	settle(tester)
	if state.Posture() != stage.PostureWriting {
		t.Fatalf("Opening a card should return to Writing, got %v", state.Posture())
	}
	if len(state.Open) != 2 || state.ActivePage != "8" {
		t.Fatalf("Plain click should open Draft (8) in place of active sheet, got open=%+v active=%q", state.Open, state.ActivePage)
	}
	if len(library.calls) == 0 {
		t.Fatal("Expected library open calls to be recorded")
	}
}

func TestFieldPanningAndCentering(t *testing.T) {
	state, library, tester := fieldLibraryState(t)
	library.extraGroups = 8
	library.extraSheets = 10

	state.ToggleField()
	settle(tester)

	before, ok := tester.Find("Novel")
	if !ok {
		t.Fatal("Missing Novel card before pan")
	}

	state.PanField(-90, -60)
	settle(tester)
	after, ok := tester.Find("Novel")
	if !ok {
		t.Fatal("Novel card missing after modest pan")
	}
	if after.X >= before.X || after.Y >= before.Y {
		t.Fatalf("Panning (-90, -60) should shift cards left/up: before=%+v after=%+v", before, after)
	}

	if err := tester.Click("Center field"); err != nil {
		t.Fatalf("Click Center field: %v", err)
	}
	settle(tester)
	if got := state.FieldPan(); got != (stage.Point{}) {
		t.Fatalf("Center field should reset pan to zero, got %+v", got)
	}
}

func TestFieldQueryEmphasizesMatchesWithoutReflow(t *testing.T) {
	state, library, tester := fieldLibraryState(t)
	library.extraSheets = 4

	state.ToggleField()
	settle(tester)
	state.SetPillScope("2")
	settle(tester)

	tester.Type("Chapter")
	settle(tester)

	layout := state.FieldLayout()
	var chapter, other *stage.PlacedCard
	for i := range layout.Cards {
		c := &layout.Cards[i]
		if c.Item.ID == "sheet:7" {
			chapter = c
		} else if c.Item.Kind == stage.CardSheet && c.Item.Match == 0 {
			other = c
		}
	}
	if chapter == nil || other == nil {
		t.Fatalf("Expected both matched and unmatched sheets in layout, got %d cards", len(layout.Cards))
	}
	if chapter.Emphasis.Opacity != 1 || chapter.Emphasis.Scale <= 1 {
		t.Fatalf("Matched sheet should step forward: %+v (query=%q, item=%+v)", chapter.Emphasis, state.PillQuery(), chapter.Item)
	}
	if other.Emphasis.Opacity >= 1 || other.Emphasis.Scale >= 1 {
		t.Fatalf("Unmatched sheet should recede: %+v", other.Emphasis)
	}
}

func TestFieldNewSheetCardCreatesAndReturnsToDesk(t *testing.T) {
	state, library, tester := fieldLibraryState(t)
	state.ToggleField()
	settle(tester)

	if err := tester.Click("New sheet"); err != nil {
		t.Fatalf("Click New sheet card: %v", err)
	}
	settle(tester)
	if state.Posture() != stage.PostureWriting {
		t.Fatalf("New sheet should return to Writing, got %v", state.Posture())
	}
	if len(library.calls) == 0 || library.calls[len(library.calls)-1] != "new sheet 1 1073741824" {
		t.Fatalf("Expected NewSheet call in Inbox, got %v", library.calls)
	}
}

func TestFieldFidelitySteppingHashScopeAndFilterTokens(t *testing.T) {
	state, _, tester := fieldLibraryState(t)

	state.ToggleField()
	settle(tester)

	if got := state.PillFidelity(); got != stage.FidelityOverview {
		t.Fatalf("Initial fidelity = %v, want FidelityOverview", got)
	}
	if err := tester.Click("Show all sheets"); err != nil {
		t.Fatalf("Click Show all sheets: %v", err)
	}
	settle(tester)
	if got := state.PillFidelity(); got != stage.FidelitySheets {
		t.Fatalf("After stepping fidelity = %v, want FidelitySheets", got)
	}

	// Typing #Novel followed by space auto-applies group scope.
	tester.Type("#Novel ")
	settle(tester)
	if state.PillScope() != "2" {
		t.Fatalf("PillScope after #Novel = %q, want %q (query=%q)", state.PillScope(), "2", state.PillQuery())
	}

	// Clicking Trash token filters trashed sheets in the field.
	if err := tester.Click("Show trash"); err != nil {
		t.Fatalf("Click Show trash: %v", err)
	}
	settle(tester)
	if _, ok := tester.Find("Scrap"); !ok {
		t.Fatalf("Expected trashed sheet card when Trash filter active, got %#v", tester.Texts())
	}
}
