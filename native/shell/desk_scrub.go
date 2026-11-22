package shell

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The scrubber stands in for the strip's scroll bar: an overview of the
// strip at the desk's foot, at its left, a cell for each column and the
// window's span over them. It shows while the strip moves, and while the
// pointer is on it, and goes a while after the strip rests. A drag on it
// moves the strip under the span as far as it moves, and comes to rest at
// a column as a drag on the strip does; a press goes to the column under
// it. The cells only show the columns: the scrubber takes the pointer
// whole, so a drag may begin anywhere on it.

const (
	// scrubWidth is the most the scrubber takes across, and scrubHeight
	// its cells' height, at the reference type size.
	scrubWidth  = 168
	scrubHeight = 16
	// scrubLinger is how long the scrubber stays once the strip rests.
	scrubLinger = 900 * time.Millisecond
)

// scrubState is what the scrubber keeps between frames: where the strip
// was, and how many times it has moved, the last
// of which its stay counts from.
type scrubState struct {
	x     float32
	moves int
	// drawn is whether the scrubber was drawn in the last frame.
	drawn bool
}

// deskFoot draws the strip's overview over the desk's foot, gutter in
// from its left and bottom, and, as it shows, a fade over the foot of the
// columns under it, so it reads over the pages running on there; both
// come and go together. It is drawn after the strip, as it reads the
// strip's columns as they were laid out.
//
// Go to Sheet grows it open into the switcher (desk_switch.go) as one
// motion from the corner it stands in: the overview stays where it is at
// the foot, the switcher's body, laid out at its open width all the way,
// is uncovered above it as the scrubber grows and fades in once there is
// room for it, its corners rounding to a panel's; a guard dims the desk
// behind it. Closing runs the same way back.
func (s *State) deskFoot(c *ui.Context, semantic theme.Theme, gutter float32, animate bool) {
	s.noteClosed()
	spans := s.desk.spans
	sw := &s.desk.switcher
	if sw.open && (!s.OnDesk() || len(s.Open) == 0) {
		sw.open = false
	}
	open := sw.open
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	scale := s.Settings.UIFontSize
	pad := 4 * k
	track := float32(0)
	var total, f float32
	if len(spans) > 0 {
		total = spans[len(spans)-1].End()
		track = min(scrubWidth*k, float32(len(spans))*scrubHeight*k*2)
		f = track / max(total, 1)
	}
	view := s.desk.viewport.W - 2*gutter
	if s.desk.strip.X != s.desk.scrub.x {
		s.desk.scrub.x = s.desk.strip.X
		s.desk.scrub.moves++
	}
	if len(spans) < 2 && !open && sw.grown < 0.001 {
		s.desk.scrub.drawn = false
		return
	}
	bar := ui.Column(c).Key("desk-scrubber").Absolute().Left(gutter).Bottom(gutter).
		Clip().
		Background(semantic.Surface.Panel).Border(1, semantic.Border.Variant).
		Role(ui.RoleGroup).Label("Desk overview")
	// grown is how far open the switcher is, 0 to 1, the guard, the
	// corners and the body following it.
	grown := float32(0)
	if open {
		grown = 1
	}
	if animate {
		if !s.desk.scrub.drawn {
			// Not drawn until the switcher opens, as with a sheet alone on
			// the desk: it grows from closed all the same.
			bar.Jump("grown", 0)
		}
		grown = components.Follow(bar, "grown", grown, switchMotion)
	}
	s.desk.scrub.drawn = true
	sw.grown = grown
	bar.Radius(lerp(6*k, theme.DefaultControls.Scaled(scale).DockRadius, grown))
	if grown > 0.001 {
		guard := ui.Box(c).Key("switch-guard").Absolute().Left(0).Right(0).Top(0).Bottom(0).ZIndex(1).
			Background(semantic.Surface.Background.Alpha(guardDim * grown))
		if !open {
			guard.IgnoreInput(true)
		} else if guard.Label("Close the switcher").Pressed() {
			s.closeSwitcher()
		}
	}
	// The stay counts from the strip's last move: the value reaches the
	// moves so far scrubLinger after the last.
	rest := bar.AnimateTo("scrub-rest", float32(s.desk.scrub.moves), ui.Animation{Duration: scrubLinger})
	moving := rest != float32(s.desk.scrub.moves)
	width := switchWidth * k
	// The body's room: the desk's height less the field, the gaps and the
	// foot, so it never overflows the panel.
	metrics := theme.DefaultControls.Scaled(scale)
	gap := theme.Space08.DIPs(scale)
	// The foot stands further in as the switcher opens, clear of its edge.
	foot := lerp(pad, theme.Space12.DIPs(scale), grown)
	inner := theme.Space12.DIPs(scale)
	most := max(3*metrics.Height*1.5, s.desk.viewport.H-2*gutter-pad-inner-scrubHeight*k-
		metrics.Height-theme.Space04.DIPs(scale)-3*gap-1)
	// Its size, grown from the scrubber's to the switcher's and back: set
	// here, not laid out, so it grows from the corner it stands in, the
	// overview staying put at its foot. Its height is set as it opens, from
	// what it lists then, and stays while words are written: what they
	// find scrolls in it.
	closedW, closedH := max(track, scrubHeight*k*2)+2*pad, scrubHeight*k+2*pad
	if grown > 0.001 && sw.height == 0 {
		sw.list = s.switchListHeight(most)
		sw.height = pad + s.switchBodyHeight(sw.list) + gap + scrubHeight*k + inner
	}
	openH := sw.height
	if animate {
		closedW = components.Follow(bar, "closed-w", closedW, components.MotionNormal)
	}
	bar.Width(lerp(closedW, width, grown)).Height(lerp(closedH, openH, grown))
	if open {
		bar.Label("Go to sheet")
	} else {
		if dx, _, ok := bar.Dragged(); ok && dx != 0 && f > 0 {
			s.stripAt(s.desk.strip.X + dx/f)
		}
		if _, _, ok := bar.DragEnded(); ok && f > 0 {
			s.settleStrip(s.desk.strip.X)
		} else if x, _, _ := bar.PointerPosition(); bar.Clicked() && f > 0 {
			s.scrubTo((x - pad) / f)
		}
	}
	target := float32(0)
	if open || moving || bar.Hovered() || bar.Dragging() {
		target = 1
	}
	shown := target
	if animate {
		shown = components.Follow(bar, "scrub-shown", target, components.MotionNormal)
	}
	bar.Opacity(max(shown, grown)).ZIndex(2)
	if shown > 0.001 && grown < 0.001 {
		depth := scrubHeight*k + 2*pad + 2*gutter
		ui.Box(c).Key("desk-foot-fade").Absolute().Left(0).Right(0).Bottom(0).Height(depth).
			IgnoreInput(true).Opacity(shown).
			Gradient(semantic.Surface.Background.Alpha(0), semantic.Surface.Background, 180)
	}
	// The body comes in once the scrubber has grown most of the way, and
	// goes first on the way back.
	seen := components.Smoothstep((grown - 0.35) / 0.65)
	bar.Children(func() {
		// Both stand from the foot, so the scrubber's edge, growing,
		// uncovers the body from the top.
		if grown > 0.001 {
			s.switcherView(c, semantic, width-2*pad, sw.list, animate).Opacity(seen).
				Absolute().Left(pad).Bottom(foot + scrubHeight*k + gap)
		}
		// The foot: the overview, a cell for each column, the window's
		// span over them, and, open, the column the keys are on lit; the
		// switcher's keys beside it.
		ui.Box(c).Key("scrub-foot").Absolute().Left(foot).Bottom(foot).Width(width - 2*foot).Height(scrubHeight * k).Children(func() {
			if len(spans) >= 2 {
				s.scrubTrack(c, semantic, track, total, view, f, k)
			}
			if grown > 0.001 {
				hints := s.switchHints(c, semantic)
				hints.Absolute().Top((scrubHeight*k - components.KeycapStyle(semantic, scale, true).Height) / 2).
					Opacity(seen).IgnoreInput(true)
				if track > 0 {
					hints.Left(track + theme.Space12.DIPs(scale))
				} else {
					hints.Left(theme.Space04.DIPs(scale))
				}
			}
		})
	})
}

