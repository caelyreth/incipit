package settings

import (
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) editorPage(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	for _, section := range s.Data.Sections {
		matches := make([]appsettings.ToggleSetting, 0)
		for _, setting := range section.Settings {
			haystack := strings.ToLower(setting.Label + " " + setting.Description)
			if query == "" || strings.Contains(haystack, query) {
				matches = append(matches, setting)
			}
		}
		if len(matches) == 0 {
			continue
		}
		components.SectionHeader{
			Label: section.Title, Description: section.Description,
		}.View(c, semantic)
		for index, setting := range matches {
			setting := setting
			components.SettingRow{
				Label: setting.Label, Description: setting.Description,
				Last: index == len(matches)-1,
				Control: func() {
					components.Switch{
						Key: setting.Selector, Label: setting.Label,
						Selected: setting.Value, Disabled: !setting.Enabled,
						Animate: s.motion(c),
						Action: func(value bool) {
							s.apply(appsettings.ToggleBoolean(setting.Key, value))
						},
					}.View(c, semantic)
				},
			}.View(c, semantic)
		}
	}
	_ = metrics
}
