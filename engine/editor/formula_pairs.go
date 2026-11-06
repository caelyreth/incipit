package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// A formula's source pairs its brackets as it is typed: an opener brings
// its closer, a closer typed before its own steps over it, and Backspace
// between an empty pair takes both. Quotes are TeX's own, and a bracket
// after a backslash is one written out, as \{ is: neither pairs.

// formulaScope is the text of node a formula's source is while the caret
// is in it: a formula block's all, an inline formula's what its mark
// covers.
func formulaScope(node *model.Node, active *model.MarkID) (model.UTF16Range, bool) {
	if node == nil {
		return model.UTF16Range{}, false
	}
	if kindOf(node).Formula {
		return model.NewUTF16Range(0, blockLength(node)), true
	}
	if kind, ok := literalMark(node, active); !ok || kind != MathMark {
		return model.UTF16Range{}, false
	}
	for _, mark := range node.Marks() {
		if mark.ID() == *active {
			return mark.Range(), true
		}
	}
	return model.UTF16Range{}, false
}

// planFormulaPair plans rule for input within scope of text, as PlanPair
// does across a paragraph's.
func planFormulaPair(rule PairRule, text string, scope model.UTF16Range, input PairInput, settings PairSettings) (PairPlan, error) {
	declined := PairPlan{Rule: rule}
	if input.Range.Start < scope.Start || input.Range.End > scope.End {
		return declined, nil
	}
	from := model.UTF16OffsetToByteClamped(text, scope.Start)
	to := model.UTF16OffsetToByteClamped(text, scope.End)
	source := text[from:to]
	local := input
	local.Range = model.NewUTF16Range(input.Range.Start-scope.Start, input.Range.End-scope.Start)
	var plan PairPlan
	var err error
	switch rule {
	case PairOpen:
		before := source[:model.UTF16OffsetToByteClamped(source, local.Range.Start)]
		if !strings.Contains("([{", input.Replacement) || len(input.Replacement) != 1 ||
			(len(before)-len(strings.TrimRight(before, "\\")))%2 == 1 {
			return declined, nil
		}
		plan, err = PlanPair(rule, source, local, settings)
	case PairOvertype:
		if !strings.Contains(")]}", input.Replacement) || len(input.Replacement) != 1 {
			return declined, nil
		}
		plan, err = PlanPair(rule, source, local, settings)
	case PairMateEdit:
		plan, err = planEmptyPairDeletion(source, local, settings)
	default:
		return declined, nil
	}
	if err != nil || !plan.Handled {
		return declined, err
	}
	edits := plan.Edits.Edits()
	shifted := make([]Correction, len(edits))
	for index, edit := range edits {
		edit.Range = model.NewUTF16Range(edit.Range.Start+scope.Start, edit.Range.End+scope.Start)
		shifted[index] = edit
	}
	if plan.Edits, err = NewCorrectionBatch(shifted); err != nil {
		return declined, err
	}
	plan.Head += scope.Start
	return plan, nil
}

// planEmptyPairDeletion takes both brackets of an empty pair when one of
// them is deleted; a pair holding source keeps its other side, which the
// source may still need.
func planEmptyPairDeletion(source string, input PairInput, settings PairSettings) (PairPlan, error) {
	plan := PairPlan{Rule: PairMateEdit}
	if !settings.EditMates || input.Replacement != "" || input.Range.Len() != 1 {
		return plan, nil
	}
	pairs, _ := DiscoverPairs(source)
	for _, pair := range pairs {
		if pair.Close != pair.Open+1 || !strings.Contains("([{", pair.Opener) ||
			input.Range.Start != pair.Open && input.Range.Start != pair.Close {
			continue
		}
		batch, err := NewCorrectionBatch([]Correction{{Range: model.NewUTF16Range(pair.Open, pair.Close+1)}})
		if err != nil {
			return plan, err
		}
		plan.Handled, plan.Edits, plan.Head = true, batch, pair.Open
		return plan, nil
	}
	return plan, nil
}
