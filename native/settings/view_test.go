package settings

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

func TestSettingsOpensOnAppearanceAndAppliesToggle(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if state.Category != appsettings.Appearance || !tester.HasText("Typewriter mode") {
		t.Fatalf("category=%v texts=%#v", state.Category, tester.Texts())
	}
	if !tester.HasText("Theme") || !tester.HasText("Animations") || len(changes) != 0 {
		t.Fatalf("texts=%#v changes=%#v", tester.Texts(), changes)
	}
	if state.expanded[appsettings.Keymap] {
		t.Fatalf("settings categories expand by default: %#v", state.expanded)
	}
}

func TestSettingsCategoryNavigation(t *testing.T) {
	state, _ := keymapState()
	state.Category = appsettings.Appearance
	tester := ui.NewTester(state.View, 900, 650)
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	if state.Category != appsettings.Keymap || !tester.HasText("New page") {
		t.Fatalf("category=%v texts=%#v", state.Category, tester.Texts())
	}
}

func TestSettingsTreeIsOneVerticalFocusGroup(t *testing.T) {
	state, _ := keymapState()
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	tester.Click("Keymap")
	tester.Advance(time.Second)
	if !tester.Focused("Keymap") {
		t.Fatal("the selected settings category did not take focus")
	}
	tester.Key(0, ui.KeyDown)
	if !tester.Focused("Pages") {
		t.Fatal("Down did not move within the settings tree")
	}
	tester.Key(0, ui.KeyTab)
	if tester.Focused("Keymap") || tester.Focused("Pages") {
		t.Fatal("Tab did not leave the settings tree")
	}
}

func TestSettingsTreeNavigatesItsHierarchy(t *testing.T) {
	state := scrollingState(3)
	tester := ui.NewTester(state.View, 900, 650)
	tester.Advance(time.Second)
	category, found := tester.AccessByRole(ui.RoleTreeItem, "Keymap")
	section, childFound := tester.AccessByRole(ui.RoleTreeItem, "Section 2")
	if !found || !childFound || category.Level != 1 || section.Level != 2 || !category.Expandable || !category.Expanded {
		t.Fatalf("the tree lost its hierarchy: category %+v, section %+v", category, section)
	}
	if err := tester.Click("Section 2"); err != nil {
		t.Fatal(err)
	}
	tester.Key(0, ui.KeyLeft)
	if !tester.Focused("Keymap") {
		t.Fatal("Left did not return to the category")
	}
	tester.Key(0, ui.KeyLeft)
	if state.expanded[appsettings.Keymap] {
		t.Fatal("Left did not close the category")
	}
	category, _ = tester.AccessByRole(ui.RoleTreeItem, "Keymap")
	if category.Expanded {
		t.Fatal("a closing category still reports itself as expanded")
	}
	tester.Advance(time.Second)
	tester.Key(0, ui.KeyRight)
	if !state.expanded[appsettings.Keymap] {
		t.Fatal("Right did not open the category")
	}
	tester.Advance(time.Second)
	tester.Key(0, ui.KeyRight)
	if !tester.Focused("Section 1") {
		t.Fatal("Right did not enter the category's sections")
	}
	tester.Key(0, ui.KeyEnd)
	if !tester.Focused("Section 3") {
		t.Fatal("End did not reach the last tree item")
	}
	tester.Key(0, ui.KeyHome)
	if !tester.Focused("Appearance") {
		t.Fatal("Home did not reach the first tree item")
	}
	tester.Key(0, ui.KeyK)
	tester.Key(0, ui.KeyE)
	tester.Key(0, ui.KeyY)
	if !tester.Focused("Keymap") {
		t.Fatal("typing a category's name did not focus it")
	}
}

func TestSettingsSearchFiltersNavigationAndRows(t *testing.T) {
	state, _ := keymapState()
	state.Query = "close"
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	if !tester.HasText("Close sheet") || tester.HasText("Toggle Zen mode") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
	if _, ok := railRow(tester, "Pages"); !ok {
		t.Fatal("matching group missing from navigation")
	}
	if _, ok := railRow(tester, "Window"); ok {
		t.Fatal("non-matching group remains in navigation")
	}
}

