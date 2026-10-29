package settings

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

func TestPolicyAndOptionTogglesPublishValidatedConfiguration(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	state := DefaultState()
	session := activeSession()

	pairsKey := kernel.NewSettingKey(editor.EditorPolicyPairs.ID(), "overtype")
	before := store.Configuration()
	outcome, err := state.Apply(
		ToggleBoolean(PolicyBoolean(editor.EditorPolicyPairs.ID()), false),
		store,
		session,
	)
	if err != nil || !outcome.Applied || !outcome.Changed || !outcome.PolicyChanged {
		t.Fatalf("toggle outcome = %#v, %v", outcome, err)
	}
	configuration := store.Configuration()
	if configuration.IsEnabled(editor.EditorPolicyPairs.ID()) {
		t.Fatal("pairs policy remained enabled")
	}
	beforeEnabled := before.IsEnabled(editor.EditorPolicyPairs.ID())
	if !beforeEnabled {
		t.Fatal("source candidate mutated")
	}
	outcome, err = state.Apply(
		ToggleBoolean(OptionBoolean(pairsKey), false),
		store,
		session,
	)
	if err != nil || !outcome.Applied || !outcome.Changed {
		t.Fatalf("option outcome = %#v, %v", outcome, err)
	}
	value, ok := store.PreparedConfiguration().Setting(pairsKey)
	boolean, _ := value.Bool()
	if !ok || boolean {
		t.Fatalf("overtype setting = %#v, %v", value, ok)
	}
}

func TestShortcutRemapUnbindAndResetAreAtomic(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	state := DefaultState()
	session := activeSession()
	command := editor.TextSelectAll.ID()
	target := kernel.NewBindingTarget(
		kernel.EditorBindingContext,
		"e",
		kernel.ModifiersPrimary|kernel.ModifiersAlt,
	)

	outcome, err := state.Apply(Remap(command, target), store, session)
	if err != nil || !outcome.Applied || !outcome.PolicyChanged {
		t.Fatalf("remap = %#v, %v", outcome, err)
	}
	if bindings := store.PreparedConfiguration().BindingsFor(command); len(bindings) != 1 ||
		bindings[0] != target {
		t.Fatalf("bindings = %#v", bindings)
	}

	source := store.Configuration()
	conflict := store.PreparedConfiguration().BindingsFor(editor.TextDocumentStart.ID())[0]
	if _, err := state.Apply(Remap(command, conflict), store, session); err == nil {
		t.Fatal("conflicting remap was accepted")
	}
	if !store.Configuration().Equal(source) {
		t.Fatal("failed remap changed configuration")
	}

	if _, err := state.Apply(Unbind(command), store, session); err != nil {
		t.Fatal(err)
	}
	if bindings := store.PreparedConfiguration().BindingsFor(command); len(bindings) != 0 {
		t.Fatalf("unbound = %#v", bindings)
	}
	configuration := store.Configuration()
	if _, overridden := configuration.BindingOverride(command); !overridden {
		t.Fatal("explicit unbind not retained")
	}
	if _, err := state.Apply(ResetBinding(command), store, session); err != nil {
		t.Fatal(err)
	}
	if bindings := store.PreparedConfiguration().BindingsFor(command); len(bindings) != 1 {
		t.Fatalf("reset bindings = %#v", bindings)
	}
	configuration = store.Configuration()
	if _, overridden := configuration.BindingOverride(command); overridden {
		t.Fatal("reset left override")
	}
}

func TestInvalidPolicyChangePreservesConfigurationAndState(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	state := DefaultState()
	session := activeSession()
	beforeState := state
	before := store.Configuration()

	_, err := state.Apply(
		ToggleBoolean(OptionBoolean(kernel.NewSettingKey("missing.policy", "x")), true),
		store,
		session,
	)
	if err == nil {
		t.Fatal("unknown option accepted")
	}
	if !store.Configuration().Equal(before) {
		t.Fatal("invalid change altered policy configuration")
	}
	if state != beforeState {
		t.Fatal("invalid change altered local settings state")
	}

	_, err = state.Apply(Change{Kind: ChangeBinding, Command: "missing.command"}, store, session)
	if err == nil {
		t.Fatal("unknown command accepted")
	}
	if !store.Configuration().Equal(before) {
		t.Fatal("failed command changed configuration")
	}
}

func TestEditorChangesRequireAnActivePage(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	state := DefaultState()
	before := store.Configuration()
	session := &fakeSession{active: false}

	_, err := state.Apply(
		ToggleBoolean(PolicyBoolean(editor.EditorPolicyPairs.ID()), false),
		store,
		session,
	)
	if !errors.Is(err, ErrPageRequired) {
		t.Fatalf("error = %v", err)
	}
	if !store.Configuration().Equal(before) {
		t.Fatal("closed page changed configuration")
	}
	outcome, err := state.Apply(
		ToggleBoolean(PresentationBoolean(PresentationZen), true),
		store,
		session,
	)
	if err != nil || !outcome.Applied || !state.Presentation.Zen {
		t.Fatalf("presentation = %#v, %#v, %v", outcome, state.Presentation, err)
	}
}

