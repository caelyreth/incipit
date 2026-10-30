package settings

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
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
