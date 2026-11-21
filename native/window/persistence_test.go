package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/page"
	"github.com/caelyreth/neverland/native/theme"
)

func TestSettingsSurviveARestart(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	app.setPresentation(appsettings.PresentationTypewriter, true)
	if _, err := app.Product.Apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
		Kind: appsettings.SetTheme, Theme: uint8(appsettings.Daybreak),
	})); err != nil {
		t.Fatal(err)
	}
	app.settingsChanged()
	app.refreshSettingsData()
	moved := keymap.NewChord("y", keymap.ModifiersPrimary|keymap.ModifiersAlt)
	app.Settings.Apply(appsettings.Remap(keymap.Typewriter, moved))
	app.Settings.Apply(appsettings.SetOption("notation.reveal", "hidden"))
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	if !app.Product.Presentation.Typewriter {
		t.Fatal("typewriter mode did not survive the restart")
	}
	if app.Product.Appearance.Appearance != appsettings.Daybreak {
		t.Fatalf("theme = %v", app.Product.Appearance.Appearance)
	}
	if chord, ok := app.Product.Keymap.ChordFor(keymap.Typewriter); !ok || chord != moved {
		t.Fatalf("typewriter key = %v, %v", chord, ok)
	}
	if value, ok := app.Product.Policy["notation.reveal"]; !ok || value != "hidden" {
		t.Fatalf("notation reveal = %v, %v", value, ok)
	}
	if app.Shell.Settings.Appearance != theme.Daybreak {
		t.Fatalf("shell theme = %v", app.Shell.Settings.Appearance)
	}
}

func TestEveryPageKeepsItsSelection(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	tester := atTheDesk(app)
	tester.Type("one")
	tester.Frame()
	selectPageText(t, app.pageElement(), 2, 0)
	app.save()

	app.newSheet()
	tester.Frame()
	tester.Type("two")
	tester.Frame()
	selectPageText(t, app.pageElement(), 0, 3)
	app.save()

	app.Shell.SelectColumn(sheetColumn(t, app, "0"))
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	first := app.pageElement()
	if first == nil {
		t.Fatal("first page did not open")
	}
	if got := first.Editor.Selection(); got.Anchor.Pos.Offset != 2 || got.Head.Pos.Offset != 0 {
		t.Fatalf("first page selection = %+v", got)
	}
	app.Shell.SelectColumn(sheetColumn(t, app, "1"))
	second := app.pageElement()
	if second == nil {
		t.Fatal("second page did not open")
	}
	if got := second.Editor.Selection(); got.Anchor.Pos.Offset != 0 || got.Head.Pos.Offset != 3 {
		t.Fatalf("second page selection = %+v", got)
	}
}

func selectPageText(t *testing.T, el *page.Element, anchor, head int) {
	t.Helper()
	if el == nil {
		t.Fatal("the page did not open")
	}
	ref, ok := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	if !ok {
		t.Fatal("the page has no text")
	}
	at := time.Now()
	if !el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, anchor))}, at) {
		t.Fatal("could not place the selection anchor")
	}
	if !el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, head)), Extend: true}, at) {
		t.Fatal("could not place the selection head")
	}
}
