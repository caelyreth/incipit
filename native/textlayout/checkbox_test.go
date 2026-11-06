package textlayout

import (
	"fmt"
	"math"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestCheckboxMarkerStandsOnTextBaseline(t *testing.T) {
	if err := theme.RegisterFonts(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, prefix, label string
		width, scale        float32
	}{
		{name: "empty", width: 300},
		{name: "Latin", label: " Task abc", width: 300},
		{name: "CJK", label: " 中文文字", width: 300},
		{name: "nested and wrapped", prefix: "before🦀\n        ", label: " mixed 文字", width: 40},
		{name: "larger text", label: " Larger 文字", width: 300, scale: 1.6},
	} {
		for _, size := range []float32{15, 17, 26} {
			for _, spacing := range []float32{1.5, 1.8, 2.2} {
				t.Run(fmt.Sprintf("%s/size=%g/spacing=%g", test.name, size, spacing), func(t *testing.T) {
					text := test.prefix + "- [ ]" + test.label
					markerByte := len(test.prefix)
					markerStart := model.UTF16Offset(model.StringLenUTF16(test.prefix))
					checkbox := surface.Checkbox{Node: 2, Marker: model.NewUTF16Range(markerStart, markerStart+5)}
					runs := []surface.StyleRun{{Bytes: model.NewByteRange(0, model.ByteOffset(markerByte+5))}}
					if test.label != "" {
						runs = append(runs, surface.StyleRun{
							Bytes: model.NewByteRange(model.ByteOffset(markerByte+5), model.ByteOffset(len(text))),
							Style: surface.TextStyle{Scale: test.scale},
						})
					}
					flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, runs, lineBlocks(text))
					if err != nil {
						t.Fatal(err)
					}
					flow, err = ownedBy(flow, asObjects([]surface.Checkbox{checkbox}))
					if err != nil {
						t.Fatal(err)
					}
					style := testStyle()
					style.Prose = theme.NewProseStyle(size).WithSpacing(spacing)
					layout := NewCache().Update(flow, nil, test.width, style)
					paragraph, local := layout.paragraphAt(checkbox.Marker.Start)
					if paragraph == nil {
						t.Fatal("checkbox paragraph missing")
					}
					local = paragraph.shapedOffset(local, false)
					objects := paragraph.layout.Objects()
					if len(objects) != 1 || objects[0].Index != local {
						t.Fatalf("objects = %+v, marker index = %d", objects, local)
					}
					marker, ok := layout.CheckboxMarker(checkbox)
					if !ok {
						t.Fatal("checkbox marker missing")
					}
					object := objects[0].Rect
					object.X += paragraph.left
					object.Y += paragraph.top
					if marker != object {
						t.Fatalf("marker = %+v, shaped object = %+v", marker, object)
					}
					near := func(a, b float32) bool { return math.Abs(float64(a-b)) < 0.001 }
					if !near(marker.W, size*0.95) || !near(marker.H, size*0.78) {
						t.Fatalf("marker = %+v at font size %g", marker, size)
					}
					lines := paragraph.layout.Lines()
					found := false
					for _, line := range lines {
						if line.Start <= local && local < line.End {
							found = true
							if !near(marker.Y+marker.H, paragraph.top+line.Baseline) {
								t.Fatalf("marker bottom %g, text baseline %g", marker.Y+marker.H, paragraph.top+line.Baseline)
							}
						}
					}
					if !found {
						t.Fatalf("marker index %d not in lines %+v", local, lines)
					}

					// Giving the object a height must not change the text's rows,
					// caret, selection, or wrapping.
					spans := spansFor(flow, nil, paragraph.startByte, paragraph.endByte, style, test.width)
					for i := range spans {
						spans[i].ObjectHeight = 0
					}
					rhythm := flow.Blocks()[paragraph.block].Rhythm
					font := style.paragraphFont(flow, paragraph.startByte, paragraph.endByte, style.font(), leading(rhythm, style.Prose.Leading()))
					withoutHeight := ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: test.width})
					otherLines := withoutHeight.Lines()
					if len(lines) != len(otherLines) {
						t.Fatalf("line counts with and without object height: %d, %d", len(lines), len(otherLines))
					}
					for i, line := range lines {
						other := otherLines[i]
						if line.Start != other.Start || line.End != other.End || !near(line.Rect.H, other.Rect.H) || !near(line.Baseline, other.Baseline) {
							t.Fatalf("line %d changed: %+v, without height %+v", i, line, other)
						}
					}
					for _, at := range []int{local, local + 1} {
						if got, want := paragraph.layout.Caret(at), withoutHeight.Caret(at); got != want {
							t.Fatalf("caret %d changed: %+v, without height %+v", at, got, want)
						}
					}
					selection := ui.TextInputRange{Start: local, End: local + 1}
					got, want := paragraph.layout.SelectionRects(selection), withoutHeight.SelectionRects(selection)
					if len(got) != len(want) || len(got) != 1 || got[0] != want[0] {
						t.Fatalf("marker selection changed: %+v, without height %+v", got, want)
					}
				})
			}
		}
	}
}

func TestCheckboxMarkerReservesOneObjectAndKeepsTextOffsets(t *testing.T) {
	flow, err := ownedBy(testFlow(t, "  - [x] 你好🦀", 0), asObjects([]surface.Checkbox{{
		Node: 2, Marker: model.NewUTF16Range(2, 7), Checked: true,
	}}))
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, nil, 300, testStyle())
	objects := layout.Paragraphs()[0].Layout().Objects()
	if len(objects) != 1 || objects[0].Index != 2 || objects[0].Rect.W <= 0 {
		t.Fatalf("objects = %+v", objects)
	}
	box, ok := layout.CheckboxMarker(surface.ObjectsOf[surface.Checkbox](flow)[0])
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
	flow, err := ownedBy(testFlow(t, "🦀\n  - [ ] a🦀z\nnext", 0), asObjects([]surface.Checkbox{{
		Node: 2, Marker: model.NewUTF16Range(5, 10),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, nil, 300, testStyle())
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
	checkbox, err := ownedBy(literal, asObjects([]surface.Checkbox{{Node: 2, Marker: model.NewUTF16Range(0, 5)}}))
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache()
	first := cache.Update(literal, nil, 300, testStyle())
	second := cache.Update(checkbox, nil, 300, testStyle())
	if first.Paragraphs()[0].Layout() == second.Paragraphs()[0].Layout() {
		t.Fatal("literal marker reused a checkbox layout")
	}
}

func TestCheckboxMarkerFollowsItsWrappedLine(t *testing.T) {
	flow, err := ownedBy(testFlow(t, "        - [ ] task", 0), asObjects([]surface.Checkbox{{
		Node: 2, Marker: model.NewUTF16Range(8, 13),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, nil, 40, testStyle())
	paragraph := layout.Paragraphs()[0]
	objects := paragraph.Layout().Objects()
	if len(objects) != 1 {
		t.Fatalf("objects = %+v", objects)
	}
	box, ok := layout.CheckboxMarker(surface.ObjectsOf[surface.Checkbox](flow)[0])
	if !ok || objects[0].Rect.Y < box.Y || objects[0].Rect.Y > box.Y+box.H {
		t.Fatalf("wrapped marker box %+v does not contain object baseline %+v", box, objects[0].Rect)
	}
}
