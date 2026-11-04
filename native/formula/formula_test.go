package formula

import "testing"

// A source's problems are told once each, in order, where they are: a
// command it does not know as such, not as the glyph it has none for.
func TestProblemsAreToldOnceWhereTheyAre(t *testing.T) {
	if got := Problems(`x+\frac12`, false); len(got) != 0 || !Sound(`x+\frac12`, false) {
		t.Fatalf("a sound source has %+v", got)
	}
	if Sound("  ", false) {
		t.Fatal("a blank source is sound")
	}
	got := Problems(`\fr`, false)
	if len(got) != 1 || got[0].From != 0 || got[0].To != 3 || got[0].Message != `undefined control sequence: \fr` {
		t.Fatalf(`\fr has %+v`, got)
	}
	got = Problems(`\frac{1}{2`, false)
	if len(got) != 2 || got[0].From > got[1].From || got[1].From != got[1].To {
		t.Fatalf("an unclosed group has %+v", got)
	}
}
