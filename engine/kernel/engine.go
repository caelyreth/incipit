package kernel

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

// EngineSnapshot is one immutable published state across document, resources,
// and the shared revision.
type EngineSnapshot struct {
	document  model.DocumentSnapshot
	resources ResourceSnapshot
	revision  uint64
}

func (s EngineSnapshot) Revision() uint64                 { return s.revision }
func (s EngineSnapshot) Document() model.DocumentSnapshot { return s.document }
func (s EngineSnapshot) Resources() ResourceSnapshot      { return s.resources }

// EngineChanges reports normalized final changes plus the successful
// text-edit footprint that survived rollback.
type EngineChanges struct {
	document  []model.Change
	textEdits []model.TextEdit
	resources []ResourceChange
}

func (c EngineChanges) Document() []model.Change {
	return append([]model.Change(nil), c.document...)
}

func (c EngineChanges) TextEdits() []model.TextEdit {
	return append([]model.TextEdit(nil), c.textEdits...)
}

func (c EngineChanges) Resources() []ResourceChange {
	resources := make([]ResourceChange, len(c.resources))
	for index, change := range c.resources {
		resources[index] = change.Clone()
	}
	return resources
}

func (c EngineChanges) Empty() bool {
	return len(c.document) == 0 && len(c.resources) == 0
}

func (c EngineChanges) RebaseTarget(target TextTarget, revision uint64) (TextTarget, bool) {
	return target.Rebase(c.textEdits, revision)
}

type EngineTransactionOutcome[T any, P any] struct {
	Value    T
	Prepared P
	Revision uint64
	Changes  EngineChanges
}

// Engine owns the semantic document and resources behind one revision.
type Engine struct {
	document  *model.Document
	resources ResourceStore
	revision  uint64
}

type EngineDraft struct {
	document          *model.DocumentDraft
	resources         *ResourceDraft
	baseRevision      uint64
	textInputConsumed bool
}

type EngineCheckpoint struct {
	document          model.DraftCheckpoint
	resources         ResourceCheckpoint
	textInputConsumed bool
}

func NewEngine(document *model.Document, resources ResourceStore) *Engine {
	if document == nil {
		panic("kernel: engine requires a document")
	}
	return &Engine{document: document, resources: resources, revision: document.Revision()}
}

func (e *Engine) Revision() uint64 { return e.revision }

func (e *Engine) Snapshot() EngineSnapshot {
	return EngineSnapshot{
		document:  e.document.Snapshot(),
		resources: e.resources.Snapshot(),
		revision:  e.revision,
	}
}

func (e *Engine) Document() *model.Document { return e.document }
func (e *Engine) Resources() ResourceStore  { return e.resources }

// NextRevision returns the revision a publication with state changes would use.
func (e *Engine) NextRevision() (uint64, error) {
	if e.revision == ^uint64(0) {
		return 0, &EngineError{Kind: EngineErrorRevisionExhausted}
	}
	return e.revision + 1, nil
}

// PrepareConfiguration exposes candidate resources and the prospective shared
// revision to a fallible preparation callback without publishing either.
func (e *Engine) PrepareConfiguration[P any](
	resources ResourceStore,
	revision uint64,
	prepare func(EngineView) (P, error),
) (P, error) {
	var zero P
	draft := resources.BeginDraft()
	if err := draft.Validate(); err != nil {
		return zero, err
	}
	return prepare(EngineView{
		document:  e.document.BeginDraft(),
		resources: draft,
		revision:  revision,
	})
}

// PublishConfiguration installs resources and a caller-owned shared revision.
// Document content is unchanged; the document revision tracks the engine's
// publication revision so snapshots remain wholly revision-consistent.
func (e *Engine) PublishConfiguration(resources ResourceStore, revision uint64) {
	e.resources = resources
	e.revision = revision
	e.document.SetRevision(revision)
}

// Transact runs a non-fallible preparation transaction.
func Transact[T any](engine *Engine, operation func(*EngineDraft) (T, error)) (EngineTransactionOutcome[T, struct{}], error) {
	return TransactPrepared(engine, operation, func(EngineView, EngineChanges, T) (struct{}, error) {
		return struct{}{}, nil
	})
}

