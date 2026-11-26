package settings

import (
	"slices"
	"testing"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// Found by no words, the settings are all their sections, Appearance's
// first; drawn, nothing changes until a control is used.
func TestTheSectionsStandInOrder(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	sections := state.Sections("")
	if len(sections) < 3 || sections[0].Category != appsettings.Appearance ||
		sections[len(sections)-1].Category != appsettings.Keymap {
		t.Fatalf("sections = %#v", sections)
	}
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1200)
	tester.Frame()
	for _, text := range []string{"Typewriter mode", "Theme", "Animations", "Appearance"} {
		if !tester.HasText(text) {
			t.Fatalf("no %q in %#v", text, tester.Texts())
		}
	}
	if len(changes) != 0 {
		t.Fatalf("changes = %#v", changes)
	}
}

// Words find the sections holding them, and in a section the rows.
func TestWordsFindSectionsAndRows(t *testing.T) {
	state, _ := keymapState()
	state.Query = "close"
	titles := []string{}
	for _, sec := range state.Sections(state.Query) {
		titles = append(titles, sec.Title)
	}
	if !slices.Contains(titles, "Pages") || slices.Contains(titles, "Window") {
		t.Fatalf("sections = %v", titles)
	}
	tester := ui.NewTester(state.cards(), 600, 1200)
	tester.Frame()
	if !tester.HasText("Close sheet") || tester.HasText("Toggle Typewriter mode") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}

func TestNotebookSectionAndBackupsPopover(t *testing.T) {
	state := New()
	state.Animate = false
	state.Data = appsettings.DefaultState().Data()
	state.NotebookRoot = "/Users/example/Documents/Neverland"
	state.Backups = []BackupOption{
		{Path: "/tmp/b1.db", Date: "Nov 26, 2026, 9:00 AM", Size: "42 KB"},
	}
	var revealed, changed bool
	var restored BackupOption
	state.RevealNotebook = func() { revealed = true }
	state.ChangeNotebook = func() { changed = true }
	state.RestoreBackup = func(opt BackupOption) { restored = opt }

	tester := ui.NewTester(state.cards(appsettings.Appearance), 640, 2000)
	tester.Frame()
	for _, want := range []string{locales.NotebookLocation(), state.NotebookRoot, locales.NotebookBackups(), locales.NotebookBackupsCount(1)} {
		if !tester.HasText(want) {
			t.Fatalf("missing %q in settings texts: %#v", want, tester.Texts())
		}
	}
	if err := tester.Click(locales.ShowInFinder()); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click(locales.ChooseFolderEllipsis()); err != nil {
		t.Fatal(err)
	}
	if !revealed || !changed {
		t.Fatalf("revealed=%v changed=%v, want both true", revealed, changed)
	}
	if err := tester.Click(locales.RestoreBackupEllipsis()); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if err := tester.Click("Nov 26, 2026, 9:00 AM"); err != nil {
		t.Fatal(err)
	}
	if restored.Path != "/tmp/b1.db" {
		t.Fatalf("restored = %+v, want /tmp/b1.db", restored)
	}

	state.PendingRoot = "/Users/example/Archive/Neverland"
	pendingTester := ui.NewTester(state.cards(appsettings.Appearance), 640, 2000)
	pendingTester.Frame()
	if !pendingTester.HasText(state.PendingRoot) || !pendingTester.HasText(locales.NotebookChangedDescription()) {
		t.Fatalf("missing pending relocation feedback: %#v", pendingTester.Texts())
	}
}
