package settings

import (
	"strings"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
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
	Category appsettings.Category
	// Key names the section in its category in every language, as Title
	// does in the one the app shows.
	Key         string
	Title       string
	Description string
}

// ID names the section among all the settings'.
func (sec Section) ID() string { return sec.Category.Key() + "/" + sec.Key }

// Sections are the sections the words find, in the order the categories
// and their sections stand: all of them for no words.
func (s *State) Sections(words string) []Section {
	query := fold(words)
	var out []Section
	for _, category := range appsettings.AllCategories {
		out = s.appendSections(out, category, query, "")
	}
	return out
}

// appendSections appends to out category's sections the folded query
// finds, in their order: every one, or the one keyed only, which spares a
// section drawn alone finding all the others first.
func (s *State) appendSections(out []Section, category appsettings.Category, query, only string) []Section {
	add := func(key, title, description string, matches func() bool) {
		if (only == "" || key == only) && matches() {
			out = append(out, Section{Category: category, Key: key, Title: title, Description: description})
		}
	}
	switch category {
	case appsettings.Appearance:
		for _, sec := range s.appearanceSections() {
			add(sec.key, sec.title, sec.description, func() bool { return query == "" || containsFold(sec.words, query) })
		}
	case appsettings.Editor:
		for _, card := range s.Data.Policies {
			if only == "" || card.ID == only {
				add(card.ID, card.Title, s.spell(card.Description), func() bool { return query == "" || s.policyMatches(card, query) })
			}
		}
	case appsettings.Updates:
		s.updatesSections(add, only, query)
	case appsettings.Keymap:
		if len(s.customized()) > 0 && s.Key == "" {
			add(changedKey, locales.ResetKeys(), locales.ResetKeysDescription(), func() bool {
				return query == "" || containsFold(locales.ResetKeys()+" "+locales.ResetKeysKeywords(), query)
			})
		}
		for _, group := range s.shortcutGroups(query, only) {
			add(group.key, group.title, group.description, func() bool { return true })
		}
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
	for _, category := range appsettings.AllCategories {
		key, ok := strings.CutPrefix(id, category.Key()+"/")
		if !ok {
			continue
		}
		for _, sec := range s.appendSections(nil, category, query, key) {
			scale := s.Theme.UIFontSize
			ui.Column(c).Key("settings-section-" + id).WidthPercent(100).Gap(theme.Space08.DIPs(scale)).Children(func() {
				components.Label(c, sec.Category.Title(), theme.TextXSmall, semantic.Text.Muted).
					PaddingX(theme.Space04.DIPs(scale))
				switch sec.Category {
				case appsettings.Appearance:
					s.appearanceSection(c, semantic, sec)
				case appsettings.Editor:
					s.editorSection(c, semantic, sec.Key, query)
				case appsettings.Updates:
					s.updatesSection(c, semantic, sec.Key, query)
				case appsettings.Keymap:
					s.keymapSection(c, semantic, sec, query)
				}
			})
		}
		return
	}
}

// section draws a section's title and description over a card of its rows.
func (s *State) section(c *ui.Context, semantic theme.Theme, sec Section, rows func()) {
	components.SectionHeader{Label: sec.Title, Description: sec.Description}.View(c, semantic)
	components.SettingGroup{
		Key: "settings-group-" + sec.Key, Label: sec.Title, Children: rows,
	}.View(c, semantic)
}

// fold is words as a search matches them.
func fold(words string) string { return strings.ToLower(strings.TrimSpace(words)) }

func containsFold(text, query string) bool {
	return strings.Contains(strings.ToLower(text), query)
}
