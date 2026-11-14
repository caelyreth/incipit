package shell

import (
	"strconv"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The search's filters are the query's conditions (query.Conditions),
// each a chip in words above what the search found: "Tag is work",
// "Updated in the last 7 days". A chip opens an editor beside it: the key,
// whether the sheets it holds of are included or excluded, and the value,
// written or chosen from the values the key knows, each with how many
// sheets it would find. The filters keep no state of their own: they are
// read from the query every frame and edit it in its text, so the field
// and the chips never disagree, and what the chips do not show stays as
// written.

// filterKeys are the keys a filter can be made of: those of the
// notebook's search, each once. $sheet is not one: it takes the query to
// the sheet.
func filterKeys() []*query.Key {
	var keys []*query.Key
	for i := range query.Keys {
		if k := &query.Keys[i]; k.Scope&query.InNotebook != 0 && k.Name != "sheet" && k.Alias == "" {
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

// filterEditor is the filter whose editor is open.
type filterEditor struct {
	// index is the condition edited, -1 for none; open holds its editor
	// open; fresh marks one just added, which goes again if it is left
	// without a value; focus gives its value field the focus.
	index              int
	open, fresh, focus bool
	// adding holds the list of keys open, for a filter more.
	adding bool
}

// closedFilter is no filter's editor open.
var closedFilter = filterEditor{index: -1}

// filterBar is the query's filters, a chip each, and the chip that adds
// one, wrapping as the list's width asks.
func (s *State) filterBar(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	text := s.SearchQuery
	q := query.Parse(text)
	conditions := query.Conditions(q)
	ed := &s.nav.find.filter
	if ed.index >= len(conditions) {
		*ed = closedFilter
	}
	ui.Row(c).Key("filters").Role(ui.RoleGroup).Label("Filters").WidthPercent(100).Shrink(0).Wrap().
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		PaddingX(theme.Space12.DIPs(scale)).PaddingY(theme.Space06.DIPs(scale)).Children(func() {
		for i, cond := range conditions {
			wrong := false
			for _, is := range q.Issues {
				if is.Kind != query.Pending && is.From >= cond.Span.From && is.To <= cond.Span.To {
					wrong = true
				}
			}
			s.filterChip(c, semantic, text, cond, i, wrong, animate)
		}
		s.addFilterChip(c, semantic, text, len(conditions), animate)
	})
}

// chipShape is a filter chip's outline, lit by the pointer where it does
// something, and set in as a field is while its panel is open.
func (s *State) chipShape(chip ui.Element, semantic theme.Theme, open, acts, animate bool) ui.Element {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	hover := components.HoverAmount(chip, acts && chip.Hovered() && !open, animate)
	fill := components.Blend(ui.Transparent, semantic.GhostElement.Hover, hover)
	border := semantic.Border.Variant
	if open {
		fill, border = semantic.Surface.Inset, semantic.Border.Focused
	}
	return chip.Height(metrics.Height).MinWidth(0).MaxWidthPercent(100).AlignItems(ui.Center).
		Radius(metrics.Radius).Border(1, border).Background(fill)
}

// filterChip is one filter: its words, which open its editor, and the
// button that takes it away. A filter the language does not read as
// written says so, in the danger color.
func (s *State) filterChip(c *ui.Context, semantic theme.Theme, text string, cond query.Condition, index int, wrong, animate bool) {
	scale := s.Settings.UIFontSize
	ed := &s.nav.find.filter
	sentence := cond.Sentence()
	chip := ui.Row(c).Key("filter-" + strconv.Itoa(index))
	s.chipShape(chip, semantic, ed.index == index && ed.open, true, animate)
	chip.Children(func() {
		edit := ui.ButtonBase(c.Key("edit")).Label("Filter: " + sentence).Tooltip("Edit filter").
			HeightPercent(100).MinWidth(0).PaddingStart(theme.Space08.DIPs(scale)).
			AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Background(ui.Transparent).FocusRing(false)
		if wrong {
			edit.Description("Not understood as written")
		}
		edit.Children(func() { s.chipWords(c, semantic, cond, wrong) })
		if edit.FocusVisible() {
			chip.Border(1, semantic.Border.Focused)
		}
		if edit.Clicked() {
			*ed = filterEditor{index: index, open: true, focus: cond.Key.Takes != query.TakesNothing}
		}
		components.IconButton{
			Key: "remove", Label: "Remove filter: " + sentence, Tooltip: "Remove filter",
			Icon: components.IconClose.SVG(), Style: components.SubtleButton, Size: components.CompactButton,
			IconSize: 14 * scale / theme.DefaultUIFontSize, IconColor: &semantic.Icon.Muted, Animate: animate,
			Action: func() {
				*ed = closedFilter
				s.setSearch(query.Cut(text, cond.Span))
			},
		}.View(c, semantic)
	})
	if ed.index == index {
		s.editFilter(c, semantic, chip, text, cond, animate)
	}
}

// queryChips say a query in words, as the search's filters do, to be
// read and not changed: its filters, then whatever else it finds by, as
// written.
func (s *State) queryChips(c *ui.Context, semantic theme.Theme, text string, animate bool) {
	scale := s.Settings.UIFontSize
	q := query.Parse(text)
	conditions := query.Conditions(q)
	rest := text
	for i := len(conditions) - 1; i >= 0; i-- {
		rest = query.Cut(rest, conditions[i].Span)
	}
	still := func(key, label string, words func()) {
		chip := ui.Row(c).Key(key).Role(ui.RoleListItem).Label(label).
			PaddingX(theme.Space08.DIPs(scale)).Gap(theme.Space04.DIPs(scale))
		s.chipShape(chip, semantic, false, false, animate)
		chip.Children(words)
	}
	ui.Row(c).Role(ui.RoleList).Label("Finds").Wrap().Grow(1).MinWidth(0).Gap(theme.Space06.DIPs(scale)).Children(func() {
		for i, cond := range conditions {
			still("finds-"+strconv.Itoa(i), cond.Sentence(), func() { s.chipWords(c, semantic, cond, false) })
		}
		if rest != "" {
			still("finds-words", "Matches "+rest, func() {
				components.Label(c, "Matches", theme.TextSmall, semantic.Text.Muted).SingleLine().Shrink(0)
				s.queryText(c, semantic, rest, theme.TextSmall).SingleLine().MinWidth(0)
			})
		}
	})
}

// chipWords are a filter in words: its key's name and what joins it
// quiet, its value as strong as a title, so the eye goes to the values.
func (s *State) chipWords(c *ui.Context, semantic theme.Theme, cond query.Condition, wrong bool) {
	k := cond.Key
	not := cond.Not && !k.Setting
	quiet := func(words string) {
		components.Label(c, words, theme.TextSmall, semantic.Text.Muted).SingleLine().Shrink(0)
	}
	strong := semantic.Text.Heading
	if wrong {
		strong = semantic.Status.Danger.Foreground
	}
	if k.Takes == query.TakesNothing {
		if not {
			quiet("Not")
		}
		components.Label(c, k.Label, theme.TextSmall, strong).SingleLine().MinWidth(0)
		return
	}
	quiet(k.Label)
	if verb := map[bool]string{false: k.Is, true: k.IsNot}[not]; verb != "" {
		quiet(verb)
	}
	if cond.Value == "" {
		quiet("…")
		return
	}
	components.Label(c, query.Describe(k, cond.Value), theme.TextSmall, strong).SingleLine().MinWidth(0)
}

// addFilterChip adds a filter: it lists the keys, and the one chosen
// comes as a filter with its editor open on its value.
func (s *State) addFilterChip(c *ui.Context, semantic theme.Theme, text string, count int, animate bool) {
	scale := s.Settings.UIFontSize
	ed := &s.nav.find.filter
	add := ui.ButtonBase(c.Key("add-filter")).Label("Add filter").Tooltip("Add filter").
		PaddingX(theme.Space08.DIPs(scale)).Gap(theme.Space04.DIPs(scale)).FocusRing(false)
	s.chipShape(add, semantic, ed.adding, true, animate)
	if add.FocusVisible() {
		add.Border(1, semantic.Border.Focused)
	}
	add.Children(func() {
		ui.Icon(c, components.IconPlus.SVG()).FontSize(14 * scale / theme.DefaultUIFontSize).TextColor(semantic.Icon.Muted)
		components.Label(c, "Filter", theme.TextSmall, semantic.Text.Muted).SingleLine()
	})
	if add.Clicked() {
		adding := !ed.adding
		*ed = closedFilter
		ed.adding = adding
	}
	if !ed.adding {
		return
	}
	ui.PopoverBase(c.Key("filter-keys"), add, &ed.adding, func(panel ui.Element) {
		s.filterPanel(panel, semantic, 280, animate)
		panel.Label("Add a filter")
		panel.Children(func() {
			components.Label(c, "Filter by", theme.TextXSmall, semantic.Text.Muted).
				PaddingX(theme.Space08.DIPs(scale)).PaddingY(theme.Space02.DIPs(scale))
			ui.Scroll(c).Key("keys").WidthPercent(100).MaxHeight(360 * scale / theme.DefaultUIFontSize).Children(func() {
				ui.Column(c).Role(ui.RoleListBox).Label("Filter by").WidthPercent(100).Children(func() {
					for _, k := range filterKeys() {
						if s.optionRow(c, semantic, "key-"+k.Name, k.Label, "$"+k.Name, k.Help, false, semantic.Accent.Foreground, animate) {
							*ed = closedFilter
							if k.Takes != query.TakesNothing {
								*ed = filterEditor{index: count, open: true, fresh: true, focus: true}
							}
							s.setSearch(query.And(text, query.Condition{Key: k}.String()))
						}
					}
				})
			})
		})
	})
}

// filterPanel is the look the filters' panels share: the outlined
// surface the search's suggestions open on, width units wide.
func (s *State) filterPanel(panel ui.Element, semantic theme.Theme, width float32, animate bool) {
	scale := s.Settings.UIFontSize
	components.OutlinedSurface(panel, semantic, scale)
	panel.Width(width*scale/theme.DefaultUIFontSize).Padding(theme.Space06.DIPs(scale)).
		Gap(theme.Space04.DIPs(scale)).Margin(theme.Space04.DIPs(scale), 0, 0, 0).
		Transition(popTransition(animate))
}

// optionRow is a row of a filter's panel: a mark where it is chosen, its
// label, and what stands at its end, as a count. It reports a click.
func (s *State) optionRow(c *ui.Context, semantic theme.Theme, key, label, end, help string, chosen bool, endColor ui.Color, animate bool) bool {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	row := ui.ButtonBase(c.Key(key)).Role(ui.RoleOption).Checked(chosen).Label(label).
		Height(metrics.Height).WidthPercent(100).Shrink(0).
		PaddingX(theme.Space06.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).FocusRing(false)
	if help != "" {
		row.Tooltip(help).Description(help)
	}
	hover := components.HoverAmount(row, row.Hovered(), animate)
	row.Background(components.Blend(ui.Transparent, semantic.GhostElement.Hover, hover))
	if row.FocusVisible() {
		row.Border(1, semantic.Border.Focused)
	} else {
		row.Border(1, ui.Transparent)
	}
	row.Children(func() {
		mark := ui.Icon(c, components.IconCheck.SVG()).FontSize(14 * scale / theme.DefaultUIFontSize).
			TextColor(semantic.Accent.Foreground)
		if !chosen {
			mark.Opacity(0)
		}
		components.Label(c, label, theme.TextSmall, semantic.Text.Default).SingleLine().MinWidth(0)
		ui.Box(c).Grow(1)
		if end != "" {
			components.Label(c, end, theme.TextXSmall, endColor).SingleLine().Shrink(0).FontFeatures("tnum")
		}
	})
	return row.Clicked()
}

// editFilter is a filter's editor, beside its chip: the key, whether the
// sheets it holds of are included or excluded, and its value, written or
// chosen, each value with how many sheets it would find in place of the
// one now. A filter just added and left without a value goes again.
func (s *State) editFilter(c *ui.Context, semantic theme.Theme, chip ui.Element, text string, cond query.Condition, animate bool) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	ed := &s.nav.find.filter
	set := func(next query.Condition) {
		ed.fresh = false
		s.setSearch(query.Replace(text, cond.Span, next.String()))
	}
	k := cond.Key
	ui.PopoverBase(c.Key("filter-editor"), chip, &ed.open, func(panel ui.Element) {
		s.filterPanel(panel, semantic, 300, animate)
		panel.Padding(theme.Space08.DIPs(scale)).Gap(theme.Space08.DIPs(scale)).
			Role(ui.RoleDialog).Label("Edit filter: " + cond.Sentence())
		panel.Children(func() {
			ui.Row(c).Key("filter-head").WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Children(func() {
				s.keyButton(c, semantic, k, func(other *query.Key) {
					ed.focus = other.Takes != query.TakesNothing
					set(query.Condition{Key: other, Not: cond.Not && !other.Setting})
				})
				ui.Box(c).Grow(1)
				if !k.Setting {
					include := func(not bool) func() {
						return func() {
							next := cond
							next.Not = not
							set(next)
						}
					}
					components.Segmented{
						Key: "filter-not", Label: "Include or exclude", Animate: animate,
						Options: []components.SegmentOption{
							{Key: "include", Label: "Include", Selected: !cond.Not, Action: include(false)},
							{Key: "exclude", Label: "Exclude", Selected: cond.Not, Action: include(true)},
						},
					}.View(c, semantic)
				}
			})
			components.Label(c, k.Help, theme.TextXSmall, semantic.Text.Muted).Key("filter-help").
				WidthPercent(100).PaddingX(theme.Space02.DIPs(scale)).MaxLines(2)
			if k.Takes == query.TakesNothing {
				return
			}
			value := cond.Value
			components.TextField{
				Key: "filter-value", Label: k.Label + " value", Placeholder: hintOf(k),
				Value: &value, FocusRequested: ed.focus, Animate: animate,
				Changed: func() {
					next := cond
					next.Value = value
					set(next)
				},
				Submit: func() { ed.open = false },
			}.View(c, semantic).WidthPercent(100)
			ed.focus = false
			choices := s.choices(k)
			if len(choices) == 0 {
				return
			}
			counts := s.Library.SearchCounts(query.Cut(text, cond.Span), k.Name, choices)
			ui.Scroll(c).Key("filter-values").WidthPercent(100).MaxHeight(8 * metrics.Height).Children(func() {
				ui.Column(c).Role(ui.RoleListBox).Label(k.Label + " values").WidthPercent(100).Children(func() {
					for _, v := range choices {
						end := ""
						if n, ok := counts[v]; ok {
							end = thousands(n)
						}
						label := query.Describe(k, v)
						if k.Name == "tag" && query.Fold(v) != "none" {
							label = "#" + v
						}
						if s.optionRow(c, semantic, "value-"+v, label, end, "", query.Fold(v) == query.Fold(cond.Value), semantic.Text.Muted, animate) {
							next := cond
							next.Value = v
							set(next)
							ed.open = false
						}
					}
				})
			})
		})
	})
	if !ed.open {
		// One just added and left without a value was not wanted.
		if ed.fresh && cond.Value == "" {
			s.setSearch(query.Cut(text, cond.Span))
		}
		*ed = closedFilter
	}
}

// keyButton shows a filter's key, and changes it from a menu of the keys.
func (s *State) keyButton(c *ui.Context, semantic theme.Theme, k *query.Key, choose func(*query.Key)) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	which := ui.ButtonBase(c.Key("filter-key")).Label("Filter by: "+k.Label).Tooltip("Filter by").
		Height(metrics.Height).PaddingX(theme.Space08.DIPs(scale)).AlignItems(ui.Center).
		Gap(theme.Space04.DIPs(scale)).Radius(metrics.Radius).Border(1, semantic.Border.Variant).FocusRing(false)
	if which.FocusVisible() {
		which.Border(1, semantic.Border.Focused)
	}
	which.Children(func() {
		components.Label(c, k.Label, theme.TextSmall, semantic.Text.Heading).SingleLine()
		ui.Icon(c, components.IconChevronDown.SVG()).FontSize(14 * scale / theme.DefaultUIFontSize).
			TextColor(semantic.Icon.Muted)
	})
	which.Menu(func(m *ui.Menu) {
		for _, other := range filterKeys() {
			if m.Item(other.Label).Tooltip(other.Help).Checked(other == k).Chosen() && other != k {
				choose(other)
			}
		}
	})
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
