package editor

import "github.com/caelyreth/neverland/engine/kernel"

// The default keymap is the one place editor shortcuts are written down.
// Command declarations name actions and nothing about keys; this layer gives
// them their default keys, and a configuration's own remaps sit above it. The
// kernel checks the layers together, so a default and a remap that collide
// are reported like any other conflict.
var defaultKeymap = [...]kernel.BindingSpec{
	defaultKey(MoveLeftCommandID, "left", 0),
	defaultKey(MoveRightCommandID, "right", 0),
	defaultKey(ExtendLeftCommandID, "left", keyShift),
	defaultKey(ExtendRightCommandID, "right", keyShift),
	defaultKey(LineStartCommandID, "home", 0),
	defaultKey(LineEndCommandID, "end", 0),
	defaultKey(ExtendLineStartCommandID, "home", keyShift),
	defaultKey(ExtendLineEndCommandID, "end", keyShift),
	defaultKey(DocumentStartCommandID, "home", keyPrimary),
	defaultKey(DocumentEndCommandID, "end", keyPrimary),
	defaultKey(ExtendDocumentStartCommandID, "home", keyPrimary|keyShift),
	defaultKey(ExtendDocumentEndCommandID, "end", keyPrimary|keyShift),
	defaultKey(MoveUpCommandID, "up", 0),
	defaultKey(MoveDownCommandID, "down", 0),
	defaultKey(ExtendUpCommandID, "up", keyShift),
	defaultKey(ExtendDownCommandID, "down", keyShift),
	defaultKey(SelectAllCommandID, "a", keyPrimary),
	defaultKey(DismissCommandID, "escape", 0),
	defaultKey(DeleteBackwardCommandID, "backspace", 0),
	defaultKey(DeleteForwardCommandID, "delete", 0),
	defaultKey(BreakCommandID, "enter", 0),
	defaultKey(SoftBreakCommandID, "enter", keyShift),

	defaultKey(EmphasisCommandID, "i", keyPrimary),
	defaultKey(StrongCommandID, "b", keyPrimary),
	defaultKey(NestDeeperCommandID, "]", keyPrimary),
	defaultKey(NestShallowerCommandID, "[", keyPrimary),

	defaultKey(UndoCommandID, "z", keyPrimary),
	defaultKey(RedoCommandID, "z", keyPrimary|keyShift),

	defaultKey(LeaveCommandID, "tab", 0),
	defaultKey(ReturnCommandID, "tab", keyShift),

	defaultKey(PreviousWordCommandID, "left", keyAlt),
	defaultKey(NextWordCommandID, "right", keyAlt),
	defaultKey(ExtendPreviousWordCommandID, "left", keyAlt|keyShift),
	defaultKey(ExtendNextWordCommandID, "right", keyAlt|keyShift),
	defaultKey(DeletePreviousWordCommandID, "backspace", keyAlt),
	defaultKey(DeleteNextWordCommandID, "delete", keyAlt),
	defaultKey(SelectWordCommandID, "d", keyPrimary),
	defaultKey(SelectSentenceCommandID, "s", keyAlt|keyShift),
	defaultKey(SelectParagraphCommandID, "p", keyAlt|keyShift),

	defaultKey(SurfaceCopyCommandID, "c", keyPrimary),
	defaultKey(SurfaceCutCommandID, "x", keyPrimary),
	defaultKey(SurfacePasteCommandID, "v", keyPrimary),
	defaultKey(SurfaceNewPageCommandID, "n", keyPrimary),
	defaultKey(SurfaceNextPageCommandID, "]", keyPrimary|keyShift),
	defaultKey(SurfacePreviousPageCommandID, "[", keyPrimary|keyShift),
	defaultKey(SurfaceClosePageCommandID, "w", keyPrimary),
	defaultKey(SurfaceZenCommandID, "f", keyPrimary|keyShift),
	defaultKey(SurfaceTypewriterCommandID, "t", keyPrimary|keyAlt),
	defaultKey(SurfaceMotionCommandID, "m", keyPrimary|keyAlt),
	defaultKey(SurfaceCJKCommandID, "j", keyPrimary|keyShift),
	defaultKey(SurfaceCommandsCommandID, "k", keyPrimary),
	defaultKey(SurfaceFindPageCommandID, "p", keyPrimary),
	defaultKey(SurfaceSidebarCommandID, "b", keyPrimary|keyAlt),
	defaultKey(SurfaceSheetListCommandID, "l", keyPrimary|keyAlt),
	defaultKey(SurfaceLibraryViewCommandID, "1", keyPrimary),
	defaultKey(SurfaceSheetsViewCommandID, "2", keyPrimary),
	defaultKey(SurfaceEditorViewCommandID, "3", keyPrimary),
	defaultKey(SurfaceSettingsCommandID, ",", keyPrimary),
	defaultKey(SurfaceInsertImageCommandID, "i", keyPrimary|keyShift),
}

const (
	keyAlt     = kernel.ModifiersAlt
	keyPrimary = kernel.ModifiersPrimary
	keyShift   = kernel.ModifiersShift
)

func defaultKey(command kernel.CommandID, key string, modifiers kernel.Modifiers) kernel.BindingSpec {
	return kernel.BindingSpec{
		Command: command,
		Target:  kernel.NewBindingTarget(kernel.EditorBindingContext, key, modifiers),
	}
}

// DefaultKeymap returns the editor's default shortcuts in declaration order.
func DefaultKeymap() []kernel.BindingSpec {
	return append([]kernel.BindingSpec(nil), defaultKeymap[:]...)
}
