package main

import "testing"

// TestCasesKeepWhatTheLanguageCounts leaves out the plural cases a
// language has no category for, exact cases and nested ones kept.
func TestCasesKeepWhatTheLanguageCounts(t *testing.T) {
	for _, c := range []struct{ lang, in, want string }{
		{"ja", "{n, plural, one {# sheet} other {# sheets}}", "{n, plural, other {# sheets}}"},
		{"ja", "{n, plural, =0 {None} one {# word} other {{total} words}}", "{n, plural, =0 {None} other {{total} words}}"},
		{"en", "{n, plural, one {# sheet} other {# sheets}}", "{n, plural, one {# sheet} other {# sheets}}"},
		{"zh-Hant", "Moved {what} to {where}", "Moved {what} to {where}"},
	} {
		if got := cases(c.in, c.lang); got != c.want {
			t.Errorf("cases(%q, %s) = %q, want %q", c.in, c.lang, got, c.want)
		}
	}
}
