package profile

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/editor/doc"
)

// TagSpelling opens a tag written in text.
const TagSpelling Spelling = "#"

// Tag is a tag written in text: its extent, its "#" included, and its
// name, the text after the "#".
type Tag struct {
	From, To int
	Name     string
}

// TagsIn reads the tags written in in[from:to), in order. A tag is a "#"
// where a word starts, at the text's start or after a space or an opening
// bracket or quote, with a letter right after it; its name runs on through
// letters, digits, "_" and "-", and does not end in "-". So "# " stays a
// heading's marker and "C#", "#1" and "a/#b" hold none. Text marked as code
// and inline objects hold no tag and end one.
func TagsIn(in *doc.Inline, from, to int) []Tag {
	if in == nil || from >= to {
		return nil
	}
	// The range's text, code and objects blanked to a byte no tag holds,
	// so offsets stay the slot's.
	var b strings.Builder
	in.Each(func(at int, it doc.Item) bool {
		lo, hi := max(at, from), min(at+it.Len(), to)
		if lo >= hi {
			return at < to
		}
		if it.Node != nil || it.Marks.Has(CodeMark) {
			b.WriteString(strings.Repeat("\x00", hi-lo))
		} else {
			b.WriteString(it.Text[lo-at : hi-at])
		}
		return true
	})
	text := b.String()
	var tags []Tag
	prev := rune(-1)
	for i := 0; i < len(text); {
		r, n := utf8.DecodeRuneInString(text[i:])
		if r == '#' && opensWord(prev) {
			if end := tagEnd(text, i+n); end > i+n {
				tags = append(tags, Tag{From: from + i, To: from + end, Name: text[i+n : end]})
				prev, i = '#', end
				continue
			}
		}
		prev, i = r, i+n
	}
	return tags
}

// TagNamed reports whether name is a whole tag's name: what "#" followed
// by it writes.
func TagNamed(name string) bool {
	text := string(TagSpelling) + name
	return name != "" && tagEnd(text, len(TagSpelling)) == len(text)
}

// TagKey is the identity of a tag's name: names that differ only in case
// are one tag.
func TagKey(name string) string { return strings.ToLower(name) }

// opensWord reports whether a word starts after prev, -1 at the text's
// start.
func opensWord(prev rune) bool {
	return prev < 0 || unicode.IsSpace(prev) || unicode.In(prev, unicode.Ps, unicode.Pi)
}

// tagEnd is where a tag's name starting at i ends: i when no letter starts
// it.
func tagEnd(text string, i int) int {
	r, n := utf8.DecodeRuneInString(text[i:])
	if n == 0 || !unicode.IsLetter(r) {
		return i
	}
	end, j := i+n, i+n
	for j < len(text) {
		r, n := utf8.DecodeRuneInString(text[j:])
		if !unicode.In(r, unicode.L, unicode.M, unicode.Nd) && r != '_' && r != '-' {
			break
		}
		if j += n; r != '-' {
			end = j
		}
	}
	return end
}
