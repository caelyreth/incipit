package kernel

import "sort"

// DocumentAccess is the authority one policy has over the semantic document.
type DocumentAccess uint8

const (
	DocumentAccessNone DocumentAccess = iota
	DocumentAccessRead
	DocumentAccessWrite
)

func (a DocumentAccess) String() string {
	switch a {
	case DocumentAccessNone:
		return "none"
	case DocumentAccessRead:
		return "read"
	case DocumentAccessWrite:
		return "write"
	default:
		return "unknown"
	}
}

// EffectSet declares the resources and document authority a policy may observe
// or mutate. Sets are copied at construction and by Clone.
type EffectSet struct {
	document DocumentAccess
	reads    map[ResourceID]struct{}
	writes   map[ResourceID]struct{}
}

func NewEffectSet(reads, writes []ResourceID) EffectSet {
	effects := EffectSet{
		reads:  make(map[ResourceID]struct{}, len(reads)),
		writes: make(map[ResourceID]struct{}, len(writes)),
	}
	for _, id := range reads {
		effects.reads[id] = struct{}{}
	}
	for _, id := range writes {
		effects.writes[id] = struct{}{}
	}
	return effects
}

func (e EffectSet) WithDocument(access DocumentAccess) EffectSet {
	result := e.Clone()
	result.document = access
	return result
}

func (e EffectSet) WithReads(reads []ResourceID) EffectSet {
	result := e.Clone()
	for _, id := range reads {
		result.reads[id] = struct{}{}
	}
	return result
}

func (e EffectSet) WithWrites(writes []ResourceID) EffectSet {
	result := e.Clone()
	for _, id := range writes {
		result.writes[id] = struct{}{}
	}
	return result
}

func (e EffectSet) WithStateReads(keys []StateKey) EffectSet {
	result := e.Clone()
	for _, key := range keys {
		if key.Document {
			if result.document < DocumentAccessRead {
				result.document = DocumentAccessRead
			}
		} else {
			result.reads[key.Resource] = struct{}{}
		}
	}
	return result
}

func (e EffectSet) Document() DocumentAccess { return e.document }

func (e EffectSet) Reads() []ResourceID { return sortedResourceSet(e.reads) }

func (e EffectSet) Writes() []ResourceID { return sortedResourceSet(e.writes) }

func (e EffectSet) CanRead(id ResourceID) bool {
	_, ok := e.reads[id]
	return ok
}

func (e EffectSet) CanWrite(id ResourceID) bool {
	_, ok := e.writes[id]
	return ok
}

func (e EffectSet) CanWriteState(key StateKey) bool {
	if key.Document {
		return e.document == DocumentAccessWrite
	}
	return e.CanWrite(key.Resource)
}

func (e EffectSet) StateWrites() []StateKey {
	keys := make([]StateKey, 0, len(e.writes)+1)
	if e.document == DocumentAccessWrite {
		keys = append(keys, DocumentStateKey())
	}
	for _, id := range sortedResourceSet(e.writes) {
		keys = append(keys, ResourceStateKey(id))
	}
	return keys
}

func (e EffectSet) Union(other EffectSet) EffectSet {
	result := e.Clone()
	if other.document > result.document {
		result.document = other.document
	}
	for id := range other.reads {
		result.reads[id] = struct{}{}
	}
	for id := range other.writes {
		result.writes[id] = struct{}{}
	}
	return result
}

// Interferes implements conservative sequential observer/rule conflict checks.
func (e EffectSet) Interferes(other EffectSet) bool {
	if e.document == DocumentAccessWrite && other.document != DocumentAccessNone {
		return true
	}
	if other.document == DocumentAccessWrite && e.document != DocumentAccessNone {
		return true
	}
	if intersects(e.writes, other.reads) || intersects(e.writes, other.writes) || intersects(other.writes, e.reads) {
		return true
	}
	return false
}

func (e EffectSet) Clone() EffectSet {
	return EffectSet{
		document: e.document,
		reads:    cloneResourceSet(e.reads),
		writes:   cloneResourceSet(e.writes),
	}
}

func cloneResourceSet(set map[ResourceID]struct{}) map[ResourceID]struct{} {
	result := make(map[ResourceID]struct{}, len(set))
	for id := range set {
		result[id] = struct{}{}
	}
	return result
}

func intersects(first, second map[ResourceID]struct{}) bool {
	if len(first) > len(second) {
		first, second = second, first
	}
	for id := range first {
		if _, ok := second[id]; ok {
			return true
		}
	}
	return false
}

func sortedResourceSet(set map[ResourceID]struct{}) []ResourceID {
	ids := make([]ResourceID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
