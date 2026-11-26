package settings

import (
	"runtime"
	"testing"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/locales"
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
		if _, offered := tester.Find(locales.Menus()); offered || tester.HasText(locales.MenusDescription()) || tester.HasText(locales.MenusNative()) {
			t.Fatal("Menus offered off macOS")
		}
		return
	}
	if _, offered := tester.Find(locales.Menus()); !offered {
		t.Fatal("Menus missing on macOS")
	}
	if err := tester.Click("Neverland"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if len(asked) != 1 || asked[0].Appearance.Kind != appsettings.SetDrawnMenus || !asked[0].Appearance.Drawn {
		t.Fatalf("asked %+v, want the app's own menus", asked)
	}
}
