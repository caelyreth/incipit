package window

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/ui/shell"
	"github.com/caelyreth/incipit/internal/ui/theme"
)

// Global shell chrome stays in Halo; each open sheet's editor wears its
// group's hue independently (Halo for new/Moonveil/Inbox sheets).
func TestTheAccentFollowsTheOpenSheetsGroup(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	now := time.Now()
	novel := app.NewGroup("", shell.Last)
	if group, _ := app.Group(novel); group.Color != shell.TintHalo {
		t.Fatalf("A new group should be in Halo, not %d.", group.Color)
	}
	app.StyleGroup(novel, "", shell.TintHalo+int(theme.Life))
	app.NewSheet(novel, shell.Last)
	lifeID := app.Shell.ActivePage
	lifeSess, ok := app.sessionOf(lifeID)
	if !ok {
		t.Fatal("expected session for new Life sheet")
	}
	if app.Shell.Settings.Accent != theme.Halo {
		t.Fatalf("Chrome/shell accent should stay Halo, got %v.", app.Shell.Settings.Accent)
	}
	if got, _ := app.sheetTheme(lifeSess, lifeID, now); got.AccentFamily != theme.Life {
		t.Fatalf("A sheet in a Life group should wear Life, not %v.", got.AccentFamily)
	}

	app.StyleGroup(novel, "", shell.TintMoonveil)
	if got, _ := app.sheetTheme(lifeSess, lifeID, now.Add(3*theme.ThemeTransition)); got.AccentFamily != theme.Halo {
		t.Fatalf("A sheet in a Moonveil group should settle to Halo, not %v.", got.AccentFamily)
	}

	app.StyleGroup(novel, "", shell.TintHalo+int(theme.Regolith))
	app.NewSheet("", shell.Last)
	inboxID := app.Shell.ActivePage
	inboxSess, ok := app.sessionOf(inboxID)
	if !ok {
		t.Fatal("expected session for Inbox sheet")
	}
	if app.Shell.Settings.Accent != theme.Halo {
		t.Fatalf("Chrome/shell accent should stay Halo with mixed sheets, got %v.", app.Shell.Settings.Accent)
	}
	if got, _ := app.sheetTheme(inboxSess, inboxID, now.Add(4*theme.ThemeTransition)); got.AccentFamily != theme.Halo {
		t.Fatalf("An Inbox sheet should wear Halo, not %v.", got.AccentFamily)
	}
	if got, _ := app.sheetTheme(lifeSess, lifeID, now.Add(4*theme.ThemeTransition)); got.AccentFamily != theme.Regolith {
		t.Fatalf("The Regolith sheet should wear Regolith independently, not %v.", got.AccentFamily)
	}
}