func TestSettingsTitleRowsAndStatusLayout(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	description, ok := tester.Find(appsettings.Appearance.Description())
	if !ok || description.Y <= 0 {
		t.Fatalf("heading composition missing from %#v", tester.Texts())
	}
	row, ok := tester.Find("Typewriter mode")
	if !ok || row.X < 220 {
		t.Fatalf("setting row is not in the content pane: %v", row)
	}
	status, ok := tester.Find("Changes stay in memory")
	if !ok || status.Y < 690 {
		t.Fatalf("status is not at the footer: %v", status)
	}
}

// scrollingData is a keymap of groups enough to scroll the page and the
// navigation.
func scrollingData(groups int) []appsettings.Shortcut {
	result := make([]appsettings.Shortcut, 0, groups*4)
	for index := range groups {
		title := fmt.Sprintf("Section %d", index+1)
		for row := range 4 {
			result = append(result, appsettings.Shortcut{
				Command: keymap.CommandID(fmt.Sprintf("s%d.r%d", index, row)),
				Label:   fmt.Sprintf("%s option %d", title, row+1),
				Scope:   keymap.ScopeApp, Group: title,
			})
		}
	}
	return result
}

func scrollingState(groups int) *State {
	state := New()
	state.Category = appsettings.Keymap
	state.expanded[appsettings.Keymap] = true
	state.Data.Shortcuts = scrollingData(groups)
	return state
}

func TestSettingsTreeAnimatesSectionsOpenAndClosed(t *testing.T) {
	state := scrollingState(3)
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	tester.Advance(time.Second)
	editorShot(t, tester, "navigation-open")
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	if state.expanded[appsettings.Keymap] {
		t.Fatal("clicking the open current category did not collapse it")
	}
	tester.Advance(components.MotionPanel / 3)
	editorShot(t, tester, "navigation-closing")
	if _, ok := railRow(tester, "Section 2"); !ok {
		t.Fatal("sections vanished at once instead of collapsing")
	}
	tester.Advance(time.Second)
	editorShot(t, tester, "navigation-closed")
	if _, ok := railRow(tester, "Section 2"); ok {
		t.Fatal("collapsed sections still show in the navigation")
	}
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(components.MotionPanel / 3)
	editorShot(t, tester, "navigation-opening")
	if _, ok := railRow(tester, "Section 2"); !ok {
		t.Fatal("sections did not begin opening")
	}
}

