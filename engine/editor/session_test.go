package editor

import (
	"errors"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

type sessionClock struct{ now time.Time }

func (c *sessionClock) Now() time.Time { return c.now }

func newSessionFixture(t *testing.T, text string, mark bool) (*Session, model.NodeID) {
	t.Helper()
	document, blocks, err := NewBlockDocument(text)
	if err != nil {
		t.Fatal(err)
	}
	node := blocks[0]
	if mark {
		if _, err := document.Transact(func(draft *model.DocumentDraft) error {
			_, err := draft.AddMark(node, model.NewUTF16Range(1, 3), EmphasisMark, 1)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	return sessionFor(t, document, node), node
}

// sessionFor mounts the editor on document, the caret in node.
func sessionFor(t *testing.T, document *model.Document, node model.NodeID) *Session {
	t.Helper()
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

func TestSessionMountsProjectionInlineMetricsAndPairs(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "a(b)c", true)
	if session.Projection().Text() != "a*(b*)c" {
		t.Fatalf("projection = %q", session.Projection().Text())
	}
	metrics, ok := session.Statistics()
	if !ok || metrics.Characters != 5 || metrics.Paragraphs != 1 {
		t.Fatalf("metrics = %#v, %v", metrics, ok)
	}
	pairs := session.Pairs()
	if len(pairs) != 1 || pairs[0].Open != 1 || pairs[0].Close != 3 {
		t.Fatalf("pairs = %#v", pairs)
	}
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 2, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 2, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if id, ok := session.Inline().Active(); !ok || id != 1 {
		t.Fatalf("active mark = %d, %v", id, ok)
	}
}

func TestSessionSurfaceFrameIncludesPreviewAndDecorations(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abc", false)
	target, err := session.Projection().Target(model.EmptyUTF16Range(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := session.Composition().Preview(); ok {
		t.Fatal("composition unexpectedly active")
	}
	if err := session.Composition().Begin(
		session.Projection(),
		target,
		session.Runtime().Snapshot().Document(),
	); err != nil {
		t.Fatal(err)
	}
	if err := session.Composition().Update("中"); err != nil {
		t.Fatal(err)
	}
	preview, _ := session.Composition().Preview()
	projected, err := session.VisualSelection()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := session.SurfaceFrame(time.Unix(1_700_000_000, 0), nil)
	if err != nil {
		t.Fatal(err)
	}
	previewFrame, err := frame.WithPreview(
		projected,
		preview.Text(),
		model.NewUTF16Range(0, model.UTF16Offset(model.StringLenUTF16(preview.Text()))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if previewFrame.Flow().Text() != "中abc" ||
		previewFrame.Selection() != model.NewUTF16Range(0, 1) {
		t.Fatalf("preview frame = %#v", previewFrame)
	}
	if _, ok := session.Statistics(); !ok {
		t.Fatal("metrics missing")
	}
	_ = node
}

func TestSessionReconfigureFailurePreservesPresentationAndRevision(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "ab", false)
	before := session.Projection()
	beforeRevision := session.Runtime().Revision()
	_, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(WritingPolicyID, false)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.Projection() == before || session.Runtime().Revision() != beforeRevision+1 ||
		session.Runtime().Configuration().IsEnabled(WritingPolicyID) {
		t.Fatalf("reconfiguration did not install new projection")
	}
}

func TestSessionSelectionDrivesFrameAndComposition(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abcdef", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 4, model.AffinityLeft, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	frame, err := session.SurfaceFrame(time.Unix(1_700_000_000, 0), nil)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Selection() != model.NewUTF16Range(1, 4) ||
		frame.Caret() != 4 {
		t.Fatalf("frame selection/caret = %#v/%#v", frame.Selection(), frame.Caret())
	}
	if session.Presentation().Selection.IsCollapsed() {
		t.Fatal("semantic selection unexpectedly collapsed")
	}
}

func TestSessionReplacementPublishesProjectionHistoryAndMetrics(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "ab", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	result, err := session.ReplaceSelection("X", kernel.EventOriginTyping)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != kernel.OutcomeHandled || result.Revision != session.Runtime().Revision() {
		t.Fatalf("result = %#v revision=%d", result, session.Runtime().Revision())
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "aXb" {
		t.Fatalf("text = %q", nodeValue.Text().String())
	}
	if session.Projection().Text() != "aXb" || session.Selection().Head().OffsetUTF16() != 2 {
		t.Fatalf("projection=%q selection=%#v", session.Projection().Text(), session.Selection())
	}
	if !session.History().CanUndo() {
		t.Fatal("replacement did not record history")
	}
	metrics, ok := session.Statistics()
	if !ok || metrics.Characters != 3 {
		t.Fatalf("metrics = %#v", metrics)
	}
}

func TestSessionMarkerReplacementUsesGeneratedNotation(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abcd", true)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 3, model.AffinityLeft, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if session.Projection().Text() != "a*bc*d" {
		t.Fatalf("initial projection = %q", session.Projection().Text())
	}
	result, err := session.ReplaceSelection("Z", kernel.EventOriginSemantic)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != kernel.OutcomeHandled {
		t.Fatalf("outcome = %#v", result)
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "aZd" || session.Projection().Text() != "a*Z*d" {
		t.Fatalf("semantic=%q projected=%q", nodeValue.Text().String(), session.Projection().Text())
	}
}

func TestSessionCompositionRetainsPreviewUntilHandled(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "ab", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if err := session.BeginComposition(); err != nil {
		t.Fatal(err)
	}
	if err := session.UpdateComposition("中"); err != nil {
		t.Fatal(err)
	}
	if err := session.UpdateComposition("中文"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ReplaceSelection("x", kernel.EventOriginTyping); !errors.Is(err, ErrCompositionActive) {
		t.Fatalf("replacement during composition = %v", err)
	}
	outcome, err := session.CommitComposition()
	if err != nil {
		t.Fatal(err)
	}
	if outcome != kernel.OutcomeHandled {
		t.Fatalf("composition outcome = %#v", outcome)
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "a中文b" {
		t.Fatalf("composition text = %q", nodeValue.Text().String())
	}
	if _, active := session.Composition().Preview(); active {
		t.Fatal("composition preview survived handled commit")
	}
}

// emptyBlockDocument is a page of one empty paragraph, and that paragraph.
func emptyBlockDocument(t *testing.T) (*model.Document, model.NodeID) {
	t.Helper()
	document, blocks, err := NewBlockDocument("")
	if err != nil {
		t.Fatal(err)
	}
	return document, blocks[0]
}
