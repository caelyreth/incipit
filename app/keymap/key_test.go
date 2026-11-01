package keymap

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

func preparedKeymap(t *testing.T, bindings ...kernel.BindingSpec) *kernel.PreparedPolicyConfiguration {
	t.Helper()
	commands := make([]kernel.CommandID, 0, len(bindings))
	seen := make(map[kernel.CommandID]struct{})
	for _, binding := range bindings {
		if binding.Target.Key() == "" {
			continue
		}
		if _, ok := seen[binding.Command]; !ok {
			seen[binding.Command] = struct{}{}
			commands = append(commands, binding.Command)
		}
	}
	catalog, err := kernel.LinkPolicyCatalog([]kernel.PolicyDescriptor{{
		ID:               "test.keys",
		EnabledByDefault: true,
		Commands:         commands,
	}}, bindings...)
	if err != nil {
		t.Fatal(err)
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return host.PreparedSnapshot()
}

func editorKeymap(t *testing.T) *kernel.PreparedPolicyConfiguration {
	t.Helper()
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return host.PreparedSnapshot()
}

func target(context kernel.BindingContext, key string, modifiers kernel.Modifiers) kernel.BindingTarget {
	return kernel.NewBindingTarget(context, key, modifiers)
}

func TestNormalizeModifiersUsesPlatformPrimary(t *testing.T) {
	t.Parallel()
	native := ui.Ctrl | ui.Alt | ui.Shift | ui.Super
	if got, want := NormalizeModifiers(native, PlatformDarwin),
		kernel.ModifiersControl|kernel.ModifiersAlt|kernel.ModifiersShift|kernel.ModifiersPrimary; got != want {
		t.Fatalf("darwin modifiers = %v, want %v", got, want)
	}
	if got, want := NormalizeModifiers(native, PlatformOther),
		kernel.ModifiersPrimary|kernel.ModifiersAlt|kernel.ModifiersShift|kernel.ModifiersSuper; got != want {
		t.Fatalf("other modifiers = %v, want %v", got, want)
	}
	if got := NormalizeModifiers(ui.Ctrl, PlatformDarwin); got != kernel.ModifiersControl {
		t.Fatalf("darwin control = %v", got)
	}
	if got := NormalizeModifiers(ui.Super, PlatformDarwin); got != kernel.ModifiersPrimary {
		t.Fatalf("darwin command = %v", got)
	}
}

func TestKeyNamesFollowBindingVocabulary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key  ui.Key
		name string
	}{
		{ui.KeyLeft, "left"},
		{ui.KeyEnter, "enter"},
		{ui.KeyPageDown, "pagedown"},
		{ui.KeyF12, "f12"},
		{ui.KeyA, "a"},
		{ui.Key7, "7"},
		{ui.KeyComma, ","},
		{ui.KeyBracketLeft, "["},
		{ui.KeyBracketRight, "]"},
		{ui.KeyBackslash, "\\"},
		{ui.KeySpace, "space"},
	}
	for _, test := range tests {
		if got, ok := KeyName(test.key); !ok || got != test.name {
			t.Errorf("KeyName(%d) = %q,%v, want %q,true", test.key, got, ok, test.name)
		}
	}
	if _, ok := KeyName(ui.KeyUnknown); ok {
		t.Fatal("unknown key received a binding name")
	}
}

func TestEscapeResolvesToDismissSelection(t *testing.T) {
	t.Parallel()
	command, ok := ResolveKey(editorKeymap(t), ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyEscape,
	}, PlatformDarwin)
	if !ok || command != editor.TextDismissSelection.ID() {
		t.Fatalf("escape = %q,%v, want %q", command, ok, editor.TextDismissSelection.ID())
	}
}

func TestResolveKeyPrefersLogicalAndFallsBackToPhysical(t *testing.T) {
	t.Parallel()
	configuration := preparedKeymap(t,
		kernel.BindingSpec{
			Command: editor.TextLeft.ID(),
			Target:  target(kernel.EditorBindingContext, "a", kernel.ModifiersPrimary),
		},
		kernel.BindingSpec{
			Command: editor.TextRight.ID(),
			Target:  target(kernel.EditorBindingContext, "s", kernel.ModifiersPrimary),
		},
	)
	logical := ui.InputEvent{
		Kind:        ui.InputKeyDown,
		Key:         ui.KeyA,
		PhysicalKey: ui.KeyS,
		Mods:        ui.Super,
	}
	if got, ok := ResolveKey(configuration, logical, PlatformDarwin); !ok ||
		got != editor.TextLeft.ID() {
		t.Fatalf("logical lookup = %q,%v", got, ok)
	}
	physical := ui.InputEvent{
		Kind:        ui.InputKeyDown,
		Key:         ui.KeyUnknown,
		PhysicalKey: ui.KeyS,
		Mods:        ui.Super,
	}
	if got, ok := ResolveKey(configuration, physical, PlatformDarwin); !ok ||
		got != editor.TextRight.ID() {
		t.Fatalf("physical lookup = %q,%v", got, ok)
	}
	unmodified := physical
	unmodified.Mods = 0
	if _, ok := ResolveKey(configuration, unmodified, PlatformDarwin); ok {
		t.Fatal("positional shortcut resolved without a shortcut modifier")
	}
	otherLogical := physical
	otherLogical.Key = ui.KeyB
	if _, ok := ResolveKey(configuration, otherLogical, PlatformDarwin); ok {
		t.Fatal("physical key replaced a logical key that is not configured")
	}
	targets := BindingTargets(logical, PlatformDarwin)
	if len(targets) == 0 || targets[0].Context() != kernel.EditorBindingContext {
		t.Fatalf("binding targets = %#v", targets)
	}
}

