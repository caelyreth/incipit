package shell

import (
	"strconv"
	"strings"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// listView is what the sheet list shows: a place's sheets, or search
// results.
type listView struct {
	place  Place
	search bool
}

func (s *State) listView() listView {
	return listView{place: s.nav.place, search: s.nav.searching}
}

func (v listView) key() string {
	if v.search {
		return "search"
	}
	return placeKey(v.place)
}

// SheetListVisible reports whether the sheet list is shown beside the
// library.
func (s *State) SheetListVisible() bool {
	return s != nil && s.Library != nil && !s.nav.listClosed && s.SidebarVisible()
}

// ToggleSheetList shows or hides the sheet list.
func (s *State) ToggleSheetList() {
	if s != nil {
		s.nav.listClosed = !s.nav.listClosed
	}
}

// sheetColumn is the sheet list: the chosen place's sheets beside the
// library, fading from one place to the next.
func (s *State) sheetColumn(c *ui.Context, semantic theme.Theme, width, reveal float32) {
	if width <= 0.01 || reveal <= 0.001 || s.Library == nil {
		return
	}
	scale := s.Settings.UIFontSize
	full := theme.DefaultLayout.SheetList * scale / theme.DefaultUIFontSize
	animate := s.animate(c)
	ui.Column(c).Width(width).HeightPercent(100).Shrink(0).Clip().
		Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		ui.Column(c).Width(full).HeightPercent(100).Opacity(reveal).Children(func() {
			s.listStage(c, semantic, animate)
		})
	})
}

// listStage draws what the list shows and, while it changes, what it
// showed: the old fades and lifts away as the new fades and settles in.
func (s *State) listStage(c *ui.Context, semantic theme.Theme, animate bool) {
	view := s.listView()
	if view != s.nav.shown {
		s.nav.from, s.nav.shown, s.nav.moved = s.nav.shown, view, true
	}
	stage := ui.Box(c).Grow(1).MinHeight(0).WidthPercent(100).Clip()
	progress := float32(1)
	if animate {
		if s.nav.moved {
			stage.Jump("fade", 0)
		}
		progress = components.Follow(stage, "fade", 1, motionPlace)
	}
	s.nav.moved = false
	progress = max(0, min(progress, 1))
	rise := 10 * s.Settings.UIFontSize / theme.DefaultUIFontSize
	stage.Children(func() {
		if progress < 0.999 {
			s.listLayer(c, semantic, s.nav.from, -rise*progress/2, 1-progress, animate)
		}
		s.listLayer(c, semantic, view, rise*(1-progress), progress, animate)
	})
}

func (s *State) listLayer(c *ui.Context, semantic theme.Theme, view listView, offset, opacity float32, animate bool) {
	ui.Column(c).Key("list-"+view.key()).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		Translate(0, offset).Opacity(opacity).Children(func() {
		switch {
		case view.search:
			s.searchList(c, semantic, animate)
		case view.place.Kind == PlaceTrash:
			s.trashList(c, semantic, animate)
		default:
			s.placeList(c, semantic, view.place, animate)
		}
	})
}

func (s *State) placeList(c *ui.Context, semantic theme.Theme, place Place, animate bool) {
	sheets := s.Library.Sheets(place)
	if len(sheets) == 0 {
		text, icon := "Nothing changed in the last seven days", components.IconClock
		switch place.Kind {
		case PlaceAll:
			text, icon = "No sheets yet", components.IconSheets
		case PlaceGroup:
			group, _ := s.Library.Group(place.Group)
			text, icon = "No sheets in "+group.Name, groupIcon(group)
		}
		s.emptyList(c, semantic, icon, text, "New Sheet", func() { s.Library.NewSheet(s.CurrentGroup()) }, animate)
		return
	}
	s.sheetRows(c, semantic, sheets, place, nil, animate)
}

func (s *State) trashList(c *ui.Context, semantic theme.Theme, animate bool) {
	groups := s.Library.TrashedGroups()
	sheets := s.Library.Sheets(Place{Kind: PlaceTrash})
	if len(groups) == 0 && len(sheets) == 0 {
		s.emptyList(c, semantic, components.IconTrash, "The trash is empty", "", nil, animate)
		return
	}
	s.sheetRows(c, semantic, sheets, Place{Kind: PlaceTrash}, func(track *components.SelectionTrack) {
		for _, group := range groups {
			s.trashedGroupRow(c, semantic, group, animate)
		}
	}, animate)
}

