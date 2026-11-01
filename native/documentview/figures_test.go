package documentview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

var red = color.RGBA{R: 230, G: 20, B: 20, A: 255}

func redPicture(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := range 100 {
		for x := range 200 {
			img.Set(x, y, red)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// figurePage is a page of a paragraph above a picture, whose bytes read
// gives, drawn by an element that animates.
func figurePage(t *testing.T, read func(string) ([]byte, error)) (*controller.DocumentController, *Element, *ui.Tester) {
	t.Helper()
	document, err := controller.NewTextDocument("before", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	if err := document.SetSelection(controller.UTF16Range{Start: 6, End: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := document.Session().InsertImage(editor.ImageShape{Media: "red", Width: 200, Height: 100}); err != nil {
		t.Fatal(err)
	}
	document.Refresh()
	input := NewBridge(document, nil, func() time.Time { return time.Unix(1_700_000_000, 0) })
	settings := theme.DefaultSettings()
	element := NewElement(input, settings.Semantic(), theme.NewProseStyle(settings.ProseFontSize), time.Unix(1_700_000_000, 0))
	element.Animate = true
	element.Pictures = &Pictures{Read: read}
	tester := ui.NewTester(element.View, 600, 400)
	tester.Frame()
	return document, element, tester
}

func figureBox(t *testing.T, element *Element) textlayout.PlacedFigure {
	t.Helper()
	figures := element.current.Figures()
	if len(figures) != 1 {
		t.Fatalf("figures = %+v", figures)
	}
	return figures[0]
}

func pixel(tester *ui.Tester, x, y float32) color.RGBA {
	return tester.Image().RGBAAt(int(x), int(y))
}

func reddish(c color.RGBA) bool { return c.R > 180 && c.G < 80 && c.B < 80 }

// A figure shows its place while its picture is read, and the picture
// fades in over it once decoded.
func TestAFigureFadesInOnceItsPictureIsRead(t *testing.T) {
	release := make(chan struct{})
	data := redPicture(t)
	_, element, tester := figurePage(t, func(string) ([]byte, error) {
		<-release
		return data, nil
	})
	figure := figureBox(t, element)
	x, y := figure.Box.X+figure.Box.W/2, figure.Box.Y+element.TopInset+figure.Box.H/2
	if reddish(pixel(tester, x, y)) {
		t.Fatal("the picture showed before it was read")
	}
	if pixel(tester, x, y) == pixel(tester, 2, y) {
		t.Fatal("the figure's place does not show")
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for !reddish(pixel(tester, x, y)) {
		if time.Now().After(deadline) {
			t.Fatalf("the picture never showed: %v", pixel(tester, x, y))
		}
		time.Sleep(5 * time.Millisecond)
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
}

// A picture that cannot be read shows that it is missing in its place,
// and the page goes on around it.
func TestAMissingPictureKeepsItsPlace(t *testing.T) {
	_, element, tester := figurePage(t, func(string) ([]byte, error) {
		return []byte("not a picture"), nil
	})
	time.Sleep(20 * time.Millisecond)
	tester.Advance(time.Second)
	tester.Frame()
	figure := figureBox(t, element)
	if figure.Box.H != 100 {
		t.Fatalf("the missing picture's place is %+v", figure.Box)
	}
	if _, state := element.Pictures.Picture("red", time.Now()); state != PictureMissing {
		t.Fatalf("state = %v", state)
	}
}

// Selecting a figure's whole line rings the figure.
func TestASelectedFigureWearsTheRing(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	ringAt := func() color.RGBA {
		return pixel(tester, figure.Box.X+figure.Box.W/2, figure.Box.Y+element.TopInset-figureRingOutset)
	}
	tester.Advance(time.Second)
	tester.Frame()
	plain := ringAt()
	if err := document.SetSelection(controller.UTF16Range{Start: int(figure.Line.Start), End: int(figure.Line.End)}); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if ringAt() != plain {
		t.Fatal("the ring showed at once rather than fading in")
	}
	tester.Advance(time.Second)
	tester.Frame()
	if ringAt() == plain {
		t.Fatal("a selected figure wears no ring")
	}
}

// figureAtScreen is a point of a figure's box, as the element takes it.
func figureAtScreen(element *Element, figure textlayout.PlacedFigure, x float32) (float32, float32) {
	return figure.Box.X + x, figure.Box.Y + element.TopInset - element.visibleScroll() + figure.Box.H/2
}

func TestFigureCursor(t *testing.T) {
	data := redPicture(t)
	_, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	tester.Move(figureAtScreen(element, figure, figure.Box.W/2))
	if cursor := tester.Cursor(); cursor != ui.CursorDefault {
		t.Fatalf("over a figure the cursor is %v, want default", cursor)
	}
	tester.Move(figureAtScreen(element, figure, figure.Box.W-2))
	if cursor := tester.Cursor(); cursor != ui.CursorResizeEW {
		t.Fatalf("over a figure's edge the cursor is %v, want horizontal resize", cursor)
	}
}

// A click on a figure selects its whole line; Return then goes on below
// it, keeping it, and Escape leaves it with the caret after it.
func TestAClickSelectsAFigure(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	tester.ClickAt(figureAtScreen(element, figure, figure.Box.W/2))
	if got := document.Selection().Range; got != (controller.UTF16Range{Start: int(figure.Line.Start), End: int(figure.Line.End)}) {
		t.Fatalf("a click selected %+v, not the figure's line %+v", got, figure.Line)
	}
	if !element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape}) {
		t.Fatal("Escape was not taken")
	}
	// The caret stays on the line, at the caption's end.
	if got := document.Selection(); !got.Range.Empty() || got.Caret() != int(figure.Line.End) {
		t.Fatalf("Escape left the selection at %+v", got)
	}
	tester.ClickAt(figureAtScreen(element, figure, figure.Box.W/2))
	element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEnter})
	tester.Frame()
	if got := document.Text(); got != "before\n\uFFFC\n\n" {
		t.Fatalf("Return on a figure left %q", got)
	}
	if len(element.current.Figures()) != 1 {
		t.Fatal("Return on a figure took it away")
	}
}

// Dragging a figure's side resizes it, the page reflowing as it goes,
// snapping to half the column; letting go keeps the size in one step, and
// Escape during a drag puts it back.
func TestDraggingASideResizesAFigure(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	column := element.current.Width()
	x, y := figureAtScreen(element, figure, figure.Box.W-2)
	tester.Press(x, y)
	half := column/2 + column/4 + 3
	tester.Move(half, y)
	tester.Frame()
	resized := figureBox(t, element)
	if resized.Box.W != column/2 {
		t.Fatalf("mid-drag the figure is %v wide, want it snapped to %v", resized.Box.W, column/2)
	}
	tester.Release(half, y)
	tester.Frame()
	if got := figureBox(t, element); got.Span != 0.5 || got.Box.W != column/2 {
		t.Fatalf("the kept figure is %+v", got)
	}
	if _, err := document.Execute(controller.Command{ID: string(editor.HistoryUndo.ID())}); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if got := figureBox(t, element); got.Span != 0 || got.Box.W != 200 {
		t.Fatalf("one undo left %+v", got)
	}

	tester.Press(x, y)
	tester.Move(half, y)
	element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	tester.Release(half, y)
	tester.Frame()
	if got := figureBox(t, element); got.Span != 0 || got.Box.W != 200 {
		t.Fatalf("Escape kept the resize: %+v", got)
	}
}

// shownFigures records what a figure's menu and keys ask of the host.
type shownFigures struct {
	viewed, copied []string
}

func (s *shownFigures) ViewPicture(figure surface.Figure) { s.viewed = append(s.viewed, figure.Media) }
func (s *shownFigures) CopyPicture(media string)          { s.copied = append(s.copied, media) }
func (s *shownFigures) SavePicture(string)                {}
func (s *shownFigures) RevealPicture(string)              {}
func (s *shownFigures) ReplacePicture(model.NodeID)       {}
func (s *shownFigures) EditAlt(model.NodeID, string)      {}

// A selected figure copies as its picture, Space and a double click view
// it, and text around it still copies as text.
func TestASelectedFigureCopiesAndViewsAsAPicture(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	host := &shownFigures{}
	bridge := element.Input.(*Bridge)
	bridge.Figures = host
	bridge.Services = &memoryClipboard{}
	figure := figureBox(t, element)
	tester.ClickAt(figureAtScreen(element, figure, figure.Box.W/2))
	if err := bridge.Execute(keymap.Action{Kind: keymap.ActionClipboard, Clipboard: keymap.ClipboardCopy}); err != nil {
		t.Fatal(err)
	}
	if len(host.copied) != 1 || host.copied[0] != "red" {
		t.Fatalf("copied %v", host.copied)
	}
	element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeySpace})
	if len(host.viewed) != 1 || document.Text() != "before\n\uFFFC\n" {
		t.Fatalf("Space viewed %v and left %q", host.viewed, document.Text())
	}
	x, y := figureAtScreen(element, figure, figure.Box.W/2)
	element.handleInput(ui.InputEvent{Kind: ui.InputPointerDown, X: x, Y: y, Clicks: 2})
	element.handleInput(ui.InputEvent{Kind: ui.InputPointerUp, X: x, Y: y})
	if len(host.viewed) != 2 {
		t.Fatalf("a double click viewed %v", host.viewed)
	}
	if err := document.SetSelection(controller.UTF16Range{Start: 0, End: 6}); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Execute(keymap.Action{Kind: keymap.ActionClipboard, Clipboard: keymap.ClipboardCopy}); err != nil {
		t.Fatal(err)
	}
	if got := bridge.Services.ReadClipboard(); got != "before" || len(host.copied) != 1 {
		t.Fatalf("text copied as %q, pictures %v", got, host.copied)
	}
}

// The caret in an empty caption brings its hint in after it, fading, and
// leaving takes it away.
func TestAnEmptyCaptionHintsWhileTheCaretIsInIt(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	caret := element.current.Caret(figure.Line.End)
	top := caret.Y + element.TopInset - element.visibleScroll()
	inked := func() int {
		count := 0
		image := tester.Image()
		for y := top; y < top+caret.H; y++ {
			for x := caret.X + 4; x < caret.X+60; x++ {
				if image.RGBAAt(int(x), int(y)) != image.RGBAAt(2, int(y)) {
					count++
				}
			}
		}
		return count
	}
	tester.Advance(time.Second)
	tester.Frame()
	if inked() != 0 {
		t.Fatal("the hint showed with the caret elsewhere")
	}
	if err := document.SetSelection(controller.UTF16Range{Start: int(figure.Line.End), End: int(figure.Line.End)}); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if inked() != 0 {
		t.Fatal("the hint showed at once rather than fading in")
	}
	tester.Advance(time.Second)
	tester.Frame()
	if inked() == 0 {
		t.Fatal("an empty caption shows no hint")
	}
	tester.Frame()
	tester.Frame()
	nativeCaret, ok := tester.TextCaret()
	if !ok {
		t.Fatal("native text caret is unavailable")
	}
	if math.Abs(float64(nativeCaret.X-element.drawnCaret.X)) > 1 ||
		math.Abs(float64(nativeCaret.Y-element.drawnCaret.Y)) > 1 {
		t.Fatalf("native caret %v differs from drawn caret %v", nativeCaret, element.drawnCaret)
	}
}

// captionRoom is the room a figure's caption line takes on the page.
func captionRoom(t *testing.T, element *Element) float32 {
	t.Helper()
	for _, paragraph := range element.current.Paragraphs() {
		if _, ok := paragraph.Figure(); ok {
			return paragraph.Height()
		}
	}
	t.Fatal("no caption line")
	return 0
}

// A picture with no caption takes no room for one; the caret coming into
// its line opens it, gliding, and leaving it empty closes it again.
func TestAnEmptyCaptionOpensWhileTheCaretIsInIt(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	if room := captionRoom(t, element); room != 0 {
		t.Fatalf("an empty caption takes %v", room)
	}
	figure := figureBox(t, element)
	if err := document.SetSelection(controller.UTF16Range{Start: int(figure.Line.End), End: int(figure.Line.End)}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	opening := captionRoom(t, element)
	tester.Advance(time.Second)
	tester.Frame()
	open := captionRoom(t, element)
	if !(0 < opening && opening < open) {
		t.Fatalf("the caption opened through %v to %v", opening, open)
	}
	if err := document.SetSelection(controller.UTF16Range{Start: 0, End: 0}); err != nil {
		t.Fatal(err)
	}
	tester.Advance(time.Second)
	tester.Frame()
	if room := captionRoom(t, element); room != 0 {
		t.Fatalf("the caption left empty keeps %v", room)
	}
}

// Selecting a picture with no caption opens its line, so Escape writes
// one there; selecting elsewhere closes it.
func TestASelectedPictureOpensItsCaption(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	tester.ClickAt(figureAtScreen(element, figure, figure.Box.W/2))
	tester.Advance(time.Second)
	tester.Frame()
	if room := captionRoom(t, element); room == 0 {
		t.Fatal("a selected picture keeps its caption closed")
	}
	element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	if got := document.Selection(); !got.Range.Empty() || got.Caret() != int(figure.Line.End) {
		t.Fatalf("Escape left the selection at %+v", got)
	}
	if err := document.SetSelection(controller.UTF16Range{Start: 0, End: 0}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		tester.Advance(time.Second)
		tester.Frame()
	}
	if room := captionRoom(t, element); room != 0 {
		t.Fatalf("the caption left empty keeps %v", room)
	}
}

// A centered caption glides to its middle as it is typed, rather than
// jumping there.
func TestACenteredCaptionGlidesAsItIsTyped(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	if err := document.SetSelection(controller.UTF16Range{Start: int(figure.Line.End), End: int(figure.Line.End)}); err != nil {
		t.Fatal(err)
	}
	tester.Advance(time.Second)
	tester.Frame()
	if _, err := document.ReplaceSelection("Dusk", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	moving := false
	for _, track := range element.figures.tracks {
		moving = moving || track.left.Moving()
	}
	if !moving {
		t.Fatal("the caption jumped to its middle")
	}
	tester.Advance(time.Second)
	tester.Frame()
	for _, track := range element.figures.tracks {
		if track.left.Moving() || track.left.Value() >= figure.Box.X+figure.Box.W/2 {
			t.Fatalf("the caption settled at %v", track.left.Value())
		}
	}
}

// Up and Down cross a picture a row at a time, stopping in its caption's
// line, which opens for the caret.
func TestArrowsCrossAPicture(t *testing.T) {
	data := redPicture(t)
	document, element, tester := figurePage(t, func(string) ([]byte, error) { return data, nil })
	figure := figureBox(t, element)
	press := func(key ui.Key) int {
		t.Helper()
		element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: key})
		tester.Advance(time.Second)
		tester.Frame()
		return document.Selection().Caret()
	}
	end := len([]rune(document.Text()))
	if err := document.SetSelection(controller.UTF16Range{Start: end, End: end}); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if got := press(ui.KeyUp); got != int(figure.Line.End) {
		t.Fatalf("Up from below reached %d, not the caption at %d", got, figure.Line.End)
	}
	if got := press(ui.KeyUp); got > 6 {
		t.Fatalf("Up from the caption reached %d, not the line above", got)
	}
	if got := press(ui.KeyDown); got != int(figure.Line.End) {
		t.Fatalf("Down from above reached %d, not the caption", got)
	}
	if got := press(ui.KeyDown); got != end {
		t.Fatalf("Down from the caption reached %d, not the line below", got)
	}
}
