package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A shelf is a box resting in a lane, almost all of it past the window's
// edge: its own edge and what stands out of it, as far as the desk shows
// at the right, are all that is in the window, so the lanes stay quiet and
// still say what they hold (field-and-desk.md §4.6). The pointer on it,
// something it takes dragged to it, or what keeps it out draws it out from
// the window's edge, its own rounded edge staying past it, so the pointer
// that drew it out stays on it; the pointer gone, it slides back. It is
// named above it at its window edge, its count beside. What it holds
// scrolls within it, softening into its edges, and the words in it show
// only as it comes out. The kept box (kept.go) and the tray (tray.go) are
// shelves.

// The kept box's measures at the reference type size.
const (
	// shelfWidth is the kept box's own width, and shelfTail how far its
	// bookmarks stand out of it.
	shelfWidth, shelfTail = 232, 14
)

// shelf is what a shelf is and does.
type shelf struct {
	// key keys it; name names it above it, and label to the screen
	// reader; count is how many things it holds.
	key, name, label string
	count            int
	// right stands it at the window's right edge, not its left.
	right bool
	// width is its own width, and tail how far what it holds stands out of
	// it, toward the window's middle.
	width, tail float32
	// top is its top, or its middle where centred; most is its greatest
	// height, and fixed holds it there, a container of its own size, not
	// as tall as what it holds.
	top, most      float32
	centred, fixed bool
	// inset sinks it, as a box things are put in; else it stands raised.
	inset bool
	// offset moves it with the field, as the field lane moves.
	offset float32
	// takes is what it takes dropped on it, and out keeps it drawn out.
	takes takes
	out   bool
	// menu is its own menu, and drag what it is dragged as whole: nil for
	// none.
	menu func(*ui.Menu)
	drag shellDrag
}

// shelfState is what a shelf keeps between frames: how far what it holds
// is scrolled, and the fades of its edges.
type shelfState struct {
	scroll ui.ScrollState
	edges  components.EdgeFade
}

// shelfView draws sh, keeping its state in st: list draws what it holds,
// which scrolls, and foot what stands under it, each told how far the
// shelf is out, to show their words as much. It returns what was dropped
// on it.
func (s *State) shelfView(c *ui.Context, semantic theme.Theme, sh shelf, st *shelfState, k float32, hidden, animate bool, list, foot func(out float32)) (shellDrag, bool) {
	paper, tail, pad, radius := sh.width, sh.tail, 10*k, 12*k
	ground, rim := semantic.Surface.Elevated, semantic.Border.Default
	if sh.inset {
		ground = semantic.Surface.Inset
	}
	box := ui.Column(c).Key(sh.key).Absolute().Top(sh.top).Width(paper + tail).MaxHeight(max(0, sh.most)).
		Gap(6 * k).Clip().Hidden(hidden).Role(ui.RoleGroup).Label(sh.label)
	if sh.fixed {
		box.Height(max(0, sh.most))
	}
	// What it holds stands out on the side of the window's middle, or
	// stands in from it where nothing does.
	inner := max(0, pad-tail)
	at, sign := float32(0), float32(-1)
	if sh.right {
		box.Right(0).Padding(14*k, pad+radius, 16*k, inner)
		at, sign = tail, 1
	} else {
		box.Left(0).Padding(14*k, inner, 16*k, pad+radius)
	}
	if sh.centred {
		box.TranslatePercent(0, -50)
	}
	if sh.menu != nil {
		box.ContextMenu(sh.menu)
	}
	if sh.drag != nil {
		box.Drag(sh.drag)
	}
	over, dropped, ok := dragOn(box, sh.takes)
	out := float32(0)
	if box.Hovered() || over || sh.out {
		out = 1
	}
	lit := map[bool]float32{true: 1}[over]
	if animate {
		settle := components.MotionSettle
		if out > 0 {
			settle = components.MotionPanel
		}
		out = components.Follow(box, "out", out, settle)
		lit = components.Follow(box, "lit", lit, components.MotionQuick)
	}
	// At rest it stands past the edge but for what the desk shows; out,
	// all of it but its rounded edge is in the window.
	rest := paper + tail - s.laneShown()
	box.Translate(sh.offset+sign*(rest+(radius-rest)*out), 0)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		edge := ui.Rect{X: r.X + at, Y: r.Y, W: paper, H: r.H}
		p.Fill(edge, components.Blend(ground, semantic.Accent.Muted, lit), radius)
		p.Stroke(edge, components.Blend(rim, semantic.Border.Focused, lit), radius, 1)
	})
	box.Children(func() {
		if sh.count > 0 {
			fade := 16 * k
			sc := &st.scroll
			ui.Scroll(c).Key("held").MinHeight(0).Shrink(1).Scrollbars(false).TrackScroll(sc).DrawOver(func(p *ui.Painter, r ui.Rect) {
				// Scrolled, what it holds softens into its edges, within it:
				// the fades span the scroll where the box stands behind it.
				left := r.X
				if sh.right {
					left = r.X + at - inner
				}
				x0, x1 := max(r.X, left), min(r.X+r.W, left+paper-pad-radius)
				st.edges.Paint(p, ui.Rect{X: x0, Y: r.Y, W: x1 - x0, H: r.H}, ground, min(sc.Y, fade), min(sc.MaxY-sc.Y, fade))
			}).Children(func() {
				ui.Column(c).Key("list").WidthPercent(100).Gap(6 * k).Children(func() { list(out) })
			})
		}
		foot(out)
	})
	top := sh.top
	if sh.centred {
		top -= box.Bounds().H / 2
	}
	label := laneLabel(c, semantic, sh.key+"-label", sh.name, sh.count, stage.Rect{Y: top}, !sh.right, k).Hidden(hidden)
	if sh.offset != 0 {
		label.Translate(sh.offset, 0)
	}
	return dropped, ok
}

// shelfHint is what an empty shelf says it is for, shown as much as out,
// kept clear of what stands out of it by tail.
func shelfHint(c *ui.Context, semantic theme.Theme, text string, right bool, tail, out, k float32) {
	pad := 10 * k
	hint := components.Label(c, text, theme.TextXSmall, semantic.Text.Muted).MaxLines(3).Opacity(out)
	if right {
		hint.Padding(4*k, 4*k, 4*k, tail+pad)
	} else {
		hint.Padding(4*k, tail+pad, 4*k, 4*k)
	}
}