func TestSettingsNavigationScrollsToSectionAndFollowsScrolling(t *testing.T) {
	state := scrollingState(6)
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	tester.Advance(time.Second)
	if got := state.activeSection(); got != "Section 1" {
		t.Fatalf("initial active section = %q", got)
	}
	if err := tester.Click("Section 4"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(time.Second)
	if state.contentScroll.Y <= 0 {
		t.Fatal("choosing a section did not scroll the page")
	}
	if got := state.activeSection(); got != "Section 4" {
		t.Fatalf("active section after navigation = %q", got)
	}
	heading, ok := tester.Find("Section 4 option 1")
	if !ok || heading.Y < 0 || heading.Y > 260 {
		t.Fatalf("Section 4 is not near the top of the page: %v", heading)
	}

	state.contentScroll.Y = 0
	tester.Frame()
	if got := state.activeSection(); got != "Section 1" {
		t.Fatalf("navigation did not follow scrolling back: %q", got)
	}
}

func TestSettingsScrollSurfacesPullPastTheirEdges(t *testing.T) {
	t.Run("page", func(t *testing.T) {
		state := scrollingState(10)
		tester := ui.NewTester(state.View, 900, 650)
		tester.Frame()
		tester.Advance(time.Second)
		tester.Scroll(700, 400, 0, 100000)
		if state.contentScroll.Y != state.contentScroll.MaxY {
			t.Fatalf("page stopped at %v of %v", state.contentScroll.Y, state.contentScroll.MaxY)
		}
		if _, y := state.contentOver.Offset(); y >= 0 {
			t.Fatalf("page pull = %v, want past its end", y)
		}
		tester.Frame()
		pulled, ok := tester.Find("Section 10 option 1")
		if !ok {
			t.Fatal("last section is not in the built page")
		}
		tester.Advance(2 * time.Second)
		if _, y := state.contentOver.Offset(); y != 0 {
			t.Fatalf("page pull did not settle: %v", y)
		}
		settled, ok := tester.Find("Section 10 option 1")
		if !ok {
			t.Fatal("last section vanished after the pull")
		}
		if pulled.Y >= settled.Y {
			t.Fatalf("end pull moved content down: %v then %v", pulled.Y, settled.Y)
		}
	})

	t.Run("navigation", func(t *testing.T) {
		state := scrollingState(20)
		tester := ui.NewTester(state.View, 900, 650)
		tester.Frame()
		tester.Advance(time.Second)
		tester.Scroll(100, 500, 0, 100000)
		if state.navScroll.MaxY <= 0 || state.navScroll.Y != state.navScroll.MaxY {
			t.Fatalf("navigation stopped at %v of %v", state.navScroll.Y, state.navScroll.MaxY)
		}
		if _, y := state.navOver.Offset(); y >= 0 {
			t.Fatalf("navigation pull = %v, want past its end", y)
		}
		tester.Advance(2 * time.Second)
		if _, y := state.navOver.Offset(); y != 0 {
			t.Fatalf("navigation pull did not settle: %v", y)
		}
	})
}

// railRow finds a navigation row by its label, ignoring page content.
func railRow(tester *ui.Tester, label string) (ui.Rect, bool) {
	rect, ok := tester.Find(label)
	return rect, ok && rect.X < 240
}

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
	state.Category = appsettings.Keymap
	state.Data.Shortcuts = []appsettings.Shortcut{
		{Command: keymap.NewPage, Label: "New page", Scope: keymap.ScopePages, Group: "Pages",
			Bindings: []keymap.Chord{chord("n")}},
		{Command: keymap.ClosePage, Label: "Close sheet", Scope: keymap.ScopePages, Group: "Pages",
			Bindings: []keymap.Chord{chord("w")}},
		{Command: keymap.Zen, Label: "Toggle Zen mode", Scope: keymap.ScopeWindow, Group: "Window",
			Bindings: []keymap.Chord{chord("z")}, Overridden: true},
		{Command: keymap.Motion, Label: "Toggle motion", Scope: keymap.ScopeWindow, Group: "Window"},
	}
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	return state, &changes
}

