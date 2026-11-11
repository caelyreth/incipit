package motion

import (
	"testing"
	"time"
)

// The caret is solid a moment after input, then breathes through fully
// transparent at the middle of its period and back.
func TestTheCaretBreathes(t *testing.T) {
	at := time.Unix(100, 0)
	for _, c := range []struct {
		after time.Duration
		want  float32
	}{
		{0, 1},
		{CaretSolid - time.Millisecond, 1},
		{CaretSolid, 1},
		{CaretSolid + CaretBreath/2, 0},
		{CaretSolid + CaretBreath, 1},
	} {
		if got := CaretOpacity(at, at.Add(c.after)); got < c.want-0.001 || got > c.want+0.001 {
			t.Fatalf("opacity %v after %v, want %v", got, c.after, c.want)
		}
	}
	if wake, ok := CaretWake(at, at.Add(time.Millisecond)); !ok || !wake.Equal(at.Add(CaretSolid)) {
		t.Fatalf("wake %v %v", wake, ok)
	}
}
