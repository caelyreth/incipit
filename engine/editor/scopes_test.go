package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func selectedText(t *testing.T, session *Session) string {
	t.Helper()
	selection, err := session.VisualSelection()
	if err != nil {
		t.Fatal(err)
	}
	text := []rune(session.Projection().Text())
	return string(text[selection.Start:selection.End])
}

func TestSelectAllGroupStartsAtItsFirstBlock(t *testing.T) {
	document, blocks, err := NewBlockDocument("code\none\ntwo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := shapeBlock(draft, blocks[0], blockShape{kind: CodeNode, info: "go"}); err != nil {
			return err
		}
		for _, block := range blocks[1:] {
			if err := shapeBlock(draft, block, blockShape{kind: BulletNode}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session := sessionFor(t, document, blocks[1])
	run(t, session, TextSelectAll)
	if got := selectedText(t, session); got != "one" {
		t.Fatalf("first scope = %q", got)
	}
	run(t, session, TextSelectAll)
	if got := selectedText(t, session); got != "- one\n- two" {
		t.Fatalf("group scope = %q", got)
	}
}

func TestSelectAllDoesNotTakePrecedingClosingNotation(t *testing.T) {
	document, blocks, err := NewBlockDocument("text\none\ntwo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if _, err := draft.AddMark(blocks[0], model.NewUTF16Range(0, 4), EmphasisMark, 2); err != nil {
			return err
		}
		for _, block := range blocks[1:] {
			if err := shapeBlock(draft, block, blockShape{kind: BulletNode}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session := sessionFor(t, document, blocks[1])
	run(t, session, TextSelectAll, TextSelectAll)
	if got := selectedText(t, session); got != "- one\n- two" {
		t.Fatalf("group scope = %q", got)
	}
}

func TestSelectAllBlockScopeIncludesInlineMarkNotation(t *testing.T) {
	session, _ := newSessionFixture(t, "abc", false)
	if _, err := session.ReplaceSelection("*", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != "**abc" {
		t.Fatalf("projection = %q", got)
	}
	run(t, session, TextSelectAll)
	if got := selectedText(t, session); got != "**abc" {
		t.Fatalf("block scope = %q", got)
	}
	if _, err := session.ReplaceSelection("", kernel.EventOriginSemantic); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != "" {
		t.Fatalf("deleting the selected ghost block left %q", got)
	}
}

func TestSelectAllBlockScopeIncludesInlineNotationAroundText(t *testing.T) {
	for _, test := range []struct {
		text string
		from int
		to   int
		want string
	}{
		{"hello", 0, 5, "*hello*"},
		{"hello world", 0, 5, "*hello* world"},
	} {
		t.Run(test.text, func(t *testing.T) {
			session := newMarkedSession(t, test.text, markSpec{
				start: model.UTF16Offset(test.from),
				end:   model.UTF16Offset(test.to),
				level: 1,
			})
			run(t, session, TextSelectAll)
			if got := selectedText(t, session); got != test.want {
				t.Fatalf("block scope = %q, want %q", got, test.want)
			}
			if _, err := session.ReplaceSelection("", kernel.EventOriginSemantic); err != nil {
				t.Fatal(err)
			}
			if got := session.Projection().Text(); got != "" {
				t.Fatalf("deleting the selected inline block left %q", got)
			}
		})
	}
}
