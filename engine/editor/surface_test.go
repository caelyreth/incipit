package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
)

func TestSurfacePolicyMatchesRustCommandCatalog(t *testing.T) {
	t.Parallel()
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	policy, ok := catalog.Policy(SurfacePolicyID)
	if !ok {
		t.Fatal("surface policy missing")
	}
	if !slices.Equal(policy.Requires, []kernel.PolicyID{TextPolicyID}) ||
		!policy.EnabledByDefault {
		t.Fatalf("surface policy = %#v", policy)
	}
	want := []kernel.CommandID{
		SurfaceUpCommandID,
		SurfaceDownCommandID,
		SurfaceSelectUpCommandID,
		SurfaceSelectDownCommandID,
		SurfaceCopyCommandID,
		SurfaceCutCommandID,
		SurfacePasteCommandID,
		SurfaceNewPageCommandID,
		SurfaceRenamePageCommandID,
		SurfaceNextPageCommandID,
		SurfacePreviousPageCommandID,
		SurfaceClosePageCommandID,
		SurfaceZenCommandID,
		SurfaceTypewriterCommandID,
		SurfaceMotionCommandID,
		SurfaceCJKCommandID,
		SurfaceCommandsCommandID,
		SurfaceFindPageCommandID,
		SurfaceSidebarCommandID,
		SurfaceSettingsCommandID,
	}
	if !slices.Equal(policy.Commands, want) {
		t.Fatalf("surface commands = %#v", policy.Commands)
	}

	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	configuration := host.PreparedSnapshot()
	if !configuration.IsEnabled(SurfacePolicyID) {
		t.Fatal("surface policy is disabled by default")
	}
	for _, command := range SurfaceCommands() {
		got, ok := configuration.CommandFor(command.Binding())
		if !ok || got != command.ID {
			t.Fatalf("%s binding resolves to %s, %v", command.ID, got, ok)
		}
	}
}

func TestSurfaceBindingsPreserveRustModifiersAndKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id        kernel.CommandID
		key       string
		modifiers kernel.Modifiers
	}{
		{SurfaceUpCommandID, "up", 0},
		{SurfaceDownCommandID, "down", 0},
		{SurfaceSelectUpCommandID, "up", kernel.ModifiersShift},
		{SurfaceSelectDownCommandID, "down", kernel.ModifiersShift},
		{SurfaceCopyCommandID, "c", kernel.ModifiersPrimary},
		{SurfaceCutCommandID, "x", kernel.ModifiersPrimary},
		{SurfacePasteCommandID, "v", kernel.ModifiersPrimary},
		{SurfaceNewPageCommandID, "n", kernel.ModifiersPrimary},
		{
			SurfaceRenamePageCommandID,
			"r",
			kernel.ModifiersPrimary | kernel.ModifiersShift,
		},
		{
			SurfaceNextPageCommandID,
			"]",
			kernel.ModifiersPrimary | kernel.ModifiersShift,
		},
		{
			SurfacePreviousPageCommandID,
			"[",
			kernel.ModifiersPrimary | kernel.ModifiersShift,
		},
		{SurfaceClosePageCommandID, "w", kernel.ModifiersPrimary},
		{
			SurfaceZenCommandID,
			"f",
			kernel.ModifiersPrimary | kernel.ModifiersShift,
		},
		{
			SurfaceTypewriterCommandID,
			"t",
			kernel.ModifiersPrimary | kernel.ModifiersAlt,
		},
		{
			SurfaceMotionCommandID,
			"m",
			kernel.ModifiersPrimary | kernel.ModifiersAlt,
		},
		{
			SurfaceCJKCommandID,
			"j",
			kernel.ModifiersPrimary | kernel.ModifiersShift,
		},
		{SurfaceCommandsCommandID, "k", kernel.ModifiersPrimary},
		{SurfaceFindPageCommandID, "p", kernel.ModifiersPrimary},
		{
			SurfaceSidebarCommandID,
			"b",
			kernel.ModifiersPrimary | kernel.ModifiersAlt,
		},
		{SurfaceSettingsCommandID, ",", kernel.ModifiersPrimary},
	}
	if len(tests) != len(AllSurfaceCommands) {
		t.Fatalf("test matrix=%d commands=%d", len(tests), len(AllSurfaceCommands))
	}
	for _, test := range tests {
		command, ok := SurfaceCommandForID(test.id)
		if !ok {
			t.Fatalf("%s missing", test.id)
		}
		if command.Key != test.key || command.Modifiers != test.modifiers ||
			command.BindingCtx != kernel.EditorBindingContext {
			t.Fatalf("%s = %#v", test.id, command)
		}
	}
}
