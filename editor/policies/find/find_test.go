package find_test

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/find"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func open(t *testing.T, text string) *page {
	t.Helper()
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 9, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	p := &page{t: t, e: engine.New(snap, prog, engine.Options{}), now: time.Unix(10, 0)}
	p.e.OpenView(1)
	p.do(vocab.Type{Text: text})
	return p
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

func (p *page) view() policy.View {
	v, _ := p.e.View(1)
	return v
}

func (p *page) matches() []find.Spot {
	v := p.view()
	return find.Matches(p.e.Program(), p.e.Store(), p.e.Snapshot(), &v)
}

// selected is what the selection selects, as offsets in its slot.
func (p *page) selected() [2]int {
	from, to := p.view().Selection.Ordered(p.e.Snapshot())
	return [2]int{from.Offset, to.Offset}
}

func TestSeekShowsEveryMatchAndSelectsTheNextFromTheSelection(t *testing.T) {
	p := open(t, "Budget plan, and the budget.")
	pub := p.do(find.Seek{Query: "budget"})
	if got := p.matches(); len(got) != 2 || got[0].From != 0 || got[1].From != 21 {
		t.Fatalf("matches %+v", got)
	}
	// The caret stood at the end: the find goes round to the first.
	if p.selected() != [2]int{0, 6} {
		t.Fatalf("selected %v", p.selected())
	}
	if len(pub.Effects) == 0 || pub.Effects[0].Kind != policy.EffectReveal {
		t.Fatalf("the match is not shown: %+v", pub.Effects)
	}
	if len(p.view().Annotations.Of(find.Flash)) != 1 {
		t.Fatal("the match does not flash")
	}
	// A query that grows keeps to the match from where the selection
	// stands, or goes on to the next that holds it.
	p.do(find.Seek{Query: "budge"})
	if p.selected() != [2]int{0, 6} {
		t.Fatalf("a word being typed selects %v", p.selected())
	}
	p.do(find.Seek{Query: `"the budget`})
	if p.selected() != [2]int{17, 27} {
		t.Fatalf("a phrase being typed selects %v", p.selected())
	}
}

func TestStepGoesRoundTheMatches(t *testing.T) {
	p := open(t, "one two one two one")
	p.do(find.Seek{Query: "one"})
	var got [][2]int
	for range 3 {
		p.do(find.Step{Dir: vocab.Forward})
		got = append(got, p.selected())
	}
	p.do(find.Step{Dir: vocab.Backward})
	got = append(got, p.selected())
	want := [][2]int{{8, 11}, {16, 19}, {0, 3}, {16, 19}}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("steps %v, want %v", got, want)
		}
	}
}

func TestMatchesFollowTheTextAndTheQuery(t *testing.T) {
	p := open(t, "a tide")
	p.do(find.Seek{Query: "tide"})
	slot := p.matches()[0].Slot
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(slot, 6))})
	p.do(vocab.Type{Text: " and a tide"})
	if got := p.matches(); len(got) != 2 {
		t.Fatalf("a match typed is found at once: %+v", got)
	}
	p.do(vocab.Undo{})
	if got := p.matches(); len(got) != 1 {
		t.Fatalf("undo takes it: %+v", got)
	}
	p.do(find.Seek{Query: ""})
	if got := p.matches(); len(got) != 0 || p.view().Modes.Of(find.ID) != nil {
		t.Fatalf("an empty query ends the find: %+v", got)
	}
	if pub := p.do(find.Step{Dir: vocab.Forward}); pub.Changed {
		t.Fatal("a step with no find changes the page")
	}
}

func TestAKeysWordsMatchOnlyInWhatTheKeyNames(t *testing.T) {
	p := open(t, "plans")
	p.do(vocab.Break{})
	p.do(vocab.Type{Text: "the plans ahead"})
	p.do(find.Seek{Query: "$heading:plans"})
	if got := p.matches(); len(got) != 0 {
		t.Fatalf("no heading holds it: %+v", got)
	}
	p.do(find.Seek{Query: "plans"})
	if got := p.matches(); len(got) != 2 {
		t.Fatalf("plans anywhere: %+v", got)
	}
}
