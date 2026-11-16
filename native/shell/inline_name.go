package shell

import "github.com/egoist/mygo/ui"

type nameEditAction uint8

const (
	nameContinue nameEditAction = iota
	nameCancel
	nameCommit
)

// inlineName follows an existing input: select when naming starts, cancel
// on Escape, and commit on submission or after the field loses focus.
func inlineName(input ui.Element) nameEditAction {
	selection := ui.Local(input, "name-selection", func() ui.Handle {
		var handle ui.Handle
		handle.SelectAll()
		return handle
	})
	input.Bind(selection)
	switch {
	case input.Shortcut(0, ui.KeyEscape):
		return nameCancel
	case input.Submitted(), input.FocusLeft():
		return nameCommit
	}
	return nameContinue
}
