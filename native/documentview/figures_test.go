package documentview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
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
