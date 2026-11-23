package settings

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/keymap"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/egoist/mygo/ui"
)

// primary is the platform's primary shortcut modifier as MyGo reports it.
func primary() ui.Modifiers {
	if keymap.CurrentPlatform() == keymap.PlatformDarwin {
		return ui.Super
	}
	return ui.Ctrl
}

func chord(key string) keymap.Chord {
	return keymap.NewChord(key, keymap.ModifiersPrimary)
}

func keymapState() (*State, *[]appsettings.Change) {
	state := New()
	state.Data.Shortcuts = []appsettings.Shortcut{
		{Command: keymap.NewPage, Label: "New page", Scope: keymap.ScopePages, Group: "Pages",
			Bindings: []keymap.Chord{chord("n")}},
		{Command: keymap.ClosePage, Label: "Close sheet", Scope: keymap.ScopePages, Group: "Pages",
			Bindings: []keymap.Chord{chord("w")}},
		{Command: keymap.Typewriter, Label: "Toggle Typewriter mode", Scope: keymap.ScopeWindow, Group: "Window",
			Bindings: []keymap.Chord{chord("t")}, Overridden: true},
		{Command: keymap.Motion, Label: "Toggle motion", Scope: keymap.ScopeWindow, Group: "Window"},
	}
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	return state, &changes
}

func TestKeymapRowsShowKeycapsAndStates(t *testing.T) {
	state, _ := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if _, ok := tester.Find(keymap.Spell(chord("n"), keymap.PlatformCurrent)); !ok {
		t.Fatalf("New page keycaps missing: %#v", tester.Texts())
	}
	for _, text := range []string{"Custom", "Reset all"} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q in %#v", text, tester.Texts())
		}
	}
	if _, ok := tester.Find("Reset Shortcut"); !ok {
		t.Fatal("customized shortcut has no reset action")
	}
}

func TestKeymapRecordsNewShortcutInPlace(t *testing.T) {
	state, changes := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Press keys…") {
		t.Fatalf("recording did not start: %#v", tester.Texts())
	}
	tester.Key(primary(), ui.KeyUnknown) // a bare modifier waits
	tester.Key(primary(), ui.KeyK)
	if len(*changes) != 1 || (*changes)[0].Kind != appsettings.ChangeBinding ||
		(*changes)[0].Command != keymap.NewPage || (*changes)[0].Binding != chord("k") {
		t.Fatalf("recorded change = %#v", *changes)
	}
	if tester.HasText("Press keys…") {
		t.Fatal("recording did not end")
	}
}

func TestKeymapRecordingCancelsAndClears(t *testing.T) {
	state, changes := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if len(*changes) != 0 || state.keyEditing != "" {
		t.Fatalf("escape did not cancel: %#v editing=%q", *changes, state.keyEditing)
	}
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(0, ui.KeyBackspace)
	if len(*changes) != 1 || (*changes)[0].Kind != appsettings.ChangeUnbind {
		t.Fatalf("backspace did not clear: %#v", *changes)
	}
}

func TestKeymapAsksBeforeTakingAnotherActionsShortcut(t *testing.T) {
	state, changes := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(primary(), ui.KeyW)
	tester.Frame()
	if len(*changes) != 0 {
		t.Fatalf("taken shortcut applied without asking: %#v", *changes)
	}
	warning := keymap.Spell(chord("w"), keymap.PlatformCurrent) + " is assigned to “Close sheet”."
	if !tester.HasText(warning) {
		t.Fatalf("conflict not shown: %#v", tester.Texts())
	}
	tester.Advance(time.Second) // the prompt folds open
	if err := tester.Click("Replace"); err != nil {
		t.Fatal(err)
	}
	if len(*changes) != 1 || (*changes)[0].Displaced != keymap.ClosePage ||
		(*changes)[0].Binding != chord("w") || (*changes)[0].Command != keymap.NewPage {
		t.Fatalf("replace change = %#v", *changes)
	}
}

