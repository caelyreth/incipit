package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// typeKeys types text one character at a time, a line break as Return.
func typeKeys(t *testing.T, session *Session, text string) {
	t.Helper()
	for _, r := range text {
		if r == '\n' {
			run(t, session, TextNewline)
			continue
		}
		if _, err := session.ReplaceSelection(string(r), kernel.EventOriginTyping); err != nil {
			t.Fatalf("type %q: %v", r, err)
		}
	}
}

func pageKinds(session *Session) []model.NodeTypeID {
	kinds := make([]model.NodeTypeID, 0)
	for _, block := range DocumentBlocks(session.Runtime().Snapshot().Document()) {
		kinds = append(kinds, block.Kind())
	}
	return kinds
}

func expectKinds(t *testing.T, session *Session, kinds ...model.NodeTypeID) {
	t.Helper()
	got := pageKinds(session)
	if len(got) != len(kinds) {
		t.Fatalf("kinds = %q, want %q", got, kinds)
	}
	for index := range got {
		if got[index] != kinds[index] {
			t.Fatalf("kinds = %q, want %q", got, kinds)
		}
	}
}

// Notation typed at a paragraph's start opens the block it reads as; the
// page keeps the kind and level, and the notation is projected.
func TestTypedNotationOpensBlocks(t *testing.T) {
	for _, test := range []struct {
		typed string
		kind  model.NodeTypeID
		view  string
		text  string
	}{
		{"## Title", HeadingNode, "## Title|", "Title"},
		{"> Said", QuoteNode, "> Said|", "Said"},
		{"- Item", BulletNode, "- Item|", "Item"},
		{"+ Item", BulletNode, "- Item|", "Item"},
		{"7. Item", OrderedNode, "1. Item|", "Item"},
	} {
		t.Run(test.typed, func(t *testing.T) {
			session, _ := newSessionFixture(t, "", false)
			typeKeys(t, session, test.typed)
			expectKinds(t, session, test.kind)
			expectPage(t, session, test.view, test.text)
		})
	}
}

// Return after a heading starts a paragraph; in a list it starts the next
// item, numbered by its place, and on an empty item it ends the list.
func TestReturnFollowsBlockKind(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Top\nbody")
	expectKinds(t, session, HeadingNode, ParagraphNode)
	expectPage(t, session, "# Top\nbody|", "Top", "body")

	session, _ = newSessionFixture(t, "", false)
	typeKeys(t, session, "1. one\ntwo\n")
	expectKinds(t, session, OrderedNode, OrderedNode, OrderedNode)
	expectPage(t, session, "1. one\n2. two\n3. |", "one", "two", "")
	typeKeys(t, session, "\nafter")
	expectKinds(t, session, OrderedNode, OrderedNode, ParagraphNode)
	expectPage(t, session, "1. one\n2. two\nafter|", "one", "two", "after")
}

// Return before a heading's text leaves a paragraph above it.
func TestReturnAtHeadingStartKeepsHeading(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "## Top")
	run(t, session, TextLineStart, TextNewline)
	expectKinds(t, session, ParagraphNode, HeadingNode)
	expectPage(t, session, "\n## |Top", "", "Top")
}

// Backspace at a block's text start deletes the notation's last character:
// notation that still reads as syntax reshapes the block, and notation that
// no longer does stays as literal text, which typing the space back reads
// as syntax again.
func TestDeletingNotationReshapesOrUnfolds(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "## Top")
	run(t, session, TextLineStart, TextBackspace)
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "##|Top", "##Top")
	typeKeys(t, session, " ")
	expectKinds(t, session, HeadingNode)
	expectPage(t, session, "## |Top", "Top")

	// Deleting one hash of two leaves a heading a level up.
	if err := session.SelectProjected(1, 2); err != nil {
		t.Fatal(err)
	}
	run(t, session, TextBackspace)
	expectKinds(t, session, HeadingNode)
	if got := session.Projection().Text(); got != "# Top" {
		t.Fatalf("page shows %q", got)
	}
}

