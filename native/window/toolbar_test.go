package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/page"
)

func TestToolbarActionsPublishPresentation(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if app.Shell.Typewriter == nil {
		t.Fatal("toolbar mode callback is not bound")
	}
	app.Shell.Typewriter(true)
	toolbar := app.Shell.Toolbar
	if !app.Product.Presentation.Typewriter || !app.Shell.Appearance.Typewriter || !toolbar.Typewriter {
		t.Fatalf("mode on: product=%#v shell=%#v toolbar=%#v",
			app.Product.Presentation, app.Shell.Appearance, toolbar)
	}
	if !toolbar.TypewriterEnabled {
		t.Fatalf("toolbar mode disabled: %#v", toolbar)
	}
	app.Shell.Typewriter(false)
	if app.Product.Presentation.Typewriter || app.Shell.Toolbar.Typewriter {
		t.Fatalf("mode did not turn off: product=%#v toolbar=%#v",
			app.Product.Presentation, app.Shell.Toolbar)
	}
}

func TestToolbarBindingsFollowTheKeymap(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	moved := keymap.NewChord("y", keymap.ModifiersPrimary|keymap.ModifiersAlt|keymap.ModifiersShift)
	app.Settings.Apply(appsettings.Remap(keymap.Typewriter, moved))
	for _, platform := range []keymap.Platform{keymap.PlatformDarwin, keymap.PlatformOther} {
		if got, want := bindingLabelFor(app.Product.Keymap, keymap.Typewriter, platform),
			keymap.Spell(moved, platform); got != want {
			t.Fatalf("typewriter label on %v = %q, want %q", platform, got, want)
		}
	}
	if app.Shell.Toolbar.Bindings.Typewriter != bindingLabel(app.Product.Keymap, keymap.Typewriter) {
		t.Fatalf("toolbar typewriter binding = %q", app.Shell.Toolbar.Bindings.Typewriter)
	}
	app.Settings.Apply(appsettings.Unbind(keymap.Typewriter))
	if app.Shell.Toolbar.Bindings.Typewriter != "" {
		t.Fatalf("toolbar bindings after unbind = %#v", app.Shell.Toolbar.Bindings)
	}
}

func TestToolbarKeepsPresentationWithoutAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Typewriter(true)
	app.closeActive()
	if !app.Shell.Toolbar.Typewriter || !app.Shell.Toolbar.TypewriterEnabled {
		t.Fatalf("page-less toolbar lost presentation state: %+v", app.Shell.Toolbar)
	}
	app.Shell.Typewriter(false)
	if app.Shell.Toolbar.Typewriter {
		t.Fatal("page-less toolbar did not follow presentation change")
	}
}

func TestToolbarModeTooltipsShowCommandBindings(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, test := range []struct {
		label   string
		binding string
	}{
		{"Typewriter", app.Shell.Toolbar.Bindings.Typewriter},
	} {
		if test.binding == "" {
			t.Fatalf("%s has no toolbar binding", test.label)
		}
		bounds, ok := tester.Find(test.label)
		if !ok {
			t.Fatalf("%s toolbar action missing", test.label)
		}
		tester.Move(bounds.X+bounds.W/2, bounds.Y+bounds.H/2)
		tester.Advance(time.Second)
		if !tester.HasText(test.binding) {
			t.Fatalf("%s tooltip texts = %#v", test.label, tester.Texts())
		}
		tester.Move(0, 0)
		tester.Advance(time.Second)
	}
}

func TestFooterCountsThePageAndTheSelection(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "one two" {
		tester.Type(string(r))
		tester.Frame()
	}
	if got, want := app.Shell.Statistics, "2 words  ·  7 characters"; got != want {
		t.Fatalf("page statistics = %q, want %q", got, want)
	}
	el := app.pageElement()
	ref, ok := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	if !ok {
		t.Fatal("the page has no text")
	}
	if !el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 0))}, time.Now()) ||
		!el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 3)), Extend: true}, time.Now()) {
		t.Fatal("could not select the first word")
	}
	app.refreshEditorChrome()
	if got, want := app.Shell.Statistics,
		"1 word of 2 words  ·  3 characters of 7 characters"; got != want {
		t.Fatalf("selection statistics = %q, want %q", got, want)
	}
}

// The dock holds what is inserted and what the selection is changed into
// beside the writing modes; a selection at rest raises the page's own
// tools over it, pressed there as their keys run them, and Show
// Formatting calls them up at the caret.
func TestFormattingStandsOverTheSelection(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "hello world" {
		tester.Type(string(r))
		tester.Frame()
	}
	run(tester, time.Second)
	for _, label := range []string{"Typewriter", "Insert table", "Strong", "Paragraph"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("The dock has no %s: %v", label, tester.Texts())
		}
	}
	if _, ok := tester.Find("Expand selection"); ok {
		t.Fatal("The selection's tools show with nothing selected.")
	}
	el := app.pageElement()
	ref, ok := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	if !ok {
		t.Fatal("the page has no text")
	}
	if !el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 0))}, tester.Now()) ||
		!el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 5)), Extend: true}, tester.Now()) {
		t.Fatal("could not select the word")
	}
	run(tester, time.Second)
	if _, ok := tester.Find("Expand selection"); !ok {
		t.Fatalf("The selection's tools did not rise: %v", tester.Texts())
	}
	if err := tester.Click("Strong"); err != nil {
		t.Fatalf("%v: %v", err, tester.Texts())
	}
	run(tester, 100*time.Millisecond)
	if !strongText(el, ref, "hello") {
		t.Fatal("Strong did not mark the selection.")
	}
	el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 8))}, tester.Now())
	run(tester, time.Second)
	if _, ok := tester.Find("Expand selection"); ok {
		t.Fatal("The selection's tools stayed with the selection gone.")
	}
	if err := app.routeCommand(keymap.FormatTools); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	if _, ok := tester.Find("Expand selection"); !ok {
		t.Fatal("Show Formatting did not call the tools up at the caret.")
	}
}

// strongText reports whether the slot holds text marked strong, as is.
func strongText(el *page.Element, ref doc.SlotRef, text string) bool {
	node, ok := el.Editor.Engine.Snapshot().Node(ref.Node)
	if !ok {
		return false
	}
	found := false
	node.Slot(ref.Slot).Content().Each(func(_ int, it doc.Item) bool {
		if it.Text == text && it.Marks != nil && it.Marks.Has(profile.Strong) {
			found = true
		}
		return !found
	})
	return found
}
