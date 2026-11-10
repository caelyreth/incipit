package vocab

import "testing"

func TestWordSegmentsKeepByteStarts(t *testing.T) {
	const text = "hello 世界"
	at, wordLike := 0, false
	for segment := range WordSegments(text) {
		if segment.Start != at {
			t.Fatalf("segment %q starts at %d, want %d", segment.Text, segment.Start, at)
		}
		at += len(segment.Text)
		wordLike = wordLike || segment.WordLike
	}
	if at != len(text) {
		t.Fatalf("segments cover %d bytes, want %d", at, len(text))
	}
	if !wordLike {
		t.Fatal("no word-like segment")
	}
}
