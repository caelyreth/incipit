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
	state := ToolbarState{
		CanUndo: s.history != nil && s.history.UndoDepth() > 0,
		CanRedo: s.history != nil && s.history.RedoDepth() > 0,
	}
	document := s.runtime.Snapshot().Document()
	active := activeMark(s.inline)
	ranges := selectionBlockRanges(document, s.presentation.Selection)
	coverage := formattingCoverageFor(document, ranges, EmphasisMark, active)
	settings := func(command MarkupCommand) MarkupSettings {
		return MarkupSettings{
			PolicyEnabled:  configuration.IsEnabled(WritingPolicyID),
			CommandEnabled: configuration.IsCommandEnabled(command.ID()),
		}
	}
	emphasis := markupStateFor(MarkupEmphasis, settings(MarkupEmphasis), coverage.strongest())
	strong := markupStateFor(MarkupStrong, settings(MarkupStrong), coverage.strongest())
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
