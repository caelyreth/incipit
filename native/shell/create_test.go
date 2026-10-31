package shell

import "testing"

func TestCreatorLabelsUseCapitalizedNouns(t *testing.T) {
	for label, want := range map[string]string{
		"New sheet":   "Sheet",
		"New chapter": "Chapter",
		"New group":   "Group",
	} {
		if got := creatorLabel(label); got != want {
			t.Errorf("creatorLabel(%q) = %q, want %q", label, got, want)
		}
	}
}
