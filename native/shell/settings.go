package shell

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The settings are a source of the field (field-and-desk.md §4.4.1). At
// $settings alone, each category stands as a block listing its sections,
// each with what it is for, as a group lists its sheets. A section pressed
// opens its category ($in:<Category>): every section of it live, the one
// pressed first and those after it following, the field's columns
// widening so live controls stand beside their labels. Words written with $settings narrow straight to
// matching live section cards. There is no settings window.

// AppSettings is the app's settings as the field shows them.
type AppSettings interface {
	// Sections are the sections words find, in order.
	Sections(words string) []SettingsSection
	// SectionView draws the section id as words find it, as wide as the
	// card it is drawn in.
	SectionView(c *ui.Context, semantic theme.Theme, id, words string)
	// Away is told the field shows no settings, so that nothing in them,
	// as a shortcut being recorded, goes on unseen.
	Away()
}

// SettingsSection is a section of the settings, as the field finds it.
type SettingsSection struct {
	ID, Category, Title, Description string
}

func sectionCategory(sec SettingsSection) string {
	if sec.Category != "" {
		return sec.Category
	}
	cat, _, _ := strings.Cut(sec.ID, "/")
	return cat
}

func settingsCategoryIcon(category string) components.IconName {
	switch strings.ToLower(category) {
	case "appearance":
		return components.IconName("palette")
	case "editor":
		return components.IconName("pen-line")
	case "keymap":
		return components.IconKeyboard
	default:
		return components.IconSettings
	}
}

// inSettings reports whether the query hands the field to the settings.
func inSettings(text string) bool {
	return slices.ContainsFunc(query.Conditions(query.Parse(text)), func(c query.Condition) bool {
		return c.Key.Name == "settings"
	})
}

// InSettings reports whether the field shows the settings.
func (s *State) InSettings() bool {
	return slices.ContainsFunc(s.field.conditions, func(text string) bool { return conditionKey(text) == "settings" })
}

// settingsScope returns the settings category/section scope ($in) and the
// search words in text.
func (s *State) settingsScope(text string) (scope, words string) {
	conds, rest := splitQuery(text)
	for _, c := range conds {
		if cond, ok := condition(c); ok && cond.Key.Name == "in" && !cond.Not {
			scope = cond.Value
		}
	}
	return scope, strings.TrimSpace(rest)
}

// settingsLive reports whether the field currently shows live settings
// section cards rather than top-level category blocks.
func (s *State) settingsLive() bool {
	if s.modeFor(s.wholeQuery()) != modeSettings {
		return false
	}
	scope, words := s.settingsScope(s.wholeQuery())
	return scope != "" || words != ""
}

// intoSettings narrows the settings query to a category, the section lead
// standing first in it, or, for none, its first.
func (s *State) intoSettings(category, lead string, from stageRect) {
	s.field.lead = lead
	s.narrow((&query.Condition{Key: mustKey("in"), Value: category}).String(), from)
}

// ShowSettings brings the field up on the settings, its pill taking the
// keys to find one; shown already, the field goes back to what it showed
// before them.
func (s *State) ShowSettings() {
	if s.InSettings() && !s.OnDesk() && s.leave("settings") {
		return
	}
	if s.InSettings() && !s.OnDesk() {
		s.field.conditions = slices.DeleteFunc(slices.Clone(s.field.conditions), func(c string) bool {
			k := conditionKey(c)
			return k == "settings" || k == "in"
		})
		s.setWords("")
		s.field.next = change{depth: stageUp}
		return
	}
	s.FindSettings("")
}

// FindSettings brings the field up on the settings the words find, its
// pill taking the keys.
func (s *State) FindSettings(words string) {
	s.ShowField()
	s.field.conditions = []string{(&query.Condition{Key: mustKey("settings")}).String()}
	s.setWords(words)
	s.field.next = change{depth: stageDown}
	s.field.focus = true
}

type settingsSource struct{}