func TestKeymapRowsShowKeycapsAndStates(t *testing.T) {
	state, _ := keymapState()
	tester := ui.NewTester(state.View, 980, 740)
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
	tester := ui.NewTester(state.View, 980, 740)
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
	tester := ui.NewTester(state.View, 980, 740)
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
	tester := ui.NewTester(state.View, 980, 740)
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
	state.keyQuery = keymap.Words(chord("w"), keymap.PlatformCurrent)
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if !tester.HasText("Close sheet") || tester.HasText("New page") {
		t.Fatalf("word search = %#v", tester.Texts())
	}
	state.keyQuery = ""
	if err := tester.Click("Search by pressing a shortcut"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(primary(), ui.KeyZ)
	tester.Frame()
	if !tester.HasText("Toggle Zen mode") || tester.HasText("Close sheet") || tester.HasText("New page") {
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
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if err := tester.Click("Reset all"); err != nil {
		t.Fatal(err)
	}
	if len(*changes) != 1 || (*changes)[0].Kind != appsettings.ChangeResetBinding ||
		(*changes)[0].Command != keymap.Zen {
		t.Fatalf("reset all = %#v", *changes)
	}
}

func TestKeycapsAreEvenSquaresWithCenteredGlyphs(t *testing.T) {
	state := New()
	semantic := state.Theme.Semantic()
	target := keymap.NewChord("b", keymap.ModifiersPrimary|keymap.ModifiersShift)
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(state.Theme.MyGoTheme())
		ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, 1)).Padding(20, 20, 20, 20).Children(func() {
			state.keycaps(c, semantic, target, false)
		})
	}, 200, 60)
	tester.SetScale(2)
	tester.Frame()
	img := tester.Image()
	lit := func(x, y int) bool {
		c := img.RGBAAt(x, y)
		return int(c.R)+int(c.G)+int(c.B) > 18
	}
	row := 60
	widths := []int{}
	for x := 0; x < img.Bounds().Dx(); {
		for x < img.Bounds().Dx() && !lit(x, row) {
			x++
		}
		start := x
		for x < img.Bounds().Dx() && lit(x, row) {
			x++
		}
		if x <= start {
			break
		}
		widths = append(widths, x-start)
		top, bottom := -1, -1
		for y := 0; y < img.Bounds().Dy(); y++ {
			if lit((start+x)/2, y) {
				if top < 0 {
					top = y
				}
				bottom = y + 1
			}
		}
		if x-start != bottom-top {
			t.Fatalf("cap %d is %dx%d, not square", len(widths), x-start, bottom-top)
		}
		face := img.RGBAAt(start+3, (top+bottom)/2)
		inkTop, inkBottom := 1<<30, -1
		for y := top + 3; y < bottom-3; y++ {
			for xx := start + 3; xx < x-3; xx++ {
				c := img.RGBAAt(xx, y)
				if int(c.R)+int(c.G)+int(c.B)-int(face.R)-int(face.G)-int(face.B) > 120 {
					inkTop, inkBottom = min(inkTop, y), max(inkBottom, y+1)
				}
			}
		}
		if above, below := inkTop-top, bottom-inkBottom; above-below > 1 || below-above > 1 {
			t.Fatalf("cap %d glyph off center: above=%d below=%d", len(widths), above, below)
		}
	}
	if len(widths) != 3 || widths[0] != widths[1] || widths[1] != widths[2] {
		t.Fatalf("cap widths = %v", widths)
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
	tester := ui.NewTester(state.View, 980, 740)
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
		tester := ui.NewTester(state.View, 980, 740)
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
	tester := ui.NewTester(state.View, 980, 740)
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
	tester := ui.NewTester(state.View, 980, 740)
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
	tester := ui.NewTester(state.View, 980, 740)
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
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if err := tester.Click("Change shortcut for New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	recording, _ := tester.Find("Press keys…")
	below, _ := tester.Find("Close sheet")

	tester.Key(primary(), ui.KeyZ)
	tester.Frame()
	for _, step := range []time.Duration{0, components.MotionNormal / 3, time.Second} {
		tester.Advance(step)
		warning := keymap.Spell(chord("z"), keymap.PlatformCurrent) + " is assigned to “Toggle Zen mode”."
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
	tester := ui.NewTester(state.View, 980, 900)
	tester.Frame()
	tester.Advance(time.Second)
	rows, _ := tester.Find("Toggle Zen mode")
	if err := tester.Click("Change shortcut for Close sheet"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(components.MotionSettle / 4)
	opening, _ := tester.Find("Toggle Zen mode")
	tester.Advance(time.Second)
	open, _ := tester.Find("Toggle Zen mode")
	if !(rows.Y < opening.Y && opening.Y < open.Y-0.5) {
		t.Fatalf("rows below at %v → %v → %v, want them eased down", rows.Y, opening.Y, open.Y)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	tester.Advance(components.MotionSettle / 4)
	closing, _ := tester.Find("Toggle Zen mode")
	if !tester.HasText("Press keys…") || !(closing.Y < open.Y && closing.Y > rows.Y+0.5) {
		t.Fatalf("recorder vanished instead of folding: rows at %v, %#v", closing.Y, tester.Texts())
	}
	tester.Advance(time.Second)
	shut, _ := tester.Find("Toggle Zen mode")
	if tester.HasText("Press keys…") || shut.Y > rows.Y+0.5 {
		t.Fatalf("recorder never shut: rows at %v", shut.Y)
	}
}
