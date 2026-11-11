package feedback_test

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/feedback"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func open(t *testing.T) *page {
	t.Helper()
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	return openProgram(t, prog)
}

func openProgram(t *testing.T, prog *policy.Program) *page {
	t.Helper()
	snap, err := profile.EmptyPage(1, 9, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	return &page{t: t, e: e, now: time.Unix(10, 0)}
}

func (p *page) do(in vocab.Intent) *engine.Published {
	p.t.Helper()
	return p.doClass(in, 0)
}

func (p *page) doClass(in vocab.Intent, class doc.OriginClass) *engine.Published {
	p.t.Helper()
	p.now = p.now.Add(time.Second)
	pub, err := p.e.Do(1, vocab.Input{Intent: in, At: p.now, Class: class})
	if err != nil {
		p.t.Fatal(err)
	}
	return pub
}

func (p *page) typeText(s string) {
	p.t.Helper()
	for _, r := range s {
		p.do(vocab.Type{Text: string(r)})
	}
}

func (p *page) pulses() []doc.Caret {
	p.t.Helper()
	v, ok := p.e.View(1)
	if !ok {
		p.t.Fatal("no view")
	}
	var out []doc.Caret
	for _, a := range v.Annotations.Of(feedback.Pulse) {
		out = append(out, a.Anchor.From)
	}
	return out
}

func (p *page) pulseRanges() [][2]int {
	p.t.Helper()
	v, _ := p.e.View(1)
	var out [][2]int
	for _, a := range v.Annotations.Of(feedback.Pulse) {
		out = append(out, [2]int{a.Anchor.From.Pos.Offset, a.Anchor.To.Pos.Offset})
	}
	return out
}

func direction(t *testing.T, p *page) vocab.Direction {
	t.Helper()
	v, _ := p.e.View(1)
	for _, a := range v.Annotations.Of(feedback.Pulse) {
		if m, ok := a.Data.(interface{ DecorationMotion() vocab.Direction }); ok {
			return m.DecorationMotion()
		}
	}
	t.Fatal("no directional pulse")
	return 0
}

// Ordinary typing is not feedback; a rewrite pulses and its deadline
// removes the pulse from the view.
func TestRewritePulsesUntilTheirDeadline(t *testing.T) {
	p := open(t)
	p.typeText("abc")
	if got := p.pulses(); len(got) != 0 {
		t.Fatalf("plain typing left %d pulses", len(got))
	}
	p.typeText(" --")
	if got := p.pulses(); len(got) == 0 {
		t.Fatal("the characters rewrite left no pulse")
	}
	deadline := p.now.Add(feedback.PulseFor)
	pub, err := p.e.Do(1, vocab.Input{Intent: vocab.Timer{Deadline: deadline}, At: deadline})
	if err != nil {
		t.Fatal(err)
	}
	if pub == nil || len(p.pulses()) != 0 {
		t.Fatal("the pulse outlived its deadline")
	}
}

// Undo plays a pulse backwards and redo plays it forwards.
func TestHistoryPulsesPlayBothWays(t *testing.T) {
	p := open(t)
	p.typeText("a --")
	p.do(vocab.Undo{})
	if got := direction(t, p); got != vocab.Backward {
		t.Fatalf("undo pulse plays %v", got)
	}
	var painted vocab.Direction
	for _, h := range p.e.Decorations(1).Highlights {
		if h.Group == vocab.GroupPulse {
			painted = h.Direction
		}
	}
	if painted != vocab.Backward {
		t.Fatalf("undo decoration plays %v", painted)
	}
	p.do(vocab.Redo{})
	if got := direction(t, p); got != vocab.Forward {
		t.Fatalf("redo pulse plays %v", got)
	}
}

// A structural rewrite from text that arrived at once pulses the block it
// made, not only character replacements.
func TestStructuralRewritePulses(t *testing.T) {
	structure := policy.Define("test-structure", func(p *policy.P) {
		p.Rewrite("heading", doc.PhaseStructure).
			Trigger(policy.Trigger{Kinds: []doc.RecordKind{doc.RecordTextInserted}}).
			Extent(policy.ChangedRanges).
			Produces(doc.RecordNodeChanged).
			Do(func(c *policy.Ctx, site policy.Site) policy.Result {
				n, ok := c.Doc().Node(site.Slot.Node)
				if !ok || n.Kind() != profile.Paragraph {
					return policy.Swallow
				}
				return policy.Do(func(tx *doc.Tx) error {
					return tx.Apply(doc.SetNode{ID: n.ID(), Kind: profile.Heading,
						Attrs: doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(1)})})
				})
			})
	})
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, structure, feedback.Policy}, Strict: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := openProgram(t, prog)
	p.do(vocab.Type{Text: "x"})
	if got := p.pulses(); len(got) == 0 {
		t.Fatal("the structural rewrite left no pulse")
	}
}

// Automatic rewrites pulse their actual change, not the replacement
// operation they used to produce it.
func TestRewritePulsesCoverOnlyChangedText(t *testing.T) {
	p := open(t)
	p.typeText("你好")
	p.do(vocab.Type{Text: "E"})
	if got := p.pulseRanges(); len(got) != 1 || got[0] != [2]int{len("你好"), len("你好") + 1} {
		t.Fatalf("CJK pulse ranges = %v", got)
	}

	p = open(t)
	p.typeText("a,")
	p.do(vocab.Type{Text: "b"})
	if got := p.pulseRanges(); len(got) != 1 || got[0] != [2]int{2, 3} {
		t.Fatalf("punctuation pulse ranges = %v", got)
	}
}

// A deletion-only rewrite pulses the grapheme beside the removed gap.
func TestDeletionPulseHasAVisibleNeighbor(t *testing.T) {
	p := open(t)
	p.typeText("hello ,")
	if got := p.pulseRanges(); len(got) != 1 || got[0] != [2]int{4, 5} {
		t.Fatalf("deletion pulse ranges = %v", got)
	}
}
