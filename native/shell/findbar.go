package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// FindCount is what a sheet's find found: how many matches, and the one
// the selection is on, from 1, or 0 for none.
type FindCount struct{ Current, Total int }

// sheetKey is what a sheet's find starts with: the search's own language,
// run in the sheet.
const sheetKey = "$sheet "

// findBar is a sheet's find as the writing dock shows it in its place.
type findBar struct {
	open  bool
	input queryInput
}

// FindOpen reports whether a sheet's find is open.
func (s *State) FindOpen() bool { return s != nil && s.finder.open }

// StartFind opens a sheet's find with text, which runs in the sheet: a
// query that holds $sheet, or words $sheet is put before. What follows
// the key is selected, for typing over.
func (s *State) StartFind(text string) {
	if query.Parse(text).Scope != query.InSheet {
		text = sheetKey + text
	}
	s.FindQuery = text
	s.finder.open = true
	s.FindRequested = true
	from := 0
	if strings.HasPrefix(text, sheetKey) {
		from = utf8.RuneCountInString(sheetKey)
	}
	s.finder.input.selectNext(from, utf8.RuneCountInString(text))
	if s.Find != nil {
		s.Find(text)
	}
}

// endFind closes the find: the page's matches go, and the selection stays
// on the match it was on.
func (s *State) endFind() {
	s.finder = findBar{}
	s.FindQuery = ""
	s.FindRequested = false
	s.FindCount = FindCount{}
	if s.EndFind != nil {
		s.EndFind()
	}
}

// stepFind goes to the next match, or the one before.
func (s *State) stepFind(forward bool) {
	if s.FindStep != nil {
		s.FindStep(forward)
	}
}

// findBarView is the find in the writing dock's place, on its material:
// the query, set as the language reads it, what it found, and the steps
// through it. A query that no longer runs in the sheet, once it reads
// whole, goes to the sidebar's search.
func (s *State) findBarView(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	bar := ui.Row(c).Key("find-dock").Role(ui.RoleToolbar).Label("Find in sheet").
		Width(440*scale/theme.DefaultUIFontSize).MaxWidthPercent(100).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Padding(theme.Space04.DIPs(scale)).
		Radius(metrics.DockRadius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Panel).Transition(popTransition(animate))
	var input ui.Element
	bar.Children(func() {
		var field ui.Element
		field, input = queryField{
			Key: "sheet-find", Label: "Find in sheet", Placeholder: "Find in this sheet, $ for keys",
			Value: &s.FindQuery, In: &s.finder.input, Scope: query.InSheet,
			Focus: s.FindRequested, Animate: animate, Keys: s.findKey,
		}.View(s, c, semantic)
		field.Grow(1)
		if note := findNote(s.FindQuery, s.FindCount); note != "" {
			components.Label(c, note, theme.TextXSmall, semantic.Text.Muted).Key("find-count").
				SingleLine().Shrink(0).FontFeatures("tnum")
		}
		components.VerticalDivider(c, semantic)
		step := func(key, label, binding string, icon components.IconName, forward bool) {
			components.IconButton{
				Key: key, Label: label, Tooltip: components.CommandTooltip(label, binding), Icon: icon.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				Disabled: s.FindCount.Total == 0, Animate: animate,
				IconColor: &semantic.Icon.Muted, Action: func() { s.stepFind(forward) },
			}.View(c, semantic)
		}
		step("find-previous", "Previous match", "⇧↩", components.IconChevronUp, false)
		step("find-next", "Next match", "↩", components.IconChevronDown, true)
		components.IconButton{
			Key: "find-close", Label: "Close find", Tooltip: components.CommandTooltip("Close find", "esc"),
			Icon: components.IconClose.SVG(), Style: components.SubtleButton, Size: components.MediumButton,
			Animate: animate, IconColor: &semantic.Icon.Muted, Action: s.endFind,
		}.View(c, semantic)
	})
	s.FindRequested = false
	if input.Changed() {
		text := s.FindQuery
		if q := query.Parse(text); q.Scope != query.InSheet && settled(q) {
			// The query left the sheet: it is the notebook's search now.
			text = strings.TrimLeft(text, " ")
			s.endFind()
			s.Search()
			s.SearchQuery = text
			n := utf8.RuneCountInString(text)
			s.nav.find.input.selectNext(n, n)
		} else if s.Find != nil {
			s.Find(text)
		}
	}
}

// settled reports a query with no key being written or wrong in it: one
// that says where it runs, and may go there.
func settled(q query.Query) bool {
	for _, is := range q.Issues {
		if is.Kind == query.Unknown || is.Kind == query.Pending {
			return false
		}
	}
	return true
}

// findNote says what the find found: the match the selection is on and
// how many, or that there is none. A query of no words says nothing.
func findNote(text string, count FindCount) string {
	q := query.Parse(text)
	if query.NewMatcher(q, false).Empty() {
		return ""
	}
	switch {
	case count.Total == 0:
		return "No matches"
	case count.Current > 0:
		return fmt.Sprintf("%s of %s", thousands(count.Current), thousands(count.Total))
	case count.Total == 1:
		return "1 match"
	}
	return thousands(count.Total) + " matches"
}

// findKey is what the keys do in a sheet's find past its completion:
// Enter and Down go to the next match, Shift+Enter and Up to the one
// before, Cmd+G and Cmd+Shift+G as on the page; Escape ends the find.
func (s *State) findKey(k ui.TextKey) bool {
	switch {
	case k.Key == ui.KeyEnter && k.Mods == 0, k.Key == ui.KeyDown && k.Mods == 0,
		k.Key == ui.KeyG && k.Mods == ui.Cmd:
		s.stepFind(true)
		return true
	case k.Key == ui.KeyEnter && k.Mods == ui.Shift, k.Key == ui.KeyUp && k.Mods == 0,
		k.Key == ui.KeyG && k.Mods == ui.Cmd|ui.Shift:
		s.stepFind(false)
		return true
	case k.Key == ui.KeyEscape:
		s.endFind()
		return true
	}
	return false
}
