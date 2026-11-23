package settings

import (
	"strings"

	"github.com/caelyreth/neverland/internal/app/keymap"
)

// Settings text may name an action's key instead of spelling it: a command
// identity in braces, as in "{core.leave} leaves the scope you are in",
// stands for whatever that command is bound to now. The text never goes
// stale when the key changes, and a view draws the reference as the live key,
// which records a new one in place.

// Fragment is one run of settings text: plain words, or a key reference.
type Fragment struct {
	Text    string
	Command keymap.CommandID
}

// Phrase splits text into words and key references, in order.
func Phrase(text string) []Fragment {
	fragments := make([]Fragment, 0, 3)
	for text != "" {
		open := strings.IndexByte(text, '{')
		if open < 0 {
			break
		}
		closing := strings.IndexByte(text[open:], '}')
		if closing < 0 {
			break
		}
		if open > 0 {
			fragments = append(fragments, Fragment{Text: text[:open]})
		}
		fragments = append(fragments, Fragment{Command: keymap.CommandID(text[open+1 : open+closing])})
		text = text[open+closing+1:]
	}
	if text != "" {
		fragments = append(fragments, Fragment{Text: text})
	}
	return fragments
}

// References lists the commands text names, in order.
func References(text string) []keymap.CommandID {
	commands := make([]keymap.CommandID, 0, 1)
	for _, fragment := range Phrase(text) {
		if fragment.Command != "" {
			commands = append(commands, fragment.Command)
		}
	}
	return commands
}

// Spell renders text with each reference written by key, for search,
// assistive technology and anywhere a key cannot be drawn.
func Spell(text string, key func(keymap.CommandID) string) string {
	if !strings.ContainsRune(text, '{') {
		return text
	}
	var b strings.Builder
	for _, fragment := range Phrase(text) {
		if fragment.Command != "" {
			b.WriteString(key(fragment.Command))
			continue
		}
		b.WriteString(fragment.Text)
	}
	return b.String()
}
