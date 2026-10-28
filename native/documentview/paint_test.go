package documentview

import (
	"image"
	"os"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/testlock"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := testlock.Acquire(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type surfaceFixture struct{ state *State }

func (f *surfaceFixture) view(c *ui.Context) {
	ui.Box(c).Fill().Background(f.state.Theme.Surface.Page).Draw(func(p *ui.Painter, r ui.Rect) {
		Paint(p, r, f.state)
	})
}

func TestSurfacePaintsTextSelectionCursorDecorations(t *testing.T) {
	text := "hello\nworld"
	flow, err := surface.NewFlow(
		projection.NewVersion(1, 1),
		text,
		[]surface.StyleRun{{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: surface.TextStyle{},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	frame, err := surface.NewFrame(
		flow,
		model.NewUTF16Range(0, 5),
		5,
		surface.TextStyle{},
		[]surface.Decoration{{
			ID: surface.DecorationID{Producer: "test"},
			Primitive: surface.FillPrimitive(
				[]model.UTF16Range{model.NewUTF16Range(6, 11)},
				surface.ColorAccent,
				2,
			),
			Opacity:    surface.FixedOpacity(90),
			Visibility: surface.VisibilityAlways,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	settings := theme.DefaultSettings()
	style := textlayout.NewStyle(
		theme.NewProseStyle(settings.ProseFontSize),
		settings.Semantic(),
	)
	layout := textlayout.NewCache().Update(flow, 360, style)
	state := &State{
		Flow:    flow,
		Frame:   frame,
		Layout:  layout,
		Motion:  motion.New(now),
		Theme:   settings.Semantic(),
		Prose:   theme.NewProseStyle(settings.ProseFontSize),
		Focused: true,
		Animate: false,
	}
	tester := ui.NewTester((&surfaceFixture{state: state}).view, 400, 240)
	tester.Frame()
	image := tester.Image()
	if image == nil || image.Bounds().Dx() != 400 || image.Bounds().Dy() != 240 {
		t.Fatalf("image = %#v", image)
	}
	if !hasNonBackground(image, state.Theme.Surface.Page) {
		t.Fatal("surface painted only its background")
	}
}

func hasNonBackground(image *image.RGBA, background ui.Color) bool {
	for offset := 0; offset < len(image.Pix); offset += 4 {
		if image.Pix[offset] != background.R || image.Pix[offset+1] != background.G ||
			image.Pix[offset+2] != background.B || image.Pix[offset+3] != background.A {
			return true
		}
	}
	return false
}
