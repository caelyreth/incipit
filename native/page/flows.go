package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/motion"
)

// A flow is a run of things laid one after another along one axis, each
// known by its identity across layouts: the page's units down the page, a
// table's rows down it and its columns across. Whatever lays a flow out
// tells it, as each paint finds a new layout, what it holds now, in order,
// where each stands and how far it reaches; the flow answers how each is
// drawn on its way there, so a change shows as a change, not a cut:
//
//   - What enters comes in, its extent from nothing or from what it
//     replaces, fading in as it does.
//   - What leaves goes, kept as it was last drawn where it stood among the
//     rest, its extent closing as it fades out.
//   - What leaves and what enters in its place, between the same
//     neighbours, cross: the newcomer's extent goes from the leavers' to
//     its own, the leavers fading out over it as it fades in.
//   - When the order of what stays changes, everything that stays glides
//     from where it was drawn to its new place.
//   - What changes shape springs to its new extent; any other change of
//     extent, as a line typed, takes effect at once.
//
// Only what its owner marks as moving enters and leaves so: text entering
// and leaving a page, as a line broken or joined, is typing and stands at
// once, unless it takes or gives the place of something that moves. A
// flow told nothing it had not told before stands as it is; the first
// time it is told, and without motion, everything stands where it is.

// flowIn is one thing a flow holds, as its owner tells it: its identity,
// its place and extent as laid out, without the flow's motion, its shape,
// whether it enters and leaves with motion, what is kept of it to draw it
// leaving, and the extent it enters from, when not from nothing.
type flowIn struct {
	id            doc.ID
	place, extent float32
	shape         int
	moves         bool
	kept          any
	from          float32
}

// flowItem is how one thing in a flow is drawn on its way: its extent,
// how strongly it shows, and how far it stands from its place, and where
// it was last drawn.
type flowItem struct {
	extent, alpha, offset motion.Spring
	target                float32
	shape                 int
	moves                 bool
	kept                  any
	drawn                 float32
}

// flowExit is something that left a flow, drawn as it was kept: after
// after (the flow's start where zero), at lead past the end of what stands
// there, its extent closing; or, where something took its place, over
// cover, within what cover is drawn at.
type flowExit struct {
	after, cover doc.ID
	lead, size   float32
	extent       motion.Spring
	alpha        motion.Spring
	kept         any
}

type flow struct {
	items map[doc.ID]*flowItem
	order []doc.ID
	exits []*flowExit
	known bool
}

// tell has the flow hold in, at now: what changed since it was last told
// starts on its way. It reports whether the flow changed at all.
func (f *flow) tell(in []flowIn, now time.Time, animate bool) bool {
	if f.items == nil {
		f.items = map[doc.ID]*flowItem{}
	}
	if f.known && animate && f.same(in) {
		for _, it := range in {
			f.items[it.id].settle(it, now)
		}
		return false
	}
	if !f.known || !animate {
		f.stand(in, now)
		return true
	}
	held := make(map[doc.ID]bool, len(in))
	for _, it := range in {
		held[it.id] = true
	}
	// Each change happens in a gap between two things that stay, named by
	// the first of them (zero at the flow's start).
	type gap struct {
		leaving []*flowItem
		after   doc.ID
		moves   bool
		entered []int
	}
	gaps := map[doc.ID]*gap{}
	gapOf := func(after doc.ID) *gap {
		g, ok := gaps[after]
		if !ok {
			g = &gap{}
			gaps[after] = g
		}
		return g
	}
	var after doc.ID
	for _, id := range f.order {
		if held[id] {
			after = id
			continue
		}
		it := f.items[id]
		delete(f.items, id)
		g := gapOf(after)
		g.after = after
		g.moves = g.moves || it.moves
		g.leaving = append(g.leaving, it)
	}
	after = 0
	for i, it := range in {
		if _, ok := f.items[it.id]; ok {
			after = it.id
			continue
		}
		g := gapOf(after)
		g.after = after
		g.moves = g.moves || it.moves
		g.entered = append(g.entered, i)
	}
	for _, g := range gaps {
		if !g.moves {
			for _, i := range g.entered {
				f.items[in[i].id] = standing(in[i], now)
			}
			continue
		}
		// What leaves where something moves goes as it was drawn, one
		// after another.
		var exits []*flowExit
		lead := float32(0)
		for _, it := range g.leaving {
			size := max(it.extent.Value(), 0)
			if size < 0.5 {
				continue
			}
			exits = append(exits, &flowExit{after: g.after, lead: lead, size: size, kept: it.kept,
				extent: motion.NewSpring(size, now), alpha: motion.NewUnitSpring(it.alpha.Value(), now)})
			lead += size
		}
		for k, i := range g.entered {
			it := in[i]
			from := it.from
			if k == 0 && len(exits) > 0 {
				// The first newcomer takes the leavers' place, from their
				// extent.
				from = lead
			}
			f.items[it.id] = &flowItem{extent: motion.NewSpring(from, now), alpha: motion.NewUnitSpring(0, now),
				offset: motion.NewSpring(0, now), target: it.extent, shape: it.shape, moves: it.moves, kept: it.kept, drawn: it.place}
		}
		for _, x := range exits {
			if len(g.entered) > 0 {
				x.cover = in[g.entered[0]].id
				x.extent = motion.NewSpring(0, now)
			}
			f.exits = append(f.exits, x)
		}
	}
	// The order of what stays changed: everything that stays glides from
	// where it was drawn.
	stays := map[doc.ID]bool{}
	var before []doc.ID
	for _, id := range f.order {
		if held[id] {
			before = append(before, id)
			stays[id] = true
		}
	}
	reordered, k := false, 0
	for _, it := range in {
		if stays[it.id] {
			reordered = reordered || before[k] != it.id
			k++
		}
	}
	for _, it := range in {
		item := f.items[it.id]
		if reordered && stays[it.id] {
			item.offset = motion.NewSpring(item.drawn-it.place, now)
		}
		item.settle(it, now)
	}
	f.order = f.order[:0]
	for _, it := range in {
		f.order = append(f.order, it.id)
	}
	return true
}

