package editor

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

var sunset = ImageShape{Media: "ab12", Width: 800, Height: 600, Alt: "A sunset"}

// pic is the character a picture stands on, as a page's text shows it.
const pic = surface.FigureObject

// imagePage is a page of a paragraph, a captioned image and a paragraph.
func imagePage(t *testing.T) *Session {
	t.Helper()
	session, _ := newSessionFixture(t, "before", false)
	at(t, session, 6)
	if _, err := session.InsertImage(sunset); err != nil {
		t.Fatal(err)
	}
	typeSession(t, session, "after")
	text := session.Projection().Text()
	at(t, session, model.UTF16Offset(strings.Index(text, pic)+1))
	typeSession(t, session, "Dusk")
	return session
}

func figures(t *testing.T, session *Session) []surface.Figure {
	t.Helper()
	flow, err := session.SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	return flow.Figures()
}

// An image goes after the caret's block, its caption its text, and the
// caret goes on below it; the flow shows its picture above its line.
func TestAnImageGoesOnThePageAboveItsCaption(t *testing.T) {
	session := imagePage(t)
	expectKinds(t, session, ParagraphNode, ImageNode, ParagraphNode)
	expectPage(t, session, "before\n"+pic+"Dusk|\nafter", "before", "Dusk", "after")
	shown := figures(t, session)
	if len(shown) != 1 {
		t.Fatalf("figures = %+v", shown)
	}
	figure := shown[0]
	if figure.At != 7 || figure.Media != sunset.Media || figure.Width != 800 ||
		figure.Height != 600 || figure.Alt != sunset.Alt {
		t.Fatalf("figure = %+v", figure)
	}
}

// An image taken into an empty paragraph takes its place, and a page
// ending on an image gets a paragraph to go on in.
func TestAnImageTakesAnEmptyParagraph(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	if _, err := session.InsertImage(sunset); err != nil {
		t.Fatal(err)
	}
	expectKinds(t, session, ImageNode, ParagraphNode)
	expectPage(t, session, pic+"\n|", "", "")
}

// Return in a caption opens a paragraph, never another picture; nor does a
// pasted line break.
func TestACaptionBreaksIntoParagraphs(t *testing.T) {
	session := imagePage(t)
	run(t, session, TextNewline)
	expectKinds(t, session, ParagraphNode, ImageNode, ParagraphNode, ParagraphNode)
	if len(figures(t, session)) != 1 {
		t.Fatal("a break in the caption made another picture")
	}

	session = imagePage(t)
	if _, err := session.ReplaceSelection("one\ntwo", kernel.EventOriginPaste); err != nil {
		t.Fatal(err)
	}
	expectKinds(t, session, ParagraphNode, ImageNode, ParagraphNode, ParagraphNode)
	expectPage(t, session, "before\n"+pic+"Duskone\ntwo|\nafter", "before", "Duskone", "two", "after")
}

// Backspace at a caption's start, or Delete at the end of the block
// above, selects the picture rather than taking it away; deleting it then
// leaves an empty line, and undo brings it back.
func TestDeletingTowardAPictureSelectsItFirst(t *testing.T) {
	session := imagePage(t)
	text := session.Projection().Text()
	start := model.UTF16Offset(strings.Index(text, pic))
	// The line is the picture's character and its caption, "Dusk".
	line := model.NewUTF16Range(start, start+5)
	selected := func() model.UTF16Range {
		presentation := session.Presentation()
		got, err := presentation.ProjectedSelection()
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	at(t, session, line.Start+1)
	run(t, session, TextBackspace)
	if got := selected(); got != line {
		t.Fatalf("Backspace selected %v, not the figure's line %v", got, line)
	}
	expectKinds(t, session, ParagraphNode, ImageNode, ParagraphNode)
	at(t, session, 6)
	run(t, session, TextDelete)
	if got := selected(); got != line {
		t.Fatalf("Delete selected %v, not the figure's line %v", got, line)
	}
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode, ParagraphNode, ParagraphNode)
	expectPage(t, session, "before\n|\nafter", "before", "", "after")
	node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Node())
	if _, has := node.Attribute(MediaAttribute); has {
		t.Fatal("the paragraph still names the picture")
	}
	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	if len(figures(t, session)) != 1 {
		t.Fatal("undo did not bring the picture back")
	}
}

