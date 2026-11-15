package shell

import (
	"fmt"
	"strings"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// navigation is what the library shows and edits in place, apart from
// any list of sheets: the groups open in its tree, the one name being
// written, a group being styled, what waits for a second click to go. The
// library keeps the places, and a place is chosen in it, never gone into:
// a browser shows the place's sheets (browser.go).
type navigation struct {
	// expanded are the groups open in the library's tree.
	expanded map[string]bool
	// renaming is a group or chapter being named in place; groupRename
	// distinguishes a group target for lifecycle reconciliation. One name
	// is written at a time in the window, whichever list shows it.
	renaming    string
	groupRename bool
	name        string
	// focusGroup is a group row or name field to bring into view and focus
	// once its presentation host has drawn it.
	focusGroup string
	// styling is the group whose icon and color are being chosen.
	styling     string
	stylingOpen bool
	// armed is what waits for a second click to be deleted, since armedAt.
	armed   string
	armedAt time.Time
	// groupsTop calls up a create area at the head of the groups.
	groupsTop components.CreateGap
	// dragged is whether a row was dragged this frame, in the library or
	// any list; quiet is until when the create areas then stay shut.
	dragged bool
	quiet   time.Time
}

type sheetDropSpan struct {
	id     string
	top    float32
	height float32
}

// Place is where the sidebar is.
func (s *State) Place() Place { return s.list.place }

// StartAt sets the place the library opens on, without showing its
// sheets as Go does.
func (s *State) StartAt(place Place) { s.list.place = place }

// ReconcileLibrary leaves an invalid group place after filing changes without
// opening a pane or ending a search the user already has open.
func (s *State) ReconcileLibrary() {
	if s == nil || s.Library == nil {
		return
	}
	if s.nav.groupRename {
		if _, ok := s.Library.Group(s.nav.renaming); !ok {
			s.nav.renaming, s.nav.focusGroup = "", ""
			s.nav.groupRename = false
		}
	}
	if s.nav.styling != "" {
		if _, ok := s.Library.Group(s.nav.styling); !ok {
			s.nav.styling, s.nav.focusGroup = "", ""
			s.nav.stylingOpen = false
		}
	}
	if s.list.place.Kind == PlaceGroup {
		if _, ok := s.Library.Group(s.list.place.Group); !ok {
			s.list.place = Place{Kind: PlaceAll}
			s.nav.renaming = ""
			s.nav.groupRename = false
			s.list.overscroll.Reset()
			s.disarm()
		}
	}
}

// Go chooses a place in the library and shows its sheets.
func (s *State) Go(place Place) {
	s.showSheetList()
	s.list.searching = false
	if place == s.list.place {
		return
	}
	s.list.place = place
	s.nav.renaming = ""
	s.nav.groupRename = false
	s.list.overscroll.Reset()
	s.disarm()
}

// files reports a place sheets are filed in, in an order of its own: a
// group, but a smart folder, whose sheets its query finds.
func (s *State) files(place Place) bool {
	if place.Kind != PlaceGroup {
		return false
	}
	group, ok := s.Library.Group(place.Group)
	return ok && !group.Smart()
}

// CurrentGroup is the group a new sheet goes in: the one shown, else the
// Inbox's.
func (s *State) CurrentGroup() string {
	if s.files(s.list.place) {
		return s.list.place.Group
	}
	for _, group := range s.Library.Groups("") {
		if group.Inbox {
			return group.ID
		}
	}
	return ""
}

// sidebar is the library: the Inbox and the filters, the groups, and the
// trash.
func (s *State) sidebar(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	width float32,
	reveal float32,
) {
	if width <= 0.01 || reveal <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	layout := s.paneLayout()
	fullWidth := layout.Sidebar * scale / theme.DefaultUIFontSize
	animate := s.animate(c)
	ui.Column(c).Key("library").Width(width).HeightPercent(100).Shrink(0).Clip().
		Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		// The library's head is the window's traffic lights' band, and
		// moves the window as a title bar would.
		ui.Box(c).Key("library-head").Width(fullWidth).Height(layout.Titlebar * scale / theme.DefaultUIFontSize).
			Shrink(0).DragWindow()
		ui.Column(c).Width(fullWidth).Grow(1).MinHeight(0).Opacity(reveal).
			PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale)).Children(func() {
			if s.Library == nil {
				return
			}
			s.wordmark(c, semantic)
			s.libraryList(c, semantic, animate)
			s.trashRow(c, semantic, animate)
		})
	})
	_ = metrics
}

