package settings

import (
	"sort"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

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
	data.Shortcuts = buildShortcuts(catalog, source, prepared, active)
	data.Sections = buildSections(catalog, source, prepared, active, data.Shortcuts)
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
	shortcuts []Shortcut,
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
				Label:       "Enable " + lowerPolicyLabel(info.label),
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
		inline := make(map[kernel.CommandID]bool)
		for _, text := range sectionTexts(info.description, toggles) {
			for _, command := range References(text) {
				inline[command] = true
			}
		}
		own := make([]Shortcut, 0)
		for _, shortcut := range shortcuts {
			if shortcut.Policy == policy.ID && !inline[shortcut.Command] {
				own = append(own, shortcut)
			}
		}
		sections = append(sections, Section{
			Policy:      policy.ID,
			Title:       info.label,
			Description: info.description,
			Settings:    toggles,
			Shortcuts:   own,
		})
	}
	return sections
}

func sectionTexts(description string, toggles []ToggleSetting) []string {
	texts := []string{description}
	for _, toggle := range toggles {
		texts = append(texts, toggle.Label, toggle.Description)
	}
	return texts
}

// buildShortcuts lists every command by what it acts on, in scope order.
// Within a scope the built-in actions come first, then each switchable
// feature's, in policy order. A command is about the policy it serves even
// when another owns it (the CJK toggle, owned by the workspace so it works
// while CJK is off), and that policy is its origin when it can be switched.
func buildShortcuts(
	catalog kernel.PolicyCatalog,
	source kernel.PolicyConfiguration,
	prepared *kernel.PreparedPolicyConfiguration,
	available bool,
) []Shortcut {
	if prepared == nil {
		return []Shortcut{}
	}
	policies := sortedPolicies(catalog)
	rank := make(map[kernel.PolicyID]int, len(policies))
	for index, policy := range policies {
		rank[policy.ID] = index
	}
	entries := make([]Shortcut, 0, 64)
	for _, policy := range policies {
		for _, command := range policy.Commands {
			meta, known := editor.CommandFor(command)
			subject, scope := policy.ID, editor.ScopeDocument
			if known {
				scope = meta.Scope
				if _, ok := catalog.Policy(meta.About()); ok {
					subject = meta.About()
				}
			}
			entry := Shortcut{
				Command:  command,
				Label:    commandLabel(command),
				Scope:    scope,
				Group:    ScopeTitle(scope),
				Policy:   subject,
				Bindings: prepared.BindingsFor(command),
				Enabled:  available && prepared.IsCommandEnabled(command),
			}
			_, entry.Overridden = source.BindingOverride(command)
			if info := infoForPolicy(subject); info.toggleable {
				entry.Origin, entry.OriginLabel = subject, info.label
			}
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		if (left.Origin == "") != (right.Origin == "") {
			return left.Origin == ""
		}
		return left.Origin != "" && rank[left.Origin] < rank[right.Origin]
	})
	return entries
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

func lowerPolicyLabel(value string) string {
	runes := []rune(value)
	if len(runes) >= 2 && runes[0] == 'C' && runes[1] == 'J' && len(runes) >= 3 && runes[2] == 'K' {
		return value
	}
	return lowerFirst(value)
}