func TestDefaultShortcutLookupHonorsPlatformPrimary(t *testing.T) {
	t.Parallel()
	configuration := editorKeymap(t)
	selectAll := ui.InputEvent{
		Kind: ui.InputKeyDown,
		Key:  ui.KeyA,
		Mods: ui.Super,
	}
	if got, ok := ResolveKey(configuration, selectAll, PlatformDarwin); !ok ||
		got != editor.TextSelectAll.ID() {
		t.Fatalf("darwin select all = %q,%v", got, ok)
	}
	if _, ok := ResolveKey(configuration, selectAll, PlatformOther); ok {
		t.Fatal("Super resolved as primary outside macOS")
	}
	other := selectAll
	other.Mods = ui.Ctrl
	if got, ok := ResolveKey(configuration, other, PlatformOther); !ok ||
		got != editor.TextSelectAll.ID() {
		t.Fatalf("other select all = %q,%v", got, ok)
	}
}

func TestDefaultShortcutMatrixRoutesEveryEditorCommandFamily(t *testing.T) {
	t.Parallel()
	configuration := editorKeymap(t)
	tests := []struct {
		name    string
		event   ui.InputEvent
		kind    ActionKind
		command kernel.CommandID
	}{
		{
			name:    "text movement",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyRight},
			kind:    ActionText,
			command: editor.TextRight.ID(),
		},
		{
			name:    "visual movement",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyDown},
			kind:    ActionSurface,
			command: editor.SurfaceDownCommandID,
		},
		{
			name:    "visual selection",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyDown, Mods: ui.Shift},
			kind:    ActionSurface,
			command: editor.SurfaceSelectDownCommandID,
		},
		{
			name:    "text selection",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyRight, Mods: ui.Shift},
			kind:    ActionText,
			command: editor.TextSelectRight.ID(),
		},
		{
			name:    "word movement",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyRight, Mods: ui.Alt},
			kind:    ActionWord,
			command: editor.WordNext.ID(),
		},
		{
			name:    "word deletion",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyBackspace, Mods: ui.Alt},
			kind:    ActionWord,
			command: editor.WordDeletePrevious.ID(),
		},
		{
			name:    "history undo",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, Mods: ui.Super},
			kind:    ActionHistory,
			command: editor.HistoryUndo.ID(),
		},
		{
			name:    "history redo",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, Mods: ui.Super | ui.Shift},
			kind:    ActionHistory,
			command: editor.HistoryRedo.ID(),
		},
		{
			name:    "strong markup",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyB, Mods: ui.Super},
			kind:    ActionMarkup,
			command: editor.MarkupStrong.ID(),
		},
		{
			name:    "emphasis markup",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyI, Mods: ui.Super},
			kind:    ActionMarkup,
			command: editor.MarkupEmphasis.ID(),
		},
		{
			name:    "tab navigation",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyTab},
			kind:    ActionNavigation,
			command: editor.TabCommand().ID,
		},
		{
			name:    "search commands",
			event:   ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyK, Mods: ui.Super},
			kind:    ActionSurface,
			command: editor.SurfaceCommandsCommandID,
		},
	}
	for _, test := range tests {
		command, ok := ResolveKey(configuration, test.event, PlatformDarwin)
		if !ok || command != test.command {
			t.Errorf("%s lookup = %q,%v, want %q", test.name, command, ok, test.command)
			continue
		}
		action, ok := ResolveCommand(command)
		if !ok || action.Kind != test.kind || action.ID != test.command {
			t.Errorf("%s action = %#v,%v", test.name, action, ok)
		}
	}
}

func TestNonKeyEventsProduceNoTargets(t *testing.T) {
	t.Parallel()
	for _, kind := range []ui.InputKind{
		ui.InputKeyUp,
		ui.InputText,
		ui.InputCompose,
		ui.InputCommand,
		ui.InputPointerDown,
	} {
		event := ui.InputEvent{Kind: kind, Key: ui.KeyRight}
		if targets := BindingTargets(event, PlatformDarwin); len(targets) != 0 {
			t.Fatalf("%v produced targets: %#v", kind, targets)
		}
	}
}
