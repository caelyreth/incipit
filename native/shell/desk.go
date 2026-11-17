package shell

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The desk is where sheets are written (references/field-and-desk.md):
// the open sheets side by side in one strip that scrolls sideways, beside
// the panes, each column as tall as the window. Where they stand follows
// from what is open and in what order, never from where anyone put them:
// one sheet alone fills the desk, its page at reading measure; two share
// it by halves; more go on along the strip, two or three to a window as
// the window holds them (stage.PerWindow). Pickers stand among them, each
// after the sheet it follows (desk_columns.go, desk_pickers.go).
//
// Everything on the strip moves on one motion (deskMotion): a column
// made opens out where it stands as its neighbours make room, one closed
// folds away as they close up, a sheet coming in front is travelled to,
// and the strip's own moves, to a column pressed in the overview or one
// it comes to rest near, go the same way. A touchpad's swipe, Shift and
// the wheel, or a drag on the strip moves along it freely, stretching
// past its ends; where it comes to rest by itself near a column's start,
// the column pins to the desk's edge, as a magnet takes what comes near it
// (magnet). The sheet in front is always brought into view. There is no
// zoom: the strip is how the desk is gone through.
//
// The sheets' columns are desk_sheets.go, the column in front
// desk_front.go, and the overview at the foot desk_scrub.go.

const (
	// flingCarry is how far a drag let go carries the strip on, in seconds
	// of the speed it was let go at.
	flingCarry = float32(0.25)
	// columnGap is the line between the strip's columns, in the window's
	// points whatever the type's size.
	columnGap = 1
	// magnetReach is how near a column's start the strip, come to rest,
	// is pinned to it, at the reference type size; magnetQuiet how long
	// the strip's input is quiet before it counts as come to rest.
	magnetReach = 72
	magnetQuiet = 90 * time.Millisecond
	// magnetHeld is how long after the last input fingers still on the
	// touchpad keep the strip from being pinned, should they lift without
	// a word.
	magnetHeld = 400 * time.Millisecond
)

type deskState struct {
	// strip is where the strip is scrolled to, and over how far past its
	// ends it is pulled; viewport is its window, spans its columns as last
	// laid out, from the first column's start.
	strip    ui.ScrollState
	over     scrollfx.Overscroll
	viewport ui.Rect
	spans    []stage.Span
	// inputs counts the strip's input; touching is whether fingers are on
	// the touchpad; still is where the strip stood the build before, and
	// armed whether it was scrolled by hand since it was last pinned
	// (magnet).
	inputs   int
	touching bool
	still    float32
	armed    bool
	// front is the sheet in front as the frame began (keepDesk).
	front string
	// bring is the key of a column to bring into view as it is built, at
	// once with bringAtOnce (bringInView).
	bring       string
	bringAtOnce bool
	// at is the key of the column in front; held is that of the column
	// the keyboard was in as the strip was last built, and holding where
	// it is as it is being built; pointer is where the pointer was in the
	// window, and pointed whether it moved since; claim is a sheet to give
	// the keyboard to as it is built (desk_front.go).
	at, held, holding string
	pointer           ui.Point
	pointed           bool
	claim             string
	// scrub is the scrubber's (desk_scrub.go).
	scrub scrubState
	// columns are the desk's open sheet columns as last built (desk_columns.go).
	columns []deskColumn
	// built is whether the desk was built before.
	built bool
}

// deskView draws the desk where the scene stands it: its strip of
// columns, its overview, and the statistics of the sheet in front. Seen
// from the field it stands in its corner, what it holds taking no input
// (leanDesk).
func (s *State) deskView(c *ui.Context, semantic theme.Theme, layout theme.Layout, chrome float32, scene stage.Scene, w, h float32, animate bool) {
	s.keepDesk()
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	gutter := layout.ContentGutter * k
	area := ui.Column(c).Key("desk").Absolute().Top(0).Left(0).Size(w, h).Background(semantic.Surface.Background)
	s.leanDesk(area, scene.Desk, scene.Upright)
	on := s.OnDesk()
	if on {
		s.followPointer(area)
	} else {
		s.deskDrops(semantic, area, scene.Desk.Scale, animate)
	}
	area.Children(func() {
		ui.Column(c).Key("desk-content").Grow(1).MinHeight(0).WidthPercent(100).IgnoreInput(!on).Children(func() {
			// All of it, on the desk or resting in its block, so it never
			// changes its layout on the way between.
			s.deskStrip(c, semantic, animate)
			s.deskFoot(c, semantic, gutter, animate)
			if chrome > 0.001 {
				s.statistics(c, semantic, layout, gutter, chrome)
			}
			if len(s.Open) == 0 {
				s.deskEmpty(c, semantic, area, w, h, k, animate)
			}
		})
	})
}

