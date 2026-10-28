package projection

import (
	"fmt"
	"slices"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/text/rope"
)

type witnessKind uint8

const (
	witnessUnchecked witnessKind = iota
	witnessAbsent
	witnessRoot
)

type textWitness struct {
	kind witnessKind
	root model.TextRoot
}

func (w textWitness) matches(node *model.Node) bool {
	root, present := node.TextRoot()
	switch w.kind {
	case witnessAbsent:
		return !present
	case witnessRoot:
		return present && root.Same(w.root)
	default:
		return false
	}
}

type Projection struct {
	id            ProjectionID
	version       Version
	text          projectionText
	utf16Len      int
	ascii         bool
	semanticNodes map[model.NodeID]struct{}
	segments      []Segment
	segmentBytes  []model.ByteRange
	witnesses     []textWitness
	dependencies  []Dependency
}

func New(
	id ProjectionID,
	version Version,
	text string,
	segments []Segment,
	dependencies []Dependency,
) (*Projection, error) {
	return fromText(
		id,
		version,
		textFromString(text),
		segments,
		make([]textWitness, len(segments)),
		dependencies,
	)
}

func fromText(
	id ProjectionID,
	version Version,
	text projectionText,
	segments []Segment,
	witnesses []textWitness,
	dependencies []Dependency,
) (*Projection, error) {
	if id == "" {
		return nil, &ProjectionError{Kind: ErrorEmptyID}
	}
	length := text.UTF16Len()
	semanticNodes := make(map[model.NodeID]struct{})
	previousEnd := 0
	segmentBytes := make([]model.ByteRange, len(segments))
	for index, segment := range segments {
		range16 := segment.range16
		if range16.Start < 0 || range16.Start > range16.End {
			return nil, &ProjectionError{
				Kind:  ErrorInvalidRange,
				Start: int(range16.Start),
				End:   int(range16.End),
			}
		}
		if int(range16.End) > length {
			return nil, &ProjectionError{
				Kind:   ErrorOutOfBounds,
				Start:  int(range16.Start),
				End:    int(range16.End),
				Length: length,
			}
		}
		for _, offset := range []model.UTF16Offset{range16.Start, range16.End} {
			if _, ok := text.ByteOffset(int(offset)); !ok {
				return nil, &ProjectionError{Kind: ErrorInvalidBoundary, Offset: int(offset)}
			}
		}
		if int(range16.Start) < previousEnd {
			return nil, &ProjectionError{
				Kind:        ErrorOverlap,
				PreviousEnd: previousEnd,
				NextStart:   int(range16.Start),
			}
		}
		if target, ok := segment.SemanticTarget(); ok {
			if target.Revision() != version.SourceRevision() {
				return nil, &ProjectionError{
					Kind:     ErrorSourceRevision,
					Expected: version.SourceRevision(),
					Actual:   target.Revision(),
				}
			}
			semanticNodes[target.Node()] = struct{}{}
			if segment.kind == SegmentEditable &&
				range16.Len() != target.RangeUTF16().Len() {
				return nil, &ProjectionError{Kind: ErrorInvalidMapping}
			}
		} else if target, ok := segment.AttributeTarget(); ok {
			if target.Key() == "" {
				return nil, &ProjectionError{
					Kind: ErrorSource,
					Err:  &model.DocumentError{Kind: model.DocumentErrorEmptyAttributeKey},
				}
			}
			if target.Revision() != version.SourceRevision() {
				return nil, &ProjectionError{
					Kind:     ErrorSourceRevision,
					Expected: version.SourceRevision(),
					Actual:   target.Revision(),
				}
			}
			semanticNodes[target.Node()] = struct{}{}
		}
		start, _ := text.ByteOffset(int(range16.Start))
		end, _ := text.ByteOffset(int(range16.End))
		segmentBytes[index] = model.NewByteRange(model.ByteOffset(start), model.ByteOffset(end))
		previousEnd = int(range16.End)
	}
	if len(witnesses) != len(segments) {
		return nil, fmt.Errorf("projection: witness count does not match segments")
	}
	deps := append([]Dependency(nil), dependencies...)
	slices.SortFunc(deps, func(a, b Dependency) int {
		if a.Document != b.Document {
			if a.Document {
				return -1
			}
			return 1
		}
		if a.Resource < b.Resource {
			return -1
		}
		if a.Resource > b.Resource {
			return 1
		}
		return 0
	})
	deps = slices.Compact(deps)
	return &Projection{
		id:            id,
		version:       version,
		text:          text,
		utf16Len:      length,
		ascii:         text.isASCII(),
		semanticNodes: semanticNodes,
		segments:      append([]Segment(nil), segments...),
		segmentBytes:  segmentBytes,
		witnesses:     append([]textWitness(nil), witnesses...),
		dependencies:  deps,
	}, nil
}

