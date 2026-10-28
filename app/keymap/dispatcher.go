package keymap

import (
	"errors"
	"reflect"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

var ErrNoExecutor = errors.New("keymap: no executor configured")

// ResultReason explains how a MyGo input event was classified.
type ResultReason uint8

const (
	ResultIgnored ResultReason = iota
	ResultDispatched
	ResultRepeatSuppressed
	ResultCompositionSuppressed
	ResultCompositionCancelled
	ResultCompositionCommitFailed
	ResultDisabled
	ResultUnsupported
	ResultFailed
)

func (r ResultReason) String() string {
	switch r {
	case ResultDispatched:
		return "dispatched"
	case ResultRepeatSuppressed:
		return "repeat-suppressed"
	case ResultCompositionSuppressed:
		return "composition-suppressed"
	case ResultCompositionCancelled:
		return "composition-cancelled"
	case ResultCompositionCommitFailed:
		return "composition-commit-failed"
	case ResultDisabled:
		return "disabled"
	case ResultUnsupported:
		return "unsupported"
	case ResultFailed:
		return "failed"
	default:
		return "ignored"
	}
}

// Result is the explicit outcome of handling one MyGo event. Consumed is what
// should be returned from ui.Element.HandleInput; Dispatched distinguishes a
// completed executor call from a recognized but suppressed event.
type Result struct {
	Consumed   bool
	Dispatched bool
	Reason     ResultReason
	Command    kernel.CommandID
	Action     Action
	Err        error
}

// Executor is the narrow boundary implemented by the application controller.
// Configuration returns the current immutable prepared keymap. Composition
// methods run on the UI thread before a menu action. Execute owns document,
// policy, selection, history, clipboard, and host-command effects.
type Executor interface {
	Configuration() *kernel.PreparedPolicyConfiguration
	IsComposing() bool
	CommitComposition() error
	CancelComposition() bool
	Execute(Action) error
}

// Dispatcher maps MyGo events to configured editor actions. It owns no buffer,
// selection, composition, clipboard, or element state.
type Dispatcher struct {
	executor Executor
	platform Platform
}

// NewDispatcher builds a dispatcher for the current operating system.
func NewDispatcher(executor Executor) *Dispatcher {
	return NewDispatcherForPlatform(executor, PlatformCurrent)
}

// NewDispatcherForPlatform builds a dispatcher with an explicit primary
// modifier platform, primarily for tests and deterministic host adapters.
func NewDispatcherForPlatform(executor Executor, platform Platform) *Dispatcher {
	if isNilExecutor(executor) {
		executor = nil
	}
	return &Dispatcher{executor: executor, platform: platform}
}

// HandleInput implements the ui.Element.HandleInput callback contract.
func (d *Dispatcher) HandleInput(event ui.InputEvent) bool {
	return d.HandleEvent(event).Consumed
}

// HandleEvent resolves and dispatches one event. Only key-down and edit-menu
// command events participate; typed text, composition text, key releases, and
// pointer events remain available to MyGo's text-input client.
func (d *Dispatcher) HandleEvent(event ui.InputEvent) Result {
	if d == nil || d.executor == nil {
		return Result{Reason: ResultIgnored, Err: ErrNoExecutor}
	}
	switch event.Kind {
	case ui.InputKeyDown:
		return d.handleKeyDown(event)
	case ui.InputCommand:
		return d.handleMenuCommand(event.Text)
	default:
		return Result{Reason: ResultIgnored}
	}
}

func (d *Dispatcher) handleKeyDown(event ui.InputEvent) Result {
	if d.executor.IsComposing() {
		if event.Key == ui.KeyEscape {
			if d.executor.CancelComposition() {
				return Result{
					Consumed: true,
					Reason:   ResultCompositionCancelled,
				}
			}
		}
		return Result{
			Consumed: true,
			Reason:   ResultCompositionSuppressed,
		}
	}

	configuration := d.executor.Configuration()
	command, ok := ResolveKey(configuration, event, d.platform)
	if !ok {
		return Result{Reason: ResultIgnored}
	}
	action, ok := ResolveEditorCommand(command)
	if !ok {
		action = Action{Kind: ActionExternal, ID: command}
	}
	result := Result{
		Consumed: true,
		Command:  command,
		Action:   action,
	}
	if event.Repeat && !AllowsRepeat(action) {
		result.Reason = ResultRepeatSuppressed
		return result
	}
	if err := d.executor.Execute(action); err != nil {
		result.Reason = ResultFailed
		result.Err = err
		return result
	}
	result.Dispatched = true
	result.Reason = ResultDispatched
	return result
}

func (d *Dispatcher) handleMenuCommand(name string) Result {
	action, ok := MenuAction(name)
	if !ok {
		return Result{Reason: ResultUnsupported}
	}
	result := Result{
		Consumed: true,
		Command:  action.ID,
		Action:   action,
	}
	if action.Kind != ActionClipboard {
		configuration := d.executor.Configuration()
		if configuration == nil || !configuration.IsCommandEnabled(action.ID) {
			result.Reason = ResultDisabled
			return result
		}
	}
	if d.executor.IsComposing() {
		if err := d.executor.CommitComposition(); err != nil {
			result.Reason = ResultCompositionCommitFailed
			result.Err = err
			return result
		}
	}
	if err := d.executor.Execute(action); err != nil {
		result.Reason = ResultFailed
		result.Err = err
		return result
	}
	result.Dispatched = true
	result.Reason = ResultDispatched
	return result
}

func isNilExecutor(executor Executor) bool {
	if executor == nil {
		return true
	}
	value := reflect.ValueOf(executor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
