package keymap

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

func TestEveryEditorCommandResolvesToTypedAction(t *testing.T) {
	t.Parallel()
	for _, command := range editor.AllTextCommands {
		action, ok := ResolveEditorCommand(command.ID())
		if !ok || action.Kind != ActionText || action.Text != command {
			t.Fatalf("text %v => %#v,%v", command.ID(), action, ok)
		}
	}
	for _, command := range editor.AllHistoryCommands {
		action, ok := ResolveEditorCommand(command.ID())
		if !ok || action.Kind != ActionHistory || action.History != command {
			t.Fatalf("history %v => %#v,%v", command.ID(), action, ok)
		}
	}
	for _, command := range []editor.MarkupCommand{editor.MarkupEmphasis, editor.MarkupStrong} {
		action, ok := ResolveEditorCommand(command.ID())
		if !ok || action.Kind != ActionMarkup || action.Markup != command {
			t.Fatalf("markup %v => %#v,%v", command.ID(), action, ok)
		}
	}
	for _, command := range editor.AllWordCommands {
		action, ok := ResolveEditorCommand(command.ID())
		if !ok || action.Kind != ActionWord || action.Word != command {
			t.Fatalf("word %v => %#v,%v", command.ID(), action, ok)
		}
	}
	action, ok := ResolveEditorCommand(editor.TabCommand().ID)
	if !ok || action.Kind != ActionNavigation || action.Navigation != NavigationTab {
		t.Fatalf("tab => %#v,%v", action, ok)
	}
	for _, command := range []ClipboardCommand{ClipboardCopy, ClipboardCut, ClipboardPaste} {
		action, ok := ResolveEditorCommand(command.ID())
		if !ok || action.Kind != ActionClipboard || action.Clipboard != command {
			t.Fatalf("clipboard %v => %#v,%v", command.ID(), action, ok)
		}
	}
	for _, command := range editor.SurfaceCommands() {
		if command.Kind == editor.SurfaceClipboard {
			continue
		}
		action, ok := ResolveEditorCommand(command.ID)
		if !ok || action.Kind != ActionSurface || action.Surface != command {
			t.Fatalf("surface %v => %#v,%v", command.ID, action, ok)
		}
	}
	if action, ok := ResolveEditorCommand(kernel.CommandID("host.external")); ok ||
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
		text       editor.TextCommand
		history    editor.HistoryCommand
		markup     editor.MarkupCommand
		shouldFind bool
	}{
		{"copy", ActionClipboard, CopyCommandID, ClipboardCopy, 0, 0, 0, true},
		{"cut", ActionClipboard, CutCommandID, ClipboardCut, 0, 0, 0, true},
		{"paste", ActionClipboard, PasteCommandID, ClipboardPaste, 0, 0, 0, true},
		{"pasteAndMatchStyle", ActionClipboard, PasteCommandID, ClipboardPaste, 0, 0, 0, true},
		{"undo", ActionHistory, editor.HistoryUndo.ID(), 0, 0, editor.HistoryUndo, 0, true},
		{"redo", ActionHistory, editor.HistoryRedo.ID(), 0, 0, editor.HistoryRedo, 0, true},
		{"selectAll", ActionText, editor.TextSelectAll.ID(), 0, editor.TextSelectAll, 0, 0, true},
		{"delete", ActionText, editor.TextBackspace.ID(), 0, editor.TextBackspace, 0, 0, true},
		{"bold", ActionMarkup, editor.MarkupStrong.ID(), 0, 0, 0, editor.MarkupStrong, true},
		{"italic", ActionMarkup, editor.MarkupEmphasis.ID(), 0, 0, 0, editor.MarkupEmphasis, true},
		{"underline", ActionNone, "", 0, 0, 0, 0, false},
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
			action.Clipboard != test.clipboard || action.Text != test.text ||
			action.History != test.history || action.Markup != test.markup {
			t.Fatalf("%s action = %#v", test.name, action)
		}
	}
}

func TestRepeatPolicyMatchesSemanticCommandFamilies(t *testing.T) {
	t.Parallel()
	for _, action := range []Action{
		{Kind: ActionText, Text: editor.TextRight},
		{Kind: ActionHistory, History: editor.HistoryUndo},
		{Kind: ActionWord, Word: editor.WordNext},
	} {
		if !AllowsRepeat(action) {
			t.Fatalf("%v did not allow repeat", action)
		}
	}
	for _, action := range []Action{
		{Kind: ActionMarkup, Markup: editor.MarkupStrong},
		{Kind: ActionNavigation, Navigation: NavigationTab},
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
		action, _ := ResolveEditorCommand(command.ID)
		want := command.Kind == editor.SurfaceVisual ||
			command.ID == editor.SurfaceNextPageCommandID ||
			command.ID == editor.SurfacePreviousPageCommandID
		if AllowsRepeat(action) != want {
			t.Fatalf("%s repeat = %v, want %v", command.ID, !want, want)
		}
	}
}
