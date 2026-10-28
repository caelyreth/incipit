package kernel

import "sort"

// ResourceStore owns the published immutable resource values.
type ResourceStore struct {
	schema map[ResourceID]ResourceKind
	values map[ResourceID]ResourceValue
}

// ResourceSnapshot is an immutable value view of a resource store.
type ResourceSnapshot struct {
	schema map[ResourceID]ResourceKind
	values map[ResourceID]ResourceValue
}

// ResourceDraft stages mutations and keeps an undo journal for ordered choice.
type ResourceDraft struct {
	schema    map[ResourceID]ResourceKind
	base      map[ResourceID]ResourceValue
	values    map[ResourceID]ResourceValue
	writes    map[ResourceID]struct{}
	rowWrites map[ResourceID]map[RowID]struct{}
	journal   []resourceUndo
}

// ResourceCheckpoint marks the draft journal's rollback boundary.
type ResourceCheckpoint struct {
	journalLen int
}

type resourceUndo struct {
	id            ResourceID
	previous      ResourceValue
	hadPrevious   bool
	wasWritten    bool
	row           *RowID
	wasRowWritten bool
}

// ResourceChange is the final difference for one resource.
type ResourceChange struct {
	ID         ResourceID
	Before     ResourceValue
	After      ResourceValue
	RowChanges []ResourceRowChange
}

// ResourceRowChange is one sorted collection-row difference.
type ResourceRowChange struct {
	ID        RowID
	Before    ResourceValue
	After     ResourceValue
	HadBefore bool
	HasAfter  bool
}

func (c ResourceChange) Clone() ResourceChange {
	c.Before = c.Before.Clone()
	c.After = c.After.Clone()
	c.RowChanges = append([]ResourceRowChange(nil), c.RowChanges...)
	for index := range c.RowChanges {
		c.RowChanges[index].Before = c.RowChanges[index].Before.Clone()
		c.RowChanges[index].After = c.RowChanges[index].After.Clone()
	}
	return c
}

type ResourceCommit[T any] struct {
	Value   T
	Changes []ResourceChange
}

func NewResourceStore(descriptors []ResourceDescriptor) ResourceStore {
	schema := make(map[ResourceID]ResourceKind, len(descriptors))
	values := make(map[ResourceID]ResourceValue, len(descriptors))
	for _, descriptor := range descriptors {
		schema[descriptor.ID] = descriptor.Kind
		values[descriptor.ID] = descriptor.Initial.Clone()
	}
	return ResourceStore{schema: schema, values: values}
}

func (s ResourceStore) Snapshot() ResourceSnapshot {
	return ResourceSnapshot{schema: cloneSchema(s.schema), values: cloneResourceValues(s.values)}
}

func (s ResourceStore) BeginDraft() *ResourceDraft {
	return &ResourceDraft{
		schema:    cloneSchema(s.schema),
		base:      cloneResourceValues(s.values),
		values:    cloneResourceValues(s.values),
		writes:    make(map[ResourceID]struct{}),
		rowWrites: make(map[ResourceID]map[RowID]struct{}),
	}
}

func (s *ResourceStore) PublishDraft(draft *ResourceDraft) {
	s.values = cloneResourceValues(draft.values)
}

// Transact runs an operation over a draft and publishes only on success.
func (s *ResourceStore) Transact[T any](operation func(*ResourceDraft) (T, error)) (ResourceCommit[T], error) {
	draft := s.BeginDraft()
	value, err := operation(draft)
	if err != nil {
		var zero ResourceCommit[T]
		return zero, err
	}
	changes := draft.FinalChanges()
	s.PublishDraft(draft)
	return ResourceCommit[T]{Value: value, Changes: changes}, nil
}

func (s ResourceSnapshot) Get(id ResourceID) (ResourceValue, bool) {
	value, ok := s.values[id]
	return value.Clone(), ok
}

func (s ResourceSnapshot) Read(effects EffectSet) ResourceRead {
	return ResourceRead{values: s.values, effects: effects}
}

func (d *ResourceDraft) WasWritten(id ResourceID) bool {
	_, ok := d.writes[id]
	return ok
}

