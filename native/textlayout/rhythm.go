package textlayout

import (
	"math"

	"github.com/caelyreth/neverland/engine/surface"
)

// The page's vertical rhythm is derived from one value, the body leading:
// the room a line of running text leaves beside its text, which the line
// spacing setting moves. Every other room is a fixed share of it, so the
// rhythm stays in proportion from compact to loose.
//
// Within a block, rows are their largest text plus the rhythm's leading.
// Between blocks stands a block step, so a new paragraph reads apart from
// the next row of the same one; every other room between blocks is counted
// in steps. A run of entries or verbatim lines reads as one unit, with no
// step between its lines, and a title stands clear of what comes before it
// by twice what it keeps from what it opens, so it reads as the head of
// what follows.
const (
	// entryLeading packs a run's lines a little closer than prose, so its
	// entries read as a set.
	entryLeading = 0.75
	// verbatimLeading packs verbatim lines closest: they are read as lines.
	verbatimLeading = 0.55
	// blockStep is the room between blocks, as a share of the body
	// leading: with a row's own leading it sets paragraphs further apart
	// than the rows of one, yet well short of an empty line.
	blockStep = 0.75
	// titleSteps is the room above a title, in block steps.
	titleSteps = 2
)

// blockGap is steps block steps of room.
func blockGap(body, steps float32) float32 {
	return float32(math.Round(float64(body * blockStep * steps)))
}

// leading is the room a row of a rhythm leaves beside its text.
func leading(rhythm surface.Rhythm, body float32) float32 {
	switch rhythm {
	case surface.RhythmEntry:
		return body * entryLeading
	case surface.RhythmVerbatim:
		return body * verbatimLeading
	}
	return body
}

// gap is the room above a block of rhythm next, after one of rhythm
// previous: a block step, none between the lines of one run, and more above
// a title.
func gap(previous, next surface.Rhythm, body float32) float32 {
	switch {
	case previous == next && isGroup(next):
		return 0
	case next == surface.RhythmTitle && previous != surface.RhythmTitle:
		return blockGap(body, titleSteps)
	}
	return blockGap(body, 1)
}

func isGroup(rhythm surface.Rhythm) bool {
	return rhythm == surface.RhythmEntry || rhythm == surface.RhythmVerbatim
}
