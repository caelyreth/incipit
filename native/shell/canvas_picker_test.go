package shell

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestPickerUnifiesNavigationAndSheetsInItsOwnColumn(t *testing.T) {
	state, _ := nestedGroupState()
	place := Place{Kind: PlaceGroup, Group: "2"}
	state.StartAt(place)
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	state.ToggleCanvas()
	state.NewPicker()
	settle(tester)
	if !tester.HasText("Chapter") || !tester.HasText("Groups") || !tester.Focused("Search sheets") {
		t.Fatalf("picker did not start at the current sheet list: %v", tester.Texts())
	}
	for _, label := range []string{"Neverland", "Back to library"} {
		if _, ok := tester.Find(label); ok {
			t.Fatalf("the picker copied the primary sidebar or hid navigation behind %q", label)
		}
	}
	column, ok := tester.Find("Sheet picker")
	if !ok {
		t.Fatal("picker disappeared")
	}
	for _, label := range []string{"Places", "Groups", "Novel", "New group", "Trash", "Search sheets", "Chapter"} {
		bounds, ok := tester.Find(label)
		if !ok || bounds.X < column.X || bounds.X+bounds.W > column.X+column.W+1 ||
			bounds.Y < column.Y || bounds.Y+bounds.H > column.Y+column.H {
			t.Fatalf("%q is outside the picker: %v in %v", label, bounds, column)
		}
	}
	for _, label := range []string{"Show groups in Novel", "Show groups in Arc", "Scene"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
		settle(tester)
	}
	picker := state.workspace().columns[1].picker
	if picker.list.place != (Place{Kind: PlaceGroup, Group: "4"}) || state.Place() != place || !state.CanvasOpen() {
		t.Fatalf("choosing a nested group did not stay in its browser: picker %+v, normal %+v", picker.list.place, state.Place())
	}
	if state.list.expanded["2"] || state.list.expanded["3"] {
		t.Fatal("expanding the picker changed the normal library's tree")
	}
	if !tester.HasText("Scene") || !tester.HasText("No sheets in Scene") {
		t.Fatal("choosing a group did not keep navigation and its sheets together")
	}
	if err := tester.Click("Recent"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if picker.list.place.Kind != PlaceRecent || state.Place() != place || len(tester.Menu()) != 0 {
		t.Fatal("the shared place chips navigated the normal pane")
	}
}

func TestPickerLibraryPresentsGroupActionsLocally(t *testing.T) {
	for _, action := range []string{"Rename…", "Icon & Color…", "New Group Inside", "New Chapter"} {
		t.Run(action, func(t *testing.T) {
			state, library := nestedGroupState()
			tester := ui.NewTester(state.View, 1260, 850)
			tester.Frame()
			state.ToggleCanvas()
			state.NewPicker()
			settle(tester)
			if err := tester.RightClick("Novel"); err != nil {
				t.Fatal(err)
			}
			if err := tester.ChooseMenuItem(action); err != nil {
				t.Fatal(err)
			}
			settle(tester)
			if !state.CanvasOpen() || state.SidebarOverride != nil || state.SheetListOverride != nil {
				t.Fatal("the picker action changed the normal pane's presentation")
			}
			switch action {
			case "Rename…", "New Group Inside":
				if !tester.Focused("Group name") {
					t.Fatal("the group's name did not take focus in the picker")
				}
				tester.Type("Renamed")
				tester.Key(0, ui.KeyEnter)
				id := "2"
				if action == "New Group Inside" {
					id = "new-group-1"
				}
				if !slices.Contains(library.calls, "rename "+id+" Renamed") {
					t.Fatalf("rename calls %v", library.calls)
				}
			case "Icon & Color…":
				if !tester.Focused("Novel") || !tester.HasText("Icon and color for Novel") {
					t.Fatal("the style picker lost its local group anchor")
				}
				if err := tester.Click("Life"); err != nil {
					t.Fatal(err)
				}
				if !slices.Contains(library.calls, "style 2  5") {
					t.Fatalf("style calls %v", library.calls)
				}
			case "New Chapter":
				picker := state.workspace().columns[1].picker
				if picker.list.place != (Place{Kind: PlaceGroup, Group: "2"}) ||
					!tester.Focused("Chapter name") {
					t.Fatalf("the chapter was not presented in the picker: %v", tester.Texts())
				}
			}
		})
	}
}

func TestPickerLibraryEditsItsOwnSmartFolderSearch(t *testing.T) {
	state, library, tester := onCanvas(t)
	library.smart = "$words:>1000"
	state.NewPicker()
	settle(tester)
	if err := tester.RightClick("Long"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Edit Query…"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	picker := state.workspace().columns[1].picker
	if !picker.list.searching || picker.list.query != library.smart ||
		!tester.Focused("Search sheets") || state.list.query != "" || !state.CanvasOpen() {
		t.Fatalf("smart folder editing escaped its picker: %v", tester.Texts())
	}
}

func TestPickerTrashUsesTheSharedConfirmation(t *testing.T) {
	state, library, tester := onCanvas(t)
	state.NewPicker()
	settle(tester)
	if err := tester.RightClick("Trash"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Empty Trash…"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.list.place.Kind == PlaceTrash || !tester.HasText("Scrap") || slices.Contains(library.calls, "empty trash") {
		t.Fatal("emptying trash did not first present its contents in the picker")
	}
	if err := tester.Click("Delete All"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(library.calls, "empty trash") {
		t.Fatal("the shared confirmation did not empty the trash")
	}
}

func TestPickerCreatesSheetsInItsOwnPlace(t *testing.T) {
	state, library, tester := onCanvas(t)
	state.NewPicker()
	settle(tester)
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("New sheet"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(library.calls, fmt.Sprint("new sheet 2 ", Last)) || state.Place() == (Place{Kind: PlaceGroup, Group: "2"}) {
		t.Fatalf("sheet creation did not use the picker's place: %v", library.calls)
	}
}

func TestPickerRevealsASheetInItsOwnNavigation(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.NewPicker()
	settle(tester)
	if err := tester.RightClick("Draft"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Show in Library"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !state.CanvasOpen() || !tester.HasText("Chapter") || state.Place() == (Place{Kind: PlaceGroup, Group: "2"}) {
		t.Fatal("revealing a picker sheet left the canvas or navigated the normal pane")
	}
}

func TestPickerLibrariesKeepTheirOwnScrollAndActionFocus(t *testing.T) {
	state, library, tester := onCanvas(t)
	library.extraGroups = 40
	state.NewPicker()
	settle(tester)
	first := state.workspace().columns[1].picker
	state.newPickerAt(len(state.workspace().columns))
	settle(tester)
	second := state.workspace().columns[2].picker
	normalScroll := state.list.libraryScroll.Y
	groups, ok := tester.Find("Groups")
	if !ok {
		t.Fatal("library groups are missing")
	}
	tester.Scroll(groups.X+20, groups.Y+80, 0, 400)
	settle(tester)
	if first.list.libraryScroll.Y <= 0 || second.list.libraryScroll.Y != 0 || state.list.libraryScroll.Y != normalScroll {
		t.Fatalf("library scroll positions: first %v, second %v, normal %v (was %v)",
			first.list.libraryScroll.Y, second.list.libraryScroll.Y, state.list.libraryScroll.Y, normalScroll)
	}
	state.pickerList(second).startRename("2")
	settle(tester)
	name, ok := tester.Find("Group name")
	if !ok || !tester.Focused("Group name") || name.X < state.canvas.spans[2].x-state.canvas.strip.X {
		t.Fatal("another picker took the group's editing focus")
	}
	state.closePicker(second.id)
	settle(tester)
	if state.Typing() || state.nav.renaming != "" {
		t.Fatal("closing the picker left its group editing session open")
	}
}

func TestPickerNavigationLeavesRoomForSheets(t *testing.T) {
	for _, window := range []struct {
		width, height int
		font          float32
	}{{1260, 850, 16}, {900, 600, 24}, {900, 420, 24}, {800, 500, 12}} {
		t.Run(fmt.Sprintf("%dx%d-at-%v", window.width, window.height, window.font), func(t *testing.T) {
			state, library := libraryState()
			state.Settings.UIFontSize = window.font
			state.StartAt(Place{Kind: PlaceGroup, Group: "2"})
			library.extraGroups = 40
			tester := ui.NewTesterWith(state.View, window.width, window.height,
				ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
			state.ToggleCanvas()
			state.NewPicker()
			settle(tester)
			row, ok := tester.Find("Chapter")
			if !ok || row.H < theme.DefaultControls.Scaled(window.font).Row || row.Y+row.H > float32(window.height) {
				t.Fatalf("navigation crowded the sheet list out of the picker: %v", row)
			}
			group, ok := tester.Find("Novel")
			if !ok || group.H < theme.DefaultControls.Scaled(window.font).Height {
				t.Fatalf("the sheet list crowded group navigation out of the picker: %v", group)
			}
			if state.workspace().columns[1].picker.list.libraryScroll.MaxY <= 0 {
				t.Fatal("long group navigation did not become scrollable")
			}
		})
	}
}

func TestPickerLibraryAcceptsSheetsDraggedFromAnotherPicker(t *testing.T) {
	state, library, tester := onCanvas(t)
	state.NewPicker()
	settle(tester)
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	state.newPickerAt(len(state.workspace().columns))
	settle(tester)
	source, sourceOK := tester.Find("Draft")
	target, targetOK := tester.Find("Novel")
	if !sourceOK || !targetOK {
		t.Fatalf("drag endpoints are missing: %v", tester.Texts())
	}
	tester.Press(source.X+20, source.Y+source.H/2)
	tester.Move(source.X+30, source.Y+source.H/2)
	tester.Advance(16 * time.Millisecond)
	tester.Move(target.X+target.W/2, target.Y+target.H/2)
	tester.Frame()
	tester.Release(target.X+target.W/2, target.Y+target.H/2)
	settle(tester)
	if len(library.moves) != 1 || library.moves[0].id != "8" || library.moves[0].group != "2" {
		t.Fatalf("the library did not accept the dragged sheet: %v", library.moves)
	}
}
