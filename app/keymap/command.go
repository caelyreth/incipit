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
	// ActionEdit carries an editor intent.
	ActionEdit
	ActionClipboard
	ActionSurface
	ActionExternal
)

func (k ActionKind) String() string {
	switch k {
	case ActionEdit:
		return "edit"
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
	Kind         ActionKind
	ID           kernel.CommandID
	Owner        editor.CommandOwner
	Requirements editor.CommandRequirements
	Resolved     bool
	Intent       editor.Intent
	Clipboard    ClipboardCommand
	Surface      editor.SurfaceCommand
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

var catalogActions = buildCatalogActions()

func buildCatalogActions() map[kernel.CommandID]Action {
	actions := make(map[kernel.CommandID]Action)
	for _, command := range editor.Commands() {
		if !command.Edits() {
			continue
		}
		action := resolvedAction(command, ActionEdit)
		action.Intent = command.Intent
		actions[command.ID] = action
	}
	for _, command := range editor.SurfaceCommands() {
		meta := command.Description()
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
		action := resolvedAction(meta, ActionSurface)
		action.Surface = command
		actions[command.ID] = action
	}
	return actions
}

func resolvedAction(command editor.Command, kind ActionKind) Action {
	return Action{
		Kind:         kind,
		ID:           command.ID,
		Owner:        command.Owner,
		Requirements: command.Requirements,
		Resolved:     true,
	}
}

// ResolveCommand maps a catalog command identity to one typed action. A false
// result means the identity is not in the editor's command inventory.
func ResolveCommand(command kernel.CommandID) (Action, bool) {
	action, ok := catalogActions[command]
	return action, ok
}

func clipboardAction(command ClipboardCommand) Action {
	meta, _ := editor.CommandFor(command.ID())
	action := resolvedAction(meta, ActionClipboard)
	action.Clipboard = command
	return action
}

// AllowsRepeat reports whether a held key may repeat the resolved action.
// Neverland permits continuous semantic navigation, visual-row movement,
// history traversal, and page cycling. It suppresses markup, Tab navigation,
// clipboard, and other host commands.
func AllowsRepeat(action Action) bool {
	switch action.Kind {
	case ActionEdit:
		return action.Intent.Repeats()
	case ActionSurface:
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
		return ResolveCommand(editor.DeleteBackwardCommandID)
	}
	command, ok := MenuCommandFor(name)
	if !ok {
		return Action{}, false
	}
	return ResolveCommand(command)
}
