package settings

import (
	"runtime"
	"testing"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/egoist/mygo/ui"
)

// TestMenusOnMacOnly offers the system's menus or the app's own in the
// Interface section on macOS alone, and Neverland pressed asks for the
// app's own.
func TestMenusOnMacOnly(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Query = "interface"
	var asked []appsettings.Change
	state.Apply = func(change appsettings.Change) { asked = append(asked, change) }
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1600)
	tester.Frame()
	if runtime.GOOS != "darwin" {
		if tester.HasText("Menus") {
			t.Fatal("Menus offered off macOS")
		}
		return
	}
	if err := tester.Click("Neverland"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if len(asked) != 1 || asked[0].Appearance.Kind != appsettings.SetDrawnMenus || !asked[0].Appearance.Drawn {
		t.Fatalf("asked %+v, want the app's own menus", asked)
	}
}
