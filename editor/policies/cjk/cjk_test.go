package cjk

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

type harness struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func newHarness(t *testing.T, values policy.Values) *harness {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, Policy},
		Strict:  true,
	}, values)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 99, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true})
	e.OpenView(1)
	return &harness{t: t, e: e, now: time.Unix(1000, 0)}
}

func (h *harness) do(in vocab.Input) *engine.Published {
	h.t.Helper()
	h.now = h.now.Add(100 * time.Millisecond)
	in.At = h.now
	pub, err := h.e.Do(1, in)
	if err != nil {
		h.t.Fatal(err)
	}
	return pub
}

func (h *harness) typeText(text string) {
	h.t.Helper()
	for _, r := range text {
		h.do(vocab.Input{Intent: vocab.Type{Text: string(r)}})
	}
}

func (h *harness) text() string {
	h.t.Helper()
	ref, ok := vocab.FirstTextSlot(h.e.Snapshot().Root())
	if !ok {
		h.t.Fatal("no text slot")
	}
	n, ok := h.e.Snapshot().Node(ref.Node)
	if !ok {
		h.t.Fatal("no paragraph")
	}
	return n.Text()
}

func (h *harness) want(text string) {
	h.t.Helper()
	if got := h.text(); got != text {
		h.t.Fatalf("text = %q, want %q", got, text)
	}
}

func TestCJKAndLatinSpacingWorksBothWays(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("中文English")
	h.want("中文 English")

	h = newHarness(t, nil)
	h.typeText("English中文")
	h.want("English 中文")

	h = newHarness(t, nil)
	h.typeText("中文123")
	h.want("中文 123")
}

// Spacing emits the insertion it made, not a replacement of the line.
func TestSpacingEmitsTheInsertedSpace(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("中文")
	pub := h.do(vocab.Input{Intent: vocab.Type{Text: "E"}})
	h.want("中文 E")
	for _, r := range pub.Records {
		if r.Origin.Phase == doc.PhaseScript && r.Kind == doc.RecordTextInserted {
			if r.Text != " " || r.From != len("中文") || r.To != len("中文")+1 {
				t.Fatalf("script insertion = %+v", r)
			}
			return
		}
	}
	t.Fatalf("no Script insertion in %+v", pub.Records)
}

func TestPunctuationTakesTheLinesScript(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("中文,English")
	h.want("中文，English")

	h = newHarness(t, nil)
	h.typeText("中文.")
	h.want("中文。")

	h = newHarness(t, nil)
	h.typeText("English,text")
	h.want("English,text")
}

func TestDecimalsAndURLsAreExceptions(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("中文3.14")
	h.want("中文 3.14")

	h = newHarness(t, nil)
	h.typeText("中文https://example.com")
	h.want("中文 https://example.com")

	h = newHarness(t, nil)
	h.typeText("中文https://example.com,中文")
	h.want("中文 https://example.com，中文")
}

func TestIMECommitRunsTheRewrite(t *testing.T) {
	h := newHarness(t, nil)
	h.do(vocab.Input{Intent: vocab.Type{Text: "中文,English"}, Class: doc.OriginIME})
	h.want("中文，English")
}

func TestNonTypingOriginsDoNotRun(t *testing.T) {
	h := newHarness(t, nil)
	h.do(vocab.Input{Intent: vocab.Paste{Text: "中文,English"}})
	h.want("中文,English")
}

func TestOptionsCanBeOff(t *testing.T) {
	h := newHarness(t, policy.Values{"cjk.punctuation": false, "cjk.spacing": false})
	h.typeText("中文,English")
	h.want("中文,English")
}

func TestScriptTriggerIsTypingAndIMEOnly(t *testing.T) {
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, Policy},
		Strict:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rewrites := prog.Rewrites(doc.PhaseScript)
	if len(rewrites) != 1 {
		t.Fatalf("Script rewrites = %d, want 1", len(rewrites))
	}
	trigger := rewrites[0].Trigger()
	for _, class := range []doc.OriginClass{
		doc.OriginTyping, doc.OriginIME, doc.OriginPaste, doc.OriginCommand,
		doc.OriginPointer, doc.OriginAgent, doc.OriginHistory, doc.OriginTimer,
	} {
		record := doc.Record{Kind: doc.RecordTextInserted, Origin: doc.Origin{Class: class}}
		want := class == doc.OriginTyping || class == doc.OriginIME
		if got := trigger.Matches(record); got != want {
			t.Errorf("trigger for %v = %v, want %v", class, got, want)
		}
	}
}

func TestRewriteStaysInsideTheLineAroundInlineNodes(t *testing.T) {
	marks := doc.NewInterner()
	empty := marks.Empty()
	formula := func(id doc.ID) *doc.Node {
		return doc.NewNodeWithID(id, profile.Formula, doc.Attrs{}, "", doc.InlineSlot(doc.TextItem("x", empty)))
	}
	paragraph := doc.NewNodeWithID(2, profile.Paragraph, doc.Attrs{}, "a", doc.InlineSlot(
		doc.TextItem("a", empty),
		doc.NodeItem(formula(10)),
		doc.NodeItem(formula(11)),
		doc.NodeItem(formula(12)),
		doc.TextItem("b", empty),
	))
	page := doc.NewNodeWithID(1, profile.Page, doc.Attrs{}, "", doc.BlockSlot(paragraph))
	snap, err := doc.NewSnapshot(profile.Schema(), marks, page, 99)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, Policy},
		Strict:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true})
	e.OpenView(1)
	h := &harness{t: t, e: e, now: time.Unix(1000, 0)}
	h.do(vocab.Input{Intent: vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}})
	h.typeText("中文,English")
	n, _ := e.Snapshot().Node(2)
	in := n.Slot(0).Content()
	if got := in.TextRange(0, in.Len()); got != "ab 中文，English" {
		t.Fatalf("text = %q, want %q", got, "ab 中文，English")
	}
}