func (settingsSource) answer(s *State, text string, f *found) {
	if s.AppSettings == nil {
		return
	}
	if s.modeFor(text) != modeSettings {
		s.AppSettings.Away()
		return
	}
	scope, words := s.settingsScope(text)
	if scope == "" && words == "" {
		// The top: each category listing its sections, as groups do.
		var order []string
		byCat := map[string][]SettingsSection{}
		for _, sec := range s.AppSettings.Sections("") {
			cat := sectionCategory(sec)
			if _, ok := byCat[cat]; !ok {
				order = append(order, cat)
			}
			byCat[cat] = append(byCat[cat], sec)
		}
		for _, cat := range order {
			f.add("settings-cat:"+cat, tierResult, settingsCategoryCard{category: cat, sections: byCat[cat]})
		}
		return
	}
	var found []SettingsSection
	for _, sec := range s.AppSettings.Sections(words) {
		if scope == "" || strings.EqualFold(sectionCategory(sec), scope) {
			found = append(found, sec)
		}
	}
	// The section pressed to open its category stands first, those after
	// it following, and those before it after them.
	if i := slices.IndexFunc(found, func(sec SettingsSection) bool { return sec.ID == s.field.lead }); i > 0 {
		found = append(found[i:], found[:i]...)
	}
	for _, sec := range found {
		f.add("settings:"+sec.ID, tierResult, settingsCard{id: sec.ID, words: words})
	}
}

// settingsCategoryCard is a category block listing its settings sections as
// rows, like a group block lists its sheets.
type settingsCategoryCard struct {
	category string
	sections []SettingsSection
}

func (sc settingsCategoryCard) shape(_ *State, _ *ui.Context, m blockMetrics) shape {
	rows := make([]row, len(sc.sections))
	for i, sec := range sc.sections {
		rows[i] = row{setting: &sc.sections[i], h: sectionRowHeight(sec, m)}
	}
	return stack(rows, m)
}

// sectionRowHeight is how tall a section's row stands: its title, and its
// description under it on a line where it has one.
func sectionRowHeight(sec SettingsSection, m blockMetrics) float32 {
	if sec.Description == "" {
		return m.groupRow
	}
	return 2*rowPad(m.scale) + theme.TextSmall.LineHeight(m.scale) + rowGap(m.scale) + theme.TextXSmall.LineHeight(m.scale)
}

func (sc settingsCategoryCard) rows() []string {
	ids := make([]string, len(sc.sections))
	for i, sec := range sc.sections {
		ids[i] = "settings-sec:" + sec.ID
	}
	return ids
}

func (settingsCategoryCard) ground() bool { return true }

func (settingsCategoryCard) drawsLive(*State) bool { return false }

func (sc settingsCategoryCard) view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, fr stageFrame, sh shape, _ bool, m blockMetrics, animate bool) {
	box.Gap(m.headGap).Children(func() {
		s.settingsCategoryHead(c, semantic, sc, fr, m, animate)
		cardRows(c, sh, m, func(r row) {
			if r.setting != nil {
				s.settingsSectionRow(c, semantic, *r.setting, r.h, fr, m, animate)
				return
			}
			s.settingsMoreRow(c, semantic, sc.category, sh.more, fr, m, animate)
		})
	})
}

func (s *State) settingsCategoryHead(c *ui.Context, semantic theme.Theme, sc settingsCategoryCard, fr stageFrame, m blockMetrics, animate bool) {
	ui.Row(c).Key("head").Height(m.head).WidthPercent(100).AlignItems(ui.Center).Gap(4 * m.k).Shrink(0).Children(func() {
		head := ui.ButtonBase(c.Key("name")).Grow(1).MinWidth(0).FillHeight().PaddingX(8 * m.k).Gap(8 * m.k).
			AlignItems(ui.Center).Radius(8 * m.k).FocusRing(false).KeepFocus().Label(sc.category)
		hover := components.HoverAmount(head, head.Hovered(), animate)
		head.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
		if head.Clicked() {
			s.intoSettings(sc.category, "", fr.Rect)
		}
		head.Children(func() {
			ui.Icon(c, settingsCategoryIcon(sc.category).SVG()).FontSize(15 * m.k).TextColor(semantic.Icon.Muted).Shrink(0)
			components.BaselineRow(c).Key("words").Grow(1).MinWidth(0).Gap(8 * m.k).Children(func() {
				components.Label(c, sc.category, theme.TextDefault, semantic.Text.Heading).Grow(1).MinWidth(0).SingleLine()
				components.Label(c, thousands(len(sc.sections)), theme.TextXSmall, semantic.Text.Muted).
					SingleLine().Shrink(0).FontFeatures("tnum")
			})
		})
	})
}