func (s *State) searchList(c *ui.Context, semantic theme.Theme, animate bool) {
	query := strings.TrimSpace(s.PageQuery)
	if query == "" {
		s.emptyList(c, semantic, components.IconSearch, "Search the words of every sheet", "", nil, animate)
		return
	}
	sheets := s.Library.Search(query)
	if len(sheets) == 0 {
		s.emptyList(c, semantic, components.IconSearch, "No sheets hold “"+query+"”", "", nil, animate)
		return
	}
	s.sheetRows(c, semantic, sheets, Place{Kind: PlaceAll}, nil, animate)
}

// emptyList says there is nothing to list, quietly, with what makes
// something when there is such a thing.
func (s *State) emptyList(
	c *ui.Context,
	semantic theme.Theme,
	icon components.IconName,
	text, action string,
	act func(),
	animate bool,
) {
	scale := s.Settings.UIFontSize
	ui.Column(c).Key("empty").Grow(1).FillHeight().AlignItems(ui.Center).Justify(ui.Center).
		PaddingX(theme.Space24.DIPs(scale)).Gap(theme.Space08.DIPs(scale)).
		Transition(fadeTransition(animate)).Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(36 * scale / theme.DefaultUIFontSize).
			TextColor(semantic.Icon.Muted).Opacity(0.35)
		components.Label(c, text, theme.TextSmall, semantic.Text.Muted).TextAlign(ui.Center)
		if act != nil {
			components.Button{
				Key: "empty-action", Label: action,
				Style: components.SubtleButton, Size: components.CompactButton,
				Animate: animate, Action: act,
			}.View(c, semantic)
		}
	}).ContextMenu(s.listMenu())
}

func (s *State) listMenu() func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("New Sheet").Chosen() {
			s.Library.NewSheet(s.CurrentGroup())
		}
	}
}

// sheetRows lists sheets under what leading draws. In a group, a sheet
// dropped on another takes its place, and one dropped below the last goes
// last.
func (s *State) sheetRows(
	c *ui.Context,
	semantic theme.Theme,
	sheets []SheetItem,
	place Place,
	leading func(*components.SelectionTrack),
	animate bool,
) {
	scale := s.Settings.UIFontSize
	now := c.Now()
	scroll := ui.Scroll(c).Grow(1).MinHeight(0)
	scroll.Children(func() {
		list := ui.Column(c).Gap(theme.Space02.DIPs(scale)).
			PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale))
		track := components.TrackSelection(list, components.NavigationSelection(c, semantic), animate)
		list.Children(func() {
			if leading != nil {
				leading(track)
			}
			for index, sheet := range sheets {
				s.sheetRow(c, semantic, sheet, index, place, track, now, animate)
			}
		})
	})
	if place.Kind != PlaceTrash {
		scroll.ContextMenu(s.listMenu())
	}
	if place.Kind == PlaceGroup {
		if dragged, ok := ui.Drop[sheetDrag](scroll); ok {
			s.Library.MoveSheet(dragged.id, place.Group, 1<<30)
		}
		if tab, ok := ui.Drop[tabDrag](scroll); ok {
			s.Library.MoveSheet(tab.id, place.Group, 1<<30)
		}
	}
}