// lightLine is how far inside the library's own padding the traffic
// lights' left edge lies, so the head of the library lines up with them.
func (s *State) lightLine() float32 {
	return max(0, theme.DefaultLayout.TrafficLightLeft-theme.Space08.DIPs(s.Settings.UIFontSize))
}

// wordmark heads the library with the app's name, in the system face. Its
// full stop wears the accent, so it turns, with the theme, to the group of
// the sheet open.
func (s *State) wordmark(c *ui.Context, semantic theme.Theme) {
	scale := s.Settings.UIFontSize
	size := 22 * scale / theme.DefaultUIFontSize
	// It starts on the traffic lights' line, sits close under them and
	// stands a little apart from the chips.
	ui.Row(c).Key("wordmark").WidthPercent(100).Shrink(0).AlignItems(ui.End).
		Padding(0, 0, theme.Space12.DIPs(scale), s.lightLine()).
		Children(func() {
			ui.Text(c, "Neverland").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
				LetterSpacing(-0.02 * size).TextColor(semantic.Text.Heading).SingleLine()
			ui.Text(c, ".").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
				TextColor(semantic.Accent.Foreground).SingleLine()
		})
}

func (s *State) libraryList(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	// The chips and the section title stand above the scrolling groups.
	s.placeChips(c, semantic, animate)
	label := s.groupsLabel(c, semantic, animate)
	none := len(s.Library.Groups("")) <= 1
	// Resting on the heading calls up an area at the head of the groups.
	_, _, near := label.PointerPosition()
	at := -1
	if near {
		at = 0
	}
	_, topOpen := s.nav.groupsTop.Follow(c, at)
	topOpen = topOpen && !none

	scroll := ui.Scroll(c).Grow(1).MinHeight(0).TrackScroll(&s.libraryScroll)
	scroll = components.BindOverscroll(
		scroll,
		&s.libraryScroll,
		&s.libraryOverscroll,
		scrollfx.Vertical,
		components.OverscrollList,
		scroll.LayoutBounds().H,
		scale,
	)
	scroll.Children(func() {
		list := ui.Column(c).Gap(theme.Space04.DIPs(scale))
		s.libraryOverscroll.Apply(c, list, animate)
		track := components.TrackSelection(list, components.NavigationSelection(c, semantic), animate)
		list.Children(func() {
			if topOpen {
				s.nav.groupsTop.Hold(s.createArea(c, semantic, "new-group-top", "New group", animate, func() {
					s.nav.groupsTop.Use()
					s.startRename(s.Library.NewGroup("", 0))
				}).Transition(unfoldTransition(animate)))
			}
			s.groupTree(c, semantic, "", 0, track, animate)
			if none {
				ui.Box(c).Key("no-groups").PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space04.DIPs(scale)).
					Transition(rowTransition(animate)).Children(func() {
					components.Label(c, "Groups file sheets by project.",
						theme.TextXSmall, semantic.Text.Muted).Opacity(0.8)
				})
			}
			s.createArea(c, semantic, "new-group", "New group", animate, func() {
				s.startRename(s.Library.NewGroup("", Last))
			}).Transition(rowTransition(animate))
		})
	}).ContextMenu(func(m *ui.Menu) {
		if m.Item("New Group").Chosen() {
			s.startRename(s.Library.NewGroup("", Last))
		}
		if m.Item("New Sheet").Chosen() {
			s.Library.NewSheet("", Last)
		}
	})
}

