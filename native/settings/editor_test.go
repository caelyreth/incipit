package settings

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

// The Editor page is the program's catalog: turning a policy off is a
// policy value, and folds its options away.
func TestEditorPageTurnsAPolicyOff(t *testing.T) {
	product := appsettings.DefaultState()
	state := New()
	state.Category = appsettings.Editor
	state.Data = product.Data()
	state.Apply = func(change appsettings.Change) {
		if _, err := product.Apply(change); err != nil {
			t.Fatal(err)
		}
		state.Data = product.Data()
	}
	tester := ui.NewTester(state.View, 980, 1400)
	tester.Frame()
	tester.Advance(2600 * time.Millisecond)
	for _, text := range []string{"Markdown", "Notation", "Asterisks for emphasis", "Mark notation"} {
		if !tester.HasText(text) {
			t.Fatalf("no %q in %#v", text, tester.Texts())
		}
	}
	editorShot(t, tester, "editor-page")
	toggle, ok := tester.AccessByRole(ui.RoleSwitch, "Markdown")
	if !ok {
		t.Fatal("no Markdown switch")
	}
	tester.ClickAt(toggle.Bounds.X+toggle.Bounds.W/2, toggle.Bounds.Y+toggle.Bounds.H/2)
	tester.Advance(time.Second)
	if product.Policy["markdown.enabled"] != false {
		t.Fatalf("policy values %v", product.Policy)
	}
	if !tester.HasText("Turn on to adjust 10 options.") {
		t.Fatalf("texts %#v", tester.Texts())
	}
	editorShot(t, tester, "editor-page-off")
}

func TestEditorPageOffersNearbyContentSelection(test *testing.T) {
	product := appsettings.DefaultState()
	state := New()
	state.Category = appsettings.Editor
	state.Query = "Select nearby content from spaces"
	state.Data = product.Data()
	state.Apply = func(change appsettings.Change) {
		if _, err := product.Apply(change); err != nil {
			test.Fatal(err)
		}
		state.Data = product.Data()
	}
	tester := ui.NewTester(state.View, 980, 800)
	tester.Advance(time.Second)
	toggle, ok := tester.AccessByRole(ui.RoleSwitch, state.Query)
	if !ok {
		test.Fatal("no nearby-content switch")
	}
	before := product.Program
	tester.ClickAt(toggle.Bounds.X+toggle.Bounds.W/2, toggle.Bounds.Y+toggle.Bounds.H/2)
	if product.Policy["words.nearbyContent"] != true || product.Program == before {
		test.Fatal("the nearby-content switch did not recompile the words policy")
	}
}

func editorShot(t *testing.T, tester *ui.Tester, name string) {
	dir := os.Getenv("NEVERLAND_SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, tester.Image())
}
