package showcase

import (
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// For returns the showcase of an editor policy, if it has one.
func For(policy kernel.PolicyID) (Script, bool) {
	script, ok := scripts[policy]
	return script, ok
}

// Configuration is source with policy turned on: a showcase always shows
// its feature working, under the options the person chose for it.
func Configuration(policy kernel.PolicyID, source kernel.PolicyConfiguration) kernel.PolicyConfiguration {
	configuration := source.Clone()
	configuration.SetEnabled(policy, true)
	return configuration
}

var scripts = map[kernel.PolicyID]Script{
	editor.PairsPolicyID: {
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
	editor.WritingPolicyID: {
		Beats: []Beat{
			Type("## Tides"),
			Wait(500 * time.Millisecond),
			Type("\nWrite *softly"),
			Wait(500 * time.Millisecond),
			Type("*, or **firmly"),
			Wait(500 * time.Millisecond),
			Type("*.\n- keep `notes"),
			Wait(500 * time.Millisecond),
			Type("` and ~strike~ drafts"),
			Wait(500 * time.Millisecond),
			Select("Write"),
			Press(editor.EmphasisCommandID),
		},
		Hold: 2600 * time.Millisecond,
	},
	editor.CJKPolicyID: {
		Beats: []Beat{
			Type("我用Go写作,每天写500字."),
		},
		Hold: 2800 * time.Millisecond,
	},
	editor.TypographyPolicyID: {
		Beats: []Beat{
			Type(`Wait... -- it's "almost" done.`),
		},
		Hold: 2800 * time.Millisecond,
	},
	editor.HistoryPolicyID: {
		Beats: []Beat{
			Type("A first draft, then a second thought."),
			Wait(500 * time.Millisecond),
			Press(editor.UndoCommandID),
			Wait(450 * time.Millisecond),
			Press(editor.UndoCommandID),
			Wait(700 * time.Millisecond),
			Press(editor.RedoCommandID),
			Wait(450 * time.Millisecond),
			Press(editor.RedoCommandID),
		},
		Hold: 2400 * time.Millisecond,
	},
	editor.NavigationPolicyID: {
		Beats: []Beat{
			Type("Leave a scope (with Tab"),
			Wait(1100 * time.Millisecond),
			Press(editor.LeaveCommandID),
			Wait(300 * time.Millisecond),
			Type(" and keep writing."),
		},
		Hold: 2600 * time.Millisecond,
	},
	editor.WordsPolicyID: {
		Setup: []Beat{
			Type("Words move as units. Sentences do too."),
		},
		Beats: []Beat{
			Wait(600 * time.Millisecond),
			Press(editor.PreviousWordCommandID),
			Press(editor.PreviousWordCommandID),
			Press(editor.PreviousWordCommandID),
			Wait(400 * time.Millisecond),
			Press(editor.SelectWordCommandID),
			Wait(700 * time.Millisecond),
			Press(editor.SelectSentenceCommandID),
			Wait(900 * time.Millisecond),
		},
		Hold: 2000 * time.Millisecond,
	},
	editor.TextPolicyID: {
		Beats: []Beat{
			Type("Every keystroke is an edit"),
			Wait(500 * time.Millisecond),
			Press(editor.ExtendLeftCommandID),
			Press(editor.ExtendLeftCommandID),
			Press(editor.ExtendLeftCommandID),
			Press(editor.ExtendLeftCommandID),
			Wait(400 * time.Millisecond),
			Type("intent."),
		},
		Hold: 2400 * time.Millisecond,
	},
}
