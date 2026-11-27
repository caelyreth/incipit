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
	"sync"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "spelling"

// Checker is the system's spelling and grammar checker. Its methods may
// be called from any goroutine.
type Checker interface {
	// Check reports text's misspelled words and, with grammar, its grammar
	// issues, in bytes of text.
	Check(text string, grammar bool) []Issue
	// Correction returns a single spelling correction for bytes [from:to]
	// of text, or "".
	Correction(text string, from, to int) string
}

// Issue is a misspelled word, or a grammar issue: bytes From to To.
type Issue struct {
	From, To int
	Grammar  bool
}

var checker Checker

// Use makes c the checker the policy asks, before a program is compiled;
// nil, as where the system has none, checks nothing.
func Use(c Checker) {
	checker = nil
	if c != nil {
		checker = &recent{Checker: c}
	}
}

// recent remembers the checker's last answers by text. The system takes
// time in proportion to the text it reads, and one keystroke reads the
// same sentence more than once: as the caret's line is laid out during
// the transaction, then as the view is presented, and again as the word
// being typed changes the sentence's context.
type recent struct {
	Checker
	mu      sync.Mutex
	answers []answer
}

type answer struct {
	text    string
	grammar bool
	issues  []Issue
}

const recentAnswers = 8

func (r *recent) Check(text string, grammar bool) []Issue {
	r.mu.Lock()
	for i, a := range r.answers {
		if a.text == text && a.grammar == grammar {
			copy(r.answers[1:i+1], r.answers[:i])
			r.answers[0] = a
			r.mu.Unlock()
			return a.issues
		}
	}
	r.mu.Unlock()
	issues := r.Checker.Check(text, grammar)
	r.mu.Lock()
	r.answers = slices.Insert(r.answers, 0, answer{text, grammar, issues})
	r.answers = r.answers[:min(len(r.answers), recentAnswers)]
	r.mu.Unlock()
	return issues
}

// Grammar checks grammar along with spelling.
var Grammar = policy.Bool("grammar", false, policy.Label("settings.spelling.grammar"), policy.Description("settings.spelling.grammar.description"))

// Correct replaces misspelled words automatically when typing finishes a word.
var Correct = policy.Bool("correct", false, policy.Label("settings.spelling.correct"), policy.Description("settings.spelling.correct.description"))

// Predict shows system inline text completions as dimmed ghost text ahead of the caret.
var Predict = policy.Bool("predict", false, policy.Label("settings.spelling.predict"), policy.Description("settings.spelling.predict.description"))

// PredictNextWord also suggests completions after a space when inline
// predictions are enabled, rather than only finishing the current word.
var PredictNextWord = policy.Bool("predictNextWord", false, policy.Label("settings.spelling.predictNextWord"), policy.Description("settings.spelling.predictNextWord.description"))

const correctedDuration = 4 * time.Second

// predictPause is how long typing rests before a prediction is asked for:
// quick typing asks nothing.
const predictPause = 350 * time.Millisecond

// Corrected marks a word just replaced automatically so a click can restore
// what was typed.
var Corrected = policy.NewAnnotationKind("spelling.corrected", false)

// TypingWord marks the word the caret is typing, which the analyzer does
// not mark misspelled while it is typed, as the system's text views wait:
// typing past the word ends it, and so does any input that is not typing
// and changes the page or the selection, wherever the caret goes.
var TypingWord = policy.NewAnnotationKind("spelling.typing", false)

// PredictDue marks the caret a prediction is asked at once typing rests
// there until its deadline; more typing moves the deadline on.
var PredictDue = policy.NewAnnotationKind("spelling.predictDue", false)

// issue is a spelling fact's data.
type issue struct{}

type correctedWord struct {
	Original, Replaced string
}

