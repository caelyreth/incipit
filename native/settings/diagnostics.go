package settings

import (
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) diagnosticsPage(c *ui.Context, semantic theme.Theme) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	sections := []struct {
		Title       string
		Description string
		Entries     []string
		Empty       string
	}{
		{"Activity", "Recent published edits in the current page.", s.Data.Activity, "No published changes"},
		{"Scopes", "Selection, navigation offers and semantic ranges.", s.Data.Scopes, "No active scopes"},
	}
	for _, section := range sections {
		if query != "" && !strings.Contains(strings.ToLower(section.Title+" "+section.Description), query) {
			continue
		}
		entries := section.Entries
		if len(entries) == 0 {
			entries = []string{section.Empty}
		}
		s.section(c, semantic, section.Title, section.Description, func() {
			for index, entry := range entries {
				components.SettingRow{
					Label: entry, Last: index == len(entries)-1,
				}.View(c, semantic)
			}
		})
	}
}
