package rope

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const chunkBytes = 4096

// Summary counts the text below a rope node.
type Summary struct {
	Bytes int
	Runes int
	Units int
	Lines int
}

func (s Summary) add(other Summary) Summary {
	return Summary{
		Bytes: s.Bytes + other.Bytes,
		Runes: s.Runes + other.Runes,
		Units: s.Units + other.Units,
		Lines: s.Lines + other.Lines,
	}
}

func summarize(text string) Summary {
	return Summary{
		Bytes: len(text),
		Runes: countRunes(text),
		Units: countUnits(text),
		Lines: strings.Count(text, "\n"),
	}
}

func countRunes(text string) int {
	all := text
	for len(text) >= 8 {
		if binary.LittleEndian.Uint64([]byte(text))&0x8080808080808080 != 0 {
			return utf8.RuneCountInString(all)
		}
		text = text[8:]
	}
	for index := range len(text) {
		if text[index] >= utf8.RuneSelf {
			return utf8.RuneCountInString(all)
		}
	}
	return len(all)
}

func countUnits(text string) int {
	if len(text) == countRunes(text) {
		return len(text)
	}
	units := 0
	for _, r := range text {
		units += utf16.RuneLen(r)
	}
	return units
}
