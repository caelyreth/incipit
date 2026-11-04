package surface

import (
	"fmt"
	"slices"
	"sort"

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

// WithFormulas is the flow with the formulas its text sets, in order, none
// within another.
func (f *Flow) WithFormulas(formulas []Formula) (*Flow, error) {
	previous := model.UTF16Offset(-1)
	for _, formula := range formulas {
		if formula.Range.Empty() || formula.Range.Start < previous || formula.Range.End > f.UTF16Len() {
			return nil, fmt.Errorf("formulas must be in order, apart")
		}
		if formula.Display && (formula.Range.Start > 0 && !f.lineStart(formula.Range.Start)) {
			return nil, fmt.Errorf("a formula block must start a line")
		}
		previous = formula.Range.End
	}
	next := *f
	next.formulas = slices.Clone(formulas)
	return &next, nil
}

// Formulas are the formulas the flow's text sets, in order.
func (f *Flow) Formulas() []Formula { return slices.Clone(f.formulas) }

// FormulasIn are the formulas within from to to, in order.
func (f *Flow) FormulasIn(from, to model.UTF16Offset) []Formula {
	index := sort.Search(len(f.formulas), func(index int) bool {
		return f.formulas[index].Range.End > from
	})
	var within []Formula
	for ; index < len(f.formulas) && f.formulas[index].Range.Start < to; index++ {
		within = append(within, f.formulas[index])
	}
	return within
}
