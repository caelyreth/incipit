package shell

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

func pickingState(t *testing.T, sheets int) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	library.extraSheets = sheets
	state.ActivePage = "7"
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	return state, library, tester
}

func TestShiftClickPicksARunOfSheets(t *testing.T) {
	state, library, tester := pickingState(t, 4)
	if err := tester.Click("Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 3"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := state.pickedSheets(); !slices.Equal(got, []string{"extra-0", "extra-1", "extra-2"}) {
		t.Fatalf("picked %v", got)
	}
	if slices.Contains(library.calls, "open extra-2") {
		t.Fatalf("a shift-click opened the sheet: %v", library.calls)
	}
	if !tester.HasText("3 selected · 300 words") {
		t.Fatalf("the header does not count the picked: %v", tester.Texts())
	}
	// A shift-click from the same anchor the other way picks back to it.
	if err := tester.ClickWith(ui.Shift, "Chapter"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-0"}) {
		t.Fatalf("picked %v after picking back", got)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("Escape kept %v", state.pickedSheets())
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Sheet 4"); err != nil {
		t.Fatal(err)
	}
	if len(state.pickedSheets()) != 0 || !slices.Contains(library.calls, "open extra-3") {
		t.Fatalf("a plain click kept %v, calls %v", state.pickedSheets(), library.calls)
	}
}

func TestThePickedSheetsTakeABatchMenu(t *testing.T) {
	state, library, tester := pickingState(t, 3)
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if err := tester.RightClick("Sheet 1"); err != nil {
		t.Fatal(err)
	}
	menu := tester.Menu()
	for _, item := range []string{"Open 3 Sheets in Tabs", "Duplicate 3 Sheets", "Move To", "Move 3 Sheets to Trash"} {
		if !slices.Contains(menu, item) {
			t.Fatalf("batch menu %v lacks %q", menu, item)
		}
	}
	if err := tester.ChooseMenuItem("Move To", "Inbox"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	var moved []string
	for _, move := range library.moves {
		moved = append(moved, move.id)
	}
	if !slices.Equal(moved, []string{"7", "extra-0", "extra-1"}) {
		t.Fatalf("moved %v, want the picked in order", moved)
	}
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("the picked stayed after the move: %v", state.pickedSheets())
	}

	// A menu on a sheet not picked is its own, and lets the picked go.
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if err := tester.RightClick("Sheet 3"); err != nil {
		t.Fatal(err)
	}
	if menu := tester.Menu(); !slices.Contains(menu, "Move to Trash") {
		t.Fatalf("an unpicked sheet's menu %v", menu)
	}
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("the picked stayed for another sheet's menu: %v", state.pickedSheets())
	}
}

func TestAPressElsewhereLetsThePickedGo(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if len(state.pickedSheets()) != 3 {
		t.Fatalf("picked %v", state.pickedSheets())
	}
	// Another shift-click is the run's to change, not a press elsewhere.
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if len(state.pickedSheets()) != 2 {
		t.Fatalf("picked %v after narrowing the run", state.pickedSheets())
	}
	// The page, right of the list.
	tester.ClickAt(1100, 500)
	tester.Frame()
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("a press on the page kept %v", state.pickedSheets())
	}
}

func TestCmdClickPicksOrLetsGoOfOneSheet(t *testing.T) {
	state, library, tester := pickingState(t, 3)
	// The open sheet is the selection a Cmd-click adds to.
	if err := tester.ClickWith(ui.Cmd, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-1"}) {
		t.Fatalf("picked %v", got)
	}
	if err := tester.ClickWith(ui.Cmd, "Chapter"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"extra-1"}) {
		t.Fatalf("picked %v after letting the open sheet go", got)
	}
	settle(tester)
	if !tester.HasText("1 selected · 100 words") {
		t.Fatalf("the header does not count one picked: %v", tester.Texts())
	}
	// A shift-click runs from the last Cmd-clicked sheet, keeping what was
	// picked apart; another reshapes that run alone.
	if err := tester.ClickWith(ui.Shift, "Sheet 3"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-0", "extra-1", "extra-2"}) {
		t.Fatalf("picked %v after a shift-click", got)
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-0", "extra-1"}) {
		t.Fatalf("picked %v after reshaping the run", got)
	}
	for _, call := range library.calls {
		if call != "open 7" && len(call) > 4 && call[:4] == "open" {
			t.Fatalf("picking opened a sheet: %v", library.calls)
		}
	}
}

// frameEdges finds the picked rows' frames down the list's middle: the
// lines across the rows that the frames' dashes ink.
type frameEdges struct {
	t        *testing.T
	tester   *ui.Tester
	ink      ui.Color
	x0, x1   int
	from, to int
}