// Policy is the spelling policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	misspelled := policy.Facts[issue](p, "misspelled", policy.Styles(profile.GroupMisspelled))
	questioned := policy.Facts[issue](p, "grammar", policy.Styles(profile.GroupGrammar))
	grammar := policy.Get(p, Grammar)
	correctOn := policy.Get(p, Correct)
	predictOn := policy.Get(p, Predict)
	predictNext := policy.Get(p, PredictNextWord)
	c := checker
	if c == nil {
		p.Unlisted()
	}
	// Sentences are checked one by one, so a keystroke checks its own
	// sentence again and every other's marks come from the store: the
	// system's checker takes time in proportion to the text it reads.
	p.Analyze(policy.Sentences, analyze(c, grammar, misspelled, questioned), []policy.FactID{misspelled.Kind(), questioned.Kind()},
		policy.Where(func(u *policy.Unit) bool { return c != nil && !u.Spec.Literal && u.Spec.Marks != nil }),
		policy.Config(struct{ Checker, Grammar bool }{c != nil, grammar}))

	p.Annotation(Corrected, validCorrected)
	p.Decorate(Corrected, profile.GroupCorrected)
	p.Annotation(TypingWord, validTypingWord)
	p.Annotation(PredictDue, validPoint)

	if c != nil && correctOn {
		p.Rewrite("autocorrect", policy.PhaseCharacters).
			Trigger(policy.Trigger{Kinds: []doc.RecordKind{doc.RecordTextInserted}, Classes: []doc.OriginClass{doc.OriginTyping}}).
			Extent(policy.ChangedRanges).
			Spares(vocab.RoleSyntax).
			Produces(doc.RecordTextInserted, doc.RecordTextDeleted).
			Do(autocorrect(c))
	}
	if c != nil {
		p.Observe("typingWord", policy.Trigger{Kinds: []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted}, Classes: []doc.OriginClass{doc.OriginTyping}}, observeTypingWord)
		// A timer or a suggestion arriving is not the writer doing
		// something else, and changes nothing a record would note.
		p.Observe("stopTyping", policy.Trigger{Classes: []doc.OriginClass{doc.OriginPaste, doc.OriginCommand, doc.OriginPointer, doc.OriginAgent, doc.OriginHistory}}, stopTyping)
	}
	if c != nil && predictOn {
		p.Observe("predict", policy.Trigger{Kinds: []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted}, Classes: []doc.OriginClass{doc.OriginTyping}}, observePredict(predictNext))
		policy.On(p, "predict-due", policy.Intent[vocab.Timer](), policy.Slot.Inside(), askPrediction(predictNext)).Caret()
	}

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
	policy.On(p, "revert-correction", policy.Intent[profile.RevertCorrection](), policy.Slot.Inside(), revertCorrection).Any()
	policy.On(p, "set-suggestion", policy.Intent[profile.SetSuggestion](), policy.Slot.Inside(), setSuggestion).Any()
	policy.On(p, "accept-suggestion", policy.Intent[vocab.Leave](), policy.Suggested.Inside(), acceptSuggestion).Caret()
	policy.On(p, "dismiss-suggestion", policy.Intent[vocab.Dismiss](), policy.Suggested.Inside(), dismissSuggestion).Caret()
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

func isWordDelimiter(r rune) bool {
	return r == ' ' || r == '\t' || strings.ContainsRune(".,;:!?)]}\"”’—–-/", r)
}

func slotInline(r doc.Reader, ref doc.SlotRef) (*doc.Inline, bool) {
	n, ok := r.Node(ref.Node)
	if !ok || ref.Slot >= n.NumSlots() || !n.Slot(ref.Slot).IsInline() {
		return nil, false
	}
	return n.Slot(ref.Slot).Content(), true
}

func validCorrected(r doc.Reader, a policy.Annotation) bool {
	data, ok := a.Data.(correctedWord)
	if !ok {
		return false
	}
	sp := a.Anchor.Span()
	in, ok := slotInline(r, sp.Slot)
	if !ok || sp.From < 0 || sp.To > in.Len() {
		return false
	}
	return in.FlatRange(sp.From, sp.To) == data.Replaced
}

