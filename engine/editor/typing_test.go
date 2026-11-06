package editor

import "testing"

// typingCorpus is well-formed Markdown a writer types key by key.
var typingCorpus = []string{
	"a **b** c",
	"a *b* c",
	"- [ ] task",
	"- [x] done",
	"($x$) after",
	"(a [b] c)",
	"`code` text",
	"# Title",
	"## Sub *em*",
	"> quote",
	"1. one",
	"- item",
	"a $x^2$ b",
	"~struck~ out",
}

// Typing well-formed Markdown key by key shows exactly what was typed:
// every closer the editor put in ahead of the writer is overtyped by
// theirs, one glyph for one, and a conversion to structure absorbs the
// closers it took.
func TestTypingShowsWhatWasTyped(t *testing.T) {
	for _, text := range typingCorpus {
		session, _ := newSessionFixture(t, "", false)
		typeKeys(t, session, text)
		if got := session.Projection().Text(); got != text {
			t.Errorf("typing %q shows %q", text, got)
		}
		if caret, end := session.Caret().RangeUTF16().Start, session.Projection().UTF16Len(); caret != end {
			t.Errorf("typing %q leaves the caret at %d of %d", text, caret, end)
		}
	}
}
