package core

import (
	"sync"
	"unicode/utf8"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// A place is where carets stand in document order: an inline text slot
// (stops at its grapheme boundaries), or a gap in a block slot where text
// can't stand: before and after block objects, and in an empty block slot.
// Objects are one unit: their own slots are entered only through their own
// stops, which the policies owning them declare.
type place struct {
	slot doc.SlotRef
	gap  bool
	at   int // the gap index
}

func (p place) start() doc.Pos {
	if p.gap {
		return doc.At(p.slot, p.at)
	}
	return doc.At(p.slot, 0)
}

func (p place) end(r doc.Reader) doc.Pos {
	if p.gap {
		return doc.At(p.slot, p.at)
	}
	n, _ := r.Node(p.slot.Node)
	return doc.At(p.slot, n.Slot(p.slot.Slot).Len())
}

var placeCache struct {
	sync.Mutex
	root   *doc.Node
	places []place
}

// placesOf lists the page's places in order, memoised per root pointer.
// TODO(perf): derive places per block from the presentation's stop maps
// once present publishes them, instead of walking the page.
func placesOf(r doc.Reader) []place {
	placeCache.Lock()
	defer placeCache.Unlock()
	if placeCache.root == r.Root() {
		return placeCache.places
	}
	var out []place
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		for i, s := range n.Slots() {
			ref := doc.SlotRef{Node: n.ID(), Slot: i}
			if s.IsInline() {
				out = append(out, place{slot: ref})
				continue
			}
			if s.Len() == 0 {
				out = append(out, place{slot: ref, gap: true})
				continue
			}
			for j, c := range s.Children() {
				if vocab.IsObject(c.Kind()) {
					if j == 0 || vocab.IsObject(s.Child(j-1).Kind()) {
						out = append(out, place{slot: ref, gap: true, at: j})
					}
					if j == s.Len()-1 {
						out = append(out, place{slot: ref, gap: true, at: j + 1})
					}
					continue
				}
				walk(c)
			}
		}
	}
	walk(r.Root())
	placeCache.root, placeCache.places = r.Root(), out
	return out
}

// placeIndex finds the place holding pos; a gap between two places
// matches the gap place, or the nearest place after it.
func placeIndex(places []place, pos doc.Pos) int {
	for i, p := range places {
		if p.slot == pos.Ref() && (!p.gap || p.at == pos.Offset) {
			return i
		}
	}
	return -1
}

// step moves one grapheme (or one inline node) in a slot, or to the
// neighbouring place at its edge.
func step(r doc.Reader, pos doc.Pos, dir vocab.Direction) (doc.Pos, bool) {
	n, ok := r.Node(pos.Node)
	if !ok {
		return pos, false
	}
	if pos.Slot < n.NumSlots() && n.Slot(pos.Slot).IsInline() {
		in := n.Slot(pos.Slot).Content()
		if dir > 0 && pos.Offset < in.Len() {
			return doc.At(pos.Ref(), nextBoundary(in, pos.Offset)), true
		}
		if dir < 0 && pos.Offset > 0 {
			return doc.At(pos.Ref(), prevBoundary(in, pos.Offset)), true
		}
		// The edge of an inline node's slot leaves it, beside the node.
		if parent, slot, inl := inlineParent(r, pos.Node); inl {
			pn, _ := r.Node(parent)
			off, _ := pn.Slot(slot).Content().NodeOffset(pos.Node)
			if dir > 0 {
				off++
			}
			return doc.Pos{Node: parent, Slot: slot, Offset: off}, true
		}
	}
	places := placesOf(r)
	i := placeIndex(places, pos)
	if i < 0 {
		return pos, false
	}
	j := i + int(dir)
	if j < 0 || j >= len(places) {
		return pos, false
	}
	if dir > 0 {
		return places[j].start(), true
	}
	return places[j].end(r), true
}

func nextBoundary(in *doc.Inline, off int) int {
	it, start, ok := in.ItemAt(off)
	if !ok || it.Node != nil {
		return off + 1
	}
	return start + vocab.NextGrapheme(it.Text, off-start)
}