// scrubTrack is the overview's cells and the window's span over them,
// track wide at f of the strip; the column the switcher's keys are on lit.
func (s *State) scrubTrack(c *ui.Context, semantic theme.Theme, track, total, view, f, k float32) {
	spans := s.desk.spans
	sw := &s.desk.switcher
	lit := ""
	if sw.open {
		if rows := s.switchRows(); sw.at >= 0 && sw.at < len(rows) {
			lit = rows[sw.at].column
		}
	}
	ui.Box(c).Key("scrub-track").Absolute().Left(0).Top(0).Width(track).Height(scrubHeight * k).Children(func() {
		for i, sp := range spans {
			fill := semantic.Element.Hover
			if lit != "" && i < len(s.Open) && s.Open[i].Column == lit {
				fill = semantic.Accent.Muted
			}
			ui.Box(c).Key(fmt.Sprintf("scrub-%d", i)).Absolute().
				Left(sp.X * f).Top(0).Width(max(1, sp.W*f-k)).Height(scrubHeight * k).
				Radius(2 * k).Background(fill)
		}
		ui.Box(c).Key("scrub-span").Absolute().Left(s.desk.strip.X*f).Top(0).
			Width(min(view, total)*f).Height(scrubHeight*k).Radius(2*k).
			Border(1, semantic.Accent.Foreground)
	})
}

// guardDim is how far the guard dims the desk behind the open switcher.
const guardDim = 0.55

// switchMotion is how long the scrubber takes to grow into the switcher
// and back.
const switchMotion = components.MotionSettle

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

// scrubTo goes to the column at x across the strip's columns.
func (s *State) scrubTo(x float32) {
	if len(s.desk.spans) == 0 {
		return
	}
	at := stageAt(s.desk.spans, x)
	s.desk.strip.ScrollTo(stageRests(s.desk.spans, s.desk.strip.MaxX)[at], 0, deskMotion(true))
}

// settleStrip brings the strip to rest at at, or pinned to a column's
// start near it, as the magnet would (magnet).
func (s *State) settleStrip(at float32) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	at = max(0, min(at, s.desk.strip.MaxX))
	if rest, ok := stageNearRest(s.desk.spans, s.desk.strip.MaxX, at, magnetReach*k); ok {
		at = rest
	}
	s.desk.armed = false
	s.desk.strip.ScrollTo(at, 0, deskMotion(true))
}
