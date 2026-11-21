package shell

import (
	"strconv"
	"strings"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A list's contents stand in the field lane, under how the place is shown
// and down to the kept box (field-and-desk.md §4.7.6): a ruler of short
// ticks on one left edge, one for each divider, the divider at the list's
// top marked, scrolling where it holds more than its room. The pointer
// coming near, the ticks under it lengthen and brighten, the nearest most
// and the rest less as they go, a wave travelling with it, and a card
// beside the rail names the divider under it and what it holds. Pressed,
// the list goes to that divider. It is the list's alone, where order is
// exact, and comes as the view turns to the list.

// railStop is one of the rail's dividers: its name, the group it is if it
// is one, how far down the list it stands, and how many sheets stand
// under it.
type railStop struct {
	name   string
	group  *GroupItem
	y      float32
	sheets int
}

// railStops appends to stops the dividers of the list f found, laid out
// in frames.
func railStops(stops []railStop, f *found, frames []stageFrame) []railStop {
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
		if b.section == "" {
			st.group = b.group
		}
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

// Its measures, at the reference type size: a stop's room, a tick at rest
// and at its longest, how far the wave reaches either side of the
// pointer, and the card's width.
const (
	railStep  = 10
	railTick  = 6
	railLong  = 20
	railReach = 56
	railCard  = 240
)

// contentsRail draws the list's rail in the lane, its ticks standing on
// left, between top and bottom.
func (s *State) contentsRail(c *ui.Context, semantic theme.Theme, left, top, bottom, k float32, animate bool) {
	stops := s.field.rail
	if !s.field.listed || len(stops) < 2 || bottom-top < railStep*k {
		return
	}
	step := railStep * k
	strip := ui.Box(c).Key("contents-rail").Absolute().Left(0).Right(0).Top(top).Height(bottom - top).
		Role(ui.RoleGroup).Label("Contents").
		Transition(appearingIf(animate, motionPlace, &ui.Motion{Opacity: 0, X: -8 * k}, &ui.Motion{Opacity: 0}))
	_, py, over := strip.PointerPosition()
	near := map[bool]float32{true: 1}[over]
	if animate {
		near = components.Follow(strip, "near", near, components.MotionQuick)
	}
	// Where the pointer is along the ruler, scrolled as it is.
	along := py + s.field.railScroll.Y
	under := -1
	if over {
		under = max(0, min(len(stops)-1, int(along/step)))
		s.field.railShown = under
	}
	current := 0
	for i, st := range stops {
		if st.y <= s.field.scroll+1 {
			current = i
		}
	}
	strip.Children(func() {
		ui.Scroll(c).Key("ruler").Fill().Scrollbars(false).TrackScroll(&s.field.railScroll).Children(func() {
			ui.Box(c).Key("ticks").WidthPercent(100).Height(step * float32(len(stops))).Children(func() {
				for i, st := range stops {
					at := step * float32(i)
					wave := near * max(0, 1-abs32(along-(at+step/2))/(railReach*k))
					if i == under {
						wave = near
					}
					s.railStopView(c, semantic, i, st, left, at, step, wave, i == current || i == under, k)
				}
			})
		})
	})
	if near < 0.001 {
		return
	}
	// The card names what is under the pointer, beside the rail, going
	// with it from stop to stop.
	shown := min(s.field.railShown, len(stops)-1)
	st := stops[shown]
	y := top + step*float32(shown) - s.field.railScroll.Y + step/2
	if animate {
		y = components.Follow(strip, "card-y", y, components.MotionQuick)
	}
	card := ui.Column(c).Key("rail-card").Absolute().Left(left+(railLong+12)*k).Top(y-28*k).Width(railCard*k).
		Padding(12*k).Gap(4*k).Radius(12*k).IgnoreInput(true).AccessHidden(true).ZIndex(1).
		Background(semantic.Surface.Elevated).Border(1, semantic.Border.Variant).Opacity(near)
	card.Children(func() {
		// A group's divider shows its group's mark; a sort's step has none.
		ui.Row(c).Key("name").Gap(8 * k).AlignItems(ui.Center).Children(func() {
			if g := st.group; g != nil {
				ink := semantic.Icon.Muted
				if tint := groupTint(semantic, *g); tint != nil {
					ink = *tint
				}
				ui.Icon(c, groupIcon(*g).SVG()).FontSize(15 * k).TextColor(ink).Shrink(0)
			}
			components.CenteredLabel(c, st.name, theme.TextDefault, semantic.Text.Heading).MinWidth(0)
		})
		components.CenteredLabel(c, thousands(st.sheets)+" "+nounForm(st.sheets, "sheet", "sheets"), theme.TextSmall, semantic.Text.Muted)
	})
}

// railStopView is the rail's i-th stop, at y, h tall: its tick, standing
// on left and as long and as bright as the wave over it; pressed, the list
// goes to it.
func (s *State) railStopView(c *ui.Context, semantic theme.Theme, i int, st railStop, left, y, h, wave float32, marked bool, k float32) {
	btn := ui.ButtonBase(c.Key("rail-" + strconv.Itoa(i))).Absolute().Left(0).Right(0).Top(y).Height(h).
		AlignItems(ui.Center).Justify(ui.Start).PaddingStart(left).FocusRing(false).KeepFocus().Label("Go to " + st.name)
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

// appearingIf is appearing while animate, else at once.
func appearingIf(animate bool, d time.Duration, enter, exit *ui.Motion) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return appearing(d, enter, exit)
}
