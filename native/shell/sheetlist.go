package shell

import (
	"fmt"
	"math"
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

func (s listing) listView() listView {
	return listView{place: s.browser.place, search: s.browser.searching}
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
	layout := s.paneLayout()
	return s.lastWidth >= (layout.Narrow+layout.SheetList)*scale/theme.DefaultUIFontSize
}

// ToggleSheetList shows or hides the sheet list.
func (s *State) ToggleSheetList() {
	if s != nil {
		visible := !s.SheetListVisible()
		s.SheetListOverride = &visible
		s.closeCanvas()
	}
}

// showSheetList shows the sheet list, as choosing what it lists does.
func (s *State) showSheetList() {
	visible := true
	s.SheetListOverride = &visible
}

// sheetColumn is the sheet list: the chosen place's sheets beside the
// library, fading from one place to the next, under head's room for its
// line in the panes' title line (paneHeader), and the place's band where
// the place's title has gone down from there.
func (s listing) sheetColumn(c *ui.Context, semantic theme.Theme, width, reveal, head float32) {
	if width <= 0.01 || reveal <= 0.001 || s.Library == nil {
		return
	}
	scale := s.Settings.UIFontSize
	layout := s.paneLayout()
	full := layout.SheetList * scale / theme.DefaultUIFontSize
	animate := s.animate(c)
	pane := ui.Column(c).Key("sheets").Width(width).HeightPercent(100).Shrink(0).Clip().
		Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant)
	pane.Children(func() {
		ui.Column(c).Width(full).HeightPercent(100).Opacity(reveal).Children(func() {
			ui.Box(c).Key("list-head").WidthPercent(100).Height(head).Shrink(0)
			s.placeBand(c, semantic, s.listView(), s.listDock, animate)
			s.listStage(c, semantic, animate)
		})
	})
}

// noteView notes, once a frame before the title line and the list draw, when
// the list comes to show something else, and from which side it comes.
func (s listing) noteView(c *ui.Context, animate bool) {
	s.browser.fresh, s.browser.rank = false, 0
	view := s.listView()
	if s.Library != nil && view != s.browser.shown {
		s.browser.side = s.side(s.browser.shown, view)
		s.browser.shown, s.browser.fresh, s.browser.shownAt = view, true, time.Time{}
		s.browser.sheetsGap, s.browser.sheetsNear = components.CreateGap{}, -1
		s.browser.sheetDropSlot, s.browser.sheetDropHeight = -1, 0
		s.browser.sheetDropVisible, s.browser.sheetDropRows = false, nil
		s.browser.picking = picking{}
		if animate {
			s.browser.shownAt = c.Now()
		}
	}
	s.browser.sheetsGap.Hush(s.nav.quiet)
}

