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
		lineBlocks(text),
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
	layout := textlayout.NewCache().Update(flow, nil, 360, style)
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

func TestSurfacePaintsPlaceholderOnlyForEmptyFlow(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	flow := surface.EmptyFlow(projection.NewVersion(1, 1))
	settings := theme.DefaultSettings()
	style := textlayout.NewStyle(
		theme.NewProseStyle(settings.ProseFontSize),
		settings.Semantic(),
	)
	layout := textlayout.NewCache().Update(flow, nil, 360, style)
	frame, err := surface.NewFrame(flow, model.EmptyUTF16Range(0), 0, surface.TextStyle{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	painter := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, rect ui.Rect) {
			Paint(p, rect, &State{
				Flow: flow, Frame: frame, Layout: layout,
				Motion: motion.New(now), Theme: settings.Semantic(),
				Prose:   theme.NewProseStyle(settings.ProseFontSize),
				Focused: true, Placeholder: "Start writing…",
			})
		})
	}, 600, 400)
	painter.Frame()
	if image := painter.Image(); image == nil || !hasNonBackground(image, settings.Semantic().Surface.Page) {
		t.Fatal("placeholder produced no painted pixels")
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

func TestSurfacePaintsFocusedFillDecorationsBeneathText(t *testing.T) {
	text := "hello world"
	flow, err := surface.NewFlow(
		projection.NewVersion(1, 1),
		text,
		[]surface.StyleRun{{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: surface.TextStyle{},
		}},
		lineBlocks(text),
	)
	if err != nil {
		t.Fatal(err)
	}
	settings := theme.DefaultSettings()
	semantic := settings.Semantic()
	prose := theme.NewProseStyle(settings.ProseFontSize)
	layout := textlayout.NewCache().Update(flow, nil, 360, textlayout.NewStyle(prose, semantic))
	tinted := model.NewUTF16Range(0, 5)
	render := func(decorated, focused bool) *image.RGBA {
		var decorations []surface.Decoration
		if decorated {
			// The engine's active-mark and pair highlights use this shape.
			decorations = []surface.Decoration{{
				ID:         surface.DecorationID{Producer: "test"},
				Primitive:  surface.FillPrimitive([]model.UTF16Range{tinted}, surface.ColorAccent, 4),
				Opacity:    surface.FixedOpacity(20),
				Visibility: surface.VisibilityFocused,
			}}
		}
		frame, err := surface.NewFrame(flow, model.EmptyUTF16Range(11), 11, surface.TextStyle{}, decorations)
		if err != nil {
			t.Fatal(err)
		}
		state := &State{
			Flow: flow, Frame: frame, Layout: layout,
			Motion: motion.New(time.Unix(0, 0)), Theme: semantic, Prose: prose,
			Focused: focused,
		}
		tester := ui.NewTester((&surfaceFixture{state: state}).view, 400, 120)
		tester.Frame()
		return tester.Image()
	}
	row := layout.RangeRects(tinted)[0]
	// The top band of the row box is empty of glyph ink, so only a fill shows.
	x, y := int(row.X+row.W/2), int(row.Y+2)
	plain := render(false, true).RGBAAt(x, y)
	if got := render(true, true).RGBAAt(x, y); got == plain {
		t.Fatalf("focused fill decoration left %v untinted", got)
	}
	if got := render(true, false).RGBAAt(x, y); got != plain {
		t.Fatalf("focused-only fill painted while unfocused: %v, want %v", got, plain)
	}
}
