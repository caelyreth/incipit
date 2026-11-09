package showcase

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/engine"
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

// Every story plays to its end under the default values: the engine
// answers each beat.
func TestEveryScriptPlaysThrough(t *testing.T) {
	now := time.Unix(1000, 0)
	for id, script := range scripts {
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
	p := NewPlayer(scripts["history"], "history", nil, open, now)
	for at := now; at.Before(now.Add(30 * time.Second)); at = at.Add(50 * time.Millisecond) {
		p.Advance(at)
	}
	if p.Loop() < 2 || p.Err() != nil {
		t.Fatalf("loop %d, %v", p.Loop(), p.Err())
	}
}
