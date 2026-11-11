package showcase

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

type enginePage struct{ e *engine.Engine }

func (p enginePage) Do(in vocab.Intent, at time.Time) bool {
	_, err := p.e.Do(View, vocab.Input{Intent: in, At: at})
	return err == nil
}

func open(e *engine.Engine) Page {
	e.OpenView(View)
	return enginePage{e}
}

// showcases are the product's declared showcases, by policy.
func showcases(t *testing.T) map[string]policy.Showcase {
	prog, err := manifest.Compile(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]policy.Showcase{}
	for _, p := range prog.Catalog().Policies {
		if p.Showcase != nil {
			out[p.ID] = *p.Showcase
		}
	}
	return out
}

// Every story plays to its end under the default values: the engine
// answers each beat.
func TestEveryScriptPlaysThrough(t *testing.T) {
	now := time.Unix(1000, 0)
	for id, script := range showcases(t) {
		p := NewPlayer(script, id, nil, open, now)
		p.Finish()
		if p.Err() != nil {
			t.Errorf("%s: %v", id, p.Err())
		}
	}
}

// A story loops: it fades out after its hold and starts over.
func TestAStoryLoops(t *testing.T) {
	now := time.Unix(1000, 0)
	p := NewPlayer(showcases(t)["history"], "history", nil, open, now)
	for at := now; at.Before(now.Add(30 * time.Second)); at = at.Add(50 * time.Millisecond) {
		p.Advance(at)
	}
	if p.Loop() < 2 || p.Err() != nil {
		t.Fatalf("loop %d, %v", p.Loop(), p.Err())
	}
}
