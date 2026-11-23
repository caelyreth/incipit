package motion

import (
	"testing"
	"time"
)

// Decorations arrive quickly and leave more gently, so they do not feel
// delayed or snap away.
func TestFadeHasFastEntranceAndSlowerExit(t *testing.T) {
	now := time.Unix(0, 0)
	var fade Fade
	if got := fade.Step(true, now, true); got != 0 {
		t.Fatalf("starts at %v", got)
	}
	if got := fade.Step(true, now.Add(FadeInDuration), true); got != 1 {
		t.Fatalf("entrance ends at %v", got)
	}
	if got := fade.Step(false, now.Add(FadeInDuration), true); got != 1 {
		t.Fatalf("exit starts at %v", got)
	}
	if got := fade.Step(false, now.Add(FadeInDuration+FadeOutDuration/2), true); got <= 0 || got >= 1 {
		t.Fatalf("mid-exit opacity = %v", got)
	}
	if got := fade.Step(false, now.Add(FadeInDuration+FadeOutDuration), true); got != 0 {
		t.Fatalf("exit ends at %v", got)
	}
}

func TestFadeSetHandsOffTheCurrentOpacity(t *testing.T) {
	now := time.Unix(0, 0)
	var fade Fade
	fade.Set(0.4)
	if got := fade.Step(false, now, true); got != 0.4 {
		t.Fatalf("handoff started at %v", got)
	}
	if got := fade.Step(false, now.Add(FadeOutDuration), true); got != 0 {
		t.Fatalf("handoff ended at %v", got)
	}
}