// groupsLabel heads the groups. A group dropped on it goes to the top.
func (s *State) groupsLabel(c *ui.Context, semantic theme.Theme, animate bool) ui.Element {
	scale := s.Settings.UIFontSize
	label := ui.Row(c).Key("groups-label").WidthPercent(100).
		Height(theme.DefaultControls.Scaled(scale).InlineHeight).MarginY(theme.Space06.DIPs(scale)).
		PaddingStart(theme.Space08.DIPs(scale)).AlignItems(ui.End).
		Radius(theme.DefaultControls.Scaled(scale).Radius)
	label.Children(func() {
		components.Label(c, "Groups", theme.TextXSmall, semantic.Text.Muted).Grow(1).SingleLine()
	})
	if dropped, ok := s.dropOn(c, semantic, label, takes{groups: true}, animate); ok {
		s.Library.MoveGroup(dropped.(groupDrag).id, "", Last)
	}
	return label
}

// reveal shows the group a sheet is filed in.
func (s *State) reveal(sheet string) {
	group := s.Library.SheetGroup(sheet)
	if group == "" {
		return
	}
	s.revealGroup(group)
	s.sheets().stopSearching()
	s.Go(Place{Kind: PlaceGroup, Group: group})
}

// placeRow is a filter in the library.
func (s *State) placeRow(
	c *ui.Context,
	semantic theme.Theme,
	place Place,
	label string,
	icon components.IconName,
	count int,
	track *components.SelectionTrack,
	animate bool,
) ui.Element {
	return components.NavigationRow{
		Key: placeKey(place), Label: label, Role: ui.RoleButton, FullWidth: true, Animate: animate,
		Selected: s.chosen(place), Track: track,
		Leading:  func() { s.rowIcon(c, semantic, icon, nil, animate) },
		Trailing: func() { s.count(c, semantic, count) },
		Action:   func() { s.Go(place) },
	}.View(c, semantic)
}

// chosen reports the place the sheet list shows.
func (s *State) chosen(place Place) bool {
	return !s.list.searching && s.SheetListVisible() && s.list.place == place
}

func placeKey(place Place) string {
	return fmt.Sprintf("place-%d-%s", place.Kind, place.Group)
}

// trashRow keeps the trash at the library's foot, where anything dragged
// onto it goes.
func (s *State) trashRow(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	place := Place{Kind: PlaceTrash}
	trash := s.Library.SheetCount(place) + len(s.Library.TrashedGroups())
	foot := ui.Column(c).WidthPercent(100).Shrink(0).Padding(theme.Space06.DIPs(scale), 0, 0, 0).
		BorderWidth(1, 0, 0, 0).BorderColor(semantic.Border.Variant)
	var row ui.Element
	foot.Children(func() {
		track := components.TrackSelection(foot, components.NavigationSelection(c, semantic), animate)
		row = s.placeRow(c, semantic, place, "Trash", components.IconTrash, trash, track, animate)
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Empty Trash…").Disabled(trash == 0).Chosen() {
			// The trash shows what goes, and asks there.
			s.Go(place)
			s.arm(emptyTrashKey)
		}
	})
	dropped, _ := s.dropOn(c, semantic, row, takes{sheets: true, groups: true}, animate)
	switch dropped := dropped.(type) {
	case sheetDrag:
		s.Library.TrashSheet(dropped.id)
	case groupDrag:
		s.Library.TrashGroup(dropped.id)
		s.ReconcileLibrary()
	}
}

// rowIcon draws a row's icon, in color when it has one.
func (s *State) rowIcon(c *ui.Context, semantic theme.Theme, icon components.IconName, tint *ui.Color, animate bool) {
	glyph := ui.Icon(c.Key("icon"), icon.SVG()).FontSize(16 * s.Settings.UIFontSize / theme.DefaultUIFontSize)
	color := semantic.Icon.Muted
	if tint != nil {
		color = *tint
	}
	if animate {
		color = glyph.AnimateColor("tint", color, components.Spring(components.MotionNormal))
	}
	glyph.TextColor(color)
}

