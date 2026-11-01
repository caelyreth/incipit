package editor

import "github.com/caelyreth/neverland/engine/model"

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

// FormattingLevel resolves the strongest applicable semantic mark. A collapsed
// selection uses the active mark; non-empty selections require full coverage.
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
		if mark.Kind() != EmphasisMark {
			continue
		}
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
	enabled := settings.PolicyEnabled && settings.CommandEnabled
	level := FormattingLevel(node, selection, activeMark, true)
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
