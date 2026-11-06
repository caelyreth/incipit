package editor

import (
	"slices"

	"github.com/caelyreth/neverland/engine/model"
)

// MarkupSettings are the product gates for semantic inline formatting.
type MarkupSettings struct {
	PolicyEnabled  bool
	CommandEnabled bool
}

// MarkupState drives formatting controls independently from platform widgets.
type MarkupState struct {
	Enabled  bool
	Selected bool
}

// formattingPartCoverage is the mark coverage of one block's part of a
// selection. Levels cover the whole part; adjacency in one level is coverage,
// but separate adjacent marks remain separate identities in the model.
type formattingPartCoverage struct {
	blockRange
	levels    map[uint8]struct{}
	strongest uint8
}

// formattingCoverage is the editor's single interpretation of what a
// formatting command would toggle. A collapsed selection is about the active
// mark at the caret; a non-empty selection requires each block's part to be
// covered at the same level.
type formattingCoverage struct {
	collapsed bool
	active    *model.InlineMark
	parts     []formattingPartCoverage
}

func formattingCoverageFor(
	document model.DocumentSnapshot,
	ranges []blockRange,
	kind model.MarkKind,
	activeMark *model.MarkID,
) formattingCoverage {
	collapsed := len(ranges) == 1 && ranges[0].Range.Empty()
	coverage := formattingCoverage{
		collapsed: collapsed,
		parts:     make([]formattingPartCoverage, 0, len(ranges)),
	}
	for _, part := range ranges {
		node, ok := document.Node(part.Node)
		if !ok {
			coverage.parts = append(coverage.parts, formattingPartCoverage{blockRange: part})
			continue
		}
		var active *model.InlineMark
		if collapsed && activeMark != nil {
			for _, mark := range node.Marks() {
				if mark.ID() == *activeMark && mark.Kind() == kind {
					value := mark
					active = &value
					break
				}
			}
		}
		if active != nil {
			coverage.active = active
		}
		levels := formattingLevelsCovering(node, part.Range, kind)
		if collapsed {
			levels = make(map[uint8]struct{}, 1)
			if active != nil {
				levels[active.Level()] = struct{}{}
			}
		}
		strongest := uint8(0)
		for level := range levels {
			strongest = max(strongest, level)
		}
		coverage.parts = append(coverage.parts, formattingPartCoverage{
			blockRange: part,
			levels:     levels,
			strongest:  strongest,
		})
	}
	return coverage
}

// strongest returns the strongest level that effectively covers the whole
// selection. For a mixed-level selection no one level covers every part, so
// no formatting control is selected.
func (c formattingCoverage) strongest() uint8 {
	if c.collapsed {
		if c.active != nil {
			return c.active.Level()
		}
		return 0
	}
	strongest := uint8(0)
	for _, part := range c.parts {
		if part.Range.Empty() {
			continue
		}
		for level := range part.levels {
			covered := true
			for _, other := range c.parts {
				if other.Range.Empty() {
					continue
				}
				if _, ok := other.levels[level]; !ok {
					covered = false
					break
				}
			}
			if covered {
				strongest = max(strongest, level)
			}
		}
	}
	return strongest
}

func (c formattingCoverage) partStrongest(index int) uint8 {
	if index < 0 || index >= len(c.parts) {
		return 0
	}
	return c.parts[index].strongest
}

// FormattingLevel resolves the strongest level that covers a selection in one
// node. A collapsed selection uses the active mark; a selection spanning nodes
// belongs to the document-aware coverage used by the session.
func FormattingLevel(
	node *model.Node,
	selection model.TextSelection,
	activeMark *model.MarkID,
	enabled bool,
) uint8 {
	if !enabled || node == nil || selection.Spans() {
		return 0
	}
	rangeUTF16, err := selection.Range()
	if err != nil {
		return 0
	}
	levels := formattingLevelsCovering(node, rangeUTF16, FormatMark)
	var active *model.InlineMark
	if rangeUTF16.Empty() && activeMark != nil {
		for _, mark := range node.Marks() {
			if mark.ID() == *activeMark && mark.Kind() == FormatMark {
				value := mark
				active = &value
				break
			}
		}
		levels = make(map[uint8]struct{}, 1)
		if active != nil {
			levels[active.Level()] = struct{}{}
		}
	}
	coverage := formattingCoverage{
		collapsed: rangeUTF16.Empty(),
		active:    active,
		parts: []formattingPartCoverage{{
			blockRange: blockRange{Node: node.ID(), Range: rangeUTF16},
			levels:     levels,
		}},
	}
	return coverage.strongest()
}

