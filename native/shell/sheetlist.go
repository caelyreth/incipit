package shell

import (
	"strconv"
	"strings"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
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

// SheetListVisible reports whether the sheet list is shown. It goes on
// its own, with the library or without it; left to the window, it shows
// when there is room for it beside the library and the page.
func (s *State) SheetListVisible() bool {
	if s == nil || s.Library == nil || s.Appearance.Zen {
		return false
	}
	if s.SheetListOverride != nil {
		return *s.SheetListOverride
	}
	scale := s.lastUIFontSize
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	layout := theme.DefaultLayout
	return s.lastWidth >= (layout.Narrow+layout.SheetList)*scale/theme.DefaultUIFontSize
}

// ToggleSheetList shows or hides the sheet list.
func (s *State) ToggleSheetList() {
	if s != nil {
		visible := !s.SheetListVisible()
		s.SheetListOverride = &visible
	}
}

// showSheetList shows the sheet list, as choosing what it lists does.
func (s *State) showSheetList() {
	visible := true
	s.SheetListOverride = &visible
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
	ui.Column(c).Key("sheets").Width(width).HeightPercent(100).Shrink(0).Clip().
		Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		ui.Column(c).Width(full).HeightPercent(100).Opacity(reveal).Children(func() {
			s.placeBand(c, semantic, s.listView(), s.listDock, animate)
			s.listStage(c, semantic, animate)
		})
	})
}

// noteView notes, once a frame before the header and the list draw, when
// the list comes to show something else, and from which side it comes.
func (s *State) noteView(c *ui.Context, animate bool) {
	s.nav.fresh, s.nav.rank = false, 0
	view := s.listView()
	if s.Library != nil && view != s.nav.shown {
		s.nav.side = s.side(s.nav.shown, view)
		s.nav.shown, s.nav.fresh, s.nav.shownAt = view, true, time.Time{}
		s.nav.sheetsGap, s.nav.sheetsNear = gap{}, -1
		s.nav.sheetDropSlot, s.nav.sheetDropHeight = -1, 0
		s.nav.sheetDropVisible, s.nav.sheetDropRows = false, nil
		if animate {
			s.nav.shownAt = c.Now()
		}
	}
	// A row was dragged last frame: the create areas stay shut until a
	// while after the drag ends, wherever it dropped.
	if s.nav.dragged {
		s.nav.dragged = false
		until := c.Now().Add(dropQuiet)
		s.nav.sheetsGap.hush(until)
		s.nav.groupsTop.hush(until)
	}
}

// dragging reports whether row is being dragged, and notes it for the
// create areas.
func (s *State) dragging(row ui.Element) bool {
	if row.Dragging() {
		s.nav.dragged = true
		return true
	}
	return false
}

// side is where a list comes in from as the list goes from one view to
// another: 1 from the right, for a place after the last, -1 from the left
// for one before, 0 for none, as to or from search. The places run as the
// library shows them: the chips left to right, the groups top to bottom,
// then the trash.
func (s *State) side(from, to listView) float32 {
	if from.search || to.search {
		return 0
	}
	order := map[Place]int{}
	add := func(place Place) {
		if _, ok := order[place]; !ok {
			order[place] = len(order)
		}
	}
	for _, group := range s.Library.Groups("") {
		if group.Inbox {
			add(Place{Kind: PlaceGroup, Group: group.ID})
		}
	}
	add(Place{Kind: PlaceAll})
	add(Place{Kind: PlaceRecent})
	var walk func(parent string)
	walk = func(parent string) {
		for _, group := range s.Library.Groups(parent) {
			if !group.Inbox {
				add(Place{Kind: PlaceGroup, Group: group.ID})
				walk(group.ID)
			}
		}
	}
	walk("")
	add(Place{Kind: PlaceTrash})
	a, okA := order[from.place]
	b, okB := order[to.place]
	switch {
	case !okA || !okB || a == b:
		return 0
	case b > a:
		return 1
	}
	return -1
}

// listStage draws what the list shows. A new place replaces the old at
// once, never over it: two lists at half strength read as a flicker, the
// more so where they share sheets. Its rows come in instead (enter).
func (s *State) listStage(c *ui.Context, semantic theme.Theme, animate bool) {
	view := s.listView()
	ui.Box(c).Key("list-stage").Grow(1).MinHeight(0).WidthPercent(100).Clip().Children(func() {
		s.listLayer(c, semantic, view, animate)
	})
}

