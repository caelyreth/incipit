package settings

import (
	"slices"
	"testing"

	"github.com/caelyreth/incipit/internal/app/fonts"
	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	"github.com/egoist/mygo/ui"
)

// typefaceState is the section's state with every face of the catalog,
// as rows says of some.
func typefaceState(values appsettings.AppearanceSettings, rows map[string]FontRow) *State {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Data.AppearanceValues = values
	state.Query = "typeface"
	for _, f := range fonts.Catalog {
		row := rows[f.ID]
		row.Face = f
		state.Fonts = append(state.Fonts, row)
	}
	return state
}

type asked struct {
	id  string
	act FontAct
}

// TestTypefaceChoosesGlyphForms lists the system's fonts and each
// language as options, each with its face's state in the style chosen,
// the faces held of the other style after them, and tells the host what
// is asked.
func TestTypefaceChoosesGlyphForms(t *testing.T) {
	state := typefaceState(appsettings.AppearanceSettings{Typeface: fonts.Serif, CJKForms: "sc"}, map[string]FontRow{
		"serif-sc": {Kept: true},
		"serif-tc": {Coming: true, Done: 1e6, Total: 4e6, Wanted: true},
		"sans-jp":  {Partial: 3},
	})
	var acts []asked
	state.Font = func(id string, act FontAct) { acts = append(acts, asked{id, act}) }
	tester := ui.NewTester(state.cards(appsettings.Appearance), 700, 2000)
	tester.Frame()
	for _, text := range []string{"System Font", "Chinese, Simplified", "Chinese, Hong Kong", "Japanese", "Also on This Computer",
		"Noto Serif SC · 29.6 MB · Downloaded", "Noto Serif TC · 1.0 MB of 4.0 MB", "Noto Sans JP"} {
		if !tester.HasText(text) {
			t.Fatalf("no %q in %v", text, tester.Texts())
		}
	}
	for _, label := range []string{"Japanese", "Stop", "Remove"} {
		if err := tester.Click(label); err != nil {
			t.Fatalf("%s: %v in %v", label, err, tester.Texts())
		}
		tester.Frame()
	}
	want := []asked{{"serif-jp", FontChoose}, {"serif-tc", FontCancel}, {"serif-sc", FontRemove}}
	if !slices.Equal(acts, want) {
		t.Errorf("asked %v, want %v", acts, want)
	}
}

// TestTypefaceStyleCarriesTheForms sets the style and asks for the face of
// the glyph forms chosen in it.
func TestTypefaceStyleCarriesTheForms(t *testing.T) {
	state := typefaceState(appsettings.AppearanceSettings{Typeface: fonts.Serif, CJKForms: "jp"}, map[string]FontRow{
		"serif-jp": {Kept: true},
	})
	var acts []asked
	state.Font = func(id string, act FontAct) { acts = append(acts, asked{id, act}) }
	var changes []appsettings.Change
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1600)
	tester.Frame()
	if err := tester.Click("Sans"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if len(changes) != 1 || changes[0].Appearance.Kind != appsettings.SetTypeface || changes[0].Appearance.Typeface != fonts.Sans {
		t.Errorf("changed %+v", changes)
	}
	if !slices.Equal(acts, []asked{{"sans-jp", FontChoose}}) {
		t.Errorf("asked %v", acts)
	}
}

// TestTypefaceWithNoFace draws Chinese and Japanese in the system's fonts,
// nothing held under the pickers.
func TestTypefaceWithNoFace(t *testing.T) {
	state := typefaceState(appsettings.AppearanceSettings{}, nil)
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1800)
	tester.Frame()
	if !tester.HasText("System Font") || tester.HasText("Remove") {
		t.Errorf("with no face: %v", tester.Texts())
	}
}

func TestTypefaceChoosesAndCarriesLatinNoto(t *testing.T) {
	state := typefaceState(appsettings.AppearanceSettings{Typeface: fonts.Serif, ProseFont: "Noto Serif"}, map[string]FontRow{
		"serif-latin": {Kept: true},
		"sans-latin":  {Kept: false},
	})
	var acts []asked
	state.Font = func(id string, act FontAct) { acts = append(acts, asked{id, act}) }
	var changes []appsettings.Change
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.cards(appsettings.Appearance), 700, 2000)
	tester.Frame()
	if !tester.HasText("Noto for Latin") || !tester.HasText("Noto Serif · 3.7 MB · Downloaded") {
		t.Fatalf("missing Latin Noto row in %v", tester.Texts())
	}
	if err := tester.Click("Sans"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if len(changes) != 1 || changes[0].Appearance.Kind != appsettings.SetTypeface || changes[0].Appearance.Typeface != fonts.Sans {
		t.Fatalf("changed %+v", changes)
	}
	if !slices.Equal(acts, []asked{{"sans-latin", FontChoose}}) {
		t.Errorf("asked %v, want sans-latin choose", acts)
	}
}

func TestFontPickerFiltersAndSelectsSystemFont(t *testing.T) {
	state := typefaceState(appsettings.AppearanceSettings{Typeface: fonts.Serif}, nil)
	state.SystemFonts = []ui.FontFamily{
		{Name: "Georgia"},
		{Name: "JetBrains Mono", Monospaced: true},
		{Name: "Palatino"},
	}
	var changes []appsettings.Change
	state.Apply = func(change appsettings.Change) {
		changes = append(changes, change)
		state.Data.AppearanceValues.Apply(change.Appearance)
	}
	tester := ui.NewTester(state.cards(appsettings.Appearance), 700, 2000)
	tester.Frame()
	if err := tester.Click("System Default"); err != nil {
		t.Fatalf("open font picker: %v", err)
	}
	tester.Frame()
	if !tester.HasText("Georgia") || !tester.HasText("JetBrains Mono") || !tester.HasText("Palatino") {
		t.Fatalf("picker did not list system fonts: %v", tester.Texts())
	}
	tester.Type("JetBrains")
	tester.Frame()
	if tester.HasText("Georgia") || !tester.HasText("JetBrains Mono") {
		t.Fatalf("picker filter failed: %v", tester.Texts())
	}
	if err := tester.Click("JetBrains Mono"); err != nil {
		t.Fatalf("select filtered font: %v", err)
	}
	tester.Frame()
	if len(changes) != 1 || changes[0].Appearance.Kind != appsettings.SetProseFont || changes[0].Appearance.Font != "JetBrains Mono" {
		t.Fatalf("changes = %+v", changes)
	}
	if !tester.HasText("JetBrains Mono") {
		t.Fatalf("trigger button did not update: %v", tester.Texts())
	}
	if err := tester.Click("Reset to Default"); err != nil {
		t.Fatalf("reset custom font: %v", err)
	}
	tester.Frame()
	if len(changes) != 2 || changes[1].Appearance.Kind != appsettings.SetProseFont || changes[1].Appearance.Font != "" {
		t.Fatalf("reset changes = %+v", changes)
	}
}
