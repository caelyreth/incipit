package editor

func lineBreakLen(text string, at int) int {
	if at < 0 || at >= len(text) {
		return 0
	}
	switch text[at] {
	case '\n':
		return 1
	case '\r':
		if at+1 < len(text) && text[at+1] == '\n' {
			return 2
		}
		return 1
	default:
		return 0
	}
}

func lineCount(text string) int {
	count := 1
	for index := 0; index < len(text); {
		if width := lineBreakLen(text, index); width > 0 {
			count++
			index += width
			continue
		}
		index++
	}
	return count
}

func lineOfByte(text string, offset int) int {
	offset = max(0, min(offset, len(text)))
	line := 0
	for index := 0; index < offset; {
		if width := lineBreakLen(text, index); width > 0 {
			line++
			index += width
			continue
		}
		index++
	}
	return line
}

func lineStartByte(text string, line int) int {
	if line <= 0 {
		return 0
	}
	current := 0
	for index := 0; index < len(text); {
		if width := lineBreakLen(text, index); width > 0 {
			current++
			index += width
			if current == line {
				return index
			}
			continue
		}
		index++
	}
	return len(text)
}

func lineEndByte(text string, offset int) int {
	offset = max(0, min(offset, len(text)))
	for index := offset; index < len(text); index++ {
		if lineBreakLen(text, index) > 0 {
			return index
		}
	}
	return len(text)
}
