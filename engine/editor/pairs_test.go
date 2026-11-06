package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func planPair(t *testing.T, rule PairRule, text string, input PairInput) PairPlan {
	t.Helper()
	plan, err := PlanPair(rule, text, input, DefaultPairSettings())
	if err != nil {
		t.Fatalf("PlanPair(%s, %q) error: %v", rule, text, err)
	}
	return plan
}

func TestPairMayApplyRejectsImpossibleTextBeforeDocumentScan(t *testing.T) {
	t.Parallel()
	settings := DefaultPairSettings()
	tests := []struct {
		rule  PairRule
		input PairInput
		want  bool
	}{
		{PairOpen, PairInput{Range: model.NewUTF16Range(0, 0), Replacement: "x", Origin: kernel.EventOriginTyping}, false},
		{PairOpen, PairInput{Range: model.NewUTF16Range(0, 0), Replacement: "(", Origin: kernel.EventOriginTyping}, true},
		{PairOvertype, PairInput{Range: model.NewUTF16Range(0, 0), Replacement: "x", Origin: kernel.EventOriginTyping}, false},
		{PairOvertype, PairInput{Range: model.NewUTF16Range(0, 0), Replacement: ")", Origin: kernel.EventOriginTyping}, true},
		{PairCloseQuote, PairInput{Range: model.NewUTF16Range(0, 0), Replacement: "x", Origin: kernel.EventOriginTyping}, false},
		{PairCloseQuote, PairInput{Range: model.NewUTF16Range(0, 0), Replacement: "\"", Origin: kernel.EventOriginTyping}, true},
		{PairMateEdit, PairInput{Range: model.NewUTF16Range(0, 0), Replacement: "x", Origin: kernel.EventOriginTyping}, false},
		{PairMateEdit, PairInput{Range: model.NewUTF16Range(0, 1), Replacement: "x", Origin: kernel.EventOriginTyping}, true},
	}
	for _, test := range tests {
		if got := PairMayApply(test.rule, test.input, settings); got != test.want {
			t.Errorf("%s input=%#v may apply=%v, want %v", test.rule, test.input, got, test.want)
		}
	}
}

func TestPlainTypingRetainsIrrelevantPairCache(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "plain prose", false)
	if pairs := session.Pairs(); len(pairs) != 0 {
		t.Fatalf("initial pairs = %#v", pairs)
	}
	if !session.pairsValid || session.pairsRelevant {
		t.Fatal("plain document did not retain an irrelevant pair cache")
	}
	typeSession(t, session, "x")
	if !session.pairsValid || session.pairsRelevant || len(session.pairs) != 0 {
		t.Fatalf(
			"pair cache after typing valid=%v relevant=%v pairs=%#v",
			session.pairsValid,
			session.pairsRelevant,
			session.pairs,
		)
	}
}

func applyPairPlan(t *testing.T, text string, plan PairPlan) string {
	t.Helper()
	if !plan.Handled {
		return text
	}
	result, _, ok := ApplyCorrections(text, 0, plan.Edits)
	if !ok {
		t.Fatal("pair plan did not apply")
	}
	return result
}

func TestDiscoverPairsHandlesBracketsSmartQuotesAndApostrophes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want []PairSpan
	}{
		{"'hello' don't dogs'", []PairSpan{{0, 6, "'", "'"}}},
		{"(a[中])", []PairSpan{{2, 4, "[", "]"}, {0, 5, "(", ")"}}},
		{"“中文” ‘ok’", []PairSpan{{0, 3, "“", "”"}, {5, 8, "‘", "’"}}},
		{"(\"open\nno close\")", []PairSpan{{0, 16, "(", ")"}}},
	}
	for _, test := range tests {
		got, ok := DiscoverPairs(test.text)
		if !ok || !slices.Equal(got, test.want) {
			t.Errorf("%q pairs=%#v want=%#v ok=%v", test.text, got, test.want, ok)
		}
	}
	if _, ok := DiscoverPairs(string([]byte{0xff})); ok {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestPairOpenWrapsSelectionsAndUsesSmartQuotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		text     string
		expected string
	}{
		{"(", "a🦀中", "(a🦀中)"},
		{"［", "a🦀中", "［a🦀中］"},
		{"\"", "a🦀中", "“a🦀中”"},
		{"'", "a🦀中", "‘a🦀中’"},
	}
	for _, test := range tests {
		textRange := model.NewUTF16Range(0, model.UTF16Offset(model.StringLenUTF16(test.text)))
		plan := planPair(t, PairOpen, test.text, PairInput{
			Range:       textRange,
			Replacement: test.input,
			Origin:      kernel.EventOriginTyping,
		})
		result := applyPairPlan(t, test.text, plan)
		if result != test.expected || plan.Head != 1 {
			t.Errorf("%q wrap %q head=%d, want %q head=1", test.input, result, plan.Head, test.expected)
		}
	}
}

