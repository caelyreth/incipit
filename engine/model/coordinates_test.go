package model

import (
	"testing"
	"unicode/utf16"
)

func TestStringConversionsRejectInvalidBytesAndRoundForward(t *testing.T) {
	t.Parallel()
	texts := []string{"", "ASCII", "a😀b", "e\u0301中🦀"}
	for _, text := range texts {
		for byteIndex := 0; byteIndex <= len(text)+1; byteIndex++ {
			got, ok := ByteOffsetToUTF16(text, ByteOffset(byteIndex))
			expected, expectedOK := func() (UTF16Offset, bool) {
				if byteIndex > len(text) || !utf8Boundary(text, byteIndex) {
					return 0, false
				}
				return UTF16Offset(StringLenUTF16(text[:byteIndex])), true
			}()
			if got != expected || ok != expectedOK {
				t.Fatalf("%q byte %d: got (%d,%v), want (%d,%v)", text, byteIndex, got, ok, expected, expectedOK)
			}
			if ok {
				roundTrip, roundOK := UTF16OffsetToByte(text, got)
				if !roundOK || roundTrip != ByteOffset(byteIndex) {
					t.Fatalf("%q byte %d round trip: (%d,%v)", text, byteIndex, roundTrip, roundOK)
				}
			}
		}
	}
	if _, ok := UTF16OffsetToByte("a😀b", 2); ok {
		t.Fatal("surrogate interior accepted")
	}
	if got := UTF16OffsetToByteClamped("a😀b", 2); got != 5 {
		t.Fatalf("clamped surrogate internal = %d, want 5", got)
	}
	if got := UTF16OffsetToByteClamped("a😀b", 100); got != 6 {
		t.Fatalf("clamped beyond end = %d, want 6", got)
	}
}

func TestNativeRangeAlignmentEnclosesScalars(t *testing.T) {
	t.Parallel()
	aligned, bytes, ok := AlignUTF16RangeForNativeQuery("a😀中", NewUTF16Range(2, 2))
	if !ok || aligned != NewUTF16Range(1, 3) || bytes != NewByteRange(1, 5) {
		t.Fatalf("aligned=%+v bytes=%+v ok=%v", aligned, bytes, ok)
	}
	if _, _, ok := AlignUTF16RangeForNativeQuery("a😀中", NewUTF16Range(3, 1)); ok {
		t.Fatal("reversed query accepted")
	}
	if _, _, ok := AlignUTF16RangeForNativeQuery("a😀中", NewUTF16Range(5, 9)); ok {
		t.Fatal("beyond-start query accepted")
	}
}

func TestStringRangesMatchScalarBoundaryOracle(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"", "ASCII", "a🦀e\u0301中"} {
		boundaries := scalarBoundaries(text)
		length := boundaries[len(boundaries)-1].unit
		for start := 0; start <= length+1; start++ {
			strictByte, strictOK := UTF16OffsetToByte(text, UTF16Offset(start))
			clampedByte := UTF16OffsetToByteClamped(text, UTF16Offset(start))
			expectedByte, expectedOK := findBoundaryAtOrAfter(boundaries, start)
			if strictOK != expectedOK || strictOK && strictByte != expectedByte {
				t.Fatalf("%q offset %d strict got (%d,%v), want (%d,%v)", text, start, strictByte, strictOK, expectedByte, expectedOK)
			}
			if !strictOK {
				expectedByte, _ = findBoundaryAtOrAfter(boundaries, start)
				if clampedByte != expectedByte {
					t.Fatalf("%q offset %d clamped got %d, want %d", text, start, clampedByte, expectedByte)
				}
			}
		}
	}
}

func utf8Boundary(text string, at int) bool {
	return at == len(text) || at >= 0 && at < len(text) && (text[at]&0xc0) != 0x80
}

type scalarBoundary struct {
	unit int
	byte int
}

func scalarBoundaries(text string) []scalarBoundary {
	boundaries := []scalarBoundary{{0, 0}}
	units := 0
	for at, r := range text {
		units += utf16.RuneLen(r)
		boundaries = append(boundaries, scalarBoundary{units, at + len(string(r))})
	}
	return boundaries
}

func findBoundaryAtOrAfter(boundaries []scalarBoundary, unit int) (ByteOffset, bool) {
	for _, boundary := range boundaries {
		if boundary.unit == unit {
			return ByteOffset(boundary.byte), true
		}
		if boundary.unit > unit {
			return ByteOffset(boundary.byte), false
		}
	}
	return ByteOffset(boundaries[len(boundaries)-1].byte), false
}
