package settings

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
)

func TestPhraseSplitsKeyReferencesOutOfText(t *testing.T) {
	t.Parallel()
	got := Phrase("Press {navigation.tab} to leave, {a.b}{c.d} done")
	want := []Fragment{
		{Text: "Press "}, {Command: "navigation.tab"}, {Text: " to leave, "},
		{Command: "a.b"}, {Command: "c.d"}, {Text: " done"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("phrase = %#v", got)
	}
	if got := Phrase("no keys {unclosed"); !slices.Equal(got, []Fragment{{Text: "no keys {unclosed"}}) {
		t.Fatalf("plain = %#v", got)
	}
	if got := References("{x.y} and {z.w}"); !slices.Equal(got, []kernel.CommandID{"x.y", "z.w"}) {
		t.Fatalf("references = %v", got)
	}
	spelled := Spell("Press {navigation.tab} now", func(kernel.CommandID) string { return "⌘J" })
	if spelled != "Press ⌘J now" {
		t.Fatalf("spell = %q", spelled)
	}
}
