package editor

import (
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
// intents and at most one primary semantic text intent, by its zero-width,
// marker-only, mixed-selection, and literal-marker branches.
func ResolveProjectedEdit(
	p *projection.Projection,
	edit ProjectedEdit,
	activeMark *model.MarkID,
	revision uint64,
) (ResolvedProjectedEdit, error) {
	return resolveProjectedEdit(p, nil, false, edit, activeMark, revision)
}

// ResolveProjectedEditWithDocument additionally performs Neverland's active
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
		// What an edit of notation leaves is placed in the notation's own
		// block, however near another block's text it lies.
		if semantic.Node() != resolved.Attribute.Target().Node() {
			if target, ok := nodeTextTarget(p, resolved.Attribute.Target().Node(), head.RangeUTF16().Start, revision); ok {
				primary = input.NewTextInput(target, "")
			}
		}
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
	target input.AttributeTarget
	// notation is the segment's text, and selected the part of it, in
	// UTF-16 units, the edit covers, while one segment is all it covers.
	notation         string
	selected         model.UTF16Range
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
		var notation bool
		attributes, notation, err = collectNotation(attributes, p, index, from, to)
		if err != nil {
			return ResolvedProjectedEdit{}, err
		}
		if notation {
			selectedAttributeSegments++
		}
	}

	markerNotation := selectedAttributeSegments == 1 &&
		replacementUnits != 0 &&
		len(attributes) == 1 &&
		notationAccepts(attributes[0].target.Key(), edit.Replacement)
	attributeInputs := make([]input.AttributeInput, 0, len(attributes))
	for index, selection := range attributes {
		replacement := ""
		if markerNotation && index == 0 {
			replacement = takeReplacement()
		}
		text := selection.remaining(replacement)
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
			// A pure resolver has no session selection: the primary is unused
			// here, and is the zero value.
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

// collectNotation adds the part of segment index from..to covers to the
// notation an edit covers, when the segment is notation, and reports
// whether it was.
func collectNotation(
	attributes []attributeSelection,
	p *projection.Projection,
	index int,
	from, to model.UTF16Offset,
) ([]attributeSelection, bool, error) {
	segment := p.Segments()[index]
	target, ok := segment.AttributeTarget()
	if !ok {
		return attributes, false, nil
	}
	segmentRange := segment.ProjectedRange()
	notation, err := p.TextRange(segmentRange)
	if err != nil {
		return attributes, false, err
	}
	selectedText, err := p.TextRange(model.NewUTF16Range(from, to))
	if err != nil {
		return attributes, false, err
	}
	if notation == "" {
		return attributes, false, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	selectedUnits := utf8.RuneCountInString(selectedText)
	for candidate := range attributes {
		if attributes[candidate].target.Node() == target.Node() &&
			attributes[candidate].target.Key() == target.Key() {
			attributes[candidate].selectedUnits += selectedUnits
			attributes[candidate].selectedSegments++
			attributes[candidate].firstSegment = min(attributes[candidate].firstSegment, index)
			attributes[candidate].lastSegment = max(attributes[candidate].lastSegment, index)
			return attributes, true, nil
		}
	}
	return append(attributes, attributeSelection{
		target:           target,
		notation:         notation,
		selected:         model.NewUTF16Range(from-segmentRange.Start, to-segmentRange.Start),
		currentUnits:     utf8.RuneCountInString(notation),
		selectedUnits:    selectedUnits,
		selectedSegments: 1,
		firstSegment:     index,
		lastSegment:      index,
	}), true, nil
}

// coveredNotation is the notation a projected range covers, any part of
// it, wherever it lies.
func coveredNotation(p *projection.Projection, range16 model.UTF16Range) ([]attributeSelection, error) {
	start, end, err := p.OverlappingSegmentIndices(range16)
	if err != nil {
		return nil, err
	}
	var attributes []attributeSelection
	for index := start; index < end; index++ {
		segmentRange := p.Segments()[index].ProjectedRange()
		from, to := max(range16.Start, segmentRange.Start), min(range16.End, segmentRange.End)
		if from >= to {
			continue
		}
		if attributes, _, err = collectNotation(attributes, p, index, from, to); err != nil {
			return nil, err
		}
	}
	return attributes, nil
}

// remaining is the notation an edit leaves, with replacement where it
// covered: one segment is edited as the text it is, while both of a mark's,
// opener and closer, lose what the edit covers of either.
func (s attributeSelection) remaining(replacement string) string {
	// A block's notation is one unit: an edit covering any of it replaces
	// all of it.
	if !MarkerKeyIsInline(s.target.Key()) {
		if s.selectedUnits == 0 && replacement == "" {
			return s.notation
		}
		return replacement
	}
	if s.selectedSegments > 1 {
		runes := []rune(s.notation)
		return string(runes[:max(0, s.currentUnits-s.selectedUnits)])
	}
	return spliceUTF16(s.notation, s.selected, replacement)
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
	selection := attributes[0]
	at := p.Segments()[selection.firstSegment].ProjectedRange().Start
	target, ok := nodeTextTarget(p, selection.target.Node(), at, revision)
	if !ok {
		return input.TextTarget{}, &projection.ProjectionError{
			Kind: projection.ErrorNoCorrespondence,
		}
	}
	return target, nil
}

// nodeTextTarget is the place in node's own text nearest a projected
// offset: where notation of node's leaves what is typed or kept, however
// near another block's text it lies. Text before the offset wins a tie.
func nodeTextTarget(
	p *projection.Projection,
	node model.NodeID,
	at model.UTF16Offset,
	revision uint64,
) (input.TextTarget, bool) {
	best, distance := model.UTF16Offset(0), model.UTF16Offset(-1)
	for _, segment := range p.Segments() {
		target, ok := segment.SemanticTarget()
		if !ok || target.Node() != node {
			continue
		}
		span := segment.ProjectedRange()
		offset, gap := target.RangeUTF16().Start, span.Start-at
		switch {
		case span.End <= at:
			offset, gap = target.RangeUTF16().End, at-span.End
		case span.Start < at:
			offset, gap = target.RangeUTF16().Start+at-span.Start, 0
		}
		if distance < 0 || gap < distance {
			best, distance = offset, gap
		}
	}
	if distance < 0 {
		return input.TextTarget{}, false
	}
	return input.NewTextTarget(node, model.EmptyUTF16Range(best), revision), true
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
	key := MarkerKeyForMark(active)
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

// notationAccepts reports whether a replacement typed over notation is an
// edit of it: anything is for a block's notation, which is parsed back as
// written, and only markers are for a mark's.
func notationAccepts(key, replacement string) bool {
	if !MarkerKeyIsInline(key) {
		return true
	}
	for _, value := range replacement {
		if !inlineMarkerRune(value) {
			return false
		}
	}
	return true
}

// spliceUTF16 replaces a UTF-16 range of text.
func spliceUTF16(text string, range16 model.UTF16Range, replacement string) string {
	start := model.UTF16OffsetToByteClamped(text, range16.Start)
	end := model.UTF16OffsetToByteClamped(text, range16.End)
	return text[:start] + replacement + text[end:]
}