func validPoint(r doc.Reader, a policy.Annotation) bool {
	p := a.Anchor.From.Pos
	in, ok := slotInline(r, p.Ref())
	return ok && a.Anchor.IsPoint() && p.Offset >= 0 && p.Offset <= in.Len()
}

func validTypingWord(r doc.Reader, a policy.Annotation) bool {
	sp := a.Anchor.Span()
	in, ok := slotInline(r, sp.Slot)
	return ok && sp.From >= 0 && sp.From < sp.To && sp.To <= in.Len()
}

func autocorrect(c Checker) policy.RewriteFunc {
	return func(ctx *policy.Ctx, site policy.Site) policy.Result {
		if site.From >= site.To {
			return policy.Swallow
		}
		spec, ok := vocab.SlotSpec(ctx.Doc(), site.Slot)
		if !ok || spec.Literal || spec.Marks == nil {
			return policy.Swallow
		}
		in, ok := ctx.Inline(site.Slot)
		if !ok || site.To > in.Len() {
			return policy.Swallow
		}
		flat := in.Flat()
		lastRune, lastSize := utf8.DecodeLastRuneInString(flat[site.From:site.To])
		if lastSize == 0 || !isWordDelimiter(lastRune) {
			return policy.Swallow
		}
		delimStart := site.To - lastSize
		if delimStart <= 0 {
			return policy.Swallow
		}
		prevRune, prevSize := utf8.DecodeLastRuneInString(flat[:delimStart])
		if prevSize == 0 || vocab.ClassOf(prevRune)&vocab.ClassWord == 0 {
			return policy.Swallow
		}
		w, ok := vocab.WordAt(in, delimStart-prevSize)
		if !ok || !w.Word || w.To != delimStart || w.From >= w.To {
			return policy.Swallow
		}
		word := flat[w.From:w.To]
		if strings.ContainsRune(word, doc.NodeByte) {
			return policy.Swallow
		}
		ignoring, _ := ctx.Mode(ID).(ignored)
		if ignoring.has(word) {
			return policy.Swallow
		}
		lineFrom, lineTo := vocab.LineAround(flat, w.From, w.To)
		tags := profile.TagsIn(in, lineFrom, lineTo)
		if slices.ContainsFunc(tags, func(t profile.Tag) bool { return t.From < w.To && w.From < t.To }) {
			return policy.Swallow
		}
		lineText := strings.ReplaceAll(flat[lineFrom:lineTo], string(doc.NodeByte), " ")
		corr := c.Correction(lineText, w.From-lineFrom, w.To-lineFrom)
		if corr == "" || corr == word || strings.ContainsAny(corr, "\n"+string(doc.NodeByte)) {
			return policy.Swallow
		}
		var marks *doc.MarkSet
		if it, _, ok := in.ItemAt(w.From); ok {
			marks = it.Marks
		}
		if marks.Has(profile.CodeMark) {
			return policy.Swallow
		}
		until := ctx.Now().Add(correctedDuration)
		res := policy.Do(vocab.Ops(doc.EditText{At: site.Slot, From: w.From, To: w.To, Items: []doc.Item{doc.TextItem(corr, marks)}}))
		res.SeparateUndo = true
		res.DropKinds = []policy.AnnotationKind{TypingWord}
		res.Annotate = []policy.Annotate{{
			Kind:     Corrected,
			Anchor:   policy.Between(doc.At(site.Slot, w.From), doc.At(site.Slot, w.From+len(corr))),
			Data:     correctedWord{Original: word, Replaced: corr},
			Lifetime: policy.Until(until),
		}}
		res.Deadlines = []time.Time{until}
		return res
	}
}

