package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
)

// The page wears the color of its group: a new group is in Halo, a sheet
// in a group of another hue turns the window to it, and one in a Moonveil
// group, or in the Inbox, is in Halo.
func TestTheAccentFollowsTheOpenSheetsGroup(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	now := time.Now()
	novel := app.NewGroup("", shell.Last)
	if group, _ := app.Group(novel); group.Color != shell.TintHalo {
		t.Fatalf("a new group is in %d, not Halo", group.Color)
	}
	app.StyleGroup(novel, "", shell.TintHalo+int(theme.Life))
	app.NewSheet(novel, shell.Last)
	app.followAccent(now)
	if app.Shell.Settings.Accent != theme.Life {
		t.Fatalf("a sheet in a Life group wears %v", app.Shell.Settings.Accent)
	}

	app.StyleGroup(novel, "", shell.TintMoonveil)
	app.followAccent(now)
	if app.Shell.Settings.Accent != theme.Halo {
		t.Fatalf("a sheet in a Moonveil group wears %v", app.Shell.Settings.Accent)
	}

	app.StyleGroup(novel, "", shell.TintHalo+int(theme.Regolith))
	app.NewSheet("", shell.Last)
	app.followAccent(now)
	if app.Shell.Settings.Accent != theme.Halo {
		t.Fatalf("a sheet in the Inbox wears %v", app.Shell.Settings.Accent)
	}
}
