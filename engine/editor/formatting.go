package editor

import "github.com/caelyreth/neverland/engine/model"

// MarkupSettings are the product gates for semantic inline formatting.
type MarkupSettings struct {
	PolicyEnabled  bool
	CommandEnabled bool
	AllowStrong    bool
}

// MarkupState drives formatting controls independently from platform widgets.
type MarkupState struct {
	Enabled  bool
	Selected bool
}

// FormattingLevel resolves the strongest applicable semantic mark. A collapsed
// selection uses the active mark; non-empty selections require full coverage.
// A legacy whole-node integer "emphasis" attribute is the fallback, clamped to
// the supported 0..2 range.
func FormattingLevel(
	node *model.Node,
	selection model.TextSelection,
	activeMark *model.MarkID,
	enabled bool,
) uint8 {
	if !enabled || node == nil {
		return 0
	}
	rangeUTF16, err := selection.Range()
	if err != nil {
		rangeUTF16 = model.UTF16Range{}
	}
	level := uint8(0)
	for _, mark := range node.Marks() {
		applicable := false
		if rangeUTF16.Empty() {
			applicable = activeMark != nil && *activeMark == mark.ID()
		} else {
			applicable = mark.Range().Start <= rangeUTF16.Start && rangeUTF16.End <= mark.Range().End
		}
		if applicable {
			level = max(level, mark.Level())
		}
	}
	if level == 0 {
		level = legacyEmphasisLevel(node)
	}
	return level
}

// MarkupStateFor resolves enabled/selected state for one product control.
func MarkupStateFor(
	command MarkupCommand,
	settings MarkupSettings,
	node *model.Node,
	selection model.TextSelection,
	activeMark *model.MarkID,
) MarkupState {
	enabled := settings.PolicyEnabled && settings.CommandEnabled &&
		(command == MarkupEmphasis || settings.AllowStrong)
	level := FormattingLevel(node, selection, activeMark, settings.PolicyEnabled)
	return MarkupState{
		Enabled:  enabled,
		Selected: level == command.Level(),
	}
}

// MarkupPlan is a checked semantic formatting request. The model draft remains
// responsible for applying or toggling the mark.
type MarkupPlan struct {
	Handled bool
	Node    model.NodeID
	Range   model.UTF16Range
	Level   uint8
	State   MarkupState
}

// PlanMarkup validates composition, selection, policy, and range before a
// formatting command reaches the model.
func PlanMarkup(
	command MarkupCommand,
	node *model.Node,
	selection model.TextSelection,
	activeMark *model.MarkID,
	settings MarkupSettings,
	compositionActive bool,
) (MarkupPlan, error) {
	if node == nil || node.ID() != selection.Node() {
		return MarkupPlan{}, &WordError{Kind: WordErrorInvalidOffset}
	}
	if compositionActive {
		return MarkupPlan{}, &MarkupError{Kind: MarkupErrorCompositionActive}
	}
	rangeUTF16, err := selection.Range()
	if err != nil {
		return MarkupPlan{}, err
	}
	if err := node.ValidateTextRangeUTF16(rangeUTF16.Start, rangeUTF16.End); err != nil {
		return MarkupPlan{}, err
	}
	state := MarkupStateFor(command, settings, node, selection, activeMark)
	if !state.Enabled {
		return MarkupPlan{Node: node.ID(), Range: rangeUTF16, Level: command.Level(), State: state}, nil
	}
	return MarkupPlan{
		Handled: true,
		Node:    node.ID(),
		Range:   rangeUTF16,
		Level:   command.Level(),
		State:   state,
	}, nil
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
	if e.Kind == MarkupErrorCompositionActive {
		return "markup command rejected while composition is active"
	}
	return "unknown markup error"
}

func legacyEmphasisLevel(node *model.Node) uint8 {
	value, ok := node.Attribute("emphasis")
	if !ok {
		return 0
	}
	level, ok := value.Integer()
	if !ok {
		return 0
	}
	return uint8(max(0, min(2, level)))
}
