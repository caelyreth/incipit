package shell

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// queryInput is a query field's state: the completion offered at its
// caret and the suggestion the keys are on in it. A sheet's find keeps one.
type queryInput struct {
	// completion is what may stand at the caret in query at caret, a byte
	// offset; suggestion is the item the keys are on, and first the first
	// the list shows.
	completion query.Completion
	query      string
	caret      int
	suggestion int
	first      int
	// pointer is where the pointer was over the suggestions, pointed
	// whether it was there.
	pointer ui.Point
	pointed bool
	// dismissed is the query the completion was put away at, with Escape
	// or a press elsewhere: it comes back once the query changes.
	dismissed string
	focus     ui.Handle
	edit      *queryEdit
}

type queryEdit struct {
	from, to int
	text     string
}

// queryField is a field a query is written in: set as the query language
// reads it, with what may be written at its caret offered under it.
type queryField struct {
	Key, Label, Placeholder string
	Value                   *string
	In                      *queryInput
	// Scope is where the query runs, which says the keys offered.
	Scope query.Scope
	// Fixed is a key that stands before what is written and is not
	// written itself, as $sheet stands in a sheet's find: the field shows
	// it as a token in a key's color, and the query is it and Value.
	// FixedLabel says what it means, for the pointer and for assistive
	// technology.
	Fixed, FixedLabel string
	// Focus asks the field for the focus in this frame.
	Focus   bool
	Animate bool
	// Keys is what a key the completion does not take does; it reports
	// whether it took it.
	Keys func(k ui.TextKey) bool
}

// View builds the field: the row it stands in, and its input.
func (f queryField) View(s *State, c *ui.Context, semantic theme.Theme) (field, input ui.Element) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	text := theme.TextSmall
	in := f.In
	field = ui.Row(c).Key(f.Key+"-field").Height(metrics.Height).MinWidth(0).
		PaddingStart(theme.Space08.DIPs(scale)).PaddingEnd(theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Inset).Cursor(ui.CursorText)
	field.Children(func() {
		ui.Icon(c, components.IconSearch.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).
			TextColor(semantic.Icon.Muted)
		if f.Fixed != "" {
			// The fixed key is a token, not text: it reads as where the
			// query runs, and no key takes it away.
			ui.Row(c).Key("fixed").Label(f.FixedLabel).Shrink(0).
				AlignItems(ui.Center).PaddingX(theme.Space04.DIPs(scale)).
				Radius(metrics.Radius - 2).Background(semantic.Accent.Muted).Children(func() {
				components.CenteredLabel(c, f.Fixed, theme.TextXSmall, semantic.Accent.Foreground)
			})
		}
		input = ui.TextInputBase(c.Key(f.Key), f.Value).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(text.Size(scale)).FixedLineHeight(text.LineHeight(scale)).
			TextColor(semantic.Text.Default).
			Placeholder(f.Placeholder).Label(f.Label).Bind(&in.focus)
		input.OnTextKey(func(k ui.TextKey) bool { return s.queryKey(in, k) || f.Keys != nil && f.Keys(k) })
		if edit := in.edit; edit != nil {
			in.edit = nil
			input.ReplaceText(edit.from, edit.to, edit.text)
		}
		input.TextRanges(queryRanges(f.prefix(), *f.Value, semantic)...)
		if f.Focus {
			input.Focus()
		}
		if *f.Value != "" {
			components.IconButton{
				Key: "clear", Label: locales.Clear(), Icon: components.IconClose.SVG(),
				Style: components.SubtleButton, Size: components.CompactButton,
				IconColor: semantic.Icon.Muted, Animate: f.Animate,
				Action: func() {
					input.ReplaceText(0, math.MaxInt, "").Focus(ui.FocusOptions{PreventScroll: true})
				},
			}.View(c, semantic)
		}
	})
	if input.Focused() {
		field.Border(1, semantic.Border.Focused)
	}
	_, caret := input.TextSelection()
	s.complete(in, f.prefix(), *f.Value, byteOffset(*f.Value, caret), f.Scope)
	open := input.Focused() && len(in.completion.Items) > 0 && in.dismissed != *f.Value
	if open {
		s.suggestions(c, semantic, in, f.Value, field, input, &open, f.Animate)
		if !open {
			in.dismissed = *f.Value
		}
	}
	return field, input
}

// prefix is what the query has before what is written: the fixed key and
// a space.
func (f queryField) prefix() string {
	if f.Fixed == "" {
		return ""
	}
	return f.Fixed + " "
}

