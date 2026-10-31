package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// navigation is where the sidebar is and how it got there: going into a
// place slides it in from the trailing side over the one left, and going
// back slides the other way.
type navigation struct {
	place Place
	from  Place
	// direction is +1 going in and -1 going back.
	direction float32
	// moved asks the next frame to start the slide.
	moved bool
	// expanded are the groups open in the library's tree.
	expanded map[string]bool
	// renaming is the group being named in place, and name its text.
	renaming string
	name     string
}

// Place is where the sidebar is.
func (s *State) Place() Place { return s.nav.place }

// Go shows a place in the sidebar, going in when forward.
func (s *State) Go(place Place, forward bool) {
	if place == s.nav.place {
		return
	}
	s.nav.from, s.nav.place = s.nav.place, place
	s.nav.direction = 1
	if !forward {
		s.nav.direction = -1
	}
	s.nav.moved = true
	s.nav.renaming = ""
}

// back is the place a place goes back to: a group's parent group, or the
// library.
func (s *State) back(place Place) Place {
	if place.Kind == PlaceGroup && s.Library != nil {
		if parent := s.Library.Parent(place.Group); parent != "" {
			return Place{Kind: PlaceGroup, Group: parent}
		}
	}
	return Place{Kind: PlaceLibrary}
}

// CurrentGroup is the group a new sheet goes in: the one shown, else the
// Inbox's.
func (s *State) CurrentGroup() string {
	if s.nav.place.Kind == PlaceGroup {
		return s.nav.place.Group
	}
	return ""
}

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
	ui.Column(c).Width(width).HeightPercent(100).Shrink(0).Clip().
		Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		ui.Column(c).Width(fullWidth).HeightPercent(100).Opacity(reveal).Children(func() {
			ui.Column(c).Grow(1).MinHeight(0).Padding(
				theme.Space12.DIPs(scale),
				theme.Space12.DIPs(scale),
				theme.Space12.DIPs(scale),
				theme.Space12.DIPs(scale),
			).Children(func() {
				components.TextField{
					Key: "page-search", Label: "Search sheets", Placeholder: "Search sheets…",
					Value: &s.PageQuery, Icon: components.IconSearch.SVG(),
					ClearIcon: components.IconClose.SVG(), Animate: animate,
					FocusRequested: s.FindPageRequested,
				}.View(c, semantic)
				s.FindPageRequested = false
				ui.Box(c).Height(theme.Space12.DIPs(scale))
				if s.Library == nil {
					return
				}
				if strings.TrimSpace(s.PageQuery) != "" {
					s.searchPage(c, semantic, animate)
					return
				}
				s.places(c, semantic, fullWidth, animate)
			})
		})
	})
	_ = metrics
}

// places draws the place shown and, while the sidebar slides between two,
// the one it leaves.
func (s *State) places(c *ui.Context, semantic theme.Theme, width float32, animate bool) {
	stage := ui.Box(c).Grow(1).MinHeight(0).WidthPercent(100).Clip()
	progress := float32(1)
	if animate {
		if s.nav.moved {
			stage.Jump("slide", 0)
		}
		progress = components.Follow(stage, "slide", 1, motionPlace)
	}
	s.nav.moved = false
	progress = max(0, min(progress, 1))
	// A place slides a third of the sidebar: enough to say which way, not
	// so far that the eye loses either.
	distance := width / 3
	stage.Children(func() {
		if progress < 0.999 {
			s.placeLayer(c, semantic, s.nav.from, -s.nav.direction*progress*distance, 1-progress, animate)
		}
		s.placeLayer(c, semantic, s.nav.place, s.nav.direction*(1-progress)*distance, progress, animate)
	})
}

func (s *State) placeLayer(
	c *ui.Context,
	semantic theme.Theme,
	place Place,
	offset, opacity float32,
	animate bool,
) {
	ui.Column(c).Key(placeKey(place)).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		Translate(offset, 0).Opacity(opacity).Children(func() {
		switch place.Kind {
		case PlaceLibrary:
			s.libraryPage(c, semantic, animate)
		case PlaceGroup:
			s.groupPage(c, semantic, place.Group, animate)
		case PlaceTrash:
			s.trashPage(c, semantic, animate)
		default:
			s.filterPage(c, semantic, place, animate)
		}
	})
}

func placeKey(place Place) string {
	return fmt.Sprintf("place-%d-%s", place.Kind, place.Group)
}

