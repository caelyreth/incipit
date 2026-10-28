package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func utf16Offset(text string) model.UTF16Offset {
	return model.UTF16Offset(model.StringLenUTF16(text))
}

func allTypography() TypographySettings {
	return TypographySettings{Symbols: true, Punctuation: true, QuotePlacement: true}
}

func TestTypographyTriggerIsSingleTypedPunctuation(t *testing.T) {
	t.Parallel()
	for _, value := range []string{".", "-", "'", ",", ";", ":", "!", "?"} {
		if !TypographyTrigger(value) {
			t.Errorf("%q did not trigger", value)
		}
	}
	for _, value := range []string{"", "a", "..", "中", "\n"} {
		if TypographyTrigger(value) {
			t.Errorf("%q unexpectedly triggered", value)
		}
	}
}

func TestTypographyConvergesSymbolsAndPunctuation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		expected string
	}{
		{"wait...", "wait…"},
		{"wait--", "wait—"},
		{"wait ,", "wait,"},
		{"wait,,", "wait,"},
		{"wait?", "wait?"},
	}
	for _, test := range tests {
		batch, ok := PlanTypography(test.text, utf16Offset(test.text), allTypography())
		if !ok {
			t.Fatalf("%q plan failed", test.text)
		}
		result, _, ok := ApplyCorrections(test.text, utf16Offset(test.text), batch)
		if !ok || result != test.expected {
			t.Errorf("%q = %q, want %q, ok=%v", test.text, result, test.expected, ok)
		}
	}
}

func TestTypographyConvergesApostropheWhenItIsTyped(t *testing.T) {
	t.Parallel()
	text := "can'"
	batch, ok := PlanTypography(text, utf16Offset(text), allTypography())
	if !ok {
		t.Fatal("plan failed")
	}
	result, head, ok := ApplyCorrections(text, utf16Offset(text), batch)
	if !ok || result != "can’" || head != 4 {
		t.Fatalf("result=%q head=%d ok=%v", result, head, ok)
	}
}

func TestTypographyConvergesRepeatedTypedPunctuation(t *testing.T) {
	t.Parallel()
	text := ",,,"
	head := utf16Offset(text)
	batches, finalHead, ok := ConvergeTypography(text, head, allTypography())
	if !ok || len(batches) == 0 {
		t.Fatalf("batches=%#v head=%d ok=%v", batches, finalHead, ok)
	}
	for _, batch := range batches {
		text, head, ok = ApplyCorrections(text, head, batch)
		if !ok {
			t.Fatal("apply failed")
		}
	}
	if text != "," || head != 1 {
		t.Fatalf("converged to %q head=%d", text, head)
	}
}

func TestTypographyMovesPeriodOutsideClosingQuotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		expected string
	}{
		{"“hello”,", "“hello,”"},
		{"‘hello’.", "‘hello.’"},
		{"hello.”,", "hello.,”"},
	}
	for _, test := range tests {
		batch, ok := PlanTypography(test.text, utf16Offset(test.text), allTypography())
		if !ok {
			t.Fatalf("%q plan failed", test.text)
		}
		result, _, ok := ApplyCorrections(test.text, utf16Offset(test.text), batch)
		if !ok || result != test.expected {
			t.Errorf("%q = %q, want %q", test.text, result, test.expected)
		}
	}
}

func TestTypographyMovesColonOutsideClosingQuotes(t *testing.T) {
	t.Parallel()
	text := "“hello:”"
	head := utf16Offset("“hello:")
	batch, ok := PlanTypography(text, head, allTypography())
	if !ok {
		t.Fatal("plan failed")
	}
	result, _, ok := ApplyCorrections(text, head, batch)
	if !ok || result != "\u201chello\u201d:" {
		t.Fatalf("result=%q ok=%v", result, ok)
	}
}

func TestTypographyDoesNotScanBeforeLineStartForClosingQuote(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"”"} {
		head := utf16Offset(text)
		batch, ok := PlanTypography(text, head, allTypography())
		if !ok {
			t.Fatalf("%q plan failed", text)
		}
		result, _, ok := ApplyCorrections(text, head, batch)
		if !ok || result != text {
			t.Errorf("%q = %q, want unchanged", text, result)
		}
	}
	head := utf16Offset("”.")
	batch, ok := PlanTypography("”./", head, allTypography())
	if !ok {
		t.Fatal("closing quote period plan failed")
	}
	result, _, ok := ApplyCorrections("”./", head, batch)
	if !ok || result != ".”/" {
		t.Fatalf("closing quote period = %q, ok=%v", result, ok)
	}
}

func TestTypographyRespectsIndependentSettings(t *testing.T) {
	t.Parallel()
	for _, settings := range []TypographySettings{{}, {Symbols: true}, {Punctuation: true}} {
		batch, ok := PlanTypography("x...", utf16Offset("x..."), settings)
		if !ok || !batch.Empty() && settings.Symbols == (settings == TypographySettings{}) {
			t.Errorf("settings=%#v batch=%#v ok=%v", settings, batch, ok)
		}
	}
}

func TestTypographyRejectsSurrogateHeadAndInvalidUTF8(t *testing.T) {
	t.Parallel()
	if _, ok := PlanTypography("🦀", 1, allTypography()); ok {
		t.Fatal("surrogate-interior head accepted")
	}
	if _, ok := PlanTypography(string([]byte{0xff}), 0, allTypography()); ok {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestConvergeTypographyCapsAtThreePasses(t *testing.T) {
	t.Parallel()
	batches, _, ok := ConvergeTypography(",,,", utf16Offset(",,,"), allTypography())
	if !ok || len(batches) > 3 {
		t.Fatalf("passes=%d ok=%v", len(batches), ok)
	}
}

func TestTypographyQuoteBoundaryUsesLineLocalParity(t *testing.T) {
	t.Parallel()
	text := "“old”\n\"new\"."
	batch, ok := PlanTypography(text, utf16Offset(text), allTypography())
	if !ok {
		t.Fatal("plan failed")
	}
	result, _, ok := ApplyCorrections(text, utf16Offset(text), batch)
	if !ok || result != "“old”\n\"new.\"" {
		t.Fatalf("result=%q", result)
	}
	if !slices.Equal(batch.Edits(), []Correction{}) && len(batch.Edits()) != 2 {
		t.Fatalf("unexpected edits %#v", batch.Edits())
	}
}
