package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func sheetMarkerBounds(test *testing.T, tester *ui.Tester, row ui.Rect, scale float32, ink ui.Color) ui.Rect {
	test.Helper()
	marker, found := findSheetMarker(tester, row, scale, ink)
	if !found {
		test.Fatal("the selected sheet's accent strip disappeared")
	}
	return marker
}

func findSheetMarker(tester *ui.Tester, row ui.Rect, scale float32, ink ui.Color) (ui.Rect, bool) {
	image := tester.Image()
	left, top := image.Bounds().Max.X, image.Bounds().Max.Y
	right, bottom := 0, 0
	for pixelY := int(row.Y * scale); pixelY < int((row.Y+row.H)*scale); pixelY++ {
		for pixelX := int(row.X * scale); pixelX < int((row.X+8)*scale); pixelX++ {
			pixel := image.RGBAAt(pixelX, pixelY)
			if delta := max(int(pixel.R)-int(ink.R), int(ink.R)-int(pixel.R),
				int(pixel.G)-int(ink.G), int(ink.G)-int(pixel.G),
				int(pixel.B)-int(ink.B), int(ink.B)-int(pixel.B)); delta <= 8 {
				left, top = min(left, pixelX), min(top, pixelY)
				right, bottom = max(right, pixelX+1), max(bottom, pixelY+1)
			}
		}
	}
	if right <= left || bottom <= top {
		return ui.Rect{}, false
	}
	return ui.Rect{X: float32(left) / scale, Y: float32(top) / scale, W: float32(right-left) / scale, H: float32(bottom-top) / scale}, true
}

func TestSheetMarkerLengthFollowsPressAndRelease(test *testing.T) {
	for _, testCase := range []struct {
		name    string
		excerpt string
		scale   float32
		reduced bool
	}{
		{"short", "", 1, false},
		{"excerpt", "A sheet with a longer excerpt that wraps onto a second line in the sheet list.", 2, false},
		{"reduced", "", 1, true},
		{"reduced retina", "A sheet with an excerpt.", 2, true},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			state, library := libraryState()
			state.ActivePage = "chosen"
			library.entries = []GroupEntry{{Kind: EntrySheet, ID: "chosen",
				Sheet: SheetItem{ID: "chosen", Title: "Chosen sheet", Excerpt: testCase.excerpt}}}
			state.Go(Place{Kind: PlaceGroup, Group: "2"})
			tester := ui.NewTesterWith(state.View, 1260, 850, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: testCase.reduced}})
			tester.SetScale(testCase.scale)
			settle(tester)
			row := selectionRow(test, tester, "Chosen sheet")
			marker := func() ui.Rect {
				return sheetMarkerBounds(test, tester, row, testCase.scale, state.Settings.Semantic().Accent.Foreground)
			}
			rest := marker()
			pointerX, pointerY := row.X+row.W/2, row.Y+row.H/2
			tester.Press(pointerX, pointerY)
			initial := marker()
			tester.Advance(32 * time.Millisecond)
			growing := marker()
			tester.Advance(300 * time.Millisecond)
			pressed := marker()
			if pressed.H < rest.H*1.4 {
				test.Fatalf("press did not lengthen the strip: %v to %v", rest, pressed)
			}
			if center := pressed.Y + pressed.H/2; center < pointerY-1 || center > pointerY+1 {
				test.Fatalf("the pressed strip moved off center: %v in %v", pressed, row)
			}
			if !testCase.reduced && (initial.H != rest.H || growing.H <= rest.H || growing.H >= pressed.H) {
				test.Fatalf("press snapped instead of growing: rest %v, initial %v, growing %v, pressed %v", rest, initial, growing, pressed)
			}
			tester.Release(pointerX, pointerY)
			initial = marker()
			tester.Advance(32 * time.Millisecond)
			shrinking := marker()
			tester.Advance(300 * time.Millisecond)
			if released := marker(); released != rest {
				test.Fatalf("release did not restore the strip: %v, want %v", released, rest)
			}
			if !testCase.reduced && (initial != pressed || shrinking.H <= rest.H || shrinking.H >= pressed.H) {
				test.Fatalf("release snapped instead of shrinking: pressed %v, initial %v, shrinking %v, rest %v", pressed, initial, shrinking, rest)
			}
			if testCase.reduced {
				if growing != pressed || initial != rest {
					test.Fatal("reduced motion did not update the strip immediately")
				}
				return
			}
			tester.Press(pointerX, pointerY)
			tester.Advance(32 * time.Millisecond)
			tester.Frame()
			before := marker()
			tester.Release(pointerX, pointerY)
			if after := marker(); after != before {
				test.Fatalf("an interrupted press jumped on release: %v to %v", before, after)
			}
			tester.Advance(32 * time.Millisecond)
			tester.Frame()
			before = marker()
			tester.Press(pointerX, pointerY)
			if after := marker(); after != before {
				test.Fatalf("an interrupted release jumped on press: %v to %v", before, after)
			}
			tester.Advance(300 * time.Millisecond)
			tester.Release(row.X+row.W+20, pointerY)
			tester.Advance(300 * time.Millisecond)
			if canceled := marker(); canceled != rest {
				test.Fatalf("release outside the row did not restore the strip: %v, want %v", canceled, rest)
			}
		})
	}
}

