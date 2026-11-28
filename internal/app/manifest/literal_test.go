package manifest

import (
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// inCode opens a page with inline code turned on at the caret, after
// before.
func inCode(t *testing.T, before string) *page {
	t.Helper()
	p := open(t)
	p.typeText(before)
	p.do(vocab.Intent(profile.Format{Mark: profile.CodeMark}))
	return p
}

// Typed in inline code, syntax is code: no formula, mark, block or fence
// comes of it, at a line's start or within it, the backtick that closes
// the code aside.
func TestInlineCodeKeepsTypedSyntaxAsTyped(t *testing.T) {
	for _, before := range []string{"", "q "} {
		for _, typed := range []string{"$x", "$$", "*b*", "**b**", "_b_", "~s~", "## ", "- ", "> ", "1. ", "* "} {
			p := inCode(t, before)
			p.typeText(typed)
			p.want("page(paragraph(" + before + "<" + typed + ">))")
			if !hasMark(p, profile.CodeMark) {
				t.Fatalf("%q typed after %q should stay code.", typed, before)
			}
		}
	}
	p := inCode(t, "")
	p.typeText("```")
	p.do(vocab.Break{})
	if holds(p, profile.Code) {
		t.Fatal("A fence typed in inline code should not make a code block.")
	}
	p = inCode(t, "q ")
	p.typeText("x`")
	p.typeText(" $y")
	if !holds(p, profile.Formula) {
		t.Fatal("Past the backtick that closes the code, a \"$\" should open a formula again.")
	}
}

// holds is whether the page holds a node of kind k.
func holds(p *page, k doc.Kind) bool {
	found := false
	p.e.Snapshot().Root().Walk(func(n *doc.Node) bool {
		if n.Kind() == k {
			found = true
		}
		return !found
	})
	return found
}

// Typed in inline code, characters stay as typed: no dash, ellipsis,
// curly quote or full-width punctuation is made of them, while the prose
// beside the code is still refined.
func TestInlineCodeKeepsItsCharacters(t *testing.T) {
	for _, typed := range []string{"a -- b", "w...", "it's", "中,好.", "---"} {
		p := inCode(t, "q ")
		p.typeText(typed)
		p.want("page(paragraph(q <" + typed + ">))")
	}
	p := inCode(t, "q ")
	p.typeText("x`")
	p.typeText(" it's")
	p.want("page(paragraph(q <x> it’s))")
}

// Pasted where the caret writes inline code, the clipboard's characters
// are code, as in a code block: in the code's middle, or with code turned
// on at the caret.
func TestPastingIntoInlineCodeKeepsTheCode(t *testing.T) {
	p := open(t)
	p.typeText("q `abcd`")
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.do(vocab.Paste{Text: "*x* $y"})
	p.want("page(paragraph(q <abc*x* $yd>))")
	p = inCode(t, "q ")
	p.do(vocab.Paste{Text: "it's -- $z"})
	p.want("page(paragraph(q <it's -- $z>))")
}
