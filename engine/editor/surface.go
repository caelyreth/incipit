package editor

import "github.com/caelyreth/neverland/engine/kernel"

// SurfacePolicyID owns input commands whose effects cross the editor surface,
// clipboard, workspace, presentation, and application boundaries.
const SurfacePolicyID kernel.PolicyID = "editor.surface"

// SurfaceCommandKind identifies the native owner of a surface command.
type SurfaceCommandKind uint8

const (
	SurfaceVisual SurfaceCommandKind = iota + 1
	SurfaceClipboard
	SurfacePage
	SurfacePresentation
	SurfacePolicy
	SurfaceApp
)

// Surface command identities match the Rust Shoreward host catalog.
const (
	SurfaceUpCommandID           kernel.CommandID = "surface.up"
	SurfaceDownCommandID         kernel.CommandID = "surface.down"
	SurfaceSelectUpCommandID     kernel.CommandID = "surface.select-up"
	SurfaceSelectDownCommandID   kernel.CommandID = "surface.select-down"
	SurfaceCopyCommandID         kernel.CommandID = "surface.copy"
	SurfaceCutCommandID          kernel.CommandID = "surface.cut"
	SurfacePasteCommandID        kernel.CommandID = "surface.paste"
	SurfaceNewPageCommandID      kernel.CommandID = "surface.new-page"
	SurfaceRenamePageCommandID   kernel.CommandID = "surface.rename-page"
	SurfaceNextPageCommandID     kernel.CommandID = "surface.next-page"
	SurfacePreviousPageCommandID kernel.CommandID = "surface.previous-page"
	SurfaceClosePageCommandID    kernel.CommandID = "surface.close-page"
	SurfaceZenCommandID          kernel.CommandID = "surface.zen"
	SurfaceTypewriterCommandID   kernel.CommandID = "surface.typewriter"
	SurfaceMotionCommandID       kernel.CommandID = "surface.motion"
	SurfaceCJKCommandID          kernel.CommandID = "surface.cjk"
	SurfaceCommandsCommandID     kernel.CommandID = "surface.commands"
	SurfaceFindPageCommandID     kernel.CommandID = "surface.find-page"
	SurfaceSidebarCommandID      kernel.CommandID = "surface.sidebar"
	SurfaceSettingsCommandID     kernel.CommandID = "surface.settings"
)

// SurfaceCommand is one complete declaration in the Rust-compatible surface
// policy. The ID is the stable routing identity; Kind selects its native owner.
type SurfaceCommand struct {
	ID         kernel.CommandID
	Label      string
	Key        string
	Modifiers  kernel.Modifiers
	Kind       SurfaceCommandKind
	BindingCtx kernel.BindingContext
}

// Binding returns the normalized default binding for this command.
func (c SurfaceCommand) Binding() kernel.BindingTarget {
	return kernel.NewBindingTarget(c.BindingCtx, c.Key, c.Modifiers)
}

// Description converts the declaration to the metadata shape used by the
// shared policy, settings, and command catalogs.
func (c SurfaceCommand) Description() Command {
	return Command{
		ID:         c.ID,
		Label:      c.Label,
		Key:        c.Key,
		Modifiers:  c.Modifiers,
		Policy:     SurfacePolicyID,
		BindingCtx: c.BindingCtx,
	}
}

var surfaceCommands = [...]SurfaceCommand{
	{
		ID: SurfaceUpCommandID, Label: "Move up", Key: "up",
		Kind: SurfaceVisual, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceDownCommandID, Label: "Move down", Key: "down",
		Kind: SurfaceVisual, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceSelectUpCommandID, Label: "Extend up", Key: "up",
		Modifiers: kernel.ModifiersShift, Kind: SurfaceVisual,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceSelectDownCommandID, Label: "Extend down", Key: "down",
		Modifiers: kernel.ModifiersShift, Kind: SurfaceVisual,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceCopyCommandID, Label: "Copy", Key: "c",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfaceClipboard,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceCutCommandID, Label: "Cut", Key: "x",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfaceClipboard,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfacePasteCommandID, Label: "Paste", Key: "v",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfaceClipboard,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceNewPageCommandID, Label: "New page", Key: "n",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfacePage,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceRenamePageCommandID, Label: "Rename page", Key: "r",
		Modifiers: kernel.ModifiersPrimary | kernel.ModifiersShift,
		Kind:      SurfacePage, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceNextPageCommandID, Label: "Next tab", Key: "]",
		Modifiers: kernel.ModifiersPrimary | kernel.ModifiersShift,
		Kind:      SurfacePage, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfacePreviousPageCommandID, Label: "Previous tab", Key: "[",
		Modifiers: kernel.ModifiersPrimary | kernel.ModifiersShift,
		Kind:      SurfacePage, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceClosePageCommandID, Label: "Close tab", Key: "w",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfacePage,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceZenCommandID, Label: "Toggle Zen mode", Key: "f",
		Modifiers: kernel.ModifiersPrimary | kernel.ModifiersShift,
		Kind:      SurfacePresentation, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceTypewriterCommandID, Label: "Toggle typewriter mode",
		Key: "t", Modifiers: kernel.ModifiersPrimary | kernel.ModifiersAlt,
		Kind: SurfacePresentation, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceMotionCommandID, Label: "Toggle motion", Key: "m",
		Modifiers: kernel.ModifiersPrimary | kernel.ModifiersAlt,
		Kind:      SurfacePresentation, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceCJKCommandID, Label: "Toggle CJK refinement", Key: "j",
		Modifiers: kernel.ModifiersPrimary | kernel.ModifiersShift,
		Kind:      SurfacePolicy, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceCommandsCommandID, Label: "Search commands", Key: "k",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfaceApp,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceFindPageCommandID, Label: "Find a page", Key: "p",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfaceApp,
		BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceSidebarCommandID, Label: "Toggle sidebar", Key: "b",
		Modifiers: kernel.ModifiersPrimary | kernel.ModifiersAlt,
		Kind:      SurfaceApp, BindingCtx: kernel.EditorBindingContext,
	},
	{
		ID: SurfaceSettingsCommandID, Label: "Open settings", Key: ",",
		Modifiers: kernel.ModifiersPrimary, Kind: SurfaceApp,
		BindingCtx: kernel.EditorBindingContext,
	},
}

// AllSurfaceCommands is the stable Rust declaration order. Callers may read
// it directly but must not append to it.
var AllSurfaceCommands = surfaceCommands

// SurfaceCommands returns a copy in stable declaration order.
func SurfaceCommands() []SurfaceCommand {
	return append([]SurfaceCommand(nil), surfaceCommands[:]...)
}

// SurfaceCommandForID resolves one surface declaration.
func SurfaceCommandForID(id kernel.CommandID) (SurfaceCommand, bool) {
	for _, command := range surfaceCommands {
		if command.ID == id {
			return command, true
		}
	}
	return SurfaceCommand{}, false
}

func surfacePolicyDescriptor() kernel.PolicyDescriptor {
	commands := make([]kernel.CommandID, 0, len(surfaceCommands))
	bindings := make([]kernel.BindingSpec, 0, len(surfaceCommands))
	for _, command := range surfaceCommands {
		commands = append(commands, command.ID)
		bindings = append(bindings, kernel.BindingSpec{
			Command: command.ID,
			Target:  command.Binding(),
		})
	}
	return kernel.PolicyDescriptor{
		ID:               SurfacePolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Commands:         commands,
		Bindings:         bindings,
	}
}
