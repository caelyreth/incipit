package doc

import (
	"slices"
	"sort"
	"strings"
	"sync/atomic"
)

// A block slot's children are a slice while they are few, and a
// persistent chunked vector above chunkAt (§6): chunks of about chunkSize
// children, so a transaction replacing one child of a page with thousands
// copies that child's chunk and the list of chunks, not every child.

const (
	// chunkAt is how many children a slot holds as one slice; past it they
	// are chunked, and below half of it they are one slice again.
	chunkAt = 256
	// chunkSize is how many children a chunk is made with.
	chunkSize = 64
)

// chunk is a run of a slot's children. tx is the transaction that made it,
// which may change it in place; any other copies it first.
type chunk struct {
	nodes []*Node
	tx    uint64
}

// chunked is a large slot's children: its chunks and, for each, how many
// children it and those before it hold. flat is them as one slice, made
// when first asked for (Children).
type chunked struct {
	chunks []*chunk
	ends   []int
	flat   atomic.Pointer[[]*Node]
}

// txSerial numbers transactions, so a chunk knows the one that owns it.
var txSerial atomic.Uint64

func chunkNodes(nodes []*Node, tx uint64) *chunked {
	c := &chunked{}
	c.chunks, c.ends = appendChunks(nil, nil, nodes, tx)
	return c
}

// appendChunks appends nodes to chunks as chunks of chunkSize, the last
// taking up to twice that, with their running ends.
func appendChunks(chunks []*chunk, ends []int, nodes []*Node, tx uint64) ([]*chunk, []int) {
	end := 0
	if len(ends) > 0 {
		end = ends[len(ends)-1]
	}
	for len(nodes) > 0 {
		n := min(len(nodes), chunkSize)
		if len(nodes) <= 2*chunkSize {
			n = len(nodes)
		}
		end += n
		chunks = append(chunks, &chunk{nodes: nodes[:n:n], tx: tx})
		ends = append(ends, end)
		nodes = nodes[n:]
	}
	return chunks, ends
}

func (c *chunked) len() int {
	if len(c.ends) == 0 {
		return 0
	}
	return c.ends[len(c.ends)-1]
}

// find is the chunk holding child i and where it starts. Chunks are near
// chunkSize, so the guess i/chunkSize is close: it steps from there, and
// searches only when far off.
func (c *chunked) find(i int) (int, int) {
	k := min(i/chunkSize, len(c.ends)-1)
	for range 4 {
		start := 0
		if k > 0 {
			start = c.ends[k-1]
		}
		switch {
		case i < start:
			k--
		case i >= c.ends[k]:
			k++
		default:
			return k, start
		}
	}
	k = sort.SearchInts(c.ends, i+1)
	start := 0
	if k > 0 {
		start = c.ends[k-1]
	}
	return k, start
}

func (c *chunked) at(i int) *Node {
	k, start := c.find(i)
	return c.chunks[k].nodes[i-start]
}

// all is the children as one slice, made once.
func (c *chunked) all() []*Node {
	if p := c.flat.Load(); p != nil {
		return *p
	}
	out := make([]*Node, 0, c.len())
	for _, ch := range c.chunks {
		out = append(out, ch.nodes...)
	}
	c.flat.Store(&out)
	return out
}

// own is a copy whose list of chunks is the transaction's to change; the
// chunks stay shared until it writes one.
func (c *chunked) own() *chunked {
	return &chunked{chunks: slices.Clone(c.chunks), ends: slices.Clone(c.ends)}
}

// set replaces child i in place, copying its chunk first where another
// transaction made it. The chunked children must be the transaction's
// (own).
func (c *chunked) set(tx uint64, i int, n *Node) {
	k, start := c.find(i)
	ch := c.chunks[k]
	if ch.tx != tx {
		ch = &chunk{nodes: slices.Clone(ch.nodes), tx: tx}
		c.chunks[k] = ch
	}
	ch.nodes[i-start] = n
	if p := c.flat.Load(); p != nil {
		// A slice already handed out within the transaction sees the
		// child replaced where it stands, as a small slot's does.
		(*p)[i] = n
	}
}

// splice replaces del children from i with ins, in chunks of its own: the
// chunks it spans are made again, the others kept. A span left short takes
// in the chunk after it, so chunks don't dwindle.
func (c *chunked) splice(tx uint64, i, del int, ins []*Node) *chunked {
	n := c.len()
	k1, start := c.find(min(i, n-1))
	k2 := k1
	if del > 0 {
		k2, _ = c.find(i + del - 1)
	}
	size := c.ends[k2] - start - del + len(ins)
	for size < chunkSize/2 && k2+1 < len(c.chunks) {
		k2++
		size += len(c.chunks[k2].nodes)
	}
	span := make([]*Node, 0, size)
	for k := k1; k <= k2; k++ {
		span = append(span, c.chunks[k].nodes...)
	}
	at := i - start
	span = slices.Concat(span[:at:at], ins, span[at+del:])
	out := &chunked{
		chunks: make([]*chunk, k1, len(c.chunks)+len(span)/chunkSize+1),
		ends:   make([]int, k1, len(c.chunks)+len(span)/chunkSize+1),
	}
	copy(out.chunks, c.chunks[:k1])
	copy(out.ends, c.ends[:k1])
	out.chunks, out.ends = appendChunks(out.chunks, out.ends, span, tx)
	shift := len(ins) - del
	for k := k2 + 1; k < len(c.chunks); k++ {
		out.chunks = append(out.chunks, c.chunks[k])
		out.ends = append(out.ends, c.ends[k]+shift)
	}
	return out
}

// indexOrd is where the child with order key ord stands, by its key.
func (c *chunked) indexOrd(ord string) (int, bool) {
	k := sort.Search(len(c.chunks), func(k int) bool {
		nodes := c.chunks[k].nodes
		return strings.Compare(nodes[len(nodes)-1].ord, ord) >= 0
	})
	if k == len(c.chunks) {
		return c.len(), false
	}
	nodes := c.chunks[k].nodes
	j, ok := slices.BinarySearchFunc(nodes, ord, func(n *Node, ord string) int {
		return strings.Compare(n.ord, ord)
	})
	start := 0
	if k > 0 {
		start = c.ends[k-1]
	}
	return start + j, ok
}
