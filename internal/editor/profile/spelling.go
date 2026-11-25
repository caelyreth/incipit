package profile

import (
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// Spelling marks the words the system's checker finds misspelled, and the
// grammar it questions, as the text reads: nothing is stored.
var (
	// GroupMisspelled is a word the checker does not know.
	GroupMisspelled = vocab.NewGroup("spelling.misspelled", 0)
	// GroupGrammar is text the checker questions the grammar of.
	GroupGrammar = vocab.NewGroup("spelling.grammar", 0)
	// GroupCorrected is a word the checker replaced automatically.
	GroupCorrected = vocab.NewGroup("spelling.corrected", 0)
	// GroupTypingWord marks the word currently touching the caret while typing.
	GroupTypingWord = vocab.NewGroup("spelling.typing", 0)
	// GroupSuggestion is an inline predictive completion ahead of the caret.
	GroupSuggestion = vocab.NewGroup("spelling.suggestion", 0)
)

var (
	intentLearnSpelling  = vocab.NewIntentKind("LearnSpelling")
	intentIgnoreSpelling = vocab.NewIntentKind("IgnoreSpelling")
	intentCorrect        = vocab.NewIntentKind("CorrectSpelling")
	intentRevert         = vocab.NewIntentKind("RevertCorrection")
	intentSetSuggestion  = vocab.NewIntentKind("SetSuggestion")
)

type (
	// LearnSpelling adds the misspelled word at the caret to the user's
	// dictionary, which every app shares: the host is asked with a
	// LearnWord.
	LearnSpelling struct{}
	// IgnoreSpelling stops marking the misspelled word at the caret in
	// this view, until the view closes.
	IgnoreSpelling struct{}
	// CorrectSpelling writes Word over the misspelled word at the caret,
	// as a guess the page's menu offers.
	CorrectSpelling struct{ Word string }
	// RevertCorrection restores the word an autocorrection replaced at At.
	RevertCorrection struct{ At doc.Caret }
	// SetSuggestion sets or clears the active inline prediction at At.
	SetSuggestion struct {
		At   doc.Pos
		Text string
	}
)

func (LearnSpelling) Kind() vocab.IntentKind    { return intentLearnSpelling }
func (IgnoreSpelling) Kind() vocab.IntentKind   { return intentIgnoreSpelling }
func (CorrectSpelling) Kind() vocab.IntentKind  { return intentCorrect }
func (RevertCorrection) Kind() vocab.IntentKind { return intentRevert }
func (SetSuggestion) Kind() vocab.IntentKind    { return intentSetSuggestion }

// The spelling commands.
const (
	LearnSpellingCommand  vocab.CommandID = "spelling.learn"
	IgnoreSpellingCommand vocab.CommandID = "spelling.ignore"
)

// LearnWord asks the host to add Word to the user's dictionary.
type LearnWord struct{ Word string }

// PredictAsk asks the host for an inline text prediction at At given Text
// immediately preceding it on the same line.
type PredictAsk struct {
	At   doc.Pos
	Text string
}