func (s *State) libraryPage(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Gap(theme.Space02.DIPs(scale)).Children(func() {
			s.sectionLabel(c, semantic, "Filters", nil)
			s.placeRow(c, semantic, Place{Kind: PlaceAll}, "All", components.IconSheets,
				len(s.Library.Sheets(Place{Kind: PlaceAll})), animate)
			s.placeRow(c, semantic, Place{Kind: PlaceRecent}, "Last 7 Days", components.IconClock,
				len(s.Library.Sheets(Place{Kind: PlaceRecent})), animate)
			ui.Box(c).Height(theme.Space12.DIPs(scale))
			s.sectionLabel(c, semantic, "Library", func() {
				components.IconButton{
					Key: "new-group", Label: "New group", Icon: components.IconNewGroup.SVG(),
					Style: components.SubtleButton, Size: components.CompactButton,
					IconColor: &semantic.Icon.Muted, Animate: animate,
					Action: func() { s.startRename(s.Library.NewGroup("")) },
				}.View(c, semantic)
			})
			s.groupTree(c, semantic, "", 0, animate)
		})
	}).ContextMenu(func(m *ui.Menu) {
		if m.Item("New Group").Chosen() {
			s.startRename(s.Library.NewGroup(""))
		}
		if m.Item("New Sheet").Chosen() {
			s.Library.NewSheet("")
		}
	})
	trash := len(s.Library.Sheets(Place{Kind: PlaceTrash})) + len(s.Library.TrashedGroups())
	row := s.placeRow(c, semantic, Place{Kind: PlaceTrash}, "Trash", components.IconTrash, trash, animate)
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Empty Trash").Disabled(trash == 0).Chosen() {
			s.Library.EmptyTrash()
		}
	})
	if sheet, ok := ui.Drop[sheetDrag](row); ok {
		s.Library.TrashSheet(sheet.id)
	}
	if group, ok := ui.Drop[groupDrag](row); ok {
		s.Library.TrashGroup(group.id)
	}
}