func (s *State) count(c *ui.Context, semantic theme.Theme, count int) {
	if count > 0 {
		components.Label(c, fmt.Sprint(count), theme.TextXSmall, semantic.Text.Muted).
			SingleLine().FontFeatures("tnum").Opacity(0.85)
	}
}

// groupTree lists the groups in parent, each opening on its own groups.
// The Inbox leads the library apart from them.
func (s *State) groupTree(
	c *ui.Context,
	semantic theme.Theme,
	parent string,
	depth int,
	track *components.SelectionTrack,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	for _, group := range s.Library.Groups(parent) {
		if group.Inbox {
			continue
		}
		ui.Column(c).Key("tree-" + group.ID).WidthPercent(100).Transition(rowTransition(animate)).Children(func() {
			s.groupRow(c, semantic, group, depth, true, track, animate)
			if group.HasGroups {
				components.Fold{Key: "fold", Open: s.nav.expanded[group.ID], Animate: animate, Settle: motionFold}.View(c, func() {
					ui.Column(c).Gap(theme.Space04.DIPs(scale)).Padding(theme.Space04.DIPs(scale), 0, 0, 0).Children(func() {
						s.groupTree(c, semantic, group.ID, depth+1, track, animate)
					})
				})
			}
		})
	}
}

// rowTransition moves rows as others come and go, and lets them come and
// go by opening and closing in place.
func rowTransition(animate bool) ui.ElementTransition {
	return listTransition(animate, false)
}

// groupIcon is the icon a group wears.
func groupIcon(group GroupItem) components.IconName {
	switch {
	case group.Inbox:
		return components.IconInbox
	case group.Icon != "":
		return components.IconName(group.Icon)
	case group.Smart():
		return components.IconFolderSearch
	}
	return components.IconFolder
}

// groupTint is a group's color: Moonveil's, the text's, or its hue's.
// The Inbox, which wears none, is nil, as the places are.
func groupTint(semantic theme.Theme, group GroupItem) *ui.Color {
	switch {
	case group.Inbox:
		return nil
	case group.Color >= TintHalo && group.Color < TintHalo+len(semantic.Families):
		return &semantic.Families[group.Color-TintHalo].Foreground
	}
	return &semantic.Text.Default
}

func (s *State) groupRow(
	c *ui.Context,
	semantic theme.Theme,
	group GroupItem,
	depth int,
	tree bool,
	track *components.SelectionTrack,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	indent := float32(depth) * theme.Space16.DIPs(scale)
	if s.nav.renaming == group.ID {
		s.renameRow(c, semantic, group, indent, animate)
		return
	}
	place := Place{Kind: PlaceGroup, Group: group.ID}
	open := s.nav.expanded[group.ID]
	row := components.NavigationRow{
		Key: "group-" + group.ID, Label: group.Name, Role: ui.RoleButton, FullWidth: true,
		Indent: indent, Animate: animate, Selected: s.chosen(place), Track: track,
		Leading: func() { s.rowIcon(c, semantic, groupIcon(group), groupTint(semantic, group), animate) },
		Trailing: func() {
			s.count(c, semantic, group.Count)
			if tree && group.HasGroups {
				s.disclosure(c, semantic, group, open, animate)
			}
		},
		Action: func() { s.Go(place) },
	}.View(c, semantic)
	if !group.Inbox {
		row.Drag(groupDrag{id: group.ID})
		s.dragging(row)
		if row.DoubleClicked() {
			s.startRename(group.ID)
		}
	}
	row.ContextMenu(s.groupMenu(group))
	s.dropInto(c, semantic, row, group, animate)
	if s.nav.focusGroup == group.ID {
		s.nav.focusGroup = ""
		row.Focus()
		row.ScrollIntoView()
	}
	s.stylePicker(c, semantic, row, group, animate)
}

