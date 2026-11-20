package manifest

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

var tide = profile.Picture{Media: "tide", Width: 800, Height: 600}

func (p *page) placeFigure() {
	p.t.Helper()
	p.do(profile.PlaceFigures{Pictures: []profile.Picture{tide}})
}

func (p *page) selection() doc.Selection {
	v, _ := p.e.View(1)
	return v.Selection
}

func (p *page) figure() doc.ID {
	p.t.Helper()
	var id doc.ID
	for _, n := range p.e.Snapshot().Root().Slot(0).Children() {
		if n.Kind() == profile.Figure {
			id = n.ID()
		}
	}
	if id == 0 {
		p.t.Fatal("no figure")
	}
	return id
}

func (p *page) wantSelected(id doc.ID) {
	p.t.Helper()
	if s := p.selection(); s.Kind != doc.SelectNode || s.Node != id {
		p.t.Fatalf("selection %+v, want the figure selected", s)
	}
}

func (p *page) wantInCaption() {
	p.t.Helper()
	if s := p.selection(); s.Kind != doc.SelectText || s.Head.Pos.Node != p.figure() {
		p.t.Fatalf("selection %+v, want the caret in the caption", s)
	}
}

func TestAPictureReplacesAnEmptyParagraphAndTakesTheCaret(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.want("page(figure())")
	p.wantInCaption()
	p.typeText("Dawn")
	p.want("page(figure(Dawn))")
	if s := profile.ShapeOf(p.e.Snapshot().Root().Slot(0).Child(0)); s.Picture != tide || s.Align != profile.AlignCenter {
		t.Fatalf("shape %+v", s)
	}
}

func TestAPictureSplitsTheTextItLandsIn(t *testing.T) {
	p := open(t)
	p.typeText("ab")
	p.do(left)
	p.placeFigure()
	p.want("page(paragraph(a) figure() paragraph(b))")
	p.do(vocab.Undo{})
	p.want("page(paragraph(ab))")
}

// Return in a caption breaks beside the figure, as in any object's own
// slot that keeps no lines: a paragraph opens after it, the caption whole.
func TestReturnInACaptionContinuesBelow(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.typeText("Dawn light")
	for range 6 {
		p.do(left)
	}
	p.do(vocab.Break{})
	p.typeText("x")
	p.want("page(figure(Dawn light) paragraph(x))")
}

func TestEscapeMovesBetweenAFigureAndItsCaption(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.typeText("Dawn")
	id := p.figure()
	p.do(vocab.Dismiss{})
	p.wantSelected(id)
	p.do(vocab.Dismiss{})
	p.wantInCaption()
	if h := p.selection().Head.Pos; h.Offset != 4 {
		t.Fatalf("caret at %v, want the caption's end", h)
	}
}

func TestReturnOnASelectedFigureOpensAParagraphBelow(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.do(vocab.Dismiss{})
	p.do(vocab.Break{})
	p.want("page(figure() paragraph())")
	p.typeText("x")
	p.want("page(figure() paragraph(x))")
}

func TestBackspaceHoldsAFigureThenDeletesIt(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.typeText("D")
	p.do(left)
	p.do(backspace)
	if p.armed() == 0 {
		t.Fatal("the figure is not held")
	}
	p.do(backspace)
	p.want("page(paragraph())")
}

// Backspace at the start of a line after a figure deletes the boundary
// between them: an empty line, holding nothing, joins and goes; a line
// with text cannot join a figure, so the figure stands beside the caret
// and is held.
func TestBackspaceAfterAFigure(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.do(vocab.Dismiss{})
	p.do(vocab.Break{})
	p.do(backspace)
	p.want("page(figure())")
	if p.armed() != 0 {
		t.Fatal("the empty line held the figure instead of going")
	}

	p = open(t)
	p.placeFigure()
	p.do(vocab.Dismiss{})
	p.do(vocab.Break{})
	p.typeText("x")
	p.do(left)
	p.do(backspace)
	p.want("page(figure() paragraph(x))")
	if p.armed() == 0 {
		t.Fatal("the figure is not held")
	}
	p.do(backspace)
	p.want("page(paragraph(x))")
}