func TestSheetMarkerStaysLongDuringDrag(test *testing.T) {
	for _, testCase := range []struct {
		name    string
		scale   float32
		reduced bool
	}{
		{"normal", 1, false},
		{"retina", 2, false},
		{"reduced", 1, true},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			state, _ := libraryState()
			state.ActivePage = "7"
			state.Go(Place{Kind: PlaceGroup, Group: "2"})
			tester := ui.NewTesterWith(state.View, 1260, 850, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: testCase.reduced}})
			tester.SetScale(testCase.scale)
			settle(tester)
			row := selectionRow(test, tester, "Chapter")
			semantic := state.Settings.Semantic()
			marker := func() ui.Rect {
				return sheetMarkerBounds(test, tester, row, testCase.scale, semantic.Accent.Foreground)
			}
			rest := marker()
			pointerX, pointerY := row.X+row.W/2, row.Y+row.H/2
			tester.Press(pointerX, pointerY)
			tester.Advance(300 * time.Millisecond)
			pressed := marker()
			tester.Move(pointerX+24, pointerY+10)
			tester.Frame()
			if !state.nav.dragged {
				test.Fatal("the pointer did not start a sheet drag")
			}
			for range 20 {
				tester.Advance(16 * time.Millisecond)
				if dragged := marker(); dragged.H < pressed.H-1 {
					test.Fatalf("drag shortened the strip: %v, want at least %v", dragged, pressed)
				}
			}
			preview := ui.Rect{X: row.X + 24, Y: row.Y + 10, W: row.W, H: row.H}
			previewInk := semantic.Accent.Foreground.Alpha(0.75).Over(semantic.Surface.Chosen)
			if strip, found := findSheetMarker(tester, preview, testCase.scale, previewInk); found {
				test.Fatalf("the dragged copy should have no accent strip, but shows %v", strip)
			}
			tester.Release(pointerX+24, pointerY+10)
			if !testCase.reduced && marker().H < pressed.H-1 {
				test.Fatal("the strip snapped short at the end of the drag")
			}
			tester.Advance(32 * time.Millisecond)
			if !testCase.reduced {
				if shrinking := marker(); shrinking.H <= rest.H || shrinking.H >= pressed.H {
					test.Fatalf("the strip did not shrink smoothly after the drag: %v", shrinking)
				}
			}
			settle(tester)
			if released := marker(); released != rest {
				test.Fatalf("the strip did not return after the drag: %v, want %v", released, rest)
			}
		})
	}
}

