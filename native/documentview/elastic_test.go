package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/scrollfx"
)

// A scroll past its end stops there and pulls the edge, which gives back
// what passes back through it and settles at the edge again.
func TestAnElasticScrollGivesAtItsEnds(t *testing.T) {
	var pull scrollfx.Elastic
	at := elasticScroll(&pull, 90, 40, 0, 100, 600, 0, scrollfx.Direct)
	if at != 100 || pull.Offset() == 0 {
		t.Fatalf("past the end: at %v, pulled %v", at, pull.Offset())
	}
	if back := elasticScroll(&pull, at, -1, 0, 100, 600, 0, scrollfx.Direct); back != 100 {
		t.Fatalf("a little back gives the pull back first, yet scrolled to %v", back)
	}
	now := time.Unix(0, 0)
	for range 120 {
		now = now.Add(16 * time.Millisecond)
		pull.Step(now, true)
	}
	if pull.Offset() != 0 {
		t.Fatalf("the pull did not settle: %v", pull.Offset())
	}
	if at := elasticScroll(&pull, 50, 10, 0, 100, 600, 0, scrollfx.Direct); at != 60 || pull.Offset() != 0 {
		t.Fatalf("within its ends: at %v, pulled %v", at, pull.Offset())
	}
}
