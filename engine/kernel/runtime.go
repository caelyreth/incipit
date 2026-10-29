package kernel

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

// PolicyRuntime coordinates the installed policy catalog, prepared
// configuration, linked event program, resource schema, and semantic engine.
// Its state changes share one publication revision.
type PolicyRuntime struct {
	engine   *Engine
	policies *PolicyHost
	program  *EventProgram
}

// ConfigurationOutcome is the owned output of a prepared reconfiguration.
type ConfigurationOutcome[P any] struct {
	Prepared P
	Revision uint64
	Changed  bool
}

// RuntimeErrorKind identifies which runtime subsystem rejected an operation.
type RuntimeErrorKind uint8

const (
	RuntimeErrorPolicy RuntimeErrorKind = iota + 1
	RuntimeErrorDispatch
	RuntimeErrorEngine
)

// RuntimeError preserves the underlying error for errors.Is/errors.As while
// identifying the subsystem boundary.
type RuntimeError struct {
	Kind RuntimeErrorKind
	Err  error
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch e.Kind {
	case RuntimeErrorPolicy:
		return "policy runtime: " + e.Err.Error()
	case RuntimeErrorDispatch:
		return "policy runtime: " + e.Err.Error()
	case RuntimeErrorEngine:
		return "policy runtime: " + e.Err.Error()
	default:
		return "policy runtime: unknown error"
	}
}

func (e *RuntimeError) Unwrap() error { return e.Err }

// ReconfigurationErrorKind distinguishes runtime validation from presentation
// preparation failure.
type ReconfigurationErrorKind uint8

const (
	ReconfigurationRuntime ReconfigurationErrorKind = iota + 1
	ReconfigurationPreparation
)

// ReconfigurationError reports why a configuration candidate was not published.
type ReconfigurationError[E any] struct {
	Kind        ReconfigurationErrorKind
	Runtime     *RuntimeError
	Preparation E
}

func (e *ReconfigurationError[E]) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Kind == ReconfigurationRuntime && e.Runtime != nil {
		return e.Runtime.Error()
	}
	return fmt.Sprintf("configuration preparation failed: %v", e.Preparation)
}

func (e *ReconfigurationError[E]) Unwrap() error {
	if e == nil || e.Runtime == nil {
		return nil
	}
	return e.Runtime
}

// NewPolicyRuntime links a catalog, installs its defaults, and creates the
// document/resource engine without publishing a synthetic revision.
func NewPolicyRuntime(document *model.Document, catalog PolicyCatalog) (*PolicyRuntime, error) {
	host, err := NewPolicyHost(catalog)
	if err != nil {
		return nil, runtimePolicyError(err)
	}
	program, err := LinkConfiguredEventProgram(host.Catalog(), host.Prepared())
	if err != nil {
		return nil, runtimeDispatchError(err)
	}
	resources := NewResourceStore(nil).Configured(host.Catalog(), host.Prepared())
	return &PolicyRuntime{
		engine:   NewEngine(document, resources),
		policies: host,
		program:  program,
	}, nil
}

func (r *PolicyRuntime) Revision() uint64 {
	return r.engine.Revision()
}

func (r *PolicyRuntime) Snapshot() EngineSnapshot {
	return r.engine.Snapshot()
}

// PolicyConfiguration returns an independent source configuration snapshot,
// including disabled policies and explicit unbindings.
func (r *PolicyRuntime) PolicyConfiguration() PolicyConfiguration {
	return r.policies.Configuration()
}

// Configuration returns the installed immutable prepared configuration. Its
// maps and keymap are private and never mutate after installation.
func (r *PolicyRuntime) Configuration() *PreparedPolicyConfiguration {
	return r.policies.Prepared()
}

// Program returns the currently installed linked event program.
func (r *PolicyRuntime) Program() *EventProgram { return r.program }

// Catalog returns an independent linked catalog snapshot.
func (r *PolicyRuntime) Catalog() PolicyCatalog { return r.policies.Catalog() }

// Reconfigure atomically installs a valid policy configuration candidate.
func (r *PolicyRuntime) Reconfigure(operation func(*PolicyConfiguration) error) error {
	outcome, err := r.ReconfigurePrepared(operation, func(EngineView, *PreparedPolicyConfiguration) (struct{}, error) {
		return struct{}{}, nil
	})
	if err != nil {
		if err.Kind == ReconfigurationRuntime && err.Runtime != nil {
			return err.Runtime
		}
		panic("runtime: infallible configuration preparation failed")
	}
	_ = outcome
	return nil
}

