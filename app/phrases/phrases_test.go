package phrases

import (
	"regexp"
	"testing"

	"github.com/caelyreth/neverland/locales"
)

var reference = regexp.MustCompile(`\{[A-Za-z0-9.]+\}`)

// TestTextNamesCommandsAsSettingsRead has every phrase the catalog holds
// read whole, a command it names written back in braces with its dots.
func TestTextNamesCommandsAsSettingsRead(t *testing.T) {
	for key, want := range map[string]string{
		"policies.core":                          "Text",
		"policies.core.description":              "Every keystroke is an edit of the page: {core.break} starts a new block, {core.softBreak} breaks the line inside one.",
		"settings.markdown.headings.description": "# at a line's start makes a heading, one level per #.",
		"no.such.key":                            "no.such.key",
	} {
		if got := Text(key); got != want {
			t.Errorf("Text(%q) = %q, want %q", key, got, want)
		}
	}
	for _, m := range []string{"policiesListsDescription", "policiesTablesDescription"} {
		msg, _, ok := locales.Catalog.Message("en", m)
		if !ok || len(reference.FindAllString(msg.String(), -1)) != 0 {
			t.Errorf("%s keeps a dotted reference or is missing", m)
		}
	}
}
