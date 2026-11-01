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

// navigation is what the library and the sheet list show. The library
// keeps the places, and a place is chosen in it, never gone into: the
// sheet list beside it shows the place's sheets, and crossfades from one
// place to the next.
type navigation struct {
	place Place
	// searching shows search results in the sheet list instead of the
	// place.
	searching bool
	// shown is what the sheet list shows, since shownAt while its rows
	// come in; fresh marks the frame it changed, and rank counts the rows
	// drawn so far in this one.
	shown   listView
	shownAt time.Time
	fresh   bool
	rank    int
	// side is where the list shown came in from: 1 the right, -1 the
	// left, 0 neither.
	side float32
	// expanded are the groups open in the library's tree.
	expanded map[string]bool
	// renaming is the group being named in place, and name its text.
	renaming string
	name     string
	// styling is the group whose icon and color are being chosen.
	styling     string
	stylingOpen bool
	// armed is what waits for a second click to be deleted, since armedAt.
	armed   string
	armedAt time.Time
	// picking is the sheets picked together in the list.
	picking picking
	// menuRing is what a menu open on the sheet list acts on.
	menuRing menuRing
	// sheetsGap calls up a create area above a sheet in a group, and
	// groupsTop one at the head of the groups.
	sheetsGap, groupsTop components.CreateGap
	// dragged is whether a row was dragged this frame.
	dragged bool
	// revealing is a chapter just made, to bring into view as it comes.
	revealing string
	// sheetsNear is the sheet whose top the pointer was by, -1 for none.
	sheetsNear int
	// sheetDropSlot is the insertion slot while a sheet is dragged over a
	// group's list, -1 when none; sheetDropRows is the last geometry used
	// to choose it, and sheetDropHeight is the dragged row's extent;
	// sheetDropRoom is the room a chapter takes above it where it lands.
	sheetDropSlot    int
	sheetDropHeight  float32
	sheetDropRoom    float32
	sheetDropVisible bool
	sheetDropRows    []sheetDropSpan
	// grab is where the pointer took the row it drags, landing the row a
	// drop is bringing to its place, and listTop the sheet list's top as
	// last laid out.
	grab    grab
	landing landing
	listTop float32
}

type sheetDropSpan struct {
	id     string
	top    float32
	height float32
}

// Place is where the sidebar is.
func (s *State) Place() Place { return s.nav.place }

// StartAt sets the place the library opens on, without showing its
// sheets as Go does.
func (s *State) StartAt(place Place) { s.nav.place = place }

// Go chooses a place in the library and shows its sheets.
func (s *State) Go(place Place) {
	s.showSheetList()
	s.nav.searching = false
	if place == s.nav.place {
		return
	}
	s.nav.place = place
	s.nav.renaming = ""
	s.sheetOverscroll.Reset()
	s.disarm()
}

// Search shows search in the sheet list, its field focused.
func (s *State) Search() {
	s.nav.searching = true
	s.sheetOverscroll.Reset()
	s.showSheetList()
	s.FindPageRequested = true
	s.disarm()
}

// stopSearching gives the sheet list back to the place.
func (s *State) stopSearching() {
	s.nav.searching = false
	s.sheetOverscroll.Reset()
	s.PageQuery = ""
}

