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
	CopyCommandID  kernel.CommandID = "surface.copy"
	CutCommandID   kernel.CommandID = "surface.cut"
	PasteCommandID kernel.CommandID = "surface.paste"
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
	for _, command := range []ClipboardCommand{
		ClipboardCopy,
		ClipboardCut,
		ClipboardPaste,
	} {
		actions[command.ID()] = clipboardAction(command)
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
// Neverland permits continuous semantic text and word navigation/edit commands
// and history traversal. It suppresses markup, Tab navigation, clipboard, and
// unclassified host commands.
func AllowsRepeat(action Action) bool {
	switch action.Kind {
	case ActionText, ActionHistory, ActionWord:
		return true
	default:
		return false
	}
}

// MenuAction maps an edit or format menu role delivered by MyGo as
// InputCommand. Paste-and-match-style deliberately degrades to plain Paste,
// matching Neverland's plain-text clipboard scope.
func MenuAction(name string) (Action, bool) {
	switch name {
	case "copy":
		return clipboardAction(ClipboardCopy), true
	case "cut":
		return clipboardAction(ClipboardCut), true
	case "paste", "pasteAndMatchStyle":
		return clipboardAction(ClipboardPaste), true
	case "undo":
		return ResolveEditorCommand(editor.HistoryUndo.ID())
	case "redo":
		return ResolveEditorCommand(editor.HistoryRedo.ID())
	case "selectAll":
		return ResolveEditorCommand(editor.TextSelectAll.ID())
	case "delete":
		// MyGo's edit-menu Delete role is backward deletion, including a
		// selected range, rather than the physical forward-delete key.
		return ResolveEditorCommand(editor.TextBackspace.ID())
	case "bold":
		return ResolveEditorCommand(editor.MarkupStrong.ID())
	case "italic":
		return ResolveEditorCommand(editor.MarkupEmphasis.ID())
	default:
		return Action{}, false
	}
}
