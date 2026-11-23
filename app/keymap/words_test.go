package keymap

import (
	"testing"

	"github.com/caelyreth/neverland/app/phrases"
)

// Every app command names its label by a message the catalog holds, read
// in the language shown each time it is asked.
func TestAppCommandsHaveWords(t *testing.T) {
	for _, c := range appCommands() {
		if !phrases.Has(c.Words) {
			t.Errorf("%s: no words for %q", c.ID, c.Words)
		}
		if c.Label() == c.Words {
			t.Errorf("%s reads as its key", c.ID)
		}
	}
}