// CurrentGroup is the group a new sheet goes in: the one shown, else the
// Inbox's.
func (s *State) CurrentGroup() string {
	if s.nav.place.Kind == PlaceGroup {
		return s.nav.place.Group
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
	fullWidth := theme.DefaultLayout.Sidebar * scale / theme.DefaultUIFontSize
	animate := s.animate(c)
	ui.Column(c).Key("library").Width(width).HeightPercent(100).Shrink(0).Clip().
		Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		ui.Column(c).Width(fullWidth).HeightPercent(100).Opacity(reveal).
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
	s.dropHighlight(c, semantic, label, false, true, false, animate)
	if dragged, ok := ui.Drop[groupDrag](label); ok {
		s.Library.MoveGroup(dragged.id, "", Last)
	}
	return label
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
	return !s.nav.searching && s.SheetListVisible() && s.nav.place == place
}

func placeKey(place Place) string {
	return fmt.Sprintf("place-%d-%s", place.Kind, place.Group)
}

// trashRow keeps the trash at the library's foot, where anything dragged
// onto it goes.
func (s *State) trashRow(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	place := Place{Kind: PlaceTrash}
	trash := len(s.Library.Sheets(place)) + len(s.Library.TrashedGroups())
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
	s.dropHighlight(c, semantic, row, true, true, true, animate)
	if sheet, ok := ui.Drop[sheetDrag](row); ok {
		s.Library.TrashSheet(sheet.id)
	}
	if tab, ok := ui.Drop[tabDrag](row); ok {
		s.Library.TrashSheet(tab.id)
	}
	if group, ok := ui.Drop[groupDrag](row); ok {
		s.Library.TrashGroup(group.id)
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
	s.stylePicker(c, semantic, row, group, animate)
}

// disclosure opens and closes a group in the tree. It trails the row, so
// every icon in the library lines up, nested or not.
func (s *State) disclosure(c *ui.Context, semantic theme.Theme, group GroupItem, open bool, animate bool) {
	size := 16 * s.Settings.UIFontSize / theme.DefaultUIFontSize
	toggle := ui.ButtonBase(c.Key("disclose")).Label("Show groups in " + group.Name).
		Width(size).Height(size).AlignItems(ui.Center).Justify(ui.Center).FocusRing(false)
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
	if toggle.Clicked() {
		s.nav.expanded[group.ID] = !open
	}
}

// dropHighlight tints a row while something it takes is dragged over it.
func (s *State) dropHighlight(
	c *ui.Context,
	semantic theme.Theme,
	row ui.Element,
	sheets, groups, tabs bool,
	animate bool,
) {
	over := false
	if _, ok := ui.DragOver[sheetDrag](row); ok && sheets {
		over = true
	}
	if _, ok := ui.DragOver[groupDrag](row); ok && groups {
		over = true
	}
	if _, ok := ui.DragOver[tabDrag](row); ok && tabs {
		over = true
	}
	if _, ok := ui.DragOver[chapterDrag](row); ok && sheets {
		over = true
	}
	amount := float32(0)
	if over {
		amount = 1
	}
	if animate {
		amount = components.Follow(row, "drop", amount, components.MotionQuick)
	}
	if amount > 0.001 {
		row.Background(components.Blend(ui.Transparent, semantic.Accent.Muted, amount))
		row.Border(1, components.Blend(ui.Transparent, semantic.Accent.Foreground, amount))
	}
}

// dropInto files sheets, tabs' sheets and groups dragged onto a group's row
// into it.
func (s *State) dropInto(c *ui.Context, semantic theme.Theme, row ui.Element, group GroupItem, animate bool) {
	s.dropHighlight(c, semantic, row, true, !group.Inbox, true, animate)
	end := Last
	if sheet, ok := ui.Drop[sheetDrag](row); ok {
		s.Library.MoveSheet(sheet.id, group.ID, end)
	}
	if tab, ok := ui.Drop[tabDrag](row); ok {
		s.Library.MoveSheet(tab.id, group.ID, end)
	}
	if chapter, ok := ui.Drop[chapterDrag](row); ok {
		s.Library.MoveChapter(chapter.id, group.ID, end)
	}
	if dragged, ok := ui.Drop[groupDrag](row); ok && dragged.id != group.ID && !group.Inbox {
		s.Library.MoveGroup(dragged.id, group.ID, end)
		s.nav.expanded[group.ID] = true
	}
}

func (s *State) groupMenu(group GroupItem) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("New Sheet").Chosen() {
			s.Go(Place{Kind: PlaceGroup, Group: group.ID})
			s.Library.NewSheet(group.ID, Last)
		}
		if m.Item("New Chapter").Chosen() {
			s.Go(Place{Kind: PlaceGroup, Group: group.ID})
			s.newChapter(group.ID, Last)
		}
		if group.Inbox {
			return
		}
		if m.Item("New Group Inside").Chosen() {
			s.nav.expanded[group.ID] = true
			s.startRename(s.Library.NewGroup(group.ID, Last))
		}
		m.Separator()
		if m.Item("Rename…").Chosen() {
			s.startRename(group.ID)
		}
		if m.Item("Icon & Color…").Chosen() {
			s.nav.styling, s.nav.stylingOpen = group.ID, true
		}
		s.moveMenu(m, func(target string) { s.Library.MoveGroup(group.ID, target, Last) }, group.ID)
		m.Separator()
		if m.Item(trashGroupLabel(group)).Chosen() {
			if s.nav.place == (Place{Kind: PlaceGroup, Group: group.ID}) {
				s.Go(Place{Kind: PlaceAll})
			}
			s.Library.TrashGroup(group.ID)
		}
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
	s.nav.renaming, s.nav.name = id, group.Name
	if parent := s.Library.Parent(id); parent != "" {
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
func trashGroupLabel(group GroupItem) string {
	switch group.Count {
	case 0:
		return "Move to Trash"
	case 1:
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
