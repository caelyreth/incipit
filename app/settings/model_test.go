package settings

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
)

func TestCategoryOrderAndMetadata(t *testing.T) {
	t.Parallel()
	if len(AllCategories) != 4 {
		t.Fatalf("categories = %d", len(AllCategories))
	}
	for _, category := range AllCategories {
		if category.Title() == "" || category.Description() == "" {
			t.Fatalf("missing metadata for %d", category)
		}
	}
}

func TestChangeKindsRemainTyped(t *testing.T) {
	t.Parallel()
	command := kernel.NewCommandID("markup.strong")
	binding := kernel.NewBindingTarget(
		kernel.EditorBindingContext,
		"b",
		kernel.ModifiersPrimary,
	)
	if Remap(command, binding).Kind != ChangeBinding || Unbind(command).Kind != ChangeUnbind ||
		ResetBinding(command).Kind != ChangeResetBinding {
		t.Fatal("binding change kind lost")
	}
	presentation := ToggleBoolean(PresentationBoolean(PresentationZen), true)
	if presentation.Kind != ChangeBoolean || !presentation.Value {
		t.Fatalf("presentation change = %#v", presentation)
	}
}
