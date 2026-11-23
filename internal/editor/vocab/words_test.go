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

func TestSentenceSegmentsKeepByteStarts(t *testing.T) {
	const text = "One. Two! 三。"
	at := 0
	var got []string
	for segment := range SentenceSegments(text) {
		if segment.Start != at {
			t.Fatalf("segment %q starts at %d, want %d", segment.Text, segment.Start, at)
		}
		at += len(segment.Text)
		got = append(got, segment.Text)
	}
	if at != len(text) {
		t.Fatalf("segments cover %d bytes, want %d", at, len(text))
	}
	want := []string{"One. ", "Two! ", "三。"}
	if len(got) != len(want) {
		t.Fatalf("sentences = %q, want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("sentence %d = %q, want %q", i, got[i], want[i])
		}
	}
}
