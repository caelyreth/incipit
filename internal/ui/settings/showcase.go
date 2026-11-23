// Package showcase draws a scripted showcase: a real page, typed by an
// internal/app/showcase player, kept centered on a small stage as it grows.
package settings

import (
	"math"
	"time"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/ui/motion"
	"github.com/caelyreth/neverland/internal/ui/page"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

const (
	// centerFrequency is how the page recenters as the text grows: slower
	// than the caret, so the line drifts into place behind the typing
	// rather than jumping with each character.
	centerFrequency = float32(9)
	// dimmed is a showcase's strength while its feature is off: it still
	// shows what the feature would do, as a suggestion.
	dimmed = float32(0.55)
)

// Stage is one showcase, as wide as its container and Height tall.
type showcaseStage struct {
	Key    string
	Script policy.Showcase
	// Policy is the policy shown, and Values the person's policy values.
	Policy string
	Values policy.Values
	Theme  theme.Theme
	Prose  theme.ProseStyle
	Height float32
	// Padding is the least margin around the text.
	Padding float32
	Radius  float32
	// Off dims the stage, for a feature that is turned off.
	Off bool
	// Animate plays the script; without it the finished page shows still.
	Animate bool
}

type stageState struct {
	player  *appsettings.ShowcasePlayer
	element *page.Element
	at      motion.Rect
	placed  bool
}

type stageKey struct{}

// View builds one leaf box. Everything that moves happens in its drawing:
// the player advances, the page paints and recenters at the display's
// rate without rebuilding the view, and a stage scrolled out of sight
// paints nothing and so plays nothing.
func (s showcaseStage) View(c *ui.Context) {
	animate := s.Animate && !c.Preferences().ReduceMotion
	box := ui.Box(c).Key(s.Key).WidthPercent(100).Height(s.Height).
		Radius(s.Radius).Background(s.Theme.Surface.Page).
		Border(1, s.Theme.Border.Variant).Clip()
	state := *ui.Local(box, stageKey{}, func() *stageState { return &stageState{} })
	now := c.Now()
	if state.player == nil {
		state.player = appsettings.NewShowcasePlayer(s.Script, s.Policy, s.Values, state.open, now)
	} else {
		state.player.Configure(s.Values, now)
	}
	box.Draw(func(p *ui.Painter, rect ui.Rect) {
		state.paint(p, rect, s, animate)
	})
}

// open is the stage's page over a loop's engine: each loop is a new page,
// and motion from the last one has no meaning for it.
func (st *stageState) open(e *engine.Engine) appsettings.ShowcasePage {
	ed := page.NewEditor(e, appsettings.ShowcaseView)
	st.element = page.NewElement(ed)
	st.placed = false
	return ed
}

func (st *stageState) paint(p *ui.Painter, rect ui.Rect, s showcaseStage, animate bool) {
	now := p.Now()
	next := now
	if animate {
		next = st.player.Advance(now)
	} else {
		st.player.Finish()
	}
	if st.element == nil {
		return
	}
	el := st.element
	el.Theme, el.Prose = s.Theme, s.Prose

	// The page centers the text it has: a line grows from the middle, and
	// a second line lifts the first. The offset lands on whole device
	// pixels so glyphs stay as crisp as anywhere else in the app.
	wrap := max(1, rect.W-2*s.Padding)
	contentW, contentH := el.Still(wrap)
	target := ui.Rect{
		X: (rect.W - min(contentW, wrap)) / 2,
		Y: (rect.H - max(contentH, s.Prose.LineHeight())) / 2,
	}
	if !st.placed || !animate {
		st.at.Hide()
		st.placed = contentH > 0
	}
	st.at.Step(target, now, centerFrequency, animate)
	at := st.at.Value()
	x, y := snap(at.X, p.Scale()), snap(at.Y, p.Scale())
	area := ui.Rect{X: rect.X + x, Y: rect.Y + y, W: wrap, H: rect.H - y}
	p.Clip(rect, s.Radius, func() {
		el.PaintStill(p, area)
		// A curtain of the page's own color fades the story in and out at
		// each loop, and dims it while the feature is off.
		strength := float32(1)
		if animate {
			strength = st.player.Opacity(now)
		}
		if s.Off {
			strength *= dimmed
		}
		if strength < 1 {
			p.Fill(rect, s.Theme.Surface.Page.Alpha(1-strength), s.Radius)
		}
	})
	if !animate {
		return
	}
	if st.at.Moving() || st.player.Opacity(now) < 1 {
		p.AnimationFrame()
	} else if wait := next.Sub(now); wait > 0 {
		p.After(min(wait, time.Second))
	} else {
		p.AnimationFrame()
	}
}

func snap(value, scale float32) float32 {
	if scale <= 0 {
		scale = 1
	}
	return float32(math.Round(float64(value*scale))) / scale
}
