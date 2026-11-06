package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
)

func TestSurfacePolicyMatchesCommandCatalog(t *testing.T) {
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
		SurfaceCopyCommandID,
		SurfaceCutCommandID,
		SurfacePasteCommandID,
		SurfaceNewPageCommandID,
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
		SurfaceSheetListCommandID,
		SurfaceLibraryViewCommandID,
		SurfaceSheetsViewCommandID,
		SurfaceEditorViewCommandID,
		SurfaceSettingsCommandID,
		SurfaceInsertImageCommandID,
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
		for _, target := range catalog.DefaultBindings(command.ID) {
			got, ok := configuration.CommandFor(target)
			if !ok || got != command.ID {
				t.Fatalf("%s binding resolves to %s, %v", command.ID, got, ok)
			}
		}
	}
}

func TestSurfaceBindingsPreserveModifiersAndKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id        kernel.CommandID
		key       string
		modifiers kernel.Modifiers
	}{
		{SurfaceCopyCommandID, "c", kernel.ModifiersPrimary},
		{SurfaceCutCommandID, "x", kernel.ModifiersPrimary},
		{SurfacePasteCommandID, "v", kernel.ModifiersPrimary},
		{SurfaceNewPageCommandID, "n", kernel.ModifiersPrimary},
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
		{
			SurfaceSheetListCommandID,
			"l",
			kernel.ModifiersPrimary | kernel.ModifiersAlt,
		},
		{SurfaceLibraryViewCommandID, "1", kernel.ModifiersPrimary},
		{SurfaceSheetsViewCommandID, "2", kernel.ModifiersPrimary},
		{SurfaceEditorViewCommandID, "3", kernel.ModifiersPrimary},
		{SurfaceSettingsCommandID, ",", kernel.ModifiersPrimary},
		{SurfaceInsertImageCommandID, "i", kernel.ModifiersPrimary | kernel.ModifiersShift},
	}
	if len(tests) != len(AllSurfaceCommands) {
		t.Fatalf("test matrix=%d commands=%d", len(tests), len(AllSurfaceCommands))
	}
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		want := []kernel.BindingTarget{
			kernel.NewBindingTarget(kernel.EditorBindingContext, test.key, test.modifiers),
		}
		if got := catalog.DefaultBindings(test.id); !slices.Equal(got, want) {
			t.Fatalf("%s defaults = %#v, want %#v", test.id, got, want)
		}
	}
}

// Every action is declared once, named once and given its keys only by the
// default keymap; the registry every catalog reads is that same set.
func TestActionRegistryMatchesThePolicyCatalog(t *testing.T) {
	t.Parallel()
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	declared := make([]kernel.CommandID, 0, 64)
	for _, policy := range catalog.Policies() {
		declared = append(declared, policy.Commands...)
	}
	registered := make([]kernel.CommandID, 0, len(declared))
	for _, command := range Commands() {
		if command.Label == "" || command.Scope == 0 || command.Owner == 0 {
			t.Fatalf("%s has no name or scope: %#v", command.ID, command)
		}
		owner, ok := catalog.CommandOwner(command.ID)
		if !ok || owner != command.Policy {
			t.Fatalf("%s is owned by %q, declared under %q", command.ID, owner, command.Policy)
		}
		if _, ok := catalog.Policy(command.About()); !ok {
			t.Fatalf("%s is about an unknown policy %q", command.ID, command.About())
		}
		registered = append(registered, command.ID)
	}
	slices.Sort(declared)
	slices.Sort(registered)
	if !slices.Equal(declared, registered) {
		t.Fatalf("registry %v, catalog %v", registered, declared)
	}
	for _, command := range Commands() {
		if command.Intent.Kind != IntentTable {
			continue
		}
		inTable := command.Requirements.Has(RequiresTable)
		if inTable == (command.Intent.Table == TableInsert) {
			t.Fatalf("%s table requirement is %v: %#v", command.ID, inTable, command)
		}
	}
	for _, binding := range DefaultKeymap() {
		if _, ok := CommandFor(binding.Command); !ok {
			t.Fatalf("default key for unregistered %s", binding.Command)
		}
	}
}
