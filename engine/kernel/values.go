package kernel

import (
	"fmt"
	"sort"
)

// ResourceKind is the declared shape of a kernel resource.
type ResourceKind uint8

const (
	ResourceBool ResourceKind = iota + 1
	ResourceInteger
	ResourceText
	ResourceCollection
)

// ResourceLifetime says whether a resource belongs to the document or only to
// the currently enabled policy session.
type ResourceLifetime uint8

const (
	ResourceDocumentLifetime ResourceLifetime = iota + 1
	ResourceSessionLifetime
)

func (k ResourceKind) String() string {
	switch k {
	case ResourceBool:
		return "bool"
	case ResourceInteger:
		return "integer"
	case ResourceText:
		return "text"
	case ResourceCollection:
		return "collection"
	default:
		return fmt.Sprintf("kind(%d)", k)
	}
}

func (l ResourceLifetime) String() string {
	switch l {
	case ResourceDocumentLifetime:
		return "document"
	case ResourceSessionLifetime:
		return "session"
	default:
		return fmt.Sprintf("lifetime(%d)", l)
	}
}

// ResourceDescriptor declares one typed kernel resource.
type ResourceDescriptor struct {
	ID       ResourceID
	Lifetime ResourceLifetime
	Kind     ResourceKind
	Initial  ResourceValue
}

// ResourceValue is an immutable scalar or ordered collection value. Collection
// maps are private copies so snapshots cannot be mutated by callers.
type ResourceValue struct {
	kind       ResourceKind
	boolean    bool
	integer    int64
	text       string
	collection map[RowID]ResourceValue
}

func BoolResource(value bool) ResourceValue {
	return ResourceValue{kind: ResourceBool, boolean: value}
}

func IntegerResource(value int64) ResourceValue {
	return ResourceValue{kind: ResourceInteger, integer: value}
}

func TextResource(value string) ResourceValue {
	return ResourceValue{kind: ResourceText, text: value}
}

func CollectionResource(rows map[RowID]ResourceValue) ResourceValue {
	copied := make(map[RowID]ResourceValue, len(rows))
	for id, value := range rows {
		copied[id] = value.Clone()
	}
	return ResourceValue{kind: ResourceCollection, collection: copied}
}

func (v ResourceValue) Kind() ResourceKind { return v.kind }

func (v ResourceValue) Bool() (bool, bool) {
	return v.boolean, v.kind == ResourceBool
}

func (v ResourceValue) Integer() (int64, bool) {
	return v.integer, v.kind == ResourceInteger
}

func (v ResourceValue) Text() (string, bool) {
	return v.text, v.kind == ResourceText
}

func (v ResourceValue) Rows() (map[RowID]ResourceValue, bool) {
	if v.kind != ResourceCollection {
		return nil, false
	}
	return cloneRows(v.collection), true
}

func (v ResourceValue) Row(id RowID) (ResourceValue, bool) {
	if v.kind != ResourceCollection {
		return ResourceValue{}, false
	}
	row, ok := v.collection[id]
	return row.Clone(), ok
}

func (v ResourceValue) Clone() ResourceValue {
	if v.kind == ResourceCollection {
		v.collection = cloneRows(v.collection)
	}
	return v
}

func (v ResourceValue) Equal(other ResourceValue) bool {
	if v.kind != other.kind {
		return false
	}
	switch v.kind {
	case ResourceBool:
		return v.boolean == other.boolean
	case ResourceInteger:
		return v.integer == other.integer
	case ResourceText:
		return v.text == other.text
	case ResourceCollection:
		if len(v.collection) != len(other.collection) {
			return false
		}
		for id, value := range v.collection {
			otherValue, ok := other.collection[id]
			if !ok || !value.Equal(otherValue) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func cloneRows(rows map[RowID]ResourceValue) map[RowID]ResourceValue {
	copied := make(map[RowID]ResourceValue, len(rows))
	for id, value := range rows {
		copied[id] = value.Clone()
	}
	return copied
}

// SortedRowIDs returns deterministic row order for diffs and diagnostics.
func SortedRowIDs(rows map[RowID]ResourceValue) []RowID {
	ids := make([]RowID, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// SettingValue is a typed policy setting.
type SettingValue struct {
	kind    ResourceKind
	boolean bool
	integer int64
	text    string
}

func BoolSetting(value bool) SettingValue {
	return SettingValue{kind: ResourceBool, boolean: value}
}

func IntegerSetting(value int64) SettingValue {
	return SettingValue{kind: ResourceInteger, integer: value}
}

func TextSetting(value string) SettingValue {
	return SettingValue{kind: ResourceText, text: value}
}

func (v SettingValue) Kind() ResourceKind { return v.kind }

func (v SettingValue) Bool() (bool, bool) {
	return v.boolean, v.kind == ResourceBool
}

func (v SettingValue) Integer() (int64, bool) {
	return v.integer, v.kind == ResourceInteger
}

func (v SettingValue) Text() (string, bool) {
	return v.text, v.kind == ResourceText
}

func (v SettingValue) Equal(other SettingValue) bool {
	return v.kind == other.kind && v.boolean == other.boolean && v.integer == other.integer && v.text == other.text
}
