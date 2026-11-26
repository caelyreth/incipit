package settings

import (
	"cmp"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/app/fonts"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

type fontPickerState struct {
	open    bool
	wasOpen bool
	query   string
	active  int
	scroll  ui.ScrollState
}

type fontPickerItem struct {
	key        string
	family     string // "" means system default matching Style
	label      string
	note       string
	badge      string
	selected   bool
	registered bool
}

// typefacePreview draws a compact live specimen using the active prose font
// stack so font and style choices give immediate visual feedback.
func (s *State) typefacePreview(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	k := scale / theme.DefaultUIFontSize
	inset := theme.Space16.DIPs(scale)
	family := s.previewFamily()
	ui.Column(c).Key("typeface-preview").WidthPercent(100).MinWidth(0).Children(func() {
		ui.Row(c).WidthPercent(100).MinWidth(0).Padding(theme.Space12.DIPs(scale), inset, theme.Space12.DIPs(scale), inset).
			Children(func() {
				ui.Column(c).WidthPercent(100).MinWidth(0).Padding(theme.Space12.DIPs(scale), theme.Space16.DIPs(scale)).
					Gap(theme.Space06.DIPs(scale)).
					Radius(theme.DefaultControls.Scaled(scale).Radius).
					Border(components.Rim, semantic.Border.Variant).
					Background(semantic.Surface.Inset).Children(func() {
					ui.Row(c).WidthPercent(100).AlignItems(ui.Baseline).Gap(theme.Space12.DIPs(scale)).Children(func() {
						ui.Text(c, "Ag").Font(family).FontSize(22 * k).FontWeight(600).
							TextColor(semantic.Text.Heading).SingleLine().Shrink(0)
						ui.Text(c, locales.TypefaceSample()).Font(family).FontSize(14 * k).
							TextColor(semantic.Text.Default).MinWidth(0).Grow(1)
					})
					ui.Row(c).WidthPercent(100).AlignItems(ui.Baseline).Gap(theme.Space12.DIPs(scale)).Children(func() {
						ui.Text(c, locales.TypefaceSample()).Font(family).Italic().FontSize(13 * k).
							TextColor(semantic.Text.Muted).MinWidth(0).Grow(1)
					})
				})
			})
		ui.Row(c).WidthPercent(100).PaddingStart(inset).Children(func() {
			components.HorizontalDivider(c, semantic, components.VariantDivider)
		})
	})
}

func (s *State) previewFamily() string {
	values := s.Data.AppearanceValues
	base := theme.ProseFontFamily
	if values.Typeface == fonts.Sans {
		base = theme.ProseSansFamily
	}
	if s.Theme.ProseFamily != "" {
		return s.Theme.ProseFamily
	}
	primary := values.ProseFont
	if f, ok := fonts.LookupFamily(primary); ok {
		kept := false
		for _, row := range s.Fonts {
			if row.Face.ID == f.ID && row.Kept {
				kept = true
				break
			}
		}
		if !kept {
			primary = ""
		}
	}
	return fonts.ResolveFamily(base, primary)
}

// fontPickerRow renders the prose font picker SettingRow and its searchable
// font selector popover.
func (s *State) fontPickerRow(c *ui.Context, semantic theme.Theme) {
	values := s.Data.AppearanceValues
	current := values.ProseFont
	buttonLabel := current
	if buttonLabel == "" {
		buttonLabel = locales.SystemDefault()
	}
	var reset func()
	if current != "" {
		reset = func() { s.chooseCustomFont("") }
	}
	var trigger ui.Element
	components.SettingRow{
		Label:       locales.ProseFont(),
		Description: locales.ProseFontDescription(),
		Reset:       reset,
		ResetIcon:   components.IconUndo.SVG(),
		Control: func() {
			trigger = components.ChoiceButton{
				Key:      "prose-font-picker",
				Label:    buttonLabel,
				Selected: s.fontPicker.open,
				Animate:  s.motion(c),
				Size:     components.MediumButton,
				Action: func() {
					s.fontPicker.open = !s.fontPicker.open
					if s.fontPicker.open {
						s.fontPicker.query = ""
						s.fontPicker.active = 0
					}
				},
			}.View(c, semantic)
		},
	}.View(c, semantic)

	shown := s.fontPicker.open
	if shown {
		items := s.fontPickerItems()
		s.handleFontPickerKeys(c, items)
		ui.PopoverBase(c.Key("font-picker-popover"), trigger, &shown, func(panel ui.Element) {
			s.fontPickerPanel(c, semantic, panel, items)
		})
	}
	if !shown && s.fontPicker.open {
		s.fontPicker.open = false
	}
	s.fontPicker.wasOpen = s.fontPicker.open
}

func (s *State) chooseCustomFont(family string) {
	s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
		Kind: appsettings.SetProseFont,
		Font: family,
	}))
}