// enter brings in the next row of a list just shown: each slides in from
// the side the place lies on and fades in a moment after the one above, so
// the place arrives top down; with no side, as for search, it rises.
// It reports whether the row is still on its way.
func (s *State) enter(c *ui.Context, row ui.Element) bool {
	rank := min(s.nav.rank, enterRanks)
	s.nav.rank++
	if s.nav.shownAt.IsZero() {
		return false
	}
	t := float32(c.Now().Sub(s.nav.shownAt)-time.Duration(rank)*enterStagger) / float32(motionEnter)
	if t >= 1 {
		return false
	}
	c.AnimationFrame()
	p := components.Decelerate(max(0, t))
	k := (1 - p) * s.Settings.UIFontSize / theme.DefaultUIFontSize
	if s.nav.side != 0 {
		row.Opacity(p).Translate(s.nav.side*enterSlide*k, 0)
		return true
	}
	row.Opacity(p).Translate(0, enterRise*k)
	return true
}

// listRowTransition is rowTransition, but rows that arrive with the list
// leave their coming in to enter.
func (s *State) listRowTransition(animate bool) ui.ElementTransition {
	if !animate || !s.nav.fresh {
		return rowTransition(animate)
	}
	return ui.ElementTransition{Duration: motionList, Position: true, Exit: &ui.Motion{Collapse: true}}
}

func (s *State) listLayer(c *ui.Context, semantic theme.Theme, view listView, animate bool) {
	ui.Column(c).Key("list-" + view.key()).Absolute().Top(0).Left(0).Right(0).Bottom(0).Children(func() {
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
	var entries []GroupEntry
	if place.Kind == PlaceGroup {
		entries = s.Library.Entries(place.Group)
	} else {
		entries = sheetEntries(s.Library.Sheets(place))
	}
	group := place.Group
	if place.Kind != PlaceGroup {
		group = s.CurrentGroup()
	}
	create := func(index int) { s.Library.NewSheet(group, index) }
	var createChapter func(index int)
	if place.Kind == PlaceGroup {
		createChapter = func(index int) { s.newChapter(group, index) }
	}
	if len(entries) == 0 {
		text, icon := "Nothing changed in the last seven days", components.IconClock
		switch place.Kind {
		case PlaceAll:
			text, icon = "No sheets yet", components.IconSheets
		case PlaceGroup:
			group, _ := s.Library.Group(place.Group)
			text, icon = "No sheets in "+group.Name, groupIcon(group)
		}
		s.emptyList(c, semantic, icon, text, create, createChapter, animate)
		return
	}
	s.sheetRows(c, semantic, entries, place, nil, create, createChapter, place.Kind != PlaceGroup, animate)
}

func sheetEntries(sheets []SheetItem) []GroupEntry {
	entries := make([]GroupEntry, 0, len(sheets))
	for _, sheet := range sheets {
		entries = append(entries, GroupEntry{
			Kind: EntrySheet, ID: sheet.ID, Title: sheet.Title, Sheet: sheet,
		})
	}
	return entries
}

func (s *State) newChapter(group string, index int) {
	if id := s.Library.NewChapter(group, index); id != "" {
		s.startChapterRename(id, "Untitled Chapter")
	}
}

func (s *State) trashList(c *ui.Context, semantic theme.Theme, animate bool) {
	groups := s.Library.TrashedGroups()
	sheets := s.Library.Sheets(Place{Kind: PlaceTrash})
	if len(groups) == 0 && len(sheets) == 0 {
		s.emptyList(c, semantic, components.IconTrash, "The trash is empty", nil, nil, animate)
		return
	}
	s.sheetRows(c, semantic, sheetEntries(sheets), Place{Kind: PlaceTrash}, func(track *components.SelectionTrack) {
		for _, group := range groups {
			s.trashedGroupRow(c, semantic, group, animate)
		}
	}, nil, nil, false, animate)
}

func (s *State) searchList(c *ui.Context, semantic theme.Theme, animate bool) {
	query := strings.TrimSpace(s.PageQuery)
	if query == "" {
		s.emptyList(c, semantic, components.IconSearch, "Search the words of every sheet", nil, nil, animate)
		return
	}
	sheets := s.Library.Search(query)
	if len(sheets) == 0 {
		s.emptyList(c, semantic, components.IconSearch, "No sheets hold “"+query+"”", nil, nil, animate)
		return
	}
	s.sheetRows(c, semantic, sheetEntries(sheets), Place{Kind: PlaceAll}, nil, nil, nil, false, animate)
}

// emptyList says there is nothing to list, quietly, with the area that
// makes the first sheet where there can be one.
func (s *State) emptyList(
	c *ui.Context,
	semantic theme.Theme,
	icon components.IconName,
	text string,
	create func(index int),
	createChapter func(index int),
	animate bool,
) {
	scale := s.Settings.UIFontSize
	empty := ui.Column(c).Key("empty").Grow(1).FillHeight().AlignItems(ui.Center).Justify(ui.Center).
		PaddingX(theme.Space24.DIPs(scale)).Gap(theme.Space08.DIPs(scale))
	if !s.nav.fresh {
		empty.Transition(fadeTransition(animate))
	}
	s.enter(c, empty)
	empty.Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(36 * scale / theme.DefaultUIFontSize).
			TextColor(semantic.Icon.Muted).Opacity(0.35)
		components.Label(c, text, theme.TextSmall, semantic.Text.Muted).TextAlign(ui.Center)
		if create != nil {
			if createChapter != nil {
				s.createPair(
					c, semantic, "empty-create", animate,
					func() { create(Last) }, func() { createChapter(Last) },
				).
					MaxWidth(360*scale/theme.DefaultUIFontSize).Margin(theme.Space08.DIPs(scale), 0, 0, 0)
			} else {
				area := s.createArea(c, semantic, "new-sheet", "New sheet", animate, func() { create(Last) })
				area.MaxWidth(180*scale/theme.DefaultUIFontSize).Margin(theme.Space08.DIPs(scale), 0, 0, 0)
			}
		}
	}).ContextMenu(s.listMenu())
}