// same reports whether in holds what the flow holds, in its order.
func (f *flow) same(in []flowIn) bool {
	if len(in) != len(f.order) {
		return false
	}
	for i, it := range in {
		if f.order[i] != it.id {
			return false
		}
	}
	return true
}

// holds reports whether the flow holds ids, in their order, as last told.
func (f *flow) holds(ids []doc.ID) bool {
	if !f.known || len(ids) != len(f.order) {
		return false
	}
	for i, id := range ids {
		if f.order[i] != id {
			return false
		}
	}
	return true
}

// knows reports whether the flow holds id.
func (f *flow) knows(id doc.ID) bool {
	_, ok := f.items[id]
	return ok
}

// from has an item drawn at place now, gliding from there to where it
// stands, as a carried line let go lands.
func (f *flow) from(id doc.ID, place float32, now time.Time) {
	if it, ok := f.items[id]; ok {
		home := it.drawn - it.offset.Value()
		it.offset = motion.NewSpring(place-home, now)
		it.drawn = place
	}
}

// stand has everything in in stand where it is, and nothing leave.
func (f *flow) stand(in []flowIn, now time.Time) {
	clear(f.items)
	f.exits = nil
	f.order = f.order[:0]
	for _, it := range in {
		f.items[it.id] = standing(it, now)
		f.order = append(f.order, it.id)
	}
	f.known = true
}

func standing(it flowIn, now time.Time) *flowItem {
	return &flowItem{extent: motion.NewSpring(it.extent, now), alpha: motion.NewUnitSpring(1, now), offset: motion.NewSpring(0, now),
		target: it.extent, shape: it.shape, moves: it.moves, kept: it.kept, drawn: it.place}
}

// settle has an item that stays go to what it is now: a change of shape,
// or one under way, springs to its new extent; any other stands there.
func (it *flowItem) settle(in flowIn, now time.Time) {
	if in.shape != it.shape || it.extent.Moving() || it.alpha.Moving() {
		it.target = in.extent
	} else if in.extent != it.target {
		it.target = in.extent
		it.extent = motion.NewSpring(in.extent, now)
	}
	it.shape, it.moves, it.kept = in.shape, in.moves, in.kept
}

// step moves the flow on to now, and reports whether anything in it still
// moves.
func (f *flow) step(now time.Time, animate bool) bool {
	moving := false
	for _, it := range f.items {
		if !animate {
			it.extent = motion.NewSpring(it.target, now)
			it.alpha = motion.NewUnitSpring(1, now)
			it.offset = motion.NewSpring(0, now)
			continue
		}
		it.extent.Step(it.target, now, layoutSettle)
		it.alpha.Step(1, now, layoutSettle)
		it.offset.Step(0, now, layoutSettle)
		moving = moving || it.extent.Moving() || it.alpha.Moving() || it.offset.Moving()
	}
	kept := f.exits[:0]
	for _, x := range f.exits {
		if !animate {
			continue
		}
		x.extent.Step(0, now, layoutSettle)
		x.alpha.Step(0, now, hoverOut)
		if x.alpha.Value() > 0.002 || x.extent.Value() > 0.5 {
			kept = append(kept, x)
			moving = true
		}
	}
	f.exits = kept
	return moving
}

// drawn is how an item is drawn: its extent, how strongly it shows, and how
// far from its place; as laid out where the flow does not hold it.
func (f *flow) drawn(id doc.ID, extent float32) (float32, float32, float32) {
	it, ok := f.items[id]
	if !ok {
		return extent, 1, 0
	}
	return max(it.extent.Value(), 0), it.alpha.Value(), it.offset.Value()
}

// saw notes where an item was drawn, for a change to glide it from.
func (f *flow) saw(id doc.ID, place float32) {
	if it, ok := f.items[id]; ok {
		it.drawn = place
	}
}

// leaving are the exits after after, in their order.
func (f *flow) leaving(after doc.ID) []*flowExit {
	var out []*flowExit
	for _, x := range f.exits {
		if x.after == after {
			out = append(out, x)
		}
	}
	return out
}