func newFrameEdges(t *testing.T, state *State, tester *ui.Tester, first, last string) frameEdges {
	semantic := state.Settings.Semantic()
	top, _ := tester.Find(first)
	bottom, _ := tester.Find(last)
	return frameEdges{
		t: t, tester: tester, ink: components.Blend(semantic.Surface.Panel, semantic.Accent.Foreground, 0.76),
		x0: int(top.X + top.W*0.3), x1: int(top.X + top.W*0.7),
		from: int(top.Y) - 8, to: int(bottom.Y+bottom.H) + 8,
	}
}

func (f frameEdges) inked(image *image.RGBA, x, y int) bool {
	c := image.RGBAAt(x, y)
	d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	return d(c.R, f.ink.R)+d(c.G, f.ink.G)+d(c.B, f.ink.B) < 60
}

// edges is the lines inked a good part of the way across, top down.
func (f frameEdges) edges() []int {
	var edges []int
	image := f.tester.Image()
	for y := f.from; y <= f.to; y++ {
		n := 0
		for x := f.x0; x < f.x1; x++ {
			if f.inked(image, x, y) {
				n++
			}
		}
		if n*10 < (f.x1-f.x0)*3 {
			continue
		}
		if len(edges) > 0 && y-edges[len(edges)-1] <= 2 {
			continue
		}
		edges = append(edges, y)
	}
	return edges
}

// outset is how far the frame stands out from the rows it holds.
const outset = 3

// near reports whether an edge lies within two pixels of y.
func near(edges []int, y float32) bool {
	return slices.ContainsFunc(edges, func(edge int) bool { return max(float32(edge)-y, y-float32(edge)) <= 2 })
}

// frames runs the next frames, calling check on each.
func (f frameEdges) frames(check func(frame int, edges []int)) {
	for frame := range 24 {
		f.tester.Advance(16 * time.Millisecond)
		f.tester.Frame()
		check(frame, f.edges())
	}
}

// Sheets picked side by side share one frame round them all.
func TestPickedNeighboursShareOneFrame(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	frame := newFrameEdges(t, state, tester, "Chapter", "Sheet 3")
	first, _ := tester.Find("Chapter")
	last, _ := tester.Find("Sheet 1")
	edges := frame.edges()
	if len(edges) != 2 || !near(edges, first.Y-outset) || !near(edges, last.Y+last.H+outset) {
		t.Fatalf("frame edges %v, want one frame from %v to %v, clear of the rows", edges, first.Y, last.Y+last.H)
	}
}

// Picking leaves the rows' own look alone: the open sheet keeps its fill.
func TestPickingLeavesTheRowsOwnStyle(t *testing.T) {
	_, _, tester := pickingState(t, 2)
	open, _ := tester.Find("Chapter")
	x, y := int(open.X+open.W-12), int(open.Y+open.H/2)
	before := tester.Image().RGBAAt(x, y)
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := tester.Image().RGBAAt(x, y); got != before {
		t.Fatalf("picking changed the open sheet's fill from %v to %v", before, got)
	}
}

// A run that grows is one frame whose edge glides over the rows it takes.
func TestARunGrowsAsOneFrame(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	frame := newFrameEdges(t, state, tester, "Chapter", "Sheet 3")
	first, _ := tester.Find("Chapter")
	last, _ := tester.Find("Sheet 3")
	if err := tester.ClickWith(ui.Shift, "Sheet 3"); err != nil {
		t.Fatal(err)
	}
	bottoms := map[int]bool{}
	previous := 0
	frame.frames(func(n int, edges []int) {
		if len(edges) != 2 || !near(edges, first.Y-outset) {
			t.Fatalf("frame %d: edges %v, want one frame from the top", n, edges)
		}
		if edges[1] < previous {
			t.Fatalf("frame %d: the bottom went back from %d to %d", n, previous, edges[1])
		}
		previous = edges[1]
		bottoms[previous] = true
	})
	if !near([]int{previous}, last.Y+last.H+outset) {
		t.Fatalf("the frame ended at %d, not Sheet 3's bottom %v", previous, last.Y+last.H)
	}
	if len(bottoms) < 4 {
		t.Fatalf("the bottom jumped rather than glided: %v", bottoms)
	}
}

// Letting go of a sheet inside a run parts its frame there, the outer
// edges staying put; picking it again joins the two.
func TestARunPartsAndJoinsWhereASheetIsCmdClicked(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	frame := newFrameEdges(t, state, tester, "Chapter", "Sheet 2")
	first, _ := tester.Find("Chapter")
	last, _ := tester.Find("Sheet 2")
	for _, step := range []struct {
		name  string
		edges int
	}{{"parts", 4}, {"joins", 2}} {
		if err := tester.ClickWith(ui.Cmd, "Sheet 1"); err != nil {
			t.Fatal(err)
		}
		frame.frames(func(n int, edges []int) {
			if !near(edges, first.Y-outset) || !near(edges, last.Y+last.H+outset) {
				t.Fatalf("%s, frame %d: the outer edges moved: %v", step.name, n, edges)
			}
		})
		settle(tester)
		if edges := frame.edges(); len(edges) != step.edges {
			t.Fatalf("%s: edges %v, want %d", step.name, edges, step.edges)
		}
	}
}