func (s *State) fontPickerItems() []fontPickerItem {
	values := s.Data.AppearanceValues
	selected := values.ProseFont
	fallback := theme.ProseFontFamily
	if values.Typeface == fonts.Sans {
		fallback = theme.ProseSansFamily
	}
	query := fold(s.fontPicker.query)
	matches := func(str string) bool {
		return query == "" || containsFold(str, query)
	}

	var items []fontPickerItem
	sysLabel := locales.SystemDefault()
	if matches(sysLabel + " " + fallback) {
		items = append(items, fontPickerItem{
			key:      "default",
			family:   "",
			label:    sysLabel,
			note:     locales.SystemDefaultNote(fallback),
			selected: selected == "",
		})
	}

	// Collect kept managed families so we can tag them and surface any not
	// already in SystemFonts (for headless/test runs).
	keptManaged := map[string]bool{}
	for _, row := range s.Fonts {
		if row.Kept {
			keptManaged[strings.ToLower(row.Face.Family)] = true
		}
	}

	families := slices.Clone(s.SystemFonts)
	if len(families) == 0 {
		for _, row := range s.Fonts {
			if row.Kept {
				families = append(families, ui.FontFamily{Name: row.Face.Family, Registered: true})
			}
		}
		slices.SortFunc(families, func(a, b ui.FontFamily) int {
			return cmp.Or(strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), strings.Compare(a.Name, b.Name))
		})
	}

	seen := map[string]bool{}
	for _, f := range families {
		name := strings.TrimSpace(f.Name)
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		low := strings.ToLower(name)
		if seen[low] {
			continue
		}
		seen[low] = true
		if !matches(name) {
			continue
		}
		badge := ""
		if f.Registered || keptManaged[low] {
			badge = locales.FontBadgeDownloaded()
		} else if f.Monospaced {
			badge = locales.FontBadgeMono()
		}
		items = append(items, fontPickerItem{
			key:        "font-" + name,
			family:     name,
			label:      name,
			badge:      badge,
			selected:   selected != "" && strings.EqualFold(selected, name),
			registered: f.Registered || keptManaged[low],
		})
	}
	return items
}

func (s *State) handleFontPickerKeys(c *ui.Context, items []fontPickerItem) {
	if c.Shortcut(0, ui.KeyEscape) {
		s.fontPicker.open = false
		return
	}
	if len(items) == 0 {
		s.fontPicker.active = 0
		return
	}
	if s.fontPicker.active >= len(items) {
		s.fontPicker.active = len(items) - 1
	}
	if s.fontPicker.active < 0 {
		s.fontPicker.active = 0
	}
	if c.Shortcut(0, ui.KeyDown) {
		s.fontPicker.active = (s.fontPicker.active + 1) % len(items)
	}
	if c.Shortcut(0, ui.KeyUp) {
		s.fontPicker.active = (s.fontPicker.active - 1 + len(items)) % len(items)
	}
	if c.Shortcut(0, ui.KeyEnter) {
		picked := items[s.fontPicker.active]
		s.chooseCustomFont(picked.family)
		s.fontPicker.open = false
	}
}

