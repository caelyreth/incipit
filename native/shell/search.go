package shell

import (
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// finding is the sidebar's search as it is being written: its field, and
// the found sheet the keys are on.
type finding struct {
	input queryInput
	// row is the found sheet Up and Down are on, -1 for none; reveal asks
	// the list to bring it into view.
	row    int
	reveal bool
	// found is the search last shown, for the keys.
	found Found
}

// Search shows search in the sheet list, its field focused.
func (s *State) Search() {
	s.nav.searching = true
	s.nav.find.row = -1
	s.sheetOverscroll.Reset()
	s.showSheetList()
	s.SearchRequested = true
	s.disarm()
}

// stopSearching gives the sheet list back to the place.
func (s *State) stopSearching() {
	s.nav.searching = false
	s.sheetOverscroll.Reset()
	s.SearchQuery = ""
	s.nav.find = finding{row: -1}
}

// searchHeader is the search's line in the title bar: the query, set as
// the query language reads it, and what may be written at its caret. A
// query that comes to run in a sheet ($sheet) goes to the sheet's find.
func (s *State) searchHeader(c *ui.Context, semantic theme.Theme, animate bool) {
	field, input := queryField{
		Key: "page-search", Label: "Search sheets", Placeholder: "Search sheets, $ for keys",
		Value: &s.SearchQuery, In: &s.nav.find.input, Scope: query.InNotebook,
		Focus: s.SearchRequested, Animate: animate, Keys: s.searchKey,
	}.View(s, c, semantic)
	field.Grow(1)
	s.SearchRequested = false
	if input.Changed() {
		// A new query starts its keys from the top.
		s.nav.find.row = -1
		if query.Parse(s.SearchQuery).Scope == query.InSheet && s.ActivePage != "" {
			text := s.SearchQuery
			s.stopSearching()
			s.StartFind(text)
		}
	}
	components.Button{
		Key: "done-searching", Label: "Done",
		Style: components.SubtleButton, Size: components.CompactButton,
		Animate: animate, Action: s.stopSearching,
	}.View(c, semantic)
}

// searchKey is what the keys do in the query past its completion: Up and
// Down go through the sheets found, Enter opens the one the keys are on,
// or the best, and Escape ends the search.
func (s *State) searchKey(k ui.TextKey) bool {
	f := &s.nav.find
	switch k.Key {
	case ui.KeyDown, ui.KeyUp:
		if k.Mods != 0 {
			return false
		}
		step := 1
		if k.Key == ui.KeyUp {
			step = -1
		}
		if n := len(f.found.Sheets); n > 0 {
			f.row = min(max(f.row+step, 0), n-1)
			f.reveal = true
		}
		return true
	case ui.KeyEnter:
		if len(f.found.Sheets) == 0 {
			return false
		}
		row := max(f.row, 0)
		if row < len(f.found.Sheets) {
			s.Library.OpenFound(f.found.Sheets[row].ID, k.Text(), k.Mods&ui.Alt != 0)
		}
		return true
	case ui.KeyEscape:
		s.stopSearching()
		return true
	}
	return false
}

// searchList lists what the query finds, the best first, each with where
// it matches.
func (s *State) searchList(c *ui.Context, semantic theme.Theme, animate bool) {
	text := strings.TrimSpace(s.SearchQuery)
	if text == "" {
		s.nav.find.found = Found{}
		s.emptyList(c, semantic, components.IconSearch, "Search every sheet’s words, or type $ for keys", nil, nil, animate)
		return
	}
	found := s.Library.Search(s.SearchQuery)
	s.nav.find.found = found
	f := &s.nav.find
	if f.row >= len(found.Sheets) {
		f.row = len(found.Sheets) - 1
	}
	if len(found.Sheets) == 0 {
		s.emptyList(c, semantic, components.IconSearch, "Nothing found for “"+text+"”", nil, nil, animate)
		return
	}
	scale := s.Settings.UIFontSize
	now := c.Now()
	scroll := ui.Scroll(c).Key("found").Grow(1).MinHeight(0).TrackScroll(&s.sheetScroll).
		Transition(appearing(motionList, nil, &ui.Motion{Opacity: 0}))
	var cursor ui.Element
	shown := false
	scroll.Children(func() {
		list := ui.Column(c).Gap(theme.Space06.DIPs(scale)).
			PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale))
		list.Children(func() {
			components.Label(c, foundNote(found), theme.TextXSmall, semantic.Text.Muted).Key("found-note").
				PaddingX(theme.Space08.DIPs(scale)).SingleLine().FontFeatures("tnum")
			for i, sheet := range found.Sheets {
				row := s.foundRow(c, semantic, sheet, i == f.row, now, animate)
				if i == f.row {
					cursor, shown = row, true
				}
			}
		})
	})
	if f.reveal && shown {
		f.reveal = false
		revealRow(&s.sheetScroll, scroll.LayoutBounds(), cursor.LayoutBounds(), theme.Space08.DIPs(scale))
	}
}

