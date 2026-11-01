package shell

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

type nameEditAction uint8

const (
	nameContinue nameEditAction = iota
	nameCancel
	nameCommit
)

// inlineName follows an existing input: select on each focus arrival, cancel
// on Escape, and commit on submission or after the field loses focus.
func inlineName(input ui.Element, name string) nameEditAction {
	focused := ui.Local(input, "was-focused", func() bool { return false })
	if !*focused && input.Focused() {
		input.SetTextSelection(0, utf8.RuneCountInString(name))
	}
	action := nameContinue
	switch {
	case input.Shortcut(0, ui.KeyEscape):
		action = nameCancel
	case input.Submitted(), *focused && !input.Focused():
		action = nameCommit
	}
	*focused = input.Focused()
	return action
}
