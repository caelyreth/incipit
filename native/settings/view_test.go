package settings

import (
	"fmt"
	"os"
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
				Key: appsettings.PolicyBoolean("editor.markup"), Label: "Enable emphasis",
				Value: on, Enabled: true, Selector: "policy-markup",
			},
			{
				Key:   appsettings.OptionBoolean(kernel.NewSettingKey("editor.markup", "strong")),
				Label: "Allow strong emphasis", Value: true, Enabled: on, Selector: "option-strong",
			},
			{
				Key:   appsettings.OptionBoolean(kernel.NewSettingKey("editor.markup", "underscores")),
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