func (s *State) fontPickerPanel(c *ui.Context, semantic theme.Theme, panel ui.Element, items []fontPickerItem) {
	scale := s.Theme.UIFontSize
	k := scale / theme.DefaultUIFontSize
	pad := theme.Space08.DIPs(scale)
	controls := theme.DefaultControls.Scaled(scale)
	animate := s.motion(c)

	components.ElevatedSurface(panel, semantic, scale, components.PopoverElevation)
	panel.Width(310*k).MaxWidthPercent(100).Padding(pad).
		Margin(theme.Space04.DIPs(scale), 0, 0, 0).
		Role(ui.RoleDialog).Label(locales.ProseFont())
	if animate {
		spring := components.Settle(components.MotionNormal)
		rise := -theme.Space04.DIPs(scale)
		panel.Transition(ui.ElementTransition{
			Animation: &spring, Enter: &ui.Motion{Y: rise, Opacity: 0},
			Exit: &ui.Motion{Y: rise / 2, Opacity: 0}, ExitDuration: components.MotionQuick,
		})
	}
	panel.Children(func() {
		ui.Column(c).WidthPercent(100).MinWidth(0).Gap(theme.Space08.DIPs(scale)).Children(func() {
			components.TextField{
				Key:         "font-picker-search",
				Label:       locales.SearchFonts(),
				Placeholder: locales.SearchFonts(),
				Value:       &s.fontPicker.query,
				Icon:        components.IconSearch.SVG(),
				ClearIcon:   components.IconClose.SVG(),
				Animate:     animate,
				AutoFocus:   !s.fontPicker.wasOpen,
				Changed: func() {
					s.fontPicker.active = 0
				},
			}.View(c, semantic)

			if len(items) == 0 {
				ui.Row(c).WidthPercent(100).Padding(theme.Space12.DIPs(scale), theme.Space08.DIPs(scale)).
					Justify(ui.Center).Children(func() {
					components.Label(c, locales.NoMatchingFonts(), theme.TextSmall, semantic.Text.Muted)
				})
				return
			}

			ui.Scroll(c).Key("font-picker-list").WidthPercent(100).MaxHeight(260*k).
				Scrollbars(true).ScrollbarAxes(false, true).
				TrackScroll(&s.fontPicker.scroll).Children(func() {
				ui.Column(c).WidthPercent(100).Gap(theme.Space02.DIPs(scale)).Children(func() {
					for i, item := range items {
						s.fontPickerOption(c, semantic, item, i == s.fontPicker.active, controls, k, animate)
					}
				})
			})
		})
	})
}

func (s *State) fontPickerOption(
	c *ui.Context,
	semantic theme.Theme,
	item fontPickerItem,
	highlighted bool,
	controls theme.ControlMetrics,
	k float32,
	animate bool,
) {
	scale := s.Theme.UIFontSize
	row := ui.ButtonBase(c.Key(item.key)).Role(ui.RoleOption).Label(item.label).Selected(item.selected).
		WidthPercent(100).MinWidth(0).MinHeight(controls.Height).
		Padding(theme.Space04.DIPs(scale), theme.Space08.DIPs(scale)).
		Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).
		Radius(controls.Radius).KeepFocus().FocusRing(false)

	hovered := row.Hovered() || highlighted
	switch {
	case item.selected:
		row.Background(semantic.Element.Selected)
	case hovered:
		hover := components.HoverAmount(row, hovered, animate)
		row.Background(semantic.GhostElement.Hover.Alpha(hover))
	default:
		components.HoverAmount(row, false, animate)
	}

	row.Children(func() {
		mark := ui.Box(c).Size(14*k, 14*k).Shrink(0).AlignItems(ui.Center).Justify(ui.Center)
		if item.selected {
			mark.Children(func() {
				ui.Icon(c, components.IconCheck.SVG()).FontSize(14 * k).TextColor(semantic.Accent.Foreground)
			})
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			components.Label(c, item.label, theme.TextSmall, semantic.Text.Default).SingleLine()
			if item.note != "" {
				components.Label(c, item.note, theme.TextXSmall, semantic.Text.Muted).SingleLine()
			}
		})
		if item.badge != "" {
			components.BadgeFrame(ui.Box(c).Background(semantic.Tag.Background), scale).
				AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				components.CenteredLabel(c, item.badge, theme.TextXSmall, semantic.Tag.Foreground)
			})
		}
	})
	if row.Clicked() {
		s.chooseCustomFont(item.family)
		s.fontPicker.open = false
	}
}