func (s *State) listMenu() func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("New Sheet").Chosen() {
			s.Library.NewSheet(s.CurrentGroup(), Last)
		}
		if s.nav.place.Kind == PlaceGroup && m.Item("New Chapter").Chosen() {
			s.newChapter(s.nav.place.Group, Last)
		}
	}
}

// sheetRows lists a place's sheets, and a group's chapter dividers among
// them, under what leading draws. In a group, an entry dropped on another
// takes its place, and one dropped below the last goes last.
//
// create, where sheets can be made, puts a create area below the last
// sheet. In a group, where sheets keep the order they are put in, the
// pointer resting by the top of a sheet opens another above it, for a
// sheet in its place.
func (s *State) sheetRows(
	c *ui.Context,
	semantic theme.Theme,
	entries []GroupEntry,
	place Place,
	leading func(*components.SelectionTrack),
	create func(index int),
	createChapter func(index int),
	createAtTop bool,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	now := c.Now()
	// Keyed, and fading out, so a list that empties, as the trash does,
	// gives way to what says it is empty rather than going at once.
	scroll := ui.Scroll(c).Key("rows").Grow(1).MinHeight(0).
		TrackScroll(&s.sheetScroll).
		Transition(appearing(motionList, nil, &ui.Motion{Opacity: 0}))
	scroll = components.BindOverscroll(
		scroll,
		&s.sheetScroll,
		&s.sheetOverscroll,
		scrollfx.Vertical,
		components.OverscrollList,
		scroll.LayoutBounds().H,
		scale,
	)
	scroll.Children(func() {
		list := ui.Column(c).Gap(theme.Space06.DIPs(scale)).
			PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale))
		s.sheetOverscroll.Apply(c, list, animate)
		track := components.TrackSelection(list, components.NavigationSelection(c, semantic), animate)
		ordered := create != nil && place.Kind == PlaceGroup
		if !ordered {
			s.nav.sheetDropSlot, s.nav.sheetDropHeight = -1, 0
			s.nav.sheetDropVisible, s.nav.sheetDropRows = false, nil
		}
		dragged, dragging := ui.DragOver[sheetMove](list)
		dragging = ordered && dragging
		dropSlot := s.nav.sheetDropSlot
		if dragging {
			slot, height := s.sheetDropPlan(list, entries, dragged)
			dropSlot = slot
			s.nav.sheetDropSlot = slot
			if _, moves := s.reorderSlot(dragged.sheetMoveID(), place.Group, slot, entries); moves {
				s.nav.sheetDropHeight = height
				s.nav.sheetDropVisible = true
			} else {
				// Either side of the dragged row is its original place.
				s.nav.sheetDropVisible = false
			}
		} else if ordered && !s.nav.sheetDropVisible {
			s.nav.sheetDropSlot = -1
		}
		// Where the pointer was by a sheet's top last frame calls up the
		// area above it.
		near := s.nav.sheetsNear
		s.nav.sheetsNear = -1
		if dragging {
			// A drag chooses an insertion point; it never calls up a create
			// area, even while the pointer rests near a row's top.
			near = -1
			s.nav.sheetsGap.close()
		}
		at, open := s.nav.sheetsGap.follow(c, near)
		open = open && ordered && !dragging
		rows := make([]ui.Element, 0, len(entries))
		list.Children(func() {
			if create != nil && createAtTop {
				area := s.createArea(c, semantic, "new-sheet", "New sheet", animate, func() { create(Last) })
				s.enter(c, area.Transition(s.listRowTransition(animate)))
			}
			if leading != nil {
				leading(track)
			}
			for index, entry := range entries {
				if ordered && s.nav.sheetDropVisible && s.nav.sheetDropSlot == index {
					s.sheetDropPlaceholder(c, semantic, dragging, animate)
				}
				if open && at == index {
					sheetAction := func() {
						s.nav.sheetsGap.use()
						create(index)
					}
					var area ui.Element
					if createChapter != nil {
						area = s.createPair(c, semantic, "gap-create-"+entry.ID, animate, sheetAction, func() {
							s.nav.sheetsGap.use()
							createChapter(index)
						})
					} else {
						area = s.createArea(c, semantic, "new-sheet-at-"+entry.ID, "New sheet", animate, sheetAction)
					}
					area.Transition(unfoldTransition(animate))
					if at > 0 {
						// Reach into the gap's proximity band so the pointer
						// is already over the area as it unfolds.
						lift := theme.Space06.DIPs(scale)/2 + theme.Space08.DIPs(scale)
						area.Margin(-lift, 0, 0, 0)
					}
					s.nav.sheetsGap.hold(area)
				}
				var row ui.Element
				if entry.Kind == EntryChapter {
					row = s.chapterRow(c, semantic, entry.Chapter, chapterSheets(entries, index), index == 0, animate)
				} else {
					row = s.sheetRow(c, semantic, entry.Sheet, index, place, track, now, animate)
				}
				rows = append(rows, row)
			}
			if ordered && !dragging {
				if s.nav.sheetsGap.over {
					s.nav.sheetsNear = s.nav.sheetsGap.at
				} else {
					s.nav.sheetsNear = s.sheetCreateGap(list, rows)
				}
			}
			if ordered && s.nav.sheetDropVisible && s.nav.sheetDropSlot == len(entries) {
				s.sheetDropPlaceholder(c, semantic, dragging, animate)
			}
			if create != nil && !createAtTop {
				var area ui.Element
				if createChapter != nil {
					area = s.createPair(
						c, semantic, "bottom-create", animate,
						func() { create(Last) }, func() { createChapter(Last) },
					)
				} else {
					area = s.createArea(c, semantic, "new-sheet", "New sheet", animate, func() { create(Last) })
				}
				s.enter(c, area.Transition(s.listRowTransition(animate)))
			}
			if ordered {
				// Keep the pre-placeholder geometry for the whole drag:
				// moving rows must not chase the pointer into a new slot.
				if !dragging && !s.nav.sheetDropVisible {
					s.trackSheetDropRows(list, entries, rows)
				}
				if moved, ok := ui.Drop[sheetMove](list); ok {
					s.moveSheetToSlot(moved.sheetMoveID(), place.Group, dropSlot, entries)
				}
			}
		})
	})
	if place.Kind != PlaceTrash {
		scroll.ContextMenu(s.listMenu())
	}
}

