package keymap

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// ActionKind identifies the typed payload of an Action.
type ActionKind uint8

const (
	ActionNone ActionKind = iota
	ActionText
	ActionHistory
	ActionMarkup
	ActionWord
	ActionNavigation
	ActionClipboard
	ActionSurface
	ActionExternal
)

func (k ActionKind) String() string {
	switch k {
	case ActionText:
		return "text"
	case ActionHistory:
		return "history"
	case ActionMarkup:
		return "markup"
	case ActionWord:
		return "word"
	case ActionNavigation:
		return "navigation"
	case ActionClipboard:
		return "clipboard"
	case ActionSurface:
		return "surface"
	case ActionExternal:
		return "external"
	default:
		return "none"
	}
}

// NavigationCommand is an editor navigation action with no document mutation
// type declared by engine/editor yet.
type NavigationCommand uint8

const (
	NavigationTab NavigationCommand = iota + 1
)

// ID returns the configured command identity for this navigation action.
func (c NavigationCommand) ID() kernel.CommandID {
	switch c {
	case NavigationTab:
		return editor.TabCommand().ID
	default:
		return ""
	}
}

// ClipboardCommand distinguishes plain-text edit menu actions. Cut performs
// its document edit and clipboard write through the injected executor, so the
// clipboard changes only after a handled semantic replacement.
type ClipboardCommand uint8

const (
	ClipboardCopy ClipboardCommand = iota + 1
	ClipboardCut
	ClipboardPaste
)

const (
	CopyCommandID  = editor.SurfaceCopyCommandID
	CutCommandID   = editor.SurfaceCutCommandID
	PasteCommandID = editor.SurfacePasteCommandID
)

// ID returns Neverland's stable surface command identity.
func (c ClipboardCommand) ID() kernel.CommandID {
	switch c {
	case ClipboardCopy:
		return CopyCommandID
	case ClipboardCut:
		return CutCommandID
	case ClipboardPaste:
		return PasteCommandID
	default:
		return ""
	}
}

// Action is a resolved command with exactly one relevant typed payload selected
// by Kind. It contains no document, selection, clipboard, window, or widget
// state.
type Action struct {
	Kind       ActionKind
	ID         kernel.CommandID
	Text       editor.TextCommand
	History    editor.HistoryCommand
	Markup     editor.MarkupCommand
	Word       editor.WordCommand
	Navigation NavigationCommand
	Clipboard  ClipboardCommand
	Surface    editor.SurfaceCommand
}

// CommandID returns the stable identity of this action.
func (a Action) CommandID() kernel.CommandID { return a.ID }

// String returns a diagnostic label without dereferencing any UI state.
func (a Action) String() string {
	if a.ID == "" {
		return a.Kind.String()
	}
	return fmt.Sprintf("%s:%s", a.Kind, a.ID)
}

var editorActions = buildEditorActions()

func buildEditorActions() map[kernel.CommandID]Action {
	actions := make(map[kernel.CommandID]Action)
	for _, command := range editor.AllTextCommands {
		meta := command.Description()
		actions[meta.ID] = Action{Kind: ActionText, ID: meta.ID, Text: command}
	}
	for _, command := range editor.AllHistoryCommands {
		meta := command.Description()
		actions[meta.ID] = Action{Kind: ActionHistory, ID: meta.ID, History: command}
	}
	for _, command := range []editor.MarkupCommand{
		editor.MarkupEmphasis,
		editor.MarkupStrong,
	} {
		meta := command.Description()
		actions[meta.ID] = Action{Kind: ActionMarkup, ID: meta.ID, Markup: command}
	}
	for _, command := range editor.AllWordCommands {
		meta := command.Description()
		actions[meta.ID] = Action{Kind: ActionWord, ID: meta.ID, Word: command}
	}
	tab := editor.TabCommand()
	actions[tab.ID] = Action{
		Kind:       ActionNavigation,
		ID:         tab.ID,
		Navigation: NavigationTab,
	}
	for _, command := range editor.SurfaceCommands() {
		if command.Kind == editor.SurfaceClipboard {
			switch command.ID {
			case CopyCommandID:
				actions[command.ID] = clipboardAction(ClipboardCopy)
			case CutCommandID:
				actions[command.ID] = clipboardAction(ClipboardCut)
			case PasteCommandID:
				actions[command.ID] = clipboardAction(ClipboardPaste)
			}
			continue
		}
		actions[command.ID] = Action{
			Kind:    ActionSurface,
			ID:      command.ID,
			Surface: command,
		}
	}
	return actions
}

// ResolveEditorCommand maps a command identity to one of the editor's closed
// typed action sets. A false result means the command belongs to another host
// policy and should be routed as an external action.
func ResolveEditorCommand(command kernel.CommandID) (Action, bool) {
	action, ok := editorActions[command]
	return action, ok
}

func clipboardAction(command ClipboardCommand) Action {
	return Action{
		Kind:      ActionClipboard,
		ID:        command.ID(),
		Clipboard: command,
	}
}

// AllowsRepeat reports whether a held key may repeat the resolved action.
// Neverland permits continuous semantic navigation, visual-row movement,
// history traversal, and page cycling. It suppresses markup, Tab navigation,
// clipboard, and other host commands.
func AllowsRepeat(action Action) bool {
	switch action.Kind {
	case ActionText, ActionHistory, ActionWord:
		return true
	case ActionSurface:
		if action.Surface.Kind == editor.SurfaceVisual {
			return true
		}
		return action.Surface.Kind == editor.SurfacePage &&
			(action.ID == editor.SurfaceNextPageCommandID ||
				action.ID == editor.SurfacePreviousPageCommandID)
	default:
		return false
	}
}

// MenuAction maps an edit or format menu role delivered by MyGo as
// InputCommand to the action MenuCommands pairs it with. Paste-and-match-style
// degrades to plain Paste, matching Neverland's plain-text clipboard scope.
func MenuAction(name string) (Action, bool) {
	if name == "delete" {
		// MyGo's edit-menu Delete role is backward deletion, including a
		// selected range, rather than the physical forward-delete key.
		return ResolveEditorCommand(editor.TextBackspace.ID())
	}
	command, ok := MenuCommandFor(name)
	if !ok {
		return Action{}, false
	}
	return ResolveEditorCommand(command)
}
