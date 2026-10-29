package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Presentation is one installed projection, semantic selection, and typed
// projected view state. The flow is derived from the projection and policy
// configuration, never from the native renderer.
type Presentation struct {
	Projection *projection.Projection
	Selection  model.TextSelection
	Caret      projection.Target
	ViewAnchor *projection.Target
	Flow       *surface.Flow
}

// NewPresentation constructs the initial presentation for one semantic node.
func NewPresentation(
	runtime *kernel.PolicyRuntime,
	node model.NodeID,
	generation uint64,
) (Presentation, error) {
	if runtime == nil {
		return Presentation{}, &MarkupProjectionError{Kind: projection.ErrorSource}
	}
	projectionValue, err := MarkupProject(runtime.Snapshot(), node, generation, runtime.Configuration())
	if err != nil {
		return Presentation{}, err
	}
	selection := model.NewTextSelection(
		model.NewTextAnchor(node, 0, model.AffinityRight, runtime.Revision()),
		model.NewTextAnchor(node, 0, model.AffinityRight, runtime.Revision()),
	)
	caret, err := projectionValue.ProjectAnchor(selection.Head())
	if err != nil {
		return Presentation{}, wrapProjectionError(err)
	}
	flow, err := projectionFlow(runtime, projectionValue, selection, nil)
	if err != nil {
		return Presentation{}, err
	}
	return Presentation{
		Projection: projectionValue,
		Selection:  selection,
		Caret:      caret,
		Flow:       flow,
	}, nil
}

