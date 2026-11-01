package components

import (
	"math"
	"slices"
	"time"

	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

// pickedRow is a row of the list as Pick saw it: who it is, its geometry
// relative to the list as last laid out, and whether it is picked.
type pickedRow struct {
	id     string
	bounds ui.Rect
	picked bool
}

// pickRun is rows picked side by side: their ids, and the span from the
// first one's top to the last one's bottom, the space between them too.
type pickRun struct {
	ids   []string
	frame ui.Rect
}

// pickBlock is the fill of one run of picked rows. It follows the run,
// gliding its edges as the run grows, shrinks or moves; it fades in where a
// run appears apart from any other and out where one goes.
type pickBlock struct {
	// ids is the run it follows, as of the last frame.
	ids []string
	// top and bottom are its edges relative to the list; left and width
	// follow the rows at once.
	top, bottom motion.Spring
	left, width float32
	// presence is how far it is there: its strength, and how far in from
	// the rows' sides it stands short of them.
	presence motion.Spring
	leaving  bool
	// run is the run it follows this frame, or -1.
	run int
}

// pickInset is how far in from the rows' sides a block stands as it
// appears or leaves.
const pickInset = 6

// Pick records row, in the list's order, by id and whether it is picked.
// Call it for every row the list shows, and for what stands between rows
// too, with no id, not picked: rows picked side by side share one fill
// across the space between them, rounded only where the run begins and
// ends, and anything not picked parts it. As the picked change, the
// fill's edges glide to the runs' new ones.
func (t *SelectionTrack) Pick(row ui.Element, id string, picked bool) {
	if t == nil {
		return
	}
	t.picks = append(t.picks, pickedRow{id: id, bounds: t.relative(row), picked: picked && id != ""})
}

// Ring rings the rows ids, as those a menu open on the list acts on: one
// ring round each run of them, clear of the rows' edges.
func (t *SelectionTrack) Ring(ids []string) {
	if t != nil {
		t.ring = ids
	}
}

// Covers is how strongly the picked rows' fill is over row, as it was last
// drawn, 0 to 1: what goes over the fill on the row goes with it, so the
// two leave together.
func (t *SelectionTrack) Covers(row ui.Element) float32 {
	if t == nil {
		return 0
	}
	bounds := t.relative(row)
	strength := float32(0)
	for _, block := range t.blocks {
		if block.top.Value() < bounds.Y+bounds.H && block.bottom.Value() > bounds.Y {
			strength = max(strength, block.presence.Value())
		}
	}
	return strength
}

// Beneath fills row in color under the picked rows' fill, for a row that
// keeps a fill of its own while covered, as its hover: the picked fill
// hides it, and it is there as that fill goes, so the row goes straight
// from the one to the other. Call it each frame the row has it.
func (t *SelectionTrack) Beneath(row ui.Element, color ui.Color) {
	if t == nil || color.A == 0 {
		return
	}
	t.beneath = append(t.beneath, beneathRow{bounds: t.relative(row), color: color})
}

// beneathRow is a fill under the picked rows' (Beneath).
type beneathRow struct {
	bounds ui.Rect
	color  ui.Color
}

func (t *SelectionTrack) paintBeneath(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	for _, row := range t.beneath {
		row.bounds.X += rect.X
		row.bounds.Y += rect.Y
		p.Fill(row.bounds, row.color, style.Radius)
	}
}

// runs is the runs of the rows in, side by side, top down.
func (t *SelectionTrack) runs(in func(pickedRow) bool) []pickRun {
	var runs []pickRun
	open := false
	for _, row := range t.picks {
		if !in(row) || row.bounds.W <= 0 || row.bounds.H <= 0 {
			open = false
			continue
		}
		if !open {
			runs = append(runs, pickRun{frame: row.bounds})
			open = true
		}
		run := &runs[len(runs)-1]
		run.ids = append(run.ids, row.id)
		run.frame.H = row.bounds.Y + row.bounds.H - run.frame.Y
	}
	return runs
}

// shared counts the ids a and b share.
func shared(a, b []string) int {
	n := 0
	for _, id := range a {
		if slices.Contains(b, id) {
			n++
		}
	}
	return n
}

// followRuns gives each block the run it follows. A block follows the run
// it shares the most rows with, so a run that grows or shrinks keeps its
// block; a run whose rows were another's, split from it, starts from a copy
// of that one; and two blocks whose runs join both follow the joined run,
// meeting over the rows between. A block sharing no rows leaves, and a run
// sharing none with any block starts one of its own.
func (t *SelectionTrack) followRuns(runs []pickRun, now time.Time) {
	followed := make([]bool, len(runs))
	for index := range t.blocks {
		block := &t.blocks[index]
		block.run = -1
		if block.leaving {
			continue
		}
		best := 0
		for r, run := range runs {
			if n := shared(block.ids, run.ids); n > best {
				best, block.run = n, r
			}
		}
		if block.run < 0 {
			block.leaving = true
			continue
		}
		followed[block.run] = true
	}
	// A block whose rows went to several runs, as one let go from its
	// middle, is a fragment for each, parted where the runs part.
	origins := make([]int, len(t.blocks))
	for index := range origins {
		origins[index] = index
	}
	for r, run := range runs {
		if followed[r] {
			continue
		}
		from := -1
		for index := range t.blocks {
			if !t.blocks[index].leaving && shared(t.blocks[index].ids, run.ids) > 0 {
				from = index
				break
			}
		}
		if from >= 0 {
			split := t.blocks[from]
			split.ids = slices.Clone(split.ids)
			split.run = r
			t.blocks = append(t.blocks, split)
			origins = append(origins, from)
			continue
		}
		// It appears where it is, short of its full size, and grows.
		center := run.frame.Y + run.frame.H/2
		short := run.frame.H / 2 * 0.9
		t.blocks = append(t.blocks, pickBlock{
			top:      motion.NewSpring(center-short, now),
			bottom:   motion.NewSpring(center+short, now),
			presence: motion.NewUnitSpring(0, now),
			run:      r,
		})
		origins = append(origins, -1)
	}
	t.part(runs, origins, now)
	for index := range t.blocks {
		if block := &t.blocks[index]; block.run >= 0 {
			block.ids = slices.Clone(runs[block.run].ids)
			block.left, block.width = runs[block.run].frame.X, runs[block.run].frame.W
		}
	}
}

// part starts the fragments of a block that split, each following one of
// its runs, from where those runs part: between two of them the fragments
// meet at the middle of the space they leave, so the space opens from the
// row let go outward rather than as the fragments' far edges sweep past.
func (t *SelectionTrack) part(runs []pickRun, origins []int, now time.Time) {
	for origin := range t.blocks {
		if origins[origin] != origin || t.blocks[origin].run < 0 {
			continue
		}
		var fragments []int
		for index, from := range origins {
			if from == origin && t.blocks[index].run >= 0 {
				fragments = append(fragments, index)
			}
		}
		if len(fragments) < 2 {
			continue
		}
		slices.SortFunc(fragments, func(a, b int) int { return t.blocks[a].run - t.blocks[b].run })
		for n := 1; n < len(fragments); n++ {
			above, below := &t.blocks[fragments[n-1]], &t.blocks[fragments[n]]
			upper, lower := runs[above.run].frame, runs[below.run].frame
			middle := (upper.Y + upper.H + lower.Y) / 2
			above.bottom = motion.NewSpring(middle, now)
			below.top = motion.NewSpring(middle, now)
		}
	}
}

// paintPicks fills the picked rows' runs, each block gliding to its run,
// and rings what a menu open on them acts on.
func (t *SelectionTrack) paintPicks(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	now := p.Now()
	runs := t.runs(func(row pickedRow) bool { return row.picked })
	if !t.animate {
		t.blocks = t.blocks[:0]
		for r, run := range runs {
			t.blocks = append(t.blocks, pickBlock{
				ids:      run.ids,
				top:      motion.NewSpring(run.frame.Y, now),
				bottom:   motion.NewSpring(run.frame.Y+run.frame.H, now),
				left:     run.frame.X,
				width:    run.frame.W,
				presence: motion.NewUnitSpring(1, now),
				run:      r,
			})
		}
	} else {
		t.followRuns(runs, now)
	}
	moving := false
	for index := range t.blocks {
		block := &t.blocks[index]
		presence := float32(0)
		if block.run >= 0 {
			run := runs[block.run].frame
			block.top.Step(run.Y, now, selectionGlide)
			block.bottom.Step(run.Y+run.H, now, selectionGlide)
			presence = 1
		}
		block.presence.Step(presence, now, selectionFade)
		moving = moving || block.top.Moving() || block.bottom.Moving() || block.presence.Moving()
	}
	// Blocks that have faded out go, and of blocks that have joined into
	// one run and met, all but one.
	t.blocks = slices.DeleteFunc(t.blocks, func(block pickBlock) bool {
		return block.leaving && block.presence.Value() <= 0.001
	})
	for index := len(t.blocks) - 1; index > 0; index-- {
		block := t.blocks[index]
		if block.run < 0 {
			continue
		}
		for _, other := range t.blocks[:index] {
			if other.run == block.run &&
				abs32(other.top.Value()-block.top.Value()) < 0.5 &&
				abs32(other.bottom.Value()-block.bottom.Value()) < 0.5 {
				t.blocks = slices.Delete(t.blocks, index, index+1)
				break
			}
		}
	}
	if moving {
		p.AnimationFrame()
	}
	// The blocks wholly there are one shape, so where they overlap, as two
	// meeting, they fill once; those fading are each their own.
	var whole ui.Path
	filled := false
	for _, block := range t.blocks {
		a := block.presence.Value()
		if a <= 0.001 {
			continue
		}
		inset := pickInset * (1 - a)
		share := ui.Rect{
			X: rect.X + block.left + inset, Y: rect.Y + block.top.Value(),
			W: block.width - 2*inset, H: block.bottom.Value() - block.top.Value(),
		}
		if a >= 0.999 {
			roundedRect(&whole, share, style.Radius)
			filled = true
			continue
		}
		var path ui.Path
		roundedRect(&path, share, style.Radius)
		p.FillPath(&path, style.Picked.Alpha(a))
	}
	if filled {
		p.FillPath(&whole, style.Picked)
	}
}

// paintRing rings the runs of the rows Ring named, standing RingOutset out
// from them, its corners rounded about theirs: a point-wide line on whole
// pixels, clear of the fill, which fades in and out where it stands, so it
// reads as the menu's mark without drawing the eye from the menu.
//
// It fades in from nothing as the menu opens, the window drawing on while
// the system's menu shows; a ring that moves to other rows fades in anew
// there, rather than jumping across.
func (t *SelectionTrack) paintRing(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	now := p.Now()
	target := float32(0)
	if len(t.ring) > 0 {
		target = 1
		if !slices.Equal(t.ring, t.ringShown) {
			t.ringShown = slices.Clone(t.ring)
			t.ringAmount = motion.NewUnitSpring(0, now)
		}
		t.ringRuns = t.ringRuns[:0]
		for _, run := range t.runs(func(row pickedRow) bool { return slices.Contains(t.ring, row.id) }) {
			t.ringRuns = append(t.ringRuns, run.frame)
		}
	} else {
		t.ringShown = nil
	}
	if !t.animate {
		t.ringAmount = motion.NewUnitSpring(target, now)
	}
	a := t.ringAmount.Step(target, now, selectionFade)
	if t.ringAmount.Moving() {
		p.AnimationFrame()
	}
	if a <= 0.001 {
		t.ringRuns = t.ringRuns[:0]
		return
	}
	out := style.RingOutset
	scale := max(1, p.Scale())
	snap := func(v float32) float32 { return float32(math.Round(float64(v*scale))) / scale }
	width := max(1/scale, snap(1))
	for _, run := range t.ringRuns {
		left, top := snap(rect.X+run.X-out), snap(rect.Y+run.Y-out)
		right, bottom := snap(rect.X+run.X+run.W+out), snap(rect.Y+run.Y+run.H+out)
		// The stroke lies just inside the box: on whole pixels, it is whole
		// rows of them, not a blur across one more.
		box := ui.Rect{X: left, Y: top, W: right - left, H: bottom - top}
		p.Stroke(box, style.Ring.Alpha(a), style.Radius+out, width)
	}
}

func abs32(v float32) float32 { return max(v, -v) }

// roundedRect adds r, its corners rounded by radius, to path. Every rect it
// adds winds the same way, so a path of several fills their union.
func roundedRect(path *ui.Path, r ui.Rect, radius float32) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	radius = max(0, min(radius, r.W/2, r.H/2))
	k := radius * (1 - 0.5522848)
	right, bottom := r.X+r.W, r.Y+r.H
	path.MoveTo(r.X+radius, r.Y)
	path.LineTo(right-radius, r.Y)
	path.CubeTo(right-k, r.Y, right, r.Y+k, right, r.Y+radius)
	path.LineTo(right, bottom-radius)
	path.CubeTo(right, bottom-k, right-k, bottom, right-radius, bottom)
	path.LineTo(r.X+radius, bottom)
	path.CubeTo(r.X+k, bottom, r.X, bottom-k, r.X, bottom-radius)
	path.LineTo(r.X, r.Y+radius)
	path.CubeTo(r.X, r.Y+k, r.X+k, r.Y, r.X+radius, r.Y)
	path.Close()
}