func observeTypingWord(ctx *policy.Ctx, _ []doc.Record) policy.Result {
	drop := policy.Result{DropKinds: []policy.AnnotationKind{TypingWord}}
	sel := ctx.Selection()
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		return drop
	}
	pos := sel.Head.Pos
	spec, ok := vocab.SlotSpec(ctx.Doc(), pos.Ref())
	if !ok || spec.Literal || spec.Marks == nil {
		return drop
	}
	in, ok := ctx.Inline(pos.Ref())
	if !ok || pos.Offset <= 0 || pos.Offset > in.Len() {
		return drop
	}
	flat := in.Flat()
	r, sz := utf8.DecodeLastRuneInString(flat[:pos.Offset])
	if sz == 0 || vocab.ClassOf(r)&vocab.ClassWord == 0 {
		return drop
	}
	w, ok := vocab.WordAt(in, pos.Offset-sz)
	if !ok || !w.Word || w.From >= w.To || w.To < pos.Offset {
		return drop
	}
	lineFrom, lineTo := vocab.LineAround(flat, w.From, w.To)
	tags := profile.TagsIn(in, lineFrom, lineTo)
	if slices.ContainsFunc(tags, func(t profile.Tag) bool { return t.From < w.To && w.From < t.To }) {
		return drop
	}
	return policy.Result{
		DropKinds: []policy.AnnotationKind{TypingWord},
		Annotate: []policy.Annotate{{
			Kind:     TypingWord,
			Anchor:   policy.Between(doc.At(pos.Ref(), w.From), doc.At(pos.Ref(), w.To)),
			Lifetime: policy.Lifetime{Kind: policy.CaretLeaves},
		}},
	}
}

// stopTyping ends the typed word, and the prediction typing would have
// asked for: the writer did something else.
func stopTyping(ctx *policy.Ctx, _ []doc.Record) policy.Result {
	if len(ctx.Annotations().Of(TypingWord)) == 0 && len(ctx.Annotations().Of(PredictDue)) == 0 {
		return policy.Swallow
	}
	return policy.Result{DropKinds: []policy.AnnotationKind{TypingWord, PredictDue}}
}

func revertCorrection(ctx *policy.Ctx, in profile.RevertCorrection, _ policy.At[doc.SlotRef]) policy.Result {
	at := ctx.Annotations().At(Corrected, in.At)
	if len(at) == 0 {
		return policy.Pass
	}
	a := at[0]
	data, ok := a.Data.(correctedWord)
	if !ok || data.Original == "" {
		return policy.Pass
	}
	sp := a.Anchor.Span()
	inSlot, ok := ctx.Inline(sp.Slot)
	if !ok || sp.From < 0 || sp.To > inSlot.Len() || inSlot.FlatRange(sp.From, sp.To) != data.Replaced {
		return policy.Pass
	}
	var marks *doc.MarkSet
	if it, _, ok := inSlot.ItemAt(sp.From); ok {
		marks = it.Marks
	}
	end := sp.From + len(data.Original)
	res := policy.Do(vocab.Ops(doc.EditText{At: sp.Slot, From: sp.From, To: sp.To, Items: []doc.Item{doc.TextItem(data.Original, marks)}}))
	res.Drop = []policy.AnnotationID{a.ID}
	res.Then = func(doc.Reader) doc.Selection { return doc.Collapsed(doc.CaretAt(doc.At(sp.Slot, end))) }
	return res
}

const maxPredictPrefix = 512

func isProseForPrediction(ctx *policy.Ctx, pos doc.Pos) bool {
	spec, ok := vocab.SlotSpec(ctx.Doc(), pos.Ref())
	if !ok || spec.Literal || spec.Marks == nil {
		return false
	}
	in, ok := ctx.Inline(pos.Ref())
	if !ok || pos.Offset <= 0 || pos.Offset > in.Len() {
		return false
	}
	if pend := ctx.Pending(); pend != nil && pend.Marks.Has(profile.CodeMark) {
		return false
	}
	if it, _, ok := in.ItemAt(pos.Offset - 1); ok && it.Marks.Has(profile.CodeMark) {
		return false
	}
	flat := in.Flat()
	lineFrom, lineTo := vocab.LineAround(flat, pos.Offset, pos.Offset)
	if pos.Offset <= lineFrom {
		return false
	}
	if pos.Offset < lineTo {
		nextRune, _ := utf8.DecodeRuneInString(flat[pos.Offset:lineTo])
		if vocab.ClassOf(nextRune)&(vocab.ClassWord|vocab.ClassCJK) != 0 {
			return false
		}
	}
	tags := profile.TagsIn(in, lineFrom, lineTo)
	if slices.ContainsFunc(tags, func(t profile.Tag) bool { return t.From <= pos.Offset && pos.Offset <= t.To }) {
		return false
	}
	if strings.HasSuffix(flat[lineFrom:pos.Offset], "#") {
		return false
	}
	return true
}

