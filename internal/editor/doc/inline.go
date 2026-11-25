package doc

import "strings"

// Item is one piece of inline content: a text run with its marks, or an
// inline node, which occupies exactly one position.
type Item struct {
	Text  string
	Marks *MarkSet
	Node  *Node
}

// TextItem is text carrying marks.
func TextItem(text string, marks *MarkSet) Item { return Item{Text: text, Marks: marks} }

// NodeItem is an inline node.
func NodeItem(n *Node) Item { return Item{Node: n} }

// Len is how many positions the item occupies.
func (it Item) Len() int {
	if it.Node != nil {
		return 1
	}
	return len(it.Text)
}

// MarkRun gives a stretch of positions one mark set; an inline node's run
// carries nil.
type MarkRun struct {
	Len   int
	Marks *MarkSet
}

type run struct {
	n     int // text bytes; 1 for a node
	marks *MarkSet
	node  *Node
}

// Inline is the content of an inline slot: its text, held once, and the
// runs that divide it. Offsets count text bytes, and one for each inline
// node, whose text is not part of the slot's.
//
// A slot is one chunk however long (§6): a keystroke in 64 KiB of text
// copies it in about 11 µs, against milliseconds for what reads the text
// after, which a chunk tree would not change. The API (Each, TextRange)
// hands out substrings, so a tree could replace it without changing
// callers.
type Inline struct {
	text string
	runs []run
	size int
}

var emptyInline = &Inline{}

// NewInline builds inline content from items. It is not normalised.
func NewInline(items ...Item) *Inline {
	if len(items) == 0 {
		return emptyInline
	}
	var b strings.Builder
	in := &Inline{runs: make([]run, 0, len(items))}
	for _, it := range items {
		if it.Node != nil {
			in.runs = append(in.runs, run{n: 1, node: it.Node})
			in.size++
			continue
		}
		b.WriteString(it.Text)
		in.runs = append(in.runs, run{n: len(it.Text), marks: it.Marks})
		in.size += len(it.Text)
	}
	in.text = b.String()
	return in
}

// Len is the number of positions in the slot.
func (in *Inline) Len() int {
	if in == nil {
		return 0
	}
	return in.size
}

// Text is the slot's text, without inline nodes.
func (in *Inline) Text() string {
	if in == nil {
		return ""
	}
	return in.text
}

// NodeByte stands for an inline node in Flat text: one byte, as the node
// is one position. It is a control character no text holds.
const NodeByte = '\x1a'

// Flat is the slot's text aligned with its offsets: each inline node is
// NodeByte. Analyzers and motion index it by slot offset.
func (in *Inline) Flat() string {
	if in == nil {
		return ""
	}
	if in.size == len(in.text) {
		return in.text
	}
	var b strings.Builder
	b.Grow(in.size)
	in.Each(func(_ int, it Item) bool {
		if it.Node != nil {
			b.WriteByte(NodeByte)
		} else {
			b.WriteString(it.Text)
		}
		return true
	})
	return b.String()
}

// FlatRange is Flat text in [from, to).
func (in *Inline) FlatRange(from, to int) string {
	var b strings.Builder
	for _, it := range in.Slice(from, to) {
		if it.Node != nil {
			b.WriteByte(NodeByte)
		} else {
			b.WriteString(it.Text)
		}
	}
	return b.String()
}

// Each calls fn with every item and the offset it starts at. Text items
// are substrings of the slot's text.
func (in *Inline) Each(fn func(at int, it Item) bool) {
	if in == nil {
		return
	}
	at, byteAt := 0, 0
	for _, r := range in.runs {
		var it Item
		if r.node != nil {
			it = Item{Node: r.node}
		} else {
			it = Item{Text: in.text[byteAt : byteAt+r.n], Marks: r.marks}
			byteAt += r.n
		}
		if !fn(at, it) {
			return
		}
		at += r.n
	}
}

// Slice returns the items covering [from, to), split at the edges.
func (in *Inline) Slice(from, to int) []Item {
	var out []Item
	in.Each(func(at int, it Item) bool {
		end := at + it.Len()
		if end <= from || at >= to {
			return at < to
		}
		if it.Node == nil {
			lo, hi := max(from, at)-at, min(to, end)-at
			it.Text = it.Text[lo:hi]
		}
		out = append(out, it)
		return true
	})
	return out
}

