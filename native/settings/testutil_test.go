package settings

import (
	"slices"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

// cards draws the sections the query finds, of the categories given or of
// all, one under another as wide as the field's cards, as the field shows
// them.
func (s *State) cards(categories ...appsettings.Category) func(*ui.Context) {
	return func(c *ui.Context) {
		c.SetTheme(s.Theme.MyGoTheme())
		semantic := s.Theme.Semantic()
		ui.Column(c).Key("cards").Fill().Background(semantic.Surface.Background).Padding(16, 16, 16, 16).Gap(24).Children(func() {
			for _, sec := range s.Sections(s.Query) {
				if len(categories) > 0 && !slices.Contains(categories, sec.Category) {
					continue
				}
				ui.Column(c).Key(sec.ID()).Width(560).Shrink(0).Children(func() {
					s.SectionView(c, semantic, sec.ID(), s.Query)
				})
			}
		})
	}
}
