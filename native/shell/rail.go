package shell

import (
	"strconv"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A long list's contents stand beside it, a rail (field-and-desk.md
// §4.7.6): at rest one short tick for each divider, quiet, the place read
// as a ruler, the divider at the list's top marked. The pointer coming
// near, the ticks under it widen into the dividers' names, the nearest
// widest and the rest less as they go, a wave travelling with it. Pressed,
// a name brings its divider to the list's top. It is the list's alone,
// where order is exact.

// railStop is one of the rail's dividers: its name, and how far down the
// list it stands.
type railStop struct {
	name string
	y    float32
}

// railStops are the dividers of the list f found, laid out in frames.
func railStops(f *found, frames []stageFrame) []railStop {
	var stops []railStop
	for i, wide := range listWide(f.cards) {
		if !wide || i >= len(frames) {
			continue
		}
		b := f.cards[i].kind.(block)
		name := b.section
		if name == "" {
			name = b.name()
		}
		stops = append(stops, railStop{name: name, y: frames[i].Rect.Y})
	}
	return stops
}

// Its measures, at the reference type size: a stop's room at rest and
// opened to its name, how far the wave reaches either side of the
// pointer, and the tick's length at rest and opened.
const (
	railRest  = 8
	railOpen  = 24
	railReach = 72
	railTick  = 8
	railLong  = 16
	railName  = 180
)

// contentsRail draws the rail of a list's stops by its column, whose right
// edge is at right, centred in the list's view, x within the field w wide.
func (s *State) contentsRail(c *ui.Context, semantic theme.Theme, stops []railStop, right, w float32, k float32, animate bool) {
	if len(stops) < 2 || s.field.hi-s.field.lo < 1 {
		return
	}
	top, bottom := s.field.viewTop, s.field.viewBottom
	rest, open := railRest*k, railOpen*k
	// At rest the stops close up if the view holds no more.
	rest = min(rest, (bottom-top)/float32(len(stops)))
	edge := min(right+railLong*k+12*k, w-8*k)
	strip := ui.Box(c).Key("contents-rail").Absolute().Top(top).Height(bottom - top).
		Role(ui.RoleGroup).Label("Contents")
	_, py, over := strip.PointerPosition()
	near := map[bool]float32{true: 1}[over]
	if animate {
		near = components.Follow(strip, "near", near, components.MotionQuick)
	}
	width := railLong*k + 12*k + (railName*k)*near
	strip.Left(edge - width).Width(width)
	// Where each stop stands at rest, and how near the pointer each is
	// there: the wave is measured against the rail at rest, so it does
	// not chase its own opening.
	restTop := (bottom-top)/2 - rest*float32(len(stops))/2
	heights := make([]float32, len(stops))
	closeness := make([]float32, len(stops))
	total := float32(0)
	for i := range stops {
		at := restTop + rest*(float32(i)+0.5)
		closeness[i] = near * max(0, 1-abs32(py-at)/(railReach*k))
		heights[i] = rest + (open-rest)*closeness[i]
		total += heights[i]
	}
	// The stop at the list's top is where the list is read.
	current := 0
	for i, st := range stops {
		if st.y <= s.field.scroll+1 {
			current = i
		}
	}
	strip.Children(func() {
		if near > 0.001 {
			ui.Box(c).Key("ground").Absolute().Left(0).Right(0).Top(max(0, (bottom-top)/2-total/2-6*k)).Height(total + 12*k).
				Radius(10 * k).Background(semantic.Surface.Background.Alpha(0.92 * near)).IgnoreInput(true)
		}
		y := (bottom-top)/2 - total/2
		for i, st := range stops {
			s.railStopView(c, semantic, i, st, i == current, y, heights[i], closeness[i], k)
			y += heights[i]
		}
	})
}

// railStopView is the rail's i-th stop, at y, h tall, opened by open: its
// tick, and its name as it opens; pressed, the list goes to it.
func (s *State) railStopView(c *ui.Context, semantic theme.Theme, i int, st railStop, current bool, y, h, open, k float32) {
	btn := ui.ButtonBase(c.Key("rail-" + strconv.Itoa(i))).Absolute().Left(0).Right(0).Top(y).Height(h).
		AlignItems(ui.Center).Justify(ui.End).Gap(6 * k).PaddingEnd(6 * k).
		FocusRing(false).KeepFocus().Label("Go to " + st.name)
	if btn.Clicked() {
		s.field.home, s.field.homeTo = true, max(s.field.lo, min(st.y, s.field.hi))
	}
	ink := components.Blend(semantic.Border.Default, semantic.Text.Muted, open)
	if current {
		ink = semantic.Text.Heading
	}
	btn.Children(func() {
		if open > 0.05 {
			components.CenteredLabel(c, st.name, theme.TextXSmall, ink).Opacity(open).MinWidth(0).Shrink(1)
		}
		ui.Box(c).Key("tick").Width((railTick + (railLong-railTick)*open) * k).Height(2 * k).Radius(k).Shrink(0).Background(ink)
	})
}
