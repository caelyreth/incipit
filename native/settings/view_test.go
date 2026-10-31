package settings

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/testlock"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := testlock.Acquire(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestSettingsShowsSectionsAndAppliesToggle(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Editing",
		Settings: []appsettings.ToggleSetting{{
			Key:      appsettings.PolicyBoolean("editor.pairs"),
			Label:    "Pair punctuation",
			Value:    true,
			Enabled:  true,
			Selector: "pairs",
		}},
	}}
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	if !tester.HasText("Pair punctuation") || !tester.HasText("Editing") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}

func TestSettingsCategoryNavigation(t *testing.T) {
	state := New()
	state.Data.Shortcuts = []appsettings.Shortcut{{
		Command: "bold",
		Label:   "Bold",
		Group:   "Format",
		Enabled: true,
	}}
	tester := ui.NewTester(state.View, 900, 650)
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	if state.Category != appsettings.Keymap || !tester.HasText("Bold") {
		t.Fatalf("category=%v texts=%#v", state.Category, tester.Texts())
	}
}

func TestSettingsTreeIsOneVerticalFocusGroup(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Automatic pairs",
		Settings: []appsettings.ToggleSetting{{
			Key: appsettings.PolicyBoolean("editor.pairs"), Label: "Pair punctuation",
			Value: true, Enabled: true, Selector: "pairs",
		}},
	}}
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	tester.Click("Editor")
	if !tester.Focused("Editor") {
		t.Fatal("the selected settings category did not take focus")
	}
	tester.Key(0, ui.KeyDown)
	if !tester.Focused("Automatic pairs") {
		t.Fatal("Down did not move within the settings tree")
	}
	tester.Key(0, ui.KeyTab)
	if tester.Focused("Editor") || tester.Focused("Automatic pairs") {
		t.Fatal("Tab did not leave the settings tree")
	}
}

func TestSettingsNavigationBuildsExpandedEditorTree(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Automatic pairs",
		Settings: []appsettings.ToggleSetting{{
			Key:      appsettings.PolicyBoolean("editor.pairs"),
			Label:    "Pair quotation marks",
			Value:    true,
			Enabled:  true,
			Selector: "quotes",
		}},
	}}
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	for _, label := range []string{"Editor", "Automatic pairs"} {
		if !tester.HasText(label) {
			t.Fatalf("missing %q in %#v", label, tester.Texts())
		}
	}
	if _, ok := tester.Find("Automatic pairs"); !ok {
		t.Fatal("editor section is not rendered")
	}
}

func TestSettingsSearchFiltersNavigationAndRows(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{
		{
			Title: "Automatic pairs",
			Settings: []appsettings.ToggleSetting{{
				Key:      appsettings.PolicyBoolean("editor.pairs"),
				Label:    "Pair quotation marks",
				Value:    true,
				Enabled:  true,
				Selector: "quotes",
			}},
		},
		{
			Title: "Typography",
			Settings: []appsettings.ToggleSetting{{
				Key:      appsettings.PolicyBoolean("editor.typography"),
				Label:    "Smart symbols",
				Value:    true,
				Enabled:  true,
				Selector: "symbols",
			}},
		},
	}
	state.Query = "quotation"
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	if !tester.HasText("Pair quotation marks") || tester.HasText("Smart symbols") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
	if _, ok := tester.Find("Automatic pairs"); !ok {
		t.Fatal("matching section missing from navigation")
	}
	if _, ok := tester.Find("Typography"); ok {
		t.Fatal("non-matching section remains in navigation")
	}
}

