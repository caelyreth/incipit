package documentview

import (
	"image"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type guideFixture struct {
	flow   *surface.Flow
	layout *textlayout.Layout
	theme  theme.Theme
	prose  theme.ProseStyle
}

func newGuideFixture(t *testing.T, text string, width float32) guideFixture {
	t.Helper()
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
	return guideFixture{
		flow:   flow,
		layout: textlayout.NewCache().Update(flow, nil, width, textlayout.NewStyle(prose, semantic)),
		theme:  semantic,
		prose:  prose,
	}
}

// state is a collapsed caret at 0 with a guide at each offset in at.
func (f guideFixture) state(t *testing.T, guides *Guides, focused bool, at ...model.UTF16Offset) *State {
	t.Helper()
	decorations := make([]surface.Decoration, 0, len(at))
	for index, offset := range at {
		decorations = append(decorations, surface.Decoration{
			ID:         surface.DecorationID{Producer: "navigation", Local: uint64(index)},
			Primitive:  surface.GuidePrimitive(offset, []string{"Tab"}, surface.ColorMuted),
			Opacity:    surface.FixedOpacity(255),
			Visibility: surface.VisibilityFocused,
		})
	}
	frame, err := surface.NewFrame(f.flow, model.EmptyUTF16Range(0), 0, surface.TextStyle{}, decorations)
	if err != nil {
		t.Fatal(err)
	}
	return &State{
		Flow: f.flow, Frame: frame, Layout: f.layout,
		Motion: motion.New(time.Unix(0, 0)), Theme: f.theme, Prose: f.prose,
		Focused: focused, Animate: true, Guides: guides,
	}
}

func onlyGuide(t *testing.T, guides *Guides) GuideSample {
	t.Helper()
	visible := guides.Visible()
	if len(visible) != 1 {
		t.Fatalf("visible guides = %#v", visible)
	}
	return visible[0]
}

func TestGuideFadesInGlidesAndFadesOut(t *testing.T) {
	fixture := newGuideFixture(t, "hello world", 360)
	guides := &Guides{}
	start := time.Unix(0, 0)
	options := func(at time.Duration) PaintOptions {
		return PaintOptions{Now: start.Add(at), Animate: true, Focused: true}
	}

	near := fixture.state(t, guides, true, 5)
	if !guides.step(near, options(0)) {
		t.Fatal("an appearing guide must request frames")
	}
	if len(guides.Visible()) != 0 {
		t.Fatal("a new guide appears from nothing, not at full strength")
	}
	guides.step(near, options(40*time.Millisecond))
	rising := onlyGuide(t, guides).Opacity
	if rising <= 0 || rising >= 0.9 {
		t.Fatalf("opacity 40ms in = %v, want a fade in progress", rising)
	}
	for at := 56 * time.Millisecond; at <= 600*time.Millisecond; at += 16 * time.Millisecond {
		guides.step(near, options(at))
	}
	settled := onlyGuide(t, guides)
	from := fixture.layout.Caret(5).X
	if settled.Opacity < 0.99 || settled.Rect.X != from {
		t.Fatalf("settled guide = %#v, want opaque at %v", settled, from)
	}

	far := fixture.state(t, guides, true, 11)
	to := fixture.layout.Caret(11).X
	guides.step(far, options(616*time.Millisecond))
	guides.step(far, options(632*time.Millisecond))
	if x := onlyGuide(t, guides).Rect.X; x <= from || x >= to {
		t.Fatalf("guide x = %v mid-glide, want between %v and %v", x, from, to)
	}
	if opacity := onlyGuide(t, guides).Opacity; opacity < 0.99 {
		t.Fatalf("a moving guide dimmed to %v", opacity)
	}

	gone := fixture.state(t, guides, true)
	guides.step(gone, options(700*time.Millisecond))
	guides.step(gone, options(740*time.Millisecond))
	leaving := onlyGuide(t, guides)
	if leaving.Opacity <= 0 || leaving.Opacity >= 0.99 {
		t.Fatalf("leaving opacity = %v, want a fade out in progress", leaving.Opacity)
	}
	for at := 756 * time.Millisecond; at <= 1500*time.Millisecond; at += 16 * time.Millisecond {
		guides.step(gone, options(at))
	}
	if len(guides.tracks) != 0 {
		t.Fatalf("a faded guide must be forgotten, %d tracks remain", len(guides.tracks))
	}
}

func TestGuideSettlesAtOnceWithoutMotion(t *testing.T) {
	fixture := newGuideFixture(t, "hello world", 360)
	guides := &Guides{}
	options := PaintOptions{Now: time.Unix(0, 0), Animate: false, Focused: true}
	if guides.step(fixture.state(t, guides, true, 5), options) {
		t.Fatal("reduced motion must not request frames")
	}
	if sample := onlyGuide(t, guides); sample.Opacity != 1 || sample.Rect.X != fixture.layout.Caret(5).X {
		t.Fatalf("guide = %#v", sample)
	}
	options.Focused = false
	guides.step(fixture.state(t, guides, false, 5), options)
	if len(guides.Visible()) != 0 {
		t.Fatal("a focused-only guide painted while unfocused")
	}
}

func TestGuidePaintsKeycapsAboveTheLandingPoint(t *testing.T) {
	const width, height = 400, 120
	fixture := newGuideFixture(t, "hello world", 360)
	render := func(at ...model.UTF16Offset) *image.RGBA {
		state := fixture.state(t, nil, true, at...)
		tester := ui.NewTester((&surfaceFixture{state: state}).view, width, height)
		tester.Frame()
		return tester.Image()
	}
	plain, guided := render(), render(5)
	marker := fixture.layout.Caret(5)
	changed := image.Rectangle{Min: image.Point{width, height}}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if plain.RGBAAt(x, y) == guided.RGBAAt(x, y) {
				continue
			}
			changed.Min.X, changed.Min.Y = min(changed.Min.X, x), min(changed.Min.Y, y)
			changed.Max.X, changed.Max.Y = max(changed.Max.X, x+1), max(changed.Max.Y, y+1)
		}
	}
	if changed.Empty() {
		t.Fatal("a guide painted nothing")
	}
	if float32(changed.Min.X) < marker.X-1 || float32(changed.Min.Y) >= marker.Y {
		t.Fatalf("guide ink %v, want it to start at x=%v and rise above y=%v", changed, marker.X, marker.Y)
	}
}
