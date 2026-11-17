package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The settings are a source of the field (field-and-desk.md §4.4.1): the
// query holding $settings hands it to them, and its words find their
// sections, each a card of its controls, live. The settings button in the
// field lane, and the Settings command, put $settings in the query; there
// is no settings window. While the settings answer, the field's columns
// are wider, for a setting's words and its control to stand side by side.

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
	ID, Title string
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

// ShowSettings brings the field up on the settings, its pill taking the
// keys to find one; shown already, the field goes back to what it showed
// before them.
func (s *State) ShowSettings() {
	settings := (&query.Condition{Key: mustKey("settings")}).String()
	if s.InSettings() && !s.OnDesk() {
		s.field.conditions = slices.DeleteFunc(slices.Clone(s.field.conditions), func(c string) bool { return c == settings })
		s.setWords("")
		s.field.next = change{depth: stage.Up}
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
	s.field.next = change{depth: stage.Down}
	s.field.focus = true
}

type settingsSource struct{}

func (settingsSource) answer(s *State, text string, f *found) {
	if s.AppSettings == nil {
		return
	}
	if !inSettings(text) || strings.HasPrefix(s.field.words, ">") {
		s.AppSettings.Away()
		return
	}
	_, words := splitQuery(text)
	for _, sec := range s.AppSettings.Sections(words) {
		f.add("settings:"+sec.ID, tierResult, settingsCard{id: sec.ID, words: words})
	}
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
	return shape{size: stage.Size{W: m.col + 2*m.pad, H: h + 2*m.pad}}
}

func (settingsCard) rows() []string { return nil }

// ground: a section draws its own, as a title over a card of its rows.
func (settingsCard) ground() bool { return false }

func (settingsCard) drawsLive(*State) bool { return true }

func (sc settingsCard) view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, fr stage.Frame, sh shape, current bool, m blockMetrics, animate bool) {
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
	btn := ui.ButtonBase(c.Key("settings")).Size(size, size).Radius(size / 2).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).Label(label).Tooltip("Settings").Selected(shown)
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	ground, ink := semantic.Surface.Page, semantic.Icon.Muted
	if shown {
		ground, ink = semantic.Accent.Muted, semantic.Accent.Foreground
	}
	btn.Background(components.Blend(ground, semantic.GhostElement.Hover, hover)).Border(1, semantic.Border.Variant)
	if btn.Clicked() {
		s.ShowSettings()
	}
	btn.Children(func() {
		ui.Icon(c, components.IconSettings.SVG()).FontSize(size / 2).TextColor(ink)
	})
	return btn
}

func abs32(x float32) float32 { return max(x, -x) }