// formattingLevelsCovering returns every level whose marks cover all of range.
// Marks at one level form a union, so adjacent fragments cover a selection
// without being merged in the model.
func formattingLevelsCovering(
	node *model.Node,
	rangeUTF16 model.UTF16Range,
	kind model.MarkKind,
) map[uint8]struct{} {
	levels := make(map[uint8]struct{})
	if node == nil || rangeUTF16.Empty() {
		return levels
	}
	byteRange, err := node.TextByteRangeUTF16(rangeUTF16.Start, rangeUTF16.End)
	if err != nil {
		return levels
	}
	lines := nonEmptyLines(node.Text().Slice(int(byteRange.Start), int(byteRange.End)), rangeUTF16.Start)
	if len(lines) == 0 {
		return levels
	}
	byLevel := make(map[uint8][]model.UTF16Range)
	for _, mark := range node.Marks() {
		if mark.Kind() == kind {
			byLevel[mark.Level()] = append(byLevel[mark.Level()], mark.Range())
		}
	}
	for level, spans := range byLevel {
		slices.SortFunc(spans, func(left, right model.UTF16Range) int {
			if left.Start != right.Start {
				return compareOffset(left.Start, right.Start)
			}
			return compareOffset(left.End, right.End)
		})
		covered := true
		for _, line := range lines {
			end := line.Start
			for _, span := range spans {
				if span.Start > end {
					break
				}
				end = max(end, span.End)
				if end >= line.End {
					break
				}
			}
			if end < line.End {
				covered = false
				break
			}
		}
		if covered {
			levels[level] = struct{}{}
		}
	}
	return levels
}

// markCovers reports full coverage at one level. It is the range-oriented
// form of formattingLevelsCovering for callers that need one level's answer.
func markCovers(
	node *model.Node,
	rangeUTF16 model.UTF16Range,
	kind model.MarkKind,
	level uint8,
) bool {
	_, covered := formattingLevelsCovering(node, rangeUTF16, kind)[level]
	return covered
}

func compareOffset(left, right model.UTF16Offset) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// MarkupStateFor resolves enabled/selected state for one product control in a
// single node. Session.ToolbarState uses the same coverage across blocks.
func MarkupStateFor(
	command markupOp,
	settings MarkupSettings,
	node *model.Node,
	selection model.TextSelection,
	activeMark *model.MarkID,
) MarkupState {
	return markupStateFor(
		command,
		settings,
		FormattingLevel(node, selection, activeMark, true),
	)
}

func markupStateFor(command markupOp, settings MarkupSettings, level uint8) MarkupState {
	enabled := settings.PolicyEnabled && settings.CommandEnabled
	return MarkupState{
		Enabled:  enabled,
		Selected: level == command.Level(),
	}
}

// MarkupErrorKind identifies formatting command failures.
type MarkupErrorKind uint8

const (
	MarkupErrorCompositionActive MarkupErrorKind = iota + 1
)

// MarkupError reports a formatting command contract failure.
type MarkupError struct {
	Kind MarkupErrorKind
}

func (e *MarkupError) Error() string {
	switch e.Kind {
	case MarkupErrorCompositionActive:
		return "markup command rejected while composition is active"
	}
	return "unknown markup error"
}

// markupOp is one emphasis toggle, by the level it sets.
type markupOp uint8

const (
	markupEmphasis markupOp = 1
	markupStrong   markupOp = 2
)

// Level returns the emphasis strength.
func (c markupOp) Level() uint8 { return uint8(c) }
