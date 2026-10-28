package kernel

import "sort"

// WriteSet records successful writes that survived rollback, including writes
// of equal values. It is a dependency footprint, not a final change diff.
type WriteSet struct {
	keys map[StateKey]struct{}
}

func NewWriteSet(document bool, resources []ResourceID) WriteSet {
	set := WriteSet{keys: make(map[StateKey]struct{}, len(resources)+1)}
	if document {
		set.keys[DocumentStateKey()] = struct{}{}
	}
	for _, id := range resources {
		set.keys[ResourceStateKey(id)] = struct{}{}
	}
	return set
}

func (s WriteSet) Contains(key StateKey) bool {
	_, ok := s.keys[key]
	return ok
}

func (s WriteSet) Empty() bool { return len(s.keys) == 0 }

func (s WriteSet) Keys() []StateKey {
	keys := make([]StateKey, 0, len(s.keys))
	for key := range s.keys {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Document != keys[j].Document {
			return keys[i].Document
		}
		return keys[i].Resource < keys[j].Resource
	})
	return keys
}