// complete offers what may stand at caret in text, as either changes. The
// query is prefix and text; the completion's span is in text.
func (s *State) complete(in *queryInput, prefix, text string, caret int, scope query.Scope) {
	if in.query == text && in.caret == caret {
		return
	}
	var names func(string) []string
	if s.Library != nil {
		names = s.Library.SearchNames
	}
	in.query, in.caret, in.suggestion, in.first = text, caret, 0, 0
	in.completion = query.Complete(prefix+text, len(prefix)+caret, scope, names)
	in.completion.Span.From = max(in.completion.Span.From-len(prefix), 0)
	in.completion.Span.To = max(in.completion.Span.To-len(prefix), 0)
}

// completing reports whether a completion is offered for text, the query
// as a key sees it.
func (in *queryInput) completing(text string) bool {
	return in.query == text && len(in.completion.Items) > 0 && in.dismissed != text
}

// suggestionsShown is how many suggestions the completion lists.
const suggestionsShown = 8

// suggestions lists the completion by the field, on the outlined surface
// the page's tag picker has, as a part of the field rather than a layer
// over the window: under it, or over it where there is no room below. The
// chosen row's fill slides as the keys or the pointer move it, and the
// chosen key's help is said whole at the foot.
func (s *State) suggestions(c *ui.Context, semantic theme.Theme, in *queryInput, value *string, field, input ui.Element, open *bool, animate bool) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	pad := theme.Space04.DIPs(scale)
	row := metrics.Height
	all := in.completion.Items
	in.suggestion = min(in.suggestion, len(all)-1)
	chosen := all[in.suggestion]
	// The rows shown are a window on the suggestions that keeps the chosen
	// one in it, as the keys move it past either end.
	in.first = min(max(in.first, in.suggestion-suggestionsShown+1), in.suggestion, max(len(all)-suggestionsShown, 0))
	items := all[in.first:min(in.first+suggestionsShown, len(all))]
	width := field.LayoutBounds().W
	ui.PopoverBase(c.Key("suggestions"), field, open, func(panel ui.Element) {
		components.OutlinedSurface(panel, semantic, scale)
		panel.Width(width).Padding(pad).Margin(pad, 0, 0, 0).
			Role(ui.RoleList).Label(locales.Suggestions()).Transition(popTransition(animate))
		// Only the pointer's moves choose the row under it, so a list the
		// keys move under a resting pointer keeps their choice.
		px, py, over := panel.PointerPosition()
		if p := (ui.Point{X: px, Y: py}); over {
			if in.pointed && p != in.pointer {
				if i := int((py - 1 - pad) / row); i >= 0 && i < len(items) {
					in.suggestion = in.first + i
				}
			}
			in.pointer, in.pointed = p, true
		} else {
			in.pointed = false
		}
		var spring *ui.Animation
		if animate {
			sp := components.Spring(components.MotionNormal)
			spring = &sp
		}
		panel.Children(func() {
			ui.Box(c).Key("suggestion-chosen").Absolute().Left(pad).Right(pad).
				Top(pad + float32(in.suggestion-in.first)*row).Height(row).
				Radius(metrics.PanelRadius - pad).Background(semantic.GhostElement.Hover).ZIndex(-1).
				Transition(ui.ElementTransition{Animation: spring, Position: true})
			for i, item := range items {
				button := ui.ButtonBase(c.Key("suggestion-" + item.Label)).Height(row).WidthPercent(100).Shrink(0).
					PaddingX(theme.Space06.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
					FocusRing(false).KeepFocus().Selected(in.first+i == in.suggestion).Background(ui.Transparent).Label(item.Label)
				button.Children(func() {
					if item.Of == "tag" {
						components.TagChip(c, "#"+strings.TrimPrefix(item.Label, "#"), theme.TextSmall,
							theme.Space04.DIPs(scale), width, semantic.Tag.Background, semantic.Tag.Foreground)
						ui.Box(c).Grow(1)
						if item.Hint != "" {
							components.CenteredLabel(c, item.Hint, theme.TextXSmall, semantic.Text.Muted).
								Shrink(0)
						}
						return
					}
					components.BaselineRow(c).Grow(1).MinWidth(0).Gap(theme.Space08.DIPs(scale)).Children(func() {
						color := semantic.Text.Default
						if item.Of == "" {
							color = semantic.Accent.Foreground
							components.Label(c, item.Label, theme.TextSmall, color).SingleLine().Shrink(0)
						} else {
							components.Label(c, item.Label, theme.TextSmall, color).SingleLine().MinWidth(0)
						}
						ui.Box(c).Grow(1)
						if item.Hint != "" {
							components.Label(c, item.Hint, theme.TextXSmall, semantic.Text.Muted).SingleLine().Shrink(0)
						}
					})
				})
				if button.Clicked() {
					edit := in.replacement(*value, item)
					input.ReplaceText(edit.from, edit.to, edit.text).Focus(ui.FocusOptions{PreventScroll: true})
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

func (in *queryInput) replacement(text string, item query.Suggestion) *queryEdit {
	span := in.completion.Span
	edit := &queryEdit{
		from: utf8.RuneCountInString(text[:span.From]),
		to:   utf8.RuneCountInString(text[:span.To]),
		text: item.Text,
	}
	if item.Done {
		edit.text += " "
		if strings.HasPrefix(text[span.To:], " ") {
			edit.to++
		}
	}
	return edit
}

// queryKey is what the keys do in a field's completion while it is
// offered: Up and Down go through the suggestions, Tab and Enter take one,
// Escape puts them away. It reports whether it took the key.
func (s *State) queryKey(in *queryInput, k ui.TextKey) bool {
	text := k.Text()
	if !in.completing(text) {
		return false
	}
	items := len(in.completion.Items)
	switch k.Key {
	case ui.KeyDown, ui.KeyUp:
		if k.Mods != 0 {
			return false
		}
		step := 1
		if k.Key == ui.KeyUp {
			step = -1
		}
		in.suggestion = (in.suggestion + step + items) % items
		return true
	case ui.KeyTab, ui.KeyEnter:
		if k.Mods != 0 {
			return false
		}
		in.edit = in.replacement(text, in.completion.Items[in.suggestion])
		return true
	case ui.KeyEscape:
		in.dismissed = text
		return true
	}
	return false
}

// queryColors are the colors a query's parts are set in, as the language
// reads it: keys and tags in the accent, their values in the heading's
// color, a - quieter, what is wrong marked and what means nothing here
// muted. Each is a byte span of text; a later one is set over an earlier.
func queryColors(text string, semantic theme.Theme) []queryColor {
	q := query.Parse(text)
	var out []queryColor
	add := func(sp query.Span, color ui.Color) { out = append(out, queryColor{sp, color}) }
	query.Walk(q.Root, func(n query.Node) {
		switch n := n.(type) {
		case *query.Filter:
			name := query.Span{From: n.From, To: n.To}
			if n.HasValue && !n.Hash {
				name.To = n.ValueSpan.From
				add(n.ValueSpan, semantic.Text.Heading)
			}
			add(name, semantic.Accent.Foreground)
		case *query.Not:
			add(query.Span{From: n.From, To: n.From + 1}, semantic.Text.Muted)
		}
	})
	// What is still being written keeps its colors; what is wrong is
	// marked, and what means nothing here is quiet.
	for _, issue := range q.Issues {
		switch issue.Kind {
		case query.Unknown, query.Invalid:
			add(issue.Span, semantic.Status.Danger.Foreground)
		case query.Inert:
			add(issue.Span, semantic.Text.Muted)
		}
	}
	return out
}

// queryColor is a part of a query and its color.
type queryColor struct {
	span  query.Span
	color ui.Color
}

// queryRanges sets what is written of a query, after prefix, in its
// colors (queryColors), as a field's text ranges.
func queryRanges(prefix, written string, semantic theme.Theme) []ui.TextRange {
	var out []ui.TextRange
	for _, qc := range queryColors(prefix+written, semantic) {
		from, to := max(qc.span.From-len(prefix), 0), qc.span.To-len(prefix)
		if to > from {
			out = append(out, ui.TextRange{
				Start: utf8.RuneCountInString(written[:from]), End: utf8.RuneCountInString(written[:to]), Color: qc.color,
			})
		}
	}
	return out
}

// queryText is a query set in its colors (queryColors), as text to read:
// the field's look, where it cannot be written in.
func (s *State) queryText(c *ui.Context, semantic theme.Theme, text string, size theme.TextSize) ui.Element {
	scale := s.Settings.UIFontSize
	colors := make([]ui.Color, len(text))
	for i := range colors {
		colors[i] = semantic.Text.Default
	}
	for _, qc := range queryColors(text, semantic) {
		for i := qc.span.From; i < qc.span.To && i < len(text); i++ {
			colors[i] = qc.color
		}
	}
	var spans []ui.Span
	for from := 0; from < len(text); {
		to := from + 1
		for to < len(text) && colors[to] == colors[from] {
			to++
		}
		spans = append(spans, ui.Span{Text: text[from:to], Font: theme.UIFontFamily, Size: size.Size(scale), Color: colors[from]})
		from = to
	}
	return ui.RichText(c, spans...).FixedLineHeight(size.LineHeight(scale))
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