func (p *Projection) ID() ProjectionID   { return p.id }
func (p *Projection) Version() Version   { return p.version }
func (p *Projection) Text() string       { return p.text.Flat() }
func (p *Projection) SharedText() string { return p.Text() }
func (p *Projection) UTF16Len() model.UTF16Offset {
	return model.UTF16Offset(p.utf16Len)
}

func (p *Projection) Segments() []Segment {
	return append([]Segment(nil), p.segments...)
}

func (p *Projection) Dependencies() []Dependency {
	return append([]Dependency(nil), p.dependencies...)
}

func (p *Projection) TextRange(range16 model.UTF16Range) (string, error) {
	if err := p.validateRange(range16); err != nil {
		return "", err
	}
	start, ok := p.text.ByteOffset(int(range16.Start))
	if !ok {
		return "", &ProjectionError{Kind: ErrorInvalidMapping}
	}
	end, ok := p.text.ByteOffset(int(range16.End))
	if !ok {
		return "", &ProjectionError{Kind: ErrorInvalidMapping}
	}
	return p.text.Range(start, end), nil
}

func (p *Projection) TextRangeCharMetrics(range16 model.UTF16Range) (rune, int, error) {
	text, err := p.TextRange(range16)
	if err != nil {
		return 0, 0, err
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return 0, 0, nil
	}
	return runes[0], len(runes), nil
}

func (p *Projection) OverlappingSegmentIndices(range16 model.UTF16Range) (start, end int, err error) {
	if err = p.validateRange(range16); err != nil {
		return 0, 0, err
	}
	if range16.Empty() {
		return 0, 0, nil
	}
	start = searchSegments(p.segments, func(index int) bool {
		return p.segments[index].range16.End > range16.Start
	})
	end = searchSegments(p.segments, func(index int) bool {
		return p.segments[index].range16.Start >= range16.End
	})
	return start, end, nil
}

func (p *Projection) Target(range16 model.UTF16Range) (Target, error) {
	if err := p.validateRange(range16); err != nil {
		return Target{}, err
	}
	return NewTarget(p.id, p.version, range16), nil
}

func (p *Projection) ValidateTarget(target Target) error {
	if err := p.validateTargetVersion(target); err != nil {
		return err
	}
	return p.validateRange(target.range16)
}

func (p *Projection) CaretAddress(target Target) (Caret, error) {
	if !target.range16.Empty() {
		return Caret{}, &ProjectionError{Kind: ErrorInvalidMapping}
	}
	index, segment, err := p.resolveSegment(target)
	if err != nil {
		return Caret{}, err
	}
	if _, ok := segment.AttributeTarget(); !ok {
		anchor, err := p.SemanticAnchor(target.range16.Start, target.affinity)
		if err != nil {
			return Caret{}, err
		}
		return TextCaret(anchor), nil
	}
	attribute, _ := segment.AttributeTarget()
	first, last := -1, -1
	for candidateIndex, candidate := range p.segments {
		other, ok := candidate.AttributeTarget()
		if !ok || other.Node() != attribute.Node() || other.Key() != attribute.Key() {
			continue
		}
		if first < 0 {
			first = candidateIndex
		}
		last = candidateIndex
	}
	if first < 0 || last < 0 {
		return Caret{}, &ProjectionError{Kind: ErrorNoCorrespondence}
	}
	side := AttributeCaretOpening
	if index == last && index != first {
		side = AttributeCaretClosing
	} else if index != first {
		return Caret{}, &ProjectionError{Kind: ErrorAmbiguousCorrespondence}
	}
	return AttributeCaret(
		attribute.Node(),
		attribute.Key(),
		side,
		int(target.range16.Start-segment.range16.Start),
	), nil
}