// "```" and Return open a code block whose text keeps its line breaks; an
// empty last line and Return leave it.
func TestFencedCode(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "```go\nx := 1\ny -- \"z\"\n")
	expectKinds(t, session, CodeNode)
	expectPage(t, session, "```go\nx := 1\ny -- \"z\"\n|\n```", "x := 1\ny -- \"z\"\n")
	typeKeys(t, session, "\nprose")
	expectKinds(t, session, CodeNode, ParagraphNode)
	expectPage(t, session, "```go\nx := 1\ny -- \"z\"\n```\nprose|", "x := 1\ny -- \"z\"", "prose")
}

// Backspace into a fence unfolds the code into the paragraphs it shows.
func TestDeletingFenceUnfoldsCode(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "```\na\nb")
	run(t, session, TextDocumentStart)
	at(t, session, 4)
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode, ParagraphNode, ParagraphNode)
	expectPage(t, session, "```|a\nb\n```", "```a", "b", "```")
}

// Three dashes make a rule and go on in a paragraph after it; Backspace
// there takes the rule away.
func TestRules(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "---")
	expectKinds(t, session, RuleNode, ParagraphNode)
	expectPage(t, session, "---\n|", "", "")
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "|", "")
}

// Backticks and tildes mark code and struck text as stars mark emphasis;
// inside code, markers are text.
func TestInlineCodeAndStrike(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "a `b*c` ~d~")
	expectPage(t, session, "a `b*c` ~d~|", "a b*c d")
	marks := DocumentBlocks(session.Runtime().Snapshot().Document())[0].Marks()
	if len(marks) != 2 || marks[0].Kind() != CodeMark || marks[1].Kind() != StrikeMark {
		t.Fatalf("marks = %#v", marks)
	}
}

// Writing settings gate what typing reads as syntax.
func TestWritingSettingsGateSyntax(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetSetting(NewWritingSettingKey(headingsSetting), kernel.BoolSetting(false))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	typeKeys(t, session, "# a")
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "# a|", "# a")
}

// Kinds are built in: with the writing policy off, typed notation stays
// text, while a heading still shows its notation and Return after it still
// starts a paragraph.
func TestKindsOutliveTheWritingPolicy(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "## Top")
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(WritingPolicyID, false)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	typeKeys(t, session, "\n# *x*")
	expectKinds(t, session, HeadingNode, ParagraphNode)
	expectPage(t, session, "## Top\n# *x*|", "Top", "# *x*")
}

// Deleting across blocks takes the notation it covers as a deletion within
// a block does: Select All and Backspace leave an empty paragraph, not a
// heading's "# " or a mark's empty "**".
func TestSpanDeletionTakesCoveredNotation(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Title\nsome *emph* text")
	run(t, session, TextSelectAll, TextBackspace)
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "|", "")
	if marks := DocumentBlocks(session.Runtime().Snapshot().Document())[0].Marks(); len(marks) != 0 {
		t.Fatalf("marks left = %#v", marks)
	}
	undoTo(t, session, "# Title\nsome *emph* text")
}

// undoTo undoes once and expects the page to show text again.
func undoTo(t *testing.T, session *Session, text string) {
	t.Helper()
	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != text {
		t.Fatalf("undo shows %q, want %q", got, text)
	}
}

// A span from inside one block into another keeps what stays before and
// after it, its notation with it, and drops a mark whose markers it cut.
func TestSpanDeletionBetweenBlocks(t *testing.T) {
	for _, test := range []struct {
		name     string
		from, to model.UTF16Offset
		view     string
		kinds    []model.NodeTypeID
		texts    []string
	}{
		// "# Title\nsome *emph* text": from inside the title into the mark.
		{"into a mark", 4, 16, "# Ti|ph text", []model.NodeTypeID{HeadingNode}, []string{"Tiph text"}},
		// From inside the title to past the mark: it goes whole.
		{"past a mark", 4, 19, "# Ti| text", []model.NodeTypeID{HeadingNode}, []string{"Ti text"}},
		// From the page's start, the heading's notation covered.
		{"from the start", 0, 13, "|*emph* text", []model.NodeTypeID{ParagraphNode}, []string{"emph text"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newSessionFixture(t, "", false)
			typeKeys(t, session, "# Title\nsome *emph* text")
			if err := session.SelectProjected(test.from, test.to); err != nil {
				t.Fatal(err)
			}
			run(t, session, TextBackspace)
			expectKinds(t, session, test.kinds...)
			expectPage(t, session, test.view, test.texts...)
			undoTo(t, session, "# Title\nsome *emph* text")
		})
	}
}