func (s *State) settingsSectionRow(c *ui.Context, semantic theme.Theme, sec SettingsSection, h float32, fr stageFrame, m blockMetrics, animate bool) {
	id := "settings-sec:" + sec.ID
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(h).PaddingX(rowPad(m.scale)).
		Gap(8 * m.k).AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(sec.Title)
	s.rowFill(c, semantic, btn, id, animate)
	if btn.Clicked() {
		s.intoSettings(sectionCategory(sec), sec.ID, fr.Rect)
	}
	btn.Children(func() {
		ui.Column(c).Key("words").Grow(1).MinWidth(0).Gap(rowGap(m.scale)).Children(func() {
			components.Label(c, sec.Title, theme.TextSmall, semantic.Text.Default).SingleLine()
			if sec.Description != "" {
				components.Label(c, sectionLine(sec.Description), theme.TextXSmall, semantic.Text.Muted).SingleLine()
			}
		})
		ui.Icon(c, components.IconChevronRight.SVG()).FontSize(12 * m.k).TextColor(semantic.Icon.Muted).Shrink(0)
	})
}

func (s *State) settingsMoreRow(c *ui.Context, semantic theme.Theme, category string, more int, fr stageFrame, m blockMetrics, animate bool) {
	label := fmt.Sprintf("%s more", thousands(more))
	btn := ui.ButtonBase(c.Key("more")).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).FocusRing(false).KeepFocus().Label(label)
	s.rowFill(c, semantic, btn, "", animate)
	if btn.Clicked() {
		s.intoSettings(category, "", fr.Rect)
	}
	btn.Children(func() {
		components.CenteredLabel(c, label, theme.TextXSmall, semantic.Text.Muted)
	})
}

// settingsCard is a section of the settings: drawn live, as its controls
// take the pointer and the keys, and as tall as it was last laid out.
type settingsCard struct {
	id, words string
}

func (sc settingsCard) shape(s *State, c *ui.Context, m blockMetrics) shape {
	h, ok := s.field.measured[sc.id]
	if !ok {
		h = 240 * m.k
	}
	return shape{size: stageSize{W: m.col + 2*m.pad, H: h + 2*m.pad}}
}

func (settingsCard) rows() []string { return nil }

// ground: a section draws its own, as a title over a card of its rows.
func (settingsCard) ground() bool { return false }

func (settingsCard) drawsLive(*State) bool { return true }

func (sc settingsCard) view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, fr stageFrame, sh shape, current bool, m blockMetrics, animate bool) {
	box.Children(func() {
		content := ui.Column(c).Key("section").WidthPercent(100).Shrink(0)
		// It is as tall as it lays itself out; a change, as a fold
		// opening, lays the field out again around it.
		if h := content.LayoutBounds().H; h > 0 && current && abs32(h-s.field.measured[sc.id]) > 0.5 {
			if s.field.measured == nil {
				s.field.measured = map[string]float32{}
			}
			s.field.measured[sc.id] = h
			s.field.measures++
			if s.field.settleAt.IsZero() {
				s.field.settled = true
			}
			c.AnimationFrame()
		}
		content.Children(func() { s.AppSettings.SectionView(c, semantic, sc.id, sc.words) })
	})
}

// settingsButton puts the settings in the field, or takes them away, a
// round button size wide, filled while they are shown.
func (s *State) settingsButton(c *ui.Context, semantic theme.Theme, size float32, animate bool) ui.Element {
	shown := s.InSettings()
	label := map[bool]string{true: "Hide settings", false: "Show settings"}[shown]
	btn := laneCircleButton(c, semantic, "settings", components.IconSettings, label, "Settings", s.binding("app.settings"), shown, size, animate)
	if btn.Clicked() {
		s.ShowSettings()
	}
	return btn
}

func abs32(x float32) float32 { return max(x, -x) }

// ShowShortcuts brings the field up on the keymap's settings: every
// command and its key, where each is changed.
func (s *State) ShowShortcuts() {
	s.FindSettings("")
	s.intoSettings("Keymap", "", stageRect{})
}

// sectionLine is what a section's row says of it under its title: its
// description's first clause, which reads whole on a line where the rest
// would be cut off, begun as a sentence is.
func sectionLine(desc string) string {
	line := desc
	for _, stop := range []string{". ", ": ", "; ", " — "} {
		if i := strings.Index(line, stop); i > 0 {
			line = line[:i]
		}
	}
	line = strings.TrimSuffix(strings.TrimSpace(line), ".")
	if r, size := utf8.DecodeRuneInString(line); size > 0 {
		line = string(unicode.ToUpper(r)) + line[size:]
	}
	return line
}
