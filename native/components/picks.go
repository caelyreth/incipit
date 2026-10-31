package components

import (
	"cmp"
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

// pickBlock is the frame round one run of picked rows. It follows the run,
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

// pickInset is how far in from the rows' sides a frame stands as it
// appears or leaves.
const pickInset = 6

// Pick records row, in the list's order, by id and whether it is picked.
// Call it for every row the list shows, and for what stands between rows
// too, with no id, not picked: rows picked side by side share one dashed
// frame round them all, and anything not picked parts it in two. As the
// picked change, the frames' edges glide to the runs' new ones. The frame
// goes over the rows, so it leaves their own fills, as their hover or the
// open one's, as they are.
func (t *SelectionTrack) Pick(row ui.Element, id string, picked bool) {
	if t == nil {
		return
	}
	bounds := row.LayoutBounds()
	bounds.X -= t.list.X
	bounds.Y -= t.list.Y
	t.picks = append(t.picks, pickedRow{id: id, bounds: bounds, picked: picked && id != ""})
}

// runs is the runs of picked rows, top down.
func (t *SelectionTrack) runs() []pickRun {
	var runs []pickRun
	open := false
	for _, row := range t.picks {
		if !row.picked || row.bounds.W <= 0 || row.bounds.H <= 0 {
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
			presence: motion.NewSpring(0, now),
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

// paintPicks frames the picked rows' runs, each block gliding to its run.
func (t *SelectionTrack) paintPicks(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	now := p.Now()
	runs := t.runs()
	if !t.animate {
		t.blocks = t.blocks[:0]
		for r, run := range runs {
			t.blocks = append(t.blocks, pickBlock{
				ids:      run.ids,
				top:      motion.NewSpring(run.frame.Y, now),
				bottom:   motion.NewSpring(run.frame.Y+run.frame.H, now),
				left:     run.frame.X,
				width:    run.frame.W,
				presence: motion.NewSpring(1, now),
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
	// Blocks that overlap, as two meeting, are one frame round them both.
	type frame struct {
		top, bottom, left, width, presence float32
	}
	var frames []frame
	for _, block := range t.blocks {
		if a := block.presence.Value(); a > 0.001 {
			frames = append(frames, frame{block.top.Value(), block.bottom.Value(), block.left, block.width, a})
		}
	}
	slices.SortFunc(frames, func(a, b frame) int { return cmp.Compare(a.top, b.top) })
	// On whole pixels, so the dashes stay sharp as the edges glide.
	scale := max(1, p.Scale())
	snap := func(v float32) float32 { return float32(math.Round(float64(v*scale))) / scale }
	for index := 0; index < len(frames); index++ {
		f := frames[index]
		for index+1 < len(frames) && frames[index+1].top <= f.bottom {
			next := frames[index+1]
			f.bottom, f.presence = max(f.bottom, next.bottom), max(f.presence, next.presence)
			index++
		}
		out := style.PickedOutset
		inset := pickInset*(1-f.presence) - out
		left, right := snap(rect.X+f.left+inset), snap(rect.X+f.left+f.width-inset)
		top, bottom := snap(rect.Y+f.top-out), snap(rect.Y+f.bottom+out)
		box := ui.Rect{X: left, Y: top, W: right - left, H: bottom - top}
		// Its corners round about the rows' own.
		DashedOutline(p, box, style.Radius+out, 1, style.Picked.Alpha(f.presence))
	}
}

func abs32(v float32) float32 { return max(v, -v) }