// sheetCreateGap chooses the create-sheet gap nearest the pointer. The
// first row keeps the larger top-edge target; boundaries between rows use
// the center of the actual gap.
func (s *State) sheetCreateGap(list ui.Element, rows []ui.Element) int {
	if len(rows) == 0 {
		return -1
	}
	_, pointerY, over := list.PointerPosition()
	if !over {
		return -1
	}
	scale := s.Settings.UIFontSize
	listTop := list.LayoutBounds().Y
	first := rows[0].LayoutBounds()
	if pointerY < first.Y-listTop+20*scale/theme.DefaultUIFontSize {
		return 0
	}
	reach := theme.Space08.DIPs(scale)
	best, nearest := -1, reach+1
	for index := 1; index < len(rows); index++ {
		before := rows[index-1].LayoutBounds()
		after := rows[index].LayoutBounds()
		center := (before.Y+before.H+after.Y)/2 - listTop
		distance := pointerY - center
		if distance < 0 {
			distance = -distance
		}
		if distance < nearest {
			best, nearest = index, distance
		}
	}
	if nearest <= reach {
		return best
	}
	return -1
}

// sheetDropPlan chooses the insertion slot from the rows as last laid out,
// and keeps the dragged row's height for the in-flow placeholder.
func (s *State) sheetDropPlan(
	list ui.Element,
	entries []GroupEntry,
	dragged sheetMove,
) (slot int, height float32) {
	slot = len(entries)
	_, pointerY, over := list.PointerPosition()
	if over {
		for index, entry := range entries {
			span, ok := s.sheetDropSpan(entry.ID)
			if !ok {
				continue
			}
			if pointerY < span.top+span.height/2 {
				slot = index
				break
			}
		}
	}
	if dragged != nil {
		if span, ok := s.sheetDropSpan(dragged.sheetMoveID()); ok {
			height = span.height
		}
	}
	if height <= 0 && slot < len(entries) {
		if span, ok := s.sheetDropSpan(entries[slot].ID); ok {
			height = span.height
		}
	}
	if height <= 0 {
		height = theme.DefaultControls.Scaled(s.Settings.UIFontSize).Row * 2
	}
	return slot, height
}

