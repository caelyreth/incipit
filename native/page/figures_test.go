package page

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// figurePage is "above", a figure of a 400×300 picture, and "below".
func figurePage(t *testing.T) (*Element, *unitLayout) {
	t.Helper()
	el := newElement(t)
	typeText(el, "above")
	el.Editor.Do(profile.PlaceFigures{Pictures: []profile.Picture{{Media: "m", Width: 400, Height: 300}}}, time.Unix(200, 0))
	el.Editor.Do(vocab.Dismiss{}, time.Unix(201, 0))
	el.Editor.Do(vocab.Break{}, time.Unix(202, 0))
	typeText(el, "below")
	pl := el.relayout()
	if len(pl.units) != 3 || pl.units[1].figure == nil {
		t.Fatalf("units %d, want a figure between two paragraphs", len(pl.units))
	}
	return el, pl.units[1]
}

// A figure keeps its picture's room from its recorded size, centered. An
// empty caption takes no room until the figure is selected; one with text
// keeps its line.
func TestAFigureLaysOutFromItsShape(t *testing.T) {
	el, u := figurePage(t)
	if u.box.W != 400 || u.box.H != 300 {
		t.Fatalf("box %v, want 400×300", u.box)
	}
	width := el.Editor.metrics.Width - u.x
	if d := u.box.X - (width-400)/2; d > 0.5 || d < -0.5 {
		t.Fatalf("box at %v, want centered in %v", u.box.X, width)
	}
	if u.h != u.box.H {
		t.Fatalf("closed caption: height %v, picture %v", u.h, u.box.H)
	}
	el.Editor.Do(vocab.Place{Node: u.pres.Unit}, time.Unix(300, 0))
	el.relayout()
	open := el.Editor.layout.units[1].h
	if open <= u.box.H {
		t.Fatalf("selected: height %v", open)
	}
	el.Editor.Do(vocab.Dismiss{}, time.Unix(301, 0))
	typeText(el, "Dawn")
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(302, 0))
	el.relayout()
	if u = el.Editor.layout.units[1]; u.h != open {
		t.Fatalf("captioned: height %v, open %v", u.h, open)
	}
}

// A click on the picture selects the figure, which opens its caption's
// line; a click there is in the caption.
func TestClicksSelectAFigureOrEnterItsCaption(t *testing.T) {
	el, u := figurePage(t)
	el.Editor.Do(vocab.Place{Node: u.pres.Unit}, time.Unix(300, 0))
	pl := el.relayout()
	u = pl.units[1]
	_, node, ok := pl.hit(u.x+u.box.X+u.box.W/2, u.y+u.box.H/2)
	if !ok || node != u.pres.Unit {
		t.Fatalf("hit on the picture: node %v", node)
	}
	c, node, ok := pl.hit(u.x+u.box.X+u.box.W/2, u.y+u.box.H+captionGap(el.Editor.metrics)+2)
	if !ok || node != 0 || c.Pos.Node != u.pres.Unit {
		t.Fatalf("hit under the picture: %v node %v", c, node)
	}
}

// Down passes through a figure row by row: its gap before, its caption
// when it has one, its gap after, then the text below; Up retraces it.
func TestVerticalMotionPassesThroughAFigure(t *testing.T) {
	for _, caption := range []string{"", "Dawn"} {
		el, u := figurePage(t)
		ed := el.Editor
		if caption != "" {
			ed.Do(vocab.Place{Node: u.pres.Unit}, time.Unix(300, 0))
			ed.Do(vocab.Dismiss{}, time.Unix(301, 0))
			typeText(el, caption)
		}
		ed.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(400, 0))
		el.relayout()
		var down []doc.Caret
		for i := range 5 {
			in, ok := ed.resolve(vocab.VisualMove{Dir: vocab.Down}, 600)
			if !ok || in.Target == (doc.Caret{}) {
				break
			}
			ed.Do(in, time.Unix(int64(401+i), 0))
			el.relayout()
			down = append(down, ed.Selection().Head)
		}
		var path []string
		for _, c := range down {
			n, _ := ed.Engine.Snapshot().Node(c.Pos.Node)
			path = append(path, profile.Schema().KindName(n.Kind()))
		}
		want := []string{"page", "page", "paragraph"}
		if caption != "" {
			want = []string{"page", "figure", "page", "paragraph"}
		}
		if fmt.Sprint(path) != fmt.Sprint(want) {
			t.Fatalf("caption %q: down %v, want %v", caption, path, want)
		}
		for i := len(down) - 2; i >= 0; i-- {
			in, _ := ed.resolve(vocab.VisualMove{Dir: vocab.Up}, 600)
			ed.Do(in, time.Unix(int64(500+i), 0))
			el.relayout()
			if got := ed.Selection().Head; got.Pos.Node != down[i].Pos.Node || got.Pos.Slot != down[i].Pos.Slot {
				t.Fatalf("caption %q: up to %v, want %v", caption, got, down[i])
			}
		}
	}
}

