package shell

import (
	"fmt"
	"runtime"

	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// fieldView draws the field between the field lane and the desk lane, x
// from the window's left, w by h, as the scene stands it: what the query finds in slots on either
// side of its horizon (stageHorizon), the pill in the horizon, and the
// whole scrolling up and down as one, the pill keeping near the window's
// top or foot once its horizon has gone past them; pressed there, the
// pill brings the field back to its horizon. Hidden behind the desk, it
// skips building its children and keeps its outer veil while closed.
func (s *State) fieldView(c *ui.Context, semantic theme.Theme, look stageLook, x, w, h float32, hidden, animate bool) {
	m := s.blockMetrics(w, h)
	k := m.k
	// The window resized, or the type, the field stands at once as its new
	// measures lay it out: nothing of it springs from where it stood, so
	// each frame of a resize is drawn whole and at its own size. Measures
	// the query changes, as the settings' wider columns, move as any change
	// of the query does.
	if room := (fieldRoom{w, h, m.scale}); room != s.field.room {
		animate = false
		s.field.room = room
	}
	view := ui.Box(c).Key("field").Absolute().Top(0).Left(x).Size(w, h).Clip().Hidden(hidden).
		Role(ui.RoleGroup).Label(locales.Field())
	s.field.hidden = hidden
	if hidden {
		// Occluded by the desk: skip building cards, ribbons, and search input
		// while preserving the outer node until returning to the field.
		return
	}
	if look.Offset.X != 0 {
		view.Translate(look.Offset.X, 0)
	}
	s.notePlace()
	view.ContextMenu(s.fieldMenu)
	view.HandleInput(s.fieldInput)
	// The field takes the keyboard from the pill let go of, and as it
	// comes back from the desk, for that frame alone: after it, nothing
	// holds the keyboard, and Escape goes on with the field.
	if s.field.quiet && !hidden {
		s.field.quiet = false
		view.Focusable().FocusRing(false).Focus()
	}
	if !s.OnDesk() && !c.FocusTakesText() {
		switch {
		case c.Shortcut(0, ui.KeyEscape):
			s.escapeField()
		case !s.InSettings() && c.Shortcut(ui.Cmd, ui.KeyZ):
			s.Undo()
		case !s.InSettings() && (c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) || (runtime.GOOS != "darwin" && c.Shortcut(ui.Cmd, ui.KeyY))):
			s.Redo()
		case s.field.acting && c.Shortcut(0, ui.KeyUp):
			s.actionKey(ui.KeyUp)
		case s.field.acting && c.Shortcut(0, ui.KeyDown):
			s.actionKey(ui.KeyDown)
		case s.field.acting && c.Shortcut(0, ui.KeyEnter):
			s.actionKey(ui.KeyEnter)
		}
	}
	listed := s.listed()
	// Turning between the views is a swap: one view goes, the other comes.
	// What goes stays where it was drawn. Detect this before results() so
	// switching via $sort: skips steady() and ranks for the list at once.
	if listed != s.field.listed && s.field.rest != nil {
		s.field.next = change{depth: stageSwap}
		s.field.swapFrom = s.field.origin
	}
	// A list has no depth to go through: going into a group in it, or back
	// out, what stays moves to where it stands and the rest fades where it
	// is, as any change across. A divider is as wide as the list, and what
	// came out of it or went back into it would stretch.
	if listed && s.field.listed && s.field.next.depth != stageSwap {
		s.field.next = change{}
	}
	s.field.listed = listed
	f := s.results(c.Now())
	shapes := s.shapes(c, f, m)
	heights := make([]float32, len(shapes))
	for i, sh := range shapes {
		heights[i] = sh.size.H
	}
	// Its band reaches past the pill as far as the ribbons around it do.
	maxPillW := max(0, w-2*(m.pad+horizonSide*k))
	idlePillW, openPillW := min(280*k, maxPillW), min(440*k, maxPillW)
	activePill := s.field.open || (s.field.focus && !s.OnDesk() && !s.field.hidden)
	targetPillW := idlePillW
	pillAnim := components.Settle(components.MotionNormal)
	if activePill {
		targetPillW = openPillW
		pillAnim = components.Settle(components.MotionPanel)
	}
	pillW := targetPillW
	if animate {
		pillW = max(0, min(w, view.AnimateTo("pill-w", targetPillW, pillAnim)))
	} else {
		view.Jump("pill-w", targetPillW)
	}
	pill := stageSize{W: pillW, H: 40 * k}
	rl := s.ribbons(c, k, w/2, targetPillW, pill.H, -1)
	z := m.horizon
	z.Above += rl.above
	z.Below += rl.below
	// Each band chosen past the last, the ring's room between holding its
	// name.
	bands := cardBands(f)
	if bands != nil {
		z.Named = true
		z.Ring = theme.TextXSmall.LineHeight(m.scale) + z.Gap
	}
	rects, marks := z.Place(heights, bands)
	if listed {
		rects, marks = z.Rows(heights, listWide(f.cards)), nil
	}
	frames := make([]stageFrame, len(rects))
	for i, r := range rects {
		frames[i] = stageFrame{ID: f.cards[i].id, Size: shapes[i].size, Rect: r, Shown: 1}
	}
	below, above := stageReach(rects)
	below, above = max(below, z.Below), max(above, z.Above)
	top, foot := 48*k, 24*k
	// At rest, the pill stands at the window's middle, its ribbons with it,
	// however many stand on either side: only cards standing on one side
	// more than the other move it, the horizon rising or falling into the
	// room they leave empty over or under it; and the cards,
	// where a ring of columns is taken on one side alone, go across to
	// stand even about the pill. While the pill is open it keeps still,
	// under the pointer and the caret, and the field with it, until the
	// field answers what is written (follow), cards and horizon moving
	// together; but never so far that something of the field would leave
	// the window or its columns, which brings it in as far as needs be.
	// Let go of, the pill goes where the field is even.
	first := s.field.rest == nil
	up, down := max(0, h/2-top-above), max(0, h/2-foot-below)
	lo, hi := stageExtent(rects)
	reach := (float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap) / 2
	if !s.field.open || first || s.field.settled {
		s.field.settled = false
		cardsBelow, cardsAbove := max(0, below-z.Below), max(0, above-z.Above)
		s.field.poiseTo = stagePoint{X: -(lo + hi) / 2, Y: (cardsBelow - cardsAbove) / 2}
	}
	// What it is held to stays as it was: brought in only as far as the
	// field needs now, it goes back as far as the field lets it.
	poise := stagePoint{
		X: max(-reach-lo, min(reach-hi, s.field.poiseTo.X)),
		Y: max(-down, min(up, s.field.poiseTo.Y)),
	}
	// A list is laid out from the window's top down; only poise.X centers its columns.
	if listed {
		poise.Y = 0
	}
	lift := poise.Y
	// What shows clear of the window's top band and of the pill with what
	// stands about it, and how far the field scrolls to show all it holds.
	// The field scrolls about its horizon, as far as its cards reach past
	// it; a list reads from the window's top down to its foot, the pill
	// with all that stands over it there.
	reached := s.field.hi
	if listed {
		_, end := stageReachDown(rects)
		footTop := h - foot - pill.H/2 - rl.heldBelow - z.Above
		room := max(0, footTop-top)
		s.field.base, s.field.viewTop, s.field.viewBottom = top, top, footTop
		s.field.lo, s.field.hi = min(0, end-room), end-room
	} else {
		s.field.base, s.field.viewTop, s.field.viewBottom = h/2-lift, top+rl.above, h-foot-rl.below
		s.field.lo, s.field.hi = -max(0, above+3*m.pad-h/2+lift), max(0, below+3*m.pad-h/2-lift)
	}
	// The keys start from what ranks first, nearest the pill; they go on,
	// and a Shift-click takes its run, in the order the field is read on
	// the screen (field-and-desk.md §4.7.7).
	// The list's dividers, for the rail in the field lane (rail.go).
	s.field.rail = s.field.rail[:0]
	if listed {
		s.field.rail = railStops(s.field.rail, &f, frames)
	}
	s.field.ranked, s.field.order = s.field.ranked[:0], s.field.order[:0]
	for _, cd := range f.cards {
		s.field.ranked = append(s.field.ranked, cd.rows...)
	}
	if listed {
		// A list is read as it is ranked.
		s.field.order = append(s.field.order, s.field.ranked...)
	} else {
		for _, i := range stageScreenOrder(rects) {
			s.field.order = append(s.field.order, f.cards[i].rows...)
		}
	}
	s.keepPicked()
	s.keepRowPlaces()
	// Return pressed on words not answered yet enters what they rank,
	// now the field has ranked them.
	if s.field.entered {
		s.field.entered = false
		s.enter(s.field.enteredAside)
	}
	// Each place, the view with the query it shows, keeps where it was
	// scrolled to: gone to, a place stands as it was left, or at its home
	// when new (the field's horizon, the end for filed order and time
	// sorts, or the top for title and word-count sorts; short lists stand
	// on the foot). Within a place, the field stays where it is as what it
	// holds changes; at the end of what it reaches, it keeps to the end, as
	// when the ribbons come over a list's foot.
	atEnd := (listed || reached > 0) && s.field.scroll >= reached-0.5
	place := placeKey(listed, f.query)
	switch {
	case s.field.rest == nil:
		s.field.place = place
		s.scrollTo(view, s.placeHome(listed))
	case place != s.field.place:
		s.field.places = remember(s.field.places, s.field.place, s.scrollAt())
		s.field.place = place
		s.scrollTo(view, recall(s.field.places, place, s.placeHome(listed)))
	case !s.field.home:
		to := max(s.field.lo, min(s.field.scroll, s.field.hi))
		if atEnd {
			to = s.field.hi
		}
		if to != s.field.scroll {
			s.field.home, s.field.homeTo = true, to
		}
	default:
		if (listed || reached > 0) && abs32(s.field.homeTo-reached) < 0.5 && s.field.hi != reached {
			s.field.homeTo = s.field.hi
		} else {
			s.field.homeTo = max(s.field.lo, min(s.field.homeTo, s.field.hi))
		}
	}
	// What arrives is brought into view, wherever the change left it.
	s.reveal(c, frames, f)
	visTop := -(s.field.base - (s.field.scroll - s.field.over.Offset()))
	drawn := s.shift(view, frames, f, shapes, visTop, visTop+h, animate)
	// The field goes there on the change's own spring, from the frame
	// after its first, as the cards do. Swapping views keeps the cards
	// at their target poise while only the pill travels.
	switch {
	case !animate || first || s.field.now.depth == stageSwap:
		view.Jump("poise-x", poise.X)
		view.Jump("poise-y", poise.Y)
		s.field.poise = poise
	default:
		s.field.poise = stagePoint{
			X: s.along(view, "poise-x", s.field.poise.X, poise.X),
			Y: s.along(view, "poise-y", s.field.poise.Y, poise.Y),
		}
	}
	s.comeHome(view, animate)
	if s.field.hold {
		s.field.hold = false
		c.AnimationFrame()
	}
	live := make(map[string]bool, len(frames))
	for _, fr := range frames {
		live[fr.ID] = true
	}
	// The pill stands at the horizon, moving with the field as it
	// scrolls, but no further than near the window's top or foot: there it
	// stays, and what the field holds passes under it. A list's pill stands
	// at the window's foot and does not scroll; on swap it travels directly
	// between the field pill's screen position and the foot.
	s.field.viewH = h
	_, pulling := s.field.over.Step(c.Now(), animate)
	if pulling {
		c.AnimationFrame()
	}
	// The room the query's own ribbons take goes along with the change
	// that brings them, as the pill does.
	heldAbove, heldBelow := rl.heldAbove, rl.heldBelow
	if animate && !first {
		rl.heldAbove = s.along(view, "held-above", s.field.heldAbove, rl.heldAbove)
		rl.heldBelow = s.along(view, "held-below", s.field.heldBelow, rl.heldBelow)
	}
	s.field.heldAbove, s.field.heldBelow = rl.heldAbove, rl.heldBelow
	footY := h - foot - pill.H - rl.heldBelow
	var origin, free, fieldPy float32
	var away [2]bool
	if listed {
		origin = s.field.base - (s.field.scroll - s.field.over.Offset())
		fieldPy = s.field.fieldPillY
		if first {
			fieldPy = footY
		}
	} else {
		horizon := h/2 - s.field.poise.Y - (s.field.scroll - s.field.over.Offset())
		origin = horizon
		free = horizon - pill.H/2
		bound := h/2 - poise.Y - (s.field.scroll - s.field.over.Offset()) - pill.H/2
		away = [2]bool{bound > h-foot-pill.H-rl.below, bound < top+rl.above}
		fieldPy = max(top+rl.heldAbove, min(free, footY))
		s.field.fieldPillY = fieldPy
		// Stuck at one edge, what the pill offers toward it stands on
		// the side with room instead.
		if away[0] != away[1] {
			blocked := s.ribbons(c, k, w/2, targetPillW, pill.H, map[bool]int{true: 1}[away[1]])
			rl.places, rl.above, rl.below = blocked.places, blocked.above, blocked.below
		}
	}
	pin := float32(0)
	if listed {
		pin = 1
	}
	if animate {
		pin = s.along(view, "pin", s.field.pin, pin)
	}
	s.field.pin = pin
	py := fieldPy + (footY-fieldPy)*pin
	// While a change is made, the pill goes straight from where it was
	// drawn to where it stands once the change is done, on the change's
	// clock: where it stands is held at the window's top or foot by the
	// room its ribbons take and moved by the poise and the scroll, each
	// going along on its own, and so taken together they would leap or go
	// past its place and back.
	if animate && !first && s.field.changeT >= 0 {
		restY := h - foot - pill.H - heldBelow
		if !listed {
			free := h/2 - poise.Y - (s.scrollAt() - s.field.over.Offset()) - pill.H/2
			restY = max(top+heldAbove, min(free, restY))
		}
		py = s.along(view, "pill-y", s.field.pillY, restY)
	}
	s.field.pillY = py
	for side, put := range away {
		s.field.away[side] = map[bool]float32{true: 1}[put]
		if animate {
			s.field.away[side] = components.Follow(view, fmt.Sprint("away-", side), s.field.away[side], components.MotionQuick)
		}
	}
	s.field.offered = swapOffered(s.field.swapT)
	var drawnAbove, drawnBelow float32
	for _, fr := range drawn {
		if _, ok := s.field.shown[fr.ID]; !ok {
			continue
		}
		if fr.Rect.Y < 0 {
			drawnAbove = max(drawnAbove, -fr.Rect.Y)
		}
		if fr.Rect.Y+fr.Rect.H > 0 {
			drawnBelow = max(drawnBelow, fr.Rect.Y+fr.Rect.H)
		}
	}
	topFade, bottomFade := min(top, max(0, drawnAbove-origin)), min(top, max(0, origin+drawnBelow-h))
	// The window's edges shade what passes under them; as what stands
	// there comes and goes, the shade comes and goes with it.
	if animate {
		topFade = components.Follow(view, "top-fade", topFade, components.MotionQuick)
		bottomFade = components.Follow(view, "bottom-fade", bottomFade, components.MotionQuick)
	}
	// Staying at the window's top or foot, the pill and its conditions
	// stand out from what passes under them as macOS bars do: the field
	// blurs under them, more toward the window's edge, under its ground
	// fading away from that edge, coming in as the horizon goes past.
	var stuckTop, stuckFoot float32
	if listed {
		// A list's foot is a bar: what passes under the pill and all that
		// stands over it is shaded as under a pill stuck at the foot.
		stuckTop, stuckFoot = 0, origin+drawnBelow-s.field.viewBottom
	} else {
		stuckTop, stuckFoot = top+rl.heldAbove-free, free-footY
	}
	if stuckFoot > 0 {
		s.field.stuckFoot = true
	} else if stuckTop > 0 {
		s.field.stuckFoot = false
	}
	s.field.stuck = max(0, min(1, stuckTop/foot), min(1, stuckFoot/foot))
	if animate {
		s.field.stuck = components.Follow(view, "stuck", s.field.stuck, components.MotionQuick)
	}
	atFoot := s.field.stuckFoot
	baseEdge, rawExtra := py+pill.H+rl.heldBelow+10*k, max(0, rl.below-rl.heldBelow)
	// The backdrop reaches over what the pill offers on the side away
	// from the edge it is stuck at.
	side := 0
	if atFoot {
		baseEdge, rawExtra = h-(py-rl.heldAbove)+10*k, max(0, rl.above-rl.heldAbove)
		side = 1
	}
	baseEdge = max(0, baseEdge)
	if rawExtra > 0 {
		s.field.backdropSpan = rawExtra
	}
	ribbonTarget := float32(0)
	ribbonAnim := ui.Animation{Duration: components.MotionQuick, Ease: ui.EaseOut}
	if rawExtra > 0 {
		ribbonTarget = 1
		ribbonAnim = components.Settle(components.MotionScene)
	}
	ribbonFade := ribbonTarget
	if animate && !first {
		ribbonFade = view.AnimateTo("backdrop-ribbons", ribbonTarget, ribbonAnim)
	} else {
		view.Jump("backdrop-ribbons", ribbonTarget)
	}
	ribbonFade = max(0, min(1, ribbonFade)) * s.field.offered * (1 - s.field.away[side])
	ribbonSpan := s.field.backdropSpan
	if ribbonFade <= 0.001 {
		ribbonFade = 0
		if rawExtra == 0 {
			s.field.backdropSpan = 0
			ribbonSpan = 0
		}
	}
	edge := baseEdge + ribbonSpan*ribbonFade
	s.field.backdrop = edge
	view.Children(func() {
		// The top band drags the window where nothing of the field stands over it.
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		at := stagePoint{X: w/2 + s.field.poise.X, Y: origin}
		// Swapping views, only the pill travels: what goes out fades where
		// it stood, and what comes in rises where it will stand.
		swapping := s.field.from != nil && s.field.now.depth == stageSwap
		if swapping {
			at = stagePoint{X: w/2 + poise.X, Y: s.field.base - (s.field.scroll - s.field.over.Offset())}
		}
		s.field.origin = at
		s.bandNames(c, semantic, f, marks, at, m, animate)
		// Cards stand aside only while a sheet is dragged in a list, and as
		// they come back after.
		parting := s.field.parts != nil || s.field.parted
		s.field.parted = false
		for _, fr := range drawn {
			cd, ok := s.field.shown[fr.ID]
			if !ok {
				continue
			}
			from := at
			if swapping && !live[fr.ID] {
				from = s.field.swapFrom
			}
			// A card well out of the window is not built: nothing of it
			// shows, and a long list holds many. Its rows stand where the
			// card does, for the keys to go to, which bring it into view.
			if y := from.Y + fr.Rect.Y; y > h+h/2 || y+fr.Rect.H < -h/2 {
				if live[fr.ID] {
					s.unbuilt(cd, fr.Rect)
				}
				continue
			}
			if parting {
				from.X += s.partOf(view, fr.ID, animate)
			}
			s.cardView(c, semantic, fr, cd, s.field.shownShape[fr.ID], from, live[fr.ID], m, animate)
		}
		if s.field.drag != nil {
			s.arrangeTargets(c, semantic, drawn, at, m, animate)
		} else {
			// Let go of, what stood aside goes back.
			s.field.parts = nil
		}
		if topFade > 0 || bottomFade > 0 {
			ground := semantic.Surface.Background
			ui.Box(c).Key("field-edges").Absolute().Top(0).Left(0).Size(w, h).IgnoreInput(true).
				Draw(func(p *ui.Painter, r ui.Rect) {
					s.field.edges.Paint(p, r, ground, topFade, bottomFade)
				})
		}
		if s.field.stuck > 0 {
			s.pillBackdrop(c, semantic, w, edge, ribbonFade, atFoot, listed)
		}
		s.horizonView(c, semantic, stageRect{X: w/2 - pill.W/2, Y: py, W: pill.W, H: pill.H}, rl, w, k, animate)
	})
}