// noteDrags notes, once a frame before the library and the lists draw,
// whether a row was dragged last frame: the create areas stay shut until a
// while after the drag ends, wherever it dropped.
func (s *State) noteDrags(c *ui.Context) {
	if s.nav.dragged {
		s.nav.dragged = false
		s.nav.quiet = c.Now().Add(dropQuiet)
		s.nav.groupsTop.Hush(s.nav.quiet)
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

// placeAt is a place as the library shows it: depth is how far within
// other groups a group stands.
type placeAt struct {
	place Place
	depth int
}

// places are the library's places in its order: the chips left to right,
// the groups top to bottom, then the trash.
func (s *State) places() []placeAt {
	var out []placeAt
	for _, group := range s.Library.Groups("") {
		if group.Inbox {
			out = append(out, placeAt{place: Place{Kind: PlaceGroup, Group: group.ID}})
		}
	}
	out = append(out, placeAt{place: Place{Kind: PlaceAll}}, placeAt{place: Place{Kind: PlaceRecent}})
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, group := range s.Library.Groups(parent) {
			if !group.Inbox {
				out = append(out, placeAt{place: Place{Kind: PlaceGroup, Group: group.ID}, depth: depth})
				walk(group.ID, depth+1)
			}
		}
	}
	walk("", 0)
	return append(out, placeAt{place: Place{Kind: PlaceTrash}})
}

// side is where a list comes in from as the list goes from one view to
// another: 1 from the right, for a place after the last, -1 from the left
// for one before, 0 for none, as to or from search. The places run as the
// library shows them (places).
func (s *State) side(from, to listView) float32 {
	if from.search || to.search {
		return 0
	}
	order := map[Place]int{}
	for i, at := range s.places() {
		order[at.place] = i
	}
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
func (s listing) listStage(c *ui.Context, semantic theme.Theme, animate bool) {
	view := s.listView()
	ui.Box(c).Key("list-stage").Grow(1).MinHeight(0).WidthPercent(100).Clip().Children(func() {
		s.listLayer(c, semantic, view, animate)
	})
}

// enter brings in the next row of a list just shown: each slides in from
// the side the place lies on and fades in a moment after the one above, so
// the place arrives top down; with no side, as for search, it rises.
// It reports whether the row is still on its way.
func (s listing) enter(c *ui.Context, row ui.Element) bool {
	rank := min(s.browser.rank, enterRanks)
	s.browser.rank++
	if s.browser.shownAt.IsZero() {
		return false
	}
	t := float32(c.Now().Sub(s.browser.shownAt)-time.Duration(rank)*enterStagger) / float32(motionEnter)
	if t >= 1 {
		return false
	}
	c.AnimationFrame()
	p := components.Decelerate(max(0, t))
	k := (1 - p) * s.Settings.UIFontSize / theme.DefaultUIFontSize
	if s.browser.side != 0 {
		row.Opacity(p).Translate(s.browser.side*enterSlide*k, 0)
		return true
	}
	row.Opacity(p).Translate(0, enterRise*k)
	return true
}

// listRowTransition is rowTransition, but rows that arrive with the list
// leave their coming in to enter.
func (s listing) listRowTransition(animate bool) ui.ElementTransition {
	if !animate || !s.browser.fresh {
		return rowTransition(animate)
	}
	return ui.ElementTransition{Duration: motionList, Position: true, Exit: &ui.Motion{Collapse: true}}
}

func (s listing) listLayer(c *ui.Context, semantic theme.Theme, view listView, animate bool) {
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

func (s listing) placeList(c *ui.Context, semantic theme.Theme, place Place, animate bool) {
	// A smart folder lists what its query finds, as it finds it: nothing is
	// made, filed or ordered in it.
	if group, ok := s.Library.Group(place.Group); ok && place.Kind == PlaceGroup && group.Smart() {
		s.smartQueryStrip(c, semantic, group, animate)
		sheets := s.Library.Sheets(place)
		if len(sheets) == 0 {
			s.emptyList(c, semantic, groupIcon(group), "Nothing found for “"+group.Query+"”", nil, nil, animate)
			return
		}
		s.sheetRows(c, semantic, sheetEntries(sheets), place, nil, nil, nil, false, animate)
		return
	}
	var entries []GroupEntry
	files := s.files(place)
	if files {
		entries = s.Library.Entries(place.Group)
	} else {
		entries = sheetEntries(s.Library.Sheets(place))
	}
	group := place.Group
	if !files {
		group = s.CurrentGroup()
	}
	create := func(index int) { s.Library.NewSheet(group, index) }
	var createChapter func(index int)
	if files {
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
	s.sheetRows(c, semantic, entries, place, nil, create, createChapter, !files, animate)
}

// smartQueryStrip says what a smart folder finds, in the words the
// search's filters use, with the way to change it.
func (s listing) smartQueryStrip(c *ui.Context, semantic theme.Theme, group GroupItem, animate bool) {
	scale := s.Settings.UIFontSize
	ui.Row(c).Key("smart-query").WidthPercent(100).Shrink(0).AlignItems(ui.Start).Gap(theme.Space06.DIPs(scale)).
		PaddingStart(theme.Space12.DIPs(scale)).PaddingEnd(theme.Space08.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale)).
		Children(func() {
			s.queryChips(c, semantic, group.Query, animate)
			components.Button{
				Key: "edit-query", Label: "Edit", Tooltip: "Edit the query", Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate, Action: func() { s.editSmartGroup(group) },
			}.View(c, semantic).Shrink(0)
		})
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

func (s listing) newChapter(group string, index int) {
	if id := s.Library.NewChapter(group, index); id != "" {
		s.startChapterRename(id, "Untitled Chapter")
		s.browser.revealing = id
	}
}

func (s listing) trashList(c *ui.Context, semantic theme.Theme, animate bool) {
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

// emptyList says there is nothing to list, quietly, with the area that
// makes the first sheet where there can be one.
func (s listing) emptyList(
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
	if !s.browser.fresh {
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

func (s listing) listMenu() func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("New Sheet").Chosen() {
			s.Library.NewSheet(s.CurrentGroup(), Last)
		}
		if s.files(s.browser.place) && m.Item("New Chapter").Chosen() {
			s.newChapter(s.browser.place.Group, Last)
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
func (s listing) sheetRows(
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
		TrackScroll(&s.browser.scroll).
		Transition(appearing(motionList, nil, &ui.Motion{Opacity: 0}))
	scroll = components.BindOverscroll(
		scroll,
		&s.browser.scroll,
		&s.browser.overscroll,
		scrollfx.Vertical,
		components.OverscrollList,
		scroll.LayoutBounds().H,
		scale,
	)
	scroll.Children(func() {
		rowGap := theme.Space06.DIPs(scale)
		inset, side := theme.Space06.DIPs(scale), theme.Space08.DIPs(scale)
		list := ui.Column(c).Gap(rowGap).Padding(inset, side, inset+s.foot, side)
		s.browser.overscroll.Apply(c, list, animate)
		selection := components.NavigationSelection(c, semantic)
		// The menu's ring stands midway between the rows, clear of both.
		selection.RingOutset = rowGap / 2
		track := components.TrackSelection(list, selection, animate)
		s.noteOrder(entries)
		ordered := create != nil && place.Kind == PlaceGroup
		if !ordered {
			s.browser.sheetDropSlot, s.browser.sheetDropHeight = -1, 0
			s.browser.sheetDropVisible, s.browser.sheetDropRows = false, nil
		}
		dragged, dragging := ui.DragOver[sheetMove](list)
		dragging = ordered && dragging
		dropSlot := s.browser.sheetDropSlot
		s.browser.listTop = list.LayoutBounds().Y
		if dragging {
			slot, height := s.sheetDropPlan(list, entries, dragged)
			dropSlot = slot
			s.browser.sheetDropSlot = slot
			if target, moves := s.reorderSlot(dragged.sheetMoveID(), place.Group, slot, entries); moves {
				s.browser.sheetDropHeight = height
				s.browser.sheetDropRoom = 0
				if isChapter(entries, dragged.sheetMoveID()) && target > 0 {
					// The placeholder holds the room the chapter will take,
					// so nothing moves again as it lands.
					s.browser.sheetDropRoom = chapterRoom(scale)
				}
				s.browser.sheetDropVisible = true
			} else {
				// Either side of the dragged row is its original place.
				s.browser.sheetDropVisible = false
			}
		} else if ordered && !s.browser.sheetDropVisible {
			s.browser.sheetDropSlot = -1
		}
		// Where the pointer was by a sheet's top last frame calls up the
		// area above it.
		near := s.browser.sheetsNear
		s.browser.sheetsNear = -1
		// A drag chooses an insertion point, and sheets picked are for
		// what is done to them: neither calls up a create area, even
		// while the pointer rests near a row's top.
		quiet := dragging || len(s.browser.picking.ids) > 0
		if quiet {
			near = -1
			s.browser.sheetsGap.Close()
		}
		at, open := s.browser.sheetsGap.Follow(c, near)
		open = open && ordered && !quiet
		rows := make([]ui.Element, 0, len(entries))
		var sheets []ui.Element
		list.Children(func() {
			if create != nil && createAtTop {
				area := s.createArea(c, semantic, "new-sheet", "New sheet", animate, func() { create(Last) })
				s.enter(c, area.Transition(s.listRowTransition(animate)))
			}
			if leading != nil {
				leading(track)
			}
			for index, entry := range entries {
				// What stands between sheets parts their runs of picks.
				if ordered && s.browser.sheetDropVisible && s.browser.sheetDropSlot == index {
					track.Pick(s.sheetDropPlaceholder(c, semantic, dragging, animate), "", false)
				}
				if open && at == index {
					sheetAction := func() {
						s.browser.sheetsGap.Use()
						create(index)
					}
					var area ui.Element
					if createChapter != nil {
						area = s.createPair(c, semantic, "gap-create-"+entry.ID, animate, sheetAction, func() {
							s.browser.sheetsGap.Use()
							createChapter(index)
						})
					} else {
						area = s.createArea(c, semantic, "new-sheet-at-"+entry.ID, "New sheet", animate, sheetAction)
					}
					area.Transition(unfoldTransition(animate))
					track.Pick(area, "", false)
					// The pointer that called it up is in the space between
					// the rows around it: that keeps it open too, so it
					// keeps the list's spacing rather than reaching into
					// the space.
					s.browser.sheetsGap.HoldAround(area, list, theme.Space06.DIPs(scale)/2+theme.Space08.DIPs(scale))
				}
				var row ui.Element
				if entry.Kind == EntryChapter {
					// An area open above a chapter makes a sheet in the
					// section before, so the room stays between the two.
					// A chapter at the head that a drop will put below what
					// lands above it takes its room while the drop is open.
					apart := index > 0 || ordered && s.browser.sheetDropVisible && s.browser.sheetDropSlot == 0
					row = s.chapterRow(c, semantic, entry.Chapter, chapterSheets(entries, index), apart, animate)
					track.Pick(row, "", false)
				} else {
					row = s.sheetRow(c, semantic, entry.Sheet, index, place, track, now, animate)
					sheets = append(sheets, row)
				}
				rows = append(rows, row)
			}
			if ordered && !dragging {
				if s.browser.sheetsGap.Over() {
					s.browser.sheetsNear, _ = s.browser.sheetsGap.Open()
				} else {
					s.browser.sheetsNear = s.sheetCreateGap(list, rows)
				}
			}
			if ordered && s.browser.sheetDropVisible && s.browser.sheetDropSlot == len(entries) {
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
				if !dragging && !s.browser.sheetDropVisible {
					s.trackSheetDropRows(list, entries, rows)
				}
				if moved, ok := ui.Drop[sheetMove](list); ok {
					if s.browser.sheetDropVisible {
						s.startLanding(c, list, moved.sheetMoveID())
					}
					s.moveSheetToSlot(moved.sheetMoveID(), place.Group, dropSlot, entries)
					// The row lands in the placeholder's space: it goes at
					// once rather than closing behind the row, which would
					// move the rows after it a second time.
					s.browser.sheetDropVisible, s.browser.sheetDropSlot = false, -1
					s.browser.sheetDropHeight, s.browser.sheetDropRoom = 0, 0
				}
			}
		})
		s.unpickOnPressElsewhere(sheets)
		s.ringMenu(list, track)
	})
	if place.Kind != PlaceTrash {
		scroll.ContextMenu(s.listMenu())
	}
	if scroll.Shortcut(0, ui.KeyEscape) {
		s.unpick()
	}
	if scroll.Shortcut(ui.Cmd, ui.KeyA) {
		s.pickAll()
	}
}

// sheetCreateGap chooses the create-sheet gap nearest the pointer: within
// reach of the middle of the gap between two rows, or of the first row's
// top as if a gap as wide stood above it, and anywhere above that. The
// rows keep the rest of their height for their own clicks.
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
	reach := theme.Space08.DIPs(scale)
	gap := theme.Space06.DIPs(scale)
	first := rows[0].LayoutBounds()
	if len(rows) > 1 {
		gap = rows[1].LayoutBounds().Y - first.Y - first.H
	}
	best, nearest := -1, reach+1
	if head := first.Y - gap/2 - listTop; pointerY <= head+reach {
		best, nearest = 0, max(0, pointerY-head)
	}
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
func (s listing) sheetDropPlan(
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

func (s listing) sheetDropSpan(id string) (sheetDropSpan, bool) {
	for _, span := range s.browser.sheetDropRows {
		if span.id == id {
			return span, true
		}
	}
	return sheetDropSpan{}, false
}

// trackSheetDropRows keeps the row geometry the next frame uses to choose
// an insertion slot.
func (s listing) trackSheetDropRows(list ui.Element, entries []GroupEntry, rows []ui.Element) {
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
	s.browser.sheetDropRows = spans
}

// sheetDropPlaceholder reserves the dragged row's space at the insertion
// target. It opens and closes on a spring, so rows move around it instead
// of underneath the dragged copy.
func (s listing) sheetDropPlaceholder(
	c *ui.Context,
	semantic theme.Theme,
	active, animate bool,
) ui.Element {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	height := float32(0)
	opacity := float32(0)
	if active {
		height, opacity = s.browser.sheetDropRoom+s.browser.sheetDropHeight, 1
	}
	room := s.browser.sheetDropRoom
	// It opens and closes by its own height, and only moves between
	// slots by its transition: one that also took it out would leave its
	// outline under a row that has just landed in its space.
	transition := ui.ElementTransition{Duration: 1}
	if animate {
		transition = ui.ElementTransition{Duration: motionList, Position: true}
	}
	area := ui.Box(c).Key("sheet-drop-placeholder").WidthPercent(100).
		Height(0).Shrink(0).Clip().Radius(metrics.Radius).
		Transition(transition)
	if animate {
		height = components.Follow(area, "height", height, components.MotionQuick)
		opacity = components.Follow(area, "opacity", opacity, components.MotionQuick)
	}
	area.Height(height).Opacity(opacity).
		Draw(func(p *ui.Painter, r ui.Rect) {
			// The outline takes the row's own extent, below the room a
			// chapter keeps above it.
			r.Y, r.H = r.Y+min(room, r.H), max(0, r.H-room)
			components.DropFrame(p, r, metrics.Radius, 1, semantic)
		})
	if !active && (!animate || area.Arrived("height")) {
		s.browser.sheetDropVisible = false
	}
	return area
}

// grab is where the pointer took a row: its id, how far below the row's
// top, and where in the list the row stands while dragged.
type grab struct {
	id     string
	y, top float32
}

// landing is a dropped row on its way from where it was let go to its
// place. from is the dragged copy's top in the list as it was let go,
// source the row's top before the drop; at is when the row was first laid
// out in its place, and dropped when it was let go.
type landing struct {
	id           string
	from, source float32
	at, dropped  time.Time
}

// noteGrab keeps where the pointer took row, for a drag that may follow,
// and where the row stands while it is dragged.
func (s listing) noteGrab(row ui.Element, id string, dragging bool) {
	if dragging {
		if s.browser.grab.id == id {
			s.browser.grab.top = row.LayoutBounds().Y - s.browser.listTop
		}
		return
	}
	if !row.Pressed() {
		return
	}
	if _, y, ok := row.PointerPosition(); ok {
		s.browser.grab = grab{id: id, y: y, top: row.LayoutBounds().Y - s.browser.listTop}
	}
}

// startLanding begins a dropped row's landing from where its dragged copy
// was let go.
func (s listing) startLanding(c *ui.Context, list ui.Element, id string) {
	if s.browser.grab.id != id {
		return
	}
	_, pointer, over := list.PointerPosition()
	if !over {
		return
	}
	s.browser.landing = landing{
		id: id, from: pointer - s.browser.grab.y, source: s.browser.grab.top, dropped: c.Now(),
	}
}

// landWait bounds how long a dropped row waits to be laid out in its
// place before it shows there without landing.
const landWait = 100 * time.Millisecond

// land carries a dropped row from where it was let go into its place,
// instead of from where it was dragged from. It reports whether the row
// is landing.
//
// Its place is known only once the list has laid the row out there,
// which can take more than one build: until its layout leaves where it was
// dragged from, it stays hidden, as the dragged copy has just gone, rather
// than be drawn from a place it is not.
func (s listing) land(c *ui.Context, row ui.Element, id string, animate bool) bool {
	l := &s.browser.landing
	if l.id != id {
		return false
	}
	now := c.Now()
	top := row.LayoutBounds().Y - s.browser.listTop
	if l.at.IsZero() {
		if !animate || now.Sub(l.dropped) > landWait {
			*l = landing{}
			return false
		}
		if math.Abs(float64(top-l.source)) < 0.5 {
			c.AnimationFrame()
			row.Transition(ui.ElementTransition{Duration: 1}).Opacity(0)
			return true
		}
		l.at = now
	}
	t := float32(now.Sub(l.at)) / float32(motionList)
	if !animate || t >= 1 {
		*l = landing{}
		return false
	}
	c.AnimationFrame()
	p := components.Decelerate(max(0, t))
	row.Transition(ui.ElementTransition{Duration: 1}).Translate(0, (l.from-top)*(1-p))
	return true
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

func (s listing) sheetRow(
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
	confirming := s.nav.armed == deleteKey || s.nav.armed == deletePickedKey(sheet.ID)
	row := ui.Column(c).Key("sheet-" + sheet.ID).Role(ui.RoleButton).Label(sheet.Title).Selected(selected).
		WidthPercent(100).Shrink(0).
		Focusable().FocusRing(false).Transition(s.listRowTransition(animate))
	// MyGo paints Selected rows in the accent; the shared track or the
	// card below owns this row's fill.
	row.Background(ui.Transparent)
	entering := s.enter(c, row)
	s.land(c, row, sheet.ID, animate)
	picked := s.picked(sheet.ID)
	track.Pick(row, sheet.ID, picked)
	pick := s.pickAmount(row, sheet.ID, animate)
	// The picked rows' fill is over the row while it is picked and until
	// it has gone from it.
	cover := track.Covers(row)
	covered := picked || cover > 0.001
	if selected {
		// Under the picked fill, which hides it, so a run letting go goes
		// straight to it.
		track.Mark(row)
	}
	hover := components.HoverAmount(row, !selected && row.Hovered(), animate)
	selection := components.NavigationSelection(c, semantic)
	dragging := s.dragging(row)
	s.noteGrab(row, sheet.ID, dragging)
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
				title := components.Blend(semantic.Text.Default, semantic.Text.Heading, pick)
				if selected {
					title = semantic.Text.Heading
				}
				components.Label(c, sheet.Title, theme.TextSmall, title).Grow(1).MinWidth(0).SingleLine()
				if trashed && s.nav.armed == deletePickedKey(sheet.ID) {
					label := fmt.Sprintf("Delete %d", len(s.pickedSheets()))
					s.confirmButton(c, semantic, deletePickedKey(sheet.ID), "", label, animate, s.deletePicked)
				} else if trashed && confirming {
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
			s.sheetTags(c, semantic, sheet.Tags)
			components.Label(c, words(sheet.Words), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().FontFeatures("tnum").Opacity(0.7)
		})
	})
	hoverFill := components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)
	if covered {
		// Over the picked fill the hover is a step of the accent; the grey
		// one waits under the fill, there as the fill goes, so the row goes
		// straight from the one look to the other either way.
		track.Beneath(row, hoverFill)
		hoverFill = components.Blend(semantic.GhostElement.Background, selection.PickedHover, hover*cover)
	}
	card.Background(hoverFill)
	press := pressTarget
	if animate {
		press = components.Follow(card, "press", pressTarget, components.MotionQuick)
	}
	if dragging {
		press = 0
	}
	if selected && !covered && (entering || (pressTarget > 0 || press > 0.001) && track.At(row)) {
		// Let the card carry its fill on entry or a press once the shared
		// highlight has arrived; a new selection must glide there first.
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
	// A menu on a picked sheet is for all the picked; one on a sheet not
	// picked is for that sheet alone, and leaves the picked as they are.
	if picked && len(s.pickedSheets()) > 1 {
		row.ContextMenu(s.ringedMenu(s.pickedSheets(), func(m *ui.Menu) { s.batchMenu(m, sheet.ID, trashed) }))
	} else {
		row.ContextMenu(s.ringedMenu([]string{sheet.ID}, s.sheetMenu(sheet, trashed)))
	}
	if !confirming && row.Clicked() {
		if !s.pickClick(sheet.ID, row.ClickModifiers()) {
			// Cmd picks, so Option opens alongside the sheets open.
			s.open(sheet.ID, "", row.ClickModifiers()&ui.Alt != 0)
		}
	} else if !confirming && row.Shortcut(0, ui.KeyEnter) {
		s.open(sheet.ID, "", false)
	}
	return row
}

// isChapter reports whether id is a chapter among entries.
func isChapter(entries []GroupEntry, id string) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return entry.Kind == EntryChapter
		}
	}
	return false
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
	s.nav.groupRename = false
}

// chapterRow is a divider in a group's ordered list. Its sheets are those
// below it, up to the next divider. It reads as the head of a section, set
// at the sheets' text edge with its count at their dates' edge, and answers
// the pointer as a sheet card does. Renamed, it is the same row: the name
// turns into its field where it stands as the row takes the field's inset
// and edge, so nothing else in the list moves.
func (s listing) chapterRow(
	c *ui.Context,
	semantic theme.Theme,
	chapter ChapterItem,
	sheets int,
	apart bool,
	animate bool,
) ui.Element {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	editing := s.nav.renaming == chapter.ID
	row := ui.Column(c).Key("chapter-" + chapter.ID).Label(chapter.Name).
		WidthPercent(100).Height(metrics.Height).Shrink(0).
		Radius(metrics.Radius).Clip().Transition(s.chapterTransition(animate))
	if apart {
		row.Margin(chapterRoom(scale), 0, 0, 0)
	}
	if !editing {
		row.Role(ui.RoleButton).Focusable().FocusRing(false)
	}
	s.enter(c, row)
	s.land(c, row, chapter.ID, animate)
	if s.browser.revealing == chapter.ID {
		// Just made: wherever it lands, its field comes into view.
		s.browser.revealing = ""
		components.Reveal(c, row, components.RevealMargin(c), ui.ScrollIntoViewOptions{})
	}
	hover := components.HoverAmount(row, !editing && (row.Hovered() || row.FocusVisible()), animate)
	edit := float32(0)
	if editing {
		edit = 1
	}
	dragging := s.dragging(row)
	s.noteGrab(row, chapter.ID, dragging)
	pressTarget := float32(0)
	if !editing && !dragging && row.Pressed() {
		pressTarget = 1
	}
	press := pressTarget
	if animate {
		edit = components.Follow(row, "edit", edit, components.MotionQuick)
		press = components.Follow(row, "press", pressTarget, components.MotionQuick)
	}
	rest := components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)
	row.Background(components.Blend(rest, semantic.Surface.Inset, edit)).
		Border(1, components.Blend(ui.Transparent, semantic.Border.Focused, edit)).
		Scale(1-0.012*press, 1-0.012*press)
	if !editing && row.FocusVisible() {
		row.Border(1, semantic.Border.Focused)
	}
	// The contents keep the row's whole height and its top, so a row that
	// unfolds shows them from the top down instead of sliding them. A line
	// box centered in the row sets its letters low, by the room it keeps
	// for descenders: they go up to put their optical middle, between the
	// capitals' middle and the lowercase's, at the row's.
	row.Children(func() {
		ui.Row(c).WidthPercent(100).Height(metrics.Height).Shrink(0).
			PaddingX(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
			Translate(0, -chapterLift*theme.TextSmall.Size(scale)).
			Children(func() { s.chapterContents(c, semantic, chapter, sheets, editing, hover, edit) })
	})
	if editing {
		return row
	}
	row.Drag(chapterDrag{id: chapter.ID})
	row.ContextMenu(s.chapterMenu(chapter))
	if row.DoubleClicked() || row.Shortcut(0, ui.KeyEnter) {
		s.startChapterRename(chapter.ID, chapter.Name)
	}
	return row
}

// chapterContents is a chapter's name, or its field while renamed, then
// its rule and its count.
func (s *State) chapterContents(
	c *ui.Context,
	semantic theme.Theme,
	chapter ChapterItem,
	sheets int,
	editing bool,
	hover, edit float32,
) {
	scale := s.Settings.UIFontSize
	if editing {
		s.chapterNameField(c, semantic, chapter)
	} else {
		ink := components.Blend(semantic.Text.Default, semantic.Text.Heading, hover)
		line := components.Blend(semantic.Border.Default, semantic.Accent.Foreground, hover)
		components.Label(c, chapter.Name, theme.TextSmall, ink).FontWeight(600).
			Shrink(1).MinWidth(0).SingleLine()
		// Centered as the name's line box is, the rule would sit at the
		// capitals' middle, which reads high beside lowercase: it goes down
		// to the letters' optical middle. Back from a rename, it draws in
		// again as the field goes.
		ui.Box(c).Height(1).Grow(1).MinWidth(theme.Space12.DIPs(scale)).
			Translate(0, chapterRule*theme.TextSmall.Size(scale)).
			Background(line).Opacity((0.6 + 0.4*hover) * (1 - edit))
	}
	components.Label(c, sheetCount(sheets), theme.TextXSmall, semantic.Text.Muted).
		Shrink(0).SingleLine().FontFeatures("tnum").Opacity(1 - 0.5*edit)
}

// chapterLift raises a chapter's letters, and chapterRule lowers its rule
// from their line box's middle, both in parts of the text size, to put the
// two on the row's middle.
const (
	chapterLift = 0.14
	chapterRule = 0.1
)

// chapterRoom is the room above a chapter that sets its section apart
// from the sheets before it: with the list's gap, four times the space
// between sheets, so it reads as a division and not as a gap a little off.
func chapterRoom(scale float32) float32 { return theme.Space20.DIPs(scale) }

// chapterTransition brings a chapter in as its row unfolds, its name or
// field showing only as the row has room for it.
func (s listing) chapterTransition(animate bool) ui.ElementTransition {
	if !animate || s.browser.fresh {
		return s.listRowTransition(animate)
	}
	return ui.ElementTransition{
		Duration: motionList,
		Position: true,
		Enter:    &ui.Motion{Collapse: true, Opacity: 0},
		Exit:     &ui.Motion{Collapse: true, Opacity: 0},
	}
}

// chapterNameField edits a chapter's name in the place and type its name
// is shown in.
func (s *State) chapterNameField(c *ui.Context, semantic theme.Theme, chapter ChapterItem) {
	scale := s.Settings.UIFontSize
	input := ui.TextInputBase(c.Key("name"), &s.nav.name).Grow(1).MinWidth(0).
		Font(theme.UIFontFamily).FontSize(theme.TextSmall.Size(scale)).FontWeight(600).
		FixedLineHeight(theme.TextSmall.LineHeight(scale)).
		TextColor(semantic.Text.Heading).Label("Chapter name")
	switch inlineName(input) {
	case nameCancel:
		s.nav.renaming = ""
	case nameCommit:
		s.nav.renaming = ""
		s.Library.RenameChapter(chapter.ID, s.nav.name)
	}
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
func (s listing) trashedGroupRow(c *ui.Context, semantic theme.Theme, group GroupItem, animate bool) {
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

func (s listing) sheetMenu(sheet SheetItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if s.chose(m.Item("Open")) {
			s.open(sheet.ID, "", false)
		}
		if s.chose(m.Item("Open Alongside")) {
			s.open(sheet.ID, "", true)
		}
		m.Separator()
		if trashed {
			if s.chose(m.Item("Put Back")) {
				s.Library.RestoreSheet(sheet.ID)
			}
			m.Separator()
			if s.chose(m.Item("Delete Immediately…")) {
				s.arm("delete-sheet-" + sheet.ID)
			}
			return
		}
		if s.chose(m.Item("Duplicate")) {
			s.Library.DuplicateSheet(sheet.ID)
		}
		s.moveMenu(m, func(group string) {
			s.browser.menuRing.chose = true
			s.Library.MoveSheet(sheet.ID, group, Last)
		}, "")
		if s.chose(m.Item("Show in Library")) {
			s.reveal(sheet.ID)
		}
		m.Separator()
		if s.chose(m.Item("Move to Trash")) {
			s.Library.TrashSheet(sheet.ID)
		}
	}
}

// listHeader is the sheet list's part of the panes' title line: the place
// it shows, or the search field, and its toggle at its end.
func (s listing) listHeader(c *ui.Context, semantic theme.Theme, width, reveal float32, animate bool) {
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
				Transition(swapTransition(animate, s.browser.side*enterSlide*scale/theme.DefaultUIFontSize))
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

// placeInfo is what the list says of a place: its title, icon and tint,
// and the count of what it holds.
func (s listing) placeInfo(semantic theme.Theme, place Place) (title string, icon components.IconName, tint *ui.Color, detail string) {
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
	if picked, ok := s.pickedDetail(sheets); ok {
		detail = picked
	}
	return title, icon, tint, detail
}

// placeHeader is the place's part of the panes' title line. Where the
// library is closed its title and count go down to the top of the list
// (placeBand), leaving the line to the buttons; dock is how far they have
// gone.
func (s listing) placeHeader(c *ui.Context, semantic theme.Theme, place Place, dock float32, animate bool) {
	title, icon, tint, detail := s.placeInfo(semantic, place)
	scale := s.Settings.UIFontSize
	// The line narrows to its toggle as the title goes (titleLines); the
	// title takes what is left, which closes to none.
	info := ui.Row(c).Key("head-info").Grow(1).MinWidth(0).Clip().AlignItems(ui.Center).
		Gap(headTitle.gap.DIPs(scale))
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
			components.Label(c, title, headTitle.text, semantic.Text.Heading).SingleLine().FontWeight(headTitle.weight)
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
		chromeButton{
			key: "hide-sheets", label: "Hide sheet list", icon: components.IconSidebarClose, action: s.ToggleSheetList,
		}.view(c, semantic, animate)
	})
}

// placeBand is the place's title and count at the top of the list, where
// the library is closed and the title line has the panes' toggles to
// share. It opens as the line's title gives way, so the two read as one
// title moving down and back up.
func (s listing) placeBand(c *ui.Context, semantic theme.Theme, view listView, dock float32, animate bool) {
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
			PaddingX(theme.Space12.DIPs(scale)).AlignItems(ui.Center).Gap(bandTitle.gap.DIPs(scale)).
			Opacity(shown).
			Transition(swapTransition(animate, s.browser.side*enterSlide*scale/theme.DefaultUIFontSize))
		layer.Children(func() {
			title, icon, tint, detail := s.placeInfo(semantic, view.place)
			s.rowIcon(c, semantic, icon, tint, animate)
			heading := ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				components.Label(c, title, bandTitle.text, semantic.Text.Heading).SingleLine().FontWeight(bandTitle.weight)
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
// day is the local midnight that starts t's day.
func day(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

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
