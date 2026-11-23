// © 2016 and later: Unicode, Inc. and others.
// Copyright (C) 2006-2016, International Business Machines Corporation and others.
// ICU CjkBreakEngine character set; see LICENSES/Unicode-3.0.txt.

// Package wordseg finds word boundaries: Unicode UAX #29 rules with ICU
// tailoring, and ICU's Chinese and Japanese dictionary engine.
package wordseg

func dictionaryCharacter(r rune) bool {
	return wordRuleClass(r)&ruleDictionaryCJK != 0
}