func prevBoundary(in *doc.Inline, off int) int {
	it, start, ok := in.ItemAt(off - 1)
	if !ok || it.Node != nil {
		return off - 1
	}
	return start + vocab.PrevGrapheme(it.Text, off-start)
}

// unitTarget is where one unit of motion from pos lands in its slot, or
// beyond it.
// TODO(words): the words policy answers ByWord, BySentence and
// ByParagraph with UAX #29 boundaries; core's are a plain fallback: words
// are runs of one character class, sentences and paragraphs the slot.
func unitTarget(r doc.Reader, pos doc.Pos, unit vocab.Unit, dir vocab.Direction) (doc.Pos, bool) {
	n, ok := r.Node(pos.Node)
	inline := ok && pos.Slot < n.NumSlots() && n.Slot(pos.Slot).IsInline()
	switch unit {
	case vocab.ToPageEdge:
		places := placesOf(r)
		if len(places) == 0 {
			return pos, false
		}
		if dir > 0 {
			return places[len(places)-1].end(r), true
		}
		return places[0].start(), true
	case vocab.ToSlotEdge, vocab.BySentence, vocab.ByParagraph:
		if !inline {
			return step(r, pos, dir)
		}
		in := n.Slot(pos.Slot).Content()
		lineEdge := unit == vocab.ToSlotEdge
		if dir > 0 {
			if pos.Offset == in.Len() {
				return step(r, pos, dir)
			}
			end := in.Len()
			if lineEdge {
				if i := indexByte(in.Text(), '\n', pos.Offset); i >= 0 {
					end = i
				}
			}
			return doc.At(pos.Ref(), end), true
		}
		if pos.Offset == 0 {
			return step(r, pos, dir)
		}
		start := 0
		if lineEdge {
			if i := lastIndexByte(in.Text()[:pos.Offset], '\n'); i >= 0 {
				start = i + 1
			}
		}
		return doc.At(pos.Ref(), start), true
	case vocab.ByWord:
		if !inline {
			return step(r, pos, dir)
		}
		return wordTarget(n.Slot(pos.Slot).Content(), pos, dir, r)
	}
	return step(r, pos, dir)
}

func indexByte(s string, b byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// wordTarget skips spaces, then one run of word characters (or a single
// other character).
func wordTarget(in *doc.Inline, pos doc.Pos, dir vocab.Direction, r doc.Reader) (doc.Pos, bool) {
	text := in.Text()
	off := pos.Offset
	if (dir > 0 && off == len(text)) || (dir < 0 && off == 0) {
		return step(r, pos, dir)
	}
	class := func(i int) vocab.CharClass {
		var ru rune
		if dir > 0 {
			ru, _ = utf8.DecodeRuneInString(text[i:])
		} else {
			ru, _ = utf8.DecodeLastRuneInString(text[:i])
		}
		return vocab.ClassOf(ru)
	}
	adv := func(i int) int {
		if dir > 0 {
			_, n := utf8.DecodeRuneInString(text[i:])
			return i + n
		}
		_, n := utf8.DecodeLastRuneInString(text[:i])
		return i - n
	}
	more := func(i int) bool { return (dir > 0 && i < len(text)) || (dir < 0 && i > 0) }
	for more(off) && class(off)&vocab.ClassSpace != 0 {
		off = adv(off)
	}
	if more(off) && class(off)&vocab.ClassWord != 0 {
		for more(off) && class(off)&vocab.ClassWord != 0 {
			off = adv(off)
		}
	} else if more(off) {
		off = adv(off)
	}
	return doc.At(pos.Ref(), off), true
}

// move is logical motion; without Extend, a range collapses to its edge
// in the direction of motion first.
func move(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Move)
	r := c.Doc()
	sel := c.Selection()
	if in.Dir == 0 {
		in.Dir = vocab.Forward
	}
	if !in.Extend && sel.Kind == doc.SelectText && !sel.IsCollapsed() && in.Unit == vocab.ByGrapheme {
		from, to := sel.Ordered(r)
		edge := from
		if in.Dir > 0 {
			edge = to
		}
		return revealed(doc.Collapsed(edge))
	}
	if sel.Kind == doc.SelectNode && !in.Extend {
		// Leaving a selected object: to the gap or text on that side.
		head := c.Head().Pos
		if in.Dir > 0 {
			head.Offset++
		}
		if target, ok := settle(r, head, in.Dir); ok {
			return revealed(doc.Collapsed(doc.CaretAt(target)))
		}
		return revealed(doc.Collapsed(doc.CaretAt(head)))
	}
	head := c.Head()
	target, ok := unitTarget(r, head.Pos, in.Unit, in.Dir)
	if !ok {
		return policy.Swallow
	}
	caret := doc.CaretAt(target)
	if in.Extend && sel.Kind == doc.SelectText {
		return revealed(doc.TextRange(sel.Anchor, caret))
	}
	return revealed(doc.Collapsed(caret))
}

