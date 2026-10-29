package editor

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

type markSpec struct {
	start, end model.UTF16Offset
	level      uint8
}

// newMarkedSession is a session over text with the given inline marks.
func newMarkedSession(t *testing.T, text string, marks ...markSpec) *Session {
	t.Helper()
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(node, text); err != nil {
			return err
		}
		for _, mark := range marks {
			if _, err := draft.AddMark(node, model.NewUTF16Range(mark.start, mark.end), mark.level); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := kernel.NewPolicyRuntime(document, catalog)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(runtime, node, &sessionClock{now: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

// shown is the projection with the caret drawn where the surface draws it.
func shown(session *Session) string {
	text := []rune(session.Projection().Text())
	at := int(session.Caret().RangeUTF16().Start)
	return string(text[:at]) + "|" + string(text[at:])
}

// insideNotation reports a caret standing within one generated marker run,
// such as between the two stars of "**": a stop for editing the notation
// itself, where typing text is not text insertion.
func insideNotation(session *Session) bool {
	at := session.Caret().RangeUTF16().Start
	for _, segment := range session.Projection().Segments() {
		span := segment.ProjectedRange()
		if _, ok := segment.AttributeTarget(); ok && span.Start < at && at < span.End {
			return true
		}
	}
	return false
}

// checkTypingFollowsCaret is the law the surface promises: a typed
// character appears exactly where the caret is drawn, and the caret stays
// right after it, on the same side of every generated marker.
func checkTypingFollowsCaret(t *testing.T, session *Session, context string) {
	t.Helper()
	before := shown(session)
	at := int(session.Caret().RangeUTF16().Start)
	node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Node())
	membership := caretMembership(node, session.Projection(), session.Caret().RangeUTF16().Start)
	if !sameMark(activeMark(session.Inline()), membership) {
		t.Errorf("%s: at %s the active mark %v is not the caret's membership %v",
			context, before, activeMark(session.Inline()), membership)
	}
	text := []rune(session.Projection().Text())
	want := string(text[:at]) + "x|" + string(text[at:])
	if _, err := session.ReplaceSelection("x", kernel.EventOriginTyping); err != nil {
		t.Fatalf("%s: typing at %s: %v", context, before, err)
	}
	if got := shown(session); got != want {
		t.Errorf("%s: typing at %s gave %s, want %s", context, before, got, want)
	}
}

type caretStep struct {
	name string
	run  func(*Session) error
}

func textStep(command TextCommand) caretStep {
	return caretStep{
		name: string(command.ID()),
		run: func(session *Session) error {
			_, err := session.ExecuteText(command)
			return err
		},
	}
}

var (
	stepLeft      = textStep(TextLeft)
	stepRight     = textStep(TextRight)
	stepBackspace = textStep(TextBackspace)
	stepDelete    = textStep(TextDelete)
	stepUndo      = caretStep{name: "undo", run: func(session *Session) error {
		_, err := session.ExecuteHistory(HistoryUndo)
		return err
	}}
	stepRedo = caretStep{name: "redo", run: func(session *Session) error {
		_, err := session.ExecuteHistory(HistoryRedo)
		return err
	}}
	stepWordPrevious = caretStep{name: "word-previous", run: func(session *Session) error {
		_, err := session.ExecuteWord(WordPrevious)
		return err
	}}
	stepWordNext = caretStep{name: "word-next", run: func(session *Session) error {
		_, err := session.ExecuteWord(WordNext)
		return err
	}}
	stepType = caretStep{name: "type", run: func(session *Session) error {
		_, err := session.ReplaceSelection("y", kernel.EventOriginTyping)
		return err
	}}
)

func TestTypingFollowsTheDrawnCaretAfterEveryStep(t *testing.T) {
	documents := caretDocuments
	steps := []caretStep{
		stepLeft, stepRight, stepBackspace, stepDelete,
		stepUndo, stepRedo, stepWordPrevious, stepWordNext, stepType,
	}
	for _, document := range documents {
		probe := newMarkedSession(t, document.text, document.marks...)
		length := len([]rune(probe.Projection().Text()))
		for click := 0; click <= length; click++ {
			for _, first := range steps {
				for _, second := range steps {
					session := newMarkedSession(t, document.text, document.marks...)
					if err := session.SelectProjected(model.UTF16Offset(click), model.UTF16Offset(click)); err != nil {
						continue
					}
					start := shown(session)
					// A first edit gives undo something to restore.
					if _, err := session.ReplaceSelection("z", kernel.EventOriginTyping); err != nil {
						continue
					}
					failed := false
					for _, step := range []caretStep{first, second} {
						if err := step.run(session); err != nil {
							failed = true
							break
						}
					}
					if failed || insideNotation(session) {
						continue
					}
					checkTypingFollowsCaret(t, session,
						fmt.Sprintf("%v from %s, z, %s, %s", document.marks, start, first.name, second.name))
				}
			}
		}
	}
}

var caretDocuments = []struct {
	text  string
	marks []markSpec
}{
	{"abcd", []markSpec{{1, 3, 1}}},
	{"abcd", []markSpec{{1, 3, 2}}},
	{"abcde", []markSpec{{0, 5, 1}, {1, 3, 2}}},
	{"abcd", []markSpec{{0, 2, 1}, {2, 4, 1}}},
	{"ab", []markSpec{{0, 2, 1}}},
	{"abcd", []markSpec{{0, 4, 1}, {2, 4, 2}}},
	{"ab cd", []markSpec{{0, 2, 2}, {2, 5, 1}}},
}

// Arrow keys move one drawn position at a time, notation included: no
// press skips past a closer or lands on the far side of an opener.
func TestArrowsStepThroughEveryDrawnPosition(t *testing.T) {
	for _, document := range caretDocuments {
		session := newMarkedSession(t, document.text, document.marks...)
		if err := session.SelectProjected(0, 0); err != nil {
			t.Fatal(err)
		}
		length := model.UTF16Offset(len([]rune(session.Projection().Text())))
		for want := model.UTF16Offset(1); want <= length; want++ {
			before := shown(session)
			if _, err := session.ExecuteText(TextRight); err != nil {
				t.Fatal(err)
			}
			if got := session.Caret().RangeUTF16().Start; got != want {
				t.Fatalf("%v: Right from %s reached %s (%d, want %d)", document.marks, before, shown(session), got, want)
			}
		}
		for want := length - 1; want >= 0; want-- {
			before := shown(session)
			if _, err := session.ExecuteText(TextLeft); err != nil {
				t.Fatal(err)
			}
			if got := session.Caret().RangeUTF16().Start; got != want {
				t.Fatalf("%v: Left from %s reached %s", document.marks, before, shown(session))
			}
		}
	}
}

// Undo puts back the caret the person saw, on the same side of the
// notation, not merely the same text offset.
func TestUndoRestoresTheDrawnCaret(t *testing.T) {
	edits := []caretStep{stepType, stepBackspace, stepDelete}
	for _, document := range caretDocuments {
		probe := newMarkedSession(t, document.text, document.marks...)
		length := len([]rune(probe.Projection().Text()))
		for click := 0; click <= length; click++ {
			for _, edit := range edits {
				session := newMarkedSession(t, document.text, document.marks...)
				if err := session.SelectProjected(model.UTF16Offset(click), model.UTF16Offset(click)); err != nil ||
					insideNotation(session) {
					continue
				}
				before := shown(session)
				revision := session.Runtime().Revision()
				if err := edit.run(session); err != nil || session.Runtime().Revision() == revision {
					continue
				}
				if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
					t.Fatal(err)
				}
				if got := shown(session); got != before {
					t.Errorf("%v: %s at %s then undo shows %s", document.marks, edit.name, before, got)
				}
			}
		}
	}
}

// Deleting the text character beside the caret removes exactly that
// character from what is drawn and leaves the caret where it was relative
// to the notation, so the next keystroke lands where it is drawn.
func TestDeletionRemovesTheCharacterBesideTheCaret(t *testing.T) {
	for _, document := range caretDocuments {
		probe := newMarkedSession(t, document.text, document.marks...)
		length := len([]rune(probe.Projection().Text()))
		for click := 0; click <= length; click++ {
			for _, backward := range []bool{true, false} {
				session := newMarkedSession(t, document.text, document.marks...)
				if err := session.SelectProjected(model.UTF16Offset(click), model.UTF16Offset(click)); err != nil {
					continue
				}
				at := int(session.Caret().RangeUTF16().Start)
				victim := at
				if backward {
					victim = at - 1
				}
				if victim < 0 || victim >= length || !textAt(session, victim) {
					continue
				}
				text := []rune(session.Projection().Text())
				want := string(text[:victim]) + "|" + string(text[victim+1:])
				before := shown(session)
				command := TextDelete
				if backward {
					command = TextBackspace
				}
				if _, err := session.ExecuteText(command); err != nil {
					t.Fatal(err)
				}
				node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Node())
				if len(node.Marks()) != len(document.marks) {
					continue // emptying a mark may retire its notation
				}
				if got := shown(session); got != want {
					t.Errorf("%v: deleting beside %s gave %s, want %s", document.marks, before, got, want)
				}
			}
		}
	}
}

// textAt reports whether the projected unit at index is semantic text.
func textAt(session *Session, index int) bool {
	for _, segment := range session.Projection().Segments() {
		span := segment.ProjectedRange()
		if _, ok := segment.SemanticTarget(); ok && int(span.Start) <= index && index < int(span.End) {
			return true
		}
	}
	return false
}

// Replacing a selected text character draws the replacement in its place,
// with the caret right after it on the same side of the notation.
func TestReplacingASelectionKeepsItsPlace(t *testing.T) {
	for _, document := range caretDocuments {
		probe := newMarkedSession(t, document.text, document.marks...)
		length := len([]rune(probe.Projection().Text()))
		for start := 0; start < length; start++ {
			if !textAt(probe, start) {
				continue
			}
			session := newMarkedSession(t, document.text, document.marks...)
			if err := session.SelectProjected(model.UTF16Offset(start), model.UTF16Offset(start+1)); err != nil {
				t.Fatal(err)
			}
			text := []rune(session.Projection().Text())
			want := string(text[:start]) + "y|" + string(text[start+1:])
			if _, err := session.ReplaceSelection("y", kernel.EventOriginTyping); err != nil {
				t.Fatal(err)
			}
			if got := shown(session); got != want {
				t.Errorf("%v: replacing %q at %d gave %s, want %s",
					document.marks, string(text[start]), start, got, want)
			}
		}
	}
}