// ReconfigurePrepared prepares owned presentation against the candidate
// resources and revision before publishing the configuration, program,
// resources, and shared revision as one operation.
func (r *PolicyRuntime) ReconfigurePrepared[P any](
	operation func(*PolicyConfiguration) error,
	prepare func(EngineView, *PreparedPolicyConfiguration) (P, error),
) (ConfigurationOutcome[P], *ReconfigurationError[error]) {
	var zero ConfigurationOutcome[P]
	if r == nil || r.policies == nil || r.engine == nil {
		return zero, &ReconfigurationError[error]{
			Kind:    ReconfigurationRuntime,
			Runtime: runtimePolicyError(&PolicyError{Kind: PolicyErrorUnknownPolicy}),
		}
	}
	candidate := r.policies.Clone()
	if err := candidate.Reconfigure(operation); err != nil {
		return zero, &ReconfigurationError[error]{
			Kind:    ReconfigurationRuntime,
			Runtime: runtimePolicyError(err),
		}
	}
	candidateConfiguration := candidate.Configuration()
	currentConfiguration := r.policies.Configuration()
	changed := !candidateConfiguration.Equal(currentConfiguration)
	var nextProgram *EventProgram
	if changed {
		linked, err := LinkConfiguredEventProgram(candidate.Catalog(), candidate.Prepared())
		if err != nil {
			return zero, &ReconfigurationError[error]{
				Kind:    ReconfigurationRuntime,
				Runtime: runtimeDispatchError(err),
			}
		}
		nextProgram = linked
	}
	resources := r.engine.Resources().Configured(candidate.Catalog(), candidate.Prepared())
	revision := r.engine.Revision()
	if changed {
		var err error
		revision, err = r.engine.NextRevision()
		if err != nil {
			return zero, &ReconfigurationError[error]{
				Kind:    ReconfigurationRuntime,
				Runtime: runtimeEngineError(err),
			}
		}
	}
	prepared, err := r.engine.PrepareConfiguration(
		resources,
		revision,
		func(view EngineView) (P, error) {
			return prepare(view, candidate.Prepared())
		},
	)
	if err != nil {
		return zero, &ReconfigurationError[error]{
			Kind:        ReconfigurationPreparation,
			Preparation: err,
		}
	}
	if changed {
		r.engine.PublishConfiguration(resources, revision)
		r.policies = candidate
		r.program = nextProgram
	}
	return ConfigurationOutcome[P]{
		Prepared: prepared,
		Revision: revision,
		Changed:  changed,
	}, nil
}

// Dispatch executes one event inside the runtime's publication transaction.
func (r *PolicyRuntime) Dispatch(
	event Event,
	invoker EngineInvoker,
) (EngineTransactionOutcome[DispatchOutcome, struct{}], error) {
	return Transact(r.engine, func(draft *EngineDraft) (DispatchOutcome, error) {
		return r.program.DispatchEngine(event, draft, invoker)
	})
}

// DispatchPrepared prepares owned presentation with the actual dispatch
// verdict. Preparation failure publishes nothing.
func (r *PolicyRuntime) DispatchPreparedWithOutcome[P any](
	event Event,
	invoker EngineInvoker,
	prepare func(EngineView, EngineChanges, DispatchOutcome) (P, error),
) (EngineTransactionOutcome[DispatchOutcome, P], error) {
	var outcomeValue DispatchOutcome
	transaction, err := TransactPrepared(
		r.engine,
		func(draft *EngineDraft) (DispatchOutcome, error) {
			outcome, err := r.program.DispatchEngine(event, draft, invoker)
			outcomeValue = outcome
			return outcome, err
		},
		func(view EngineView, changes EngineChanges, _ DispatchOutcome) (P, error) {
			return prepare(view, changes, outcomeValue)
		},
	)
	return transaction, err
}

// DispatchPreparedBatchWithInvoker executes an atomic ordered batch. Every
// event is validated against the pre-write draft and the caller decides whether
// the handled vector is complete during preparation.
func (r *PolicyRuntime) DispatchPreparedBatchWithInvoker[P any](
	events []Event,
	invoker EngineInvoker,
	prepare func(
		EngineView,
		EngineChanges,
		BatchDispatchOutcome,
		*PreparedPolicyConfiguration,
	) (P, error),
) (EngineTransactionOutcome[BatchDispatchOutcome, P], error) {
	var outcomeValue BatchDispatchOutcome
	configuration := r.policies.Prepared()
	transaction, err := TransactPrepared(
		r.engine,
		func(draft *EngineDraft) (BatchDispatchOutcome, error) {
			outcome, err := r.program.DispatchEngineBatch(events, draft, invoker)
			outcomeValue = outcome
			return outcome, err
		},
		func(view EngineView, changes EngineChanges, _ BatchDispatchOutcome) (P, error) {
			return prepare(view, changes, outcomeValue, configuration)
		},
	)
	return transaction, err
}

// TransactPrepared runs a caller-owned state operation through the same
// publication boundary used by dispatches.
func (r *PolicyRuntime) TransactPrepared[P any](
	operation func(*EngineDraft) error,
	prepare func(EngineView, EngineChanges) (P, error),
) (EngineTransactionOutcome[struct{}, P], error) {
	return TransactPrepared(
		r.engine,
		func(draft *EngineDraft) (struct{}, error) {
			return struct{}{}, operation(draft)
		},
		func(view EngineView, changes EngineChanges, _ struct{}) (P, error) {
			return prepare(view, changes)
		},
	)
}

func runtimePolicyError(err error) *RuntimeError {
	return &RuntimeError{Kind: RuntimeErrorPolicy, Err: err}
}

func runtimeDispatchError(err error) *RuntimeError {
	return &RuntimeError{Kind: RuntimeErrorDispatch, Err: err}
}

func runtimeEngineError(err error) *RuntimeError {
	return &RuntimeError{Kind: RuntimeErrorEngine, Err: err}
}
