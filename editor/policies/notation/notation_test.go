package notation

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/lists"
	"github.com/caelyreth/neverland/editor/policies/markdown"
	"github.com/caelyreth/neverland/editor/policies/navigation"
	"github.com/caelyreth/neverland/editor/policies/pairs"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
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
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, Policy, pairs.Policy, markdown.Policy, lists.Policy, navigation.Policy},
		Strict:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 5, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true})
	e.OpenView(1)
	return &page{t: t, e: e, now: time.Unix(10, 0)}
}

func (p *page) do(in vocab.Intent) {
	p.t.Helper()
	p.now = p.now.Add(time.Second)
	if _, err := p.e.Do(1, vocab.Input{Intent: in, At: p.now}); err != nil {
		p.t.Fatal(err)
	}
}

func (p *page) typeText(s string) {
	p.t.Helper()
	for _, r := range s {
		p.do(vocab.Type{Text: string(r)})
	}
}

func (p *page) item() *doc.Node {
	p.t.Helper()
	return p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Child(0)
}

func (p *page) marker() string {
	p.t.Helper()
	item := p.item()
	pres := p.e.Present(1, []doc.ID{item.ID()})[0]
	var out string
	for _, it := range pres.Surfaces[0].Flow.Items {
		if it.Kind == present.GhostItem {
			out += string(it.Spelling)
		}
	}
	return out
}

func TestTypedGlyphsToggleATaskBox(t *testing.T) {
	p := open(t)
	p.typeText("- [ ] ")
	if got := p.marker(); got != "- [ ] " {
		t.Fatalf("marker spells %q, want an open task", got)
	}
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.typeText("x")
	if got := profile.TaskOf(p.item()); got != profile.TaskDone {
		t.Fatalf("typing x made task %v, want done", got)
	}
	if text := p.item().Slot(0).Content().Text(); text != "" {
		t.Fatalf("typing x left %q in the item", text)
	}
	if got := p.marker(); got != "- [x] " {
		t.Fatalf("marker spells %q, want a done task", got)
	}
	p.typeText(" ")
	if got := profile.TaskOf(p.item()); got != profile.TaskOpen {
		t.Fatalf("typing space made task %v, want open", got)
	}
}

func TestTaskCommandTogglesTheItemAtTheCaret(t *testing.T) {
	p := open(t)
	p.typeText("- [ ] ")
	if _, ok := p.e.Program().Command("lists.toggleTask"); !ok {
		t.Fatal("the program has no core.toggleTask command")
	}
	p.do(vocab.Command{ID: "lists.toggleTask"})
	if got := profile.TaskOf(p.item()); got != profile.TaskDone {
		t.Fatalf("the command made task %v, want done", got)
	}
	p.do(vocab.Command{ID: "lists.toggleTask"})
	if got := profile.TaskOf(p.item()); got != profile.TaskOpen {
		t.Fatalf("the second command made task %v, want open", got)
	}
}