// TransactPrepared validates and prepares owned output before publishing both
// state domains at one revision.
func TransactPrepared[T any, P any](
	engine *Engine,
	operation func(*EngineDraft) (T, error),
	prepare func(EngineView, EngineChanges, T) (P, error),
) (EngineTransactionOutcome[T, P], error) {
	var zero EngineTransactionOutcome[T, P]
	draft := &EngineDraft{
		document:     engine.document.BeginDraft(),
		resources:    engine.resources.BeginDraft(),
		baseRevision: engine.revision,
	}
	value, err := operation(draft)
	if err != nil {
		return zero, err
	}
	if err := draft.Validate(); err != nil {
		return zero, err
	}
	changes := draft.Changes()
	revision := engine.revision
	if !changes.Empty() {
		if engine.revision == ^uint64(0) {
			return zero, &EngineError{Kind: EngineErrorRevisionExhausted}
		}
		revision++
	}
	prepared, err := prepare(EngineView{
		document:  draft.document,
		resources: draft.resources,
		revision:  revision,
	}, changes, value)
	if err != nil {
		return zero, err
	}
	if !changes.Empty() {
		if err := engine.document.PublishDraft(draft.document, revision); err != nil {
			return zero, err
		}
		engine.resources.PublishDraft(draft.resources)
		engine.revision = revision
	} else if !draft.resources.Equal(engine.resources) {
		// Resource-only writes are legitimate engine changes even when no
		// document node changed. The shared revision and document snapshot must
		// move together.
		engine.resources.PublishDraft(draft.resources)
		engine.revision = revision
		engine.document.SetRevision(revision)
	}
	return EngineTransactionOutcome[T, P]{
		Value:    value,
		Prepared: prepared,
		Revision: engine.revision,
		Changes:  changes,
	}, nil
}

func (d *EngineDraft) BaseRevision() uint64           { return d.baseRevision }
func (d *EngineDraft) Document() *model.DocumentDraft { return d.document }
func (d *EngineDraft) Resources() *ResourceDraft      { return d.resources }
func (d *EngineDraft) TextEdits() []model.TextEdit    { return d.document.TextEdits() }

func (d *EngineDraft) Writes() WriteSet {
	return NewWriteSet(d.document.WasWritten(), d.resources.WrittenResources())
}

func (d *EngineDraft) WasWritten(key StateKey) bool {
	if key.Document {
		return d.document.WasWritten()
	}
	return d.resources.WasWritten(key.Resource)
}

func (d *EngineDraft) Changes() EngineChanges {
	resources := d.resources.FinalChanges()
	return EngineChanges{
		document:  d.document.FinalChanges(),
		textEdits: d.document.TextEdits(),
		resources: resources,
	}
}

func (d *EngineDraft) Validate() error {
	if err := d.document.Validate(); err != nil {
		return err
	}
	return d.resources.Validate()
}

func (d *EngineDraft) Checkpoint() EngineCheckpoint {
	return EngineCheckpoint{
		document:          d.document.Checkpoint(),
		resources:         d.resources.Checkpoint(),
		textInputConsumed: d.textInputConsumed,
	}
}

func (d *EngineDraft) Rollback(checkpoint EngineCheckpoint) {
	d.document.Rollback(checkpoint.document)
	d.resources.Rollback(checkpoint.resources)
	d.textInputConsumed = checkpoint.textInputConsumed
}

// EngineView is the validated candidate visible during output preparation.
// Its revision is the revision publication would produce.
type EngineView struct {
	document  *model.DocumentDraft
	resources *ResourceDraft
	revision  uint64
}

func (v EngineView) Revision() uint64               { return v.revision }
func (v EngineView) Document() *model.DocumentDraft { return v.document }
func (v EngineView) Resources() *ResourceDraft      { return v.resources }

type EngineErrorKind uint8

const (
	EngineErrorDocument EngineErrorKind = iota + 1
	EngineErrorResource
	EngineErrorInput
	EngineErrorInvalidEventKey
	EngineErrorOutOfScopeDocumentWrite
	EngineErrorRevisionExhausted
)

type EngineError struct {
	Kind    EngineErrorKind
	Err     error
	Key     string
	Target  model.NodeID
	Changed model.NodeID
}

func (e *EngineError) Error() string {
	switch e.Kind {
	case EngineErrorDocument, EngineErrorResource, EngineErrorInput:
		return e.Err.Error()
	case EngineErrorInvalidEventKey:
		return fmt.Sprintf("command event cannot use reserved input key %s", e.Key)
	case EngineErrorOutOfScopeDocumentWrite:
		return fmt.Sprintf("input targeted at %s cannot mutate document node %s", e.Target, e.Changed)
	case EngineErrorRevisionExhausted:
		return "engine revisions are exhausted"
	default:
		return "unknown engine error"
	}
}

func (e *EngineError) Unwrap() error { return e.Err }