func (p *Projection) CollapsedCaretAddress(target Target) (Caret, error) {
	_, segment, err := p.resolveSegment(target)
	if err != nil {
		return Caret{}, err
	}
	offset := target.range16.Start
	onTextSide := target.affinity == model.AffinityLeft &&
		offset == segment.range16.Start ||
		target.affinity == model.AffinityRight && offset == segment.range16.End
	if _, ok := segment.AttributeTarget(); ok && onTextSide {
		anchor, err := p.SemanticAnchor(offset, target.affinity)
		if err != nil {
			return Caret{}, err
		}
		return TextCaret(anchor), nil
	}
	return p.CaretAddress(target)
}

func (p *Projection) ResolveCaret(caret Caret, revision uint64) (Target, error) {
	if caret.Kind == CaretText {
		anchor := model.NewTextAnchor(
			caret.Text.Node(),
			caret.Text.OffsetUTF16(),
			caret.Text.Affinity(),
			revision,
		)
		return p.ProjectAnchor(anchor)
	}
	first, last := -1, -1
	for index, segment := range p.segments {
		target, ok := segment.AttributeTarget()
		if !ok || target.Node() != caret.Node || target.Key() != caret.Key {
			continue
		}
		if first < 0 {
			first = index
		}
		last = index
	}
	index := first
	if caret.Side == AttributeCaretClosing {
		index = last
	}
	if index < 0 {
		return Target{}, &ProjectionError{Kind: ErrorNoCorrespondence}
	}
	segment := p.segments[index]
	position := segment.range16.Start +
		model.UTF16Offset(min(caret.Offset, segment.range16.Len()))
	return p.Target(model.EmptyUTF16Range(position))
}

func (p *Projection) ResolveSelection(selection Selection, revision uint64) (Target, Target, error) {
	anchor, err := p.ResolveCaret(selection.anchor, revision)
	if err != nil {
		return Target{}, Target{}, err
	}
	head, err := p.ResolveCaret(selection.head, revision)
	if err != nil {
		return Target{}, Target{}, err
	}
	return anchor, head, nil
}

func (p *Projection) ResolveTarget(target Target) (input.TextTarget, error) {
	_, segment, err := p.resolveSegment(target)
	if err != nil {
		return input.TextTarget{}, err
	}
	if segment.kind != SegmentEditable {
		return input.TextTarget{}, &ProjectionError{Kind: ErrorNonEditable}
	}
	semantic, ok := segment.SemanticTarget()
	if !ok {
		return input.TextTarget{}, &ProjectionError{Kind: ErrorNoCorrespondence}
	}
	start := semantic.RangeUTF16().Start +
		(target.range16.Start - segment.range16.Start)
	end := semantic.RangeUTF16().Start +
		(target.range16.End - segment.range16.Start)
	return input.NewTextTarget(semantic.Node(), model.NewUTF16Range(start, end), semantic.Revision()), nil
}

func (p *Projection) ResolveEdit(target Target, replacement string) (Input, error) {
	index, segment, err := p.resolveSegment(target)
	if err != nil {
		return Input{}, err
	}
	switch segment.kind {
	case SegmentEditable:
		resolved, err := p.ResolveTarget(target)
		if err != nil {
			return Input{}, err
		}
		return Input{Kind: InputText, Text: input.NewTextInput(resolved, replacement)}, nil
	case SegmentAttribute:
		attribute, _ := segment.AttributeTarget()
		fragment := p.text.Range(int(p.segmentBytes[index].Start), int(p.segmentBytes[index].End))
		start := model.UTF16OffsetToByteClamped(fragment, target.range16.Start-segment.range16.Start)
		end := model.UTF16OffsetToByteClamped(fragment, target.range16.End-segment.range16.Start)
		updated := fragment[:start] + replacement + fragment[end:]
		return Input{
			Kind:      InputAttribute,
			Attribute: input.NewAttributeInput(attribute, updated),
		}, nil
	default:
		return Input{}, &ProjectionError{Kind: ErrorNonEditable}
	}
}

func (p *Projection) TextInput(target Target, replacement string) (input.TextInput, error) {
	resolved, err := p.ResolveTarget(target)
	if err != nil {
		return input.TextInput{}, err
	}
	return input.NewTextInput(resolved, replacement), nil
}

