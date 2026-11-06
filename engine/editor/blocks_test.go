package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// pageTexts is each block's text, in page order.
func pageTexts(session *Session) []string {
	texts := make([]string, 0)
	for _, block := range DocumentBlocks(session.Runtime().Snapshot().Document()) {
		texts = append(texts, block.Text().String())
	}
	return texts
}

func at(t *testing.T, session *Session, offset model.UTF16Offset) {
	t.Helper()
	if err := session.SelectProjected(offset, offset); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, session *Session, commands ...textOp) {
	t.Helper()
	for _, command := range commands {
		if _, err := session.doText(command); err != nil {
			t.Fatalf("%v: %v", command, err)
		}
	}
}

func expectPage(t *testing.T, session *Session, view string, blocks ...string) {
	t.Helper()
	if got := pageTexts(session); !slices.Equal(got, blocks) {
		t.Fatalf("blocks = %q, want %q", got, blocks)
	}
	if got := shown(session); got != view {
		t.Fatalf("page shows %q, want %q", got, view)
	}
}

// Return breaks a block in two at the caret, which starts the new block;
// Backspace there joins them again, and Delete at a block's end does too.
func TestReturnSplitsAndDeletionJoinsBlocks(t *testing.T) {
	session, _ := newSessionFixture(t, "abcd", false)
	at(t, session, 2)
	run(t, session, textNewline)
	expectPage(t, session, "ab\n|cd", "ab", "cd")

	run(t, session, textBackspace)
	expectPage(t, session, "ab|cd", "abcd")

	run(t, session, textNewline, textLeft, textDelete)
	expectPage(t, session, "ab|cd", "abcd")
}

// Arrows step across a block's edge; Shift extends across it.
func TestArrowsCrossBlockEdges(t *testing.T) {
	session, _ := newSessionFixture(t, "ab\ncd", false)
	at(t, session, 3)
	run(t, session, textLeft)
	expectPage(t, session, "ab|\ncd", "ab", "cd")
	run(t, session, textRight)
	expectPage(t, session, "ab\n|cd", "ab", "cd")
	run(t, session, textSelectLeft, textSelectLeft)
	if range16, _ := session.VisualSelection(); range16 != model.NewUTF16Range(1, 3) {
		t.Fatalf("extended selection %v", range16)
	}
	run(t, session, textDocumentEnd)
	expectPage(t, session, "ab\ncd|", "ab", "cd")
	run(t, session, textDocumentStart)
	expectPage(t, session, "|ab\ncd", "ab", "cd")
}

// Typing over a selection that spans blocks takes out what lies between
// and joins the ends around the text; text with line breaks makes blocks.
func TestTypingOverASpanJoinsAndPastingLinesSplits(t *testing.T) {
	session, _ := newSessionFixture(t, "abc\nmid\ndef", false)
	if err := session.SelectProjected(2, 9); err != nil {
		t.Fatal(err)
	}
	typeSession(t, session, "X")
	expectPage(t, session, "abX|ef", "abXef")

	if _, err := session.replaceSelection("1\n2\n3", kernel.EventOriginPaste); err != nil {
		t.Fatal(err)
	}
	expectPage(t, session, "abX1\n2\n3|ef", "abX1", "2", "3ef")

	run(t, session, textSelectAll, textSelectAll, textBackspace)
	expectPage(t, session, "|", "")
}

func TestPastedLineEndingsUseTheCanonicalSeparator(t *testing.T) {
	for _, input := range []string{"a\r\nb", "a\rb"} {
		session, _ := newSessionFixture(t, "", false)
		if _, err := session.replaceSelection(input, kernel.EventOriginPaste); err != nil {
			t.Fatal(err)
		}
		if got := session.Projection().Text(); got != "a\nb" {
			t.Fatalf("%q pasted as %q", input, got)
		}
		if got := pageTexts(session); !slices.Equal(got, []string{"a", "b"}) {
			t.Fatalf("%q blocks = %#v", input, got)
		}
	}
}

func TestPageRejectsNestedBlockShapes(t *testing.T) {
	document, blocks, err := NewBlockDocument("outer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		_, err := draft.CreateNode(blocks[0], 0, ParagraphNode)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePage(document.Snapshot()); err == nil {
		t.Fatal("nested page block accepted")
	}
}

// Undo brings back the page as it was, blocks and all; redo goes forward.
func TestUndoAndRedoRestoreBlocks(t *testing.T) {
	session, _ := newSessionFixture(t, "abcd", false)
	at(t, session, 2)
	run(t, session, textNewline)
	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	expectPage(t, session, "ab|cd", "abcd")
	if _, err := session.doHistory(historyRedo); err != nil {
		t.Fatal(err)
	}
	if got := pageTexts(session); !slices.Equal(got, []string{"ab", "cd"}) {
		t.Fatalf("redo = %q", got)
	}
}

// A word step at a block's edge goes on into the next block.
func TestWordStepsCrossBlockEdges(t *testing.T) {
	session, _ := newSessionFixture(t, "one two\nthree", false)
	at(t, session, 7)
	if _, err := session.doWord(wordNext); err != nil {
		t.Fatal(err)
	}
	expectPage(t, session, "one two\n|three", "one two", "three")
	if _, err := session.doWord(wordDeletePrevious); err != nil {
		t.Fatal(err)
	}
	expectPage(t, session, "one two|three", "one twothree")
}