// A resize grows a centered figure from both sides and commits one span.
func TestAResizeCommitsOneSpan(t *testing.T) {
	el, u := figurePage(t)
	el.Editor.Do(vocab.Place{Node: u.pres.Unit}, time.Unix(300, 0))
	el.focused = true
	el.relayout()
	u = el.Editor.layout.units[1]
	grip, ok := gripRect(u, 1)
	if !ok {
		t.Fatal("no grip")
	}
	if !el.beginResize(grip.X+grip.W/2, grip.Y+grip.H/2) {
		t.Fatal("the grip did not start a resize")
	}
	r := el.Editor.resize
	r.span = r.spanAt(grip.X + grip.W/2 + 50)
	el.finishResize(false, time.Unix(301, 0))
	n, _ := el.Editor.Engine.Snapshot().Node(u.pres.Unit)
	width := el.Editor.metrics.Width - u.x
	want := int((400 + 100) / width * profile.SpanFull)
	if got := profile.ShapeOf(n).Span; got < want-2 || got > want+2 {
		t.Fatalf("span %d, want about %d", got, want)
	}
}

// Pictures are sized to read well: small ones at their own pixels, wide
// ones to the column, tall ones narrowed to a bound, long ones cropped
// under it, and a writer's span kept.
func TestFigureSizesReadWell(t *testing.T) {
	m := Metrics{Body: 18, Spacing: 1.6}
	const col = 640
	tall := float32(col * figureTall) // 480, over 14 lines of 28.8
	pic := func(w, h, span int) profile.Shape {
		return profile.Shape{Picture: profile.Picture{Width: w, Height: h}, Span: span}
	}
	near := func(a, b float32) bool { return a-b < 0.5 && b-a < 0.5 }
	cases := []struct {
		name        string
		shape       profile.Shape
		w, h, shown float32
	}{
		{"an icon stays its size", pic(16, 16, 0), 16, 16, 1},
		{"a small picture is not scaled up", pic(200, 150, 0), 200, 150, 1},
		{"a wide photo fits the column", pic(4000, 3000, 0), col, 480, 1},
		{"a panorama fits the column", pic(6000, 1000, 0), col, col / 6.0, 1},
		{"a tall photo narrows to the bound", pic(3000, 4000, 0), 360, tall, 1},
		{"a phone screenshot narrows, then crops", pic(1170, 2532, 0), col * figureNarrow, tall, tall / (col * figureNarrow * 2532 / 1170)},
		{"a long page is cropped at its top", pic(800, 8000, 0), col * figureNarrow, tall, tall / (col * figureNarrow * 10)},
		{"a span is kept", pic(400, 300, 500), 320, 240, 1},
		{"a span on a tall picture crops", pic(1000, 2000, 1000), col, tall, tall / (col * 2)},
		{"an unknown size takes the column", pic(0, 0, 0), col, col * 9 / 16.0, 1},
	}
	for _, c := range cases {
		got := figureSize(c.shape, col, m)
		if !near(got.w, c.w) || !near(got.h, c.h) || got.shown-c.shown > 0.01 || c.shown-got.shown > 0.01 {
			t.Errorf("%s: got %.1f×%.1f showing %.2f, want %.1f×%.1f showing %.2f", c.name, got.w, got.h, got.shown, c.w, c.h, c.shown)
		}
	}
}
