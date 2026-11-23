package settings

import (
	"testing"

	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/app/phrases"
)

// Everything a policy declares for people to read has words, every key a
// text names is a declared command's, and every declared key parses: a
// renamed command or a new option cannot leave a card, a shortcut or a
// live key behind.
func TestEveryCatalogEntryHasDisplayText(t *testing.T) {
	t.Parallel()
	state := DefaultState()
	catalog := state.Program.Catalog()
	commands := map[keymap.CommandID]bool{}
	for _, c := range catalog.Commands {
		commands[keymap.CommandID(c.ID)] = true
	}
	for _, c := range state.Keymap.Commands() {
		commands[c.ID] = true
	}
	want := func(what, key string) {
		if !phrases.Has(key) {
			t.Errorf("%s: no words for %q", what, key)
			return
		}
		for _, id := range References(phrases.Text(key)) {
			if !commands[id] {
				t.Errorf("%s: %q names no command %s", what, key, id)
			}
		}
	}
	for _, p := range catalog.Policies {
		want("policy "+p.ID, p.Label)
		want("policy "+p.ID, p.Description)
		for _, o := range p.Options[1:] { // [0] is the policy's switch
			want("option "+o.Key, o.Label)
			want("option "+o.Key, o.Description)
			for i, label := range o.ChoiceLabels {
				want("option "+o.Key+" choice "+string(rune('0'+i)), label)
			}
		}
	}
	for _, c := range catalog.Commands {
		want("command "+string(c.ID), c.Label)
		if _, ok := keymap.ParseChord(c.Key); c.Key != "" && !ok {
			t.Errorf("command %s: key %q does not parse", c.ID, c.Key)
		}
	}
}
