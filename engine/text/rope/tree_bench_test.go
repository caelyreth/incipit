package rope

import (
	"strings"
	"testing"
)

func BenchmarkReplaceMiddleLargeText(b *testing.B) {
	text := strings.Repeat("stable prose 中文 🦀\n", 4096)
	base := New(text)
	middle := len(text) / 2
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = base.Replace(middle, middle, "x")
	}
}

func BenchmarkUTF16LookupLargeText(b *testing.B) {
	text := strings.Repeat("stable prose 中文 🦀\n", 4096)
	tree := New(text)
	offset := tree.UTF16Len() / 2
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, _ = tree.CheckedByteOfUTF16(offset)
	}
}
