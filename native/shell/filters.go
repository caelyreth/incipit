package shell

import (
	"strconv"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The filter builder shows the search's query as its conditions
// (query.Conditions), a row each: the key, whether it holds or not, and
// the value. It keeps no state of its own: the rows are read from the
// query every frame and edit it in its text, so the query and the rows
// never disagree, and what the rows do not show stays as written.

// builderKeys are the keys a condition can be made of: those of the
// notebook's search. $sheet is not one: it takes the query to the sheet.
func builderKeys() []*query.Key {
	var keys []*query.Key
	for i := range query.Keys {
		if k := &query.Keys[i]; k.Scope&query.InNotebook != 0 && k.Name != "sheet" {
			keys = append(keys, k)
		}
	}
	return keys
}

// setSearch puts text in the search's field, as typing it there would.
func (s *State) setSearch(text string) {
	s.SearchQuery = text
	s.nav.find.row = -1
}

// filters is the builder, above what the search found.
func (s *State) filters(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	text := s.SearchQuery
	conditions := query.Conditions(query.Parse(text))
	focus := s.nav.find.focusFilter
	s.nav.find.focusFilter = -1
	panel := ui.Column(c).Key("filters").WidthPercent(100).Shrink(0).
		Gap(theme.Space04.DIPs(scale)).
		PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale)).
		Transition(appearing(motionList, nil, &ui.Motion{Opacity: 0}))
	panel.Children(func() {
		for i, cond := range conditions {
			s.condition(c, semantic, text, cond, i, i == focus, animate)
		}
		ui.Row(c).Key("filters-add").AlignItems(ui.Center).Children(func() {
			components.Button{
				Key: "add-filter", Label: "Add Filter", Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
			}.View(c, semantic).Menu(func(m *ui.Menu) {
				s.keyItems(m, nil, func(k *query.Key) {
					s.setSearch(query.And(text, query.Condition{Key: k}.String()))
					s.nav.find.focusFilter = len(conditions)
				})
			})
		})
		components.HorizontalDivider(c, semantic, components.VariantDivider)
	})
}

// condition is one condition's row. Its value is written in a field, and,
// where the key knows its values, chosen from them, each with how many
// sheets it would find in place of what is chosen now.
func (s *State) condition(c *ui.Context, semantic theme.Theme, text string, cond query.Condition, index int, focus, animate bool) {
	scale := s.Settings.UIFontSize
	// The key and whether it holds take columns of their own, so the
	// values line up down the rows.
	column := func(w float32) float32 { return w * scale / theme.DefaultUIFontSize }
	button := components.CompactButton.Height(scale)
	set := func(next query.Condition) { s.setSearch(query.Replace(text, cond.Span, next.String())) }
	ui.Row(c).Key("filter-" + strconv.Itoa(index)).WidthPercent(100).AlignItems(ui.Center).
		Gap(theme.Space04.DIPs(scale)).Children(func() {
		components.Button{
			Key: "key", Label: "$" + cond.Key.Name, Tooltip: cond.Key.Help,
			Style: components.SubtleButton, Size: components.CompactButton, Animate: animate,
		}.View(c, semantic).Width(column(76)).Shrink(0).Menu(func(m *ui.Menu) {
			s.keyItems(m, cond.Key, func(k *query.Key) {
				set(query.Condition{Key: k, Not: cond.Not && !k.Setting})
			})
		})
		if cond.Key.Setting {
			ui.Box(c).Width(column(60)).Shrink(0)
		} else {
			label := "is"
			if cond.Not {
				label = "is not"
			}
			components.Button{
				Key: "not", Label: label, Style: components.SubtleButton,
				Size: components.CompactButton, Animate: animate,
			}.View(c, semantic).Width(column(60)).Shrink(0).Menu(func(m *ui.Menu) {
				for _, not := range []bool{false, true} {
					label := "is"
					if not {
						label = "is not"
					}
					if m.Item(label).Checked(cond.Not == not).Chosen() {
						next := cond
						next.Not = not
						set(next)
					}
				}
			})
		}
		if cond.Key.Takes != query.TakesNothing {
			value := cond.Value
			field := components.TextField{
				Key: "value", Label: "$" + cond.Key.Name + " value", Placeholder: hintOf(cond.Key),
				Value: &value, FocusRequested: focus, Animate: animate,
				Changed: func() {
					next := cond
					next.Value = value
					set(next)
				},
			}.View(c, semantic)
			field.Grow(1).MinWidth(0)
			if choices := s.choices(cond.Key); len(choices) == 0 {
				ui.Box(c).Width(button).Shrink(0)
			} else {
				components.IconButton{
					Key: "values", Label: "Choose a value", Icon: components.IconChevronDown.SVG(),
					Style: components.SubtleButton, Size: components.CompactButton,
					IconColor: &semantic.Icon.Muted, Animate: animate,
				}.View(c, semantic).Menu(func(m *ui.Menu) {
					counts := s.Library.SearchCounts(query.Cut(text, cond.Span), cond.Key.Name, choices)
					for _, v := range choices {
						label := v
						if n, ok := counts[v]; ok {
							label += " · " + thousands(n)
						}
						if m.Item(label).Checked(query.Fold(v) == query.Fold(cond.Value)).Chosen() {
							next := cond
							next.Value = v
							set(next)
						}
					}
				})
			}
		} else {
			ui.Box(c).Grow(1)
		}
		components.IconButton{
			Key: "remove", Label: "Remove filter", Icon: components.IconClose.SVG(),
			Style: components.SubtleButton, Size: components.CompactButton,
			IconColor: &semantic.Icon.Muted, Animate: animate,
			Action: func() { s.setSearch(query.Cut(text, cond.Span)) },
		}.View(c, semantic)
	})
}

// keyItems lists the keys a condition can be made of, the one it is of
// checked, each with what it finds.
func (s *State) keyItems(m *ui.Menu, of *query.Key, choose func(*query.Key)) {
	for _, k := range builderKeys() {
		if m.Item("$" + k.Name).Tooltip(k.Help).Checked(k == of).Chosen() {
			choose(k)
		}
	}
}

// choices are the values a key knows: its own, the dates named, or the
// names the notebook holds for it.
func (s *State) choices(k *query.Key) []string {
	switch k.Takes {
	case query.TakesChoice:
		return k.Choices
	case query.TakesDate:
		return query.DatePresets
	case query.TakesName:
		names := s.Library.SearchNames(k.Name)
		if k.Name == "tag" && len(names) > 0 {
			names = append(names[:len(names):len(names)], "none")
		}
		return names
	}
	return nil
}

// hintOf is what a key's value field says while it is empty: how dates
// and counts are written, or what the key takes.
func hintOf(k *query.Key) string {
	switch k.Takes {
	case query.TakesDate:
		return "7d, 2026-11, >2026-01-01"
	case query.TakesNumber:
		return "500, <100, 100..500"
	}
	return k.Hint()
}
