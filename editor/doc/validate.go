package doc

import (
	"fmt"
	"strings"
)

// validateNode checks one node's local validity and canonical form: its
// slots against its kind, the kinds and marks they hold, its attributes,
// and whatever its kind adds.
func validateNode(schema *Schema, marks *Interner, n *Node) error {
	spec, ok := schema.Kind(n.kind)
	if !ok {
		return fmt.Errorf("%w: %s has an undeclared kind %d", ErrInvalid, n.id, n.kind)
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s %s: %s", ErrInvalid, spec.Name, n.id, fmt.Sprintf(format, args...))
	}
	if len(n.slots) != len(spec.Slots) {
		return fail("has %d slots, its kind declares %d", len(n.slots), len(spec.Slots))
	}
	if err := validAttrs(spec.Name, spec.Fields, n.attrs); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	for i, slot := range spec.Slots {
		s := n.slots[i]
		if s.inline == slot.Blocks {
			return fail("slot %s has the wrong type", slot.Name)
		}
		if slot.Blocks {
			for _, child := range s.children {
				cspec, ok := schema.Kind(child.kind)
				if !ok || cspec.Placement != PlaceBlock || slot.Accepts == nil || !slot.Accepts(child.kind) {
					return fail("slot %s does not accept a %s", slot.Name, schema.KindName(child.kind))
				}
			}
			continue
		}
		if err := validInline(schema, marks, slot, s.Content()); err != nil {
			return fail("slot %s: %v", slot.Name, err)
		}
	}
	if spec.Check != nil {
		if err := spec.Check(n); err != nil {
			return fail("%v", err)
		}
	}
	return nil
}

func validInline(schema *Schema, marks *Interner, slot SlotSpec, in *Inline) error {
	if in.size > 0 && len(in.runs) == 0 {
		return fmt.Errorf("content without runs")
	}
	if strings.ContainsRune(in.text, '\r') {
		return fmt.Errorf("a line break that is not LF")
	}
	if !slot.Lines && strings.ContainsRune(in.text, '\n') {
		return fmt.Errorf("a line break where lines are not kept")
	}
	for i, r := range in.runs {
		if r.node != nil {
			if slot.Literal {
				return fmt.Errorf("an inline node in a literal slot")
			}
			cspec, ok := schema.Kind(r.node.kind)
			if !ok || cspec.Placement != PlaceInline || slot.Accepts == nil || !slot.Accepts(r.node.kind) {
				return fmt.Errorf("an inline %s it does not accept", schema.KindName(r.node.kind))
			}
			continue
		}
		if r.n == 0 {
			return fmt.Errorf("an empty text run")
		}
		if !marks.Owns(r.marks) {
			return fmt.Errorf("a mark set from another page")
		}
		if i > 0 && in.runs[i-1].node == nil && in.runs[i-1].marks == r.marks {
			return fmt.Errorf("adjacent runs with the same marks")
		}
		if slot.Literal && !r.marks.Empty() {
			return fmt.Errorf("marks in a literal slot")
		}
		for _, m := range r.marks.Marks() {
			mspec, ok := schema.Mark(m.Kind)
			if !ok || slot.Marks == nil || !slot.Marks(m.Kind) {
				return fmt.Errorf("a mark %d it does not accept", m.Kind)
			}
			if err := validAttrs(mspec.Name, mspec.Fields, m.Attrs); err != nil {
				return err
			}
			if mspec.Excludes != nil {
				for _, other := range r.marks.Marks() {
					if other.Kind != m.Kind && mspec.Excludes(other.Kind) {
						return fmt.Errorf("mark %s excludes mark %d", mspec.Name, other.Kind)
					}
				}
			}
		}
	}
	return nil
}

// Validate checks every node of the snapshot, as debug builds and tests
// do after each transaction; Commit checks only the nodes it touched.
func (s *Snapshot) Validate() error {
	var walk func(n *Node) error
	walk = func(n *Node) error {
		if err := validateNode(s.schema, s.marks, n); err != nil {
			return err
		}
		for _, sl := range n.slots {
			if sl.inline {
				for _, r := range sl.Content().runs {
					if r.node != nil {
						if err := walk(r.node); err != nil {
							return err
						}
					}
				}
				continue
			}
			for _, c := range sl.children {
				if err := walk(c); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(s.root)
}
