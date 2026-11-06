package editor

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
)

// Typing a block's own glyph at its token steps its level up, another
// kind's opener re-shapes it, and "1." typed digit by digit over "-" makes
// a numbered item whose "." is not typed twice.
func TestTypingAtATokenEditsTheShape(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "## Top")
	run(t, session, textLineStart, textLeft)
	expectPage(t, session, "#|# Top", "Top")
	typeKeys(t, session, "#")
	expectKinds(t, session, HeadingNode)
	expectPage(t, session, "##|# Top", "Top")

	session, _ = newSessionFixture(t, "", false)
	typeKeys(t, session, "- item")
	run(t, session, textLineStart, textLeft)
	expectPage(t, session, "|- item", "item")
	typeKeys(t, session, "1.")
	expectKinds(t, session, OrderedNode)
	expectPage(t, session, "1. |item", "item")
}

// A space or x typed in a checkbox's box sets it from the keyboard.
func TestTypingInACheckboxBoxSetsIt(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- [ task")
	run(t, session, textLineStart, textLeft)
	expectPage(t, session, "- [| ] task", "task")
	if _, err := session.Do(Insert("x", kernel.EventOriginTyping), time.Time{}); err != nil {
		t.Fatal(err)
	}
	expectPage(t, session, "- [|x] task", "task")
	if _, err := session.Do(Insert(" ", kernel.EventOriginTyping), time.Time{}); err != nil {
		t.Fatal(err)
	}
	expectPage(t, session, "- [| ] task", "task")
}