func TestKeymapSearchesByNameWordsAndPressedKeys(t *testing.T) {
	state, _ := keymapState()
	state.Query = keymap.Words(chord("w"), keymap.PlatformCurrent)
	tester := ui.NewTester(state.cards(), 600, 1600)
	tester.Frame()
	if !tester.HasText("Close sheet") || tester.HasText("New page") {
		t.Fatalf("word search = %#v", tester.Texts())
	}
	// Without words, every section stands again, the finder lower.
	state.Query = ""
	tester.Frame()
	if err := tester.Click("Search by pressing a shortcut"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(primary(), ui.KeyT)
	tester.Frame()
	if !tester.HasText("Toggle Typewriter mode") || tester.HasText("Close sheet") || tester.HasText("New page") {
		t.Fatalf("keystroke search = %#v", tester.Texts())
	}
	if err := tester.Click("Clear shortcut search"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("New page") {
		t.Fatal("clearing the keystroke search did not restore the list")
	}
}

func TestKeymapResetAllRestoresEveryCustomShortcut(t *testing.T) {
	state, changes := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Reset all"); err != nil {
		t.Fatal(err)
	}
	if len(*changes) != 1 || (*changes)[0].Kind != appsettings.ChangeResetBinding ||
		(*changes)[0].Command != keymap.Typewriter {
		t.Fatalf("reset all = %#v", *changes)
	}
}

func textContaining(tester *ui.Tester, part string) bool {
	for _, text := range tester.Texts() {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

// A menu role's command arriving while a key is recorded is dropped: it
// neither acts nor records.
func TestRecordingDropsMenuCommands(t *testing.T) {
	state, changes := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Change shortcut for Close sheet"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	tester.Command("copy")
	tester.Frame()
	if len(*changes) != 0 || !tester.HasText("Press keys…") {
		t.Fatalf("menu command while recording: %#v %#v", *changes, tester.Texts())
	}
}

// The keycaps pressed to start recording take the focus as they are clicked;
// the capture keeps it from them, so Tab is recorded rather than moving the
// focus along.
func TestRecordingTakesTabAndShiftTab(t *testing.T) {
	for _, mods := range []ui.Modifiers{0, ui.Shift} {
		state, changes := keymapState()
		tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
		tester.Frame()
		// Started from the keyboard, the keycaps hold the focus as the
		// recorder opens, as a click leaves it with them natively.
		keys, ok := tester.Find("Change shortcut for New page")
		if !ok {
			t.Fatal("no keycaps")
		}
		tester.ClickAt(keys.X+keys.W/2, keys.Y+keys.H/2)
		tester.Frame()
		tester.Advance(time.Second)
		tester.Key(0, ui.KeyEscape)
		tester.Frame()
		tester.Key(0, ui.KeySpace)
		tester.Frame()
		tester.Advance(time.Second)
		tester.Key(mods, ui.KeyTab)
		want := keymap.NewChord("tab", keymap.NormalizeModifiers(mods, keymap.PlatformCurrent))
		if len(*changes) != 1 || (*changes)[0].Binding != want {
			t.Fatalf("mods %v: recorded %#v", mods, *changes)
		}
	}
}

// A key the system's menu keeps can never reach an action: recording one
// says so and offers another try instead of assigning it.
func TestRecordingRefusesKeysTheMenuKeeps(t *testing.T) {
	state, changes := keymapState()
	quit := keymap.NewChord("q", keymap.ModifiersPrimary)
	state.Reserved = func(target keymap.Chord) (string, bool) {
		return "Quit Neverland", target == quit
	}
	recording := make([]bool, 0)
	state.Recording = func(on bool) { recording = append(recording, on) }
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	tester.Key(primary(), ui.KeyQ)
	tester.Frame()
	tester.Advance(time.Second)
	if len(*changes) != 0 || !textContaining(tester, "is kept by “Quit Neverland” in the app menu.") {
		t.Fatalf("reserved key: %#v %#v", *changes, tester.Texts())
	}
	// The line stays live: another chord is simply tried instead.
	tester.Key(primary(), ui.KeyK)
	if len(*changes) != 1 || (*changes)[0].Binding != chord("k") {
		t.Fatalf("second try = %#v", *changes)
	}
	tester.Frame()
	// The host heard each recording begin and end, to stand its menu down.
	if len(recording) < 2 || recording[0] != true || recording[len(recording)-1] != false {
		t.Fatalf("recording reports %v", recording)
	}
	for index := 1; index < len(recording); index++ {
		if recording[index] == recording[index-1] {
			t.Fatalf("recording reports repeat: %v", recording)
		}
	}
}

// When a capture ends the key it started from takes the keyboard back
// quietly: its outline eases away rather than staying on as a focus ring.
func TestKeyOutlineEasesAwayAfterCapture(t *testing.T) {
	state, _ := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	keys, ok := tester.Find("Change shortcut for New page")
	if !ok {
		t.Fatal("no keycaps")
	}
	// How far the keys' left edge stands out from just inside it: the ring.
	ring := func() float64 {
		image := tester.Image()
		y := int(keys.Y + keys.H/2)
		edge := image.At(int(keys.X), y)
		inside := image.At(int(keys.X)+3, y)
		er, eg, eb, _ := edge.RGBA()
		ir, ig, ib, _ := inside.RGBA()
		d := func(a, b uint32) float64 { return math.Abs(float64(a) - float64(b)) }
		return (d(er, ir) + d(eg, ig) + d(eb, ib)) / 3 / 0xffff
	}
	recording := ring()
	tester.Key(primary(), ui.KeyK)
	tester.Frame()
	tester.Advance(components.MotionQuick / 4)
	during := ring()
	tester.Advance(time.Second)
	after := ring()
	if !(recording > 0.05 && during < recording && during > after) || after > 0.01 {
		t.Fatalf("ring %.3f while recording, %.3f, then %.3f: want an eased fade to none",
			recording, during, after)
	}
	if !tester.Focused("Change shortcut for New page") {
		t.Fatal("the keyboard did not come back to the keys")
	}
}

// A capture started from the keyboard gives the focus back without a ring;
// the ring returns when the keyboard brings the focus there again.
func TestReturnedFocusIsQuietUntilTheKeyboardMovesIt(t *testing.T) {
	state, _ := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	for range 60 {
		if tester.Focused("Change shortcut for New page") {
			break
		}
		tester.Key(0, ui.KeyTab)
		tester.Frame()
	}
	if !tester.Focused("Change shortcut for New page") {
		t.Fatal("Tab never reached the keys")
	}
	keys, _ := tester.Find("Change shortcut for New page")
	ring := func() float64 {
		image := tester.Image()
		y := int(keys.Y + keys.H/2)
		er, eg, eb, _ := image.At(int(keys.X), y).RGBA()
		ir, ig, ib, _ := image.At(int(keys.X)+3, y).RGBA()
		d := func(a, b uint32) float64 { return math.Abs(float64(a) - float64(b)) }
		return (d(er, ir) + d(eg, ig) + d(eb, ib)) / 3 / 0xffff
	}
	tester.Advance(time.Second)
	if ring() < 0.05 {
		t.Fatal("keys reached by Tab show no ring")
	}
	tester.Key(0, ui.KeySpace)
	tester.Frame()
	tester.Advance(time.Second)
	tester.Key(primary(), ui.KeyK)
	tester.Frame()
	tester.Advance(time.Second)
	if !tester.Focused("Change shortcut for New page") || ring() > 0.01 {
		t.Fatalf("returned focus rings %.3f", ring())
	}
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	tester.Key(ui.Shift, ui.KeyTab)
	tester.Frame()
	tester.Advance(time.Second)
	if !tester.Focused("Change shortcut for New page") || ring() < 0.05 {
		t.Fatalf("focus the keyboard brought back rings %.3f", ring())
	}
}

// A taken key is answered in the recording's own line: it turns to a
// warning in place, moving nothing around it, and stays live, so another
// chord tries again and Return takes the key anyway.
func TestConflictAnswersInTheCaptureLine(t *testing.T) {
	state, changes := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	recording, _ := tester.Find("Press keys…")
	below, _ := tester.Find("Close sheet")

	tester.Key(primary(), ui.KeyT)
	tester.Frame()
	for _, step := range []time.Duration{0, components.MotionNormal / 3, time.Second} {
		tester.Advance(step)
		warning := keymap.Spell(chord("t"), keymap.PlatformCurrent) + " is assigned to “Toggle Typewriter mode”."
		line, ok := tester.Find(warning)
		moved, _ := tester.Find("Close sheet")
		if !ok || line != recording || moved.Y != below.Y {
			t.Fatalf("after %v: line %v (was %v), row below at %v (was %v)",
				step, line, recording, moved.Y, below.Y)
		}
	}
	if len(*changes) != 0 {
		t.Fatalf("taken key applied: %#v", *changes)
	}

	// Another taken chord answers in place again; Return takes it.
	tester.Key(primary(), ui.KeyW)
	tester.Frame()
	if !tester.HasText(keymap.Spell(chord("w"), keymap.PlatformCurrent) + " is assigned to “Close sheet”.") {
		t.Fatalf("second chord: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	if len(*changes) != 1 || (*changes)[0].Displaced != keymap.ClosePage || (*changes)[0].Binding != chord("w") {
		t.Fatalf("Return took %#v", *changes)
	}
}

func TestRecorderFoldsOpenAndShut(t *testing.T) {
	state, _ := keymapState()
	tester := ui.NewTester(state.cards(appsettings.Keymap), 600, 1600)
	tester.Frame()
	tester.Advance(time.Second)
	rows, _ := tester.Find("Toggle Typewriter mode")
	if err := tester.Click("Change shortcut for Close sheet"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(components.MotionSettle / 4)
	opening, _ := tester.Find("Toggle Typewriter mode")
	tester.Advance(time.Second)
	open, _ := tester.Find("Toggle Typewriter mode")
	if !(rows.Y < opening.Y && opening.Y < open.Y-0.5) {
		t.Fatalf("rows below at %v → %v → %v, want them eased down", rows.Y, opening.Y, open.Y)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	tester.Advance(components.MotionSettle / 4)
	closing, _ := tester.Find("Toggle Typewriter mode")
	if !tester.HasText("Press keys…") || !(closing.Y < open.Y && closing.Y > rows.Y+0.5) {
		t.Fatalf("recorder vanished instead of folding: rows at %v, %#v", closing.Y, tester.Texts())
	}
	tester.Advance(time.Second)
	shut, _ := tester.Find("Toggle Typewriter mode")
	if tester.HasText("Press keys…") || shut.Y > rows.Y+0.5 {
		t.Fatalf("recorder never shut: rows at %v", shut.Y)
	}
}