func (p *Projection) InvalidatedBy(changes Changes) bool {
	for _, dependency := range p.dependencies {
		if dependency.Document && len(changes.Document) > 0 {
			return true
		}
		if !dependency.Document {
			for _, resource := range changes.Resources {
				if resource == dependency.Resource {
					return true
				}
			}
		}
	}
	for _, change := range changes.Document {
		if _, ok := p.semanticNodes[change.Node]; ok {
			return true
		}
	}
	for _, edit := range changes.TextEdits {
		if _, ok := p.semanticNodes[edit.Node()]; ok {
			return true
		}
	}
	return false
}

func (p *Projection) ValidateSource(revision uint64, document model.DocumentSnapshot) error {
	return p.validateNodes(revision, func(id model.NodeID) (*model.Node, bool) {
		return document.Node(id)
	})
}

func (p *Projection) ValidateCandidate(revision uint64, document model.DocumentSnapshot) error {
	return p.validateNodes(revision, func(id model.NodeID) (*model.Node, bool) {
		return document.Node(id)
	})
}

func (p *Projection) validateNodes(revision uint64, lookup func(model.NodeID) (*model.Node, bool)) error {
	if revision != p.version.SourceRevision() {
		return &ProjectionError{
			Kind:     ErrorSourceRevision,
			Expected: p.version.SourceRevision(),
			Actual:   revision,
		}
	}
	for index, segment := range p.segments {
		if target, ok := segment.SemanticTarget(); ok {
			node, found := lookup(target.Node())
			if !found {
				return &ProjectionError{
					Kind: ErrorSource,
					Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: target.Node()},
				}
			}
			if p.witnesses[index].matches(node) {
				continue
			}
			range16 := target.RangeUTF16()
			if err := node.ValidateTextRangeUTF16(range16.Start, range16.End); err != nil {
				return &ProjectionError{Kind: ErrorSource, Err: err}
			}
			if segment.kind == SegmentEditable {
				nodeText := node.Text().String()
				textBytes := p.text.Range(int(p.segmentBytes[index].Start), int(p.segmentBytes[index].End))
				byteRange, err := node.TextByteRangeUTF16(range16.Start, range16.End)
				if err != nil {
					return &ProjectionError{Kind: ErrorSource, Err: err}
				}
				if nodeText[byteRange.Start:byteRange.End] != textBytes {
					return &ProjectionError{Kind: ErrorInvalidMapping}
				}
			}
		} else if target, ok := segment.AttributeTarget(); ok {
			if _, found := lookup(target.Node()); !found {
				return &ProjectionError{
					Kind: ErrorSource,
					Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: target.Node()},
				}
			}
		}
	}
	return nil
}

func (p *Projection) resolveSegment(target Target) (int, Segment, error) {
	if err := p.validateTargetVersion(target); err != nil {
		return 0, Segment{}, err
	}
	if err := p.validateRange(target.range16); err != nil {
		return 0, Segment{}, err
	}
	candidates := make([]int, 0, 2)
	for index, segment := range p.segments {
		if segment.range16.Start <= target.range16.Start &&
			target.range16.End <= segment.range16.End {
			candidates = append(candidates, index)
		}
	}
	for _, index := range candidates {
		segment := p.segments[index]
		if !target.range16.Empty() || segment.range16.Empty() ||
			target.affinity == model.AffinityLeft && segment.range16.Start < target.range16.Start ||
			target.affinity == model.AffinityRight && target.range16.End < segment.range16.End {
			return index, segment, nil
		}
	}
	if len(candidates) > 0 {
		return candidates[0], p.segments[candidates[0]], nil
	}
	return 0, Segment{}, &ProjectionError{Kind: ErrorNoCorrespondence}
}

func (p *Projection) validateTargetVersion(target Target) error {
	if target.projection != p.id {
		return &ProjectionError{
			Kind:     ErrorWrongProjection,
			Expected: p.id,
			Actual:   target.projection,
		}
	}
	if target.version != p.version {
		return &ProjectionError{
			Kind:     ErrorStaleTarget,
			Expected: p.version,
			Actual:   target.version,
		}
	}
	return nil
}

