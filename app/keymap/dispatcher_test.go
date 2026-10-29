package keymap

import (
	"errors"
	"reflect"
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

type fakeExecutor struct {
	configuration *kernel.PreparedPolicyConfiguration
	composing     bool
	commits       int
	cancels       int
	executed      []Action
	executeErr    error
	commitErr     error
}

func (f *fakeExecutor) Configuration() *kernel.PreparedPolicyConfiguration {
	return f.configuration
}

func (f *fakeExecutor) IsComposing() bool { return f.composing }

func (f *fakeExecutor) CommitComposition() error {
	f.commits++
	if f.commitErr != nil {
		return f.commitErr
	}
	f.composing = false
	return nil
}

func (f *fakeExecutor) CancelComposition() bool {
	f.cancels++
	if !f.composing {
		return false
	}
	f.composing = false
	return true
}

func (f *fakeExecutor) Execute(action Action) error {
	f.executed = append(f.executed, action)
	return f.executeErr
}

func TestDispatcherRoutesKeyDownShortcut(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{configuration: editorKeymap(t)}
	dispatcher := NewDispatcherForPlatform(executor, PlatformDarwin)
	result := dispatcher.HandleEvent(ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyA,
		Mods: ui.Super,
	})
	if !result.Consumed || !result.Dispatched || result.Reason != ResultDispatched ||
		result.Command != editor.TextSelectAll.ID() || result.Action.Kind != ActionText ||
		result.Action.Text != editor.TextSelectAll {
		t.Fatalf("result = %#v", result)
	}
	if len(executor.executed) != 1 || executor.executed[0].CommandID() != editor.TextSelectAll.ID() {
		t.Fatalf("executed = %#v", executor.executed)
	}
	if !dispatcher.HandleInput(ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyLeft,
	}) {
		t.Fatal("HandleInput did not consume a configured shortcut")
	}
}

func TestDispatcherSuppressesRepeatedNonNavigationCommands(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{configuration: editorKeymap(t)}
	dispatcher := NewDispatcherForPlatform(executor, PlatformDarwin)
	events := []struct {
		event  ui.InputEvent
		reason ResultReason
	}{
		{
			event: ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyRight, Repeat: true},
		},
		{
			event: ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyRight, Mods: ui.Alt, Repeat: true},
		},
		{
			event: ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, Mods: ui.Super, Repeat: true},
		},
		{
			event:  ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyI, Mods: ui.Super, Repeat: true},
			reason: ResultRepeatSuppressed,
		},
		{
			event:  ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyTab, Repeat: true},
			reason: ResultRepeatSuppressed,
		},
	}
	for index := range events {
		result := dispatcher.HandleEvent(events[index].event)
		if !result.Consumed {
			t.Fatalf("event %d was not consumed", index)
		}
		if index < 3 {
			if !result.Dispatched || result.Reason != ResultDispatched {
				t.Fatalf("repeatable event %d = %#v", index, result)
			}
		} else if result.Dispatched || result.Reason != events[index].reason {
			t.Fatalf("suppressed event %d = %#v", index, result)
		}
	}
	if len(executor.executed) != 3 {
		t.Fatalf("executed = %#v", executor.executed)
	}
}

func TestDispatcherGatesKeysButNeverConsumesTextOrComposition(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{
		configuration: editorKeymap(t),
		composing:     true,
	}
	dispatcher := NewDispatcherForPlatform(executor, PlatformDarwin)

	result := dispatcher.HandleEvent(ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyRight,
	})
	if !result.Consumed || result.Dispatched || result.Reason != ResultCompositionSuppressed {
		t.Fatalf("composed key = %#v", result)
	}
	result = dispatcher.HandleEvent(ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyEscape,
	})
	if !result.Consumed || result.Dispatched || result.Reason != ResultCompositionCancelled ||
		executor.cancels != 1 || executor.composing {
		t.Fatalf("escape = %#v executor=%#v", result, executor)
	}

	executor.composing = true
	for _, kind := range []ui.InputKind{ui.InputText, ui.InputCompose, ui.InputKeyUp} {
		result = dispatcher.HandleEvent(ui.InputEvent{Kind: kind, Key: ui.KeyRight, Text: "x"})
		if result.Consumed || result.Dispatched || result.Reason != ResultIgnored {
			t.Fatalf("%v = %#v", kind, result)
		}
	}
	if len(executor.executed) != 0 {
		t.Fatalf("text events executed commands: %#v", executor.executed)
	}
}

func TestDispatcherCommitsCompositionBeforeMenuAction(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{
		configuration: editorKeymap(t),
		composing:     true,
	}
	dispatcher := NewDispatcherForPlatform(executor, PlatformDarwin)
	result := dispatcher.HandleEvent(ui.InputEvent{Kind: ui.InputCommand, Text: "undo"})
	if !result.Consumed || !result.Dispatched || result.Reason != ResultDispatched ||
		executor.commits != 1 || len(executor.executed) != 1 ||
		executor.executed[0].History != editor.HistoryUndo {
		t.Fatalf("undo menu = %#v executor=%#v", result, executor)
	}
}