// TextRange is the text in [from, to), without inline nodes.
func (in *Inline) TextRange(from, to int) string {
	var b strings.Builder
	for _, it := range in.Slice(from, to) {
		b.WriteString(it.Text)
	}
	return b.String()
}

// ItemAt returns the item holding offset at and where it starts. At an
// item boundary it is the item that starts there.
func (in *Inline) ItemAt(off int) (Item, int, bool) {
	var found Item
	var start int
	ok := false
	in.Each(func(at int, it Item) bool {
		if off >= at && off < at+it.Len() {
			found, start, ok = it, at, true
			return false
		}
		return true
	})
	return found, start, ok
}

// MarksAt returns the marks text typed at off takes: those of the text
// run before it, or of the run after it at the slot's start. Nil means
// none.
func (in *Inline) MarksAt(off int) *MarkSet {
	var before, first *MarkSet
	in.Each(func(at int, it Item) bool {
		if at >= off {
			if off == 0 && it.Node == nil {
				first = it.Marks
			}
			return false
		}
		if it.Node != nil {
			before = nil
		} else {
			before = it.Marks
		}
		return true
	})
	if off == 0 {
		return first
	}
	return before
}

// Nodes calls fn with every inline node and its offset.
func (in *Inline) Nodes(fn func(at int, n *Node) bool) {
	in.Each(func(at int, it Item) bool {
		if it.Node == nil {
			return true
		}
		return fn(at, it.Node)
	})
}

// NodeOffset returns where the inline node id stands.
func (in *Inline) NodeOffset(id ID) (int, bool) {
	off, ok := 0, false
	in.Nodes(func(at int, n *Node) bool {
		if n.id == id {
			off, ok = at, true
			return false
		}
		return true
	})
	return off, ok
}

func (in *Inline) replace(from, to int, items []Item) (*Inline, []Item) {
	removed := in.Slice(from, to)
	all := make([]Item, 0, len(in.runs)+len(items))
	all = append(all, in.Slice(0, from)...)
	all = append(all, items...)
	all = append(all, in.Slice(to, in.Len())...)
	return NewInline(all...), removed
}

// remark gives each text run in [from, to) the set f returns for it.
func (in *Inline) remark(from, to int, f func(*MarkSet) *MarkSet) (*Inline, []MarkRun) {
	var before []MarkRun
	all := make([]Item, 0, len(in.runs)+2)
	all = append(all, in.Slice(0, from)...)
	for _, it := range in.Slice(from, to) {
		before = append(before, MarkRun{Len: it.Len(), Marks: it.Marks})
		if it.Node == nil {
			it.Marks = f(it.Marks)
		}
		all = append(all, it)
	}
	all = append(all, in.Slice(to, in.Len())...)
	return NewInline(all...), before
}

// restoreMarks gives the runs in [from, from+total) the sets listed.
func (in *Inline) restoreMarks(from int, sets []MarkRun) (*Inline, []MarkRun) {
	total := 0
	for _, s := range sets {
		total += s.Len
	}
	var before []MarkRun
	all := make([]Item, 0, len(in.runs)+len(sets))
	all = append(all, in.Slice(0, from)...)
	at := from
	for _, s := range sets {
		for _, it := range in.Slice(at, at+s.Len) {
			before = append(before, MarkRun{Len: it.Len(), Marks: it.Marks})
			if it.Node == nil {
				it.Marks = s.Marks
			}
			all = append(all, it)
		}
		at += s.Len
	}
	all = append(all, in.Slice(from+total, in.Len())...)
	return NewInline(all...), before
}

// normalized merges adjacent text runs with the same marks and drops empty
// ones, giving every text run a set from empty.
func (in *Inline) normalized(empty *MarkSet) *Inline {
	if in == nil || in.Len() == 0 {
		return emptyInline
	}
	canonical := true
	for i, r := range in.runs {
		if r.node == nil && (r.n == 0 || r.marks == nil ||
			(i > 0 && in.runs[i-1].node == nil && in.runs[i-1].marks == r.marks)) {
			canonical = false
			break
		}
	}
	if canonical {
		return in
	}
	out := &Inline{text: in.text, size: in.size}
	for _, r := range in.runs {
		if r.node == nil {
			if r.n == 0 {
				continue
			}
			if r.marks == nil {
				r.marks = empty
			}
			if last := len(out.runs) - 1; last >= 0 && out.runs[last].node == nil && out.runs[last].marks == r.marks {
				out.runs[last].n += r.n
				continue
			}
		}
		out.runs = append(out.runs, r)
	}
	return out
}
