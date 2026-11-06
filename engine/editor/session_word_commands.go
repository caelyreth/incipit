package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"time"
)

// doWord applies word movement, deletion, or enclosing-unit selection.
func (s *Session) doWord(command wordOp) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(WordsPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	selection := s.presentation.Selection
	snapshot := s.runtime.Snapshot()
	node, ok := snapshot.Document().Node(selection.Node())
	if !ok {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	text := ""
	if node.TextPresent() {
		text = node.Text().String()
	}
	spans := selection.Spans()
	if spans && (command == wordDeletePrevious || command == wordDeleteNext) {
		outcome, err := s.replaceSelection("", kernel.EventOriginSemantic)
		return outcome.Outcome, err
	}
	// Words live within a block: a selection reaching into another plans
	// from its head, keeping its anchor where it is.
	planAnchor := selection.Anchor().OffsetUTF16()
	if spans {
		planAnchor = selection.Head().OffsetUTF16()
	}
	plan, err := PlanWordCommand(
		text,
		WordSelection{Anchor: planAnchor, Head: selection.Head().OffsetUTF16()},
		command,
	)
	if err != nil {
		return 0, err
	}
	// At a block's edge there is no word further that way; the step goes on
	// into the next block as a character step would.
	if beyond, ok := wordStepBeyond(command); ok && !plan.Delete && plan.Head == selection.Head().OffsetUTF16() ||
		ok && plan.Delete && plan.DeleteAt.Empty() {
		outcome, err := s.answer(beyond, time.Time{})
		return outcome.Outcome, err
	}
	if plan.Delete {
		if plan.DeleteAt.Empty() {
			return kernel.OutcomeHandled, nil
		}
		anchor, err := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(selection.Node(), plan.DeleteAt.Start, model.AffinityRight, snapshot.Revision()),
		)
		if err != nil {
			return 0, err
		}
		head, err := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(selection.Node(), plan.DeleteAt.End, model.AffinityLeft, snapshot.Revision()),
		)
		if err != nil {
			return 0, err
		}
		edit, err := FromProjectedRange(
			s.presentation.Projection,
			model.NewUTF16Range(
				min(anchor.RangeUTF16().Start, head.RangeUTF16().Start),
				max(anchor.RangeUTF16().Start, head.RangeUTF16().Start),
			),
			"",
			kernel.EventOriginSemantic,
		)
		if err != nil {
			return 0, err
		}
		result, err := s.applyProjectedEdit(edit)
		return result.Outcome, err
	}
	anchor := model.NewTextAnchor(selection.Node(), plan.Anchor, model.AffinityRight, snapshot.Revision())
	if spans && (command == wordExtendPrevious || command == wordExtendNext) {
		anchor = selection.Anchor()
	}
	from := s.presentation.Caret.RangeUTF16().Start
	defer s.snapAtoms(from, command == wordExtendPrevious || command == wordExtendNext)
	return kernel.OutcomeHandled, s.Select(model.NewTextSelection(
		anchor,
		model.NewTextAnchor(
			selection.Node(),
			plan.Head,
			motionAffinity(selection.Head().OffsetUTF16(), plan.Head),
			snapshot.Revision(),
		),
	))
}

// wordStepBeyond is the character intent that continues a word command
// past a block's edge, answered as that intent is anywhere.
func wordStepBeyond(command wordOp) (Intent, bool) {
	switch command {
	case wordPrevious:
		return Move(ByGrapheme, Backward, false), true
	case wordNext:
		return Move(ByGrapheme, Forward, false), true
	case wordExtendPrevious:
		return Move(ByGrapheme, Backward, true), true
	case wordExtendNext:
		return Move(ByGrapheme, Forward, true), true
	case wordDeletePrevious:
		return Delete(ByGrapheme, Backward), true
	case wordDeleteNext:
		return Delete(ByGrapheme, Forward), true
	default:
		return Intent{}, false
	}
}

// snapAtoms passes an atom whole that motion from `from` came to rest in,
// keeping the selection's anchor when it extends.
func (s *Session) snapAtoms(from model.UTF16Offset, extend bool) {
	to := s.presentation.Caret.RangeUTF16().Start
	snapped := s.SnapMotion(from, to)
	if snapped == to {
		return
	}
	anchor := snapped
	if extend {
		if at, err := s.currentViewAnchor(); err == nil {
			anchor = at
		}
	}
	_ = s.SelectProjected(anchor, snapped)
}
