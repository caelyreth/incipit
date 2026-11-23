package window

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/policies/statistics"
)

// TestFooterSaysPageAndSelectionTotals spells the page's counts, and a
// selection's against them.
func TestFooterSaysPageAndSelectionTotals(t *testing.T) {
	page := statistics.Counts{Words: 1204, Characters: 5210, CJK: 3}
	if got, want := formatStatistics(page, nil), "1,204 words  ·  5,210 characters  ·  3 CJK"; got != want {
		t.Fatalf("page = %q, want %q", got, want)
	}
	selection := statistics.Counts{Words: 1, Characters: 4, CJK: 1}
	if got, want := formatStatistics(page, &selection),
		"1 word of 1,204 words  ·  4 characters of 5,210 characters  ·  1 of 3 CJK"; got != want {
		t.Fatalf("selection = %q, want %q", got, want)
	}
	if got, want := formatStatistics(statistics.Counts{}, &statistics.Counts{}), "0 words of 0 words  ·  0 characters of 0 characters"; got != want {
		t.Fatalf("empty selection = %q, want %q", got, want)
	}
}
