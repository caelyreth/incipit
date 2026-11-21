package settings

import (
	"testing"

	"github.com/caelyreth/neverland/app/fonts"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

// TestWritingSystemsAskWhatTheyDo lists each face with what can be done
// with it, chooses among the kept Han faces only when there are two, and
// tells the host what a row asks.
func TestWritingSystemsAskWhatTheyDo(t *testing.T) {
	face := func(id string) fonts.Face { f, _ := fonts.Lookup(id); return f }
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Query = "writing systems"
	state.Fonts = []FontRow{
		{Face: face("sc"), Kept: true},
		{Face: face("jp")},
		{Face: face("thai"), Coming: true, Done: 1, Total: 4},
	}
	type asked struct {
		id  string
		act FontAct
	}
	var acts []asked
	state.Font = func(id string, act FontAct) { acts = append(acts, asked{id, act}) }
	var changes []appsettings.Change
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1600)
	tester.Frame()
	if tester.HasText("Han Characters") {
		t.Error("one Han face kept, yet a choice among them")
	}
	for _, label := range []string{"Remove", "Download", "Stop"} {
		if err := tester.Click(label); err != nil {
			t.Fatalf("%s: %v in %v", label, err, tester.Texts())
		}
		tester.Frame()
	}
	want := []asked{{"sc", FontRemove}, {"jp", FontDownload}, {"thai", FontCancel}}
	if len(acts) != len(want) || acts[0] != want[0] || acts[1] != want[1] || acts[2] != want[2] {
		t.Errorf("asked %v, want %v", acts, want)
	}
	state.Fonts[1].Kept = true
	tester.Frame()
	if err := tester.Click("Japanese"); err != nil {
		t.Fatalf("no Han choice with two kept: %v", err)
	}
	if len(changes) != 1 || changes[0].Appearance.Kind != appsettings.SetHanFace || changes[0].Appearance.Face != "jp" {
		t.Errorf("chose %+v", changes)
	}
}
