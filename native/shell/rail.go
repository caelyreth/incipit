package shell

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A list with dividers keeps a column at its left for their rail
// (field-and-desk.md §4.7.6): a ruler of short ticks, one for each
// divider, the divider at the list's top marked. The pointer coming near,
// the ticks under it lengthen and brighten, the nearest most and the rest
// less as they go, a wave travelling with it, and a card beside the rail
// names the divider under it and what it holds. Pressed, the list goes to
// that divider. It is the list's alone, where order is exact.

// railStop is one of the rail's dividers: its name, how far down the list
// it stands, and how many sheets stand under it.
type railStop struct {
	name   string
	y      float32
	sheets int
}

// railStops are the dividers of the list f found, laid out in frames, if
// given.
func railStops(f *found, frames []stageFrame) []railStop {
	var stops []railStop
	for i, wide := range listWide(f.cards) {
		if !wide {
			if n := len(stops); n > 0 {
				for _, id := range f.cards[i].kind.rows() {
					if strings.HasPrefix(id, "sheet:") {
						stops[n-1].sheets++
					}
				}
			}
			continue
		}
		b := f.cards[i].kind.(block)
		st := railStop{name: b.section}
		if st.name == "" {
			st.name = b.name()
		}
		if i < len(frames) {
			st.y = frames[i].Rect.Y
		}
		stops = append(stops, st)
	}
	return stops
}

// Its measures, at the reference type size: the column it keeps, a stop's
// room, a tick at rest and at its longest, how far the wave reaches either
// side of the pointer, and the card's width.
const (
	railRoom  = 56
	railStep  = 10
	railTick  = 10
	railLong  = 32
	railReach = 56
	railCard  = 260
)

// contentsRail draws the rail of a list's stops in its column at left,
// centred in the list's view.
func (s *State) contentsRail(c *ui.Context, semantic theme.Theme, stops []railStop, left, k float32, animate bool) {
	top, bottom := s.field.viewTop, s.field.viewBottom
	step := min(railStep*k, (bottom-top)/float32(len(stops)))
	first := (bottom-top)/2 - step*float32(len(stops))/2
	strip := ui.Box(c).Key("contents-rail").Absolute().Left(left).Top(top).Width(railRoom * k).Height(bottom - top).
		Role(ui.RoleGroup).Label("Contents")
	_, py, over := strip.PointerPosition()
	near := map[bool]float32{true: 1}[over]
	if animate {
		near = components.Follow(strip, "near", near, components.MotionQuick)
	}
	// The stop under the pointer, and the one at the list's top.
	under := -1
	if over {
		under = max(0, min(len(stops)-1, int((py-first)/step)))
	}
	current := 0
	for i, st := range stops {
		if st.y <= s.field.scroll+1 {
			current = i
		}
	}
	strip.Children(func() {
		for i, st := range stops {
			at := first + step*float32(i)
			wave := near * max(0, 1-abs32(py-(at+step/2))/(railReach*k))
			if i == under {
				wave = near
			}
			s.railStopView(c, semantic, i, st, at, step, wave, i == current || i == under, k)
		}
	})
	if under >= 0 {
		s.field.railShown = under
	}
	if near < 0.001 {
		return
	}
	// The card names what is under the pointer, beside the rail, going
	// with it from stop to stop.
	shown := min(s.field.railShown, len(stops)-1)
	st := stops[shown]
	y := top + first + step*float32(shown) + step/2
	if animate {
		y = components.Follow(strip, "card-y", y, components.MotionQuick)
	}
	card := ui.Column(c).Key("rail-card").Absolute().Left(left+railRoom*k).Top(y-30*k).Width(railCard*k).
		Padding(12*k).Gap(4*k).Radius(12*k).IgnoreInput(true).AccessHidden(true).
		Background(semantic.Surface.Elevated).Border(1, semantic.Border.Variant).Opacity(near)
	card.Children(func() {
		components.CenteredLabel(c, st.name, theme.TextDefault, semantic.Text.Heading).MinWidth(0)
		components.CenteredLabel(c, thousands(st.sheets)+" "+nounForm(st.sheets, "sheet", "sheets"), theme.TextSmall, semantic.Text.Muted)
	})
}

// railStopView is the rail's i-th stop, at y, h tall: its tick, as long
// and as bright as the wave over it; pressed, the list goes to it.
func (s *State) railStopView(c *ui.Context, semantic theme.Theme, i int, st railStop, y, h, wave float32, marked bool, k float32) {
	btn := ui.ButtonBase(c.Key("rail-" + strconv.Itoa(i))).Absolute().Left(0).Right(0).Top(y).Height(h).
		AlignItems(ui.Center).PaddingStart(8 * k).FocusRing(false).KeepFocus().Label("Go to " + st.name)
	if btn.Clicked() {
		s.field.home, s.field.homeTo = true, max(s.field.lo, min(st.y, s.field.hi))
	}
	ink := components.Blend(semantic.Border.Default, semantic.Text.Muted, wave)
	if marked {
		ink = components.Blend(ink, semantic.Text.Heading, max(wave, 0.6))
	}
	btn.Children(func() {
		ui.Box(c).Key("tick").Width((railTick + (railLong-railTick)*wave) * k).Height(2 * k).Radius(k).Shrink(0).Background(ink)
	})
}
