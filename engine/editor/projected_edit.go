package editor

import (
	"fmt"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// ProjectedCaret and ProjectedSelection are the projection-owned visual
// addresses retained by a projected edit.
type ProjectedCaret = projection.Caret
type ProjectedSelection = projection.Selection

// ProjectedEditInput is the canonical ordered batch consumed by the kernel.
type ProjectedEditInput = kernel.ProjectedEditInput

// ProjectedEdit is one visual replacement resolved against a concrete
// projection version. ProjectedCaret is present for a collapsed edit and
// ProjectedSelection preserves any marker-only visual selection.
type ProjectedEdit struct {
	ProjectionID       projection.ProjectionID
	ProjectionVersion  projection.Version
	UTF16Range         model.UTF16Range
	Replacement        string
	Origin             kernel.EventOrigin
	ProjectedCaret     *ProjectedCaret
	ProjectedSelection *ProjectedSelection
}

// ResolvedProjectedEdit is the canonical batch produced by projected
// decomposition. Attribute inputs precede the primary text input when both
// are present.
type ResolvedProjectedEdit struct {
	ProjectedEditInput ProjectedEditInput
	ActiveMarkID       *model.MarkID
	VisualSelection    *ProjectedSelection
}

// ProjectedEditErrorKind identifies malformed projected-edit values.
type ProjectedEditErrorKind uint8

const (
	ProjectedEditNilProjection ProjectedEditErrorKind = iota + 1
	ProjectedEditInvalidReplacement
	ProjectedEditInvalidCaret
	ProjectedEditInvalidSelection
)

// ProjectedEditError reports a projected-edit value contract failure.
type ProjectedEditError struct {
	Kind ProjectedEditErrorKind
}

func (e *ProjectedEditError) Error() string {
	switch e.Kind {
	case ProjectedEditNilProjection:
		return "projected edit requires a projection"
	case ProjectedEditInvalidReplacement:
		return "projected edit replacement must be valid UTF-8"
	case ProjectedEditInvalidCaret:
		return "projected edit caret does not match its range"
	case ProjectedEditInvalidSelection:
		return "projected edit visual selection does not match its range"
	default:
		return "invalid projected edit"
	}
}

// FromProjectionTarget creates an edit from an already validated projected
// target. Collapsed targets additionally capture their caret identity.
func FromProjectionTarget(
	p *projection.Projection,
	target projection.Target,
	replacement string,
	origin kernel.EventOrigin,
) (ProjectedEdit, error) {
	if p == nil {
		return ProjectedEdit{}, &ProjectedEditError{Kind: ProjectedEditNilProjection}
	}
	if err := p.ValidateTarget(target); err != nil {
		return ProjectedEdit{}, err
	}
	range16 := target.RangeUTF16()
	var caret *projection.Caret
	if range16.Empty() {
		address, err := p.CaretAddress(target)
		if err != nil {
			return ProjectedEdit{}, err
		}
		caret = &address
	}
	return newProjectedEdit(p, range16, replacement, origin, caret, nil)
}

// FromProjectedRange creates an edit from a projected UTF-16 range.
func FromProjectedRange(
	p *projection.Projection,
	range16 model.UTF16Range,
	replacement string,
	origin kernel.EventOrigin,
) (ProjectedEdit, error) {
	if p == nil {
		return ProjectedEdit{}, &ProjectedEditError{Kind: ProjectedEditNilProjection}
	}
	target, err := p.Target(range16)
	if err != nil {
		return ProjectedEdit{}, err
	}
	return FromProjectionTarget(p, target, replacement, origin)
}

// FromSelectionAndRange creates an edit that retains a complete projected
// visual selection. This is how marker-only selections remain addressable.
func FromSelectionAndRange(
	p *projection.Projection,
	range16 model.UTF16Range,
	selection projection.Selection,
	replacement string,
	origin kernel.EventOrigin,
) (ProjectedEdit, error) {
	if p == nil {
		return ProjectedEdit{}, &ProjectedEditError{Kind: ProjectedEditNilProjection}
	}
	if err := p.ValidateTarget(projection.NewTarget(p.ID(), p.Version(), range16)); err != nil {
		return ProjectedEdit{}, err
	}
	var caret *projection.Caret
	if range16.Empty() {
		head := selection.Head()
		caret = &head
	}
	return newProjectedEdit(p, range16, replacement, origin, caret, &selection)
}

func newProjectedEdit(
	p *projection.Projection,
	range16 model.UTF16Range,
	replacement string,
	origin kernel.EventOrigin,
	caret *projection.Caret,
	selection *projection.Selection,
) (ProjectedEdit, error) {
	if !utf8.ValidString(replacement) {
		return ProjectedEdit{}, &ProjectedEditError{Kind: ProjectedEditInvalidReplacement}
	}
	edit := ProjectedEdit{
		ProjectionID:      p.ID(),
		ProjectionVersion: p.Version(),
		UTF16Range:        range16,
		Replacement:       replacement,
		Origin:            origin,
	}
	if caret != nil {
		value := *caret
		edit.ProjectedCaret = &value
	}
	if selection != nil {
		edit.ProjectedSelection = cloneProjectedSelection(*selection)
	}
	return edit, nil
}

// Validate checks the value against the current projection identity, version,
// range, and retained visual address before any decomposition occurs.
func (e ProjectedEdit) Validate(p *projection.Projection) error {
	if p == nil {
		return &ProjectedEditError{Kind: ProjectedEditNilProjection}
	}
	if e.ProjectionID != p.ID() {
		return &projection.ProjectionError{
			Kind:     projection.ErrorWrongProjection,
			Expected: p.ID(),
			Actual:   e.ProjectionID,
		}
	}
	if e.ProjectionVersion != p.Version() {
		return &projection.ProjectionError{
			Kind:     projection.ErrorStaleTarget,
			Expected: p.Version(),
			Actual:   e.ProjectionVersion,
		}
	}
	if !utf8.ValidString(e.Replacement) {
		return &ProjectedEditError{Kind: ProjectedEditInvalidReplacement}
	}
	if _, err := p.Target(e.UTF16Range); err != nil {
		return err
	}
	if !e.UTF16Range.Empty() && e.ProjectedCaret != nil {
		return &ProjectedEditError{Kind: ProjectedEditInvalidCaret}
	}
	if e.ProjectedSelection != nil {
		anchor, head, err := p.ResolveSelection(*e.ProjectedSelection, p.Version().SourceRevision())
		if err != nil {
			return err
		}
		start := min(anchor.RangeUTF16().Start, head.RangeUTF16().Start)
		end := max(anchor.RangeUTF16().Start, head.RangeUTF16().Start)
		if start != e.UTF16Range.Start || end != e.UTF16Range.End {
			return &ProjectedEditError{Kind: ProjectedEditInvalidSelection}
		}
	}
	return nil
}

// ResolveProjectedEdit decomposes a visual replacement into ordered attribute
// intents and at most one primary semantic text intent. It mirrors the Rust
// engine's zero-width, marker-only, mixed-selection, and literal-marker branches.
func ResolveProjectedEdit(
	p *projection.Projection,
	edit ProjectedEdit,
	activeMark *model.MarkID,
	revision uint64,
) (ResolvedProjectedEdit, error) {
	return resolveProjectedEdit(p, nil, false, edit, activeMark, revision)
}

// ResolveProjectedEditWithDocument additionally performs Shoreward's active
// mark inference for a non-marker selection that is wholly covered by one or
// more semantic marks. It selects the smallest covering mark.
func ResolveProjectedEditWithDocument(
	p *projection.Projection,
	document model.DocumentSnapshot,
	edit ProjectedEdit,
	activeMark *model.MarkID,
	revision uint64,
) (ResolvedProjectedEdit, error) {
	return resolveProjectedEdit(p, &document, true, edit, activeMark, revision)
}

func resolveProjectedEdit(
	p *projection.Projection,
	document *model.DocumentSnapshot,
	haveDocument bool,
	edit ProjectedEdit,
	activeMark *model.MarkID,
	revision uint64,
) (ResolvedProjectedEdit, error) {
	if err := edit.Validate(p); err != nil {
		return ResolvedProjectedEdit{}, err
	}
	if edit.UTF16Range.Empty() {
		return resolveCollapsedProjectedEdit(p, edit, activeMark, revision)
	}
	return resolveRangeProjectedEdit(
		p,
		document,
		haveDocument,
		edit,
		activeMark,
		revision,
	)
}

func cloneMarkID(id *model.MarkID) *model.MarkID {
	if id == nil {
		return nil
	}
	value := *id
	return &value
}

func cloneProjectedSelection(selection projection.Selection) *projection.Selection {
	value := projection.NewSelection(selection.Anchor(), selection.Head())
	return &value
}

func resolveCollapsedProjectedEdit(
	p *projection.Projection,
	edit ProjectedEdit,
	activeMark *model.MarkID,
	revision uint64,
) (ResolvedProjectedEdit, error) {
	if edit.ProjectedCaret == nil {
		return ResolvedProjectedEdit{}, &projection.ProjectionError{
			Kind: projection.ErrorNoCorrespondence,
		}
	}
	caret := *edit.ProjectedCaret
	if caret.Kind == projection.CaretText {
		anchor := model.NewTextAnchor(
			caret.Text.Node(),
			caret.Text.OffsetUTF16(),
			caret.Text.Affinity(),
			revision,
		)
		textTarget := input.NewTextTarget(
			anchor.Node(),
			model.EmptyUTF16Range(anchor.OffsetUTF16()),
			revision,
		)
		textInput := input.NewTextInput(textTarget, edit.Replacement)
		return ResolvedProjectedEdit{
			ProjectedEditInput: kernel.NewProjectedEditInput(
				textInput,
				nil,
				true,
				true,
				nil,
			),
			ActiveMarkID:    cloneMarkID(activeMark),
			VisualSelection: cloneProjectedSelectionPtr(edit.ProjectedSelection),
		}, nil
	}
	head, err := p.ResolveCaret(caret, revision)
	if err != nil {
		return ResolvedProjectedEdit{}, err
	}
	resolved, err := p.ResolveEdit(head, edit.Replacement)
	if err != nil {
		return ResolvedProjectedEdit{}, err
	}
	var projected kernel.ProjectedEditInput
	switch resolved.Kind {
	case projection.InputText:
		projected = kernel.NewProjectedEditInput(
			resolved.Text,
			nil,
			true,
			true,
			nil,
		)
	case projection.InputAttribute:
		semantic, err := p.SemanticAnchor(head.RangeUTF16().Start, head.Affinity())
		if err != nil {
			return ResolvedProjectedEdit{}, err
		}
		primary := input.NewTextInput(
			input.NewTextTarget(
				semantic.Node(),
				model.EmptyUTF16Range(semantic.OffsetUTF16()),
				revision,
			),
			"",
		)
		attributeCaret := caret
		projected = kernel.NewProjectedEditInput(
			primary,
			[]input.AttributeInput{resolved.Attribute},
			false,
			false,
			&attributeCaret,
		)
	default:
		return ResolvedProjectedEdit{}, &projection.ProjectionError{
			Kind: projection.ErrorNoCorrespondence,
		}
	}
	return ResolvedProjectedEdit{
		ProjectedEditInput: projected,
		ActiveMarkID:       cloneMarkID(activeMark),
		VisualSelection:    cloneProjectedSelectionPtr(edit.ProjectedSelection),
	}, nil
}

type attributeSelection struct {
	target           input.AttributeTarget
	marker           rune
	currentUnits     int
	selectedUnits    int
	selectedSegments int
	firstSegment     int
	lastSegment      int
}

func resolveRangeProjectedEdit(
	p *projection.Projection,
	document *model.DocumentSnapshot,
	haveDocument bool,
	edit ProjectedEdit,
	activeMark *model.MarkID,
	revision uint64,
) (ResolvedProjectedEdit, error) {
	start, end, err := p.OverlappingSegmentIndices(edit.UTF16Range)
	if err != nil {
		return ResolvedProjectedEdit{}, err
	}
	segments := p.Segments()
	projectedRange := edit.UTF16Range
	replacementUnits := utf8.RuneCountInString(edit.Replacement)
	replacementIsMarker := edit.Replacement != "" && allMarkerRunes(edit.Replacement)
	replacement := edit.Replacement
	replacementTaken := false
	takeReplacement := func() string {
		if replacementTaken {
			return ""
		}
		replacementTaken = true
		return replacement
	}
	attributes := make([]attributeSelection, 0, end-start)
	var textNode *model.NodeID
	var selectedText *model.UTF16Range
	selectedAttributeSegments := 0

	for index := start; index < end; index++ {
		segment := segments[index]
		segmentRange := segment.ProjectedRange()
		from := max(projectedRange.Start, segmentRange.Start)
		to := min(projectedRange.End, segmentRange.End)
		if target, ok := segment.SemanticTarget(); ok {
			if textNode != nil && *textNode != target.Node() {
				return ResolvedProjectedEdit{}, &projection.ProjectionError{
					Kind: projection.ErrorInvalidMapping,
				}
			}
			base := target.RangeUTF16().Start
			mapped := model.NewUTF16Range(
				base+from-segmentRange.Start,
				base+to-segmentRange.Start,
			)
			if selectedText == nil {
				value := mapped
				selectedText = &value
			} else {
				selectedText = &model.UTF16Range{
					Start: min(selectedText.Start, mapped.Start),
					End:   max(selectedText.End, mapped.End),
				}
			}
			node := target.Node()
			textNode = &node
			continue
		}
		target, ok := segment.AttributeTarget()
		if !ok {
			continue
		}
		marker, hasMarker, currentUnits, err := textRangeCharMetrics(p, segmentRange)
		if err != nil {
			return ResolvedProjectedEdit{}, err
		}
		_, _, selectedUnits, err := textRangeCharMetrics(
			p,
			model.NewUTF16Range(from, to),
		)
		if err != nil {
			return ResolvedProjectedEdit{}, err
		}
		if !hasMarker {
			return ResolvedProjectedEdit{}, &projection.ProjectionError{
				Kind: projection.ErrorNoCorrespondence,
			}
		}
		selectedAttributeSegments++
		found := -1
		for candidate := range attributes {
			if attributes[candidate].target.Node() == target.Node() &&
				attributes[candidate].target.Key() == target.Key() {
				found = candidate
				break
			}
		}
		if found >= 0 {
			attributes[found].selectedUnits += selectedUnits
			attributes[found].selectedSegments++
			attributes[found].firstSegment = min(attributes[found].firstSegment, index)
			attributes[found].lastSegment = max(attributes[found].lastSegment, index)
			continue
		}
		attributes = append(attributes, attributeSelection{
			target:           target,
			marker:           marker,
			currentUnits:     currentUnits,
			selectedUnits:    selectedUnits,
			selectedSegments: 1,
			firstSegment:     index,
			lastSegment:      index,
		})
	}

	markerNotation := selectedAttributeSegments == 1 &&
		replacementUnits != 0 &&
		len(attributes) == 1 &&
		replacementIsMarker
	attributeInputs := make([]input.AttributeInput, 0, len(attributes))
	for index, selection := range attributes {
		text := ""
		if markerNotation && index == 0 {
			text = takeReplacement()
		} else {
			finalUnits := max(0, selection.currentUnits-selection.selectedUnits)
			text = repeatRune(selection.marker, finalUnits)
		}
		attributeInputs = append(
			attributeInputs,
			input.NewAttributeInput(selection.target, text),
		)
	}

	literalMarkerInsertion := selectedText == nil &&
		replacementUnits != 0 &&
		!markerNotation &&
		len(attributes) != 0
	var insertionTarget *input.TextTarget
	if literalMarkerInsertion || textNode == nil && len(attributes) != 0 {
		target, err := markerInsertionTarget(p, attributes, revision)
		if err != nil {
			return ResolvedProjectedEdit{}, err
		}
		insertionTarget = &target
	}

	var text *input.TextInput
	if textNode != nil && selectedText != nil {
		value := input.NewTextInput(
			input.NewTextTarget(*textNode, *selectedText, revision),
			takeReplacement(),
		)
		text = &value
	} else if literalMarkerInsertion {
		if insertionTarget == nil {
			return ResolvedProjectedEdit{}, &projection.ProjectionError{
				Kind: projection.ErrorNoCorrespondence,
			}
		}
		value := input.NewTextInput(*insertionTarget, takeReplacement())
		text = &value
	}

	appliesText := text != nil
	var primary input.TextInput
	if text != nil {
		primary = *text
	} else {
		target := insertionTarget
		if target == nil {
			// The Rust resolver uses the session's current node as an unused
			// placeholder here. A pure resolver has no session selection, so
			// it exposes the same unused primary as the zero value.
			primary = input.TextInput{}
		} else {
			primary = input.NewTextInput(*target, "")
		}
	}
	interactive := len(attributes) == 0 && appliesText
	var caret *projection.Caret
	if !appliesText {
		caret = projectedCaret(
			p,
			segments,
			attributes,
			markerNotation,
			insertionTarget,
			replacementUnits,
			revision,
		)
	}
	projected := kernel.NewProjectedEditInput(
		primary,
		attributeInputs,
		appliesText,
		interactive,
		caret,
	)

	resolvedActive := cloneMarkID(activeMark)
	if resolvedActive == nil && haveDocument && len(attributes) == 0 &&
		selectedText != nil && textNode != nil && document != nil {
		if node, ok := document.Node(*textNode); ok {
			var best *model.InlineMark
			for _, mark := range node.Marks() {
				markRange := mark.Range()
				if markRange.Start <= selectedText.Start &&
					selectedText.End <= markRange.End &&
					(best == nil || markRange.Len() < best.Range().Len()) {
					value := mark
					best = &value
				}
			}
			if best != nil {
				id := best.ID()
				resolvedActive = &id
			}
		}
	}
	if resolvedActive != nil && clearedActiveMark(
		p,
		attributes,
		*resolvedActive,
	) {
		resolvedActive = nil
	}
	return ResolvedProjectedEdit{
		ProjectedEditInput: projected,
		ActiveMarkID:       resolvedActive,
		VisualSelection:    cloneProjectedSelectionPtr(edit.ProjectedSelection),
	}, nil
}

func markerInsertionTarget(
	p *projection.Projection,
	attributes []attributeSelection,
	revision uint64,
) (input.TextTarget, error) {
	if len(attributes) == 0 {
		return input.TextTarget{}, &projection.ProjectionError{
			Kind: projection.ErrorNoCorrespondence,
		}
	}
	segments := p.Segments()
	selection := attributes[0]
	for index := selection.firstSegment - 1; index >= 0; index-- {
		if target, ok := segments[index].SemanticTarget(); ok {
			return input.NewTextTarget(
				target.Node(),
				model.EmptyUTF16Range(target.RangeUTF16().End),
				revision,
			), nil
		}
	}
	for index := selection.lastSegment + 1; index < len(segments); index++ {
		if target, ok := segments[index].SemanticTarget(); ok {
			return input.NewTextTarget(
				target.Node(),
				model.EmptyUTF16Range(target.RangeUTF16().Start),
				revision,
			), nil
		}
	}
	return input.TextTarget{}, &projection.ProjectionError{
		Kind: projection.ErrorNoCorrespondence,
	}
}

func projectedCaret(
	p *projection.Projection,
	segments []projection.Segment,
	attributes []attributeSelection,
	markerNotation bool,
	insertionTarget *input.TextTarget,
	replacementUnits int,
	revision uint64,
) *projection.Caret {
	if len(attributes) == 0 {
		return nil
	}
	selection := attributes[0]
	if markerNotation {
		side := projection.AttributeCaretClosing
		if selection.firstSegment < selection.lastSegment ||
			segments[selection.firstSegment].ProjectedRange().Start == 0 {
			side = projection.AttributeCaretOpening
		}
		caret := projection.AttributeCaret(
			selection.target.Node(),
			selection.target.Key(),
			side,
			replacementUnits,
		)
		return &caret
	}
	if insertionTarget == nil {
		return nil
	}
	caret := projection.TextCaret(model.NewTextAnchor(
		insertionTarget.Node(),
		insertionTarget.RangeUTF16().Start,
		model.AffinityRight,
		revision,
	))
	return &caret
}

func clearedActiveMark(
	p *projection.Projection,
	attributes []attributeSelection,
	active model.MarkID,
) bool {
	key := markerKey(active)
	selected := 0
	for _, selection := range attributes {
		if selection.target.Key() == key {
			selected = selection.selectedSegments
			break
		}
	}
	total := 0
	for _, segment := range p.Segments() {
		if target, ok := segment.AttributeTarget(); ok && target.Key() == key {
			total++
		}
	}
	return total != 0 && selected == total
}

func cloneProjectedSelectionPtr(
	selection *projection.Selection,
) *projection.Selection {
	if selection == nil {
		return nil
	}
	return cloneProjectedSelection(*selection)
}

func allMarkerRunes(text string) bool {
	for _, value := range text {
		if value != '*' && value != '_' {
			return false
		}
	}
	return true
}

func textRangeCharMetrics(
	p *projection.Projection,
	range16 model.UTF16Range,
) (rune, bool, int, error) {
	text, err := p.TextRange(range16)
	if err != nil {
		return 0, false, 0, err
	}
	if text == "" {
		return 0, false, 0, nil
	}
	first, _ := utf8.DecodeRuneInString(text)
	return first, true, utf8.RuneCountInString(text), nil
}

func repeatRune(value rune, count int) string {
	if count <= 0 {
		return ""
	}
	runes := make([]rune, count)
	for index := range runes {
		runes[index] = value
	}
	return string(runes)
}

func markerKey(mark model.MarkID) string {
	return fmt.Sprintf("emphasis/%d", mark.Value())
}
