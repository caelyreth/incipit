package settings

import (
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The settings stand in the field as cards (field-and-desk.md §4.4.1):
// each section of a category is one card, found by the words written with
// $settings. Sections lists the sections words find, in order, and
// SectionView draws one, as wide as the card it is drawn in. What words
// find in a section, as one option of a policy or one shortcut of a
// group, is what it shows of it.

// Section is one section of the settings, as a card shows it.
type Section struct {
	Category    appsettings.Category
	Title       string
	Description string
}

// ID names the section among all the settings'.
func (sec Section) ID() string { return sec.Category.Title() + "/" + sec.Title }

// Sections are the sections the words find, in the order the categories
// and their sections stand: all of them for no words.
func (s *State) Sections(words string) []Section {
	query := fold(words)
	var out []Section
	add := func(category appsettings.Category, title, description string, matches bool) {
		if matches {
			out = append(out, Section{Category: category, Title: title, Description: description})
		}
	}
	for _, sec := range s.appearanceSections() {
		add(appsettings.Appearance, sec.title, sec.description, query == "" || containsFold(sec.words, query))
	}
	for _, card := range s.Data.Policies {
		add(appsettings.Editor, card.Title, s.spell(card.Description), query == "" || s.policyMatches(card, query))
	}
	groups := s.shortcutGroups(query)
	add(appsettings.Keymap, findShortcut, "Find an action by pressing its keys, or put every key back as it was.",
		query == "" || len(groups) > 0 || containsFold(findShortcut+" keymap keys shortcuts reset", query))
	for _, group := range groups {
		add(appsettings.Keymap, group.title, group.description, true)
	}
	return out
}

// SectionView draws the section id as the words find it: its category
// and title over its rows, or nothing where the words no longer find it.
func (s *State) SectionView(c *ui.Context, semantic theme.Theme, id, words string) {
	// A recording begun or ended while building, as a capture that lost
	// the keyboard, is told before the frame ends.
	defer s.reportRecording()
	s.Query = words
	query := fold(words)
	for _, sec := range s.Sections(words) {
		if sec.ID() != id {
			continue
		}
		scale := s.Theme.UIFontSize
		ui.Column(c).Key("settings-section-" + id).WidthPercent(100).Gap(theme.Space08.DIPs(scale)).Children(func() {
			components.Label(c, sec.Category.Title(), theme.TextXSmall, semantic.Text.Muted).
				PaddingX(theme.Space04.DIPs(scale))
			switch sec.Category {
			case appsettings.Appearance:
				s.appearanceSection(c, semantic, sec)
			case appsettings.Editor:
				s.editorSection(c, semantic, sec.Title, query)
			case appsettings.Keymap:
				s.keymapSection(c, semantic, sec, query)
			}
		})
		return
	}
}

// section draws a section's title and description over a card of its rows.
func (s *State) section(c *ui.Context, semantic theme.Theme, sec Section, rows func()) {
	components.SectionHeader{Label: sec.Title, Description: sec.Description}.View(c, semantic)
	components.SettingGroup{
		Key: "settings-group-" + sec.Title, Label: sec.Title, Children: rows,
	}.View(c, semantic)
}

// fold is words as a search matches them.
func fold(words string) string { return strings.ToLower(strings.TrimSpace(words)) }

func containsFold(text, query string) bool {
	return strings.Contains(strings.ToLower(text), query)
}
