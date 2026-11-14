package shell

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/neverland/app/search"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// finding is the sidebar's search as it is being written: the found sheet
// the keys are on, the completion offered at the caret, and the
// suggestion the keys are on in it.
type finding struct {
	// row is the found sheet Up and Down are on, -1 for none; reveal asks
	// the list to bring it into view.
	row    int
	reveal bool
	// found is the search last shown, for the keys.
	found Found
	// completion is what may stand at the caret in query at caret, a
	// byte offset; suggestion is the item the keys are on.
	completion search.Completion
	query      string
	caret      int
	suggestion int
	// first is the first suggestion the list shows.
	first int
	// pointer is where the pointer was over the suggestions, pointed
	// whether it was there.
	pointer ui.Point
	pointed bool
	// dismissed is the query the completion was put away at, with Escape
	// or a press elsewhere: it comes back once the query changes.
	dismissed string
	// placeCaret, when not -1, is where the caret goes in the next frame,
	// in runes, as after a suggestion is taken.
	placeCaret int
}

// Search shows search in the sheet list, its field focused.
func (s *State) Search() {
	s.nav.searching = true
	s.nav.find.row, s.nav.find.placeCaret = -1, -1
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
	s.nav.find = finding{row: -1, placeCaret: -1}
}

// searchHeader is the search's line in the title bar: the query, set as
// the query language reads it, and what may be written at its caret.
func (s *State) searchHeader(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	text := theme.TextSmall
	var input ui.Element
	field := ui.Row(c).Key("search-field").Height(metrics.Height).Grow(1).MinWidth(0).
		PaddingStart(theme.Space08.DIPs(scale)).PaddingEnd(theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Inset).Cursor(ui.CursorText)
	field.Children(func() {
		ui.Icon(c, components.IconSearch.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).
			TextColor(semantic.Icon.Muted)
		input = ui.TextInputBase(c.Key("page-search"), &s.SearchQuery).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(text.Size(scale)).FixedLineHeight(text.LineHeight(scale)).
			TextColor(semantic.Text.Default).
			Placeholder("Search sheets, $ for keys").Label("Search sheets")
		input.TextRanges(queryRanges(s.SearchQuery, semantic)...)
		input.OnTextKey(s.searchKey)
		if s.SearchRequested {
			input.Focus()
		}
		if at := s.nav.find.placeCaret; at >= 0 {
			input.SetTextSelection(at, at)
			s.nav.find.placeCaret = -1
		}
		if s.SearchQuery != "" {
			components.IconButton{
				Key: "clear", Label: "Clear", Icon: components.IconClose.SVG(),
				Style: components.SubtleButton, Size: components.CompactButton,
				IconColor: &semantic.Icon.Muted, Animate: animate,
				Action: func() {
					s.SearchQuery = ""
					input.SetChanged()
					input.Focus()
				},
			}.View(c, semantic)
		}
	})
	s.SearchRequested = false
	if input.Changed() {
		// A new query starts its keys from the top.
		s.nav.find.row = -1
	}
	if input.Focused() {
		field.Border(1, semantic.Border.Focused)
	}
	_, caret := input.TextSelection()
	s.complete(byteOffset(s.SearchQuery, caret))
	open := input.Focused() && len(s.nav.find.completion.Items) > 0 && s.nav.find.dismissed != s.SearchQuery
	if open {
		s.suggestions(c, semantic, field, input, &open, animate)
		if !open {
			s.nav.find.dismissed = s.SearchQuery
		}
	}
	components.Button{
		Key: "done-searching", Label: "Done",
		Style: components.SubtleButton, Size: components.CompactButton,
		Animate: animate, Action: s.stopSearching,
	}.View(c, semantic)
}

// complete offers what may stand at caret in the query, as it changes.
func (s *State) complete(caret int) {
	f := &s.nav.find
	if f.query == s.SearchQuery && f.caret == caret {
		return
	}
	f.query, f.caret, f.suggestion, f.first = s.SearchQuery, caret, 0, 0
	f.completion = search.Complete(s.SearchQuery, caret, search.InNotebook, s.Library.SearchNames)
}

// completing reports whether a completion is offered for text, the query
// as a key sees it.
func (s *State) completing(text string) bool {
	f := &s.nav.find
	return f.query == text && len(f.completion.Items) > 0 && f.dismissed != text
}