func (s *State) sheetRow(
	c *ui.Context,
	semantic theme.Theme,
	sheet SheetItem,
	index int,
	place Place,
	track *components.SelectionTrack,
	now time.Time,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	selected := sheet.ID == s.ActivePage
	trashed := place.Kind == PlaceTrash
	deleteKey := "delete-sheet-" + sheet.ID
	confirming := s.nav.armed == deleteKey
	row := ui.Column(c).Key("sheet-"+sheet.ID).Role(ui.RoleButton).Label(sheet.Title).Selected(selected).
		WidthPercent(100).Shrink(0).
		PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space08.DIPs(scale)).
		Gap(theme.Space02.DIPs(scale)).
		Radius(metrics.Radius).Border(1, ui.Transparent).
		Focusable().FocusRing(false).Transition(rowTransition(animate))
	if selected {
		track.Mark(row)
	}
	hover := components.HoverAmount(row, !selected && row.Hovered(), animate)
	row.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
	if row.FocusVisible() {
		row.Border(1, semantic.Border.Focused)
	}
	row.Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).Children(func() {
			title := semantic.Text.Default
			if selected {
				title = semantic.Text.Heading
			}
			components.Label(c, sheet.Title, theme.TextSmall, title).Grow(1).MinWidth(0).SingleLine()
			if trashed && confirming {
				s.confirmButton(c, semantic, deleteKey, "", "Delete", animate, func() { s.Library.DeleteSheet(sheet.ID) })
			} else {
				components.Label(c, when(sheet.Updated, now), theme.TextXSmall, semantic.Text.Muted).
					SingleLine().Shrink(0).FontFeatures("tnum")
			}
		})
		if sheet.Excerpt != "" {
			components.Label(c, sheet.Excerpt, theme.TextXSmall, semantic.Text.Muted).
				WidthPercent(100).MaxLines(2)
		}
		components.Label(c, words(sheet.Words), theme.TextXSmall, semantic.Text.Muted).
			SingleLine().FontFeatures("tnum").Opacity(0.7)
	})
	if !trashed {
		row.Drag(sheetDrag{id: sheet.ID})
	}
	row.ContextMenu(s.sheetMenu(sheet, trashed))
	if place.Kind == PlaceGroup {
		s.dropHighlight(c, semantic, row, true, false, true, animate)
		if dragged, ok := ui.Drop[sheetDrag](row); ok && dragged.id != sheet.ID {
			s.Library.MoveSheet(dragged.id, place.Group, index)
		}
		if tab, ok := ui.Drop[tabDrag](row); ok && tab.id != sheet.ID {
			s.Library.MoveSheet(tab.id, place.Group, index)
		}
	}
	if !confirming && (row.Clicked() || row.Shortcut(0, ui.KeyEnter)) {
		s.Library.OpenSheet(sheet.ID, row.ClickModifiers()&ui.Cmd != 0)
	}
}

// trashedGroupRow is a group in the trash, with what it would take with it.
func (s *State) trashedGroupRow(c *ui.Context, semantic theme.Theme, group GroupItem, animate bool) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	key := "delete-group-" + group.ID
	row := ui.Row(c).Key("trashed-" + group.ID).Role(ui.RoleButton).Label(group.Name).
		WidthPercent(100).Shrink(0).
		PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space08.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
		Radius(metrics.Radius).Transition(rowTransition(animate))
	hover := components.HoverAmount(row, row.Hovered(), animate)
	row.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
	row.Children(func() {
		s.rowIcon(c, semantic, groupIcon(group), groupTint(semantic, group), animate)
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
			components.Label(c, group.Name, theme.TextSmall, semantic.Text.Default).SingleLine()
			components.Label(c, "Group · "+sheetCount(group.Count), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().FontFeatures("tnum")
		})
		// A group is deleted with all it holds.
		armed := "Delete"
		if group.Count > 0 {
			armed = "Delete All"
		}
		s.confirmButton(c, semantic, key, "", armed, animate, func() { s.Library.DeleteGroup(group.ID) })
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Put Back").Chosen() {
			s.Library.RestoreGroup(group.ID)
		}
		m.Separator()
		if m.Item(deleteGroupLabel(group)).Chosen() {
			s.arm(key)
		}
	})
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
				s.arm("delete-sheet-" + sheet.ID)
			}
			return
		}
		if m.Item("Duplicate").Chosen() {
			s.Library.DuplicateSheet(sheet.ID)
		}
		s.moveMenu(m, func(group string) {
			s.Library.MoveSheet(sheet.ID, group, 1<<30)
		}, "")
		if m.Item("Show in Library").Chosen() {
			s.reveal(sheet.ID)
		}
		m.Separator()
		if m.Item("Move to Trash").Chosen() {
			s.Library.TrashSheet(sheet.ID)
		}
	}
}

// listHeader is the sheet list's line in the title bar: what it shows and
// what it makes, or the search field.
func (s *State) listHeader(c *ui.Context, semantic theme.Theme, width, reveal float32, animate bool) {
	if width <= 0.01 || s.Library == nil {
		return
	}
	scale := s.Settings.UIFontSize
	full := theme.DefaultLayout.SheetList * scale / theme.DefaultUIFontSize
	view := s.listView()
	ui.Box(c).Width(width).HeightPercent(100).Shrink(0).Clip().
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		ui.Box(c).Width(full).HeightPercent(100).Opacity(reveal).Children(func() {
			layer := ui.Row(c).Key("head-" + view.key()).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				PaddingStart(theme.Space12.DIPs(scale)).PaddingEnd(theme.Space06.DIPs(scale)).
				AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Transition(fadeTransition(animate))
			layer.Children(func() {
				if view.search {
					s.searchHeader(c, semantic, animate)
					return
				}
				s.placeHeader(c, semantic, view.place, animate)
			})
		})
	})
}

