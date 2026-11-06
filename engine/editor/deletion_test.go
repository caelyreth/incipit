package editor

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

// armedSpan is what an armed deletion takes, failing when none is armed.
func armedSpan(t *testing.T, session *Session) model.UTF16Range {
	t.Helper()
	f, ok := session.live()
	if !ok || f.arm == nil {
		t.Fatal("nothing armed")
	}
	return f.arm.span
}

// Backspace joining a paragraph to the code above it arms the join, marking
// the code, with a guide naming the key; the second press joins, and any
// other intent disarms.
func TestJoiningProseToLiteralTextArmsFirst(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "after")
	run(t, session, textLineStart, textNewline)
	at(t, session, 0)
	typeKeys(t, session, "```\nx = 1")
	text := session.Projection().Text()
	start := model.UTF16Offset(strings.Index(text, "after"))
	at(t, session, start)
	run(t, session, textBackspace)
	if session.Projection().Text() != text {
		t.Fatalf("the first Backspace changed %q to %q", text, session.Projection().Text())
	}
	armedSpan(t, session)
	guides := session.armGuides(time.Time{})
	if len(guides) != 1 || guides[0].Command != DeleteBackwardCommandID {
		t.Fatalf("arm guides = %#v", guides)
	}
	if _, err := session.Do(Move(ByGrapheme, Forward, false), time.Time{}); err != nil {
		t.Fatal(err)
	}
	if len(session.armGuides(time.Time{})) != 0 {
		t.Fatal("moving kept the arm")
	}
	at(t, session, start)
	run(t, session, textBackspace, textBackspace)
	if session.Projection().Text() == text {
		t.Fatalf("the second Backspace did not join %q", text)
	}
}
