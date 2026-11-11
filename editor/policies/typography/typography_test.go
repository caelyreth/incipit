package typography

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

func TestTypedSymbolsBecomeTypographic(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("...")
	h.want("…")

	h = newHarness(t, nil)
	h.typeText("a--b")
	h.want("a—b")

	h = newHarness(t, nil)
	h.typeText("a---b")
	h.want("a—b")

	h = newHarness(t, nil)
	h.typeText("---")
	h.want("---")
}

func TestTypedApostrophesAndQuotePlacement(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("can't")
	h.want("can’t")

	h = newHarness(t, nil)
	h.typeText(`He said "go".`)
	h.want(`He said "go."`)
}

func TestPunctuationSpacing(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("hello ,world")
	h.want("hello, world")
}

// Spacing emits only the inserted space.
func TestPunctuationSpacingEmitsMinimalChanges(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("hello,")
	pub := h.do(vocab.Input{Intent: vocab.Type{Text: "w"}})
	h.want("hello, w")
	inserted := false
	for _, r := range pub.Records {
		if r.Origin.Phase != doc.PhaseCharacters {
			continue
		}
		switch r.Kind {
		case doc.RecordTextInserted:
			inserted = r.Text == " " && r.From == len("hello,") && r.To == len("hello,")+1
		case doc.RecordTextDeleted:
			t.Fatalf("spacing deleted %q at %d", r.Text, r.From)
		}
	}
	if !inserted {
		t.Fatalf("character records = %+v", pub.Records)
	}
}

func TestIMECommitRunsTheRewrite(t *testing.T) {
	h := newHarness(t, nil)
	h.do(vocab.Input{Intent: vocab.Type{Text: "..."}, Class: doc.OriginIME})
	h.want("…")
}

func TestNonTypingOriginsDoNotRun(t *testing.T) {
	h := newHarness(t, nil)
	h.do(vocab.Input{Intent: vocab.Paste{Text: "can't -- ..."}})
	h.want("can't -- ...")

	h = newHarness(t, nil)
	h.do(vocab.Input{Intent: vocab.Type{Text: "..."}, Class: doc.OriginCommand})
	h.want("...")
}

func TestSymbolsOptionCanBeOff(t *testing.T) {
	h := newHarness(t, policy.Values{"typography.symbols": false})
	h.typeText("...")
	h.want("...")
}

func TestCharactersTriggerIsTypingAndIMEOnly(t *testing.T) {
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, Policy},
		Strict:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rewrites := prog.Rewrites(doc.PhaseCharacters)
	if len(rewrites) != 1 {
		t.Fatalf("Characters rewrites = %d, want 1", len(rewrites))
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
	h.typeText("...")
	n, _ := e.Snapshot().Node(2)
	in := n.Slot(0).Content()
	if got := in.TextRange(0, in.Len()); got != "ab…" {
		t.Fatalf("text = %q, want %q", got, "ab…")
	}
}
