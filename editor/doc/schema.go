package doc

import (
	"errors"
	"fmt"
)

// Kind names a node kind. The vocabulary declares what each kind is.
type Kind uint16

// MarkKind names a mark kind.
type MarkKind uint16

// Placement is where a kind's nodes stand.
type Placement uint8

const (
	PlaceRoot Placement = iota + 1
	PlaceBlock
	PlaceInline
)

// SlotSpec declares one slot of a kind.
type SlotSpec struct {
	Name string
	// Blocks makes the slot hold block children; otherwise it is inline.
	Blocks bool
	// Accepts reports the child kinds a block slot takes, or the inline
	// node kinds an inline slot takes. Nil accepts none.
	Accepts func(Kind) bool
	// Literal makes an inline slot hold text only: no marks, no inline
	// nodes.
	Literal bool
	// Lines makes an inline slot keep line breaks.
	Lines bool
	// Marks reports the mark kinds a rich inline slot takes. Nil takes
	// none.
	Marks func(MarkKind) bool
}

// FieldSpec declares one attribute field.
type FieldSpec struct {
	Num  uint16
	Name string
	// Default is the value an absent field has. A stored field never
	// equals it.
	Default Value
	// Valid reports whether a value is within the field's domain.
	Valid func(Value) bool
}

// KindSpec declares a node kind.
type KindSpec struct {
	Kind      Kind
	Name      string
	Placement Placement
	Slots     []SlotSpec
	Fields    []FieldSpec
	// Check holds validity a kind adds beyond its slots and fields, such
	// as a table's rows matching its columns. It sees the node's children.
	Check func(n *Node) error
	// Object makes a node one unit in the slot that holds it, which the
	// caret selects whole rather than enters as text: a block between its
	// gaps, or an inline node in its line. Its own inline slots, such as a
	// figure's caption or a formula's source, are entered as its surfaces.
	Object bool
	// Plain is the text block a block becomes when its structure is taken
	// away. Exactly one kind declares it.
	Plain bool
	// Entry makes a kind an entry of the container whose slot holds it:
	// its first slot is its head and the rest its body, and taken out of
	// its container it becomes its head as a plain block, its body after.
	Entry bool
}

// MarkSpec declares a mark kind.
type MarkSpec struct {
	Kind   MarkKind
	Name   string
	Fields []FieldSpec
	// Excludes reports the marks that may not share a set with this one.
	Excludes func(MarkKind) bool
	// Bounded marks do not extend to text typed at their end.
	Bounded bool
}

// Schema is the kinds and marks a document may hold.
type Schema struct {
	root      Kind
	plain     Kind
	kinds     map[Kind]*KindSpec
	marks     map[MarkKind]*MarkSpec
	kindOrder []Kind
	markOrder []MarkKind
}

// NewSchema checks and assembles kind and mark declarations.
func NewSchema(root Kind, kinds []KindSpec, marks []MarkSpec) (*Schema, error) {
	s := &Schema{root: root, kinds: map[Kind]*KindSpec{}, marks: map[MarkKind]*MarkSpec{}}
	for i := range kinds {
		k := kinds[i]
		if _, dup := s.kinds[k.Kind]; dup {
			return nil, fmt.Errorf("doc: kind %d declared twice", k.Kind)
		}
		for _, slot := range k.Slots {
			if slot.Blocks && (slot.Literal || slot.Lines || slot.Marks != nil) {
				return nil, fmt.Errorf("doc: block slot %s.%s declares inline properties", k.Name, slot.Name)
			}
		}
		if err := checkFields(k.Name, k.Fields); err != nil {
			return nil, err
		}
		if k.Plain {
			if s.plain != 0 {
				return nil, errors.New("doc: two kinds are declared plain")
			}
			s.plain = k.Kind
		}
		s.kinds[k.Kind] = &k
		s.kindOrder = append(s.kindOrder, k.Kind)
	}
	for i := range marks {
		m := marks[i]
		if _, dup := s.marks[m.Kind]; dup {
			return nil, fmt.Errorf("doc: mark %d declared twice", m.Kind)
		}
		if err := checkFields(m.Name, m.Fields); err != nil {
			return nil, err
		}
		s.marks[m.Kind] = &m
		s.markOrder = append(s.markOrder, m.Kind)
	}
	spec, ok := s.kinds[root]
	if !ok || spec.Placement != PlaceRoot {
		return nil, errors.New("doc: the root kind is not declared as a root")
	}
	if s.plain == 0 {
		return nil, errors.New("doc: no kind is declared plain")
	}
	return s, nil
}

