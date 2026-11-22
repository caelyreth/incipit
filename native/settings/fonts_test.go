package settings

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/fonts"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

// TestWritingSystemsChooseAFace names the face chosen on the picker, which
// offers the system's fonts and each face, the size of one not kept
// beside it, lists under it the faces held with what can be done with
// each, and tells the host what is asked.
func TestWritingSystemsChooseAFace(t *testing.T) {
	face := func(id string) fonts.Face { f, _ := fonts.Lookup(id); return f }
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Query = "writing systems"
	state.CJKFace = "sc"
	state.Fonts = []FontRow{
		{Face: face("sc"), Kept: true},
		{Face: face("tc"), Coming: true, Done: 1, Total: 4, Wanted: true},
		{Face: face("hk")},
		{Face: face("jp"), Partial: 3},
	}
	type asked struct {
		id  string
		act FontAct
	}
	var acts []asked
	state.Font = func(id string, act FontAct) { acts = append(acts, asked{id, act}) }
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1600)
	tester.Frame()
	for _, text := range []string{"Chinese and Japanese", "Noto Serif SC", "Noto Serif TC", "Noto Serif JP"} {
		if !tester.HasText(text) {
			t.Fatalf("no %q in %v", text, tester.Texts())
		}
	}
	if tester.HasText("Noto Serif HK") {
		t.Error("a face neither kept nor coming is listed under the picker")
	}
	if err := tester.Click("Noto Serif SC"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if menu := tester.Menu(); !slices.Equal(menu, []string{systemFont, "-", "Noto Serif SC", "Noto Serif TC", "Noto Serif HK", "Noto Serif JP"}) {
		t.Fatalf("the picker offers %v", menu)
	}
	if err := tester.ChooseMenuItem("Noto Serif HK"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, label := range []string{"Stop", "Resume", "Remove"} {
		if err := tester.Click(label); err != nil {
			t.Fatalf("%s: %v in %v", label, err, tester.Texts())
		}
		tester.Frame()
	}
	want := []asked{{"hk", FontChoose}, {"tc", FontCancel}, {"jp", FontChoose}, {"sc", FontRemove}}
	if !slices.Equal(acts, want) {
		t.Errorf("asked %v, want %v", acts, want)
	}
}

// TestWritingSystemsWithNoFace draws Chinese and Japanese in the system's
// fonts, nothing held under the picker.
func TestWritingSystemsWithNoFace(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Query = "writing systems"
	for _, f := range fonts.Catalog {
		state.Fonts = append(state.Fonts, FontRow{Face: f})
	}
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1600)
	tester.Frame()
	if !tester.HasText(systemFont) || tester.HasText("Remove") {
		t.Errorf("with no face: %v", tester.Texts())
	}
}
