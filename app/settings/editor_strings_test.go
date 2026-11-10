package settings

import (
	"strings"
	"testing"
)

// Every catalog entry is rendered from a display string; a missing key
// falls back to its internal settings key.
func TestEveryCatalogEntryHasDisplayText(t *testing.T) {
	t.Parallel()
	for _, card := range DefaultState().Data().Policies {
		if card.Title == "" || card.Title == card.ID || strings.HasPrefix(card.Title, "settings.") {
			t.Errorf("policy %s has no title", card.ID)
		}
		if strings.TrimSpace(card.Description) == "" {
			t.Errorf("policy %s has no description", card.ID)
		}
		for _, option := range card.Options {
			if option.Label == "" || strings.HasPrefix(option.Label, "settings.") {
				t.Errorf("policy %s option %s has no label", card.ID, option.Key)
			}
			if option.Description == "" || strings.HasPrefix(option.Description, "settings.") {
				t.Errorf("policy %s option %s has no description", card.ID, option.Key)
			}
			for _, choice := range option.Choices {
				if choice.Label == "" || strings.HasPrefix(choice.Label, "settings.") {
					t.Errorf("policy %s option %s choice %v has no label", card.ID, option.Key, choice.Value)
				}
			}
		}
	}
}
