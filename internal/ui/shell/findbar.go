package shell

import (
	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// FindCount is what a sheet's find found: how many matches, and the one
// the selection is on, from 1, or 0 for none.
type FindCount struct{ Current, Total int }

// sheetKey is the key a sheet's find stands under: the search's own
// language, run in the sheet. The find bar holds it fixed, as the bar
// itself says where the query runs, and FindQuery is what is written
// after it.
const sheetKey = "$sheet"

// findBar is a sheet's find as the writing dock shows it in its place.
type findBar struct {
	open  bool
	input queryInput
}

// FindOpen reports whether a sheet's find is open.
func (s *State) FindOpen() bool { return s != nil && s.finder.open }

// StartFind opens a sheet's find with text, which runs in the sheet: its
// words are written after the find's fixed $sheet, the key taken out of
// them where they hold it, and selected, for typing over.
func (s *State) StartFind(text string) {
	text = withoutSheet(text)
	s.FindQuery = text
	s.finder.open = true
	s.FindRequested = true
	s.finder.input.focus.SelectAll()
	if s.Find != nil {
		s.Find(s.FindText())
	}
}

// FindText is the query a sheet's find runs: its fixed key, then what is
// written.
func (s *State) FindText() string { return sheetKey + " " + s.FindQuery }

// withoutSheet is text without its $sheet keys.
func withoutSheet(text string) string {
	var cuts []query.Span
	query.Walk(query.Parse(text).Root, func(n query.Node) {
		if f, ok := n.(*query.Filter); ok && f.Key != nil && f.Key.Name == "sheet" {
			cuts = append(cuts, f.Span)
		}
	})
	for i := len(cuts) - 1; i >= 0; i-- {
		text = query.Cut(text, cuts[i])
	}
	return text
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
// the query under its fixed $sheet, set as the language reads it, what it
// found, and the steps through it.
func (s *State) findBarView(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	bar := ui.Row(c).Key("find-dock").Role(ui.RoleToolbar).Label(locales.FindInSheet()).
		Width(480*scale/theme.DefaultUIFontSize).MaxWidthPercent(100).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Padding(theme.Space04.DIPs(scale)).
		Radius(metrics.DockRadius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Panel).Transition(popTransition(animate))
	var input ui.Element
	bar.Children(func() {
		var field ui.Element
		field, input = queryField{
			Key: "sheet-find", Label: locales.FindInSheet(), Placeholder: locales.FindPlaceholder(),
			Value: &s.FindQuery, In: &s.finder.input, Scope: query.InSheet,
			Fixed: sheetKey, FixedLabel: locales.FindsInThisSheet(),
			Focus: s.FindRequested, Animate: animate, Keys: s.findKey,
		}.View(s, c, semantic)
		field.Grow(1)
		// What the find found is said as it changes, to the eye and to
		// assistive technology; finding nothing is marked.
		note := findNote(s.FindText(), s.FindCount)
		color := semantic.Text.Muted
		if note != "" && s.FindCount.Total == 0 {
			color = semantic.Status.Danger.Foreground
		}
		components.CenteredLabel(c, note, theme.RowMeta, color).FontFeatures("tnum").Key("find-count").Role(ui.RoleStatus).
			Shrink(0).FontFeatures("tnum")
		components.VerticalDivider(c, semantic)
		step := func(key, label, binding string, icon components.IconName, forward bool) {
			components.IconButton{
				Key: key, Label: label, Binding: binding, Icon: icon.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				Disabled: s.FindCount.Total == 0, Animate: animate,
				IconColor: semantic.Icon.Muted, Action: func() { s.stepFind(forward) },
			}.View(c, semantic)
		}
		step("find-previous", locales.PreviousMatch(), "⇧↩", components.IconChevronUp, false)
		step("find-next", locales.NextMatch(), "↩", components.IconChevronDown, true)
		components.IconButton{
			Key: "find-close", Label: locales.CloseFind(), Binding: "esc",
			Icon: components.IconClose.SVG(), Style: components.SubtleButton, Size: components.MediumButton,
			Animate: animate, IconColor: semantic.Icon.Muted, Action: s.endFind,
		}.View(c, semantic)
	})
	s.FindRequested = false
	if input.Changed() && s.Find != nil {
		s.Find(s.FindText())
	}
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
		return locales.NoMatches()
	case count.Current > 0:
		return locales.MatchOf(thousands(count.Current), thousands(count.Total))
	}
	return locales.Matches(count.Total, thousands(count.Total))
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
