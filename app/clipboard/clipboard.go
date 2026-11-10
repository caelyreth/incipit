// Package clipboard carries Neverland slices across page instances as a
// versioned wire value. It is a serialization boundary, not a second
// document model: the slice remains vocab.Slice.
package clipboard

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/store"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Format is the clipboard representation of a Neverland slice. Plain text
// remains the fallback for every other application.
const Format = "application/x-neverland-slice+json"

// MaxBytes bounds a slice read from the clipboard before it is decoded.
const MaxBytes = 4 << 20

const version = 1

var (
	// ErrInvalid rejects a slice record this build cannot read.
	ErrInvalid = errors.New("clipboard: invalid slice")
)

type wireSlice struct {
	Version   int        `json:"version"`
	OpenStart int        `json:"openStart,omitempty"`
	OpenEnd   int        `json:"openEnd,omitempty"`
	Inline    []wireItem `json:"inline,omitempty"`
	Nodes     []wireNode `json:"nodes,omitempty"`
}

type wireNode struct {
	Kind  doc.Kind   `json:"kind"`
	Attrs []byte     `json:"attrs,omitempty"`
	Slots []wireSlot `json:"slots,omitempty"`
}

type wireSlot struct {
	Blocks   bool       `json:"blocks,omitempty"`
	Inline   []wireItem `json:"inline,omitempty"`
	Children []wireNode `json:"children,omitempty"`
}

type wireItem struct {
	Text  string     `json:"text,omitempty"`
	Marks []wireMark `json:"marks,omitempty"`
	Node  *wireNode  `json:"node,omitempty"`
}

type wireMark struct {
	Kind  doc.MarkKind `json:"kind"`
	Attrs []byte       `json:"attrs,omitempty"`
}

// Encode serializes a slice. The result contains only stable kind,
// attribute-field and mark-field numbers; node ids and order keys never
// leave the page.
func Encode(slice vocab.Slice) ([]byte, error) {
	w := wireSlice{Version: version, OpenStart: slice.OpenStart, OpenEnd: slice.OpenEnd}
	var err error
	if w.Inline, err = encodeItems(slice.Inline); err != nil {
		return nil, err
	}
	w.Nodes = make([]wireNode, len(slice.Nodes))
	for i, n := range slice.Nodes {
		if n == nil {
			return nil, fmt.Errorf("%w: nil node", ErrInvalid)
		}
		w.Nodes[i], err = encodeNode(n)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(w)
}

func encodeNode(n *doc.Node) (wireNode, error) {
	w := wireNode{Kind: n.Kind(), Attrs: store.EncodeAttrs(n.Attrs())}
	w.Slots = make([]wireSlot, n.NumSlots())
	for i, slot := range n.Slots() {
		ws := wireSlot{Blocks: !slot.IsInline()}
		if slot.IsInline() {
			var items []doc.Item
			slot.Content().Each(func(_ int, item doc.Item) bool {
				items = append(items, item)
				return true
			})
			var err error
			if ws.Inline, err = encodeItems(items); err != nil {
				return wireNode{}, err
			}
		} else {
			ws.Children = make([]wireNode, slot.Len())
			for j, child := range slot.Children() {
				var err error
				if ws.Children[j], err = encodeNode(child); err != nil {
					return wireNode{}, err
				}
			}
		}
		w.Slots[i] = ws
	}
	return w, nil
}

func encodeItems(items []doc.Item) ([]wireItem, error) {
	out := make([]wireItem, len(items))
	for i, item := range items {
		if item.Node != nil {
			n, err := encodeNode(item.Node)
			if err != nil {
				return nil, err
			}
			out[i].Node = &n
			continue
		}
		out[i].Text = item.Text
		if item.Marks != nil {
			out[i].Marks = make([]wireMark, 0, item.Marks.Len())
			for _, m := range item.Marks.Marks() {
				out[i].Marks = append(out[i].Marks, wireMark{Kind: m.Kind, Attrs: store.EncodeAttrs(m.Attrs)})
			}
		}
	}
	return out, nil
}

// Decode reads a slice record. Data is bounded and malformed records are
// rejected before any node reaches a transaction.
func Decode(data []byte) (vocab.Slice, error) {
	if len(data) > MaxBytes {
		return vocab.Slice{}, fmt.Errorf("%w: %d bytes", ErrInvalid, len(data))
	}
	var w wireSlice
	if err := json.Unmarshal(data, &w); err != nil {
		return vocab.Slice{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if w.Version != version {
		return vocab.Slice{}, fmt.Errorf("%w: version %d", ErrInvalid, w.Version)
	}
	marks := doc.NewInterner()
	inline, err := decodeItems(w.Inline, marks)
	if err != nil {
		return vocab.Slice{}, err
	}
	nodes := make([]*doc.Node, len(w.Nodes))
	for i, n := range w.Nodes {
		if nodes[i], err = decodeNode(n, marks); err != nil {
			return vocab.Slice{}, err
		}
	}
	return vocab.Slice{Nodes: nodes, OpenStart: w.OpenStart, OpenEnd: w.OpenEnd, Inline: inline}, nil
}

func decodeNode(w wireNode, marks *doc.Interner) (*doc.Node, error) {
	spec, ok := vocab.Schema().Kind(w.Kind)
	if !ok {
		return nil, fmt.Errorf("%w: kind %d", ErrInvalid, w.Kind)
	}
	attrs, err := store.DecodeAttrs(w.Attrs, knownFields(spec))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	slots := make([]doc.Slot, len(w.Slots))
	for i, ws := range w.Slots {
		if ws.Blocks {
			children := make([]*doc.Node, len(ws.Children))
			for j, child := range ws.Children {
				if children[j], err = decodeNode(child, marks); err != nil {
					return nil, err
				}
			}
			slots[i] = doc.BlockSlot(children...)
			continue
		}
		items, err := decodeItems(ws.Inline, marks)
		if err != nil {
			return nil, err
		}
		slots[i] = doc.InlineSlot(items...)
	}
	return doc.NewNode(w.Kind, attrs, slots...), nil
}

func decodeItems(items []wireItem, marks *doc.Interner) ([]doc.Item, error) {
	out := make([]doc.Item, len(items))
	for i, item := range items {
		if item.Node != nil {
			n, err := decodeNode(*item.Node, marks)
			if err != nil {
				return nil, err
			}
			out[i] = doc.NodeItem(n)
			continue
		}
		var set *doc.MarkSet
		if item.Marks != nil {
			ms := make([]doc.Mark, len(item.Marks))
			for j, m := range item.Marks {
				spec, ok := vocab.Schema().Mark(m.Kind)
				if !ok {
					return nil, fmt.Errorf("%w: mark %d", ErrInvalid, m.Kind)
				}
				attrs, err := store.DecodeAttrs(m.Attrs, knownFields(spec))
				if err != nil {
					return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
				}
				ms[j] = doc.Mark{Kind: m.Kind, Attrs: attrs}
			}
			set = marks.Intern(ms)
		}
		out[i] = doc.TextItem(item.Text, set)
	}
	return out, nil
}

func knownFields(spec any) func(uint16) bool {
	switch spec := spec.(type) {
	case *doc.KindSpec:
		return func(num uint16) bool {
			for _, f := range spec.Fields {
				if f.Num == num {
					return true
				}
			}
			return false
		}
	case *doc.MarkSpec:
		return func(num uint16) bool {
			for _, f := range spec.Fields {
				if f.Num == num {
					return true
				}
			}
			return false
		}
	}
	return func(uint16) bool { return false }
}