// Selecting the whole line and typing replaces the picture with the text.
func TestTypingOverAnImageReplacesIt(t *testing.T) {
	session := imagePage(t)
	text := session.Projection().Text()
	start := model.UTF16Offset(strings.Index(text, pic))
	if err := session.SelectProjected(start, start+5); err != nil {
		t.Fatal(err)
	}
	typeSession(t, session, "x")
	expectKinds(t, session, ParagraphNode, ParagraphNode, ParagraphNode)
	expectPage(t, session, "before\nx|\nafter", "before", "x", "after")
}

// Shaping an image changes its picture's span, alignment and words in one
// step of history.
func TestShapingAnImageIsOneStep(t *testing.T) {
	session := imagePage(t)
	image := figures(t, session)[0].Node
	outcome, err := session.ShapeImage(image, func(shape *ImageShape) {
		shape.Span = 0.5
		shape.Align = surface.FigureStart
		shape.Alt = "Dusk over the bay"
	})
	if err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("ShapeImage = %v, %v", outcome, err)
	}
	figure := figures(t, session)[0]
	if figure.Span != 0.5 || figure.Align != surface.FigureStart || figure.Alt != "Dusk over the bay" {
		t.Fatalf("figure = %+v", figure)
	}
	if outcome, _ := session.ShapeImage(image, func(*ImageShape) {}); outcome != kernel.OutcomePass {
		t.Fatalf("an unchanged shape was %v", outcome)
	}
	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	if figure := figures(t, session)[0]; figure.Span != 0 || figure.Align != surface.FigureCenter {
		t.Fatalf("undo left %+v", figure)
	}
	if _, err := session.ShapeImage(figures(t, session)[0].Node+100, func(*ImageShape) {}); err == nil {
		t.Fatal("a missing block was shaped")
	}
}

// pictureBetween is a page of a paragraph, an image captioned as given and
// a paragraph, with where the picture's line starts and ends.
func pictureBetween(t *testing.T, caption string) (*Session, model.UTF16Range) {
	t.Helper()
	session, _ := newSessionFixture(t, "before", false)
	at(t, session, 6)
	if _, err := session.InsertImage(sunset); err != nil {
		t.Fatal(err)
	}
	typeSession(t, session, "after")
	start := model.UTF16Offset(strings.Index(session.Projection().Text(), pic))
	at(t, session, start+1)
	typeSession(t, session, caption)
	return session, model.NewUTF16Range(start, start+1+model.UTF16Offset(len(caption)))
}

// The edges around a picture behave alike whether or not it has a
// caption: Backspace from the block below selects it rather than joining
// text to it, Backspace in an empty block below takes the block away, the
// caret going to the caption's end, and Select All on the picture widens
// to the page.
func TestAPictureEdgesAlikeWithOrWithoutACaption(t *testing.T) {
	for _, caption := range []string{"", "Dusk"} {
		session, line := pictureBetween(t, caption)
		selected := func() model.UTF16Range {
			presentation := session.Presentation()
			got, err := presentation.ProjectedSelection()
			if err != nil {
				t.Fatal(err)
			}
			return got
		}
		at(t, session, line.End+1)
		run(t, session, TextBackspace)
		if got := selected(); got != line {
			t.Fatalf("%q: Backspace below selected %v, not the picture's line %v", caption, got, line)
		}
		run(t, session, TextSelectAll)
		if got, page := selected(), model.UTF16Offset(session.Projection().UTF16Len()); got != model.NewUTF16Range(0, page) {
			t.Fatalf("%q: Select All on the picture selected %v of %v", caption, got, page)
		}

		at(t, session, line.End)
		run(t, session, TextNewline)
		run(t, session, TextBackspace)
		if got := selected(); got != model.EmptyUTF16Range(line.End) {
			t.Fatalf("%q: Backspace in an empty block below left %v", caption, got)
		}
		expectKinds(t, session, ParagraphNode, ImageNode, ParagraphNode)
	}
}