func (s *State) sectionLabel(c *ui.Context, semantic theme.Theme, title string, trailing func()) {
	scale := s.Settings.UIFontSize
	ui.Row(c).Height(theme.DefaultControls.Scaled(scale).InlineHeight).
		PaddingStart(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Children(func() {
		components.Label(c, title, theme.TextXSmall, semantic.Text.Muted).Grow(1).SingleLine()
		if trailing != nil {
			trailing()
		}
	})
}

// placeRow is a row that goes to a place.
func (s *State) placeRow(
	c *ui.Context,
	semantic theme.Theme,
	place Place,
	label string,
	icon components.IconName,
	count int,
	animate bool,
) ui.Element {
	return components.NavigationRow{
		Key: placeKey(place), Label: label, Role: ui.RoleButton, FullWidth: true, Animate: animate,
		Leading:  func() { s.rowIcon(c, semantic, icon) },
		Trailing: func() { s.count(c, semantic, count) },
		Action:   func() { s.Go(place, true) },
	}.View(c, semantic)
}

func (s *State) rowIcon(c *ui.Context, semantic theme.Theme, icon components.IconName) {
	ui.Icon(c, icon.SVG()).FontSize(16 * s.Settings.UIFontSize / theme.DefaultUIFontSize).
		TextColor(semantic.Icon.Muted)
}

func (s *State) count(c *ui.Context, semantic theme.Theme, count int) {
	if count > 0 {
		components.Label(c, fmt.Sprint(count), theme.TextXSmall, semantic.Text.Muted).SingleLine()
	}
}

// groupTree lists the groups in parent, each opening on its own groups.
func (s *State) groupTree(c *ui.Context, semantic theme.Theme, parent string, depth int, animate bool) {
	for _, group := range s.Library.Groups(parent) {
		ui.Column(c).Key("tree-" + group.ID).WidthPercent(100).Transition(rowTransition(animate)).Children(func() {
			s.groupRow(c, semantic, group, depth, true, animate)
			if group.HasGroups {
				components.Fold{Key: "fold", Open: s.nav.expanded[group.ID], Animate: animate, Settle: motionFold}.View(c, func() {
					s.groupTree(c, semantic, group.ID, depth+1, animate)
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

func (s *State) groupRow(c *ui.Context, semantic theme.Theme, group GroupItem, depth int, tree bool, animate bool) {
	scale := s.Settings.UIFontSize
	indent := float32(depth) * theme.Space16.DIPs(scale)
	icon := components.IconFolder
	if group.Inbox {
		icon = components.IconInbox
	}
	if s.nav.renaming == group.ID {
		s.renameRow(c, semantic, group, indent, icon)
		return
	}
	open := s.nav.expanded[group.ID]
	row := components.NavigationRow{
		Key: "group-" + group.ID, Label: group.Name, Role: ui.RoleButton, FullWidth: true,
		Indent: indent, Animate: animate,
		Leading: func() {
			if tree {
				s.disclosure(c, semantic, group, open, animate)
			}
			s.rowIcon(c, semantic, icon)
		},
		Trailing: func() { s.count(c, semantic, group.Count) },
		Action:   func() { s.Go(Place{Kind: PlaceGroup, Group: group.ID}, true) },
	}.View(c, semantic)
	row.Drag(groupDrag{id: group.ID}).ContextMenu(s.groupMenu(group))
	s.dropInto(c, semantic, row, group.ID)
}

// disclosure opens and closes a group in the tree; a group without groups
// keeps its room, so names line up.
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
		if group.HasGroups {
			ui.Icon(c, components.IconChevronRight.SVG()).FontSize(size * 0.8).
				TextColor(semantic.Icon.Muted).Rotate(turn)
		}
	})
	if group.HasGroups && toggle.Clicked() {
		s.nav.expanded[group.ID] = !open
	}
}

// dropInto lets sheets and groups dragged onto a group's row file into it.
func (s *State) dropInto(c *ui.Context, semantic theme.Theme, row ui.Element, group string) {
	if _, over := ui.DragOver[sheetDrag](row); over {
		row.Border(1, semantic.Border.Focused)
	}
	if _, over := ui.DragOver[groupDrag](row); over {
		row.Border(1, semantic.Border.Focused)
	}
	if sheet, ok := ui.Drop[sheetDrag](row); ok {
		s.Library.MoveSheet(sheet.id, group, len(s.Library.Sheets(Place{Kind: PlaceGroup, Group: group})))
	}
	if dragged, ok := ui.Drop[groupDrag](row); ok && dragged.id != group {
		s.Library.MoveGroup(dragged.id, group, len(s.Library.Groups(group)))
		s.nav.expanded[group] = true
	}
	_ = c
}

func (s *State) groupMenu(group GroupItem) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("New Sheet").Chosen() {
			s.Library.NewSheet(group.ID)
		}
		if m.Item("New Group").Chosen() {
			s.nav.expanded[group.ID] = true
			s.startRename(s.Library.NewGroup(group.ID))
		}
		m.Separator()
		if m.Item("Rename…").Disabled(group.Inbox).Chosen() {
			s.startRename(group.ID)
		}
		if !group.Inbox {
			s.moveMenu(m, func(target string) { s.Library.MoveGroup(group.ID, target, 1<<30) }, group.ID)
		}
		m.Separator()
		if m.Item("Move to Trash").Disabled(group.Inbox).Chosen() {
			if s.nav.place == (Place{Kind: PlaceGroup, Group: group.ID}) {
				s.Go(s.back(s.nav.place), false)
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
func (s *State) renameRow(c *ui.Context, semantic theme.Theme, group GroupItem, indent float32, icon components.IconName) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	ui.Row(c).Key("rename-"+group.ID).WidthPercent(100).Height(metrics.Height).
		PaddingStart(theme.Space08.DIPs(scale)+indent+16*scale/theme.DefaultUIFontSize+theme.Space06.DIPs(scale)).
		PaddingEnd(theme.Space04.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Focused).Background(semantic.Surface.Inset).
		Children(func() {
			s.rowIcon(c, semantic, icon)
			input := ui.TextInputBase(c.Key("name"), &s.nav.name).Grow(1).MinWidth(0).
				Font(theme.UIFontFamily).FontSize(theme.TextSmall.Size(scale)).
				FixedLineHeight(theme.TextSmall.LineHeight(scale)).
				TextColor(semantic.Text.Heading).Label("Group name")
			input.AutoFocus()
			focused := ui.Local(input, "was-focused", func() bool { return false })
			if !*focused && input.Focused() {
				input.SetTextSelection(0, len([]rune(s.nav.name)))
			}
			switch {
			case input.Shortcut(0, ui.KeyEscape):
				s.nav.renaming = ""
			case input.Submitted(), *focused && !input.Focused():
				s.nav.renaming = ""
				s.Library.RenameGroup(group.ID, s.nav.name)
			}
			*focused = input.Focused()
		})
}

func (s *State) groupPage(c *ui.Context, semantic theme.Theme, id string, animate bool) {
	group, ok := s.Library.Group(id)
	if !ok {
		s.placeHeader(c, semantic, "Gone", nil, animate)
		return
	}
	place := Place{Kind: PlaceGroup, Group: id}
	s.placeHeader(c, semantic, group.Name, func() {
		components.IconButton{
			Key: "new-sheet", Label: "New sheet in " + group.Name, Icon: components.IconNewSheet.SVG(),
			Style: components.SubtleButton, Size: components.CompactButton,
			IconColor: &semantic.Icon.Muted, Animate: animate,
			Action: func() { s.Library.NewSheet(id) },
		}.View(c, semantic)
	}, animate)
	groups := s.Library.Groups(id)
	sheets := s.Library.Sheets(place)
	s.sheetList(c, semantic, sheets, place, animate, func() {
		for _, child := range groups {
			ui.Column(c).Key("sub-" + child.ID).WidthPercent(100).Transition(rowTransition(animate)).Children(func() {
				s.groupRow(c, semantic, child, 0, false, animate)
			})
		}
	}, func(m *ui.Menu) {
		if m.Item("New Sheet").Chosen() {
			s.Library.NewSheet(id)
		}
		if m.Item("New Group").Chosen() {
			s.startRename(s.Library.NewGroup(id))
		}
	}, len(groups) == 0 && len(sheets) == 0, "No sheets yet")
}

func (s *State) filterPage(c *ui.Context, semantic theme.Theme, place Place, animate bool) {
	title := "All"
	if place.Kind == PlaceRecent {
		title = "Last 7 Days"
	}
	s.placeHeader(c, semantic, title, nil, animate)
	sheets := s.Library.Sheets(place)
	s.sheetList(c, semantic, sheets, place, animate, nil, nil, len(sheets) == 0, "Nothing here yet")
}

func (s *State) trashPage(c *ui.Context, semantic theme.Theme, animate bool) {
	groups := s.Library.TrashedGroups()
	sheets := s.Library.Sheets(Place{Kind: PlaceTrash})
	empty := len(groups) == 0 && len(sheets) == 0
	s.placeHeader(c, semantic, "Trash", func() {
		if !empty {
			components.Button{
				Key: "empty-trash", Label: "Empty",
				Style: components.SubtleButton, Size: components.CompactButton,
				Animate: animate, Action: s.Library.EmptyTrash,
			}.View(c, semantic)
		}
	}, animate)
	s.sheetList(c, semantic, sheets, Place{Kind: PlaceTrash}, animate, func() {
		for _, group := range groups {
			group := group
			ui.Column(c).Key("trashed-" + group.ID).WidthPercent(100).Transition(rowTransition(animate)).Children(func() {
				components.NavigationRow{
					Key: "group-" + group.ID, Label: group.Name, Role: ui.RoleButton, FullWidth: true,
					Animate:  animate,
					Leading:  func() { s.rowIcon(c, semantic, components.IconFolder) },
					Trailing: func() { s.count(c, semantic, group.Count) },
				}.View(c, semantic).ContextMenu(func(m *ui.Menu) {
					if m.Item("Put Back").Chosen() {
						s.Library.RestoreGroup(group.ID)
					}
					m.Separator()
					if m.Item("Delete Immediately…").Chosen() {
						s.Library.DeleteGroup(group.ID)
					}
				})
			})
		}
	}, nil, empty, "The trash is empty")
}

func (s *State) searchPage(c *ui.Context, semantic theme.Theme, animate bool) {
	sheets := s.Library.Search(strings.TrimSpace(s.PageQuery))
	s.sectionLabel(c, semantic, fmt.Sprintf("Results · %d", len(sheets)), nil)
	s.sheetList(c, semantic, sheets, Place{Kind: PlaceAll}, animate, nil, nil, len(sheets) == 0, "No sheets match your search")
}

// placeHeader is a place's title, with the way back before it.
func (s *State) placeHeader(c *ui.Context, semantic theme.Theme, title string, trailing func(), animate bool) {
	scale := s.Settings.UIFontSize
	ui.Row(c).Height(theme.DefaultControls.Scaled(scale).Height).Shrink(0).
		AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Children(func() {
		components.IconButton{
			Key: "back", Label: "Back", Icon: components.IconBack.SVG(),
			Style: components.SubtleButton, Size: components.CompactButton,
			IconColor: &semantic.Icon.Muted, Animate: animate,
			Action: func() { s.Go(s.back(s.nav.place), false) },
		}.View(c, semantic)
		components.Label(c, title, theme.TextSmall, semantic.Text.Heading).Grow(1).MinWidth(0).SingleLine()
		if trailing != nil {
			trailing()
		}
	})
}

// sheetList lists sheets under what leading draws, or says empty when
// there is nothing.
func (s *State) sheetList(
	c *ui.Context,
	semantic theme.Theme,
	sheets []SheetItem,
	place Place,
	animate bool,
	leading func(),
	menu func(*ui.Menu),
	empty bool,
	emptyText string,
) {
	scale := s.Settings.UIFontSize
	scroll := ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
		list := ui.Column(c).Gap(theme.Space02.DIPs(scale))
		track := components.TrackSelection(list, components.NavigationSelection(c, semantic), animate)
		list.Children(func() {
			if leading != nil {
				leading()
			}
			if empty {
				ui.Box(c).Padding(theme.Space08.DIPs(scale)).Children(func() {
					components.Label(c, emptyText, theme.TextSmall, semantic.Text.Muted)
				})
				return
			}
			for index, sheet := range sheets {
				s.sheetRow(c, semantic, sheet, index, place, track, animate)
			}
		})
	})
	if menu != nil {
		scroll.ContextMenu(menu)
	}
}

func (s *State) sheetRow(
	c *ui.Context,
	semantic theme.Theme,
	sheet SheetItem,
	index int,
	place Place,
	track *components.SelectionTrack,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	selected := sheet.ID == s.ActivePage
	row := ui.Row(c).Key("sheet-"+sheet.ID).Role(ui.RoleButton).Label(sheet.Title).Selected(selected).
		WidthPercent(100).Shrink(0).
		PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
		Radius(metrics.Radius).Border(1, ui.Transparent).
		Focusable().FocusRing(false).Transition(rowTransition(animate))
	rest := semantic.GhostElement.Background
	if selected {
		track.Mark(row)
	}
	hover := components.HoverAmount(row, !selected && row.Hovered(), animate)
	row.Background(components.Blend(rest, semantic.GhostElement.Hover, hover))
	if row.FocusVisible() {
		row.Border(1, semantic.Border.Focused)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
			title := semantic.Text.Default
			if selected {
				title = semantic.Text.Heading
			}
			components.Label(c, sheet.Title, theme.TextSmall, title).SingleLine()
			if sheet.Excerpt != "" {
				components.Label(c, sheet.Excerpt, theme.TextXSmall, semantic.Text.Muted).SingleLine()
			}
		})
	})
	trashed := place.Kind == PlaceTrash
	row.Drag(sheetDrag{id: sheet.ID}).ContextMenu(s.sheetMenu(sheet, trashed))
	if place.Kind == PlaceGroup {
		// A sheet dropped on another in its group takes its place.
		if _, over := ui.DragOver[sheetDrag](row); over {
			row.Border(1, semantic.Border.Focused)
		}
		if dragged, ok := ui.Drop[sheetDrag](row); ok && dragged.id != sheet.ID {
			s.Library.MoveSheet(dragged.id, place.Group, index)
		}
	}
	if row.Clicked() || row.Shortcut(0, ui.KeyEnter) {
		s.Library.OpenSheet(sheet.ID, row.ClickModifiers()&ui.Cmd != 0)
	}
}

func (s *State) sheetMenu(sheet SheetItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Open").Chosen() {
			s.Library.OpenSheet(sheet.ID, false)
		}
		if m.Item("Open in New Tab").Chosen() {
			s.Library.OpenSheet(sheet.ID, true)
		}
		m.Separator()
		if trashed {
			if m.Item("Put Back").Chosen() {
				s.Library.RestoreSheet(sheet.ID)
			}
			m.Separator()
			if m.Item("Delete Immediately…").Chosen() {
				s.Library.DeleteSheet(sheet.ID)
			}
			return
		}
		if m.Item("Duplicate").Chosen() {
			s.Library.DuplicateSheet(sheet.ID)
		}
		s.moveMenu(m, func(group string) {
			s.Library.MoveSheet(sheet.ID, group, 1<<30)
		}, "")
		m.Separator()
		if m.Item("Move to Trash").Chosen() {
			s.Library.TrashSheet(sheet.ID)
		}
	}
}
