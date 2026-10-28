package surface

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

func TestSparseMetricsMatchCanonicalUnicodeConversions(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("a中🦀e\u0301", 400)
	flow, err := NewFlow(
		projection.NewVersion(0, 0),
		text,
		[]StyleRun{{Bytes: model.NewByteRange(0, model.ByteOffset(len(text))), Style: TextStyle{}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	units := 0
	for byteIndex, r := range text {
		got, ok := flow.ByteOfUTF16(model.UTF16Offset(units))
		if !ok || got != model.ByteOffset(byteIndex) {
			t.Fatalf("unit %d byte = %d,%v want %d", units, got, ok, byteIndex)
		}
		reverse, ok := flow.UTF16OfByte(model.ByteOffset(byteIndex))
		if !ok || reverse != model.UTF16Offset(units) {
			t.Fatalf("byte %d unit = %d,%v want %d", byteIndex, reverse, ok, units)
		}
		for continuation := byteIndex + 1; continuation < byteIndex+len(string(r)); continuation++ {
			if _, ok := flow.UTF16OfByte(model.ByteOffset(continuation)); ok {
				t.Fatalf("continuation byte %d accepted", continuation)
			}
		}
		if model.RuneUTF16Len(r) == 2 {
			if _, ok := flow.ByteOfUTF16(model.UTF16Offset(units + 1)); ok {
				t.Fatalf("surrogate interior %d accepted", units+1)
			}
		}
		units += model.RuneUTF16Len(r)
	}
	if _, ok := flow.ByteOfUTF16(model.UTF16Offset(units + 1)); ok {
		t.Fatal("beyond-end UTF-16 accepted")
	}
	if _, ok := flow.UTF16OfByte(model.ByteOffset(len(text) + 1)); ok {
		t.Fatal("beyond-end byte accepted")
	}
}
