package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

func TestHostEffectsReachTheHost(t *testing.T) {
	el := newElement(t)
	var got []policy.EffectKind
	el.Editor.Effect = func(fx policy.Effect) { got = append(got, fx.Kind) }
	el.Editor.Do(vocab.Undo{}, time.Unix(700, 0))
	if len(got) != 1 || got[0] != policy.EffectBeep {
		t.Fatalf("effects = %v, want one beep", got)
	}
}

func TestRevealStaysInsideThePage(t *testing.T) {
	el := newElement(t)
	var got []policy.EffectKind
	el.Editor.Effect = func(fx policy.Effect) { got = append(got, fx.Kind) }
	el.Editor.Do(vocab.Type{Text: "a"}, time.Unix(701, 0))
	if len(got) != 0 {
		t.Fatalf("reveal reached the host: %v", got)
	}
}
