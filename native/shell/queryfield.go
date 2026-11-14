package shell

import (
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// queryInput is a query field's state: the completion offered at its
// caret and the suggestion the keys are on in it. The sidebar's search and
// a sheet's find each keep one.
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
	// selecting, when set, is the runes the next frame selects in the
	// field: a caret after a suggestion taken, or the words a find starts
	// with.
	selecting bool
	selection [2]int
}

// selectNext selects runes from to to in the field's next frame.
func (in *queryInput) selectNext(from, to int) {
	in.selecting, in.selection = true, [2]int{from, to}
}

// queryField is a field a query is written in: set as the query language
// reads it, with what may be written at its caret offered under it.
type queryField struct {
	Key, Label, Placeholder string
	Value                   *string
	In                      *queryInput
	// Scope is where the query runs, which says the keys offered.
	Scope query.Scope
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
		input = ui.TextInputBase(c.Key(f.Key), f.Value).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(text.Size(scale)).FixedLineHeight(text.LineHeight(scale)).
			TextColor(semantic.Text.Default).
			Placeholder(f.Placeholder).Label(f.Label)
		input.TextRanges(queryRanges(*f.Value, semantic)...)
		input.OnTextKey(func(k ui.TextKey) bool { return s.queryKey(in, k) || f.Keys != nil && f.Keys(k) })
		if f.Focus {
			input.Focus()
		}
		if in.selecting {
			input.SetTextSelection(in.selection[0], in.selection[1])
			in.selecting = false
		}
		if *f.Value != "" {
			components.IconButton{
				Key: "clear", Label: "Clear", Icon: components.IconClose.SVG(),
				Style: components.SubtleButton, Size: components.CompactButton,
				IconColor: &semantic.Icon.Muted, Animate: f.Animate,
				Action: func() {
					*f.Value = ""
					input.SetChanged()
					input.Focus()
				},
			}.View(c, semantic)
		}
	})
	if input.Focused() {
		field.Border(1, semantic.Border.Focused)
	}
	_, caret := input.TextSelection()
	s.complete(in, *f.Value, byteOffset(*f.Value, caret), f.Scope)
	open := input.Focused() && len(in.completion.Items) > 0 && in.dismissed != *f.Value
	if open {
		s.suggestions(c, semantic, in, f.Value, field, input, &open, f.Animate)
		if !open {
			in.dismissed = *f.Value
		}
	}
	return field, input
}

// complete offers what may stand at caret in text, as either changes.
func (s *State) complete(in *queryInput, text string, caret int, scope query.Scope) {
	if in.query == text && in.caret == caret {
		return
	}
	var names func(string) []string
	if s.Library != nil {
		names = s.Library.SearchNames
	}
	in.query, in.caret, in.suggestion, in.first = text, caret, 0, 0
	in.completion = query.Complete(text, caret, scope, names)
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
					FocusRing(false).Selected(in.first+i == in.suggestion).Background(ui.Transparent).Label(item.Label)
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
					text, caret := in.take(*value, item)
					*value = text
					in.selectNext(caret, caret)
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
func (in *queryInput) take(text string, item query.Suggestion) (string, int) {
	span := in.completion.Span
	before, after := text[:span.From]+item.Text, text[span.To:]
	if item.Done {
		// Past the space after it, written now or there already.
		after = " " + strings.TrimPrefix(after, " ")
		return before + after, utf8.RuneCountInString(before) + 1
	}
	return before + after, utf8.RuneCountInString(before)
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
		out, caret := in.take(text, in.completion.Items[in.suggestion])
		k.SetText(out)
		in.selectNext(caret, caret)
		return true
	case ui.KeyEscape:
		in.dismissed = text
		return true
	}
	return false
}

// queryRanges sets a query as the language reads it: keys and tags in the
// accent, their values in the heading's color, a - quieter, what is wrong
// marked and what means nothing here muted.
func queryRanges(text string, semantic theme.Theme) []ui.TextRange {
	q := query.Parse(text)
	var out []ui.TextRange
	add := func(sp query.Span, color ui.Color) {
		if sp.To > sp.From {
			out = append(out, ui.TextRange{
				Start: utf8.RuneCountInString(text[:sp.From]), End: utf8.RuneCountInString(text[:sp.To]), Color: color,
			})
		}
	}
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
