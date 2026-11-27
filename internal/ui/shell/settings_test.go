package shell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// settingsState is the field with the app's settings behind it.
func settingsState(t *testing.T) (*State, *fakeSettings, *ui.Tester) {
	t.Helper()
	state, _, tester := onField(t, 1260, 850)
	settings := &fakeSettings{tall: 40}
	state.AppSettings = settings
	settle(tester)
	return state, settings, tester
}

// The settings button puts the settings in the field: the categories
// listing their sections, as groups list their sheets; a section pressed
// opens its category, every section live, in its order; words
// find sections directly, and pressed again the field goes back to the
// library.
func TestTheSettingsStandInTheField(t *testing.T) {
	state, settings, tester := settingsState(t)
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	top := []string{"settings-cat:Appearance", "settings-cat:Editor", "settings-cat:Keymap"}
	if got := shownBlocks(state); !slices.Equal(got, top) {
		t.Fatalf("The field should show the categories, but shows %v.", got)
	}
	if !tester.Focused("Search") || state.pillPlaceholder() != "Search Settings" {
		t.Fatal("The pill should take the keys to find a setting.")
	}
	if _, ok := tester.Find("Tag"); ok {
		t.Fatal("The settings should be found by their words alone, no filters offered.")
	}

	// A section pressed opens its category, in its order.
	if err := tester.Click("Pages"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Keymap/Keys", "settings:Keymap/Pages"}) {
		t.Fatalf("Pressing a section should open its category in its order, but shows %v.", got)
	}
	// Removing the category's ribbon steps back up to the top.
	if err := tester.Click("Remove Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, top) {
		t.Fatalf("Removing the ribbon should return to the top, but shows %v.", got)
	}

	// A category's head opens it from its first section.
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Keymap/Keys", "settings:Keymap/Pages"}) {
		t.Fatalf("Clicking a category head should show its sections in order, but shows %v.", got)
	}
	if err := tester.Click("Remove Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("key")
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Keymap/Keys"}) {
		t.Fatalf("The words should narrow the sections, but the field shows %v.", got)
	}
	away := settings.away
	if err := tester.Click("Hide settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.InSettings() || slices.ContainsFunc(shownBlocks(state), func(id string) bool { return strings.HasPrefix(id, "settings") }) {
		t.Fatalf("Pressed again, the field should go back to the library: %v.", shownBlocks(state))
	}
	if settings.away == away {
		t.Fatal("The settings should be told the field no longer shows them.")
	}
}