// suggestionsShown is how many suggestions the completion lists.
const suggestionsShown = 8

// suggestions lists the completion under the field, on the outlined
// surface the page's tag picker has, as a part of the field rather than a
// layer over the window. The chosen row's fill slides as the keys or the
// pointer move it, and the chosen key's help is said whole at the foot.
func (s *State) suggestions(c *ui.Context, semantic theme.Theme, field, input ui.Element, open *bool, animate bool) {
	f := &s.nav.find
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	pad := theme.Space04.DIPs(scale)
	row := metrics.Height
	all := f.completion.Items
	f.suggestion = min(f.suggestion, len(all)-1)
	chosen := all[f.suggestion]
	// The rows shown are a window on the suggestions that keeps the chosen
	// one in it, as the keys move it past either end.
	f.first = min(max(f.first, f.suggestion-suggestionsShown+1), f.suggestion, max(len(all)-suggestionsShown, 0))
	items := all[f.first:min(f.first+suggestionsShown, len(all))]
	halo := semantic.Families[theme.Halo]
	width := field.LayoutBounds().W
	ui.PopoverBase(c.Key("suggestions"), field, open, func(panel ui.Element) {
		components.OutlinedSurface(panel, semantic, scale)
		panel.Width(width).Padding(pad).Margin(pad, 0, 0, 0).
			Role(ui.RoleList).Label("Suggestions").Transition(popTransition(animate))
		// Only the pointer's moves choose the row under it, so a list the
		// keys move under a resting pointer keeps their choice.
		px, py, over := panel.PointerPosition()
		if p := (ui.Point{X: px, Y: py}); over {
			if f.pointed && p != f.pointer {
				if i := int((py - 1 - pad) / row); i >= 0 && i < len(items) {
					f.suggestion = f.first + i
				}
			}
			f.pointer, f.pointed = p, true
		} else {
			f.pointed = false
		}
		var spring *ui.Animation
		if animate {
			sp := components.Spring(components.MotionNormal)
			spring = &sp
		}
		panel.Children(func() {
			ui.Box(c).Key("suggestion-chosen").Absolute().Left(pad).Right(pad).
				Top(pad + float32(f.suggestion-f.first)*row).Height(row).
				Radius(metrics.PanelRadius - pad).Background(semantic.GhostElement.Hover).ZIndex(-1).
				Transition(ui.ElementTransition{Animation: spring, Position: true})
			for i, item := range items {
				button := ui.ButtonBase(c.Key("suggestion-" + item.Label)).Height(row).WidthPercent(100).Shrink(0).
					PaddingX(theme.Space06.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
					FocusRing(false).Selected(i == f.suggestion).Background(ui.Transparent).Label(item.Label)
				button.Children(func() {
					switch {
					case item.Of == "tag":
						components.TagChip(c, "#"+strings.TrimPrefix(item.Label, "#"), theme.TextSmall,
							theme.Space04.DIPs(scale), width, halo.Surface, halo.Foreground)
					case item.Of == "":
						components.Label(c, item.Label, theme.TextSmall, semantic.Accent.Foreground).
							SingleLine().Shrink(0)
					default:
						components.Label(c, item.Label, theme.TextSmall, semantic.Text.Default).
							SingleLine().MinWidth(0)
					}
					ui.Box(c).Grow(1)
					if item.Hint != "" {
						components.Label(c, item.Hint, theme.TextXSmall, semantic.Text.Muted).
							SingleLine().Shrink(0)
					}
				})
				if button.Clicked() {
					text, caret := s.take(s.SearchQuery, item)
					s.SearchQuery = text
					f.placeCaret = caret
					input.SetChanged()
					input.Focus()
				}
			}
			if chosen.Help != "" {
				ui.Box(c).Key("suggestion-rule").WidthPercent(100).Height(1).
					Margin(pad, 0, pad, 0).Background(semantic.Border.Variant)
				components.Label(c, chosen.Help, theme.TextXSmall, semantic.Text.Muted).Key("suggestion-help").
					WidthPercent(100).PaddingX(theme.Space06.DIPs(scale)).PaddingY(theme.Space02.DIPs(scale)).
					MaxLines(3)
			}
		})
	})
}

// take writes a suggestion into text in place of the completion's span:
// the text it makes, and the caret after it, in runes. One that finishes
// its part has a space after it, for the next.
func (s *State) take(text string, item search.Suggestion) (string, int) {
	span := s.nav.find.completion.Span
	before, after := text[:span.From]+item.Text, text[span.To:]
	if item.Done {
		// Past the space after it, written now or there already.
		after = " " + strings.TrimPrefix(after, " ")
		return before + after, utf8.RuneCountInString(before) + 1
	}
	return before + after, utf8.RuneCountInString(before)
}

// searchKey is what the keys do in the query: Up and Down go through the
// suggestions offered, or else the sheets found; Tab and Enter take a
// suggestion; Enter opens the sheet the keys are on, or the best;
// Escape puts the suggestions away, then ends the search.
func (s *State) searchKey(k ui.TextKey) bool {
	f := &s.nav.find
	text := k.Text()
	completing := s.completing(text)
	items := len(f.completion.Items)
	switch k.Key {
	case ui.KeyDown, ui.KeyUp:
		if k.Mods != 0 {
			return false
		}
		step := 1
		if k.Key == ui.KeyUp {
			step = -1
		}
		if completing {
			f.suggestion = (f.suggestion + step + items) % items
			return true
		}
		if n := len(f.found.Sheets); n > 0 {
			f.row = min(max(f.row+step, 0), n-1)
			f.reveal = true
		}
		return true
	case ui.KeyTab, ui.KeyEnter:
		if completing && k.Mods == 0 {
			out, caret := s.take(text, f.completion.Items[f.suggestion])
			k.SetText(out)
			f.placeCaret = caret
			return true
		}
		if k.Key == ui.KeyTab || len(f.found.Sheets) == 0 {
			return false
		}
		row := max(f.row, 0)
		if row < len(f.found.Sheets) {
			s.Library.OpenFound(f.found.Sheets[row].ID, text, k.Mods&ui.Alt != 0)
		}
		return true
	case ui.KeyEscape:
		if completing {
			f.dismissed = text
			return true
		}
		s.stopSearching()
		return true
	}
	return false
}

// searchList lists what the query finds, the best first, each with where
// it matches.
func (s *State) searchList(c *ui.Context, semantic theme.Theme, animate bool) {
	query := strings.TrimSpace(s.SearchQuery)
	if query == "" {
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
		s.emptyList(c, semantic, components.IconSearch, "Nothing found for “"+query+"”", nil, nil, animate)
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
func (s *State) marked(c *ui.Context, semantic theme.Theme, text string, matches []search.Span, size theme.TextSize, color ui.Color) ui.Element {
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

// queryRanges sets a query as the language reads it: keys and tags in the
// accent, their values in the heading's color, a - quieter, what is wrong
// marked and what means nothing here muted.
func queryRanges(text string, semantic theme.Theme) []ui.TextRange {
	q := search.Parse(text)
	var out []ui.TextRange
	add := func(sp search.Span, color ui.Color) {
		if sp.To > sp.From {
			out = append(out, ui.TextRange{
				Start: utf8.RuneCountInString(text[:sp.From]), End: utf8.RuneCountInString(text[:sp.To]), Color: color,
			})
		}
	}
	search.Walk(q.Root, func(n search.Node) {
		switch n := n.(type) {
		case *search.Filter:
			name := search.Span{From: n.From, To: n.To}
			if n.HasValue && !n.Hash {
				name.To = n.ValueSpan.From
				add(n.ValueSpan, semantic.Text.Heading)
			}
			add(name, semantic.Accent.Foreground)
		case *search.Not:
			add(search.Span{From: n.From, To: n.From + 1}, semantic.Text.Muted)
		}
	})
	// What is still being written keeps its colors; what is wrong is
	// marked, and what means nothing here is quiet.
	for _, issue := range q.Issues {
		switch issue.Kind {
		case search.Unknown, search.Invalid:
			add(issue.Span, semantic.Status.Danger.Foreground)
		case search.Inert:
			add(issue.Span, semantic.Text.Muted)
		}
	}
	return out
}

// byteOffset is the byte offset of the rune at runes in text.
func byteOffset(text string, runes int) int {
	for i := range text {
		if runes == 0 {
			return i
		}
		runes--
	}
	return len(text)
}