func (s *State) searchHeader(c *ui.Context, semantic theme.Theme, animate bool) {
	field := components.TextField{
		Key: "page-search", Label: "Search sheets", Placeholder: "Search all sheets…",
		Value: &s.PageQuery, Icon: components.IconSearch.SVG(),
		ClearIcon: components.IconClose.SVG(), Animate: animate,
		FocusRequested: s.FindPageRequested,
	}.View(c, semantic).Grow(1)
	s.FindPageRequested = false
	if field.Shortcut(0, ui.KeyEscape) {
		s.stopSearching()
	}
	components.Button{
		Key: "done-searching", Label: "Done",
		Style: components.SubtleButton, Size: components.CompactButton,
		Animate: animate, Action: s.stopSearching,
	}.View(c, semantic)
}

func (s *State) placeHeader(c *ui.Context, semantic theme.Theme, place Place, animate bool) {
	scale := s.Settings.UIFontSize
	title, icon, tint := "All", components.IconSheets, (*ui.Color)(nil)
	sheets := s.Library.Sheets(place)
	detail := sheetCount(len(sheets))
	switch place.Kind {
	case PlaceRecent:
		title, icon = "Last 7 Days", components.IconClock
	case PlaceTrash:
		title, icon = "Trash", components.IconTrash
		detail = itemCount(len(sheets) + len(s.Library.TrashedGroups()))
	case PlaceGroup:
		group, _ := s.Library.Group(place.Group)
		title, icon, tint = group.Name, groupIcon(group), groupTint(semantic, group)
	}
	if place.Kind != PlaceTrash && len(sheets) > 0 {
		total := 0
		for _, sheet := range sheets {
			total += sheet.Words
		}
		detail += " · " + words(total)
	}
	s.rowIcon(c, semantic, icon, tint, animate)
	heading := ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		components.Label(c, title, theme.TextSmall, semantic.Text.Heading).SingleLine()
		components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
	})
	// The title is the group, with the group's menu.
	if place.Kind == PlaceGroup {
		if group, ok := s.Library.Group(place.Group); ok {
			heading.ContextMenu(s.groupMenu(group))
		}
	}
	if place.Kind == PlaceTrash {
		if detail != itemCount(0) {
			s.confirmButton(c, semantic, emptyTrashKey, "Empty", "Delete All", animate, s.Library.EmptyTrash)
		}
	} else {
		components.IconButton{
			Key: "new-sheet", Label: "New sheet", Icon: components.IconNewSheet.SVG(),
			Style: components.SubtleButton, Size: components.CompactButton,
			IconColor: &semantic.Icon.Muted, Animate: animate,
			Action: func() { s.Library.NewSheet(s.CurrentGroup()) },
		}.View(c, semantic)
	}
	components.IconButton{
		Key: "hide-sheets", Label: "Hide sheet list", Icon: components.IconSidebarClose.SVG(),
		Style: components.SubtleButton, Size: components.CompactButton,
		IconColor: &semantic.Icon.Muted, Animate: animate, Action: s.ToggleSheetList,
	}.View(c, semantic)
	_ = scale
}

// when is a sheet's last change as the list says it: the time today, the
// day this week, the date before.
func when(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	at, now = at.Local(), now.Local()
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	days := int(day(now).Sub(day(at)).Hours() / 24)
	switch {
	case days <= 0:
		return at.Format("15:04")
	case days == 1:
		return "Yesterday"
	case days < 7:
		return at.Format("Monday")
	case at.Year() == now.Year():
		return at.Format("Jan 2")
	}
	return at.Format("Jan 2, 2006")
}

// words says how many words there are, grouped by thousands.
func words(n int) string {
	switch n {
	case 0:
		return "No words yet"
	case 1:
		return "1 word"
	}
	return thousands(n) + " words"
}

func sheetCount(n int) string {
	if n == 1 {
		return "1 sheet"
	}
	return thousands(n) + " sheets"
}

func itemCount(n int) string {
	switch n {
	case 0:
		return "Empty"
	case 1:
		return "1 item"
	}
	return thousands(n) + " items"
}

func thousands(n int) string {
	digits := strconv.Itoa(n)
	var out strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}
