package kernel

import (
	"errors"
	"testing"
)

func resourceFixture() ResourceStore {
	return NewResourceStore([]ResourceDescriptor{
		{
			ID:       "enabled",
			Lifetime: ResourceSessionLifetime,
			Kind:     ResourceBool,
			Initial:  BoolResource(false),
		},
		{
			ID:       "document",
			Lifetime: ResourceDocumentLifetime,
			Kind:     ResourceText,
			Initial:  TextResource("plain"),
		},
		{
			ID:       "rows",
			Lifetime: ResourceSessionLifetime,
			Kind:     ResourceCollection,
			Initial: CollectionResource(map[RowID]ResourceValue{
				1: TextResource("one"),
			}),
		},
	})
}

func TestTypedResourceAccessAndEqualFinalValues(t *testing.T) {
	t.Parallel()
	store := resourceFixture()
	effects := NewEffectSet([]ResourceID{"enabled"}, []ResourceID{"enabled"})
	commit, err := store.Transact(func(draft *ResourceDraft) (struct{}, error) {
		write := draft.Write(effects)
		if value, err := write.Get("enabled"); err != nil {
			return struct{}{}, err
		} else if enabled, ok := value.Bool(); !ok || enabled {
			return struct{}{}, errors.New("wrong initial value")
		}
		if err := write.Set("enabled", BoolResource(true)); err != nil {
			return struct{}{}, err
		}
		if err := write.Set("enabled", BoolResource(false)); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(commit.Changes) != 0 {
		t.Fatalf("changes = %#v", commit.Changes)
	}
}

func TestUndeclaredAndWrongTypeAccessRejected(t *testing.T) {
	t.Parallel()
	store := resourceFixture()
	_, err := store.Transact(func(draft *ResourceDraft) (struct{}, error) {
		return struct{}{}, draft.Write(NewEffectSet([]ResourceID{"enabled"}, nil)).Set("enabled", BoolResource(true))
	})
	var resourceErr *ResourceError
	if !errors.As(err, &resourceErr) || resourceErr.Kind != ResourceErrorUndeclaredWrite {
		t.Fatalf("error = %v", err)
	}
	_, err = store.Transact(func(draft *ResourceDraft) (struct{}, error) {
		return struct{}{}, draft.Write(NewEffectSet(nil, []ResourceID{"enabled"})).Set("enabled", TextResource("wrong"))
	})
	if !errors.As(err, &resourceErr) || resourceErr.Kind != ResourceErrorTypeMismatch {
		t.Fatalf("error = %v", err)
	}
}

func TestCollectionRowsDiffAndCheckpointRollback(t *testing.T) {
	t.Parallel()
	store := resourceFixture()
	effects := NewEffectSet([]ResourceID{"rows"}, []ResourceID{"rows"})
	commit, err := store.Transact(func(draft *ResourceDraft) (struct{}, error) {
		write := draft.Write(effects)
		if err := write.PutRow("rows", 2, TextResource("two")); err != nil {
			return struct{}{}, err
		}
		removed, ok, err := write.RemoveRow("rows", 1)
		if err != nil {
			return struct{}{}, err
		}
		removedText, isText := removed.Text()
		if !ok || !isText || removedText != "one" {
			return struct{}{}, errors.New("wrong removed row")
		}
		checkpoint := draft.Checkpoint()
		if err := write.PutRow("rows", 3, TextResource("discarded")); err != nil {
			return struct{}{}, err
		}
		draft.Rollback(checkpoint)
		if _, ok, err := draft.Read(effects).Row("rows", 3); err != nil || ok {
			return struct{}{}, errors.New("rollback retained discarded row")
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(commit.Changes) != 1 || len(commit.Changes[0].RowChanges) != 2 {
		t.Fatalf("changes = %#v", commit.Changes)
	}
	rows := commit.Changes[0].RowChanges
	if rows[0].ID != 1 || !rows[0].HadBefore || rows[0].HasAfter {
		t.Fatalf("row 1 = %#v", rows[0])
	}
	if rows[1].ID != 2 || rows[1].HadBefore || !rows[1].HasAfter {
		t.Fatalf("row 2 = %#v", rows[1])
	}
}

func TestCollectionAccessReturnsIndependentNestedValues(t *testing.T) {
	store := NewResourceStore([]ResourceDescriptor{{
		ID: "rows", Lifetime: ResourceSessionLifetime, Kind: ResourceCollection,
		Initial: CollectionResource(map[RowID]ResourceValue{
			1: CollectionResource(map[RowID]ResourceValue{2: TextResource("original")}),
		}),
	}})
	draft := store.BeginDraft()
	effects := NewEffectSet([]ResourceID{"rows"}, []ResourceID{"rows"})
	read, write := draft.Read(effects), draft.Write(effects)
	for _, access := range []struct {
		name string
		get  func(ResourceID) (ResourceValue, error)
		rows func(ResourceID) (map[RowID]ResourceValue, error)
		row  func(ResourceID, RowID) (ResourceValue, bool, error)
	}{
		{"read", read.Get, read.Rows, read.Row},
		{"write", write.Get, write.Rows, write.Row},
	} {
		t.Run(access.name, func(t *testing.T) {
			value, err := access.get("rows")
			if err != nil {
				t.Fatal(err)
			}
			rows, err := access.rows("rows")
			if err != nil {
				t.Fatal(err)
			}
			row, ok, err := access.row("rows", 1)
			if err != nil || !ok {
				t.Fatalf("row: ok=%v err=%v", ok, err)
			}
			value.collection[1].collection[2] = TextResource("changed through get")
			rows[1].collection[2] = TextResource("changed through rows")
			row.collection[2] = TextResource("changed through row")
			stored, ok, err := read.Row("rows", 1)
			if err != nil || !ok {
				t.Fatalf("stored row: ok=%v err=%v", ok, err)
			}
			child, _ := stored.Row(2)
			if text, _ := child.Text(); text != "original" {
				t.Fatalf("stored nested row = %q", text)
			}
		})
	}
}

func TestCollectionRowLookupDoesNotCloneOtherRows(t *testing.T) {
	rows := make(map[RowID]ResourceValue, 1024)
	for id := range 1024 {
		rows[RowID(id)] = TextResource("value")
	}
	read := ResourceRead{
		values:  map[ResourceID]ResourceValue{"rows": CollectionResource(rows)},
		effects: NewEffectSet([]ResourceID{"rows"}, nil),
	}
	allocations := testing.AllocsPerRun(100, func() {
		_, ok, err := read.Row("rows", 1)
		if err != nil || !ok {
			t.Fatalf("row: ok=%v err=%v", ok, err)
		}
	})
	if allocations > 1 {
		t.Fatalf("one scalar row lookup allocated %v times", allocations)
	}
}
