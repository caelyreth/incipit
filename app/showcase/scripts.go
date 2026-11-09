package showcase

import (
	"time"

	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// For returns the showcase of a policy, if it has one.
func For(policyID string) (Script, bool) {
	script, ok := scripts[policyID]
	return script, ok
}

func extendLeft() vocab.Intent {
	return vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true}
}

var scripts = map[string]Script{
	policy.CoreID: {
		Beats: []Beat{
			Type("Every keystroke is an edit"),
			Wait(500 * time.Millisecond),
			Press(extendLeft()), Press(extendLeft()), Press(extendLeft()), Press(extendLeft()),
			Wait(400 * time.Millisecond),
			Type("intent."),
		},
		Hold: 2400 * time.Millisecond,
	},
	"notation": {
		Beats: []Beat{
			Type("Marks keep their *signs"),
			Wait(500 * time.Millisecond),
			Type("*, as **written**."),
		},
		Hold: 2600 * time.Millisecond,
	},
	"markdown": {
		Beats: []Beat{
			Type("## Tides"),
			Wait(500 * time.Millisecond),
			Type("\nWrite *softly"),
			Wait(500 * time.Millisecond),
			Type("*, or **firmly**."),
		},
		Hold: 2600 * time.Millisecond,
	},
	"lists": {
		Beats: []Beat{
			Type("- tides\nebb"),
			Wait(500 * time.Millisecond),
			Press(vocab.Nest{Dir: vocab.Forward}),
			Wait(300 * time.Millisecond),
			Type("\nflow"),
			Wait(500 * time.Millisecond),
			Press(vocab.Nest{Dir: vocab.Backward}),
		},
		Hold: 2400 * time.Millisecond,
	},
	"pairs": {
		Beats: []Beat{
			Type("The tide turned (slowly"),
			Wait(700 * time.Millisecond),
			Type(")"),
			Wait(500 * time.Millisecond),
			Type(` and she said "stay`),
			Wait(600 * time.Millisecond),
			Type(`".`),
		},
		Hold: 2600 * time.Millisecond,
	},
	"navigation": {
		Beats: []Beat{
			Type("Leave a scope (with Tab"),
			Wait(1100 * time.Millisecond),
			Press(vocab.Leave{}),
			Wait(300 * time.Millisecond),
			Type(" and keep writing."),
		},
		Hold: 2600 * time.Millisecond,
	},
	"guides": {
		Beats: []Beat{
			Type("Keys show [where they act"),
			Wait(1400 * time.Millisecond),
			Press(vocab.Leave{}),
			Wait(1400 * time.Millisecond),
			Press(vocab.Return{}),
			Wait(600 * time.Millisecond),
		},
		Hold: 2000 * time.Millisecond,
	},
	"history": {
		Beats: []Beat{
			Type("A first draft, then a second thought."),
			Wait(500 * time.Millisecond),
			Press(vocab.Undo{}),
			Wait(450 * time.Millisecond),
			Press(vocab.Undo{}),
			Wait(700 * time.Millisecond),
			Press(vocab.Redo{}),
			Wait(450 * time.Millisecond),
			Press(vocab.Redo{}),
		},
		Hold: 2400 * time.Millisecond,
	},
}