func (d *ResourceDraft) WrittenResources() []ResourceID {
	return sortedResourceKeys(d.writes)
}

func (d *ResourceDraft) WrittenRows(id ResourceID) []RowID {
	rows := d.rowWrites[id]
	ids := make([]RowID, 0, len(rows))
	for row := range rows {
		ids = append(ids, row)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (d *ResourceDraft) Snapshot() ResourceSnapshot {
	return ResourceSnapshot{schema: cloneSchema(d.schema), values: cloneResourceValues(d.values)}
}

func (d *ResourceDraft) Read(effects EffectSet) ResourceRead {
	return ResourceRead{values: d.values, effects: effects}
}

func (d *ResourceDraft) Write(effects EffectSet) *ResourceWrite {
	return &ResourceWrite{draft: d, effects: effects}
}

func (d *ResourceDraft) FinalChanges() []ResourceChange {
	ids := make([]ResourceID, 0, len(d.values))
	for id := range d.values {
		if _, ok := d.base[id]; ok {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	changes := make([]ResourceChange, 0, len(ids))
	for _, id := range ids {
		before, after := d.base[id], d.values[id]
		if before.Equal(after) {
			continue
		}
		changes = append(changes, ResourceChange{
			ID:         id,
			Before:     before.Clone(),
			After:      after.Clone(),
			RowChanges: diffRows(before, after),
		})
	}
	return changes
}

func (d *ResourceDraft) Checkpoint() ResourceCheckpoint {
	return ResourceCheckpoint{journalLen: len(d.journal)}
}

func (d *ResourceDraft) Rollback(checkpoint ResourceCheckpoint) {
	for len(d.journal) > checkpoint.journalLen {
		undo := d.journal[len(d.journal)-1]
		d.journal = d.journal[:len(d.journal)-1]
		if undo.hadPrevious {
			d.values[undo.id] = undo.previous.Clone()
		} else {
			delete(d.values, undo.id)
		}
		if undo.wasWritten {
			d.writes[undo.id] = struct{}{}
		} else {
			delete(d.writes, undo.id)
		}
		if undo.row != nil {
			rows := d.rowWrites[undo.id]
			if undo.wasRowWritten {
				if rows == nil {
					rows = make(map[RowID]struct{})
					d.rowWrites[undo.id] = rows
				}
				rows[*undo.row] = struct{}{}
			} else if rows != nil {
				delete(rows, *undo.row)
				if len(rows) == 0 {
					delete(d.rowWrites, undo.id)
				}
			}
		}
	}
}

func diffRows(before, after ResourceValue) []ResourceRowChange {
	beforeRows, beforeOK := before.Rows()
	afterRows, afterOK := after.Rows()
	if !beforeOK || !afterOK {
		return nil
	}
	ids := make(map[RowID]struct{}, len(beforeRows)+len(afterRows))
	for id := range beforeRows {
		ids[id] = struct{}{}
	}
	for id := range afterRows {
		ids[id] = struct{}{}
	}
	sorted := make([]RowID, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	changes := make([]ResourceRowChange, 0, len(sorted))
	for _, id := range sorted {
		old, hadOld := beforeRows[id]
		newValue, hasNew := afterRows[id]
		if hadOld && hasNew && old.Equal(newValue) {
			continue
		}
		changes = append(changes, ResourceRowChange{
			ID:        id,
			Before:    old.Clone(),
			After:     newValue.Clone(),
			HadBefore: hadOld,
			HasAfter:  hasNew,
		})
	}
	return changes
}

func cloneSchema(schema map[ResourceID]ResourceKind) map[ResourceID]ResourceKind {
	copied := make(map[ResourceID]ResourceKind, len(schema))
	for id, kind := range schema {
		copied[id] = kind
	}
	return copied
}

func cloneResourceValues(values map[ResourceID]ResourceValue) map[ResourceID]ResourceValue {
	copied := make(map[ResourceID]ResourceValue, len(values))
	for id, value := range values {
		copied[id] = value.Clone()
	}
	return copied
}

func sortedResourceKeys(set map[ResourceID]struct{}) []ResourceID {
	return sortedResourceSet(set)
}