// deskDrops makes the desk, resting in its lane at scale, take sheets
// dropped on it, or the tray whole: they open on it beside the sheet in
// front, the field staying in front. It is ringed while they are over it.
func (s *State) deskDrops(semantic theme.Theme, area ui.Element, scale float32, animate bool) {
	over, dropped, ok := dragOn(area, takes{sheets: true})
	if ok {
		for _, id := range s.carried(dropped) {
			s.Library.OpenSheet(id, true)
		}
		s.delivered(dropped)
	}
	lit := map[bool]float32{true: 1}[over]
	if animate {
		lit = components.Follow(area, "drop", lit, components.MotionQuick)
	}
	if lit > 0.001 {
		width := 2 / max(scale, 0.01)
		area.DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Stroke(r, semantic.Accent.Foreground.Alpha(lit), 10*width, width)
		})
	}
}

// deskEmpty draws the empty desk placeholder so that resting in the lane
// and returning with the last sheet closed share the desk's single path.
func (s *State) deskEmpty(c *ui.Context, semantic theme.Theme, area ui.Element, w, h, k float32, animate bool) {
	rest := s.deskRest(w, h)
	inv := max(w, 1) / max(rest.W, 1)
	visible := max(0, w-rest.X) * inv
	radius := theme.DefaultControls.Scaled(s.Settings.UIFontSize).Radius * inv
	hover := components.HoverAmount(area, area.Hovered(), animate)
	line := components.Blend(semantic.Border.Default, semantic.Accent.Foreground, hover)
	box := ui.Row(c).Key("desk-empty").Absolute().Top(0).Left(0).Size(w, h).
		PaddingEnd(max(0, w-visible)).Justify(ui.Center).AlignItems(ui.Center).
		Transition(emptyTransition(animate)).
		Draw(func(p *ui.Painter, r ui.Rect) { components.DashedOutline(p, r, radius, inv, line) })
	box.Children(func() {
		ui.Icon(c, components.IconNewSheet.SVG()).FontSize(18 * k * inv).TextColor(semantic.Icon.Muted)
	})
}

func emptyTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	spring := components.Spring(components.MotionScene)
	gone := &ui.Motion{Opacity: 0}
	return ui.ElementTransition{Animation: &spring, Enter: gone, Exit: gone}
}

// deskStrip draws the strip's columns.
func (s *State) deskStrip(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	strip := ui.ScrollHorizontal(c).Key("desk-strip").Grow(1).MinHeight(0).WidthPercent(100).
		Scrollbars(false).TrackScroll(&s.desk.strip)
	viewport := strip.LayoutBounds()
	// The desk resized, the column in front stays in view where it was.
	if viewport.W != s.desk.viewport.W && s.desk.viewport.W > 0 {
		s.bringInView(s.desk.at, true)
	}
	s.desk.viewport = viewport
	// The strip goes where it is moved, and stretches past its ends as the
	// lists do; its input is counted, and the fingers followed, for the
	// magnet.
	strip.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputScroll {
			return false
		}
		s.desk.inputs++
		s.desk.armed = true
		s.desk.touching = scrollfx.Phase(ev.Precise, ev.Momentum) == scrollfx.Direct && (ev.DX != 0 || ev.DY != 0)
		return components.OverscrollPanel.Handle(&s.desk.over, ev, &s.desk.strip, scrollfx.Horizontal,
			s.desk.viewport.W, scale)
	})
	s.magnet(strip, k, animate)
	wasEmpty := len(s.desk.columns) == 0
	columns := s.deskColumns()
	editor := s.editorWidth(strip, columns, viewport.W, k, animate)
	strip.Children(func() {
		// The flow is at least as wide as the desk, its lines between the
		// columns and the room after them its own (room). A column folding
		// away takes no room, so as one closes the flow keeps its room a
		// while, easing back with it, rather than the strip being cut back
		// under it at once (components.Settled).
		natural := float32(0)
		if n := len(s.desk.spans); n > 0 {
			natural = s.desk.spans[n-1].End()
		}
		flow := ui.Row(c).Key("desk-flow").FillHeight().AlignItems(ui.Stretch)
		extent := components.Settled(flow, "extent", natural, components.MotionEnter, animate)
		flow.MinWidth(max(viewport.W, extent))
		s.desk.over.Apply(c, flow, animate)
		// The columns' places are counted from where the first stands.
		origin := flow.LayoutBounds().X
		s.pan(flow)
		pages := make(map[string]Page, len(s.Open))
		for _, page := range s.Open {
			pages[page.ID] = page
		}
		flow.Children(func() {
			s.desk.spans = s.desk.spans[:0]
			s.desk.holding = ""
			for i, col := range columns {
				s.openColumn(c, semantic, pages[col.sheet], editor, origin, !wasEmpty, animate)
				s.room(c, semantic, "room-"+col.key(), i+1, i == len(columns)-1)
			}
			s.keyboardMoved()
		})
	})
}