func (s *State) sheetDropSpan(id string) (sheetDropSpan, bool) {
	for _, span := range s.nav.sheetDropRows {
		if span.id == id {
			return span, true
		}
	}
	return sheetDropSpan{}, false
}

// trackSheetDropRows keeps the row geometry the next frame uses to choose
// an insertion slot.
func (s *State) trackSheetDropRows(list ui.Element, entries []GroupEntry, rows []ui.Element) {
	listTop := list.LayoutBounds().Y
	spans := make([]sheetDropSpan, 0, len(rows))
	for index, row := range rows {
		bounds := row.LayoutBounds()
		spans = append(spans, sheetDropSpan{
			id:     entries[index].ID,
			top:    bounds.Y - listTop,
			height: bounds.H,
		})
	}
	s.nav.sheetDropRows = spans
}

// sheetDropPlaceholder reserves the dragged row's space at the insertion
// target. It opens and closes on a spring, so rows move around it instead
// of underneath the dragged copy.
func (s *State) sheetDropPlaceholder(
	c *ui.Context,
	semantic theme.Theme,
	active, animate bool,
) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	height := float32(0)
	opacity := float32(0)
	if active {
		height, opacity = s.nav.sheetDropHeight, 1
	}
	area := ui.Box(c).Key("sheet-drop-placeholder").WidthPercent(100).
		Height(0).Shrink(0).Clip().Radius(metrics.Radius).
		Transition(s.listRowTransition(animate))
	if animate {
		height = components.Follow(area, "height", height, components.MotionQuick)
		opacity = components.Follow(area, "opacity", opacity, components.MotionQuick)
	}
	area.Height(height).Opacity(opacity).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if r.H <= 0.5 {
				return
			}
			p.Fill(r, semantic.Accent.Muted.Alpha(0.14), metrics.Radius)
			components.DashedOutline(
				p, r, metrics.Radius, 1,
				semantic.Accent.Foreground.Alpha(0.65),
			)
		})
	if !active && (!animate || area.Arrived("height")) {
		s.nav.sheetDropVisible = false
	}
}

// reorderSlot converts a gap in the visible list to the index MoveSheet
// takes after removing the dragged sheet. Either side of its own row is
// not a move.
func (s *State) reorderSlot(
	id, group string,
	slot int,
	entries []GroupEntry,
) (int, bool) {
	if id == "" {
		return 0, false
	}
	if s.Library.EntryGroup(id) != group {
		return slot, true
	}
	source := -1
	for index, entry := range entries {
		if entry.ID == id {
			source = index
			break
		}
	}
	if source < 0 || slot == source || slot == source+1 {
		return 0, false
	}
	if source < slot {
		slot--
	}
	return slot, true
}