// A section pressed further in its category opens it in its order, the
// field never reordered by what was pressed: the section is brought into
// view and marked as it arrives.
func TestASectionPressedArrivesWhereItStands(t *testing.T) {
	state, _, tester := settingsState(t)
	state.ShowSettings()
	settle(tester)
	if err := tester.Click("Markdown"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(50 * time.Millisecond)
	tester.Frame()
	tester.Frame()
	if state.arrivalAt("settings:Editor/Markdown", tester.Now()) <= 0 {
		t.Fatal("Opened, Markdown should be marked as it arrives.")
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Editor/Pairs", "settings:Editor/Markdown"}) {
		t.Fatalf("Opened, the editor's sections should keep their order: %v.", got)
	}
}

// The Settings command brings the field up on the settings from the desk.
func TestTheSettingsCommandShowsThemFromTheDesk(t *testing.T) {
	state, _, tester := settingsState(t)
	state.ShowDesk()
	settle(tester)
	state.ShowSettings()
	settle(tester)
	if state.OnDesk() || !state.InSettings() {
		t.Fatal("The settings should come up in the field.")
	}
}

// Opening a category moves the field as any change of the query does,
// though its live cards stand in wider columns.
func TestOpeningASettingsCategoryMoves(t *testing.T) {
	state, _, tester := settingsState(t)
	state.ShowSettings()
	settle(tester)
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	tester.Frame()
	if slices.Equal(state.field.drawn, state.field.rest) {
		t.Fatal("Opening a category should move the field, not stand it at once.")
	}
}

// A category opened and closed again leaves the pill where it stood over
// the categories, even while it holds the keys: what holds it in the
// window while the category is open lets go as it closes.
func TestAClosedCategoryBringsThePillBack(t *testing.T) {
	state, _, tester := onField(t, 420, 700)
	state.AppSettings = &fakeSettings{tall: 600}
	settle(tester)
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	before, _ := tester.Find("Search")
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	opened, _ := tester.Find("Search")
	if err := tester.Click("Remove Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	after, _ := tester.Find("Search")
	if !tester.Focused("Search") || opened.Y == before.Y || after.Y != before.Y {
		t.Fatalf("Closed, the pill should stand again at %v (held at %v while open), not %v.", before.Y, opened.Y, after.Y)
	}
}

// Entering settings, clicking a category centers the field and pill right
// away so controls inside its section work on the very first click even
// though the pill loses focus on pointer press.
func TestClickingSettingsCategoryCentersFieldAndAllowsDirectFirstClick(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settings := &fakeSettings{tall: 80, showTheme: true}
	state.AppSettings = settings
	settle(tester)

	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	topPill, _ := tester.Find("Search")

	if err := tester.Click("Appearance"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !tester.Focused("Search") {
		t.Fatal("Clicking Appearance should leave Search focused.")
	}
	appPill, _ := tester.Find("Search")
	if appPill.Y <= topPill.Y+20 {
		t.Fatalf("Opening Appearance should lower and re-center the pill from top settings (%v -> %v).", topPill.Y, appPill.Y)
	}

	// Simulate a normal human press-hold-release on Choose Theme while Search
	// is still focused: blurring Search on press must not jump the card away.
	themeBtn, ok := tester.Find("Choose Theme")
	if !ok {
		t.Fatal("Choose Theme control should be present.")
	}
	cx, cy := themeBtn.X+themeBtn.W/2, themeBtn.Y+themeBtn.H/2
	tester.Press(cx, cy)
	tester.Frame()
	tester.Advance(40 * time.Millisecond)
	tester.Frame()
	tester.Release(cx, cy)
	settle(tester)
	if settings.themeClicks != 1 {
		t.Fatalf("First click on Choose Theme should activate directly (clicks=%d).", settings.themeClicks)
	}
	if blurredPill, _ := tester.Find("Search"); abs(blurredPill.Y-appPill.Y) > 0.5 {
		t.Fatalf("Blurring Search inside Appearance should not jump pill Y (%v -> %v).", appPill.Y, blurredPill.Y)
	}
}

// Entering settings or opening any live settings subview widens the cards
// without shifting the field lane's controls or kept slab horizontally.
func TestFieldLaneControlsStayPutAcrossSettingsSubviews(t *testing.T) {
	_, _, tester := settingsState(t)
	home0, _ := tester.Find("Home")
	trash0, _ := tester.Find("Show Trash")
	settings0, _ := tester.Find("Show settings")
	kept0, _ := tester.Find("Kept searches")

	checkSteady := func(phase string, settingsLabel string) {
		home, _ := tester.Find("Home")
		trash, _ := tester.Find("Show Trash")
		settings, _ := tester.Find(settingsLabel)
		kept, _ := tester.Find("Kept searches")
		if home != home0 || trash != trash0 || settings != settings0 {
			t.Fatalf("%s shifted lane buttons: home=%v (want %v) trash=%v (want %v) settings=%v (want %v)",
				phase, home, home0, trash, trash0, settings, settings0)
		}
		if abs(kept.X-kept0.X) > 0.5 || abs(kept.W-kept0.W) > 0.5 {
			t.Fatalf("%s shifted kept slab horizontally: got X=%v W=%v, want X=%v W=%v",
				phase, kept.X, kept.W, kept0.X, kept0.W)
		}
	}

	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	checkSteady("settings top", "Hide settings")

	if err := tester.Click("Appearance"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	checkSteady("settings subview Appearance", "Hide settings")

	if err := tester.Click("Remove Appearance"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("key")
	settle(tester)
	checkSteady("settings live search", "Hide settings")
}

// A category scrolled in and closed, and opened again, moves the pill
// straight to where it stands, each way: the field's scroll goes along
// the change with what it draws, so the pill neither leaps first nor goes
// past its place and back.
func TestTheScrollRememberedGoesAlongTheChange(t *testing.T) {
	// Scrolled so far the pill stays at the window's top, and only a
	// little, the pill short of it.
	for _, times := range []int{12, 1} {
		scrollRememberedGoesAlong(t, times)
	}
}

func scrollRememberedGoesAlong(t *testing.T, times int) {
	t.Helper()
	state, _, tester := onField(t, 420, 700)
	state.AppSettings = &fakeSettings{tall: 600, showTheme: true}
	settle(tester)
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("Appearance"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	for range times {
		tester.Scroll(210, 400, 0, 40)
		tester.Frame()
	}
	tester.Scroll(210, 400, 0, 0)
	settle(tester)
	if state.field.scroll < 20 {
		t.Fatalf("The category should have scrolled, but went %v (%v..%v, %v).", state.field.scroll, state.field.lo, state.field.hi, shownBlocks(state))
	}
	straight := func(press, how string) {
		t.Helper()
		start, _ := tester.Find("Search")
		if err := tester.Click(press); err != nil {
			t.Fatal(err)
		}
		ys := []float32{start.Y}
		for range 60 {
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
			pill, _ := tester.Find("Search")
			ys = append(ys, pill.Y)
		}
		end := ys[len(ys)-1]
		for i := 1; i < len(ys); i++ {
			if (end-ys[0])*(ys[i]-ys[i-1]) < -0.25 {
				t.Fatalf("%s, the pill should go straight from %v to %v, but went back: %v.", how, ys[0], end, ys)
			}
			if abs(ys[i]-ys[i-1]) > abs(end-ys[0])/2+1 {
				t.Fatalf("%s, the pill should not leap, but went %v to %v: %v.", how, ys[i-1], ys[i], ys)
			}
		}
	}
	straight("Remove Appearance", "Closed")
	straight("Appearance", "Opened again")
}

// Words written in the pill and taken away quickly move the pill and the
// cards from the frame after each, though the sections they find measure
// themselves over the frames that follow: the change goes on toward where
// they come to stand from where they are drawn, never standing still while
// they measure, nor leaping as they do.
func TestWordsTakenAwayQuicklyMoveWithoutStandingOrLeaping(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.AppSettings = &fakeSettings{tall: 240, grow: 24, showTheme: true}
	settle(tester)
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	pillY := func() float32 {
		pill, _ := tester.Find("Search")
		return pill.Y
	}
	// Where each card is drawn in the field, by id.
	drawn := func() map[string]stageRect {
		at := map[string]stageRect{}
		for _, fr := range state.field.drawn {
			r := fr.Rect
			r.X += state.field.poise.X
			r.Y += state.field.origin.Y
			at[fr.ID] = r
		}
		return at
	}
	// A leap is a step far longer than the one before it, of the card or
	// of the pill.
	type step struct{ x, y float32 }
	leaps := func(what string, was, now, next step) {
		t.Helper()
		before, after := max(abs(now.x-was.x), abs(now.y-was.y)), max(abs(next.x-now.x), abs(next.y-now.y))
		if after > 2*before+6 {
			t.Errorf("%q: %s should go on as it went, not leap %v after going %v.", state.FieldQuery(), what, after, before)
		}
	}
	for _, write := range []func(){
		func() { tester.Type("m") },
		func() { tester.Type("o") },
		func() { tester.Key(0, ui.KeyBackspace) },
		func() { tester.Key(0, ui.KeyBackspace) },
	} {
		before := pillY()
		write()
		ys := []float32{before}
		frames := []map[string]stageRect{drawn()}
		for range 8 {
			tester.Advance(16 * time.Millisecond)
			ys = append(ys, pillY())
			frames = append(frames, drawn())
		}
		// From the frame after the change, which stands where it was.
		for i := 3; i < len(frames); i++ {
			leaps("the pill", step{0, ys[i-2]}, step{0, ys[i-1]}, step{0, ys[i]})
			for id, r := range frames[i] {
				a, okA := frames[i-2][id]
				b, okB := frames[i-1][id]
				if okA && okB {
					leaps(id, step{a.X, a.Y}, step{b.X, b.Y}, step{r.X, r.Y})
				}
			}
		}
		if ys[2] == ys[4] && ys[len(ys)-1] != ys[2] {
			t.Fatalf("%q: the pill should move as the sections measure themselves, but stood: %v.", state.FieldQuery(), ys)
		}
	}
}

// $key passes the shortcut it names on to the settings, apart from the
// words.
func TestKeyFindsSettingsByTheirShortcut(t *testing.T) {
	state, _, _ := settingsState(t)
	scope, find := state.settingsScope("$settings $in:Keymap $key:⌘K close")
	if scope != "Keymap" || find != (SettingsFind{Words: "close", Key: "⌘K"}) {
		t.Fatalf("scope %q, find %#v", scope, find)
	}
}

// Scrolled until the pill stands stuck at the window's top, its blur
// under it, the open pill keeps offering Key, which stands under it where
// there is room, as a list's bar keeps what it offers.
func TestTheKeyRibbonStaysAsThePillSticks(t *testing.T) {
	state, settings, tester := settingsState(t)
	settings.tall = 3000
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("Search"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if _, ok := tester.Find("Key"); !ok {
		t.Fatalf("The open pill should offer Key: %v", tester.Texts())
	}
	for range 20 {
		tester.Scroll(600, 600, 0, 60)
		tester.Frame()
	}
	settle(tester)
	if state.field.stuck < 0.99 {
		t.Fatalf("The pill should stand stuck, but stands %v so.", state.field.stuck)
	}
	if !state.field.open {
		t.Fatal("Scrolling should leave the pill open.")
	}
	if away := state.field.away[ribbonSide("key")]; away > 0.01 {
		t.Fatalf("Key stands where there is room, but is put away %v.", away)
	}
	if _, ok := tester.Find("Key"); !ok {
		t.Fatalf("The stuck pill should still offer Key: %v", tester.Texts())
	}
}