func TestDispatcherReportsMenuFailuresAndPolicyGates(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{configuration: preparedKeymap(t,
		kernel.BindingSpec{
			Command: CopyCommandID,
			Target:  target(kernel.EditorBindingContext, "c", kernel.ModifiersPrimary),
		},
		kernel.BindingSpec{
			Command: PasteCommandID,
			Target:  target(kernel.EditorBindingContext, "v", kernel.ModifiersPrimary),
		},
	)}
	dispatcher := NewDispatcherForPlatform(executor, PlatformDarwin)

	sentinel := errors.New("clipboard unavailable")
	executor.executeErr = sentinel
	result := dispatcher.HandleEvent(ui.InputEvent{Kind: ui.InputCommand, Text: "copy"})
	if !result.Consumed || result.Dispatched || result.Reason != ResultFailed ||
		!errors.Is(result.Err, sentinel) || result.Action.Clipboard != ClipboardCopy {
		t.Fatalf("copy failure = %#v", result)
	}

	executor.executeErr = nil
	executor.composing = true
	executor.commitErr = errors.New("stale composition")
	result = dispatcher.HandleEvent(ui.InputEvent{Kind: ui.InputCommand, Text: "paste"})
	if !result.Consumed || result.Dispatched || result.Reason != ResultCompositionCommitFailed ||
		result.Err == nil || executor.commits != 1 || len(executor.executed) != 1 {
		t.Fatalf("paste commit failure = %#v executor=%#v", result, executor)
	}

	executor.commitErr = nil
	executor.composing = false
	executor.configuration = preparedKeymap(t)
	result = dispatcher.HandleEvent(ui.InputEvent{Kind: ui.InputCommand, Text: "undo"})
	if !result.Consumed || result.Dispatched || result.Reason != ResultDisabled {
		t.Fatalf("disabled undo = %#v", result)
	}
	result = dispatcher.HandleEvent(ui.InputEvent{Kind: ui.InputCommand, Text: "underline"})
	if result.Consumed || result.Reason != ResultUnsupported {
		t.Fatalf("unsupported menu = %#v", result)
	}
}

func TestDispatcherRoutesExternalConfiguredCommands(t *testing.T) {
	t.Parallel()
	configuration := preparedKeymap(t, kernel.BindingSpec{
		Command: kernel.CommandID("host.save"),
		Target:  target(kernel.EditorBindingContext, "s", kernel.ModifiersPrimary),
	})
	executor := &fakeExecutor{configuration: configuration}
	dispatcher := NewDispatcherForPlatform(executor, PlatformOther)
	result := dispatcher.HandleEvent(ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyS,
		Mods: ui.Ctrl,
	})
	if !result.Consumed || !result.Dispatched ||
		result.Action.Kind != ActionExternal || result.Action.ID != "host.save" {
		t.Fatalf("external result = %#v", result)
	}
	if !reflect.DeepEqual(executor.executed, []Action{result.Action}) {
		t.Fatalf("executed = %#v", executor.executed)
	}
}

func TestDispatcherRoutesConfiguredClipboardShortcut(t *testing.T) {
	t.Parallel()
	configuration := preparedKeymap(t, kernel.BindingSpec{
		Command: CopyCommandID,
		Target:  target(kernel.EditorBindingContext, "c", kernel.ModifiersPrimary),
	})
	executor := &fakeExecutor{configuration: configuration}
	dispatcher := NewDispatcherForPlatform(executor, PlatformDarwin)
	result := dispatcher.HandleEvent(ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyC,
		Mods: ui.Super,
	})
	if !result.Consumed || !result.Dispatched ||
		result.Action.Kind != ActionClipboard ||
		result.Action.Clipboard != ClipboardCopy {
		t.Fatalf("copy shortcut = %#v", result)
	}
}

func TestDispatcherResolvesRemappedSurfaceCommand(t *testing.T) {
	t.Parallel()
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	remapped := target(
		kernel.EditorBindingContext,
		"n",
		kernel.ModifiersPrimary|kernel.ModifiersAlt,
	)
	if err := host.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.Remap(editor.SurfaceNewPageCommandID, remapped)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{configuration: host.PreparedSnapshot()}
	dispatcher := NewDispatcherForPlatform(executor, PlatformOther)
	result := dispatcher.HandleEvent(ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyN,
		Mods: ui.Ctrl | ui.Alt,
	})
	if !result.Consumed || !result.Dispatched || result.Command != editor.SurfaceNewPageCommandID ||
		result.Action.Kind != ActionSurface ||
		result.Action.Surface.ID != editor.SurfaceNewPageCommandID {
		t.Fatalf("remapped surface command = %#v", result)
	}
	if len(executor.executed) != 1 ||
		executor.executed[0].Surface.ID != editor.SurfaceNewPageCommandID {
		t.Fatalf("executed = %#v", executor.executed)
	}
	if _, ok := ResolveKey(executor.configuration, ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyN,
		Mods: ui.Ctrl,
	}, PlatformOther); ok {
		t.Fatal("surface remap retained the previous default binding")
	}
}

func TestDispatcherNeverPanicsOnMissingState(t *testing.T) {
	t.Parallel()
	event := ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyRight}
	var nilDispatcher *Dispatcher
	if result := nilDispatcher.HandleEvent(event); result.Consumed ||
		!errors.Is(result.Err, ErrNoExecutor) {
		t.Fatalf("nil dispatcher = %#v", result)
	}
	dispatcher := NewDispatcher(nil)
	if result := dispatcher.HandleEvent(event); result.Consumed ||
		!errors.Is(result.Err, ErrNoExecutor) {
		t.Fatalf("nil executor = %#v", result)
	}
	var typedNil *fakeExecutor
	dispatcher = NewDispatcher(typedNil)
	if result := dispatcher.HandleEvent(event); result.Consumed ||
		!errors.Is(result.Err, ErrNoExecutor) {
		t.Fatalf("typed nil executor = %#v", result)
	}
	executor := &fakeExecutor{}
	dispatcher = NewDispatcherForPlatform(executor, PlatformDarwin)
	if result := dispatcher.HandleEvent(event); result.Consumed ||
		result.Reason != ResultIgnored {
		t.Fatalf("nil configuration = %#v", result)
	}
}
