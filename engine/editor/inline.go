package editor

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

const inlineMarkerPrefix = "emphasis/"

// InlineState retains the mark that owns the caret. It deliberately stores
// identity rather than a derived range so later edits cannot silently retarget
// marker typing to a different mark.
type InlineState struct {
	active    *model.MarkID
	hasActive bool
}

// Active returns the remembered mark identity.
func (s InlineState) Active() (model.MarkID, bool) {
	return derefMark(s.active), s.hasActive
}

// Activate replaces the remembered mark identity.
func (s *InlineState) Activate(id *model.MarkID) {
	if id == nil {
		s.active = nil
		s.hasActive = false
		return
	}
	value := *id
	s.active = &value
	s.hasActive = true
}

// Select infers the active mark from a semantic selection. The narrowest
// strictly containing mark wins; an already-active mark may contain the head
// inclusively so typing at its end can exit it.
func (s *InlineState) Select(node *model.Node, selection model.TextSelection) {
	if node == nil || node.ID() != selection.Node() {
		s.Activate(nil)
		return
	}
	head := selection.Head().OffsetUTF16()
	var best *model.InlineMark
	for _, mark := range node.Marks() {
		value := mark
		contains := value.Range().Start < head && head < value.Range().End
		if s.hasActive && *s.active == value.ID() {
			contains = value.Range().Start <= head && head <= value.Range().End
		}
		if !contains {
			continue
		}
		if best == nil || value.Range().Len() < best.Range().Len() {
			best = &value
		}
	}
	if best == nil {
		s.Activate(nil)
		return
	}
	id := best.ID()
	s.Activate(&id)
}

// SelectProjected infers the active mark when the projected caret lies between
// that mark's opener and closer. Generated notation is intentionally part of
// this test, while semantic selection remains marker-free.
func (s *InlineState) SelectProjected(
	node *model.Node,
	p *projection.Projection,
	head model.UTF16Offset,
) {
	if node == nil || p == nil {
		s.Activate(nil)
		return
	}
	var best *model.InlineMark
	for _, mark := range node.Marks() {
		value := mark
		key := MarkerKeyForMark(value.ID())
		firstEnd := -1
		lastStart := -1
		for _, segment := range p.Segments() {
			target, ok := segment.AttributeTarget()
			if !ok || target.Key() != key {
				continue
			}
			if firstEnd < 0 {
				firstEnd = int(segment.ProjectedRange().End)
			}
			lastStart = int(segment.ProjectedRange().Start)
		}
		if firstEnd < 0 || lastStart < 0 ||
			head < model.UTF16Offset(firstEnd) || model.UTF16Offset(lastStart) < head {
			continue
		}
		if best == nil || value.Range().Len() < best.Range().Len() {
			best = &value
		}
	}
	if best == nil {
		s.Activate(nil)
		return
	}
	id := best.ID()
	s.Activate(&id)
}

// MarkerKeyForMark returns the generated attribute identity for one mark.
func MarkerKeyForMark(id model.MarkID) string {
	return inlineMarkerPrefix + strconv.FormatUint(uint64(id), 10)
}

// MarkerKeyIsInline reports whether an attribute key belongs to inline
// emphasis rather than a legacy or unrelated attribute.
func MarkerKeyIsInline(key string) bool {
	return strings.HasPrefix(key, inlineMarkerPrefix)
}

// MarkerKeyIdentifies reports whether a key names one concrete inline mark.
func MarkerKeyIdentifies(key string, id model.MarkID) bool {
	return key == MarkerKeyForMark(id)
}

// MarkerActionKind identifies one semantic marker-typing operation.
type MarkerActionKind uint8

const (
	MarkerCreate MarkerActionKind = iota + 1
	MarkerUpgrade
	MarkerExit
	MarkerFormat
)

// MarkerAction is a checked, pure marker-typing plan. It is deliberately
// separate from EngineInvoker: the kernel can execute the document mutation,
// while selection/exit facts remain with the caller.
type MarkerAction struct {
	Handled  bool
	Kind     MarkerActionKind
	Node     model.NodeID
	Range    model.UTF16Range
	Mark     model.MarkID
	HasMark  bool
	Level    uint8
	Head     model.UTF16Offset
	Exited   bool
	NoOp     bool
	Consumed bool
}

// MarkerSettings gate marker typing independently of projection settings.
type MarkerSettings struct {
	Enabled     bool
	AllowStrong bool
}

// PlanMarkerTypes interprets one typed "*" or "_" replacement against the
// active mark. Non-marker text and non-typing origins pass through.
func PlanMarkerTypes(
	node *model.Node,
	input model.UTF16Range,
	text string,
	active *model.MarkID,
	settings MarkerSettings,
	origin kernel.EventOrigin,
) (MarkerAction, error) {
	action := MarkerAction{Node: nodeID(node), Range: input, Head: input.End}
	if node == nil || !settings.Enabled || origin != kernel.EventOriginTyping ||
		(text != "*" && text != "_") {
		return action, nil
	}
	if err := node.ValidateTextRangeUTF16(input.Start, input.End); err != nil {
		return MarkerAction{}, err
	}
	var current *model.InlineMark
	if active != nil {
		for _, mark := range node.Marks() {
			if mark.ID() == *active {
				value := mark
				current = &value
				break
			}
		}
	}
	if current != nil && input.Empty() && current.Range().End == input.Start {
		if !current.Range().Empty() {
			return MarkerAction{
				Handled:  true,
				Kind:     MarkerExit,
				Node:     node.ID(),
				Range:    input,
				Mark:     current.ID(),
				HasMark:  true,
				Level:    current.Level(),
				Head:     input.End,
				Exited:   true,
				Consumed: true,
			}, nil
		}
		next := uint8(2)
		if current.Level() == 2 {
			next = 0
		}
		if next == 2 && !settings.AllowStrong {
			return MarkerAction{
				Handled:  true,
				Kind:     MarkerUpgrade,
				Node:     node.ID(),
				Range:    input,
				Mark:     current.ID(),
				HasMark:  true,
				Level:    current.Level(),
				Head:     input.End,
				NoOp:     true,
				Consumed: true,
			}, nil
		}
		return MarkerAction{
			Handled:  true,
			Kind:     MarkerUpgrade,
			Node:     node.ID(),
			Range:    input,
			Mark:     current.ID(),
			HasMark:  true,
			Level:    next,
			Head:     input.End,
			Consumed: true,
		}, nil
	}
	if input.Empty() {
		return MarkerAction{
			Handled:  true,
			Kind:     MarkerCreate,
			Node:     node.ID(),
			Range:    input,
			Level:    1,
			Head:     input.End,
			Consumed: true,
		}, nil
	}
	return MarkerAction{
		Handled:  true,
		Kind:     MarkerFormat,
		Node:     node.ID(),
		Range:    input,
		Level:    1,
		Head:     input.End,
		Consumed: true,
	}, nil
}