func TestSettingsTitleRowsAndStatusLayout(t *testing.T) {
	state := New()
	state.Data.Page = "Untitled"
	state.Data.Sections = []appsettings.Section{{
		Title:       "Automatic pairs",
		Description: "Complete matching punctuation.",
		Settings: []appsettings.ToggleSetting{{
			Key:         appsettings.PolicyBoolean("editor.pairs"),
			Label:       "Pair quotation marks",
			Description: "Complete matching quotation marks while typing.",
			Value:       true,
			Enabled:     true,
			Selector:    "quotes",
		}},
	}}
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	description, ok := tester.Find("Shape the way your words are edited.")
	if !ok || !tester.HasText("Editor") || description.Y <= 0 {
		t.Fatalf("heading composition missing from %#v", tester.Texts())
	}
	row, ok := tester.Find("Pair quotation marks")
	if !ok || row.X < 220 {
		t.Fatalf("setting row is not in the content pane: %v", row)
	}
	status, ok := tester.Find("Untitled · changes stay in memory")
	if !ok || status.Y < 690 {
		t.Fatalf("status is not at the footer: %v", status)
	}
}

func TestSettingsSectionLabelsUseDisplayNames(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Automatic pairs",
		Settings: []appsettings.ToggleSetting{{
			Key:      appsettings.PolicyBoolean("editor.pairs"),
			Label:    "Enable automatic pairs",
			Value:    true,
			Enabled:  true,
			Selector: "pairs",
		}},
	}}
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	if !tester.HasText("Automatic pairs") || tester.HasText("section-Automatic pairs") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}

func TestSettingsDefaultsToEditorOnlyAndReferenceSearch(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Automatic pairs",
		Settings: []appsettings.ToggleSetting{{
			Key: appsettings.PolicyBoolean("editor.pairs"), Label: "Enable automatic pairs", Value: true, Enabled: true,
			Selector: "pairs",
		}},
	}}
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if !tester.HasText("Automatic pairs") {
		t.Fatalf("Editor is not expanded: %#v", tester.Texts())
	}
	if _, ok := tester.Find("Search settings"); !ok {
		t.Fatal("settings search missing")
	}
	if _, ok := tester.Find("Close"); ok {
		t.Fatal("settings footer still has a Close action")
	}
	if state.expanded[appsettings.Keymap] || state.expanded[appsettings.Appearance] ||
		state.expanded[appsettings.Diagnostics] {
		t.Fatalf("settings categories expand by default: %#v", state.expanded)
	}
}

func scrollingEditorData(sections int) []appsettings.Section {
	result := make([]appsettings.Section, 0, sections)
	for index := range sections {
		title := fmt.Sprintf("Section %d", index+1)
		settings := make([]appsettings.ToggleSetting, 0, 4)
		for row := range 4 {
			settings = append(settings, appsettings.ToggleSetting{
				Key:         appsettings.PolicyBoolean(kernel.PolicyID(fmt.Sprintf("editor.s%d.r%d", index, row))),
				Label:       fmt.Sprintf("%s option %d", title, row+1),
				Description: "A setting that takes a row of the page.",
				Value:       true, Enabled: true,
				Selector: fmt.Sprintf("s%d-r%d", index, row),
			})
		}
		result = append(result, appsettings.Section{
			Title: title, Description: "Settings grouped under one heading.", Settings: settings,
		})
	}
	return result
}

