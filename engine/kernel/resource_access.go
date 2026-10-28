package kernel

// ResourceRead grants only the reads declared by an EffectSet.
type ResourceRead struct {
	values  map[ResourceID]ResourceValue
	effects EffectSet
}

func (r ResourceRead) Get(id ResourceID) (ResourceValue, error) {
	if !r.effects.CanRead(id) {
		return ResourceValue{}, &ResourceError{Kind: ResourceErrorUndeclaredRead, Resource: id}
	}
	value, ok := r.values[id]
	if !ok {
		return ResourceValue{}, &ResourceError{Kind: ResourceErrorUnknown, Resource: id}
	}
	return value.Clone(), nil
}

func (r ResourceRead) Rows(id ResourceID) (map[RowID]ResourceValue, error) {
	value, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	rows, ok := value.Rows()
	if !ok {
		return nil, &ResourceError{Kind: ResourceErrorNotCollection, Resource: id}
	}
	return rows, nil
}

func (r ResourceRead) Row(id ResourceID, row RowID) (ResourceValue, bool, error) {
	rows, err := r.Rows(id)
	if err != nil {
		return ResourceValue{}, false, err
	}
	value, ok := rows[row]
	return value, ok, nil
}

// ResourceWrite grants declared reads plus declared writes, journaling every
// successful mutation for checkpoint rollback.
type ResourceWrite struct {
	draft   *ResourceDraft
	effects EffectSet
}

func (w *ResourceWrite) Changed(id ResourceID) (bool, error) {
	if _, err := w.Get(id); err != nil {
		return false, err
	}
	_, changed := w.draft.writes[id]
	return changed, nil
}

func (w *ResourceWrite) Get(id ResourceID) (ResourceValue, error) {
	if !w.effects.CanRead(id) && !w.effects.CanWrite(id) {
		return ResourceValue{}, &ResourceError{Kind: ResourceErrorUndeclaredRead, Resource: id}
	}
	value, ok := w.draft.values[id]
	if !ok {
		return ResourceValue{}, &ResourceError{Kind: ResourceErrorUnknown, Resource: id}
	}
	return value.Clone(), nil
}

func (w *ResourceWrite) Rows(id ResourceID) (map[RowID]ResourceValue, error) {
	value, err := w.Get(id)
	if err != nil {
		return nil, err
	}
	rows, ok := value.Rows()
	if !ok {
		return nil, &ResourceError{Kind: ResourceErrorNotCollection, Resource: id}
	}
	return rows, nil
}

func (w *ResourceWrite) Row(id ResourceID, row RowID) (ResourceValue, bool, error) {
	rows, err := w.Rows(id)
	if err != nil {
		return ResourceValue{}, false, err
	}
	value, ok := rows[row]
	return value, ok, nil
}

func (w *ResourceWrite) Set(id ResourceID, value ResourceValue) error {
	if !w.effects.CanWrite(id) {
		return &ResourceError{Kind: ResourceErrorUndeclaredWrite, Resource: id}
	}
	expected, ok := w.draft.schema[id]
	if !ok {
		return &ResourceError{Kind: ResourceErrorUnknown, Resource: id}
	}
	if value.Kind() != expected {
		return &ResourceError{
			Kind:     ResourceErrorTypeMismatch,
			Resource: id,
			Expected: expected,
			Actual:   value.Kind(),
		}
	}
	w.recordUndo(id, nil)
	w.draft.values[id] = value.Clone()
	w.draft.writes[id] = struct{}{}
	return nil
}

func (w *ResourceWrite) PutRow(id ResourceID, row RowID, value ResourceValue) error {
	if err := w.ensureCollectionWrite(id); err != nil {
		return err
	}
	w.recordUndo(id, &row)
	resource := w.draft.values[id]
	resource.collection[row] = value.Clone()
	w.draft.values[id] = resource
	w.markRowWrite(id, row)
	return nil
}

func (w *ResourceWrite) RemoveRow(id ResourceID, row RowID) (ResourceValue, bool, error) {
	if err := w.ensureCollectionWrite(id); err != nil {
		return ResourceValue{}, false, err
	}
	w.recordUndo(id, &row)
	resource := w.draft.values[id]
	removed, ok := resource.collection[row]
	delete(resource.collection, row)
	w.draft.values[id] = resource
	w.markRowWrite(id, row)
	return removed.Clone(), ok, nil
}

func (w *ResourceWrite) ensureCollectionWrite(id ResourceID) error {
	if !w.effects.CanWrite(id) {
		return &ResourceError{Kind: ResourceErrorUndeclaredWrite, Resource: id}
	}
	kind, ok := w.draft.schema[id]
	if !ok {
		return &ResourceError{Kind: ResourceErrorUnknown, Resource: id}
	}
	if kind != ResourceCollection {
		return &ResourceError{Kind: ResourceErrorNotCollection, Resource: id}
	}
	return nil
}

func (w *ResourceWrite) recordUndo(id ResourceID, row *RowID) {
	previous, hadPrevious := w.draft.values[id]
	_, wasWritten := w.draft.writes[id]
	undo := resourceUndo{
		id:          id,
		previous:    previous.Clone(),
		hadPrevious: hadPrevious,
		wasWritten:  wasWritten,
		row:         row,
	}
	if row != nil {
		_, undo.wasRowWritten = w.draft.rowWrites[id][*row]
	}
	w.draft.journal = append(w.draft.journal, undo)
}

func (w *ResourceWrite) markRowWrite(id ResourceID, row RowID) {
	rows := w.draft.rowWrites[id]
	if rows == nil {
		rows = make(map[RowID]struct{})
		w.draft.rowWrites[id] = rows
	}
	rows[row] = struct{}{}
	w.draft.writes[id] = struct{}{}
}
