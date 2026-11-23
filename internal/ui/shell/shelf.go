package shell

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// A shelf is a box resting in a lane, almost all of it past the window's
// edge: its own edge and what stands out of it, as far as the desk shows
// at the right, are all that is in the window, so the lanes stay quiet and
// still say what they hold (field-and-desk.md §4.6). The pointer on it,
// something it takes dragged to it, or what keeps it out draws it out from
// the window's edge, its own rounded edge staying past it, so the pointer
// that drew it out stays on it; the pointer gone, it slides back. It is
// named above it at its window edge, by its icon and name, its count
// beside. What it holds
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
	// key keys it; icon and name mark it above; label tells it to the
	// screen reader; count is how many things it holds.
	key, name, label string
	icon             components.IconName
	count            int
	// right stands it at the window's right edge, not its left.
	right bool
	// width is its own width, and tail how far what it holds stands out of
	// it, toward the window's middle.
	width, tail float32
	// top is its top, or its foot where bottom; most is its greatest
	// height, and fixed holds it there, a container of its own size, not as
	// tall as what it holds, which it otherwise grows and shrinks to fit.
	top, most     float32
	bottom, fixed bool
	// inset sinks it, as a box things are put in; else it stands raised.
	inset bool
	// offset moves it with the field, as the field lane moves.
	offset float32
	// listing keeps its list drawn while it holds nothing, for what stands
	// in it besides, coming and going.
	listing bool
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
	// top is where it stood last, with its label over it, and height how
	// tall it stood.
	top, height float32
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
	// Resting past the window's edge, it is found by a drag from the
	// lane's width and more, about where it stood.
	reach := dropReach * k
	zone, near, fromNear, droppedNear := s.reachFor(c, sh.key+"-reach", sh.takes)
	zone.Top(st.top+24*k-reach).Height(st.height+2*reach).Width(s.laneShown()+reach).Translate(sh.offset, 0).Hidden(hidden)
	box := ui.Box(c).Key(sh.key).Absolute().Width(paper + tail).Clip().Hidden(hidden).Role(ui.RoleGroup).Label(sh.label)
	// What it holds stands out on the side of the window's middle, or
	// stands in from it where nothing does.
	inner := max(0, pad-tail)
	padTop, padFoot := 14*k, 16*k
	// Past the window's edge it keeps clear of its own rounded edge.
	padLeft, padRight := pad+radius, inner
	at, sign := float32(0), float32(-1)
	if sh.right {
		box.Right(0)
		zone.Right(0)
		padLeft, padRight = inner, pad+radius
		at, sign = tail, 1
	} else {
		box.Left(0)
		zone.Left(0)
	}
	if sh.menu != nil {
		box.ContextMenu(sh.menu)
	}
	if sh.drag != nil {
		s.dragFrom(box, sh.drag)
	}
	over, dropped, ok := dragOn(box, sh.takes)
	if !ok && droppedNear {
		dropped, ok = fromNear, true
	}
	over = over || near
	out := float32(0)
	if box.Hovered() || over || sh.out {
		out = 1
	}
	lit := map[bool]float32{true: 1}[over]
	if animate {
		settle := components.MotionPanel
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
	fill, stroke := components.Blend(ground, semantic.Accent.Muted, lit), components.Blend(rim, semantic.Border.Focused, lit)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		edge := ui.Rect{X: r.X + at, Y: r.Y, W: paper, H: r.H}
		p.Fill(edge, fill, radius)
		p.Stroke(edge, stroke, radius, 1)
	})
	// Fixed, it is as tall as it may be; else as tall as what it holds,
	// following it as things come and go, what it holds standing at its
	// foot, so that the slab grows and shrinks from it as smoothly as they
	// fold open and shut, and its icon above it rises and falls with it.
	most := max(0, sh.most)
	height := most
	box.Children(func() {
		content := ui.Column(c).Key("content").Absolute().Left(padLeft).Right(padRight).Bottom(padFoot).
			MaxHeight(max(0, most-padTop-padFoot)).Gap(6 * k)
		if sh.fixed {
			content.Top(padTop)
		} else {
			held := content.LayoutBounds().H
			height = min(most, held+padTop+padFoot)
			if animate && held > 0 {
				height = components.Follow(box, "height", height, components.MotionPanel)
			} else {
				box.Jump("height", height)
			}
		}
		lists, right := sh.count > 0 || sh.listing, sh.right
		content.Children(func() {
			if lists {
				fade := 16 * k
				sc := &st.scroll
				ui.Scroll(c).Key("held").MinHeight(0).Shrink(1).Scrollbars(false).TrackScroll(sc).DrawOver(func(p *ui.Painter, r ui.Rect) {
					// Scrolled, what it holds softens into its edges, within it:
					// the fades span the scroll where the box stands behind it.
					left := r.X
					if right {
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
	})
	box.Height(height)
	top := sh.top
	if sh.bottom {
		top -= height
	}
	box.Top(top)
	st.top, st.height = top-24*k, height
	label := s.laneIcon(c, semantic, sh.key+"-label", sh.icon, sh.name, sh.count, stageRect{Y: top}, !sh.right, k).Hidden(hidden)
	if sh.offset != 0 {
		label.Translate(sh.offset, 0)
	}
	return dropped, ok
}

// shelfHint is what an empty shelf says it is for, shown as much as out,
// kept clear of what stands out of it by tail. It unfolds as it comes, at
// the pace what went before it folds shut, so the shelf moves one way.
func shelfHint(c *ui.Context, semantic theme.Theme, text string, right bool, tail, out, k float32, animate bool) {
	pad := 10 * k
	room := ui.Box(c).Key("hint").WidthPercent(100).Shrink(0)
	room.Children(func() {
		hint := components.Label(c, text, theme.TextXSmall, semantic.Text.Muted).MaxLines(3).Opacity(out).Shrink(0)
		if right {
			hint.Padding(4*k, 4*k, 4*k, tail+pad)
		} else {
			hint.Padding(4*k, tail+pad, 4*k, 4*k)
		}
		if !animate {
			return
		}
		// New, it starts shut.
		if laid := room.LayoutBounds(); laid.W == 0 && laid.H == 0 {
			room.Jump("appear", 0)
		}
		if appear := components.Follow(room, "appear", 1, components.MotionPanel); appear < 1 {
			room.Height(hint.LayoutBounds().H * appear).Clip()
		}
	})
}
