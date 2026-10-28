package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func typeSession(t *testing.T, session *Session, text string) {
	t.Helper()
	if _, err := session.ReplaceSelection(text, kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
}

func TestSessionTypingInsertsPairsAndOvertypesClosers(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "", false)
	typeSession(t, session, "(")
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "()" || session.Selection().Head().OffsetUTF16() != 1 {
		t.Fatalf("pair open text=%q head=%d", nodeValue.Text().String(), session.Selection().Head().OffsetUTF16())
	}
	typeSession(t, session, "x")
	typeSession(t, session, ")")
	nodeValue, _ = session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "(x)" || session.Selection().Head().OffsetUTF16() != 3 {
		t.Fatalf("overtype text=%q head=%d", nodeValue.Text().String(), session.Selection().Head().OffsetUTF16())
	}
}

func TestSessionTypingAppliesCJKObserver(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "", false)
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(CJKPolicyID, true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	typeSession(t, session, "中")
	typeSession(t, session, "a")
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "中 a" {
		t.Fatalf("CJK spacing text = %q", nodeValue.Text().String())
	}
}

func TestSessionTypingAppliesTypographyObserver(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abc", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 3, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 3, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	typeSession(t, session, ".")
	typeSession(t, session, ".")
	typeSession(t, session, ".")
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "abc…" {
		t.Fatalf("typography text = %q", nodeValue.Text().String())
	}
}

func TestSessionTypingSmartClosesQuotes(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "", false)
	typeSession(t, session, "\"")
	typeSession(t, session, "x")
	typeSession(t, session, "\"")
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "“x”" {
		t.Fatalf("smart quotes text = %q", nodeValue.Text().String())
	}
}

func TestSessionTypingKeepsCollapsedGeneratedStrongPairOpen(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "", false)
	if outcome, err := session.ExecuteMarkup(MarkupStrong); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("strong outcome=%v err=%v", outcome, err)
	}
	if session.Projection().Text() != "****" {
		t.Fatalf("empty strong projection = %q", session.Projection().Text())
	}
	typeSession(t, session, "a")
	typeSession(t, session, "b")
	if session.Projection().Text() != "**ab**" {
		t.Fatalf("strong typing projection = %q", session.Projection().Text())
	}
	if session.Caret().RangeUTF16().Start != 4 {
		t.Fatalf("strong typing caret = %d", session.Caret().RangeUTF16().Start)
	}
}
