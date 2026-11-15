package window

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/lab"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// LabHomeVariable names the folder of the lab's sample notebook. The lab
// never opens the writer's own notebook: NEVERLAND_HOME is not read.
const LabHomeVariable = "NEVERLAND_LAB_HOME"

// labHome is where the lab keeps its sample notebook: LabHomeVariable,
// or a folder of the user's cache.
func labHome() (string, error) {
	if home := os.Getenv(LabHomeVariable); home != "" {
		return filepath.Abs(home)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "Neverland Lab"), nil
}

// RunLab opens the design lab beside its sample notebook: the app as it
// is, on a notebook of its own, and a window that sets candidates on it.
func RunLab() error {
	root, err := labHome()
	if err != nil {
		return err
	}
	return run(root, seedSample, openLab)
}

// MustRunLab runs the lab, and exits with its error.
func MustRunLab() {
	if err := RunLab(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// openLab opens the lab window and hands it the app's choices.
func openLab(app *AppState) {
	l := &lab.Lab{
		Choice:   app.labChoice(),
		Apply:    app.applyLab,
		Settings: func() theme.Settings { return app.Shell.Settings },
		Colors:   &app.colors,
	}
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:                "Neverland Lab",
		Width:                860,
		Height:               820,
		MinWidth:             560,
		MinHeight:            420,
		TitleBarStyle:        mygo.TitleBarHidden,
		TitleBarHeight:       40,
		TrafficLightPosition: trafficLights(),
		StateKey:             "neverland.lab",
		Content:              ui.View(l.View),
		BackgroundColor:      colorHex(app.Shell.Settings.Semantic().Surface.Background),
	})
	window.OnClosed(func() { app.labWindow = nil })
	app.labWindow = window
}

// labChoice is what the app paints with now, as the lab says it.
func (a *AppState) labChoice() lab.Choice {
	choice := lab.Choice{Appearance: a.Shell.Settings.Appearance, Accent: theme.Halo, FollowGroup: a.accent == nil}
	if a.accent != nil {
		choice.Accent = *a.accent
	}
	for i, scheme := range lab.Schemes() {
		if scheme == a.scheme || a.scheme == nil && scheme == theme.Cosmofi() {
			choice.Scheme = i
		}
	}
	return choice
}

// applyLab paints the app with the lab's choice, every window blending
// to it as a theme change does.
func (a *AppState) applyLab(choice lab.Choice) {
	if err := a.setLab(choice); err != nil {
		a.fail("Couldn't change the appearance", err)
		return
	}
	a.applyAppearance()
	a.refreshSettingsData()
}

// setLab takes the lab's choice as what the windows paint with. The
// appearance is the product's own setting, kept with the sample notebook.
func (a *AppState) setLab(choice lab.Choice) error {
	schemes := lab.Schemes()
	a.scheme = schemes[max(0, min(choice.Scheme, len(schemes)-1))]
	a.accent = nil
	if !choice.FollowGroup {
		accent := choice.Accent
		a.accent = &accent
	}
	if choice.Appearance == a.Shell.Settings.Appearance {
		return nil
	}
	next := appsettings.Starfield
	if choice.Appearance == theme.Daybreak {
		next = appsettings.Daybreak
	}
	if _, err := a.Product.Apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
		Kind: appsettings.SetTheme, Theme: uint8(next),
	})); err != nil {
		return err
	}
	a.settingsChanged()
	return nil
}

// sampleGroup is a group of the sample notebook and the sheets it holds,
// each typed as a writer would: a heading, then its lines.
type sampleGroup struct {
	name, icon string
	accent     theme.Accent
	sheets     [][]string
}

var sample = []sampleGroup{
	{"The Tidewater Novel", "feather", theme.Life, [][]string{
		{"# Chapter 1 — Low Water", "#draft #novel She walked out past the last post, where the mud began to sing.",
			"The heron did not move. It had been standing there since before the tide turned, and it would stand there after."},
		{"# Chapter 2 — The Weir", "#draft #novel Nobody remembered who built the weir.",
			"Tomas counted the stakes twice and got a different number each time."},
		{"# Characters", "#novel Ines, the keeper of the lower sluice. Tomas, the boy who counts birds."},
	}},
	{"Research", "compass", theme.Regolith, [][]string{
		{"# Salt marsh ecology", "#research Cordgrass holds the bank; its roots trap the silt the tide brings in.",
			"Notes on sediment, on roots, and on what the water takes back."},
		{"# Tide tables 2026", "#research #data Spring tides fall near the new and the full moon."},
	}},
	{"Journal", "moon", theme.Halo, [][]string{
		{"# Morning field notes", "#journal The marsh was quiet before the tide turned."},
		{"# A long walk", "#journal Out to the point and back by the dyke, eleven kilometres."},
	}},
}

// seedSample fills a new sample notebook, one without groups of its own,
// by typing it into the app's pages through the editor, so its pages are
// what writing makes. No line ends on a tag: the tag's completion would
// take the line's end. A notebook already seeded is left as it is.
func seedSample(app *AppState) error {
	for _, group := range app.Groups("") {
		if !group.Inbox {
			return nil
		}
	}
	// The Inbox's thought goes into the sheet a new notebook opens on.
	if err := typeSample(app, []string{"# Untitled thought", "Something about how water forgets."}); err != nil {
		return err
	}
	for index, g := range sample {
		id := app.NewGroup("", index)
		app.RenameGroup(id, g.name)
		app.StyleGroup(id, g.icon, shell.TintHalo+int(g.accent))
		for at, lines := range g.sheets {
			app.NewSheet(id, at)
			if err := typeSample(app, lines); err != nil {
				return err
			}
		}
	}
	// Seeding opened a tab for each sheet; the notebook opens on the last.
	if active, ok := app.Workspace.Active(); ok {
		app.closeTabsWhere(sheetName(active), func(at, kept int) bool { return at != kept })
	}
	app.save()
	return nil
}

// typeSample types lines into the page in front, a line a paragraph.
func typeSample(app *AppState, lines []string) error {
	el := app.pageElement()
	if el == nil {
		return fmt.Errorf("lab: the sample sheet did not open")
	}
	now := time.Now()
	for i, line := range lines {
		if i > 0 {
			el.Editor.Do(vocab.Break{}, now)
		}
		for _, r := range line {
			el.Editor.Do(vocab.Type{Text: string(r)}, now)
		}
	}
	app.summarize()
	app.save()
	return nil
}
