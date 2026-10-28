package kernel

import "fmt"

// ConditionKind identifies an analyzable policy guard.
type ConditionKind uint8

const (
	ConditionTrue ConditionKind = iota + 1
	ConditionFalse
	ConditionEventKey
	ConditionEventOrigin
	ConditionResourceEquals
	ConditionNot
	ConditionAll
	ConditionAny
)

// Condition is a normalized boolean policy guard.
type Condition struct {
	kind     ConditionKind
	key      string
	origin   EventOrigin
	resource ResourceID
	value    ResourceValue
	children []Condition
}

func TrueCondition() Condition               { return Condition{kind: ConditionTrue} }
func FalseCondition() Condition              { return Condition{kind: ConditionFalse} }
func EventKeyCondition(key string) Condition { return Condition{kind: ConditionEventKey, key: key} }
func EventOriginCondition(o EventOrigin) Condition {
	return Condition{kind: ConditionEventOrigin, origin: o}
}
func ResourceEqualsCondition(id ResourceID, value ResourceValue) Condition {
	return Condition{kind: ConditionResourceEquals, resource: id, value: value.Clone()}
}
func NotCondition(child Condition) Condition {
	return Condition{kind: ConditionNot, children: []Condition{child}}
}
func AllConditions(children ...Condition) Condition {
	return Condition{kind: ConditionAll, children: cloneConditions(children)}
}
func AnyConditions(children ...Condition) Condition {
	return Condition{kind: ConditionAny, children: cloneConditions(children)}
}

func cloneConditions(children []Condition) []Condition {
	copied := make([]Condition, len(children))
	copy(copied, children)
	return copied
}

func (c Condition) Kind() ConditionKind { return c.kind }

// CompiledCondition is normalized and carries conservative dependency metadata.
type CompiledCondition struct {
	node  Condition
	reads map[ResourceID]struct{}
	keys  map[string]struct{}
}

// CompileCondition normalizes constant structure while preserving declared
// resource reads and event-key dependencies.
func CompileCondition(condition Condition) CompiledCondition {
	reads := make(map[ResourceID]struct{})
	keys := make(map[string]struct{})
	collectConditionMetadata(condition, reads, keys)
	return CompiledCondition{node: normalizeCondition(condition, nil), reads: reads, keys: keys}
}

type conditionHeader struct {
	key    *string
	origin *EventOrigin
}

func normalizeCondition(c Condition, header *conditionHeader) Condition {
	switch c.kind {
	case ConditionTrue, ConditionFalse:
		return c
	case ConditionEventKey:
		if header == nil {
			return c
		}
		return constantCondition(header.key != nil && *header.key == c.key)
	case ConditionEventOrigin:
		if header == nil {
			return c
		}
		return constantCondition(header.origin != nil && c.origin.Matches(*header.origin))
	case ConditionResourceEquals:
		return c.Clone()
	case ConditionNot:
		child := normalizeCondition(c.children[0], header)
		switch child.kind {
		case ConditionTrue:
			return FalseCondition()
		case ConditionFalse:
			return TrueCondition()
		case ConditionNot:
			return child.children[0].Clone()
		default:
			return NotCondition(child)
		}
	case ConditionAll:
		return normalizeAll(c.children, header)
	case ConditionAny:
		return normalizeAny(c.children, header)
	default:
		return c.Clone()
	}
}

func normalizeAll(children []Condition, header *conditionHeader) Condition {
	normalized := make([]Condition, 0, len(children))
	for _, child := range children {
		child = normalizeCondition(child, header)
		switch child.kind {
		case ConditionTrue:
			continue
		case ConditionFalse:
			return FalseCondition()
		case ConditionAll:
			normalized = append(normalized, child.children...)
		default:
			normalized = append(normalized, child)
		}
	}
	switch len(normalized) {
	case 0:
		return TrueCondition()
	case 1:
		return normalized[0]
	default:
		return AllConditions(normalized...)
	}
}

func normalizeAny(children []Condition, header *conditionHeader) Condition {
	normalized := make([]Condition, 0, len(children))
	for _, child := range children {
		child = normalizeCondition(child, header)
		switch child.kind {
		case ConditionFalse:
			continue
		case ConditionTrue:
			return TrueCondition()
		case ConditionAny:
			normalized = append(normalized, child.children...)
		default:
			normalized = append(normalized, child)
		}
	}
	switch len(normalized) {
	case 0:
		return FalseCondition()
	case 1:
		return normalized[0]
	default:
		return AnyConditions(normalized...)
	}
}

func constantCondition(value bool) Condition {
	if value {
		return TrueCondition()
	}
	return FalseCondition()
}

func collectConditionMetadata(c Condition, reads map[ResourceID]struct{}, keys map[string]struct{}) {
	switch c.kind {
	case ConditionEventKey:
		keys[c.key] = struct{}{}
	case ConditionResourceEquals:
		reads[c.resource] = struct{}{}
	case ConditionNot, ConditionAll, ConditionAny:
		for _, child := range c.children {
			collectConditionMetadata(child, reads, keys)
		}
	}
}

func (c CompiledCondition) Node() Condition { return c.node.Clone() }

func (c CompiledCondition) IsConstant() (bool, bool) {
	switch c.node.kind {
	case ConditionTrue:
		return true, true
	case ConditionFalse:
		return false, true
	default:
		return false, false
	}
}

func (c CompiledCondition) Specialize(key string, origin EventOrigin) CompiledCondition {
	header := conditionHeader{key: &key, origin: &origin}
	return CompiledCondition{node: normalizeCondition(c.node, &header), reads: c.reads, keys: c.keys}
}

func (c CompiledCondition) ResourceReads() []ResourceID {
	ids := make([]ResourceID, 0, len(c.reads))
	for id := range c.reads {
		ids = append(ids, id)
	}
	sortResourceIDs(ids)
	return ids
}

func (c Condition) Clone() Condition {
	c.children = cloneConditions(c.children)
	c.value = c.value.Clone()
	return c
}

func (c CompiledCondition) String() string {
	return fmt.Sprint(c.Node())
}

func sortResourceIDs(ids []ResourceID) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}