func TestMotionCrossesAFigureThroughItsCaption(t *testing.T) {
	p := open(t)
	p.typeText("a")
	p.placeFigure()
	p.typeText("c")
	p.do(vocab.Dismiss{})
	p.do(vocab.Break{})
	p.typeText("b")
	p.want("page(paragraph(a) figure(c) paragraph(b))")
	p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward})
	var path []string
	for range 6 {
		p.do(right)
		h := p.selection().Head.Pos
		n, _ := p.e.Snapshot().Node(h.Node)
		path = append(path, profile.Schema().KindName(n.Kind()))
	}
	// No gap where text stands beside the figure.
	want := []string{"paragraph", "figure", "figure", "paragraph", "paragraph", "paragraph"}
	for i := range want {
		if path[i] != want[i] {
			t.Fatalf("path %v, want %v", path, want)
		}
	}
}

func TestResizingAndAligningAFigure(t *testing.T) {
	p := open(t)
	p.placeFigure()
	id := p.figure()
	p.do(profile.ResizeFigure{ID: id, Span: 500})
	p.do(profile.AlignFigure{To: profile.AlignStart})
	n, _ := p.e.Snapshot().Node(id)
	if s := profile.ShapeOf(n); s.Span != 500 || s.Align != profile.AlignStart {
		t.Fatalf("shape %+v", s)
	}
}

func TestTypingOverASelectedFigureReplacesIt(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.do(vocab.Dismiss{})
	p.typeText("x")
	p.want("page(paragraph(x))")
}

func TestAnEmptyCaptionOpensOnlyWhileItsFigureIsHeld(t *testing.T) {
	p := open(t)
	p.typeText("a")
	p.placeFigure()
	p.do(vocab.Dismiss{})
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward})
	var path []string
	for range 4 {
		p.do(right)
		n, _ := p.e.Snapshot().Node(p.selection().Head.Pos.Node)
		path = append(path, profile.Schema().KindName(n.Kind()))
	}
	// Motion passes through the figure's caption, empty or not, as through
	// any object's own slot.
	if want := []string{"paragraph", "figure", "paragraph", "paragraph"}; fmt.Sprint(path) != fmt.Sprint(want) {
		t.Fatalf("path %v, want %v", path, want)
	}
	// Selected, the caption opens, and Escape writes in it.
	p.do(vocab.Place{Node: p.figure()})
	p.do(vocab.Dismiss{})
	p.wantInCaption()
}

// Return from the start of any line after a figure enters it: the line's
// edge and the boundary crossed going back are no scope to enter.
func TestReturnAfterAFigureEntersIt(t *testing.T) {
	for _, text := range []string{"", "x", "# x"} {
		p := open(t)
		p.placeFigure()
		p.do(vocab.Dismiss{})
		p.do(vocab.Break{})
		p.typeText(text)
		if text != "" {
			p.do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Backward})
		}
		p.do(vocab.Return{})
		p.wantInCaption()
	}
}

// A figure's picture is asked of the host to use, and replaced in place:
// the caption, alignment and span stay.
func TestAFiguresPictureIsUsedAndReplacedInPlace(t *testing.T) {
	p := open(t)
	p.placeFigure()
	p.typeText("Dawn")
	id := p.figure()
	p.do(profile.ResizeFigure{ID: id, Span: 500})
	p.now = p.now.Add(time.Second)
	out, err := p.e.Do(1, vocab.Input{Intent: profile.UsePicture{Use: profile.PictureSave}, At: p.now})
	if err != nil {
		t.Fatal(err)
	}
	want := profile.PictureAsk{Use: profile.PictureSave, Figure: id, Media: tide.Media}
	if len(out.Effects) != 1 || out.Effects[0].Kind != policy.EffectMedia || out.Effects[0].Data != want {
		t.Fatalf("effects %+v, want %+v", out.Effects, want)
	}
	shore := profile.Picture{Media: "shore", Width: 300, Height: 200}
	p.do(profile.ReplacePicture{ID: id, Picture: shore})
	p.want("page(figure(Dawn))")
	n, _ := p.e.Snapshot().Node(id)
	if s := profile.ShapeOf(n); s.Picture != shore || s.Span != 500 || s.Align != profile.AlignCenter {
		t.Fatalf("shape %+v", s)
	}
}
