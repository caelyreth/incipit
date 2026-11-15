package window

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/lab"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The lab's notebook is seeded once, by typing, and opens on one sheet.
func TestTheLabSeedsItsNotebookOnce(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	if err := seedSample(app); err != nil {
		t.Fatal(err)
	}
	var names []string
	sheets := 0
	for _, group := range app.Groups("") {
		names = append(names, group.Name)
		sheets += app.SheetCount(shell.Place{Kind: shell.PlaceGroup, Group: group.ID})
	}
	if !slices.Contains(names, "The Tidewater Novel") || len(names) != 4 || sheets != 8 {
		t.Fatalf("groups %v, %d sheets", names, sheets)
	}
	if tabs := app.Workspace.Tabs(); len(tabs) != 1 {
		t.Fatalf("%d tabs open", len(tabs))
	}
	var titles []string
	for _, sheet := range app.Sheets(shell.Place{Kind: shell.PlaceAll}) {
		titles = append(titles, sheet.Title)
	}
	if !slices.Contains(titles, "Chapter 1 — Low Water") || !slices.Contains(titles, "Untitled thought") {
		t.Fatalf("titles %v", titles)
	}
	if err := seedSample(app); err != nil {
		t.Fatal(err)
	}
	if again := app.SheetCount(shell.Place{Kind: shell.PlaceAll}); again != 8 {
		t.Fatalf("seeding again made %d sheets", again)
	}
}

// The lab's choice is what the windows paint with: the scheme, the one
// accent or the group's, and the appearance.
func TestTheLabSetsWhatTheWindowsPaintWith(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	quiet := lab.Schemes()[1]
	if err := app.setLab(lab.Choice{Scheme: 1, Appearance: theme.Daybreak, Accent: theme.Unripe}); err != nil {
		t.Fatal(err)
	}
	settings := app.themeSettings()
	if settings.Scheme != quiet || settings.Accent != theme.Unripe || settings.Appearance != theme.Daybreak {
		t.Fatalf("the windows paint with %+v", settings)
	}
	if got := settings.Semantic().Surface.Page; got != quiet.Daybreak.Page {
		t.Fatalf("the page is %v", got)
	}
	if choice := app.labChoice(); choice.Scheme != 1 || choice.FollowGroup || choice.Accent != theme.Unripe {
		t.Fatalf("the lab reads back %+v", choice)
	}
	if err := app.setLab(lab.Choice{Scheme: 0, Appearance: theme.Daybreak, FollowGroup: true}); err != nil {
		t.Fatal(err)
	}
	if settings := app.themeSettings(); settings.Scheme != theme.Cosmofi() || settings.Accent != theme.Halo {
		t.Fatalf("back on Cosmofi the windows paint with %+v", settings)
	}
}

// Each scheme is rendered on the sample notebook in both appearances,
// and its color read as the budget counts it, pane by pane. The renders
// are kept with NEVERLAND_SHOTS.
func TestTheLabReadsEachSchemesColor(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	if err := seedSample(app); err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1440, 900)
	panes := []struct {
		name string
		r    image.Rectangle
	}{
		{"library", image.Rect(0, 0, 232, 900)},
		{"list", image.Rect(232, 0, 520, 900)},
		{"page", image.Rect(520, 0, 1440, 900)},
	}
	for i, scheme := range lab.Schemes() {
		for _, appearance := range []theme.Appearance{theme.Starfield, theme.Daybreak} {
			if err := app.setLab(lab.Choice{Scheme: i, Appearance: appearance, FollowGroup: true}); err != nil {
				t.Fatal(err)
			}
			app.Shell.Settings = app.themeSettings()
			app.colors.Jump(app.Shell.Settings.Semantic())
			tester.Frame()
			tester.Advance(time.Second)
			name := "lab-" + scheme.Name + "-starfield"
			if appearance == theme.Daybreak {
				name = "lab-" + scheme.Name + "-daybreak"
			}
			shot(t, tester, name)
			for _, pane := range panes {
				r := lab.Measure(tester.Image(), pane.r)
				t.Logf("%-24s %-8s %5.2f%% colored, hues %v", name, pane.name, r.Area*100, r.Hues)
			}
		}
	}
	l := &lab.Lab{Choice: app.labChoice(), Settings: func() theme.Settings { return app.Shell.Settings }, Colors: &app.colors}
	window := ui.NewTester(l.View, 860, 1100)
	window.Frame()
	window.Advance(time.Second)
	shot(t, window, "lab-window")
}
