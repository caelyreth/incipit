package rope

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestTreeSummariesAndSlicesMatchString(t *testing.T) {
	t.Parallel()
	texts := []string{
		"",
		"ASCII",
		"a🦀b",
		"line one\nline two\n",
		strings.Repeat("mixed 中文🦀\n", 1024),
	}
	for _, text := range texts {
		tree := New(text)
		units := 0
		for _, r := range text {
			units += utf16.RuneLen(r)
		}
		if tree.ByteLen() != len(text) || tree.RuneLen() != len([]rune(text)) || tree.UTF16Len() != units {
			t.Fatalf("%q summary = %+v", text[:min(len(text), 20)], tree.Summary())
		}
		if tree.String() != text {
			t.Fatalf("materialized text differs")
		}
		for start := 0; start <= len(text); start++ {
			for end := start; end <= len(text); end++ {
				if !utf8Boundary(text, start) || !utf8Boundary(text, end) {
					continue
				}
				if got, want := tree.Slice(start, end), text[start:end]; got != want {
					t.Fatalf("slice %d:%d got %q want %q", start, end, got, want)
				}
			}
		}
	}
}

func TestReplaceAndSnapshotsArePersistent(t *testing.T) {
	t.Parallel()
	base := New("hello world")
	changed := base.Replace(6, 11, "there")
	if base.String() != "hello world" {
		t.Fatalf("base mutated: %q", base.String())
	}
	if changed.String() != "hello there" {
		t.Fatalf("changed = %q", changed.String())
	}
	if base.Same(changed) {
		t.Fatal("new version shared old identity")
	}
	noop := base.Replace(2, 2, "")
	if noop.String() != base.String() {
		t.Fatalf("empty insertion changed text")
	}
}

func TestCheckedUTF16ConversionsRejectSurrogateInteriors(t *testing.T) {
	t.Parallel()
	tree := New("a😀中")
	if got, ok := tree.CheckedByteOfUTF16(0); !ok || got != 0 {
		t.Fatalf("offset 0 = %d,%v", got, ok)
	}
	if got, ok := tree.CheckedByteOfUTF16(1); !ok || got != 1 {
		t.Fatalf("offset 1 = %d,%v", got, ok)
	}
	if _, ok := tree.CheckedByteOfUTF16(2); ok {
		t.Fatal("surrogate interior accepted")
	}
	if got, ok := tree.CheckedByteOfUTF16(3); !ok || got != 5 {
		t.Fatalf("offset 3 = %d,%v", got, ok)
	}
	if got, ok := tree.CheckedUTF16OfByte(5); !ok || got != 3 {
		t.Fatalf("byte 5 = %d,%v", got, ok)
	}
	if _, ok := tree.CheckedUTF16OfByte(3); ok {
		t.Fatal("continuation byte accepted")
	}
}

func TestLinesMatchIndependentStringModel(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"", "one", "one\n", "one\ntwo", "one\ntwo\n", "\n\n"} {
		tree := New(text)
		lines := strings.Split(text, "\n")
		if tree.LineCount() != len(lines) {
			t.Fatalf("%q line count = %d, want %d", text, tree.LineCount(), len(lines))
		}
		for index, want := range lines {
			if got := tree.Line(index); got != want {
				t.Fatalf("%q line %d = %q, want %q", text, index, got, want)
			}
		}
	}
}

func utf8Boundary(text string, at int) bool {
	return at == len(text) || at >= 0 && at < len(text) && (text[at]&0xc0) != 0x80
}