// foundNote says how many sheets the query found, and that words a typo
// away were taken.
func foundNote(found Found) string {
	note := sheetCount(found.Total)
	if found.Total > len(found.Sheets) {
		note = "The best " + thousands(len(found.Sheets)) + " of " + note
	}
	if found.Close {
		note += " · with close matches"
	}
	return note
}

// revealRow scrolls the list so row is in view, with room around it.
func revealRow(scroll *ui.ScrollState, view, row ui.Rect, room float32) {
	switch {
	case row.Y < view.Y+room:
		scroll.ScrollTo(scroll.X, scroll.Y-(view.Y+room-row.Y), ui.Animation{Duration: motionList})
	case row.Y+row.H > view.Y+view.H-room:
		scroll.ScrollTo(scroll.X, scroll.Y+(row.Y+row.H-(view.Y+view.H-room)), ui.Animation{Duration: motionList})
	}
}

// foundRow is a sheet the query found: its title and the line of its text
// it matches, the matches marked, as the sheet's row shows it otherwise.
func (s *State) foundRow(c *ui.Context, semantic theme.Theme, sheet FoundSheet, cursor bool, now time.Time, animate bool) ui.Element {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	open := sheet.ID == s.ActivePage
	row := ui.Column(c).Key("found-"+sheet.ID).Role(ui.RoleButton).Label(sheet.Title).Selected(cursor).
		WidthPercent(100).Shrink(0).
		PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space08.DIPs(scale)).
		Gap(theme.Space02.DIPs(scale)).Radius(metrics.Radius).Border(1, ui.Transparent).
		Transition(s.listRowTransition(animate))
	s.enter(c, row)
	hover := components.HoverAmount(row, !cursor && row.Hovered(), animate)
	fill := components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)
	if cursor {
		fill = components.NavigationSelection(c, semantic).Fill
	}
	row.Background(fill)
	row.Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).Children(func() {
			title := semantic.Text.Default
			if open || cursor {
				title = semantic.Text.Heading
			}
			s.marked(c, semantic, sheet.Title, sheet.TitleMatches, theme.TextSmall, title).
				Grow(1).MinWidth(0).SingleLine()
			components.Label(c, when(sheet.Updated, now), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().Shrink(0).FontFeatures("tnum")
		})
		switch {
		case sheet.Line != "":
			s.marked(c, semantic, sheet.Line, sheet.LineMatches, theme.TextXSmall, semantic.Text.Muted).
				WidthPercent(100).MaxLines(2)
		case sheet.Excerpt != "":
			components.Label(c, sheet.Excerpt, theme.TextXSmall, semantic.Text.Muted).
				WidthPercent(100).MaxLines(2)
		}
		s.sheetTags(c, semantic, sheet.Tags)
	})
	if row.FocusVisible() {
		row.Border(1, semantic.Border.Focused)
	}
	row.Drag(sheetDrag{id: sheet.ID})
	row.ContextMenu(s.sheetMenu(sheet.SheetItem, false))
	if row.Clicked() {
		s.Library.OpenFound(sheet.ID, s.SearchQuery, row.ClickModifiers()&ui.Alt != 0)
	}
	return row
}

// marked is text with its matches marked, as a highlighter marks them.
func (s *State) marked(c *ui.Context, semantic theme.Theme, text string, matches []query.Span, size theme.TextSize, color ui.Color) ui.Element {
	scale := s.Settings.UIFontSize
	plain := func(t string) ui.Span {
		return ui.Span{Text: t, Font: theme.UIFontFamily, Size: size.Size(scale), Color: color}
	}
	var spans []ui.Span
	at := 0
	for _, m := range matches {
		if m.From < at || m.To > len(text) {
			continue
		}
		if m.From > at {
			spans = append(spans, plain(text[at:m.From]))
		}
		hit := plain(text[m.From:m.To])
		hit.Color = semantic.Text.Heading
		// A highlighter's yellow, apart from the accent a chosen row
		// wears.
		hit.Background = semantic.Families[theme.Regolith].Muted
		hit.Radius = 3 * scale / theme.DefaultUIFontSize
		spans = append(spans, hit)
		at = m.To
	}
	if at < len(text) || len(spans) == 0 {
		spans = append(spans, plain(text[at:]))
	}
	return ui.RichText(c, spans...).FixedLineHeight(size.LineHeight(scale))
}
