package vocab

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
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
