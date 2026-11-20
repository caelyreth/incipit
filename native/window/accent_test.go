package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
)

// The window wears the color of the sheet in front's group: a new group is
// in Halo, a sheet in a group of another hue turns the window to it, and
// one in a Moonveil group, or in the Inbox, is in Halo.
func TestTheAccentFollowsTheOpenSheetsGroup(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	now := time.Now()
	novel := app.NewGroup("", shell.Last)
	if group, _ := app.Group(novel); group.Color != shell.TintHalo {
		t.Fatalf("A new group should be in Halo, not %d.", group.Color)
	}
	app.StyleGroup(novel, "", shell.TintHalo+int(theme.Life))
	app.NewSheet(novel, shell.Last)
	app.followAccent(now)
	if app.Shell.Settings.Accent != theme.Life {
		t.Fatalf("A sheet in a Life group should wear Life, not %v.", app.Shell.Settings.Accent)
	}

	app.StyleGroup(novel, "", shell.TintMoonveil)
	app.followAccent(now)
	if app.Shell.Settings.Accent != theme.Halo {
		t.Fatalf("A sheet in a Moonveil group should wear Halo, not %v.", app.Shell.Settings.Accent)
	}

	app.StyleGroup(novel, "", shell.TintHalo+int(theme.Regolith))
	app.NewSheet("", shell.Last)
	app.followAccent(now)
	if app.Shell.Settings.Accent != theme.Halo {
		t.Fatalf("A sheet in the Inbox should wear Halo, not %v.", app.Shell.Settings.Accent)
	}
}