// observePredict keeps a suggestion the typing follows, and otherwise
// sets a prediction due where typing that could be completed rests, a
// pause on (askPrediction); typing that could not lets it go.
func observePredict(nextWord bool) policy.ObserverFunc {
	return func(ctx *policy.Ctx, _ []doc.Record) policy.Result {
		drop := policy.Result{DropKinds: []policy.AnnotationKind{PredictDue}}
		if s := ctx.Suggestion(); s != nil {
			// Typing what the suggestion said keeps the rest of it; any
			// other typing lets it go, as the engine does by itself.
			if rest, ok := typedAhead(ctx, *s); ok {
				drop.Suggestion = &rest
				return drop
			}
		}
		pos := ctx.Head().Pos
		if _, ok := predictPrefix(ctx, nextWord); !ok {
			return drop
		}
		due := ctx.Now().Add(predictPause)
		drop.Annotate = []policy.Annotate{{Kind: PredictDue, Anchor: policy.PointAnchor(pos), Lifetime: policy.Until(due)}}
		drop.Deadlines = []time.Time{due}
		return drop
	}
}

// askPrediction asks the host for a prediction once typing has rested
// where one is due, from the text before the caret as it reads then.
func askPrediction(nextWord bool) func(*policy.Ctx, vocab.Timer, policy.At[doc.SlotRef]) policy.Result {
	return func(ctx *policy.Ctx, _ vocab.Timer, _ policy.At[doc.SlotRef]) policy.Result {
		due := ctx.Annotations().At(PredictDue, ctx.Head())
		if len(due) == 0 || ctx.Now().Before(due[0].Lifetime.At) || ctx.Suggestion() != nil {
			return policy.Pass
		}
		prefix, ok := predictPrefix(ctx, nextWord)
		if !ok {
			return policy.Pass
		}
		return policy.Result{
			Drop:    []policy.AnnotationID{due[0].ID},
			Effects: []policy.Effect{{Kind: policy.EffectPredict, Data: profile.PredictAsk{At: ctx.Head().Pos, Text: prefix}}},
		}
	}
}

// predictPrefix is the line's text before a collapsed caret a prediction
// completes: an unfinished word of three letters or more, or with
// nextWord a space after one; none in a composition or where the text is
// not prose.
func predictPrefix(ctx *policy.Ctx, nextWord bool) (string, bool) {
	if ctx.Composition() != nil {
		return "", false
	}
	sel := ctx.Selection()
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		return "", false
	}
	pos := sel.Head.Pos
	if !isProseForPrediction(ctx, pos) {
		return "", false
	}
	in, ok := ctx.Inline(pos.Ref())
	if !ok {
		return "", false
	}
	flat := in.Flat()
	lineFrom, _ := vocab.LineAround(flat, pos.Offset, pos.Offset)
	prevRune, prevSize := utf8.DecodeLastRuneInString(flat[lineFrom:pos.Offset])
	if prevSize == 0 {
		return "", false
	}
	if vocab.ClassOf(prevRune)&vocab.ClassWord != 0 {
		w, wok := vocab.WordAt(in, pos.Offset-prevSize)
		if !wok || !w.Word || w.To != pos.Offset || utf8.RuneCountInString(flat[w.From:w.To]) < 3 {
			return "", false
		}
	} else if !nextWord || vocab.ClassOf(prevRune)&vocab.ClassSpace == 0 {
		return "", false
	}
	start := max(lineFrom, pos.Offset-maxPredictPrefix)
	for start < pos.Offset && !utf8.RuneStart(flat[start]) {
		start++
	}
	prefix := strings.ReplaceAll(flat[start:pos.Offset], string(doc.NodeByte), " ")
	if strings.TrimSpace(prefix) == "" {
		return "", false
	}
	return prefix, true
}

