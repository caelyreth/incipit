package page

import (
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/egoist/mygo/ui"
)

// The page's menu offers corrections over a word the spelling policy
// marks, first, as the system's menus do: the host's guesses for the word
// the page presents in the misspelled group at the caret, a secondary
// click having put the caret there. Choosing one asks the rules to write
// it (profile.CorrectSpelling).

// guessesShown is how many corrections the menu offers at most.
const guessesShown = 5

// guessed is the word the menu last asked guesses for, and its guesses,
// so a menu standing open asks once.
type guessed struct {
	word    string
	guesses []string
}

// misspelledWord is the misspelled word holding the caret, or "".
func (el *Element) misspelledWord() string {
	ed := el.Editor
	sel := ed.Selection()
	if ed.layout == nil || sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		return ""
	}
	_, _, word, ok := ed.layout.runAt(sel.Head, profile.GroupMisspelled)
	if !ok {
		return ""
	}
	return word
}

// guessItems puts the corrections for the misspelled word at the caret
// in the menu, each writing itself over the word; it reports whether it
// put any.
func (el *Element) guessItems(m *ui.Menu, services ui.Services) bool {
	word := el.misspelledWord()
	if word == "" || el.Editor.Guesses == nil {
		return false
	}
	if el.guessed.word != word {
		guesses := el.Editor.Guesses(word)
		el.guessed = guessed{word: word, guesses: guesses[:min(len(guesses), guessesShown)]}
	}
	for _, g := range el.guessed.guesses {
		if m.Item(g).Chosen() {
			el.run(profile.CorrectSpelling{Word: g}, services, el.Editor.now())
		}
	}
	return len(el.guessed.guesses) > 0
}