func (s *State) moveSheetToSlot(id, group string, slot int, entries []GroupEntry) {
	if target, ok := s.reorderSlot(id, group, slot, entries); ok {
		s.Library.MoveEntry(id, group, target)
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
) ui.Element {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	selected := sheet.ID == s.ActivePage
	trashed := place.Kind == PlaceTrash
	deleteKey := "delete-sheet-" + sheet.ID
	confirming := s.nav.armed == deleteKey
	row := ui.Column(c).Key("sheet-" + sheet.ID).Role(ui.RoleButton).Label(sheet.Title).Selected(selected).
		WidthPercent(100).Shrink(0).
		Focusable().FocusRing(false).Transition(s.listRowTransition(animate))
	// MyGo paints Selected rows in the accent; the shared track or the
	// card below owns this row's fill.
	row.Background(ui.Transparent)
	entering := s.enter(c, row)
	if selected {
		track.Mark(row)
	}
	hover := components.HoverAmount(row, !selected && row.Hovered(), animate)
	selection := components.NavigationSelection(c, semantic)
	dragging := s.dragging(row)
	pressTarget := float32(0)
	if !dragging && row.Pressed() {
		pressTarget = 1
	}
	var card ui.Element
	row.Children(func() {
		// MyGo only measures children built in this pass as flow content.
		card = ui.Column(c).Key("sheet-card-"+sheet.ID).
			WidthPercent(100).Shrink(0).
			PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space08.DIPs(scale)).
			Gap(theme.Space02.DIPs(scale)).
			Radius(metrics.Radius).Border(1, ui.Transparent)
		card.Children(func() {
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
	})
	card.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
	press := pressTarget
	if animate {
		press = components.Follow(card, "press", pressTarget, components.MotionQuick)
	}
	if dragging {
		press = 0
	}
	if selected && (entering || pressTarget > 0 || press > 0.001) {
		// The shared highlight cannot shrink with one row. While the card
		// enters or presses, it owns the same fill so both scale together.
		track.Hide()
		card.Background(selection.Fill)
	}
	card.Scale(1-0.012*press, 1-0.012*press)
	if row.FocusVisible() {
		card.Border(1, semantic.Border.Focused)
	}
	if !trashed {
		row.Drag(sheetDrag{id: sheet.ID})
	}
	row.ContextMenu(s.sheetMenu(sheet, trashed))
	if !confirming && (row.Clicked() || row.Shortcut(0, ui.KeyEnter)) {
		s.Library.OpenSheet(sheet.ID, row.ClickModifiers()&ui.Cmd != 0)
	}
	return row
}

// chapterSheets counts the sheets under the chapter at index, up to the
// next one.
func chapterSheets(entries []GroupEntry, index int) int {
	n := 0
	for _, entry := range entries[index+1:] {
		if entry.Kind == EntryChapter {
			break
		}
		n++
	}
	return n
}

func (s *State) startChapterRename(id, name string) {
	s.nav.renaming, s.nav.name = id, name
}

// chapterRow is a divider in a group's ordered list. Its sheets are those
// below it, up to the next divider. It reads as the head of a section, set
// at the sheets' text edge with its count at their dates' edge, and answers
// the pointer as a sheet card does.
func (s *State) chapterRow(
	c *ui.Context,
	semantic theme.Theme,
	chapter ChapterItem,
	sheets int,
	first bool,
	animate bool,
) ui.Element {
	if s.nav.renaming == chapter.ID {
		return s.renameChapterRow(c, semantic, chapter, first, animate)
	}
	scale := s.Settings.UIFontSize
	row := s.chapterFrame(c, "chapter-"+chapter.ID, first, animate).
		Role(ui.RoleButton).Label(chapter.Name).Focusable().FocusRing(false)
	s.enter(c, row)
	hover := components.HoverAmount(row, row.Hovered() || row.FocusVisible(), animate)
	dragging := s.dragging(row)
	pressTarget := float32(0)
	if !dragging && row.Pressed() {
		pressTarget = 1
	}
	press := pressTarget
	if animate {
		press = components.Follow(row, "press", pressTarget, components.MotionQuick)
	}
	if dragging {
		press = 0
	}
	row.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)).
		Scale(1-0.012*press, 1-0.012*press)
	ink := components.Blend(semantic.Text.Default, semantic.Text.Heading, hover)
	line := components.Blend(semantic.Border.Default, semantic.Accent.Foreground, hover)
	row.Children(func() {
		components.Label(c, chapter.Name, theme.TextSmall, ink).FontWeight(600).
			Shrink(1).MinWidth(0).SingleLine()
		ui.Box(c).Height(1).Grow(1).MinWidth(theme.Space12.DIPs(scale)).
			Background(line).Opacity(0.6 + 0.4*hover)
		components.Label(c, sheetCount(sheets), theme.TextXSmall, semantic.Text.Muted).
			Shrink(0).SingleLine().FontFeatures("tnum")
	})
	if row.FocusVisible() {
		row.Border(1, semantic.Border.Focused)
	}
	row.Drag(chapterDrag{id: chapter.ID})
	row.ContextMenu(s.chapterMenu(chapter))
	if row.DoubleClicked() || row.Shortcut(0, ui.KeyEnter) {
		s.startChapterRename(chapter.ID, chapter.Name)
	}
	return row
}

