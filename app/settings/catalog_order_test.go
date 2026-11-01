package settings

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// The Editor page lists its sections in a fixed order.
func TestEditorSectionOrder(t *testing.T) {
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0)
	for _, policy := range sortedPolicies(catalog) {
		if info := infoForPolicy(policy.ID); info.toggleable {
			got = append(got, info.label)
		}
	}
	want := []string{
		"Writing Profile", "Typography", "Automatic pairs", "Scope navigation",
		"Word navigation", "CJK refinement", "Undo & redo",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("sections = %q, want %q", got, want)
	}
}

func TestDismissSelectionAppearsInSettingsWithEscape(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	data := BuildData(
		editorHost(t, catalog),
		activeSession(),
		DefaultPresentation(),
		DefaultAppearance(),
	)
	shortcut, ok := data.Shortcut(editor.TextDismissSelection.ID())
	if !ok {
		t.Fatal("dismiss selection is missing from settings")
	}
	if shortcut.Label != "Dismiss selection" ||
		!slices.Equal(shortcut.Bindings, []kernel.BindingTarget{
			kernel.NewBindingTarget(kernel.EditorBindingContext, "escape", 0),
		}) {
		t.Fatalf("dismiss selection shortcut = %#v", shortcut)
	}
}