// Apply writes a marker action to a document draft. Caller-owned selection
// state must still consume Exited/Head after the transaction succeeds.
func (a MarkerAction) Apply(draft *model.DocumentDraft) error {
	if draft == nil || !a.Handled || a.Exited || a.NoOp {
		return nil
	}
	switch a.Kind {
	case MarkerCreate:
		_, err := draft.AddMark(a.Node, a.Range, a.Level)
		return err
	case MarkerUpgrade:
		if !a.HasMark {
			return fmt.Errorf("marker upgrade has no mark identity")
		}
		return draft.SetMarkLevel(a.Node, a.Mark, a.Level)
	case MarkerFormat:
		return draft.FormatEmphasis(a.Node, a.Range, a.Level)
	default:
		return fmt.Errorf("unknown marker action %d", a.Kind)
	}
}

// MarkerEffect resolves the active mark after a create/format action. It
// prefers the newest exact-range mark, matching Shoreward's marker_effect.
func MarkerEffect(node *model.Node, range16 model.UTF16Range) (model.MarkID, bool) {
	if node == nil {
		return 0, false
	}
	var best *model.InlineMark
	for _, mark := range node.Marks() {
		if mark.Range() != range16 {
			continue
		}
		value := mark
		if best == nil || value.ID() > best.ID() {
			best = &value
		}
	}
	if best == nil {
		return 0, false
	}
	return best.ID(), true
}

// InlineInputStyle resolves the current typing style from the active mark,
// then falls back to the legacy emphasis attribute.
func InlineInputStyle(
	node *model.Node,
	active *model.MarkID,
	enabled bool,
) surface.TextStyle {
	if !enabled || node == nil {
		return emphasisStyle(0)
	}
	level := legacyEmphasisLevel(node)
	if active != nil {
		for _, mark := range node.Marks() {
			if mark.ID() == *active {
				level = mark.Level()
				break
			}
		}
	}
	return emphasisStyle(level)
}

// InlineStyleRuns converts projection segments to contiguous byte style runs.
// Generated attribute notation is muted; editable semantic text receives the
// strongest enclosing mark and falls back to legacy whole-node emphasis.
func InlineStyleRuns(
	p *projection.Projection,
	node *model.Node,
	enabled bool,
) ([]surface.StyleRun, error) {
	if p == nil || node == nil {
		return nil, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	text := p.Text()
	byteOffset := 0
	runs := make([]surface.StyleRun, 0, len(p.Segments()))
	for _, segment := range p.Segments() {
		units := segment.ProjectedRange().Len()
		if units == 0 {
			continue
		}
		end := byteOffset
		seen := 0
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if size <= 0 {
				return nil, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			}
			seen += model.RuneUTF16Len(r)
			end += size
			if seen >= units {
				break
			}
		}
		if seen != units {
			return nil, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
		}
		style := emphasisStyle(0)
		style.Color = surface.ColorMuted
		if target, ok := segment.SemanticTarget(); ok {
			targetRange := target.RangeUTF16()
			level := uint8(0)
			if enabled {
				level = legacyEmphasisLevel(node)
				for _, mark := range node.Marks() {
					if mark.Range().Start <= targetRange.Start && targetRange.End <= mark.Range().End {
						level = max(level, mark.Level())
					}
				}
			}
			style = emphasisStyle(level)
		}
		pushSurfaceRun(&runs, surface.StyleRun{
			Bytes: model.NewByteRange(model.ByteOffset(byteOffset), model.ByteOffset(end)),
			Style: style,
		})
		byteOffset = end
	}
	if byteOffset != len(text) {
		return nil, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	return runs, nil
}

func emphasisStyle(level uint8) surface.TextStyle {
	return surface.TextStyle{
		Bold:   level == 2,
		Italic: level == 1,
		Color:  surface.ColorText,
	}
}

func pushSurfaceRun(runs *[]surface.StyleRun, next surface.StyleRun) {
	if next.Bytes.Empty() {
		return
	}
	if len(*runs) > 0 {
		last := &(*runs)[len(*runs)-1]
		if last.Style == next.Style && last.Bytes.End == next.Bytes.Start {
			last.Bytes.End = next.Bytes.End
			return
		}
	}
	*runs = append(*runs, next)
}

func derefMark(id *model.MarkID) model.MarkID {
	if id == nil {
		return 0
	}
	return *id
}

func nodeID(node *model.Node) model.NodeID {
	if node == nil {
		return 0
	}
	return node.ID()
}
