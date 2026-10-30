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
// Within a block of one rhythm, rows are their largest text plus the
// rhythm's leading. Between blocks, a gap sets groups apart: a run of
// entries or verbatim lines reads as one unit, and a title binds to what
// it opens more than to what came before.
const (
	// entryLeading packs a run's lines a little closer than prose, so its
	// entries read as a set.
	entryLeading = 0.75
	// verbatimLeading packs verbatim lines closest: they are read as lines.
	verbatimLeading = 0.55
	// titleGap sets a title apart from what comes before it.
	titleGap = 1.0
	// groupGap sets a run of entries or verbatim lines apart from the text
	// around it.
	groupGap = 0.5
)

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
// previous. Blocks of one rhythm follow on with none; nothing is set apart
// from a title above it, which it opens.
func gap(previous, next surface.Rhythm, body float32) float32 {
	share := float32(0)
	switch {
	case previous == next || previous == surface.RhythmTitle:
	case next == surface.RhythmTitle:
		share = titleGap
	case isGroup(previous) || isGroup(next):
		share = groupGap
	}
	return float32(math.Round(float64(body * share)))
}

func isGroup(rhythm surface.Rhythm) bool {
	return rhythm == surface.RhythmEntry || rhythm == surface.RhythmVerbatim
}

// paragraphRhythm is the rhythm of the block a paragraph stands in: that
// of the text at its start, or of the break before it when it is empty.
func paragraphRhythm(flow *surface.Flow, start int) surface.Rhythm {
	rhythm := surface.RhythmProse
	flow.EachRun(func(run surface.StyleRun) bool {
		rhythm = run.Style.Rhythm
		return int(run.Bytes.End) <= start
	})
	return rhythm
}
