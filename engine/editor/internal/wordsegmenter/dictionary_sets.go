// © 2016 and later: Unicode, Inc. and others.
// Copyright (C) 2006-2016, International Business Machines Corporation and others.
// ICU CjkBreakEngine character set; see LICENSES/Unicode-3.0.txt.

// Package wordsegmenter implements ICU-tailored word segments for the editor.
package wordsegmenter

func dictionaryCharacter(r rune) bool {
	return wordRuleClass(r)&ruleDictionaryCJK != 0
}
