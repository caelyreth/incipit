package editor

import "strings"

func lineCount(text string) int {
	return strings.Count(text, "\n") + 1
}

func lineOfByte(text string, offset int) int {
	offset = max(0, min(offset, len(text)))
	return strings.Count(text[:offset], "\n")
}

func lineStartByte(text string, line int) int {
	if line <= 0 {
		return 0
	}
	offset := 0
	for index := 0; index < line; index++ {
		next := strings.IndexByte(text[offset:], '\n')
		if next < 0 {
			return len(text)
		}
		offset += next + 1
	}
	return offset
}
