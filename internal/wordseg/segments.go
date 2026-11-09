// © 2016 and later: Unicode, Inc. and others.
// Copyright (C) 1999-2016, International Business Machines Corporation and others.
// ICU DictionaryCache::populateDictionary and break-engine selection translation; see LICENSES/Unicode-3.0.txt.

package wordseg

import (
	"iter"
	"sort"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/words"
)

// Segment is one word-granularity segment of a text: its text, the UTF-8
// byte offset it starts at, and whether it is word-like (letters, digits,
// or ideographs, rather than spaces or punctuation).
type Segment struct {
	Text     string
	Start    int
	WordLike bool
}

func wordLike(text string) bool {
	for _, r := range text {
		if r < 128 {
			if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				return true
			}
			continue
		}
		i := sort.Search(len(wordLikeRanges), func(i int) bool { return wordLikeRanges[i][1] >= r })
		if i < len(wordLikeRanges) && wordLikeRanges[i][0] <= r {
			return true
		}
	}
	return false
}

// Segments applies Unicode word rules, ICU rule tailoring, and ICU's
// Chinese/Japanese dictionary engine. Unsupported language-specific scripts use
// the Unicode fallback boundaries.
func Segments(text string) iter.Seq[Segment] {
	return func(yield func(Segment) bool) {
		scanner := words.FromString(tailoredWordRules(text))
		scanned := 0
		scanValue := func() string {
			end := scanned + len(scanner.Value())
			part := text[scanned:end]
			scanned = end
			return part
		}
		var engines dictionaryEngines
		pending := ""
		offset := 0
		emit := func(end int, like bool) bool {
			result := Segment{Text: text[offset:end], Start: offset, WordLike: like}
			offset = end
			return yield(result)
		}
		for pending != "" || scanner.Next() {
			part := pending
			if part == "" {
				part = scanValue()
			}
			pending = ""
			start, end := offset, offset+len(part)
			for scanner.Next() {
				next := scanValue()
				if !joinWordRules(part, next) {
					pending = next
					break
				}
				end += len(next)
				part = text[start:end]
			}
			like, keepGoing := ruleWordLike(part), true
			engines.dictionaryBoundaries(part, func(boundary int) {
				if keepGoing {
					keepGoing = emit(start+boundary, like)
				}
			})
			if !keepGoing || !emit(end, like) {
				return
			}
		}
	}
}

// dictionaryEngines holds the break-engine selection state that ICU keeps on one break iterator for one segment() call. rbbi.cpp getLanguageBreakEngine searches the iterator's engine stack from the top before asking the process factory; UnhandledEngine sits at the bottom. Its handleCharacter applies the Script value of each new character to fHandled, and UnicodeSet::applyIntPropertyValue replaces the set, so it claims only the most recent script. The process factory is modeled after it has loaded every engine.
type dictionaryEngines struct {
	cjkOnStack bool
	unhandled  rune // UnhandledEngine fHandled as an unhandledScriptRanges ID; 0 before its first character
}

func unhandledScript(r rune) rune {
	i := sort.Search(len(unhandledScriptRanges), func(i int) bool { return unhandledScriptRanges[i][1] >= r })
	if i < len(unhandledScriptRanges) && unhandledScriptRanges[i][0] <= r {
		return unhandledScriptRanges[i][2]
	}
	return 0
}

func (e *dictionaryEngines) unhandledHandles(r rune) bool {
	return e.unhandled != 0 && unhandledScript(r) == e.unhandled
}

// engineFor returns 5 for CjkBreakEngine or 0 for UnhandledEngine. Only the CJK
// set shares Common characters (U+30FC and U+FF70) with a Script value
// UnhandledEngine can claim, so stack order matters there.
func (e *dictionaryEngines) engineFor(r rune) rune {
	if dictionaryCharacter(r) {
		if e.cjkOnStack {
			return 5
		}
		if !e.unhandledHandles(r) {
			e.cjkOnStack = true
			return 5
		}
		return 0
	}
	if !e.unhandledHandles(r) {
		e.unhandled = unhandledScript(r) // UnhandledEngine::handleCharacter
	}
	return 0
}

func (e *dictionaryEngines) handles(engine, r rune) bool {
	switch engine {
	case 0:
		return e.unhandledHandles(r)
	case 5:
		return dictionaryCharacter(r)
	}
	return false
}

func ruleDictionary(r rune) bool {
	return wordRuleClass(r)&(ruleKanaKanji|ruleHangul) != 0
}

// dictionaryBoundaries ports DictionaryCache::populateDictionary over one rule span. A span starts only at a $dictionary character; DictionaryBreakEngine::findBreaks then extends it over the selected engine's set, and UnhandledEngine consumes its claimed script without breaks. Engine range ends are omitted: the containing rule span supplies only its outer endpoints.
func (e *dictionaryEngines) dictionaryBoundaries(text string, visit func(int)) {
	if utf16Length(text) <= 1 {
		return
	}
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !ruleDictionary(r) {
			i += size
			continue
		}
		engine, start := e.engineFor(r), i
		for i < len(text) {
			r, size := utf8.DecodeRuneInString(text[i:])
			if !e.handles(engine, r) {
				break
			}
			i += size
		}
		if i == start {
			i += size // Every engine handles its start character; never stall the input loop on a data gap.
		}
		if engine == 5 {
			for _, boundary := range cjkBoundaries(text[start:i]) {
				if start+boundary < i {
					visit(start + boundary)
				}
			}
		}
	}
}

func utf16Length(text string) int {
	count := 0
	for _, r := range text {
		count++
		if r > 0xffff {
			count++
		}
	}
	return count
}
