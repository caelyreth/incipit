package doc

import (
	"fmt"
	"slices"
	"strings"
)

// ValueKind is the type of an attribute value.
type ValueKind uint8

const (
	ValueNone ValueKind = iota
	ValueInt
	ValueBool
	ValueText
	// ValueList is a list of records, such as a table's columns.
	ValueList
	// ValueRaw is a field this build does not know, kept as its wire form
	// so rewriting a row preserves it.
	ValueRaw
)

// Value is one attribute value. It is immutable.
type Value struct {
	kind ValueKind
	i    int64
	s    string
	list []Attrs
}

func Int(v int64) Value   { return Value{kind: ValueInt, i: v} }
func Text(v string) Value { return Value{kind: ValueText, s: v} }

func Bool(v bool) Value {
	if v {
		return Value{kind: ValueBool, i: 1}
	}
	return Value{kind: ValueBool}
}

// List is a list of records.
func List(records ...Attrs) Value {
	return Value{kind: ValueList, list: slices.Clone(records)}
}

// Raw keeps an unknown field's wire type and bytes.
func Raw(wire uint8, bytes string) Value {
	return Value{kind: ValueRaw, i: int64(wire), s: bytes}
}

func (v Value) Kind() ValueKind { return v.kind }
func (v Value) Int() int64      { return v.i }
func (v Value) Bool() bool      { return v.i != 0 }
func (v Value) Text() string    { return v.s }
func (v Value) Len() int        { return len(v.list) }
func (v Value) At(i int) Attrs  { return v.list[i] }

// Records returns the list's records. The slice must not be changed.
func (v Value) Records() []Attrs { return v.list }

// RawWire returns a raw value's wire type and bytes.
func (v Value) RawWire() (uint8, string) { return uint8(v.i), v.s }

// Equal reports whether two values are the same.
func (v Value) Equal(o Value) bool {
	if v.kind != o.kind || v.i != o.i || v.s != o.s || len(v.list) != len(o.list) {
		return false
	}
	for i := range v.list {
		if !v.list[i].Equal(o.list[i]) {
			return false
		}
	}
	return true
}

func (v Value) String() string {
	switch v.kind {
	case ValueInt:
		return fmt.Sprint(v.i)
	case ValueBool:
		return fmt.Sprint(v.Bool())
	case ValueText:
		return fmt.Sprintf("%q", v.s)
	case ValueList:
		parts := make([]string, len(v.list))
		for i, record := range v.list {
			parts[i] = record.String()
		}
		return "[" + strings.Join(parts, " ") + "]"
	case ValueRaw:
		return fmt.Sprintf("raw(%d,%d bytes)", v.i, len(v.s))
	}
	return "none"
}

// Field is one numbered attribute.
type Field struct {
	Num   uint16
	Value Value
}

// Attrs are a node's or a mark's attributes, sorted by field number. They
// are immutable: every change returns new attributes.
type Attrs struct {
	fields []Field
}

// MakeAttrs builds attributes from fields in any order. A later field with
// the same number replaces an earlier one.
func MakeAttrs(fields ...Field) Attrs {
	var a Attrs
	for _, f := range fields {
		a = a.With(f.Num, f.Value)
	}
	return a
}

func (a Attrs) Len() int        { return len(a.fields) }
func (a Attrs) Fields() []Field { return a.fields }

func (a Attrs) find(num uint16) (int, bool) {
	return slices.BinarySearchFunc(a.fields, num, func(f Field, n uint16) int { return int(f.Num) - int(n) })
}

// Get returns the value stored for num.
func (a Attrs) Get(num uint16) (Value, bool) {
	if i, ok := a.find(num); ok {
		return a.fields[i].Value, true
	}
	return Value{}, false
}

// With returns the attributes with num set to v.
func (a Attrs) With(num uint16, v Value) Attrs {
	i, ok := a.find(num)
	fields := make([]Field, 0, len(a.fields)+1)
	fields = append(fields, a.fields[:i]...)
	fields = append(fields, Field{Num: num, Value: v})
	if ok {
		i++
	}
	fields = append(fields, a.fields[i:]...)
	return Attrs{fields: fields}
}

// Without returns the attributes with num removed.
func (a Attrs) Without(num uint16) Attrs {
	i, ok := a.find(num)
	if !ok {
		return a
	}
	fields := make([]Field, 0, len(a.fields)-1)
	fields = append(fields, a.fields[:i]...)
	fields = append(fields, a.fields[i+1:]...)
	return Attrs{fields: fields}
}

// Equal reports whether two attribute sets hold the same fields.
func (a Attrs) Equal(o Attrs) bool {
	if len(a.fields) != len(o.fields) {
		return false
	}
	for i := range a.fields {
		if a.fields[i].Num != o.fields[i].Num || !a.fields[i].Value.Equal(o.fields[i].Value) {
			return false
		}
	}
	return true
}

func (a Attrs) String() string {
	parts := make([]string, len(a.fields))
	for i, f := range a.fields {
		parts[i] = fmt.Sprintf("%d=%s", f.Num, f.Value)
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// key is a canonical string for interning.
func (a Attrs) key(b *strings.Builder) {
	for _, f := range a.fields {
		fmt.Fprintf(b, "%d:%d:", f.Num, f.Value.kind)
		switch f.Value.kind {
		case ValueList:
			for _, r := range f.Value.list {
				b.WriteByte('(')
				r.key(b)
				b.WriteByte(')')
			}
		default:
			fmt.Fprintf(b, "%d:%d:%s;", f.Value.i, len(f.Value.s), f.Value.s)
		}
	}
}
