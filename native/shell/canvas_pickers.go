package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A picker is a list of sheets standing on the canvas as a column: the
// sheet list's, on a browser of its own, with its own place and its own
// search. It is how a sheet comes onto the canvas: the canvas opens with
// one beside the sheet in front, and as many more are made as are wanted
// ("New picker"). A sheet clicked in a picker takes its place, the picker
// turning into the sheet's page; one opened alongside (Option, "Open
// Alongside") comes in beside it, and the picker stays.

type picker struct {
	id   int
	list browser
}

// canvasColumn is one of the strip's columns: an open sheet's, or a
// picker.
type canvasColumn struct {
	sheet  string
	picker *picker
}

func (col canvasColumn) key() string {
	if col.picker != nil {
		return fmt.Sprintf("picker-%d", col.picker.id)
	}
	return "sheet-" + col.sheet
}

// arrange keeps the strip's columns with what is open: a sheet that closed
// leaves, and one that opened comes in where the picker it was chosen in
// asked, in that picker's place or after it, or else at the end. As the
// canvas comes, a picker stands before the sheet in front.
func (s *State) arrange() {
	open := make(map[string]bool, len(s.Open))
	for _, page := range s.Open {
		open[page.ID] = true
	}
	var cols []canvasColumn
	have := map[string]bool{}
	for _, col := range s.canvas.columns {
		if col.picker != nil || open[col.sheet] && !have[col.sheet] {
			cols = append(cols, col)
			have[col.sheet] = col.picker == nil
		}
	}
	for _, page := range s.Open {
		if have[page.ID] {
			continue
		}
		col := canvasColumn{sheet: page.ID}
		into, isInto := s.canvas.into[page.ID]
		beside, isBeside := s.canvas.beside[page.ID]
		switch {
		case isInto && pickerIndex(cols, into) >= 0:
			cols[pickerIndex(cols, into)] = col
		case isBeside && pickerIndex(cols, beside) >= 0:
			at := pickerIndex(cols, beside) + 1
			cols = append(cols[:at], append([]canvasColumn{col}, cols[at:]...)...)
		default:
			cols = append(cols, col)
		}
		delete(s.canvas.into, page.ID)
		delete(s.canvas.beside, page.ID)
		have[page.ID] = true
	}
	s.canvas.columns = cols
	if s.canvas.placePicker {
		s.canvas.placePicker = false
		s.pickerBeforeFront()
	}
}

// pickerBeforeFront has a picker stand just before the sheet in front, or
// first where none is, and brings it into view with the sheet beside it:
// the sheet list and the page, side by side as they were.
func (s *State) pickerBeforeFront() {
	at := 0
	for i, col := range s.canvas.columns {
		if col.sheet != "" && col.sheet == s.ActivePage {
			at = i
		}
	}
	if at > 0 && s.canvas.columns[at-1].picker != nil {
		s.canvas.revealPicker = s.canvas.columns[at-1].picker.id
	} else {
		s.canvas.revealPicker = s.insertPicker(at, false)
	}
	s.canvas.listPicker = s.canvas.revealPicker
}

// NewPicker makes a picker after the column in front, and brings it into
// view, its search ready to be written in.
func (s *State) NewPicker() {
	at := len(s.canvas.columns)
	for i, col := range s.canvas.columns {
		if col.sheet != "" && col.sheet == s.ActivePage {
			at = i + 1
		}
	}
	s.canvas.revealPicker = s.insertPicker(at, true)
}

// insertPicker puts a new picker at the strip's column at, showing the
// place the sheet list shows, and gives its id.
func (s *State) insertPicker(at int, focus bool) int {
	s.canvas.pickers++
	p := &picker{id: s.canvas.pickers, list: newBrowser(s.list.place)}
	p.list.focus = focus
	col := canvasColumn{picker: p}
	s.canvas.columns = append(s.canvas.columns[:at], append([]canvasColumn{col}, s.canvas.columns[at:]...)...)
	return p.id
}

// closePicker takes a picker off the strip.
func (s *State) closePicker(id int) {
	if at := pickerIndex(s.canvas.columns, id); at >= 0 {
		s.canvas.columns = append(s.canvas.columns[:at], s.canvas.columns[at+1:]...)
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
// it, alongside. A sheet open already is brought in front, and taken into
// the picker's place when it is chosen to take it.
func (s *State) openFromPicker(picker int, id, query string, alongside bool) {
	if s.isOpen(id) {
		if !alongside {
			s.moveIntoPicker(picker, id)
		}
		s.openOnCanvas(id)
		return
	}
	if s.canvas.into == nil {
		s.canvas.into, s.canvas.beside = map[string]int{}, map[string]int{}
	}
	if alongside {
		s.canvas.beside[id] = picker
	} else {
		s.canvas.into[id] = picker
	}
	if query != "" {
		s.Library.OpenFound(id, query, true)
		return
	}
	s.Library.OpenSheet(id, true)
}

// moveIntoPicker takes an open sheet's column from where it stands into a
// picker's place.
func (s *State) moveIntoPicker(picker int, id string) {
	cols := s.canvas.columns
	at := pickerIndex(cols, picker)
	if at < 0 {
		return
	}
	cols[at] = canvasColumn{sheet: id}
	for i, col := range cols {
		if i != at && col.sheet == id {
			cols = append(cols[:i], cols[i+1:]...)
			break
		}
	}
	s.canvas.columns = cols
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
	// The picker the canvas comes with is the sheet list's place: it comes
	// with the canvas, and holds the list once the page mode's layout has
	// let it go (held), as the list holds it back.
	first := p.id == s.canvas.listPicker
	col := s.column(c, fmt.Sprintf("picker-%d", p.id), w, origin, !(first && s.canvas.entered), animate).
		Role(ui.RoleGroup).Label("Sheet picker").Clip().Radius(10*k).
		Background(semantic.Surface.Panel).Border(1, semantic.Border.Variant)
	if first {
		s.morph.Mark(c, col, "list", "canvas", s.held == ModeCanvas, morphSettle, animate)
	}
	// A picker comes into view at the strip's start, the column after it
	// beside it; as the canvas comes, at once, and held there while the
	// panes close.
	if s.canvas.revealPicker == p.id {
		options := ui.ScrollIntoViewOptions{Inline: ui.ScrollStart}
		if animate && !s.canvas.entered {
			spring := components.Spring(components.MotionScene)
			options.Animation = &spring
		}
		col.ScrollIntoView(options)
		if !s.canvas.entered {
			s.canvas.revealPicker = 0
		}
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
