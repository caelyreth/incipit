package textlayout

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

func TestCheckboxMarkerReservesOneObjectAndKeepsTextOffsets(t *testing.T) {
	flow, err := testFlow(t, "  - [x] 你好🦀", 0).WithCheckboxes([]surface.Checkbox{{
		Node: 2, Marker: model.NewUTF16Range(2, 7), Checked: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, 300, testStyle())
	objects := layout.Paragraphs()[0].Layout().Objects()
	if len(objects) != 1 || objects[0].Index != 2 || objects[0].Rect.W <= 0 {
		t.Fatalf("objects = %+v", objects)
	}
	box, ok := layout.CheckboxMarker(flow.Checkboxes()[0])
	if !ok || box.W <= 0 || box.H <= 0 {
		t.Fatalf("marker box = %+v, %v", box, ok)
	}
	for _, at := range []model.UTF16Offset{8, 9, 10, 12} {
		if caret := layout.Caret(at); caret.H <= 0 || caret.X < box.X+box.W-1 {
			t.Fatalf("caret %d = %+v, marker %+v", at, caret, box)
		}
	}
}

func TestCheckboxCoordinatesRoundTripAcrossUnicodeAndLines(t *testing.T) {
	flow, err := testFlow(t, "🦀\n  - [ ] a🦀z\nnext", 0).WithCheckboxes([]surface.Checkbox{{
		Node: 2, Marker: model.NewUTF16Range(5, 10),
	}})
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, 300, testStyle())
	for _, test := range []struct {
		at, previous, next model.UTF16Offset
	}{
		{5, 4, 10}, {10, 5, 11}, {11, 10, 12}, {12, 11, 14}, {14, 12, 15},
	} {
		if got := layout.PreviousBoundary(test.at); got != test.previous {
			t.Errorf("PreviousBoundary(%d) = %d, want %d", test.at, got, test.previous)
		}
		if got := layout.NextBoundary(test.at); got != test.next {
			t.Errorf("NextBoundary(%d) = %d, want %d", test.at, got, test.next)
		}
	}
	caret := layout.Caret(12)
	if got := layout.Hit(ui.Point{X: caret.X, Y: caret.Y + caret.H/2}); got != 12 {
		t.Fatalf("hit at Unicode caret = %d, want 12", got)
	}
	if got := layout.RowNear(0, 1, caret.X); got != 12 {
		t.Fatalf("moving down to checkbox text = %d, want 12", got)
	}
	whole := layout.RangeRects(model.NewUTF16Range(5, 10))
	partial := layout.RangeRects(model.NewUTF16Range(6, 8))
	if len(whole) != 1 || len(partial) != 1 || whole[0] != partial[0] {
		t.Fatalf("marker selection: whole=%+v partial=%+v", whole, partial)
	}
}

func TestCheckboxAndLiteralTextDoNotShareShapedLayout(t *testing.T) {
	literal := testFlow(t, "- [ ] task", 0)
	checkbox, err := literal.WithCheckboxes([]surface.Checkbox{{Node: 2, Marker: model.NewUTF16Range(0, 5)}})
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache()
	first := cache.Update(literal, 300, testStyle())
	second := cache.Update(checkbox, 300, testStyle())
	if first.Paragraphs()[0].Layout() == second.Paragraphs()[0].Layout() {
		t.Fatal("literal marker reused a checkbox layout")
	}
}

func TestCheckboxMarkerFollowsItsWrappedLine(t *testing.T) {
	flow, err := testFlow(t, "        - [ ] task", 0).WithCheckboxes([]surface.Checkbox{{
		Node: 2, Marker: model.NewUTF16Range(8, 13),
	}})
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, 40, testStyle())
	paragraph := layout.Paragraphs()[0]
	objects := paragraph.Layout().Objects()
	if len(objects) != 1 {
		t.Fatalf("objects = %+v", objects)
	}
	box, ok := layout.CheckboxMarker(flow.Checkboxes()[0])
	if !ok || objects[0].Rect.Y < box.Y || objects[0].Rect.Y > box.Y+box.H {
		t.Fatalf("wrapped marker box %+v does not contain object baseline %+v", box, objects[0].Rect)
	}
}
