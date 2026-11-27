package search

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/incipit/internal/text/query"
)

// commonWords are words of English too common to say what a title is of.
var commonWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "into": true, "about": true,
	"that": true, "this": true, "these": true, "those": true, "are": true, "was": true, "were": true,
	"not": true, "but": true, "you": true, "your": true, "our": true, "its": true, "has": true,
	"have": true, "had": true, "how": true, "what": true, "why": true, "who": true, "when": true,
	"where": true, "all": true, "any": true, "out": true, "over": true, "than": true, "then": true,
	"there": true, "their": true, "they": true, "will": true, "can": true, "untitled": true,
}

// TitleWords extracts up to limit distinctive words from titles, ordered by
// the number of titles containing each word (and first appearance on ties).
func TitleWords(titles []string, limit int) []string {
	type count struct {
		word         string
		titles, from int
	}
	counts := map[string]*count{}
	for _, title := range titles {
		seen := map[string]bool{}
		for _, word := range strings.FieldsFunc(title, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
		}) {
			key := query.Fold(word)
			if utf8.RuneCountInString(key) < 3 || commonWords[key] || seen[key] {
				continue
			}
			seen[key] = true
			c, ok := counts[key]
			if !ok {
				c = &count{word: word, from: len(counts)}
				counts[key] = c
			}
			c.titles++
		}
	}
	ranked := slices.Collect(maps.Values(counts))
	slices.SortFunc(ranked, func(a, b *count) int {
		return cmp.Or(cmp.Compare(b.titles, a.titles), cmp.Compare(a.from, b.from))
	})
	var words []string
	for _, c := range ranked[:min(len(ranked), limit)] {
		words = append(words, c.word)
	}
	return words
}
