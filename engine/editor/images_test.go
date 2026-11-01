package editor

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

var sunset = ImageShape{Media: "ab12", Width: 800, Height: 600, Alt: "A sunset"}

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
	at(t, session, model.UTF16Offset(strings.Index(text, "![")+2))
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
	expectPage(t, session, "before\n![Dusk|]\nafter", "before", "Dusk", "after")
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
	expectPage(t, session, "![]\n|", "", "")
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
	expectPage(t, session, "before\n![Duskone]\ntwo|\nafter", "before", "Duskone", "two", "after")
}

// Backspace at a caption's start peels the picture away, as it peels any
// block's notation, leaving the caption; undo brings the picture back.
func TestPeelingAnImageLeavesItsCaption(t *testing.T) {
	session := imagePage(t)
	text := session.Projection().Text()
	at(t, session, model.UTF16Offset(strings.Index(text, "Dusk")))
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode, ParagraphNode, ParagraphNode)
	expectPage(t, session, "before\n|Dusk\nafter", "before", "Dusk", "after")
	if len(figures(t, session)) != 0 {
		t.Fatal("the picture stayed on a paragraph")
	}
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
	start := model.UTF16Offset(strings.Index(text, "!["))
	if err := session.SelectProjected(start, start+7); err != nil {
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