func (p *Projection) validateRange(range16 model.UTF16Range) error {
	if range16.Start < 0 || range16.Start > range16.End {
		return &ProjectionError{
			Kind:  ErrorInvalidRange,
			Start: int(range16.Start),
			End:   int(range16.End),
		}
	}
	if int(range16.End) > p.utf16Len {
		return &ProjectionError{
			Kind:   ErrorOutOfBounds,
			Start:  int(range16.Start),
			End:    int(range16.End),
			Length: p.utf16Len,
		}
	}
	if !p.ascii {
		for _, offset := range []model.UTF16Offset{range16.Start, range16.End} {
			if _, ok := p.text.ByteOffset(int(offset)); !ok {
				return &ProjectionError{Kind: ErrorInvalidBoundary, Offset: int(offset)}
			}
		}
	}
	return nil
}

func searchSegments(segments []Segment, predicate func(int) bool) int {
	low, high := 0, len(segments)
	for low < high {
		middle := (low + high) / 2
		if predicate(middle) {
			high = middle
		} else {
			low = middle + 1
		}
	}
	return low
}

// Builder constructs projection text and source-checked correspondence.
type Builder struct {
	id        ProjectionID
	version   Version
	utf16Len  int
	byteLen   int
	parts     []textPart
	segments  []Segment
	witnesses []textWitness
}

func NewBuilder(id ProjectionID, version Version) *Builder {
	return &Builder{id: id, version: version}
}

func (b *Builder) Generated(text string) error {
	part := wholeTextPart(rope.New(text))
	range16, err := b.nextRange(part.range16.Len(), part.bytes.Len())
	if err != nil {
		return err
	}
	if text == "" {
		b.parts = append(b.parts, part)
		return nil
	}
	b.push(part, GeneratedSegment(range16), textWitness{kind: witnessUnchecked})
	return nil
}

func (b *Builder) Attribute(node model.NodeID, key, text string) error {
	part := wholeTextPart(rope.New(text))
	range16, err := b.nextRange(part.range16.Len(), part.bytes.Len())
	if err != nil {
		return err
	}
	b.push(part, AttributeSegment(
		range16,
		input.NewAttributeTarget(node, key, b.version.SourceRevision()),
	), textWitness{kind: witnessUnchecked})
	return nil
}

func (b *Builder) Editable(node *model.Node, range16 model.UTF16Range) error {
	return b.source(node, range16, true)
}

func (b *Builder) Semantic(node *model.Node, range16 model.UTF16Range) error {
	return b.source(node, range16, false)
}

func (b *Builder) Build(dependencies []Dependency) (*Projection, error) {
	return fromText(b.id, b.version, textFromParts(b.parts), b.segments, b.witnesses, dependencies)
}

func (b *Builder) source(node *model.Node, range16 model.UTF16Range, editable bool) error {
	bytes, err := node.TextByteRangeUTF16(range16.Start, range16.End)
	if err != nil {
		return &ProjectionError{Kind: ErrorSource, Err: err}
	}
	root, present := node.TextRoot()
	tree := rope.Tree{}
	witness := textWitness{kind: witnessAbsent}
	if present {
		tree = root.Tree()
		witness = textWitness{kind: witnessRoot, root: root}
	}
	projected, err := b.nextRange(range16.Len(), bytes.Len())
	if err != nil {
		return err
	}
	target := input.NewTextTarget(node.ID(), range16, b.version.SourceRevision())
	segment := EditableSegment(projected, target)
	if !editable {
		segment = SemanticSegment(projected, target)
	}
	b.push(textPart{root: tree, bytes: bytes, range16: range16}, segment, witness)
	return nil
}

func (b *Builder) nextRange(length, bytes int) (model.UTF16Range, error) {
	if length < 0 || bytes < 0 || b.utf16Len > int(^uint(0)>>1)-length ||
		b.byteLen > int(^uint(0)>>1)-bytes {
		return model.UTF16Range{}, &ProjectionError{Kind: ErrorLengthOverflow}
	}
	return model.NewUTF16Range(model.UTF16Offset(b.utf16Len), model.UTF16Offset(b.utf16Len+length)), nil
}

func (b *Builder) push(part textPart, segment Segment, witness textWitness) {
	b.utf16Len += part.range16.Len()
	b.byteLen += part.bytes.Len()
	b.parts = append(b.parts, part)
	b.segments = append(b.segments, segment)
	b.witnesses = append(b.witnesses, witness)
}
