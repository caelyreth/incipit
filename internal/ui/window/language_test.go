package window

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"

	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/app/phrases"
)

// A language chosen while the app runs names every command of the menu bar
// in it, the commands the keymap declares as well as the field's verbs:
// the keymap, made in the language the app started in, keeps no words.
func TestTheMenuBarSpeaksALanguageChosenWhileRunning(t *testing.T) {
	lang := "en"
	i18n.SetLocaleFunc(func() string { return lang })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.MainMenu()
	english := map[string]string{}
	for _, c := range keymap.Declared(app.Product.Program.Catalog()) {
		english[string(c.ID)] = c.Label()
	}

	lang = "ja"
	menu := app.MainMenu()
	app.syncMenu()
	checked := 0
	var walk func([]*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, it := range items {
			walk(it.Submenu)
			command, ok := app.Product.Keymap.Command(keymap.CommandID(it.ID))
			if !ok {
				continue
			}
			checked++
			want := app.commandState(command.ID, phrases.Text(command.Words)).Label
			if it.Label != want && it.Label != want+"…" || it.Label == english[it.ID] {
				t.Errorf("%s reads %q in Japanese, not %q", it.ID, it.Label, want)
			}
		}
	}
	walk(menu.Items())
	if checked < 20 {
		t.Fatalf("only %d of the menu's items are commands", checked)
	}
}
