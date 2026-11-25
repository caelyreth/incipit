package window

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/manifest"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policies/spelling"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo"
)

// A keystroke at the end of a long paragraph, presented, with the
// system's checker: it reads the sentence typed in, once, and every other
// sentence's marks are kept.
func BenchmarkAKeystrokeInALongParagraphChecksItsSentence(b *testing.B) {
	if !mygo.Spelling.Available() {
		b.Skip("no system checker")
	}
	spelling.Use(systemSpelling{})
	b.Cleanup(func() { spelling.Use(nil) })
	sentence := "The tide went out at dawn and teh boats lay on the sand, waitng for the sea. "
	for _, grammar := range []bool{false, true} {
		b.Run(fmt.Sprintf("words=2500/grammar=%v", grammar), func(b *testing.B) {
			prog, err := manifest.Compile(policy.Values{checkGrammarKey: grammar}, true)
			if err != nil {
				b.Fatal(err)
			}
			snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
			if err != nil {
				b.Fatal(err)
			}
			e := engine.New(snap, prog, engine.Options{})
			e.OpenView(1)
			now := time.Unix(10, 0)
			do := func(in vocab.Intent) {
				now = now.Add(time.Second)
				if _, err := e.Do(1, vocab.Input{Intent: in, At: now}); err != nil {
					b.Fatal(err)
				}
				v, _ := e.View(1)
				e.Present(1, []doc.ID{v.Selection.Head.Pos.Node})
			}
			do(vocab.Type{Text: strings.Repeat(sentence, 2500/len(strings.Fields(sentence)))})
			for i := 0; b.Loop(); i++ {
				do(vocab.Type{Text: string(rune('a' + i%26))})
			}
		})
	}
}