// settle moves a gap position onto the place it belongs to: a gap that is
// a place stays; otherwise the text next to it in dir.
func settle(r doc.Reader, pos doc.Pos, dir vocab.Direction) (doc.Pos, bool) {
	places := placesOf(r)
	if placeIndex(places, pos) >= 0 {
		return pos, true
	}
	n, ok := r.Node(pos.Node)
	if !ok || pos.Slot >= n.NumSlots() {
		return pos, false
	}
	s := n.Slot(pos.Slot)
	if dir > 0 && pos.Offset < s.Len() {
		if ref, ok := vocab.FirstTextSlot(s.Child(pos.Offset)); ok {
			return doc.At(ref, 0), true
		}
	}
	if dir < 0 && pos.Offset > 0 {
		if ref, ok := vocab.LastTextSlot(s.Child(pos.Offset - 1)); ok {
			cn, _ := r.Node(ref.Node)
			return doc.At(ref, cn.Slot(ref.Slot).Len()), true
		}
	}
	return pos, false
}

func revealed(s doc.Selection) policy.Result {
	return policy.Result{Selection: &s, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
}

// visualMove accepts the stop the renderer resolved from geometry.
func visualMove(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.VisualMove)
	target := in.Target
	if _, ok := c.Doc().Node(target.Pos.Node); !ok {
		return policy.Swallow
	}
	res := revealed(doc.Collapsed(target))
	if sel := c.Selection(); in.Extend && sel.Kind == doc.SelectText {
		s := doc.TextRange(sel.Anchor, target)
		res.Selection = &s
	}
	if in.Dir == vocab.Up || in.Dir == vocab.Down || in.Dir == vocab.PageUp || in.Dir == vocab.PageDown {
		goal := float64(in.GoalX)
		res.GoalX = &goal
	}
	return res
}

// placeCaret puts the caret where the pointer hit: a stop, an object, a word
// on a double click, the slot on a triple click.
func placeCaret(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Place)
	r := c.Doc()
	if in.Node != 0 {
		if _, ok := r.Node(in.Node); ok {
			return policy.Select(doc.NodeSelection(in.Node))
		}
	}
	stop := in.Stop
	if _, ok := r.Node(stop.Pos.Node); !ok {
		return policy.Swallow
	}
	sel := c.Selection()
	if in.Extend && sel.Kind == doc.SelectText {
		return policy.Select(doc.TextRange(sel.Anchor, stop))
	}
	switch {
	case in.Clicks == 2:
		n, _ := r.Node(stop.Pos.Node)
		if stop.Pos.Slot < n.NumSlots() && n.Slot(stop.Pos.Slot).IsInline() {
			in := n.Slot(stop.Pos.Slot).Content()
			back, _ := wordTarget(in, doc.At(stop.Pos.Ref(), stop.Pos.Offset), vocab.Backward, r)
			fwd, _ := wordTarget(in, back, vocab.Forward, r)
			return policy.Select(doc.TextRange(doc.CaretAt(back), doc.CaretAt(fwd)))
		}
	case in.Clicks >= 3:
		n, _ := r.Node(stop.Pos.Node)
		if stop.Pos.Slot < n.NumSlots() && n.Slot(stop.Pos.Slot).IsInline() {
			ref := stop.Pos.Ref()
			return policy.Select(doc.TextRange(doc.CaretAt(doc.At(ref, 0)), doc.CaretAt(doc.At(ref, n.Slot(stop.Pos.Slot).Len()))))
		}
	}
	return policy.Select(doc.Collapsed(stop))
}