// room is the room after the desk's column at, before the one after it:
// between two columns the line between them, after the last the desk's
// empty room on, which may grow.
func (s *State) room(c *ui.Context, semantic theme.Theme, key string, at int, last bool) {
	room := ui.Box(c).Key(key).Width(columnGap).Shrink(0).FillHeight().Label("Desk")
	line := semantic.Border.Variant
	switch {
	case last:
		// Its title line is the window's title bar, as the columns' heads
		// are.
		room.Grow(1).Children(func() {
			if at > 0 {
				ui.Box(c).Width(columnGap).FillHeight().Background(line)
			}
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		})
	default:
		room.Background(line)
	}
}

// pan has a drag on el move the strip, and a drag let go carry it on, to
// pin at a column's start it comes near. It reports a drag let go, whose
// release is no click.
func (s *State) pan(el ui.Element) bool {
	if dx, _, ok := el.Dragged(); ok && dx != 0 {
		s.stripAt(s.desk.strip.X - dx)
	}
	vx, _, ok := el.DragEnded()
	if ok {
		s.settleStrip(s.desk.strip.X - vx*flingCarry)
	}
	return ok
}

// stripAt puts the strip at x at once, as a drag moves it, taking it from
// any glide on its way.
func (s *State) stripAt(x float32) {
	s.desk.strip.ScrollTo(max(0, min(x, s.desk.strip.MaxX)), 0, ui.Animation{Duration: 1})
}

// magnet pins the strip to a column's start once, scrolled by hand, it has
// come to rest by itself near one: its input quiet a moment, the fingers
// off the touchpad, and it no longer moving. Nowhere near one, it stays
// where it stopped. While it moves it is left alone: the pull is felt only
// at the end, all at once, as a magnet takes what has come near it. Where
// the desk moves the strip itself, it goes where it is sent.
func (s *State) magnet(strip ui.Element, k float32, animate bool) {
	x := s.desk.strip.X
	// The value reaches the input so far magnetQuiet after the last.
	inputs := float32(s.desk.inputs)
	quiet := strip.AnimateTo("magnet-quiet", inputs, ui.Animation{Duration: magnetQuiet}) == inputs
	// Fingers that lifted without saying so are taken to have lifted a
	// while after the last input.
	held := s.desk.touching &&
		strip.AnimateTo("magnet-held", inputs, ui.Animation{Duration: magnetHeld}) != inputs
	moved := x != s.desk.still
	s.desk.still = x
	if moved || !s.desk.armed || !quiet || held || strip.Dragging() {
		return
	}
	s.desk.armed = false
	if rest, ok := stage.NearRest(s.desk.spans, s.desk.strip.MaxX, x, magnetReach*k); ok && math.Abs(float64(rest-x)) > 0.5 {
		s.desk.strip.ScrollTo(rest, 0, deskMotion(animate))
	}
}

// deskMotion is how the strip moves of its own accord: travelling to a
// column, or to rest, as far as it goes, in the time its columns take to
// open out or fold away (columnTransition), so the two arrive together.
func deskMotion(animate bool) ui.Animation {
	if !animate {
		return ui.Animation{Duration: 1}
	}
	return components.Spring(components.MotionEnter)
}