func TestPairOpenDeclinesApostrophesAfterWordsAndNonTypingOrigins(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"a", "中", "e\u0301", "🧭 a"} {
		end := model.UTF16Offset(model.StringLenUTF16(text))
		plan := planPair(t, PairOpen, text, PairInput{
			Range:       model.EmptyUTF16Range(end),
			Replacement: "'",
			Origin:      kernel.EventOriginTyping,
		})
		if plan.Handled {
			t.Errorf("%q apostrophe after word was paired", text)
		}
	}
	for _, origin := range []kernel.EventOrigin{
		kernel.EventOriginPaste,
		kernel.EventOriginIME,
		kernel.EventOriginSemantic,
	} {
		plan := planPair(t, PairOpen, "", PairInput{
			Range:       model.NewUTF16Range(0, 0),
			Replacement: "(",
			Origin:      origin,
		})
		if plan.Handled {
			t.Errorf("origin %s paired", origin)
		}
	}
	plan, err := PlanPair(PairOpen, "", PairInput{
		Range:       model.NewUTF16Range(0, 0),
		Replacement: "\"",
		Origin:      kernel.EventOriginTyping,
	}, PairSettings{})
	if err != nil || plan.Handled {
		t.Fatalf("quotes-disabled plan=%#v err=%v", plan, err)
	}
}

func TestPairOpenDeclinesApostrophesAfterOtherAlphabetic(t *testing.T) {
	t.Parallel()
	for _, mark := range []rune{'\u0345', '\u05b0', '\u093e'} {
		text := string(mark)
		plan := planPair(t, PairOpen, text, PairInput{
			Range:       model.EmptyUTF16Range(model.UTF16Offset(model.StringLenUTF16(text))),
			Replacement: "'",
			Origin:      kernel.EventOriginTyping,
		})
		if plan.Handled {
			t.Errorf("%U apostrophe after Other_Alphabetic was paired", mark)
		}
	}
}

func TestPairOvertypeConsumesMatchingCloserAndSmartQuote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		closeAt  model.UTF16Offset
		repl     string
		expected model.UTF16Offset
	}{
		{"(abc)", 4, ")", 5},
		{"“abc”", 4, "\"", 5},
		{"‘abc’", 4, "'", 5},
	}
	for _, test := range tests {
		plan := planPair(t, PairOvertype, test.text, PairInput{
			Range:       model.EmptyUTF16Range(test.closeAt),
			Replacement: test.repl,
			Origin:      kernel.EventOriginTyping,
		})
		result := applyPairPlan(t, test.text, plan)
		if !plan.Handled || result != test.text || plan.Head != test.expected {
			t.Errorf("%q overtype %q => %q head=%d handled=%v", test.text, test.repl, result, plan.Head, plan.Handled)
		}
	}
}

func TestPairCloseQuoteUsesLineLocalDepthAndApostropheRules(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		text     string
		repl     string
		expected string
	}{
		{"“hello", "\"", "“hello”"},
		{"“old” “new", "\"", "“old” “new”"},
		{"“hello", "\"", "“hello”"},
	} {
		plan := planPair(t, PairCloseQuote, test.text, PairInput{
			Range:       model.EmptyUTF16Range(model.UTF16Offset(model.StringLenUTF16(test.text))),
			Replacement: test.repl,
			Origin:      kernel.EventOriginTyping,
		})
		result := applyPairPlan(t, test.text, plan)
		if result != test.expected {
			t.Errorf("%q close %q => %q, want %q", test.text, test.repl, result, test.expected)
		}
	}
	for _, test := range []struct {
		text string
		repl string
	}{
		{"‘hello", "'"},
		{"can", "'"},
	} {
		plan := planPair(t, PairCloseQuote, test.text, PairInput{
			Range:       model.EmptyUTF16Range(model.UTF16Offset(model.StringLenUTF16(test.text))),
			Replacement: test.repl,
			Origin:      kernel.EventOriginTyping,
		})
		if plan.Handled {
			t.Errorf("%q close %q unexpectedly handled", test.text, test.repl)
		}
	}
}