// typedAhead is what is left of s once the caret, collapsed in its slot,
// has typed the start of it: none where what stands between s's place and
// the caret is not how s begins, or nothing of it is left.
func typedAhead(ctx *policy.Ctx, s policy.Suggestion) (policy.Suggestion, bool) {
	sel := ctx.Selection()
	head := sel.Head.Pos
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() || head.Ref() != s.At.Ref() || head.Offset <= s.At.Offset {
		return policy.Suggestion{}, false
	}
	in, ok := ctx.Inline(head.Ref())
	if !ok || head.Offset > in.Len() {
		return policy.Suggestion{}, false
	}
	typed := in.FlatRange(s.At.Offset, head.Offset)
	rest, ok := strings.CutPrefix(s.Text, typed)
	if !ok || rest == "" {
		return policy.Suggestion{}, false
	}
	return policy.Suggestion{At: head, Text: rest}, true
}

func setSuggestion(ctx *policy.Ctx, in profile.SetSuggestion, _ policy.At[doc.SlotRef]) policy.Result {
	if in.Text == "" || strings.ContainsAny(in.Text, "\r\n"+string(doc.NodeByte)) || ctx.Composition() != nil {
		return policy.Result{EndSuggestion: true}
	}
	sel := ctx.Selection()
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() || sel.Head.Pos != in.At || !isProseForPrediction(ctx, in.At) {
		return policy.Result{EndSuggestion: true}
	}
	return policy.Result{Suggestion: &policy.Suggestion{At: in.At, Text: in.Text}}
}

func acceptSuggestion(ctx *policy.Ctx, _ vocab.Leave, _ policy.At[doc.Pos]) policy.Result {
	s := ctx.Suggestion()
	if s == nil || s.Text == "" || ctx.Composition() != nil || ctx.Head().Pos != s.At {
		return policy.Pass
	}
	inSlot, ok := ctx.Inline(s.At.Ref())
	if !ok || s.At.Offset < 0 || s.At.Offset > inSlot.Len() {
		return policy.Pass
	}
	var marks *doc.MarkSet
	if pend := ctx.Pending(); pend != nil {
		marks = pend.Marks
	} else if s.At.Offset > 0 {
		if it, _, ok := inSlot.ItemAt(s.At.Offset - 1); ok {
			marks = it.Marks
		}
	} else if it, _, ok := inSlot.ItemAt(0); ok {
		marks = it.Marks
	}
	end := s.At.Offset + len(s.Text)
	res := policy.Do(vocab.Ops(doc.EditText{At: s.At.Ref(), From: s.At.Offset, To: s.At.Offset, Items: []doc.Item{doc.TextItem(s.Text, marks)}}))
	res.EndSuggestion = true
	res.Then = func(doc.Reader) doc.Selection { return doc.Collapsed(doc.CaretAt(doc.At(s.At.Ref(), end))) }
	return res
}

func dismissSuggestion(ctx *policy.Ctx, _ vocab.Dismiss, _ policy.At[doc.Pos]) policy.Result {
	if ctx.Suggestion() == nil {
		return policy.Pass
	}
	return policy.Result{EndSuggestion: true}
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
				!slices.ContainsFunc(tags, func(t profile.Tag) bool { return t.From < to && from < t.To }) &&
				!slices.ContainsFunc(u.Annotations, func(a policy.Annotation) bool {
					return a.Kind == TypingWord && a.Anchor.From.Offset < to && from < a.Anchor.To.Offset
				}) {
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