func TestPresentationAndAppearanceChangesAreLocalAndBounded(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	session := activeSession()
	state := DefaultState()
	before := store.Configuration()

	for _, change := range []Change{
		ToggleBoolean(PresentationBoolean(PresentationMotion), false),
		ToggleBoolean(PresentationBoolean(PresentationTypewriter), true),
		ToggleBoolean(PresentationBoolean(PresentationZen), true),
		NewAppearanceChange(AppearanceChange{Kind: SetTheme, Theme: uint8(Daybreak)}),
		NewAppearanceChange(AppearanceChange{Kind: SetAccent, Accent: uint8(Life)}),
		NewAppearanceChange(AppearanceChange{Kind: SetUIFontSize, FontSize: 99}),
		NewAppearanceChange(AppearanceChange{Kind: SetProseFontSize, FontSize: 2}),
	} {
		if _, err := state.Apply(change, store, session); err != nil {
			t.Fatalf("change %#v: %v", change, err)
		}
	}
	if state.Presentation.Motion || !state.Presentation.Typewriter || !state.Presentation.Zen {
		t.Fatalf("presentation = %#v", state.Presentation)
	}
	if state.Appearance.Appearance != Daybreak ||
		state.Appearance.Accent != Life ||
		state.Appearance.UIFontSize != MaxUIFontSize ||
		state.Appearance.ProseFontSize != MinProseFontSize {
		t.Fatalf("appearance = %#v", state.Appearance)
	}
	if !store.Configuration().Equal(before) {
		t.Fatal("presentation/appearance changed policy configuration")
	}

	snapshot := state
	for _, change := range []Change{
		NewAppearanceChange(AppearanceChange{Kind: SetTheme, Theme: 255}),
		NewAppearanceChange(AppearanceChange{Kind: 255}),
	} {
		if _, err := state.Apply(change, store, session); !errors.Is(err, ErrInvalidAppearance) {
			t.Fatalf("invalid appearance error = %v", err)
		}
		if state != snapshot {
			t.Fatal("invalid appearance changed state")
		}
	}
}

func TestBuildDataSnapshotsSectionsShortcutsDiagnosticsAndAppearance(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	state := DefaultState()
	session := activeSession()

	data := state.Data(store, session)
	if data.PageID == nil || *data.PageID != 1 || data.Page != "Research notes" {
		t.Fatalf("page data = %#v", data)
	}
	if len(data.Sections) == 0 || len(data.Shortcuts) == 0 || len(data.Appearance) != 3 {
		t.Fatalf("data sizes = %d/%d/%d", len(data.Sections), len(data.Shortcuts), len(data.Appearance))
	}
	if data.Activity[0] != "Revision 4  ·  9 characters  ·  2 marks" ||
		data.Activity[1] != "Revision 3  ·  8 characters  ·  1 marks" {
		t.Fatalf("activity = %#v", data.Activity)
	}
	if len(data.Scopes) != 1 || data.Scopes[0] != "Selection 2 → 4" {
		t.Fatalf("scopes = %#v", data.Scopes)
	}
	pairsSelector := "policy-editor.pairs"
	foundPairs := false
	foundCJKLabel := false
	for _, section := range data.Sections {
		for _, setting := range section.Settings {
			if setting.Selector == pairsSelector {
				foundPairs = setting.Value && setting.Enabled
			}
			if setting.Selector == "policy-editor.cjk" {
				foundCJKLabel = setting.Label == "Enable CJK refinement"
			}
		}
	}
	if !foundPairs || !foundCJKLabel {
		t.Fatalf("pairs toggle=%v CJK label=%v", foundPairs, foundCJKLabel)
	}
	foundText := false
	for _, shortcut := range data.Shortcuts {
		if shortcut.Command == editor.TextSelectAll.ID() {
			foundText = shortcut.Label == "Select all" && shortcut.Enabled
		}
	}
	if !foundText {
		t.Fatal("select-all shortcut missing")
	}
	if data.PolicyConfiguration.IsEnabled(editor.EditorPolicyPairs.ID()) == false {
		t.Fatal("data did not own source configuration")
	}
}

func TestBuildDataMarksClosedPageChangesUnavailable(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	state := DefaultState()
	data := state.Data(store, &fakeSession{title: "Notebook", active: false})
	if data.PageID != nil {
		t.Fatalf("closed page id = %v", *data.PageID)
	}
	if data.Page != "Notebook" {
		t.Fatalf("page title = %q", data.Page)
	}
	for _, section := range data.Sections {
		for _, setting := range section.Settings {
			if setting.Enabled {
				t.Fatalf("closed setting enabled: %#v", setting)
			}
		}
	}
	for _, shortcut := range data.Shortcuts {
		if shortcut.Enabled {
			t.Fatalf("closed shortcut enabled: %#v", shortcut)
		}
	}
}

func TestRemapReplacingMovesTakenShortcutAtomically(t *testing.T) {
	t.Parallel()
	catalog := editorCatalog(t)
	store := editorHost(t, catalog)
	state := DefaultState()
	session := activeSession()
	command := editor.TextSelectAll.ID()
	holder := editor.TextDocumentStart.ID()
	taken := store.PreparedConfiguration().BindingsFor(holder)[0]

	data := BuildData(store, session, state.Presentation, state.Appearance)
	owner, ok := data.Owner(taken)
	if !ok || owner.Command != holder {
		t.Fatalf("owner of %s = %#v, %v", taken, owner, ok)
	}
	if _, err := state.Apply(RemapReplacing(command, taken, holder), store, session); err != nil {
		t.Fatal(err)
	}
	prepared := store.PreparedConfiguration()
	if bindings := prepared.BindingsFor(command); len(bindings) != 1 || bindings[0] != taken {
		t.Fatalf("reassigned bindings = %#v", bindings)
	}
	if bindings := prepared.BindingsFor(holder); len(bindings) != 0 {
		t.Fatalf("displaced command kept its shortcut: %#v", bindings)
	}
	if got, ok := prepared.CommandFor(taken); !ok || got != command {
		t.Fatalf("shortcut now runs %q", got)
	}
}
