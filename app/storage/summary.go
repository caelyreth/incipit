package storage

import (
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/internal/wordseg"
)

// excerptRunes is how long an excerpt runs.
const excerptRunes = 120

// Summarize reads a page's summary in one walk of its text blocks: its
// first heading's text as its title, the first text after that as its
// excerpt, and its words.
func Summarize(root *doc.Node) Summary {
	var s Summary
	titled := false
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		for i, slot := range n.Slots() {
			if !slot.IsInline() {
				for _, c := range slot.Children() {
					walk(c)
				}
				continue
			}
			text := strings.Join(strings.Fields(slot.Content().Text()), " ")
			for seg := range wordseg.Segments(text) {
				if seg.WordLike {
					s.Words++
				}
			}
			switch {
			case text == "" || i > 0:
			case !titled && n.Kind() == vocab.Heading:
				s.Title, titled = text, true
			case s.Excerpt == "":
				if r := []rune(text); len(r) > excerptRunes {
					text = string(r[:excerptRunes])
				}
				s.Excerpt = text
			}
		}
	}
	walk(root)
	return s
}
