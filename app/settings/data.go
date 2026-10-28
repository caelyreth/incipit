package settings

import "github.com/caelyreth/neverland/engine/kernel"

// BuildData creates a fully owned declarative settings snapshot. It reads policy
// shape from the session's catalog, active values from the app-wide source and
// prepared environment, and page diagnostics from the owned session snapshot.
func BuildData(
	store Store,
	session Session,
	presentation PresentationState,
	appearance AppearanceSettings,
) Data {
	appearance = appearance.Normalized()
	data := Data{
		AppearanceValues: appearance,
		Appearance:       appearanceToggles(presentation),
	}
	if store == nil || session == nil {
		data.Activity = []string{}
		data.ActivityEntries = []ActivityEntry{}
		data.Scopes = []string{}
		return data
	}
	source := store.Configuration()
	prepared := store.PreparedConfiguration()
	catalog := store.PolicyCatalog()
	page, title, active := session.Page()
	data.Page = title
	if active {
		data.PageID = &page
	}
	data.Sections = buildSections(catalog, source, prepared, active)
	data.Shortcuts = buildShortcuts(catalog, source, prepared, active)
	entries := session.Activity()
	data.ActivityEntries = append([]ActivityEntry(nil), entries...)
	data.Activity = activityLines(entries)
	data.Scopes = append([]string{}, session.Scopes()...)
	data.PolicyConfiguration = source.Clone()
	return data
}

func buildSections(
	catalog kernel.PolicyCatalog,
	source kernel.PolicyConfiguration,
	prepared *kernel.PreparedPolicyConfiguration,
	available bool,
) []Section {
	if prepared == nil {
		return []Section{}
	}
	sections := make([]Section, 0)
	for _, policy := range sortedPolicies(catalog) {
		info := infoForPolicy(policy.ID)
		enabled := source.IsEnabled(policy.ID)
		toggles := make([]ToggleSetting, 0, len(policy.Settings)+1)
		if info.toggleable {
			toggles = append(toggles, ToggleSetting{
				Key:         PolicyBoolean(policy.ID),
				Label:       "Enable " + lowerFirst(info.label),
				Description: info.description,
				Value:       enabled,
				Enabled:     available,
				Selector:    "policy-" + string(policy.ID),
			})
		}
		for _, setting := range policy.Settings {
			key := kernel.NewSettingKey(policy.ID, setting.ID)
			value, ok := prepared.Setting(key)
			enabledValue, boolean := value.Bool()
			if !ok || !boolean {
				continue
			}
			toggles = append(toggles, ToggleSetting{
				Key:         OptionBoolean(key),
				Label:       labelForSetting(setting.ID),
				Description: descriptionForSetting(setting.ID),
				Value:       enabledValue,
				Enabled:     available && enabled,
				Selector:    "option-" + string(policy.ID) + "-" + string(setting.ID),
			})
		}
		if len(toggles) == 0 {
			continue
		}
		sections = append(sections, Section{
			Title:       info.label,
			Description: info.description,
			Settings:    toggles,
		})
	}
	return sections
}

func buildShortcuts(
	catalog kernel.PolicyCatalog,
	source kernel.PolicyConfiguration,
	prepared *kernel.PreparedPolicyConfiguration,
	available bool,
) []Shortcut {
	if prepared == nil {
		return []Shortcut{}
	}
	shortcuts := make([]Shortcut, 0)
	for _, policy := range sortedPolicies(catalog) {
		group := infoForPolicy(policy.ID).label
		for _, command := range policy.Commands {
			_, overridden := source.BindingOverride(command)
			shortcuts = append(shortcuts, Shortcut{
				Command:    command,
				Label:      commandLabel(command),
				Group:      group,
				Bindings:   prepared.BindingsFor(command),
				Enabled:    available && prepared.IsCommandEnabled(command),
				Overridden: overridden,
			})
		}
	}
	return shortcuts
}

func appearanceToggles(presentation PresentationState) []ToggleSetting {
	settings := []struct {
		mode        Presentation
		label       string
		description string
	}{
		{
			PresentationMotion,
			"Animations",
			"Smooth caret, selection and viewport motion. System reduced motion takes precedence.",
		},
		{
			PresentationTypewriter,
			"Typewriter mode",
			"Keep the writing line near the center of the page.",
		},
		{
			PresentationZen,
			"Zen mode",
			"Hide the surrounding notebook and window chrome.",
		},
	}
	result := make([]ToggleSetting, 0, len(settings))
	for _, setting := range settings {
		result = append(result, ToggleSetting{
			Key:         PresentationBoolean(setting.mode),
			Label:       setting.label,
			Description: setting.description,
			Value:       presentation.Get(setting.mode),
			Enabled:     true,
			Selector:    "appearance-" + presentationSelector(setting.mode),
		})
	}
	return result
}

func presentationSelector(mode Presentation) string {
	switch mode {
	case PresentationMotion:
		return "motion"
	case PresentationTypewriter:
		return "typewriter"
	case PresentationZen:
		return "zen"
	default:
		return "unknown"
	}
}

func activityLines(entries []ActivityEntry) []string {
	lines := make([]string, 0, len(entries))
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		lines = append(lines, formatActivity(entry))
	}
	return lines
}

func formatActivity(entry ActivityEntry) string {
	return "Revision " + uintString(entry.Revision) +
		"  ·  " + intString(entry.Characters) + " characters" +
		"  ·  " + intString(entry.Marks) + " marks"
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	bytes := []byte(value)
	if bytes[0] >= 'A' && bytes[0] <= 'Z' {
		bytes[0] += 'a' - 'A'
	}
	return string(bytes)
}
