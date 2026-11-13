package policy

import (
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
)

// lineRuns counts how often its analyzer reads a line.
var lineRuns int

// testWord and testLong are testWords' kinds.
var testWord, testLong *FactKind[string]

// testWords marks every word of a line, and every word of more than three
// letters as long besides: one line analyzer of two kinds.
var testWords = Define("words", func(p *P) {
	word := Facts[string](p, "word")
	long := Facts[string](p, "long")
	testWord, testLong = word, long
	p.Analyze(Lines, func(u *Unit, e *Emit) {
		lineRuns++
		text := u.Content.TextRange(u.From, u.To)
		at := u.From
		for _, w := range strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '\n' }) {
			i := strings.Index(text[at-u.From:], w) + at
			e.Fact(word.Span(i, i+len(w), w))
			if len(w) > 3 {
				e.Fact(long.Span(i, i+len(w), w))
			}
			at = i + len(w)
		}
	}, []FactID{word.Kind(), long.Kind()})
})

// A line analyzer's facts for a slot are kept while its content is: asking
// again reads no line and returns the same facts, each kind's share kept
// as well; after an edit only the edited line is read, and the facts are a
// fresh store's.
func TestLineFactsAreKeptPerSlot(t *testing.T) {
	prog, err := Compile(Manifest{Enabled: []*Policy{testCore, testWords}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, ref := testPage(t, "one three\nfour fives\nsix sevens")
	store, view := NewStore(), &View{}
	word, long := testWord.Kind(), testLong.Kind()
	lineRuns = 0
	words := prog.facts(store, snap, view, ref, word)
	longs := prog.facts(store, snap, view, ref, long)
	if lineRuns != 3 || len(words) != 6 || len(longs) != 4 {
		t.Fatalf("first read: %d line runs, %d words, %d long", lineRuns, len(words), len(longs))
	}
	again := prog.facts(store, snap, view, ref, word)
	if lineRuns != 3 || &again[0] != &words[0] {
		t.Fatalf("asking again read %d lines more or made new facts", lineRuns-3)
	}
	tx := snap.Begin(doc.Origin{Class: doc.OriginTyping})
	if err := tx.Apply(doc.EditText{At: ref, From: 4, To: 4, Items: []doc.Item{doc.TextItem("big ", nil)}}); err != nil {
		t.Fatal(err)
	}
	next, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	lineRuns = 0
	for _, k := range []FactID{word, long} {
		got := prog.facts(store, next, view, ref, k)
		fresh := prog.facts(NewStore(), next, view, ref, k)
		if !slices.EqualFunc(got, fresh, func(a, b Fact) bool { return a.Outer == b.Outer && a.Data == b.Data }) {
			t.Fatalf("kind %d: kept %v, fresh %v", k, got, fresh)
		}
	}
	if lineRuns != 1+3+3 {
		t.Fatalf("after editing one line, the kept store read %d lines, want 1", lineRuns-6)
	}
}

// A sweep evicts what no demand touched for keep generations, walking the
// caches once every keep of them: facts outlive their last demand by keep
// to 2·keep generations and are read again after.
func TestSweepEvictsWhatWentUntouched(t *testing.T) {
	prog, err := Compile(Manifest{Enabled: []*Policy{testCore, testWords}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, ref := testPage(t, "one three")
	store, view := NewStore(), &View{}
	word := testWord.Kind()
	const keep = 4
	read := func() int {
		lineRuns = 0
		prog.facts(store, snap, view, ref, word)
		return lineRuns
	}
	if read() != 1 {
		t.Fatal("first read read no line")
	}
	for range keep {
		store.Sweep(keep)
	}
	if read() != 0 {
		t.Fatalf("facts demanded within %d generations were evicted", keep)
	}
	for range 2 * keep {
		store.Sweep(keep)
	}
	if read() != 1 {
		t.Fatalf("facts left untouched for %d generations were kept", 2*keep)
	}
}
