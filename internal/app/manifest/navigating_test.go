package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// Matching Pair goes between a pair's two inner ends: from inside to its
// end, and from its end back to its start.
func TestMatchPairGoesBetweenAPairsEnds(t *testing.T) {
	p := open(t)
	p.typeText("a (bc")
	p.do(left)
	p.do(vocab.MatchPair{})
	p.typeText("1")
	p.do(vocab.MatchPair{})
	p.typeText("2")
	p.want("page(paragraph(a (2bc1)))")
}

// A mark is a pair of signs all the same.
func TestMatchPairGoesBetweenAMarksEnds(t *testing.T) {
	p := open(t)
	p.typeText("a **bold")
	p.do(vocab.MatchPair{})
	p.typeText("x")
	p.want("page(paragraph(a <xbold>))")
}

// Outside any pair, Matching Pair does nothing.
func TestMatchPairOutsideAPairStays(t *testing.T) {
	p := open(t)
	p.typeText("ab")
	p.do(vocab.MatchPair{})
	p.typeText("c")
	p.want("page(paragraph(abc))")
}

// Previous and Next Heading go to the start of the nearest heading that
// way, inside one its own start first, and stay where none is.
func TestJumpHeadingGoesToTheNearestHeadingEitherWay(t *testing.T) {
	p := open(t)
	p.typeText("# A")
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(vocab.Break{})
	p.typeText("## C")
	p.do(vocab.Break{})
	p.typeText("d")
	up := profile.JumpHeading{Dir: vocab.Backward}
	down := profile.JumpHeading{Dir: vocab.Forward}

	p.do(up)
	p.typeText("1")
	p.do(up)
	p.typeText("2")
	p.do(up)
	p.do(up)
	p.typeText("3")
	p.do(up)
	p.typeText("4")
	p.do(down)
	p.typeText("5")
	p.do(down)
	p.typeText("6")
	p.want("page(heading(43A) paragraph(b) heading(5621C) paragraph(d))")
}