// pillBackdrop is what the stuck pill stands on, reaching from the
// window's top, or its foot, as far as edge. In list view it also catches
// pointer hits over the blurred strip and its expanded ribbons area so
// clicks do not pass through to cards underneath, clearing pill focus
// instead when no ribbon or pill control is hit.
func (s *State) pillBackdrop(c *ui.Context, semantic theme.Theme, w, edge, ribbonFade float32, foot, listed bool) {
	mask := ui.LinearGradient{From: ui.RGB(0, 0, 0), To: ui.Transparent, Start: 0.15, End: 1.0, Angle: 180}
	if foot {
		mask.Angle = 0
	}
	tintOpacity := s.field.stuck * (1 - 0.18*ribbonFade)
	for i, item := range []struct {
		m       ui.Material
		opacity float32
	}{
		{glass.Blur{Radius: 8, Mask: &mask}, s.field.stuck},
		{glass.ScrollEdge{Bottom: foot, Background: semantic.Surface.Background}, tintOpacity},
	} {
		b := ui.Box(c).Key(fmt.Sprint("pill-backdrop-", i)).Absolute().Left(0).Size(w, edge).
			Opacity(item.opacity).IgnoreInput(i > 0 || !listed).Material(item.m)
		if foot {
			b.Bottom(0)
		} else {
			b.Top(0)
		}
	}
}
