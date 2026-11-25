package profile

import "github.com/caelyreth/neverland/internal/editor/vocab"

// Spelling marks the words the system's checker finds misspelled, and the
// grammar it questions, as the text reads: nothing is stored.
var (
	// GroupMisspelled is a word the checker does not know.
	GroupMisspelled = vocab.NewGroup("spelling.misspelled", 0)
	// GroupGrammar is text the checker questions the grammar of.
	GroupGrammar = vocab.NewGroup("spelling.grammar", 0)
)

var (
	intentLearnSpelling  = vocab.NewIntentKind("LearnSpelling")
	intentIgnoreSpelling = vocab.NewIntentKind("IgnoreSpelling")
)

type (
	// LearnSpelling adds the misspelled word at the caret to the user's
	// dictionary, which every app shares: the host is asked with a
	// LearnWord.
	LearnSpelling struct{}
	// IgnoreSpelling stops marking the misspelled word at the caret in
	// this view, until the view closes.
	IgnoreSpelling struct{}
)

func (LearnSpelling) Kind() vocab.IntentKind  { return intentLearnSpelling }
func (IgnoreSpelling) Kind() vocab.IntentKind { return intentIgnoreSpelling }

// The spelling commands.
const (
	LearnSpellingCommand  vocab.CommandID = "spelling.learn"
	IgnoreSpellingCommand vocab.CommandID = "spelling.ignore"
)

// LearnWord asks the host to add Word to the user's dictionary.
type LearnWord struct{ Word string }