// Select installs a directed semantic selection and its projected caret.
// The head's affinity chooses its side of any notation at a mark boundary:
// left keeps it with the text before, right with the text after.
func (p *Presentation) Select(
	snapshot kernel.EngineSnapshot,
	selection model.TextSelection,
) error {
	if p == nil || p.Projection == nil {
		return &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	if selection.Revision() != snapshot.Revision() {
		return &model.AnchorError{
			Kind:     model.AnchorErrorStale,
			Captured: selection.Revision(),
			Current:  snapshot.Revision(),
		}
	}
	if err := selection.ValidateDocument(snapshot.Document()); err != nil {
		return err
	}
	projected, err := p.Projection.ProjectAnchor(selection.Head())
	if err != nil {
		return err
	}
	caret, err := caretTarget(p.Projection, projected.RangeUTF16().Start, projected.Affinity())
	if err != nil {
		return err
	}
	p.Selection = selection
	p.Caret = caret
	p.ViewAnchor = nil
	return nil
}

// SelectProjected installs a visual selection while preserving the projected
// anchor and caret identities. It maps only the head back to semantic state.
func (p *Presentation) SelectProjected(
	snapshot kernel.EngineSnapshot,
	anchor model.UTF16Offset,
	head model.UTF16Offset,
) error {
	if p == nil || p.Projection == nil {
		return &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	anchorResult, err := p.Projection.Target(model.EmptyUTF16Range(anchor))
	if err != nil {
		return err
	}
	headResult, err := p.Projection.Target(model.EmptyUTF16Range(head))
	if err != nil {
		return err
	}
	semanticAnchor, err := p.Projection.SemanticAnchor(anchor, model.AffinityRight)
	if err != nil {
		return err
	}
	semanticHead, err := p.Projection.SemanticAnchor(head, model.AffinityRight)
	if err != nil {
		return err
	}
	selection := model.NewTextSelection(semanticAnchor, semanticHead)
	if err := selection.ValidateDocument(snapshot.Document()); err != nil {
		return err
	}
	p.Selection = selection
	p.Caret, err = caretTarget(p.Projection, headResult.RangeUTF16().Start, semanticHead.Affinity())
	if err != nil {
		return err
	}
	viewAnchor := anchorResult.WithAffinity(semanticAnchor.Affinity())
	p.ViewAnchor = &viewAnchor
	return nil
}

// ProjectedSelection returns the directed visual range. Generated marker
// positions remain addressable even when the semantic selection is collapsed.
func (p *Presentation) ProjectedSelection() (model.UTF16Range, error) {
	if p == nil || p.Projection == nil {
		return model.UTF16Range{}, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	head := p.Caret.RangeUTF16().Start
	if p.ViewAnchor != nil {
		anchor := p.ViewAnchor.RangeUTF16().Start
		return model.NewUTF16Range(min(anchor, head), max(anchor, head)), nil
	}
	if p.Selection.IsCollapsed() {
		return model.EmptyUTF16Range(head), nil
	}
	anchor, err := p.Projection.ProjectAnchor(p.Selection.Anchor())
	if err != nil {
		return model.UTF16Range{}, err
	}
	return model.NewUTF16Range(
		min(anchor.RangeUTF16().Start, head),
		max(anchor.RangeUTF16().Start, head),
	), nil
}

// ProjectedState returns the version-independent anchor/head caret identities.
func (p *Presentation) ProjectedState() (projection.Selection, error) {
	if p == nil || p.Projection == nil {
		return projection.Selection{}, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	head, err := p.caretAddress()
	if err != nil {
		return projection.Selection{}, err
	}
	if p.ViewAnchor == nil && p.Selection.IsCollapsed() {
		return projection.CollapsedSelection(head), nil
	}
	anchor := head
	if p.ViewAnchor != nil {
		anchor, err = p.Projection.CaretAddress(*p.ViewAnchor)
	} else {
		target, projectErr := p.Projection.ProjectAnchor(p.Selection.Anchor())
		if projectErr != nil {
			return projection.Selection{}, projectErr
		}
		anchor, err = p.Projection.CaretAddress(target)
	}
	if err != nil {
		return projection.Selection{}, err
	}
	return projection.NewSelection(anchor, head), nil
}

func (p *Presentation) caretAddress() (projection.Caret, error) {
	collapsed := p.Selection.IsCollapsed()
	if p.ViewAnchor != nil {
		collapsed = p.ViewAnchor.RangeUTF16() == p.Caret.RangeUTF16()
	}
	if collapsed {
		return p.Projection.CollapsedCaretAddress(p.Caret)
	}
	return p.Projection.CaretAddress(p.Caret)
}

// Install replaces the presentation from a prepared candidate and drops the
// stale flow. The caller owns publication ordering.
func (p *Presentation) Install(next Presentation) {
	*p = next
}

// Target returns one projected address in the current projection.
func (p *Presentation) Target(r model.UTF16Range) (projection.Target, error) {
	if p == nil || p.Projection == nil {
		return projection.Target{}, &projection.ProjectionError{
			Kind: projection.ErrorNoCorrespondence,
		}
	}
	return p.Projection.Target(r)
}

// CollapsedTarget returns the current visual caret target. This is the target
// used for pointer placement and IME composition.
func (p *Presentation) CollapsedTarget() (projection.Target, error) {
	range16, err := p.ProjectedSelection()
	if err != nil {
		return projection.Target{}, err
	}
	if !range16.Empty() {
		return projection.Target{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	return p.Caret, nil
}

// SurfaceFlow returns the retained flow for the current projection.
func (p *Presentation) SurfaceFlow(
	runtime *kernel.PolicyRuntime,
) (*surface.Flow, error) {
	if p == nil || p.Projection == nil {
		return nil, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	if p.Flow != nil {
		return p.Flow, nil
	}
	flow, err := projectionFlow(runtime, p.Projection, p.Selection, nil)
	if err != nil {
		return nil, err
	}
	p.Flow = flow
	return flow, nil
}

func projectionFlow(
	runtime *kernel.PolicyRuntime,
	projectionValue *projection.Projection,
	selection model.TextSelection,
	decorations []surface.Decoration,
) (*surface.Flow, error) {
	snapshot := runtime.Snapshot()
	node, ok := snapshot.Document().Node(selection.Node())
	if !ok {
		return nil, &projection.ProjectionError{Kind: projection.ErrorSource}
	}
	runs, err := InlineStyleRuns(projectionValue, node, markupEnabled(runtime.Configuration()))
	if err != nil {
		return nil, err
	}
	return surface.NewFlow(projectionValue.Version(), projectionValue.Text(), runs)
}
