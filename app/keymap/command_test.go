package keymap

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

func TestEveryEditorCommandResolvesToTypedAction(t *testing.T) {
	t.Parallel()
	for _, command := range editor.Commands() {
		if !command.Edits() {
			continue
		}
		action, ok := ResolveCommand(command.ID)
		if !ok || action.Kind != ActionEdit || action.Intent != command.Intent {
			t.Fatalf("edit %v => %#v,%v", command.ID, action, ok)
		}
	}
	for _, command := range []ClipboardCommand{ClipboardCopy, ClipboardCut, ClipboardPaste} {
		action, ok := ResolveCommand(command.ID())
		if !ok || action.Kind != ActionClipboard || action.Clipboard != command {
			t.Fatalf("clipboard %v => %#v,%v", command.ID(), action, ok)
		}
	}
	for _, command := range editor.SurfaceCommands() {
		if command.Kind == editor.SurfaceClipboard {
			continue
		}
		action, ok := ResolveCommand(command.ID)
		if !ok || action.Kind != ActionSurface || action.Surface != command {
			t.Fatalf("surface %v => %#v,%v", command.ID, action, ok)
		}
	}
	for _, command := range editor.Commands() {
		action, ok := ResolveCommand(command.ID)
		if !ok || !action.Resolved || action.Owner != command.Owner ||
			action.Requirements != command.Requirements {
			t.Fatalf("metadata %s => %#v,%v; command %#v", command.ID, action, ok, command)
		}
	}
	if action, ok := ResolveCommand(kernel.CommandID("host.external")); ok ||
		action.Kind != ActionNone {
		t.Fatalf("external command = %#v,%v", action, ok)
	}
}

func TestMenuActionsPreserveEditCommandDistinctions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		kind       ActionKind
		command    kernel.CommandID
		clipboard  ClipboardCommand
		intent     editor.Intent
		shouldFind bool
	}{
		{"copy", ActionClipboard, CopyCommandID, ClipboardCopy, editor.Intent{}, true},
		{"cut", ActionClipboard, CutCommandID, ClipboardCut, editor.Intent{}, true},
		{"paste", ActionClipboard, PasteCommandID, ClipboardPaste, editor.Intent{}, true},
		{"pasteAndMatchStyle", ActionClipboard, PasteCommandID, ClipboardPaste, editor.Intent{}, true},
		{"undo", ActionEdit, editor.UndoCommandID, 0, editor.Undo, true},
		{"redo", ActionEdit, editor.RedoCommandID, 0, editor.Redo, true},
		{"selectAll", ActionEdit, editor.SelectAllCommandID, 0, editor.SelectUnit(editor.ByScope), true},
		{"delete", ActionEdit, editor.DeleteBackwardCommandID, 0, editor.Delete(editor.ByGrapheme, editor.Backward), true},
		{"bold", ActionEdit, editor.StrongCommandID, 0, editor.Format(2), true},
		{"italic", ActionEdit, editor.EmphasisCommandID, 0, editor.Format(1), true},
		{"underline", ActionNone, "", 0, editor.Intent{}, false},
	}
	for _, test := range tests {
		action, ok := MenuAction(test.name)
		if ok != test.shouldFind {
			t.Fatalf("%s lookup = %v, want %v", test.name, ok, test.shouldFind)
		}
		if !ok {
			continue
		}
		if action.Kind != test.kind || action.ID != test.command ||
			action.Clipboard != test.clipboard || action.Intent != test.intent {
			t.Fatalf("%s action = %#v", test.name, action)
		}
	}
}

func TestRepeatPolicyMatchesSemanticCommandFamilies(t *testing.T) {
	t.Parallel()
	for _, action := range []Action{
		{Kind: ActionEdit, Intent: editor.Move(editor.ByGrapheme, editor.Forward, false)},
		{Kind: ActionEdit, Intent: editor.Undo},
		{Kind: ActionEdit, Intent: editor.Move(editor.ByWord, editor.Forward, false)},
		{Kind: ActionEdit, Intent: editor.Move(editor.ByRow, editor.Forward, true)},
	} {
		if !AllowsRepeat(action) {
			t.Fatalf("%v did not allow repeat", action)
		}
	}
	for _, action := range []Action{
		{Kind: ActionEdit, Intent: editor.Format(2)},
		{Kind: ActionEdit, Intent: editor.Leave},
		{Kind: ActionClipboard, Clipboard: ClipboardPaste},
		{Kind: ActionExternal, ID: "host.external"},
		{Kind: ActionSurface, Surface: editor.SurfaceCommand{
			ID: editor.SurfaceNewPageCommandID, Kind: editor.SurfacePage,
		}},
	} {
		if AllowsRepeat(action) {
			t.Fatalf("%v allowed repeat", action)
		}
	}
	for _, command := range editor.SurfaceCommands() {
		action, _ := ResolveCommand(command.ID)
		want := command.ID == editor.SurfaceNextPageCommandID ||
			command.ID == editor.SurfacePreviousPageCommandID
		if AllowsRepeat(action) != want {
			t.Fatalf("%s repeat = %v, want %v", command.ID, !want, want)
		}
	}
}
