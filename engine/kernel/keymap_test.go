package kernel

import "testing"

func TestModifiersParseAndNormalize(t *testing.T) {
	t.Parallel()
	modifiers, err := ParseModifiers("super", "primary", "shift", "primary")
	if err != nil {
		t.Fatal(err)
	}
	if !modifiers.Has(ModifierPrimary) || !modifiers.Has(ModifierShift) || !modifiers.Has(ModifierSuper) {
		t.Fatalf("modifiers = %v", modifiers)
	}
	if got := modifiers.All(); len(got) != 3 {
		t.Fatalf("all = %#v", got)
	}
	if _, err := ParseModifier("hyper"); err == nil {
		t.Fatal("unknown modifier accepted")
	}
}

func TestBindingTargetValidation(t *testing.T) {
	t.Parallel()
	for _, target := range []BindingTarget{
		NewBindingTarget("", "k", ModifiersPrimary),
		NewBindingTarget(EditorBindingContext, "", ModifiersPrimary),
		NewBindingTarget(EditorBindingContext, "a\n", ModifiersPrimary),
	} {
		if err := target.Validate(); err == nil {
			t.Fatalf("invalid target accepted: %#v", target)
		}
	}
}

func TestPreparedKeymapDeduplicatesAndRejectsConflicts(t *testing.T) {
	t.Parallel()
	keymap := newPreparedKeymap()
	target := NewBindingTarget(EditorBindingContext, "b", ModifiersPrimary)
	if err := keymap.bind("marks.toggle", []BindingTarget{target, target}); err != nil {
		t.Fatal(err)
	}
	if bindings := keymap.BindingsFor("marks.toggle"); len(bindings) != 1 {
		t.Fatalf("bindings = %#v", bindings)
	}
	if command, ok := keymap.CommandForKey(EditorBindingContext, "b", ModifiersPrimary); !ok || command != "marks.toggle" {
		t.Fatalf("lookup = %q,%v", command, ok)
	}
	if err := keymap.bind("other", []BindingTarget{target}); err == nil {
		t.Fatal("conflict accepted")
	}
}
