// Package store encodes snapshots as node rows and back: one row per node
// held in a block slot (and the root), inline nodes inside their owner's
// row, text stored once, attributes field-numbered with unknown fields
// preserved. Saving is a pointer diff of snapshots; loading builds nodes
// bottom-up and validates them.
package store

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/caelyreth/neverland/editor/doc"
)

// Format is the encoding version pages.format records.
const Format = 1

// ErrCorrupt reports a row that does not decode.
var ErrCorrupt = errors.New("store: corrupt row")

// Wire types of an encoded value.
const (
	wireInt  = 0
	wireBool = 1
	wireText = 2
	wireList = 3
)

type writer struct{ b []byte }

func (w *writer) uvarint(v uint64) { w.b = binary.AppendUvarint(w.b, v) }
func (w *writer) varint(v int64)   { w.b = binary.AppendVarint(w.b, v) }
func (w *writer) bytes(s string)   { w.uvarint(uint64(len(s))); w.b = append(w.b, s...) }

type reader struct {
	b   []byte
	err error
}

func (r *reader) fail() {
	if r.err == nil {
		r.err = ErrCorrupt
	}
}

func (r *reader) uvarint() uint64 {
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.fail()
		r.b = nil
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) varint() int64 {
	v, n := binary.Varint(r.b)
	if n <= 0 {
		r.fail()
		r.b = nil
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) bytes() string {
	n := r.uvarint()
	if uint64(len(r.b)) < n {
		r.fail()
		r.b = nil
		return ""
	}
	s := string(r.b[:n])
	r.b = r.b[n:]
	return s
}

// EncodeAttrs encodes attributes: per field its number, wire type and
// payload. Unknown (raw) fields are written back as they were read.
func EncodeAttrs(a doc.Attrs) []byte {
	w := &writer{}
	encodeAttrs(w, a)
	return w.b
}

func encodeAttrs(w *writer, a doc.Attrs) {
	for _, f := range a.Fields() {
		w.uvarint(uint64(f.Num))
		v := f.Value
		switch v.Kind() {
		case doc.ValueInt:
			w.uvarint(wireInt)
			w.varint(v.Int())
		case doc.ValueBool:
			w.uvarint(wireBool)
			if v.Bool() {
				w.uvarint(1)
			} else {
				w.uvarint(0)
			}
		case doc.ValueText:
			w.uvarint(wireText)
			w.bytes(v.Text())
		case doc.ValueList:
			w.uvarint(wireList)
			w.uvarint(uint64(v.Len()))
			for _, rec := range v.Records() {
				w.bytes(string(EncodeAttrs(rec)))
			}
		case doc.ValueRaw:
			wire, payload := v.RawWire()
			w.uvarint(uint64(wire))
			w.b = append(w.b, payload...)
		}
	}
}

// DecodeAttrs decodes attributes. Fields the kind does not declare are
// kept raw, so a rewrite preserves them.
func DecodeAttrs(b []byte, known func(num uint16) bool) (doc.Attrs, error) {
	r := &reader{b: b}
	a := decodeAttrs(r, known)
	return a, r.err
}

func decodeAttrs(r *reader, known func(uint16) bool) doc.Attrs {
	var fields []doc.Field
	for len(r.b) > 0 && r.err == nil {
		num := uint16(r.uvarint())
		wire := uint8(r.uvarint())
		start := r.b
		var v doc.Value
		switch wire {
		case wireInt:
			v = doc.Int(r.varint())
		case wireBool:
			v = doc.Bool(r.uvarint() != 0)
		case wireText:
			v = doc.Text(r.bytes())
		case wireList:
			n := r.uvarint()
			recs := make([]doc.Attrs, 0, n)
			for i := uint64(0); i < n && r.err == nil; i++ {
				sub := &reader{b: []byte(r.bytes())}
				recs = append(recs, decodeAttrs(sub, func(uint16) bool { return true }))
				if sub.err != nil {
					r.fail()
				}
			}
			v = doc.List(recs...)
		default:
			r.fail()
			return doc.Attrs{}
		}
		if known != nil && !known(num) {
			v = doc.Raw(wire, string(start[:len(start)-len(r.b)]))
		}
		fields = append(fields, doc.Field{Num: num, Value: v})
	}
	return doc.MakeAttrs(fields...)
}

// --- runs ---------------------------------------------------------------

// Run tags.
const (
	tagText = 0
	tagNode = 1
)

// encodeSlots appends a node's inline slots to the row's text and runs:
// per inline slot its index and items; a text item is its length and
// mark set, an inline node its id, kind, attributes and its own slots.
func encodeSlots(w *writer, text *[]byte, n *doc.Node) {
	var inline []int
	for i, s := range n.Slots() {
		if s.IsInline() {
			inline = append(inline, i)
		}
	}
	w.uvarint(uint64(len(inline)))
	for _, i := range inline {
		w.uvarint(uint64(i))
		content := n.Slot(i).Content()
		var items []doc.Item
		content.Each(func(_ int, it doc.Item) bool { items = append(items, it); return true })
		w.uvarint(uint64(len(items)))
		for _, it := range items {
			if it.Node != nil {
				w.uvarint(tagNode)
				w.uvarint(uint64(it.Node.ID()))
				w.uvarint(uint64(it.Node.Kind()))
				w.bytes(string(EncodeAttrs(it.Node.Attrs())))
				encodeSlots(w, text, it.Node)
				continue
			}
			w.uvarint(tagText)
			w.uvarint(uint64(len(it.Text)))
			*text = append(*text, it.Text...)
			var marks []doc.Mark
			if it.Marks != nil {
				marks = it.Marks.Marks()
			}
			w.uvarint(uint64(len(marks)))
			for _, m := range marks {
				w.uvarint(uint64(m.Kind))
				w.bytes(string(EncodeAttrs(m.Attrs)))
			}
		}
	}
}

// decodeSlots reads inline slots back, consuming text. Block slots are
// left empty for the caller to fill.
func decodeSlots(r *reader, text *string, schema *doc.Schema, marks *doc.Interner, kind doc.Kind) map[int][]doc.Item {
	out := map[int][]doc.Item{}
	n := r.uvarint()
	for i := uint64(0); i < n && r.err == nil; i++ {
		slot := int(r.uvarint())
		count := r.uvarint()
		var items []doc.Item
		for j := uint64(0); j < count && r.err == nil; j++ {
			switch r.uvarint() {
			case tagNode:
				id := doc.ID(r.uvarint())
				k := doc.Kind(r.uvarint())
				attrs, err := DecodeAttrs([]byte(r.bytes()), knownFields(schema, k))
				if err != nil {
					r.fail()
				}
				child := buildNode(id, k, attrs, "", decodeSlots(r, text, schema, marks, k), nil, schema)
				items = append(items, doc.NodeItem(child))
			case tagText:
				l := int(r.uvarint())
				if l > len(*text) {
					r.fail()
					return out
				}
				s := (*text)[:l]
				*text = (*text)[l:]
				nm := int(r.uvarint())
				ms := make([]doc.Mark, 0, nm)
				for m := 0; m < nm && r.err == nil; m++ {
					mk := doc.MarkKind(r.uvarint())
					attrs, err := DecodeAttrs([]byte(r.bytes()), nil)
					if err != nil {
						r.fail()
					}
					ms = append(ms, doc.Mark{Kind: mk, Attrs: attrs})
				}
				items = append(items, doc.TextItem(s, marks.Intern(ms)))
			default:
				r.fail()
			}
		}
		out[slot] = items
	}
	return out
}

func knownFields(schema *doc.Schema, k doc.Kind) func(uint16) bool {
	spec, ok := schema.Kind(k)
	return func(num uint16) bool {
		if !ok {
			return false
		}
		for _, f := range spec.Fields {
			if f.Num == num {
				return true
			}
		}
		return false
	}
}

// buildNode makes a node with its inline slots and block children.
func buildNode(id doc.ID, k doc.Kind, attrs doc.Attrs, ord string, inline map[int][]doc.Item, blocks map[int][]*doc.Node, schema *doc.Schema) *doc.Node {
	spec, ok := schema.Kind(k)
	count := 0
	if ok {
		count = len(spec.Slots)
	}
	for i := range inline {
		count = max(count, i+1)
	}
	for i := range blocks {
		count = max(count, i+1)
	}
	slots := make([]doc.Slot, count)
	for i := range slots {
		isBlocks := ok && i < len(spec.Slots) && spec.Slots[i].Blocks
		if items, has := inline[i]; has || !isBlocks {
			slots[i] = doc.InlineSlot(items...)
		} else {
			slots[i] = doc.BlockSlot(blocks[i]...)
		}
	}
	return doc.NewNodeWithID(id, k, attrs, ord, slots...)
}

func corrupt(id doc.ID, what string) error {
	return fmt.Errorf("%w: node %s: %s", ErrCorrupt, id, what)
}
