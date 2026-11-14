package page

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A paragraph made a heading grows to the heading's size through the sizes
// between, its marker with it, and the text after it moves with it; a
// heading made a paragraph again shrinks back the same way.
func TestAHeadingGrowsToItsSize(t *testing.T) {
	el := newElement(t)
	typeText(el, "#")
	tt := ui.NewTester(el.View, 900, 600)
	tt.Advance(time.Second)
	unit := func() *unitLayout { return el.Editor.layout.units[0] }
	small := unit().h
	typeText(el, " Low tide")
	tt.Frame()
	var between int
	for range 60 {
		tt.Advance(16 * time.Millisecond)
		if h := unit().h; h > small && h < lineHeight(el.Editor.metrics, el.Editor.metrics.Body*unit().size) {
			between++
		}
	}
	u := unit()
	if between < 3 || u.size <= 1 || u.key.in.size != 0 {
		t.Fatalf("the heading went %v → %v through %d sizes between, asked %v", small, u.h, between, u.key.in.size)
	}
	if el.springs.m != nil && len(el.sizing) != 0 {
		t.Fatalf("still sizing %v after it arrived", el.sizing)
	}
}
