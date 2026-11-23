package page

import "github.com/caelyreth/neverland/internal/editor/doc"

// A text unit whose size changes, as a paragraph becomes a heading or a
// heading steps a level, grows or shrinks to it rather than jumping: its
// size and the room above it glide from where they were drawn, each a
// place in the page's springs, and the page lays the unit out at the size
// on its way, its marker with its text, so what stands below moves with
// it. Its weight and face change at once, as its group's do.

// resize is where a unit's text was drawn as its size changed: its size
// over the body and the room above it, in gaps.
type resize struct {
	size, gap float32
}

// resizedFrom notes that the unit laid out as was changes size, where it
// was drawn; a change before a paint took up the last one keeps where the
// first was drawn.
func (lc *layoutCache) resizedFrom(id doc.ID, was *unitLayout) {
	if _, ok := lc.resized[id]; ok {
		return
	}
	r := resize{size: was.size, gap: was.gap}
	if was.key.in.size != 0 {
		r.size, r.gap = was.key.in.size, was.key.in.gap
	}
	if lc.resized == nil {
		lc.resized = map[doc.ID]resize{}
	}
	lc.resized[id] = r
}

func sizeKey(id doc.ID, name string) springKey {
	return springKey{unit: id, name: name, of: factorValue}
}

// stepSizes moves each resizing unit's size and the room above it on to
// the paint's time, and asks the unit's layout for them until they arrive.
func (el *Element) stepSizes(asks map[doc.ID]objectIn) {
	lc := el.Editor.cache
	for id, r := range lc.resized {
		if el.sizing == nil {
			el.sizing = map[doc.ID]bool{}
		}
		if !el.sizing[id] {
			el.springs.jump(sizeKey(id, "size"), r.size)
			el.springs.jump(sizeKey(id, "gap"), r.gap)
			el.sizing[id] = true
		}
		delete(lc.resized, id)
	}
	for id := range el.sizing {
		u, ok := lc.units[id]
		if !ok || u.size == 0 {
			delete(el.sizing, id)
			continue
		}
		size := el.springs.follow(sizeKey(id, "size"), u.size, headingSettle)
		gap := el.springs.follow(sizeKey(id, "gap"), u.gap, headingSettle)
		if el.springs.settled(sizeKey(id, "size")) && el.springs.settled(sizeKey(id, "gap")) {
			delete(el.sizing, id)
			continue
		}
		in := askOf(asks, id)
		in.size, in.gap = size, gap
		asks[id] = in
	}
}