func TestSettingsTreeAnimatesSectionsOpenAndClosed(t *testing.T) {
	state := New()
	state.Data.Sections = scrollingEditorData(3)
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	tester.Advance(time.Second)
	if err := tester.Click("Editor"); err != nil {
		t.Fatal(err)
	}
	if state.expanded[appsettings.Editor] {
		t.Fatal("clicking the open current category did not collapse it")
	}
	tester.Advance(components.MotionPanel / 3)
	if _, ok := railRow(tester, "Section 2"); !ok {
		t.Fatal("sections vanished at once instead of collapsing")
	}
	tester.Advance(time.Second)
	if _, ok := railRow(tester, "Section 2"); ok {
		t.Fatal("collapsed sections still show in the navigation")
	}
	if err := tester.Click("Editor"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(components.MotionPanel / 3)
	if _, ok := railRow(tester, "Section 2"); !ok {
		t.Fatal("sections did not begin opening")
	}
}

func TestSettingsNavigationScrollsToSectionAndFollowsScrolling(t *testing.T) {
	state := New()
	state.Data.Sections = scrollingEditorData(6)
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
		state := New()
		state.Data.Sections = scrollingEditorData(10)
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
		tester.Advance(time.Second)
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
		state := New()
		state.Data.Sections = scrollingEditorData(20)
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
		tester.Advance(time.Second)
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

func policyData(on bool) []appsettings.Section {
	return []appsettings.Section{{
		Title: "Emphasis", Description: "Inline emphasis with generated markers.",
		Settings: []appsettings.ToggleSetting{
			{
				Key: appsettings.PolicyBoolean("editor.writing"), Label: "Enable emphasis",
				Value: on, Enabled: true, Selector: "policy-markup",
			},
			{
				Key:   appsettings.OptionBoolean(kernel.NewSettingKey("editor.writing", "strong")),
				Label: "Allow strong emphasis", Value: true, Enabled: on, Selector: "option-strong",
			},
			{
				Key:   appsettings.OptionBoolean(kernel.NewSettingKey("editor.writing", "underscores")),
				Label: "Use underscore markers", Value: false, Enabled: on, Selector: "option-underscores",
			},
		},
	}}
}

func TestPolicyCardPutsMasterSwitchInHeaderAndNestsOptions(t *testing.T) {
	state := New()
	state.Data.Sections = policyData(true)
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	tester.Advance(time.Second)
	title, ok := tester.Find("Inline emphasis with generated markers.")
	if !ok {
		t.Fatal("policy header description missing")
	}
	option, ok := tester.Find("Allow strong emphasis")
	if !ok || option.X <= title.X {
		t.Fatalf("options are not nested under the header: header x=%v option=%v", title.X, option)
	}
	if err := tester.Click("Enable emphasis"); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Boolean.Kind != appsettings.BooleanPolicy || changes[0].Value {
		t.Fatalf("master switch change = %#v", changes)
	}
}

func TestPolicyCardFoldsOptionsWhileOff(t *testing.T) {
	state := New()
	state.Data.Sections = policyData(true)
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	tester.Advance(time.Second)

	state.Data.Sections = policyData(false)
	tester.Frame()
	tester.Advance(components.MotionPanel / 3)
	if !tester.HasText("Allow strong emphasis") {
		t.Fatal("options vanished at once instead of folding")
	}
	tester.Advance(time.Second)
	if tester.HasText("Allow strong emphasis") {
		t.Fatal("options of a disabled policy stayed open")
	}
	if !tester.HasText("Turn on to adjust 2 options.") {
		t.Fatalf("folded policy has no hint: %#v", tester.Texts())
	}

	state.Data.Sections = policyData(true)
	tester.Frame()
	tester.Advance(components.MotionPanel / 3)
	if !tester.HasText("Allow strong emphasis") || tester.HasText("Turn on to adjust 2 options.") {
		t.Fatalf("enabling did not begin unfolding: %#v", tester.Texts())
	}
}

func TestPolicySearchOpensFoldedOptions(t *testing.T) {
	state := New()
	state.Data.Sections = policyData(false)
	state.Query = "underscore"
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	tester.Advance(time.Second)
	if !tester.HasText("Use underscore markers") {
		t.Fatalf("matching option stayed folded: %#v", tester.Texts())
	}
	if tester.HasText("Allow strong emphasis") {
		t.Fatal("non-matching option shown during search")
	}
}

// primary is the platform's primary shortcut modifier as MyGo reports it.
func primary() ui.Modifiers {
	if keymap.CurrentPlatform() == keymap.PlatformDarwin {
		return ui.Super
	}
	return ui.Ctrl
}

func chord(key string) kernel.BindingTarget {
	return kernel.NewBindingTarget(kernel.EditorBindingContext, key, kernel.ModifiersPrimary)
}

func keymapState() (*State, *[]appsettings.Change) {
	state := New()
	state.Category = appsettings.Keymap
	state.Data.Shortcuts = []appsettings.Shortcut{
		{Command: "markup.strong", Label: "Bold", Group: "Emphasis",
			Bindings: []kernel.BindingTarget{chord("b")}, Enabled: true},
		{Command: "markup.emphasis", Label: "Italic", Group: "Emphasis",
			Bindings: []kernel.BindingTarget{chord("i")}, Enabled: true},
		{Command: "history.undo", Label: "Undo", Group: "History",
			Bindings: []kernel.BindingTarget{chord("z")}, Enabled: true, Overridden: true},
		{Command: "navigation.tab", Label: "Leave scope", Group: "Navigation",
			Bindings: nil, Enabled: false},
	}
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	return state, &changes
}

func TestKeymapRowsShowKeycapsAndStates(t *testing.T) {
	state, _ := keymapState()
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if _, ok := tester.Find(keymap.Chord(chord("b"), keymap.PlatformCurrent)); !ok {
		t.Fatalf("Bold keycaps missing: %#v", tester.Texts())
	}
	for _, text := range []string{"Custom", "Off", "Reset all"} {
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
	if err := tester.Click("Change shortcut for Bold"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Press keys…") {
		t.Fatalf("recording did not start: %#v", tester.Texts())
	}
	tester.Key(primary(), ui.KeyUnknown) // a bare modifier waits
	tester.Key(primary(), ui.KeyK)
	if len(*changes) != 1 || (*changes)[0].Kind != appsettings.ChangeBinding ||
		(*changes)[0].Command != "markup.strong" || (*changes)[0].Binding != chord("k") {
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
	if err := tester.Click("Change shortcut for Bold"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if len(*changes) != 0 || state.keyEditing != "" {
		t.Fatalf("escape did not cancel: %#v editing=%q", *changes, state.keyEditing)
	}
	if err := tester.Click("Change shortcut for Bold"); err != nil {
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
	if err := tester.Click("Change shortcut for Bold"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(primary(), ui.KeyI)
	tester.Frame()
	if len(*changes) != 0 {
		t.Fatalf("taken shortcut applied without asking: %#v", *changes)
	}
	warning := keymap.Chord(chord("i"), keymap.PlatformCurrent) + " is assigned to “Italic”."
	if !tester.HasText(warning) {
		t.Fatalf("conflict not shown: %#v", tester.Texts())
	}
	tester.Advance(time.Second) // the prompt folds open
	if err := tester.Click("Replace"); err != nil {
		t.Fatal(err)
	}
	if len(*changes) != 1 || (*changes)[0].Displaced != "markup.emphasis" ||
		(*changes)[0].Binding != chord("i") || (*changes)[0].Command != "markup.strong" {
		t.Fatalf("replace change = %#v", *changes)
	}
}

func TestKeymapSearchesByNameWordsAndPressedKeys(t *testing.T) {
	state, _ := keymapState()
	state.keyQuery = keymap.ChordWords(chord("i"), keymap.PlatformCurrent)
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if !tester.HasText("Italic") || tester.HasText("Bold") {
		t.Fatalf("word search = %#v", tester.Texts())
	}
	state.keyQuery = ""
	if err := tester.Click("Search by pressing a shortcut"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(primary(), ui.KeyZ)
	tester.Frame()
	if !tester.HasText("Undo") || tester.HasText("Italic") || tester.HasText("Bold") {
		t.Fatalf("keystroke search = %#v", tester.Texts())
	}
	if err := tester.Click("Clear shortcut search"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Bold") {
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
		(*changes)[0].Command != "history.undo" {
		t.Fatalf("reset all = %#v", *changes)
	}
}

func TestKeycapsAreEvenSquaresWithCenteredGlyphs(t *testing.T) {
	state := New()
	semantic := state.Theme.Semantic()
	target := kernel.NewBindingTarget(kernel.EditorBindingContext, "b",
		kernel.ModifiersPrimary|kernel.ModifiersShift)
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

// A policy card lists the actions its policy answers to as the Keymap's own
// rows: a shortcut recorded in the card is the same change, checked against
// every other action's shortcut, as one recorded on the Keymap page.
func TestPolicyCardEditsItsActionsLikeTheKeymap(t *testing.T) {
	state, changes := keymapState()
	state.Category = appsettings.Editor
	state.Data.Sections = policyData(true)
	state.Data.Sections[0].Policy = "editor.writing"
	state.Data.Sections[0].Shortcuts = state.Data.Shortcuts[:2]
	tester := ui.NewTester(state.View, 980, 900)
	tester.Frame()
	tester.Advance(time.Second)
	if _, ok := tester.Find(keymap.Chord(chord("b"), keymap.PlatformCurrent)); !ok {
		t.Fatalf("card shows no shortcut for its action: %#v", tester.Texts())
	}
	if tester.HasText("Undo") {
		t.Fatal("card lists another policy's action")
	}
	if err := tester.Click("Change shortcut for Bold"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(primary(), ui.KeyZ)
	tester.Frame()
	if len(*changes) != 0 {
		t.Fatalf("a shortcut held outside the card applied without asking: %#v", *changes)
	}
	tester.Advance(time.Second) // the prompt folds open
	if err := tester.Click("Replace"); err != nil {
		t.Fatal(err)
	}
	if len(*changes) != 1 || (*changes)[0].Command != "markup.strong" ||
		(*changes)[0].Displaced != "history.undo" || (*changes)[0].Binding != chord("z") {
		t.Fatalf("card change = %#v", *changes)
	}

	state.Query = "italic"
	tester.Frame()
	tester.Advance(time.Second)
	if !tester.HasText("Italic") || tester.HasText("Allow strong emphasis") {
		t.Fatalf("search by action name = %#v", tester.Texts())
	}
}

func tabSection(binding kernel.BindingTarget, overridden bool) appsettings.Data {
	shortcut := appsettings.Shortcut{
		Command: "navigation.tab", Label: "Leave scope", Group: "Writing",
		Policy: "editor.navigation", Origin: "editor.navigation", OriginLabel: "Scope navigation",
		Bindings: []kernel.BindingTarget{binding}, Enabled: true, Overridden: overridden,
	}
	return appsettings.Data{
		Shortcuts: []appsettings.Shortcut{shortcut},
		Sections: []appsettings.Section{{
			Policy: "editor.navigation", Title: "Scope navigation",
			Description: "Press {navigation.tab} to leave a pair.",
			Settings: []appsettings.ToggleSetting{{
				Key: appsettings.PolicyBoolean("editor.navigation"), Label: "Enable scope navigation",
				Value: true, Enabled: true, Selector: "policy-navigation",
			}, {
				Key:   appsettings.OptionBoolean(kernel.NewSettingKey("editor.navigation", "pairs")),
				Label: "Leave pairs", Description: "{navigation.tab} steps past the closer.",
				Value: true, Enabled: true, Selector: "option-pairs",
			}},
		}},
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

// Text that names a key names the key it has now, and records a new one in
// the sentence, where the Keymap would.
func TestSentencesShowLiveKeysAndRecordInPlace(t *testing.T) {
	state := New()
	tab := kernel.NewBindingTarget(kernel.EditorBindingContext, "tab", 0)
	state.Data = tabSection(tab, false)
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.View, 980, 900)
	tester.Frame()
	tester.Advance(time.Second)
	tabKey := keymap.Chord(tab, keymap.PlatformCurrent)
	if !textContaining(tester, "Press "+tabKey+" to leave a pair.") {
		t.Fatalf("description does not show the key: %#v", tester.Texts())
	}
	if textContaining(tester, "{navigation.tab}") {
		t.Fatal("a raw key reference reached the screen")
	}

	if err := tester.Click("Change shortcut for Leave scope"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Press keys…") || state.keyHost != "policy-editor.navigation" {
		t.Fatalf("recording did not open under the sentence: host=%q %#v", state.keyHost, tester.Texts())
	}
	tester.Key(primary(), ui.KeyJ)
	if len(changes) != 1 || changes[0].Command != "navigation.tab" || changes[0].Binding != chord("j") {
		t.Fatalf("sentence recorded %#v", changes)
	}

	// The key changes; every sentence that names it follows.
	state.Data = tabSection(chord("j"), true)
	tester.Frame()
	now := keymap.Chord(chord("j"), keymap.PlatformCurrent)
	if !textContaining(tester, "Press "+now+" to leave a pair.") ||
		!textContaining(tester, now+" steps past the closer.") || textContaining(tester, tabKey) {
		t.Fatalf("sentences kept the old key: %#v", tester.Texts())
	}
	state.Query = strings.ToLower(keymap.ChordWords(chord("j"), keymap.PlatformCurrent))
	tester.Frame()
	if !tester.HasText("Scope navigation") {
		t.Fatalf("search by the new key's words missed the card: %#v", tester.Texts())
	}
}

// The Keymap groups keys by what they act on, and names the feature that
// adds a key, leading to that feature's card.
func TestKeymapShowsScopeAndOrigin(t *testing.T) {
	state := New()
	state.Category = appsettings.Keymap
	state.Data = tabSection(kernel.NewBindingTarget(kernel.EditorBindingContext, "tab", 0), false)
	state.Data.Shortcuts = append(state.Data.Shortcuts, appsettings.Shortcut{
		Command: "surface.settings", Label: "Open settings", Scope: 4, Group: "App",
		Bindings: []kernel.BindingTarget{chord(",")}, Enabled: true,
	})
	state.Data.Shortcuts[0].Scope = 1
	tester := ui.NewTester(state.View, 980, 900)
	tester.Frame()
	for _, text := range []string{"Writing", "App", appsettings.ScopeDescription(1), "Scope navigation"} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q: %#v", text, tester.Texts())
		}
	}
	if _, ok := tester.Find("From Scope navigation: show in Editor settings"); !ok {
		t.Fatal("feature key has no origin")
	}
	if _, ok := tester.Find("From : show in Editor settings"); ok {
		t.Fatal("built-in key claims an origin")
	}
	if err := tester.Click("From Scope navigation: show in Editor settings"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tab := keymap.Chord(kernel.NewBindingTarget(kernel.EditorBindingContext, "tab", 0), keymap.PlatformCurrent)
	if state.Category != appsettings.Editor || !textContaining(tester, "Press "+tab+" to leave a pair.") {
		t.Fatalf("origin led to %v: %#v", state.Category, tester.Texts())
	}
}

// A key in a sentence is easy to press: caps the Keymap's size with a margin
// of target around them. Its sentence takes a little more leading for them,
// in the same size of type as any other description.
func TestSentenceKeysAreLargeTargetsOnAnUnchangedLine(t *testing.T) {
	state := New()
	state.Data = tabSection(kernel.NewBindingTarget(kernel.EditorBindingContext, "tab", 0), false)
	tester := ui.NewTester(state.View, 980, 900)
	tester.Frame()
	tester.Advance(time.Second)
	tab := keymap.Chord(kernel.NewBindingTarget(kernel.EditorBindingContext, "tab", 0), keymap.PlatformCurrent)
	sentence, ok := tester.Find("Press " + tab + " to leave a pair.")
	plain, okPlain := tester.Find("Shape the way your words are edited.")
	chip, okChip := tester.Find("Change shortcut for Leave scope")
	if !ok || !okPlain || !okChip {
		t.Fatalf("missing elements: %#v", tester.Texts())
	}
	geometry := chipGeometryFor(state.Theme.UIFontSize)
	if sentence.H > geometry.line+0.5 || sentence.H < plain.H {
		t.Fatalf("one line with a key is %v high, plain %v", sentence.H, plain.H)
	}
	if chip.H < geometry.cap+2*geometry.margin-0.5 || chip.W < geometry.cap+2*geometry.margin-0.5 {
		t.Fatalf("key target %vx%v", chip.W, chip.H)
	}
}

// A changed key eases to its new width, carrying the words after it along,
// rather than jumping.
func TestSentenceKeysEaseToANewWidth(t *testing.T) {
	state := New()
	tab := kernel.NewBindingTarget(kernel.EditorBindingContext, "tab", 0)
	state.Data = tabSection(tab, false)
	tester := ui.NewTester(state.View, 980, 900)
	tester.Frame()
	tester.Advance(time.Second)
	before, _ := tester.Find("Change shortcut for Leave scope")

	wide := kernel.NewBindingTarget(kernel.EditorBindingContext, "j",
		kernel.ModifiersPrimary|kernel.ModifiersAlt|kernel.ModifiersShift)
	state.Data = tabSection(wide, true)
	tester.Frame()
	tester.Advance(components.MotionNormal / 4)
	during, _ := tester.Find("Change shortcut for Leave scope")
	tester.Advance(time.Second)
	after, _ := tester.Find("Change shortcut for Leave scope")
	if !(before.W < during.W && during.W < after.W-0.5) {
		t.Fatalf("key width %v → %v → %v, want an eased change", before.W, during.W, after.W)
	}
}

// The recorder folds open under the sentence and folds shut again, rather
// than appearing and vanishing.
func TestRecorderFoldsOpenAndShut(t *testing.T) {
	state := New()
	state.Data = tabSection(kernel.NewBindingTarget(kernel.EditorBindingContext, "tab", 0), false)
	tester := ui.NewTester(state.View, 980, 900)
	tester.Frame()
	tester.Advance(time.Second)
	options, _ := tester.Find("Leave pairs")
	if err := tester.Click("Change shortcut for Leave scope"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(components.MotionSettle / 4)
	opening, _ := tester.Find("Leave pairs")
	tester.Advance(time.Second)
	open, _ := tester.Find("Leave pairs")
	if !(options.Y < opening.Y && opening.Y < open.Y-0.5) {
		t.Fatalf("rows below at %v → %v → %v, want them eased down", options.Y, opening.Y, open.Y)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	tester.Advance(components.MotionSettle / 4)
	closing, _ := tester.Find("Leave pairs")
	if !tester.HasText("Press keys…") || !(closing.Y < open.Y && closing.Y > options.Y+0.5) {
		t.Fatalf("recorder vanished instead of folding: rows at %v, %#v", closing.Y, tester.Texts())
	}
	tester.Advance(time.Second)
	shut, _ := tester.Find("Leave pairs")
	if tester.HasText("Press keys…") || shut.Y > options.Y+0.5 {
		t.Fatalf("recorder never shut: rows at %v", shut.Y)
	}
}

func TestSwappingKeysNeverShowsBothAtOnce(t *testing.T) {
	for step := 0; step <= 100; step++ {
		out, in := swapGlyphs(float32(step) / 100)
		if out > 0 && in > 0 {
			t.Fatalf("at %d%% both show: %v and %v", step, out, in)
		}
	}
	if out, in := swapGlyphs(0); out != 1 || in != 0 {
		t.Fatalf("start %v %v", out, in)
	}
	if out, in := swapGlyphs(1); out != 0 || in != 1 {
		t.Fatalf("end %v %v", out, in)
	}
}

// Mid-change, every cap stands inside the key's box as the sentence has made
// room for it: none reaches over the words beside it.
func TestReshapingCapsStayInsideTheKey(t *testing.T) {
	geometry := chipGeometryFor(16)
	font := ui.Font{Size: geometry.glyph}
	one, oneW := layoutCaps([]string{"⇥"}, font, geometry)
	four, fourW := layoutCaps([]string{"⌥", "⇧", "⌘", "J"}, font, geometry)
	for _, change := range [][2][]keyCap{{one, four}, {four, one}} {
		from, to := change[0], change[1]
		fromW, toW := oneW, fourW
		if len(from) == 4 {
			fromW, toW = fourW, oneW
		}
		for step := 0; step <= 50; step++ {
			ease := float32(step) / 50
			box := fromW + (toW-fromW)*ease
			for index := range max(len(from), len(to)) {
				x, w := morph(from, to, index, ease)
				if x < -0.01 || x+w > box+0.01 {
					t.Fatalf("%d→%d caps at %.2f: cap %d spans %.2f..%.2f of %.2f",
						len(from), len(to), ease, index, x, x+w, box)
				}
			}
		}
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
		keys, ok := tester.Find("Change shortcut for Bold")
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
		want := kernel.NewBindingTarget(kernel.EditorBindingContext, "tab",
			keymap.NormalizeModifiers(mods, keymap.PlatformCurrent))
		if len(*changes) != 1 || (*changes)[0].Binding != want {
			t.Fatalf("mods %v: recorded %#v", mods, *changes)
		}
	}
}

// A chord the app menu takes arrives as the menu's command: the capture
// reads it back as the chord, so it can still be recorded, and here, found
// taken.
func TestRecordingReadsMenuCommandsAsTheirKeys(t *testing.T) {
	state, changes := keymapState()
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if err := tester.Click("Change shortcut for Italic"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	tester.Command("bold")
	tester.Frame()
	tester.Advance(time.Second)
	warning := keymap.Chord(chord("b"), keymap.PlatformCurrent) + " is assigned to “Bold”."
	if len(*changes) != 0 || !tester.HasText(warning) {
		t.Fatalf("menu command not read as its key: %#v %#v", *changes, tester.Texts())
	}
}

// A key the system's menu keeps can never reach an action: recording one
// says so and offers another try instead of assigning it.
func TestRecordingRefusesKeysTheMenuKeeps(t *testing.T) {
	state, changes := keymapState()
	quit := kernel.NewBindingTarget(kernel.EditorBindingContext, "q", kernel.ModifiersPrimary)
	state.Reserved = func(target kernel.BindingTarget) (string, bool) {
		return "Quit Neverland", target == quit
	}
	recording := make([]bool, 0)
	state.Recording = func(on bool) { recording = append(recording, on) }
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if err := tester.Click("Change shortcut for Bold"); err != nil {
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
	if err := tester.Click("Change shortcut for Bold"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	keys, ok := tester.Find("Change shortcut for Bold")
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
	if !tester.Focused("Change shortcut for Bold") {
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
		if tester.Focused("Change shortcut for Bold") {
			break
		}
		tester.Key(0, ui.KeyTab)
		tester.Frame()
	}
	if !tester.Focused("Change shortcut for Bold") {
		t.Fatal("Tab never reached the keys")
	}
	keys, _ := tester.Find("Change shortcut for Bold")
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
	if !tester.Focused("Change shortcut for Bold") || ring() > 0.01 {
		t.Fatalf("returned focus rings %.3f", ring())
	}
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	tester.Key(ui.Shift, ui.KeyTab)
	tester.Frame()
	tester.Advance(time.Second)
	if !tester.Focused("Change shortcut for Bold") || ring() < 0.05 {
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
	if err := tester.Click("Change shortcut for Bold"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(time.Second)
	recording, _ := tester.Find("Press keys…")
	below, _ := tester.Find("Italic")

	tester.Key(primary(), ui.KeyZ)
	tester.Frame()
	for _, step := range []time.Duration{0, components.MotionNormal / 3, time.Second} {
		tester.Advance(step)
		warning := keymap.Chord(chord("z"), keymap.PlatformCurrent) + " is assigned to “Undo”."
		line, ok := tester.Find(warning)
		moved, _ := tester.Find("Italic")
		if !ok || line != recording || moved.Y != below.Y {
			t.Fatalf("after %v: line %v (was %v), row below at %v (was %v)",
				step, line, recording, moved.Y, below.Y)
		}
	}
	if len(*changes) != 0 {
		t.Fatalf("taken key applied: %#v", *changes)
	}

	// Another taken chord answers in place again; Return takes it.
	tester.Key(primary(), ui.KeyI)
	tester.Frame()
	if !tester.HasText(keymap.Chord(chord("i"), keymap.PlatformCurrent) + " is assigned to “Italic”.") {
		t.Fatalf("second chord: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	if len(*changes) != 1 || (*changes)[0].Displaced != "markup.emphasis" || (*changes)[0].Binding != chord("i") {
		t.Fatalf("Return took %#v", *changes)
	}
}
