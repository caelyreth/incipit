package vocab

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

func TestGraphemesFollowExtendedUAX29(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"combining mark", "e\u0301", []string{"e\u0301"}},
		{"CR LF", "a\r\nb", []string{"a", "\r\n", "b"}},
		{"emoji ZWJ sequence", "👩‍👩‍👧‍👦", []string{"👩‍👩‍👧‍👦"}},
		{"regional indicator pair", "🇺🇸🇨🇦", []string{"🇺🇸", "🇨🇦"}},
		{"emoji keycap", "1️⃣", []string{"1️⃣"}},
		{"Hangul jamo", "\u1100\u1161\u11a8", []string{"\u1100\u1161\u11a8"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for off := 0; off < len(test.text); {
				next := NextGrapheme(test.text, off)
				got = append(got, test.text[off:next])
				off = next
			}
			if len(got) != len(test.want) {
				t.Fatalf("graphemes = %q, want %q", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("grapheme %d = %q, want %q", i, got[i], test.want[i])
				}
			}
			for i := 1; i < len(got); i++ {
				at := len(test.text)
				for _, g := range got[i:] {
					at -= len(g)
				}
				if prev := PrevGrapheme(test.text, at); prev != at-len(got[i-1]) {
					t.Fatalf("PrevGrapheme(%d) = %d, want %d", at, prev, at-len(got[i-1]))
				}
			}
		})
	}
}

func TestGraphemeClassUsesTheWholeCluster(t *testing.T) {
	t.Parallel()
	if got := ClassOfGrapheme("1️⃣"); got&ClassDigit == 0 || got&ClassSpace != 0 {
		t.Fatalf("keycap class = %b", got)
	}
	if got := ClassOfGrapheme("e\u0301"); got&ClassWord == 0 || got&ClassCJK != 0 {
		t.Fatalf("combining letter class = %b", got)
	}
	text := "a\r\nb"
	if got, _ := GraphemeClassAt(text, 2, true); got != 0 {
		t.Fatalf("before CR LF = %b", got)
	}
	if got, _ := GraphemeClassAt(text, 1, false); got != 0 {
		t.Fatalf("after CR LF = %b", got)
	}
	nodes := "a" + string(doc.NodeByte) + "b"
	if _, ok := GraphemeClassAt(nodes, 1, false); ok {
		t.Fatal("an inline node has a character class")
	}
}

// Grapheme boundaries found from a nearby boundary are those segmenting
// the whole text finds, over text mixing ASCII, CR LF, emoji sequences,
// flags, combining marks, Hangul, CJK and inline node bytes.
func TestGraphemeBoundariesAreFoundLocally(t *testing.T) {
	parts := []string{"a", "b", " ", "\r", "\n", "\r\n", "é", "é", "👍🏽", "👩‍💻", "🇯🇵", "🇺", "한", "가", "中", "文", "!", "\x1a", "́", "‍", "\U0001F1EF"}
	rng := rand.New(rand.NewPCG(3, 5))
	for range 300 {
		var b strings.Builder
		for range 1 + rng.IntN(30) {
			b.WriteString(parts[rng.IntN(len(parts))])
		}
		s := b.String()
		var bounds []int
		iter := graphemes.FromString(s)
		for iter.Next() {
			bounds = append(bounds, iter.Start())
		}
		bounds = append(bounds, len(s))
		for off := 0; off <= len(s); off++ {
			next, prev := len(s), 0
			for _, x := range bounds {
				if x > off {
					next = x
					break
				}
			}
			for _, x := range bounds {
				if x < off {
					prev = x
				}
			}
			if off == 0 && len(s) > 0 {
				next = bounds[1]
			}
			if got := NextGrapheme(s, off); got != next {
				t.Fatalf("%q: next after %d = %d, want %d", s, off, got, next)
			}
			if got := PrevGrapheme(s, off); got != prev {
				t.Fatalf("%q: previous before %d = %d, want %d", s, off, got, prev)
			}
			if off < len(s) {
				_, start, end, _ := GraphemeAt(s, off)
				if start > off || end <= off || !slices.Contains(bounds, start) || !slices.Contains(bounds, end) {
					t.Fatalf("%q: the grapheme at %d is %d-%d", s, off, start, end)
				}
			}
		}
	}
}
