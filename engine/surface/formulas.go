package surface

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

// Formula is TeX a page sets as math: an inline formula, from its opening
// dollar to its closing one, or a formula block, from its opening fence to
// its closing one. A view sets it in its place, as one object of its
// lines, and edits its source on a card of its own while it is open, as
// the caret in its source opens it.
type Formula struct {
	Node model.NodeID
	// Range is the formula's notation and source in the projected text.
	Range model.UTF16Range
	// Text is where its source stands within Range.
	Text model.UTF16Range
	// Source is its TeX.
	Source string
	// Display sets it as a formula of its own, centered on its lines,
	// rather than within a line of text.
	Display bool
	// Open edits its source on its card, where the caret is.
	Open bool
	// Card is how far its card has opened: 0 none, 1 all of it.
	Card float32
	// Stale sets Source as what the formula last set soundly, not what it
	// now reads, which cannot be set as it is.
	Stale bool
}

func (f Formula) Run() model.UTF16Range { return f.Range }

func (f Formula) node() model.NodeID { return f.Node }

func (f Formula) validate(_ *Flow, block Block) error {
	if f.Text.Start < f.Range.Start || f.Text.End > f.Range.End || f.Text.Start > f.Text.End {
		return fmt.Errorf("a formula's source must stand within it")
	}
	if f.Display && f.Range != block.Range {
		return fmt.Errorf("a formula block must be its block")
	}
	return nil
}

// shifted moves a formula whole past an edit beside it, and stretches it
// with an edit within its source, as typing it does; any other takes it
// away.
func (f Formula) shifted(e rangeEdit) (Object, bool) {
	if e.original.Start >= f.Text.Start && e.original.End <= f.Text.End {
		f.Range.End += e.delta()
		f.Text.End += e.delta()
		return f, true
	}
	moved, ok := e.whole(f.Range)
	if !ok {
		return f, false
	}
	f.Text = model.NewUTF16Range(f.Text.Start+moved.Start-f.Range.Start, f.Text.End+moved.Start-f.Range.Start)
	f.Range = moved
	return f, true
}