func TestPairMateEditRespellOneBoundaryAndRemovesOppositeMate(t *testing.T) {
	t.Parallel()
	plan := planPair(t, PairMateEdit, "(abc)", PairInput{
		Range:       model.NewUTF16Range(0, 1),
		Replacement: "[",
		Origin:      kernel.EventOriginTyping,
	})
	result := applyPairPlan(t, "(abc)", plan)
	if !plan.Handled || result != "[abc]" || plan.Head != 1 {
		t.Fatalf("respell => %q head=%d handled=%v", result, plan.Head, plan.Handled)
	}

	plan = planPair(t, PairMateEdit, "(abc)", PairInput{
		Range:       model.NewUTF16Range(0, 2),
		Replacement: "",
		Origin:      kernel.EventOriginSemantic,
	})
	result = applyPairPlan(t, "(abc)", plan)
	if !plan.Handled || result != "bc" || plan.Head != 0 {
		t.Fatalf("delete opener => %q head=%d handled=%v", result, plan.Head, plan.Handled)
	}

	plan = planPair(t, PairMateEdit, "a(b)c", PairInput{
		Range:       model.NewUTF16Range(1, 3),
		Replacement: "x",
		Origin:      kernel.EventOriginTyping,
	})
	result = applyPairPlan(t, "a(b)c", plan)
	if !plan.Handled || result != "axc" || plan.Head != 2 {
		t.Fatalf("replace boundary => %q head=%d handled=%v", result, plan.Head, plan.Handled)
	}
}

func TestPairMateEditDeclinesPasteAndStaleSurrogateRanges(t *testing.T) {
	t.Parallel()
	plan := planPair(t, PairMateEdit, "(abc)", PairInput{
		Range:       model.NewUTF16Range(0, 2),
		Replacement: "",
		Origin:      kernel.EventOriginPaste,
	})
	if plan.Handled {
		t.Fatal("paste mate edit handled")
	}
	if _, err := PlanPair(PairMateEdit, "a🦀)", PairInput{
		Range:       model.NewUTF16Range(2, 3),
		Replacement: "",
		Origin:      kernel.EventOriginSemantic,
	}, DefaultPairSettings()); err == nil {
		t.Fatal("surrogate-splitting range accepted")
	}
}

func TestPairMateEditRemovesMultipleOppositeMates(t *testing.T) {
	t.Parallel()
	text := "([x])"
	plan := planPair(t, PairMateEdit, text, PairInput{
		Range:       model.NewUTF16Range(0, 2),
		Replacement: "",
		Origin:      kernel.EventOriginSemantic,
	})
	result := applyPairPlan(t, text, plan)
	if !plan.Handled || result != "x" || plan.Head != 0 {
		t.Fatalf("multi-mate delete => %q head=%d handled=%v", result, plan.Head, plan.Handled)
	}
}

func TestShortestPairContainingUsesRightSideInclusiveCaret(t *testing.T) {
	t.Parallel()
	pairs, _ := DiscoverPairs("(a[b])")
	if pair, ok := ShortestPairContaining(pairs, 3); !ok || pair.Open != 2 || pair.Close != 4 {
		t.Fatalf("inner pair = %#v ok=%v", pair, ok)
	}
	if pair, ok := ShortestPairContaining(pairs, 5); !ok || pair.Open != 0 {
		t.Fatalf("outer pair = %#v ok=%v", pair, ok)
	}
	if _, ok := ShortestPairContaining(pairs, 6); ok {
		t.Fatal("pair after closing matched")
	}
}

// Mates are edited together across blocks as within one: a deletion that
// cuts a pair at either end of a span takes the mate it leaves, and puts
// the caret where the span was.
func TestSpanDeletionTakesCutMates(t *testing.T) {
	for _, test := range []struct {
		name     string
		text     string
		from, to model.UTF16Offset
		view     string
		texts    []string
	}{
		// An opener at the last block's head goes; its closer stays behind.
		{"opener cut", "first line\none (two)", 10, 19, "first line|", []string{"first line"}},
		// A closer at the first block's tail goes; its opener stays before.
		{"closer cut", "one (two) x\nnext", 6, 14, "one t|xt", []string{"one txt"}},
		// Brackets split over two blocks are no pair; the span takes both,
		// and a whole pair before it stays.
		{"split brackets", "a \"q\" (b\nc) d", 6, 11, "a \"q\" | d", []string{"a \"q\"  d"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newSessionFixture(t, test.text, false)
			if err := session.SelectProjected(test.from, test.to); err != nil {
				t.Fatal(err)
			}
			run(t, session, textBackspace)
			expectPage(t, session, test.view, test.texts...)
			if _, err := session.doHistory(historyUndo); err != nil {
				t.Fatal(err)
			}
		})
	}
}
