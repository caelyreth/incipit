package feedback_test

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/feedback"
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
	snap, err := vocab.EmptyPage(1, 9, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	return &page{t: t, e: e, now: time.Unix(10, 0)}
}

func (p *page) do(in vocab.Intent) *engine.Published {
	p.t.Helper()
	p.now = p.now.Add(time.Second)
	pub, err := p.e.Do(1, vocab.Input{Intent: in, At: p.now})
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
