package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A picker brings navigation, search and sheets together in one desk
// column, using the panes' controls on a browser of its own. One is made
// by "New picker" in the writing actions, or where the desk is
// right-clicked between its columns. A sheet clicked in a picker takes its
// place, the picker turning into the sheet's page; one opened alongside
// (Option, "Open Alongside") comes in beside it, and the picker stays.

// picker is a picker on the desk, standing after the sheet after.
type picker struct {
	id    int
	after string
	list  browser
}

// NewPicker makes a picker after the column in front, or at the end, and
// brings it into view, its search ready to be written in.
func (s *State) NewPicker() {
	columns := s.deskColumns()
	at := len(columns)
	if front := s.columnIndex(s.desk.at); front >= 0 {
		at = front + 1
	}
	s.newPickerAt(at)
}

// newPickerAt makes a picker at the desk's column at, showing the place
// the sheet list shows, and brings it into view, its search ready to be
// written in.
func (s *State) newPickerAt(at int) {
	s.desk.made++
	p := &picker{id: s.desk.made, list: newBrowser(s.list.place)}
	if p.list.place.Kind == PlaceGroup {
		s.pickerList(p).expandTo(p.list.place.Group)
	}
	p.list.focus = true
	// It follows what stands before it, and comes after the pickers there.
	columns := s.deskColumns()
	at = max(0, min(at, len(columns)))
	before := 0
	for _, col := range columns[:at] {
		if col.picker != nil {
			before++
			p.after = col.picker.after
		} else {
			p.after = col.sheet
		}
	}
	s.desk.pickers = slices.Insert(s.desk.pickers, before, p)
	s.deskColumns()
	// Its search has the keyboard, so it is in front.
	s.desk.at = deskColumn{picker: p}.key()
	s.bringInView(s.desk.at, false)
}

// closePicker takes a picker off the desk.
func (s *State) closePicker(id int) {
	at := slices.IndexFunc(s.desk.pickers, func(p *picker) bool { return p.id == id })
	if at < 0 {
		return
	}
	p := s.desk.pickers[at]
	if s.nav.host == &p.list {
		s.nav.host = nil
		s.nav.renaming, s.nav.focusGroup, s.nav.styling = "", "", ""
		s.nav.groupRename, s.nav.stylingOpen = false, false
	}
	// In front, it gives way to the sheet in front.
	if s.desk.at == (deskColumn{picker: p}).key() {
		s.desk.at = ""
		if s.ActivePage != "" {
			s.desk.at = deskColumn{sheet: s.ActivePage}.key()
		}
	}
	s.desk.pickers = slices.Delete(s.desk.pickers, at, at+1)
	s.deskColumns()
}

// pickerList is a picker's list as the window draws it: what it opens comes
// onto the desk by the picker.
func (s *State) pickerList(p *picker) listing {
	// Its rows scroll on under the writing actions floating over the
	// desk's foot, as a sheet's page does.
	foot := theme.DefaultLayout.DockClearance() * s.Settings.UIFontSize / theme.DefaultUIFontSize
	return listing{State: s, browser: &p.list, foot: foot, opens: func(id, query string, alongside bool) {
		s.openFromPicker(p, id, query, alongside)
	}}
}

// openFromPicker opens a sheet chosen in a picker where the picker
// stands, after the sheet it follows: in its place, or beside it, the
// picker staying, alongside. One open already is brought there.
func (s *State) openFromPicker(p *picker, id, query string, alongside bool) {
	switch {
	case query != "":
		s.Library.OpenFound(id, query, true)
	case s.isOpen(id):
		s.bringInFront(id)
	default:
		s.Library.OpenSheet(id, true)
	}
	s.Library.PlaceOnDesk(id, p.after)
	if !alongside {
		s.closePicker(p.id)
	}
	s.bringInView(deskColumn{sheet: id}.key(), false)
}

