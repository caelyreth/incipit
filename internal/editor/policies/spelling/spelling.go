// Package spelling is the spelling policy: the words the system's checker
// does not know are marked with a wave, and the grammar it questions
// where asked, as the text reads, so nothing is stored and the marks move
// with the text. A word may be learned, into the user's dictionary every
// app shares, or ignored in the view until it closes. Code, formula
// source and tags are not prose and are not checked.
package spelling

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "spelling"

// Checker is the system's spelling and grammar checker. Its methods may
// be called from any goroutine.
type Checker interface {
	// Check reports text's misspelled words and, with grammar, its grammar
	// issues, in bytes of text.
	Check(text string, grammar bool) []Issue
}

// Issue is a misspelled word, or a grammar issue: bytes From to To.
type Issue struct {
	From, To int
	Grammar  bool
}

var checker Checker

// Use makes c the checker the policy asks, before a program is compiled;
// nil, as where the system has none, checks nothing.
func Use(c Checker) { checker = c }

// Grammar checks grammar along with spelling.
var Grammar = policy.Bool("grammar", false, policy.Label("settings.spelling.grammar"), policy.Description("settings.spelling.grammar.description"))

// issue is a spelling fact's data.
type issue struct{}

// Policy is the spelling policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	misspelled := policy.Facts[issue](p, "misspelled", policy.Styles(profile.GroupMisspelled))
	questioned := policy.Facts[issue](p, "grammar", policy.Styles(profile.GroupGrammar))
	grammar := policy.Get(p, Grammar)
	c := checker
	if c == nil {
		p.Unlisted()
	}
	// Lines are checked one by one, so a keystroke checks its own line
	// again and every other line's marks come from the store.
	p.Analyze(policy.Lines, analyze(c, grammar, misspelled, questioned), []policy.FactID{misspelled.Kind(), questioned.Kind()},
		policy.Where(func(u *policy.Unit) bool { return c != nil && !u.Spec.Literal && u.Spec.Marks != nil }),
		policy.Config(struct{ Checker, Grammar bool }{c != nil, grammar}))

	p.Command(profile.LearnSpellingCommand, "commands.learnSpelling", profile.LearnSpelling{}, policy.Offer(profile.SpellingOffers, 0, ""))
	p.Command(profile.IgnoreSpellingCommand, "commands.ignoreSpelling", profile.IgnoreSpelling{}, policy.Offer(profile.SpellingOffers, 1, ""))
	policy.On(p, "learn", policy.Intent[profile.LearnSpelling](), misspelled.Inside(), func(c *policy.Ctx, _ profile.LearnSpelling, at policy.At[issue]) policy.Result {
		word := wordOf(c, at)
		res := ignore(c, word)
		res.Effects = []policy.Effect{{Kind: policy.EffectLearnWord, Data: profile.LearnWord{Word: word}}}
		return res
	}).Caret()
	policy.On(p, "ignore", policy.Intent[profile.IgnoreSpelling](), misspelled.Inside(), func(c *policy.Ctx, _ profile.IgnoreSpelling, at policy.At[issue]) policy.Result {
		return ignore(c, wordOf(c, at))
	}).Caret()
	policy.On(p, "correct", policy.Intent[profile.CorrectSpelling](), misspelled.Inside(), correct).Caret()
}, policy.Label("policies.spelling"), policy.Description("policies.spelling.description"))

// ignored are the words a view no longer marks, the policy's mode: sorted,
// each ended by a newline, so the mode is a comparable value.
type ignored string

func (s ignored) has(word string) bool {
	return strings.Contains(string(s), "\n"+word+"\n") || strings.HasPrefix(string(s), word+"\n")
}

func (s ignored) with(word string) ignored {
	words := strings.Split(strings.TrimSuffix(string(s), "\n"), "\n")
	if s == "" {
		words = nil
	}
	if i, found := slices.BinarySearch(words, word); !found {
		words = slices.Insert(words, i, word)
	}
	return ignored(strings.Join(words, "\n") + "\n")
}

// ignore stops the view marking word; the view's lines are checked again.
func ignore(c *policy.Ctx, word string) policy.Result {
	if word == "" {
		return policy.Pass
	}
	words, _ := c.Mode(ID).(ignored)
	return policy.Result{Mode: words.with(word)}
}

// correct writes the guess over the misspelled word, in the marks its
// first letter wears, the caret after it.
func correct(c *policy.Ctx, in profile.CorrectSpelling, at policy.At[issue]) policy.Result {
	content, ok := c.Inline(at.Slot)
	if !ok || in.Word == "" || strings.ContainsAny(in.Word, "\n"+string(doc.NodeByte)) {
		return policy.Pass
	}
	from, to := at.Fact.Outer.From.At, at.Fact.Outer.To.At
	var marks *doc.MarkSet
	if it, _, ok := content.ItemAt(from); ok {
		marks = it.Marks
	}
	ref, end := at.Slot, from+len(in.Word)
	res := policy.Do(vocab.Ops(doc.EditText{At: ref, From: from, To: to, Items: []doc.Item{doc.TextItem(in.Word, marks)}}))
	res.Then = func(doc.Reader) doc.Selection { return doc.Collapsed(doc.CaretAt(doc.At(ref, end))) }
	return res
}

// wordOf is the text of the misspelled word at.
func wordOf(c *policy.Ctx, at policy.At[issue]) string {
	content, ok := c.Inline(at.Slot)
	if !ok {
		return ""
	}
	return content.FlatRange(at.Fact.Outer.From.At, at.Fact.Outer.To.At)
}

// analyze checks a line: its inline objects read as spaces, its tags are
// not words to check, and the words the view ignores are not marked. A
// grammar issue crossing a misspelled word is left to the word.
func analyze(c Checker, grammar bool, misspelled, questioned *policy.FactKind[issue]) func(u *policy.Unit, e *policy.Emit) {
	return func(u *policy.Unit, e *policy.Emit) {
		text := u.Text()
		ignoring, _ := u.Mode.(ignored)
		tags := profile.TagsIn(u.Content, u.From, u.To)
		issues := c.Check(strings.ReplaceAll(text, string(doc.NodeByte), " "), grammar)
		// Words first, then grammar that crosses none, emitted in order.
		var words, shown []Issue
		for _, is := range issues {
			from, to := u.From+is.From, u.From+is.To
			if !is.Grammar && from < to && !ignoring.has(text[is.From:is.To]) &&
				!slices.ContainsFunc(tags, func(t profile.Tag) bool { return t.From < to && from < t.To }) {
				words = append(words, is)
			}
		}
		for _, is := range issues {
			crosses := func(m Issue) bool { return m.From < is.To && is.From < m.To }
			if is.Grammar && is.From < is.To && !slices.ContainsFunc(words, crosses) && !slices.ContainsFunc(shown, crosses) {
				shown = append(shown, is)
			}
		}
		shown = append(shown, words...)
		slices.SortFunc(shown, func(a, b Issue) int { return a.From - b.From })
		for _, is := range shown {
			kind := misspelled
			if is.Grammar {
				kind = questioned
			}
			e.Fact(kind.Span(u.From+is.From, u.From+is.To, issue{}))
		}
	}
}