// chapterFrame is the row a chapter and its rename share: a control's
// height, so it is as easy to take as a button, with room above it that
// sets the section apart from the sheets before it.
func (s *State) chapterFrame(c *ui.Context, key string, first, animate bool) ui.Element {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	row := ui.Row(c).Key(key).WidthPercent(100).Height(metrics.Height).Shrink(0).
		PaddingX(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
		Radius(metrics.Radius).Border(1, ui.Transparent).Transition(s.listRowTransition(animate))
	if !first {
		row.Margin(theme.Space06.DIPs(scale), 0, 0, 0)
	}
	return row
}

func (s *State) renameChapterRow(
	c *ui.Context,
	semantic theme.Theme,
	chapter ChapterItem,
	first bool,
	animate bool,
) ui.Element {
	scale := s.Settings.UIFontSize
	row := s.chapterFrame(c, "rename-"+chapter.ID, first, animate).
		Border(1, semantic.Border.Focused).Background(semantic.Surface.Inset)
	row.Children(func() {
		input := ui.TextInputBase(c.Key("name"), &s.nav.name).Grow(1).MinWidth(0).
			Font(theme.UIFontFamily).FontSize(theme.TextSmall.Size(scale)).FontWeight(600).
			FixedLineHeight(theme.TextSmall.LineHeight(scale)).
			TextColor(semantic.Text.Heading).Label("Chapter name")
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
			s.Library.RenameChapter(chapter.ID, s.nav.name)
		}
		*focused = input.Focused()
	})
	return row
}

