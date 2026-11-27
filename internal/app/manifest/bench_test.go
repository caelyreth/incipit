package manifest

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// benchText exercises marks, pairs, code, scripts and typography.
const benchText = "Some **strong** and *em* text (pairs [nested]) `code` 中文English, done... "

func benchPage(b *testing.B) (*engine.Engine, func(vocab.Intent)) {
	b.Helper()
	prog, err := Compile(nil, false)
	if err != nil {
		b.Fatal(err)
	}
	snap, _ := profile.EmptyPage(1, 5, doc.NewInterner())
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	now := time.Unix(10, 0)
	return e, func(in vocab.Intent) {
		now = now.Add(time.Second)
		if _, err := e.Do(1, vocab.Input{Intent: in, At: now}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTyping types a marked-up paragraph a character at a time.
func BenchmarkTyping(b *testing.B) {
	for b.Loop() {
		_, do := benchPage(b)
		for _, r := range benchText {
			do(vocab.Type{Text: string(r)})
		}
	}
}

// BenchmarkMotion moves the caret across a marked-up paragraph.
func BenchmarkMotion(b *testing.B) {
	_, do := benchPage(b)
	for _, r := range benchText + benchText {
		do(vocab.Type{Text: string(r)})
	}
	dir := vocab.Backward
	for i := 0; b.Loop(); i++ {
		if i%100 == 0 {
			dir = -dir
		}
		do(vocab.Move{Unit: vocab.ByGrapheme, Dir: dir})
	}
}

// BenchmarkOffers reads the dock's and the bar's offers after each
// keystroke of a page of text, as the desk does while writing.
func BenchmarkOffers(b *testing.B) {
	e, do := benchPage(b)
	for range 40 {
		for _, r := range benchText {
			do(vocab.Type{Text: string(r)})
		}
		do(vocab.Break{})
	}
	b.ResetTimer()
	for b.Loop() {
		do(vocab.Type{Text: "a"})
		e.Offers(1, vocab.SurfaceDock)
		e.Offers(1, vocab.SurfaceTools)
	}
}