// disclosure opens and closes a group in the tree. It trails the row, so
// every icon in the library lines up, nested or not.
func (s *State) disclosure(c *ui.Context, semantic theme.Theme, group GroupItem, open bool, animate bool) {
	metrics := theme.DefaultControls.Scaled(s.Settings.UIFontSize)
	size := 16 * s.Settings.UIFontSize / theme.DefaultUIFontSize
	toggle := ui.ButtonBase(c.Key("disclose")).Label("Show groups in "+group.Name).
		Width(size).Height(size).AlignItems(ui.Center).Justify(ui.Center).
		Radius(metrics.Radius).Border(1, ui.Transparent).FocusRing(false)
	turn := float32(0)
	if open {
		turn = 90
	}
	if animate {
		turn = components.Follow(toggle, "turn", turn, motionTurn)
	}
	toggle.Children(func() {
		ui.Icon(c, components.IconChevronRight.SVG()).FontSize(size * 0.8).
			TextColor(semantic.Icon.Muted).Rotate(turn)
	})
	if toggle.FocusVisible() {
		toggle.Border(1, semantic.Border.Focused)
	}
	if toggle.Clicked() {
		s.nav.expanded[group.ID] = !open
	}
}

// dropInto files sheets, chapters and groups dragged onto a group's row
// into it. A smart folder takes none: what it holds, its query finds.
func (s *State) dropInto(c *ui.Context, semantic theme.Theme, row ui.Element, group GroupItem, animate bool) {
	if group.Smart() {
		return
	}
	dropped, _ := s.dropOn(c, semantic, row, takes{sheets: true, chapters: true, groups: !group.Inbox}, animate)
	switch dropped := dropped.(type) {
	case sheetDrag:
		s.Library.MoveSheet(dropped.id, group.ID, Last)
	case chapterDrag:
		s.Library.MoveChapter(dropped.id, group.ID, Last)
	case groupDrag:
		if dropped.id != group.ID {
			s.Library.MoveGroup(dropped.id, group.ID, Last)
			s.nav.expanded[group.ID] = true
		}
	}
}

func (s *State) groupMenu(group GroupItem) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if group.Smart() {
			if m.Item("Edit Query…").Chosen() {
				s.sheets().editSmartGroup(group)
			}
			m.Separator()
			s.groupItems(m, group)
			return
		}
		if m.Item("New Sheet").Chosen() {
			s.Go(Place{Kind: PlaceGroup, Group: group.ID})
			s.Library.NewSheet(group.ID, Last)
		}
		if m.Item("New Chapter").Chosen() {
			s.Go(Place{Kind: PlaceGroup, Group: group.ID})
			s.sheets().newChapter(group.ID, Last)
		}
		if group.Inbox {
			return
		}
		if m.Item("New Group Inside").Chosen() {
			s.startRename(s.Library.NewGroup(group.ID, Last))
		}
		m.Separator()
		s.groupItems(m, group)
	}
}

// groupItems are what a group's menu does to the group itself, as to a
// smart folder: naming it, its look, where it is, and the trash.
func (s *State) groupItems(m *ui.Menu, group GroupItem) {
	if m.Item("Rename…").Chosen() {
		s.startRename(group.ID)
	}
	if m.Item("Icon & Color…").Chosen() {
		s.startStyling(group.ID)
	}
	s.moveMenu(m, func(target string) { s.Library.MoveGroup(group.ID, target, Last) }, group.ID)
	m.Separator()
	if m.Item(trashGroupLabel(group)).Chosen() {
		s.Library.TrashGroup(group.ID)
		s.ReconcileLibrary()
	}
}

