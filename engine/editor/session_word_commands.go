package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// ExecuteWord applies word movement, deletion, or enclosing-unit selection.
func (s *Session) ExecuteWord(command WordCommand) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(WordsPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	selection := s.presentation.Selection
	snapshot := s.runtime.Snapshot()
	node, ok := snapshot.Document().Node(selection.Node())
	if !ok || !node.TextPresent() {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	text := node.Text().String()
	plan, err := PlanWordCommand(
		text,
		WordSelection{
			Anchor: selection.Anchor().OffsetUTF16(),
			Head:   selection.Head().OffsetUTF16(),
		},
		command,
	)
	if err != nil {
		return 0, err
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
	return kernel.OutcomeHandled, s.Select(model.NewTextSelection(
		model.NewTextAnchor(selection.Node(), plan.Anchor, model.AffinityRight, snapshot.Revision()),
		model.NewTextAnchor(
			selection.Node(),
			plan.Head,
			motionAffinity(selection.Head().OffsetUTF16(), plan.Head),
			snapshot.Revision(),
		),
	))
}