func checkFields(owner string, fields []FieldSpec) error {
	seen := map[uint16]bool{}
	for _, f := range fields {
		if f.Num == 0 || seen[f.Num] {
			return fmt.Errorf("doc: %s field %q has a zero or repeated number", owner, f.Name)
		}
		seen[f.Num] = true
	}
	return nil
}

// Root is the kind of a page's root node.
func (s *Schema) Root() Kind { return s.root }

// Kind returns a kind's declaration.
func (s *Schema) Kind(k Kind) (*KindSpec, bool) {
	spec, ok := s.kinds[k]
	return spec, ok
}

// Kinds are the declared kinds, in declaration order.
func (s *Schema) Kinds() []Kind { return s.kindOrder }

// MarkKinds are the declared mark kinds, in declaration order.
func (s *Schema) MarkKinds() []MarkKind { return s.markOrder }

// Plain is the plain text block kind.
func (s *Schema) Plain() Kind { return s.plain }

// IsObject reports a node the caret selects whole (KindSpec.Object).
func (s *Schema) IsObject(k Kind) bool {
	spec, ok := s.kinds[k]
	return ok && spec.Object
}

// IsEntry reports an entry of a container (KindSpec.Entry).
func (s *Schema) IsEntry(k Kind) bool {
	spec, ok := s.kinds[k]
	return ok && spec.Entry
}

// IsTextBlock reports a block whose first slot is inline text the caret
// stands in.
func (s *Schema) IsTextBlock(k Kind) bool {
	spec, ok := s.kinds[k]
	return ok && spec.Placement == PlaceBlock && len(spec.Slots) > 0 && !spec.Slots[0].Blocks
}

// Mark returns a mark kind's declaration.
func (s *Schema) Mark(m MarkKind) (*MarkSpec, bool) {
	spec, ok := s.marks[m]
	return spec, ok
}

// KindName names a kind for messages.
func (s *Schema) KindName(k Kind) string {
	if spec, ok := s.kinds[k]; ok {
		return spec.Name
	}
	return fmt.Sprintf("kind(%d)", k)
}

// Canonical is a's stored form on a node of kind k: fields equal to their
// default are dropped.
func (s *Schema) Canonical(k Kind, a Attrs) Attrs {
	spec, ok := s.Kind(k)
	if !ok {
		return a
	}
	return canonicalAttrs(spec.Fields, a)
}

// canonicalAttrs drops fields equal to their default.
func canonicalAttrs(fields []FieldSpec, a Attrs) Attrs {
	for _, f := range fields {
		if v, ok := a.Get(f.Num); ok && v.Equal(f.Default) {
			a = a.Without(f.Num)
		}
	}
	return a
}

func validAttrs(owner string, fields []FieldSpec, a Attrs) error {
	for _, field := range a.Fields() {
		var spec *FieldSpec
		for i := range fields {
			if fields[i].Num == field.Num {
				spec = &fields[i]
			}
		}
		if spec == nil {
			if field.Value.Kind() != ValueRaw {
				return fmt.Errorf("doc: %s has no field %d", owner, field.Num)
			}
			continue
		}
		if field.Value.Equal(spec.Default) {
			return fmt.Errorf("doc: %s stores %s at its default", owner, spec.Name)
		}
		if spec.Valid != nil && !spec.Valid(field.Value) {
			return fmt.Errorf("doc: %s.%s = %s is outside its domain", owner, spec.Name, field.Value)
		}
	}
	return nil
}

// Attr returns a node's field value, or the field's default.
func (s *Schema) Attr(n *Node, num uint16) Value {
	if v, ok := n.attrs.Get(num); ok {
		return v
	}
	if spec, ok := s.kinds[n.kind]; ok {
		for _, f := range spec.Fields {
			if f.Num == num {
				return f.Default
			}
		}
	}
	return Value{}
}