func (s *State) chapterMenu(chapter ChapterItem) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Rename…").Chosen() {
			s.startChapterRename(chapter.ID, chapter.Name)
		}
		m.Separator()
		if m.Item("Remove Chapter").Chosen() {
			s.Library.DeleteChapter(chapter.ID)
		}
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
		Radius(metrics.Radius).Transition(s.listRowTransition(animate))
	s.enter(c, row)
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
			s.Library.MoveSheet(sheet.ID, group, Last)
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
	view := s.listView()
	ui.Box(c).Key("sheets-line").Width(width).HeightPercent(100).Shrink(0).Clip().Children(func() {
		// The line fades out well before the list is shut, so its toggle
		// is gone before the closing line comes to cut it off.
		ui.Box(c).WidthPercent(100).HeightPercent(100).Opacity(reveal * reveal).Children(func() {
			layer := ui.Row(c).Key("head-" + view.key()).Absolute().Top(0).Left(0).WidthPercent(100).HeightPercent(100).
				PaddingStart(theme.Space12.DIPs(scale) * (1 - s.listDock)).PaddingEnd(listLineEnd.DIPs(scale)).
				AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale) * (1 - s.listDock)).
				Transition(swapTransition(animate, s.nav.side*enterSlide*scale/theme.DefaultUIFontSize))
			layer.Children(func() {
				if view.search {
					s.searchHeader(c, semantic, animate)
					return
				}
				s.placeHeader(c, semantic, view.place, s.listDock, animate)
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

// placeInfo is what the list says of a place: its title, icon and tint,
// and the count of what it holds.
func (s *State) placeInfo(semantic theme.Theme, place Place) (title string, icon components.IconName, tint *ui.Color, detail string) {
	title, icon = "All", components.IconSheets
	sheets := s.Library.Sheets(place)
	detail = sheetCount(len(sheets))
	switch place.Kind {
	case PlaceRecent:
		title, icon = "Recent", components.IconClock
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
	return title, icon, tint, detail
}

// placeHeader is the place's line in the title bar. Where the library is
// closed its title and count go down to the top of the list (placeBand),
// leaving the bar to the buttons; dock is how far they have gone.
func (s *State) placeHeader(c *ui.Context, semantic theme.Theme, place Place, dock float32, animate bool) {
	title, icon, tint, detail := s.placeInfo(semantic, place)
	scale := s.Settings.UIFontSize
	// The line narrows to its toggle as the title goes (titleLines); the
	// title takes what is left, which closes to none.
	info := ui.Row(c).Key("head-info").Grow(1).MinWidth(0).Clip().AlignItems(ui.Center).
		Gap(theme.Space06.DIPs(scale))
	if s.flying() {
		info.Opacity(0)
	} else if dock > 0.001 {
		info.Opacity(1 - dock)
	}
	info.Children(func() {
		if dock >= 0.999 {
			return
		}
		s.rowIcon(c, semantic, icon, tint, animate)
		heading := ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			components.Label(c, title, theme.TextSmall, semantic.Text.Heading).SingleLine()
			components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
		})
		// The title is the group, with the group's menu.
		if place.Kind == PlaceGroup && dock < 0.5 {
			if group, ok := s.Library.Group(place.Group); ok {
				heading.ContextMenu(s.groupMenu(group))
			}
		}
	})
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Children(func() {
		// Where the title is down in the list, the trash is emptied there.
		if place.Kind == PlaceTrash {
			s.lingering(c, "empty-trash-head", detail != itemCount(0) && dock < 0.5, animate, func() {
				s.confirmButton(c, semantic, emptyTrashKey, "Empty", "Delete All", animate, s.Library.EmptyTrash)
			})
		}
		components.IconButton{
			Key: "hide-sheets", Label: "Hide sheet list", Icon: components.IconSidebarClose.SVG(),
			Style: components.SubtleButton, Size: components.MediumButton,
			IconSize:  16 * scale / theme.DefaultUIFontSize,
			IconColor: &semantic.Icon.Muted, Animate: animate, Action: s.ToggleSheetList,
		}.View(c, semantic)
	})
}

// placeBand is the place's title and count at the top of the list, where
// the library is closed and the title bar has the window's controls to
// share. It opens as the header's give way, so the two read as one title
// moving down and back up.
func (s *State) placeBand(c *ui.Context, semantic theme.Theme, view listView, dock float32, animate bool) {
	if dock <= 0.001 || view.search {
		return
	}
	scale := s.Settings.UIFontSize
	height := placeBandHeight(scale)
	// On the way, placeFlight draws the title; the band only makes room.
	shown := dock
	if s.flying() {
		shown = 0
	}
	ui.Box(c).Key("place-band").WidthPercent(100).Height(height * dock).Shrink(0).Clip().Children(func() {
		layer := ui.Row(c).Key("band-" + view.key()).Absolute().Top(0).Left(0).Right(0).Height(height).
			PaddingX(theme.Space12.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
			Opacity(shown).
			Transition(swapTransition(animate, s.nav.side*enterSlide*scale/theme.DefaultUIFontSize))
		layer.Children(func() {
			title, icon, tint, detail := s.placeInfo(semantic, view.place)
			s.rowIcon(c, semantic, icon, tint, animate)
			heading := ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				components.Label(c, title, theme.TextLarge, semantic.Text.Heading).SingleLine().FontWeight(600)
				components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
			})
			if view.place.Kind == PlaceGroup {
				if group, ok := s.Library.Group(view.place.Group); ok {
					heading.ContextMenu(s.groupMenu(group))
				}
			}
			if view.place.Kind == PlaceTrash {
				s.lingering(c, "empty-trash-band", detail != itemCount(0) && dock >= 0.5, animate, func() {
					s.confirmButton(c, semantic, emptyTrashKey, "Empty", "Delete All", animate, s.Library.EmptyTrash)
				})
			}
		})
	})
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
