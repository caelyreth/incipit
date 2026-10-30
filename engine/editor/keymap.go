package editor

import "github.com/caelyreth/neverland/engine/kernel"

// The default keymap is the one place editor shortcuts are written down.
// Command declarations name actions and nothing about keys; this layer gives
// them their default keys, and a configuration's own remaps sit above it. The
// kernel checks the layers together, so a default and a remap that collide
// are reported like any other conflict.
var defaultKeymap = [...]kernel.BindingSpec{
	defaultKey("text.left", "left", 0),
	defaultKey("text.right", "right", 0),
	defaultKey("text.select-left", "left", keyShift),
	defaultKey("text.select-right", "right", keyShift),
	defaultKey("text.line-start", "home", 0),
	defaultKey("text.line-end", "end", 0),
	defaultKey("text.select-line-start", "home", keyShift),
	defaultKey("text.select-line-end", "end", keyShift),
	defaultKey("text.document-start", "home", keyPrimary),
	defaultKey("text.document-end", "end", keyPrimary),
	defaultKey("text.select-document-start", "home", keyPrimary|keyShift),
	defaultKey("text.select-document-end", "end", keyPrimary|keyShift),
	defaultKey("text.select-all", "a", keyPrimary),
	defaultKey("text.backspace", "backspace", 0),
	defaultKey("text.delete", "delete", 0),
	defaultKey("text.newline", "enter", 0),

	defaultKey(markupEmphasisCommandID, "i", keyPrimary),
	defaultKey(markupStrongCommandID, "b", keyPrimary),
	defaultKey(nestDeeperCommandID, "]", keyPrimary),
	defaultKey(nestShallowerCommandID, "[", keyPrimary),

	defaultKey("history.undo", "z", keyPrimary),
	defaultKey("history.redo", "z", keyPrimary|keyShift),

	defaultKey(navigationTabCommandID, "tab", 0),

	defaultKey(previousWordCommandID, "left", keyAlt),
	defaultKey(nextWordCommandID, "right", keyAlt),
	defaultKey(extendPreviousWordCommandID, "left", keyAlt|keyShift),
	defaultKey(extendNextWordCommandID, "right", keyAlt|keyShift),
	defaultKey(deletePreviousWordCommandID, "backspace", keyAlt),
	defaultKey(deleteNextWordCommandID, "delete", keyAlt),
	defaultKey(selectWordCommandID, "d", keyPrimary),
	defaultKey(selectSentenceCommandID, "s", keyAlt|keyShift),
	defaultKey(selectParagraphCommandID, "p", keyAlt|keyShift),

	defaultKey(SurfaceUpCommandID, "up", 0),
	defaultKey(SurfaceDownCommandID, "down", 0),
	defaultKey(SurfaceSelectUpCommandID, "up", keyShift),
	defaultKey(SurfaceSelectDownCommandID, "down", keyShift),
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
	defaultKey(SurfaceSettingsCommandID, ",", keyPrimary),
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
