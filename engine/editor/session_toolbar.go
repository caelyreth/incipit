package editor

import "github.com/caelyreth/neverland/engine/model"

// ToolbarState is the editor-owned state of the floating writing toolbar.
type ToolbarState struct {
	EmphasisEnabled bool
	Emphasis        bool
	StrongEnabled   bool
	Strong          bool
	CanUndo         bool
	CanRedo         bool
}

// ToolbarState resolves formatting and history state from the installed
// configuration and the current semantic selection. It does not mutate.
func (s *Session) ToolbarState() ToolbarState {
	if s == nil {
		return ToolbarState{}
	}
	configuration := s.runtime.Configuration()
	allowStrong := false
	if value, ok := configuration.Setting(NewWritingSettingKey(allowStrongSetting)); ok {
		if enabled, ok := value.Bool(); ok {
			allowStrong = enabled
		}
	}
	commandEnabled := configuration.IsCommandEnabled(MarkupEmphasis.ID())
	state := ToolbarState{
		CanUndo: s.history != nil && s.history.UndoDepth() > 0,
		CanRedo: s.history != nil && s.history.RedoDepth() > 0,
	}
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok {
		return state
	}
	active := activeMark(s.inline)
	settings := MarkupSettings{
		PolicyEnabled:  configuration.IsEnabled(WritingPolicyID),
		CommandEnabled: commandEnabled,
		AllowStrong:    allowStrong,
	}
	emphasis := MarkupStateFor(MarkupEmphasis, settings, node, s.presentation.Selection, active)
	strong := MarkupStateFor(MarkupStrong, settings, node, s.presentation.Selection, active)
	state.EmphasisEnabled = emphasis.Enabled
	state.Emphasis = emphasis.Selected
	state.StrongEnabled = strong.Enabled
	state.Strong = strong.Selected
	return state
}

// SelectionSnapshot returns the current selection for UI state consumers.
func (s *Session) SelectionSnapshot() (model.TextSelection, bool) {
	if s == nil {
		return model.TextSelection{}, false
	}
	return s.presentation.Selection, true
}