// moveMenu adds a Move To submenu of the groups, leaving out exclude and
// what is inside it.
func (s *State) moveMenu(m *ui.Menu, move func(group string), exclude string) {
	m.Submenu("Move To", func(m *ui.Menu) {
		if exclude != "" && m.Item("Top Level").Chosen() {
			move("")
		}
		skipDepth := -1
		for _, target := range s.Library.Targets() {
			if skipDepth >= 0 && target.Depth > skipDepth {
				continue
			}
			skipDepth = -1
			if target.ID == exclude {
				skipDepth = target.Depth
				continue
			}
			if m.Item(strings.Repeat("    ", target.Depth) + target.Name).Chosen() {
				move(target.ID)
			}
		}
	})
}

func (s *State) startRename(id string) {
	group, ok := s.Library.Group(id)
	if !ok || group.Inbox {
		return
	}
	s.revealGroup(id)
	s.nav.renaming, s.nav.name = id, group.Name
	s.nav.groupRename = true
	s.nav.focusGroup = id
}

// startStyling chooses a group's look beside its row, showing the library
// and opening the group's ancestors if the action came from the sheet list.
func (s *State) startStyling(id string) {
	group, ok := s.Library.Group(id)
	if !ok || group.Inbox {
		return
	}
	s.revealGroup(group.ID)
	s.nav.styling, s.nav.stylingOpen = group.ID, true
	s.nav.focusGroup = group.ID
}

// revealGroup shows the library and opens every ancestor of id, so the
// group's row has a presentation host for an in-place action.
func (s *State) revealGroup(id string) {
	visible := true
	s.SidebarOverride = &visible
	for parent := s.Library.Parent(id); parent != ""; parent = s.Library.Parent(parent) {
		s.nav.expanded[parent] = true
	}
}

// renameRow names a group in place: Return or leaving the field keeps the
// name, Escape keeps the old one.
func (s *State) renameRow(c *ui.Context, semantic theme.Theme, group GroupItem, indent float32, animate bool) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	lead := theme.Space08.DIPs(scale) + indent
	ui.Row(c).Key("rename-"+group.ID).WidthPercent(100).Height(metrics.Height).
		PaddingStart(lead-1).PaddingEnd(theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Focused).Background(semantic.Surface.Inset).
		Children(func() {
			s.rowIcon(c, semantic, groupIcon(group), groupTint(semantic, group), animate)
			input := ui.TextInputBase(c.Key("name"), &s.nav.name).Grow(1).MinWidth(0).
				Font(theme.UIFontFamily).FontSize(theme.TextSmall.Size(scale)).
				FixedLineHeight(theme.TextSmall.LineHeight(scale)).
				TextColor(semantic.Text.Heading).Label("Group name")
			input.AutoFocus()
			if s.nav.focusGroup == group.ID {
				s.nav.focusGroup = ""
				input.Focus()
				input.ScrollIntoView()
			}
			switch inlineName(input, s.nav.name) {
			case nameCancel:
				s.nav.renaming = ""
			case nameCommit:
				s.nav.renaming = ""
				s.Library.RenameGroup(group.ID, s.nav.name)
			}
		})
}

// emptyTrashKey names the trash's Empty button, for arming from a menu.
const emptyTrashKey = "empty-trash"

// trashGroupLabel says that a group goes to the trash with what it holds.
// A smart folder holds nothing: its sheets stay where they are filed.
func trashGroupLabel(group GroupItem) string {
	switch {
	case group.Smart(), group.Count == 0:
		return "Move to Trash"
	case group.Count == 1:
		return "Move to Trash with 1 Sheet"
	}
	return fmt.Sprintf("Move to Trash with %d Sheets", group.Count)
}

// deleteGroupLabel says that a group is deleted with what it holds.
func deleteGroupLabel(group GroupItem) string {
	switch group.Count {
	case 0:
		return "Delete Immediately…"
	case 1:
		return "Delete Immediately with 1 Sheet…"
	}
	return fmt.Sprintf("Delete Immediately with %d Sheets…", group.Count)
}
