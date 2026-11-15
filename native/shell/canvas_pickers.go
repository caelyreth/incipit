package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A picker is a list of sheets standing on the canvas as a column: the
// sheet list's, on a browser of its own, with its own place and its own
// search. It is how a sheet comes onto the canvas: one is made by "New
// picker" in the writing actions, or where the canvas is right-clicked
// between its columns. A sheet clicked in a picker takes its place, the
// picker turning into the sheet's page; one opened alongside (Option,
// "Open Alongside") comes in beside it, and the picker stays.

type picker struct {
	id   int
	list browser
}

// NewPicker makes a picker after the column in front, or at the end, and
// brings it into view, its search ready to be written in.
func (s *State) NewPicker() {
	w := s.workspace()
	at := len(w.columns)
	if front := w.index(s.canvasFront()); front >= 0 {
		at = front + 1
	}
	s.newPickerAt(at)
}

// newPickerAt makes a picker at the workspace's column at, showing the
// place the sheet list shows, and brings it into view, its search ready to
// be written in.
func (s *State) newPickerAt(at int) {
	s.canvas.pickers++
	p := &picker{id: s.canvas.pickers, list: newBrowser(s.list.place)}
	p.list.focus = true
	s.workspace().insert(at, canvasColumn{picker: p})
	s.canvas.revealPicker = p.id
}

// closePicker takes a picker off the canvas.
func (s *State) closePicker(id int) {
	w := s.workspace()
	if at := pickerIndex(w.columns, id); at >= 0 {
		w.columns = slices.Delete(w.columns, at, at+1)
	}
}

func pickerIndex(cols []canvasColumn, id int) int {
	for i, col := range cols {
		if col.picker != nil && col.picker.id == id {
			return i
		}
	}
	return -1
}

// pickerList is a picker's list as the window draws it: what it opens comes
// onto the canvas by the picker.
func (s *State) pickerList(p *picker) listing {
	return listing{State: s, browser: &p.list, opens: func(id, query string, alongside bool) {
		s.openFromPicker(p.id, id, query, alongside)
	}}
}

// openFromPicker opens a sheet chosen in a picker: in its place, or after
// it, alongside, as it joins the workspace (keepCanvas). One standing on it
// already is brought in front, and taken into the picker's place when it
// is chosen to take it.
func (s *State) openFromPicker(picker int, id, query string, alongside bool) {
	if s.workspace().index(id) >= 0 {
		if !alongside {
			s.moveIntoPicker(picker, id)
		}
		s.canvas.reveal = id
	} else {
		if s.canvas.into == nil {
			s.canvas.into, s.canvas.beside = map[string]int{}, map[string]int{}
		}
		if alongside {
			s.canvas.beside[id] = picker
		} else {
			s.canvas.into[id] = picker
		}
	}
	switch {
	case query != "":
		s.Library.OpenFound(id, query, true)
	case s.isOpen(id):
		s.bringInFront(id)
	default:
		s.Library.OpenSheet(id, true)
	}
}

// moveIntoPicker takes a sheet's column from where it stands into a
// picker's place.
func (s *State) moveIntoPicker(picker int, id string) {
	w := s.workspace()
	at := pickerIndex(w.columns, picker)
	if at < 0 {
		return
	}
	w.columns[at] = canvasColumn{sheet: id}
	for i, col := range w.columns {
		if i != at && col.picker == nil && col.sheet == id {
			w.columns = slices.Delete(w.columns, i, i+1)
			break
		}
	}
}

// pickerColumn draws a picker: a head that chooses its place and closes
// it, its search, and its list.
func (s *State) pickerColumn(c *ui.Context, semantic theme.Theme, p *picker, w, origin float32, animate bool) {
	list := s.pickerList(p)
	// The search is the list while it holds a query.
	p.list.searching = strings.TrimSpace(p.list.query) != ""
	list.noteView(c, animate)
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	scale := s.Settings.UIFontSize
	col := s.column(c, canvasColumn{picker: p}.key(), w, origin, true, animate).
		Role(ui.RoleGroup).Label("Sheet picker").Clip().Radius(10*k).
		Background(semantic.Surface.Panel).Border(1, semantic.Border.Variant)
	// A picker made comes into view, the column after it beside it.
	if s.canvas.revealPicker == p.id {
		options := ui.ScrollIntoViewOptions{Inline: ui.ScrollNearest}
		if animate {
			spring := components.Spring(components.MotionScene)
			options.Animation = &spring
		}
		col.ScrollIntoView(options)
		s.canvas.revealPicker = 0
	}
	col.Children(func() {
		head := ui.Row(c).Key("picker-head").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
			Gap(6 * k).PaddingStart(theme.Space06.DIPs(scale)).PaddingEnd(6 * k).PaddingY(6 * k)
		head.Children(func() {
			s.pickerPlace(c, semantic, list, animate)
			chromeButton{
				key: "close-picker", label: "Close picker", icon: components.IconClose, compact: true,
				action: func() { s.closePicker(p.id) },
			}.view(c, semantic, animate)
		})
		s.pan(head)
		ui.Row(c).Key("picker-search").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
			PaddingX(theme.Space08.DIPs(scale)).Padding(0, theme.Space08.DIPs(scale), theme.Space06.DIPs(scale), theme.Space08.DIPs(scale)).Children(func() {
			list.searchHeader(c, semantic, animate)
		})
		ui.Box(c).WidthPercent(100).Height(1).Shrink(0).Background(semantic.Border.Variant)
		list.listStage(c, semantic, animate)
	})
}

// pickerPlace is a picker's place, its mark, title and count, which,
// pressed, offers the library's places to show instead.
func (s *State) pickerPlace(c *ui.Context, semantic theme.Theme, list listing, animate bool) {
	scale := s.Settings.UIFontSize
	title, icon, tint, detail := list.placeInfo(semantic, list.browser.place)
	button := ui.Row(c).Key("picker-place").Grow(1).MinWidth(0).AlignItems(ui.Center).
		Gap(headTitle.gap.DIPs(scale)).Padding(theme.Space04.DIPs(scale)).Radius(6 * scale / theme.DefaultUIFontSize).
		Label("Choose what " + title + " shows")
	hover := components.HoverAmount(button, button.Hovered(), animate)
	button.Background(components.Blend(semantic.Surface.Panel, semantic.Element.Hover, hover))
	button.Menu(func(m *ui.Menu) {
		for _, at := range s.places() {
			name, _, _, _ := list.placeInfo(semantic, at.place)
			if m.Item(strings.Repeat("    ", at.depth) + name).Chosen() {
				list.stopSearching()
				list.show(at.place)
			}
		}
	})
	button.Children(func() {
		s.rowIcon(c, semantic, icon, tint, animate)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			components.Label(c, title, headTitle.text, semantic.Text.Heading).SingleLine().FontWeight(600)
			components.Label(c, detail, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
		})
		ui.Icon(c, components.IconChevronDown.SVG()).FontSize(14 * scale / theme.DefaultUIFontSize).
			Shrink(0).TextColor(semantic.Icon.Muted)
	})
}
