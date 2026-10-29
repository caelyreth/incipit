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
// Scope is what it acts on; Subject names the editor policy a command
// switches, when it switches one.
type SurfaceCommand struct {
	ID      kernel.CommandID
	Label   string
	Kind    SurfaceCommandKind
	Scope   Scope
	Subject kernel.PolicyID
}

// Description converts the declaration to the metadata shape used by the
// shared policy, settings, and command catalogs.
func (c SurfaceCommand) Description() Command {
	return Command{ID: c.ID, Label: c.Label, Scope: c.Scope, Policy: SurfacePolicyID, Subject: c.Subject}
}

var surfaceCommands = [...]SurfaceCommand{
	{ID: SurfaceUpCommandID, Label: "Move up", Kind: SurfaceVisual, Scope: ScopeDocument},
	{ID: SurfaceDownCommandID, Label: "Move down", Kind: SurfaceVisual, Scope: ScopeDocument},
	{ID: SurfaceSelectUpCommandID, Label: "Extend up", Kind: SurfaceVisual, Scope: ScopeDocument},
	{ID: SurfaceSelectDownCommandID, Label: "Extend down", Kind: SurfaceVisual, Scope: ScopeDocument},
	{ID: SurfaceCopyCommandID, Label: "Copy", Kind: SurfaceClipboard, Scope: ScopeDocument},
	{ID: SurfaceCutCommandID, Label: "Cut", Kind: SurfaceClipboard, Scope: ScopeDocument},
	{ID: SurfacePasteCommandID, Label: "Paste", Kind: SurfaceClipboard, Scope: ScopeDocument},
	{ID: SurfaceNewPageCommandID, Label: "New page", Kind: SurfacePage, Scope: ScopePages},
	{ID: SurfaceRenamePageCommandID, Label: "Rename page", Kind: SurfacePage, Scope: ScopePages},
	{ID: SurfaceNextPageCommandID, Label: "Next tab", Kind: SurfacePage, Scope: ScopePages},
	{ID: SurfacePreviousPageCommandID, Label: "Previous tab", Kind: SurfacePage, Scope: ScopePages},
	{ID: SurfaceClosePageCommandID, Label: "Close tab", Kind: SurfacePage, Scope: ScopePages},
	{ID: SurfaceZenCommandID, Label: "Toggle Zen mode", Kind: SurfacePresentation, Scope: ScopeWindow},
	{ID: SurfaceTypewriterCommandID, Label: "Toggle typewriter mode", Kind: SurfacePresentation, Scope: ScopeWindow},
	{ID: SurfaceMotionCommandID, Label: "Toggle motion", Kind: SurfacePresentation, Scope: ScopeWindow},
	{ID: SurfaceCJKCommandID, Label: "Toggle CJK refinement", Kind: SurfacePolicy, Scope: ScopeDocument, Subject: CJKPolicyID},
	{ID: SurfaceCommandsCommandID, Label: "Search commands", Kind: SurfaceApp, Scope: ScopeApp},
	{ID: SurfaceFindPageCommandID, Label: "Find a page", Kind: SurfaceApp, Scope: ScopePages},
	{ID: SurfaceSidebarCommandID, Label: "Toggle sidebar", Kind: SurfaceApp, Scope: ScopeWindow},
	{ID: SurfaceSettingsCommandID, Label: "Open settings", Kind: SurfaceApp, Scope: ScopeApp},
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
	for _, command := range surfaceCommands {
		commands = append(commands, command.ID)
	}
	return kernel.PolicyDescriptor{
		ID:               SurfacePolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Commands:         commands,
	}
}
