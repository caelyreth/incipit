package vocab

import "github.com/caelyreth/neverland/editor/doc"

// The selection's own intents, past the primitives: the ladder of what
// stands around the selection, a line, and the other side of a pair.
// Each is a feature some policy answers; a program without it does
// nothing with them.
var (
	intentExpand = NewIntentKind("ExpandSelection")
	intentShrink = NewIntentKind("ShrinkSelection")
	intentLine   = NewIntentKind("SelectLine")
	intentMatch  = NewIntentKind("MatchPair")
)

type (
	// ExpandSelection selects the next rung of the ladder around the
	// selection: the word at the caret, the mark or pair around it, its
	// sentence, its block, and each scope around that, up to the page.
	ExpandSelection struct{}
	// ShrinkSelection goes back down the rungs ExpandSelection climbed,
	// to where it started.
	ShrinkSelection struct{}
	// SelectLine selects the caret's line, from Start to End, as the
	// renderer resolved them; with whole lines selected, the next one
	// with them.
	SelectLine struct{ Start, End doc.Caret }
	// MatchPair puts the caret at the other inner end of the pair or mark
	// it stands in.
	MatchPair struct{}
)

func (ExpandSelection) Kind() IntentKind { return intentExpand }
func (ShrinkSelection) Kind() IntentKind { return intentShrink }
func (SelectLine) Kind() IntentKind      { return intentLine }
func (MatchPair) Kind() IntentKind       { return intentMatch }