// pickerColumn draws a picker's navigation, search and sheet list under
// the title line; on the line, its head (columnHead), what closes it.
func (s *State) pickerColumn(c *ui.Context, semantic theme.Theme, p *picker, w, origin float32, animate bool) {
	list := s.pickerList(p)
	// The search is the list while it holds a query.
	p.list.searching = strings.TrimSpace(p.list.query) != ""
	list.noteView(c, animate)
	scale := s.Settings.UIFontSize
	col, front, shown := s.column(c, deskColumn{picker: p}, w, origin, animate)
	col.Role(ui.RoleGroup).Label("Sheet picker").Clip().Background(semantic.Surface.Panel)
	lead, trail := s.clearOfChrome(c, shown)
	head := float32(0)
	if col.Hovered() {
		head = 1
	}
	if animate {
		head = components.Follow(col, "head-shown", head, components.MotionNormal)
	}
	col.Children(func() {
		pad := theme.Space08.DIPs(scale)
		ui.Column(c).Key("picker-body").WidthPercent(100).Grow(1).MinHeight(0).Padding(titleLine(c), 0, 0, 0).
			Opacity(s.columnShows(col, front, animate)).Children(func() {
			ui.Row(c).Key("picker-search").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
				Padding(theme.Space06.DIPs(scale), pad, theme.Space06.DIPs(scale), pad).Children(func() {
				list.searchHeader(c, semantic, animate)
			})
			list.pickerNavigation(c, semantic, w, animate)
			ui.Box(c).WidthPercent(100).Height(1).Shrink(0).Background(semantic.Border.Variant)
			if !p.list.searching {
				ui.Row(c).Key("picker-place-row").WidthPercent(100).Shrink(0).
					Padding(theme.Space08.DIPs(scale), pad, theme.Space04.DIPs(scale), pad).
					Children(func() { list.pickerPlace(c, semantic, animate) })
			}
			list.listStage(c, semantic, animate).
				MinHeight(theme.DefaultControls.Scaled(scale).Row + 2*theme.Space06.DIPs(scale))
		})
		s.columnHead(c, semantic, "picker-head", lead, trail, head, nil, chromeButton{
			key: "close-picker", label: "Close picker",
			action: func() { s.closePicker(p.id) },
		}, animate)
	})
}

// pickerPlace is a picker's place, its mark, title and count.
func (s listing) pickerPlace(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	place := s.browser.place
	title, icon, tint, detail := s.placeInfo(semantic, place)
	ui.Row(c).Key("picker-place").Grow(1).MinWidth(0).AlignItems(ui.Center).
		Gap(headTitle.gap.DIPs(scale)).Padding(theme.Space04.DIPs(scale)).Children(func() {
		s.rowIcon(c, semantic, icon, tint, animate)
		heading := ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Children(func() {
			components.Label(c, title, headTitle.text, semantic.Text.Heading).SingleLine().FontWeight(600).Grow(1).MinWidth(0)
			components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum").Shrink(1).MinWidth(0)
		})
		if place.Kind == PlaceGroup {
			if group, ok := s.Library.Group(place.Group); ok {
				heading.ContextMenu(s.groupMenu(group))
			}
		}
		if place.Kind == PlaceTrash && detail != itemCount(0) {
			s.confirmButton(c, semantic, emptyTrashKey, "Empty", "Delete All", animate, s.Library.EmptyTrash)
		}
	})
}

func (s listing) pickerNavigation(c *ui.Context, semantic theme.Theme, width float32, animate bool) {
	scale := s.Settings.UIFontSize
	pad, gap := theme.Space08.DIPs(scale), theme.Space04.DIPs(scale)
	metrics := theme.DefaultControls.Scaled(scale)
	ui.Column(c).Key("picker-navigation").WidthPercent(100).MinHeight(0).Shrink(1).
		Padding(0, pad, theme.Space06.DIPs(scale), pad).Children(func() {
		s.placeChips(c, semantic, width, animate)
		ui.Row(c).Key("picker-groups-head").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
			PaddingStart(pad).Gap(gap).Children(func() {
			components.Label(c, "Groups", theme.TextXSmall, semantic.Text.Muted).Grow(1)
			components.IconButton{
				Key: "new-group", Label: "New group", Icon: components.IconPlus.SVG(),
				Style: components.SubtleButton, Size: components.CompactButton, Animate: animate,
				Action: func() { s.startRename(s.Library.NewGroup("", Last)) },
			}.View(c, semantic)
			trash := components.IconButton{
				Key: "trash", Label: "Trash", Icon: components.IconTrash.SVG(),
				Style: components.SubtleButton, Size: components.CompactButton, Animate: animate,
				Selected: s.chosen(Place{Kind: PlaceTrash}), Action: func() { s.goTo(Place{Kind: PlaceTrash}) },
			}.View(c, semantic)
			s.trashTarget(c, semantic, trash, animate)
		})
		scroll := ui.Scroll(c).Key("picker-groups").WidthPercent(100).MinHeight(0).
			MaxHeight(4*metrics.Height + 3*gap).TrackScroll(&s.browser.libraryScroll)
		scroll = components.BindOverscroll(scroll, &s.browser.libraryScroll, &s.browser.libraryOverscroll,
			scrollfx.Vertical, components.OverscrollList, scroll.LayoutBounds().H, scale)
		scroll.Children(func() {
			groups := ui.Column(c).WidthPercent(100).Gap(gap)
			s.browser.libraryOverscroll.Apply(c, groups, animate)
			track := components.TrackSelection(groups, components.NavigationSelection(c, semantic), animate)
			groups.Children(func() { s.groupTree(c, semantic, "", 0, track, animate) })
		})
	})
}