func TestSheetMarkerVisitsPressedRowsAndReturns(test *testing.T) {
	for _, testCase := range []struct {
		name     string
		selected bool
		scale    float32
		reduced  bool
		chapter  bool
	}{
		{"selected", true, 1, false, false},
		{"selected retina", true, 2, false, false},
		{"nothing selected", false, 1, false, false},
		{"nothing selected retina", false, 2, false, false},
		{"reduced motion", true, 1, true, false},
		{"nothing selected reduced", false, 1, true, false},
		{"chapter", true, 1, false, true},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			state, library := libraryState()
			library.extraSheets = 2
			if testCase.selected {
				state.ActivePage = "7"
			}
			targetLabel := "Sheet 2"
			if testCase.chapter {
				library.entries = []GroupEntry{
					{Kind: EntrySheet, ID: "7", Sheet: SheetItem{ID: "7", Title: "Chapter", Excerpt: "It begins"}},
					{Kind: EntrySheet, ID: "8", Sheet: SheetItem{ID: "8", Title: "Middle"}},
					{Kind: EntryChapter, ID: "chapter:9", Chapter: ChapterItem{ID: "chapter:9", Name: "Part Two"}},
				}
				targetLabel = "Part Two"
			}
			state.Go(Place{Kind: PlaceGroup, Group: "2"})
			tester := ui.NewTesterWith(state.View, 1260, 850, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: testCase.reduced}})
			tester.SetScale(testCase.scale)
			settle(tester)
			selected := selectionRow(test, tester, "Chapter")
			target := selectionRow(test, tester, targetLabel)
			list := ui.Rect{X: selected.X, Y: selected.Y, W: selected.W, H: target.Y + target.H - selected.Y}
			ink := state.Settings.Semantic().Accent.Foreground
			marker := func() (ui.Rect, bool) { return findSheetMarker(tester, list, testCase.scale, ink) }
			rest, shown := marker()
			if shown != testCase.selected {
				test.Fatalf("resting strip visible %v, selected %v", shown, testCase.selected)
			}
			fillX := int((selected.X + selected.W/2) * testCase.scale)
			fillY := int((selected.Y + selected.H - 3) * testCase.scale)
			fill := tester.Image().RGBAAt(fillX, fillY)
			pointerX, pointerY := target.X+target.W/2, target.Y+target.H/2
			tester.Press(pointerX, pointerY)
			if initial, visible := marker(); !testCase.reduced && (initial != rest || visible != shown) {
				test.Fatalf("press jumped the strip: %v (%v) to %v (%v)", rest, shown, initial, visible)
			}
			if testCase.selected && !testCase.reduced {
				tester.Advance(48 * time.Millisecond)
				moving, visible := marker()
				if !visible || moving.Y <= rest.Y || moving.Y >= target.Y+target.H/2 {
					test.Fatalf("the strip did not glide to the pressed row: %v to %v toward %v", rest, moving, target)
				}
			}
			settle(tester)
			pressed, visible := marker()
			if !visible || pressed.Y < target.Y || pressed.Y+pressed.H > target.Y+target.H+1 || pressed.H < target.H*0.65 {
				test.Fatalf("the pressed row did not get the single extended strip: %v in %v", pressed, target)
			}
			if testCase.selected && tester.Image().RGBAAt(fillX, fillY) != fill {
				test.Fatal("the selected background moved with the strip")
			}
			tester.Move(pointerX+24, pointerY+10)
			tester.Frame()
			settle(tester)
			if !state.nav.dragged {
				test.Fatal("the pointer did not start a drag")
			}
			if dragged, visible := marker(); !visible || dragged != pressed {
				test.Fatalf("drag moved or shortened the source strip: %v to %v", pressed, dragged)
			}
			tester.Release(pointerX+24, pointerY+10)
			if initial, visible := marker(); !testCase.reduced && (!visible || initial != pressed) {
				test.Fatalf("release jumped the strip: %v to %v", pressed, initial)
			}
			tester.Advance(48 * time.Millisecond)
			if testCase.selected && !testCase.reduced {
				returning, visible := marker()
				if !visible || returning.Y <= rest.Y || returning.Y >= pressed.Y {
					test.Fatalf("the strip did not glide back to the selection: %v toward %v", returning, rest)
				}
			}
			probeX := int((target.X + 1) * testCase.scale)
			probeY := int((target.Y + target.H/2) * testCase.scale)
			fading := tester.Image().RGBAAt(probeX, probeY)
			settle(tester)
			if released, visible := marker(); visible != shown || released != rest {
				test.Fatalf("the strip did not return to its resting state: %v (%v), want %v (%v)", released, visible, rest, shown)
			}
			if !testCase.selected && !testCase.reduced && fading == tester.Image().RGBAAt(probeX, probeY) {
				test.Fatal("the unselected strip disappeared without fading")
			}
			if len(library.moves) != 0 {
				test.Fatalf("dragging within the source row reordered entries: %v", library.moves)
			}
		})
	}
}