// editorWidth is a sheet's column's width on a desk inner wide holding
// columns: the desk shared by as many sheets as it shows to a window
// (stage.PerWindow), less the lines between. Between counts it goes
// smoothly, as sheets open and close or the desk is resized across where
// they meet, on el.
func (s *State) editorWidth(el ui.Element, columns []deskColumn, inner, k float32, animate bool) float32 {
	page := s.Settings.Prose().Column() + 2*sheetPad*k
	n := float32(stage.PerWindow(len(columns), inner, page, columnGap))
	if animate {
		n = el.AnimateTo("columns-per-window", n, deskMotion(animate))
	}
	return stage.ColumnWidth(inner, n, columnGap)
}

// column is one of the strip's columns, w wide and as tall as the strip,
// whether it is in front (desk_front.go), and where across the window it
// shows, as the strip stands now, as laid out the frame before; its place
// is counted for the magnet and the scrubber, and it is brought into view
// where asked (bringInView). origin is where the strip's columns start in
// the window.
func (s *State) column(c *ui.Context, of deskColumn, w, origin float32, enter, animate bool) (ui.Element, bool, ui.Rect) {
	key := of.key()
	col := ui.Column(c).Key(key).Width(w).Shrink(0).FillHeight().Transition(columnTransition(enter, animate))
	var shown ui.Rect
	if b := col.LayoutBounds(); b.W > 0 {
		sp := stage.Span{X: b.X - origin, W: b.W}
		s.desk.spans = append(s.desk.spans, sp)
		// The strip goes on between frames, and stretches past its ends.
		over, _ := s.desk.over.Offset()
		shown = ui.Rect{X: s.desk.viewport.X + sp.X - s.desk.strip.X + over, W: sp.W}
	}
	if s.desk.bring == key {
		motion := deskMotion(animate && !s.desk.bringAtOnce)
		// Reveal the column itself, which survives a rebuild before layout;
		// a temporary child would be gone once this request is cleared and
		// the view builds again.
		col.ScrollIntoView(ui.ScrollIntoViewOptions{Inline: ui.ScrollNearest, Animation: &motion})
		s.desk.bring = ""
	}
	return col, s.inFront(col, of), shown
}

// columnTransition is how the strip's columns come and go: opening out
// where they stand, from nothing across, as they fade and rise to their
// size, and folding away the same way, those beside them making room and
// closing up on the same motion, a little past and back. What a column
// holds keeps its layout as it opens or folds, clipped, so nothing in it
// is set anew on the way.
func columnTransition(enter, animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	spring := components.Bouncy(components.MotionEnter)
	gone := &ui.Motion{Collapse: true, Opacity: 0, Scale: 0.96}
	tr := ui.ElementTransition{Animation: &spring, Position: true, Exit: gone}
	if enter {
		tr.Enter = gone
	}
	return tr
}

// columnDim is how far what a column holds shows while another is in
// front: the one in front reads whole, the rest stepped back.
const columnDim = 0.55

// columnShows is how far what a column holds shows, as it comes in front
// or goes from it. A column alone on the desk is always in front.
func (s *State) columnShows(col ui.Element, front, animate bool) float32 {
	target := float32(columnDim)
	if front || len(s.desk.columns) == 1 {
		target = 1
	}
	if !animate {
		return target
	}
	return components.Follow(col, "column-shows", target, components.MotionNormal)
}

// bringInView has the strip bring the column keyed key into view as it is
// next built, which may be once it joins: travelling there, or at once.
func (s *State) bringInView(key string, atOnce bool) {
	if key != "" {
		s.desk.bring, s.desk.bringAtOnce = key, atOnce
	}
}

// statistics are the sheet in front's counts, floating at the desk's foot
// on its right, gutter in, as far shown as chrome.
func (s *State) statistics(c *ui.Context, semantic theme.Theme, layout theme.Layout, gutter, chrome float32) {
	if s.Statistics == "" || s.ActivePage == "" {
		return
	}
	scale := s.Settings.UIFontSize
	ui.Row(c).Key("statistics").Absolute().Right(gutter).Bottom(0).
		Height(layout.Status * scale / theme.DefaultUIFontSize).Opacity(chrome).IgnoreInput(true).
		AlignItems(ui.Center).Children(func() {
		components.Label(c, s.Statistics, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
	})
}
