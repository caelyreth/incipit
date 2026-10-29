package settings

import (
	"fmt"
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	appshowcase "github.com/caelyreth/neverland/app/showcase"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/showcase"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// editorPage draws one card per policy. A policy is a master switch with
// tweaks under it, so the card's header is the policy and its switch, and
// the tweaks nest beneath, folding away while the policy is off.
func (s *State) editorPage(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	for _, section := range s.Data.Sections {
		master, options := policyParts(section)
		if query != "" && !s.sectionMatches(section, query) {
			continue
		}
		shortcuts := section.Shortcuts
		if query != "" && !containsFold(section.Title+" "+s.spellWords(section.Description), query) {
			options = s.matchingOptions(options, query)
			shortcuts = matchingShortcuts(shortcuts, query)
		}
		s.policyCard(c, semantic, section, master, options, shortcuts, query != "")
	}
	_ = metrics
}

// policyParts splits a section into its policy's enable switch, when the
// policy can be turned off, and the options that refine it.
func policyParts(section appsettings.Section) (*appsettings.ToggleSetting, []appsettings.ToggleSetting) {
	var master *appsettings.ToggleSetting
	options := make([]appsettings.ToggleSetting, 0, len(section.Settings))
	for index := range section.Settings {
		setting := section.Settings[index]
		if master == nil && setting.Key.Kind == appsettings.BooleanPolicy {
			master = &section.Settings[index]
			continue
		}
		options = append(options, setting)
	}
	return master, options
}

func (s *State) matchingOptions(options []appsettings.ToggleSetting, query string) []appsettings.ToggleSetting {
	matches := make([]appsettings.ToggleSetting, 0, len(options))
	for _, option := range options {
		if containsFold(s.spellWords(option.Label+" "+option.Description), query) {
			matches = append(matches, option)
		}
	}
	return matches
}

func matchingShortcuts(shortcuts []appsettings.Shortcut, query string) []appsettings.Shortcut {
	matches := make([]appsettings.Shortcut, 0, len(shortcuts))
	for _, shortcut := range shortcuts {
		if containsFold(shortcut.Label, query) {
			matches = append(matches, shortcut)
		}
	}
	return matches
}

func containsFold(text, query string) bool {
	return strings.Contains(strings.ToLower(text), query)
}

func (s *State) policyCard(
	c *ui.Context,
	semantic theme.Theme,
	section appsettings.Section,
	master *appsettings.ToggleSetting,
	options []appsettings.ToggleSetting,
	shortcuts []appsettings.Shortcut,
	searching bool,
) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	on := master == nil || master.Value
	s.anchored(c, section.Title, func() {
		components.SettingGroup{
			Key: "settings-group-" + section.Title, Label: section.Title,
			Children: func() {
				s.policyShowcase(c, semantic, section, on)
				s.policyHeader(c, semantic, section, master, len(options)+len(shortcuts), on)
				if len(options)+len(shortcuts) == 0 {
					return
				}
				s.policyOptions(c, semantic, section.Title, options, shortcuts, on || searching, animate, scale)
			},
		}.View(c, semantic)
	})
}

// policyHeader is the card's first row: the policy and its master switch.
// Its description names the policy's keys live; one pressed records there,
// under the header, which stays open while the policy is off.
func (s *State) policyHeader(
	c *ui.Context,
	semantic theme.Theme,
	section appsettings.Section,
	master *appsettings.ToggleSetting,
	optionCount int,
	on bool,
) {
	scale := s.Theme.UIFontSize
	inset := theme.Space16.DIPs(scale)
	host := "policy-" + string(section.Policy)
	defer s.recorder(c, semantic, host, inset)
	ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).
		Gap(theme.Space16.DIPs(scale)).
		Padding(theme.Space16.DIPs(scale), inset, theme.Space16.DIPs(scale), inset).
		Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
				components.Label(c, section.Title, theme.TextDefault, semantic.Text.Heading).
					FontWeight(600)
				if section.Description != "" {
					s.phrase(c, semantic, host, section.Description, theme.TextSmall, semantic.Text.Muted)
				}
				if !on && optionCount > 0 {
					noun := "options"
					if optionCount == 1 {
						noun = "option"
					}
					components.Label(c, fmt.Sprintf("Turn on to adjust %d %s.", optionCount, noun),
						theme.TextXSmall, semantic.Text.Muted)
				}
			})
			if master == nil {
				return
			}
			setting := *master
			components.Switch{
				Key: setting.Selector, Label: s.spell(setting.Label),
				Selected: setting.Value, Disabled: !setting.Enabled,
				Animate: s.motion(c),
				Action: func(value bool) {
					s.apply(appsettings.ToggleBoolean(setting.Key, value))
				},
			}.View(c, semantic)
		})
}

// policyShowcase opens the card with the policy's story on a small page:
// the engine typing under the person's own options, centered as it grows.
// A policy that is off still shows its story, dimmed, as what turning it
// on would do.
func (s *State) policyShowcase(
	c *ui.Context,
	semantic theme.Theme,
	section appsettings.Section,
	on bool,
) {
	script, ok := appshowcase.For(section.Policy)
	if !ok {
		return
	}
	scale := s.Theme.UIFontSize
	inset := theme.Space16.DIPs(scale)
	ui.Row(c).WidthPercent(100).Padding(inset, inset, 0, inset).Children(func() {
		showcase.Stage{
			Key:           "showcase-" + string(section.Policy),
			Script:        script,
			Configuration: appshowcase.Configuration(section.Policy, s.Data.PolicyConfiguration),
			Theme:         semantic,
			Prose:         theme.NewProseStyle(s.Theme.ProseFontSize * 0.9),
			Height:        132 * scale / theme.DefaultUIFontSize,
			Padding:       theme.Space24.DIPs(scale),
			Radius:        10 * scale / theme.DefaultUIFontSize,
			Off:           !on,
			Animate:       s.motion(c),
		}.View(c)
	})
}

// policyOptions nests the tweaks under the header, then the actions the
// policy answers to, each the Keymap's own row, so a shortcut recorded here
// is checked and stored exactly as one recorded there. They fold away while
// the policy is off: the fold animates the measured height and fades with it.
func (s *State) policyOptions(
	c *ui.Context,
	semantic theme.Theme,
	title string,
	options []appsettings.ToggleSetting,
	shortcuts []appsettings.Shortcut,
	open bool,
	animate bool,
	scale float32,
) {
	indent := theme.Space16.DIPs(scale)
	components.Fold{Key: "settings-options-" + title, Open: open, Animate: animate}.View(c, func() {
		ui.Row(c).WidthPercent(100).PaddingStart(theme.Space16.DIPs(scale)).Children(func() {
			components.HorizontalDivider(c, semantic, components.VariantDivider)
		})
		for index, option := range options {
			option := option
			host := "option-" + option.Selector
			label := s.spell(option.Label)
			components.SettingRow{
				Label: label, Description: s.spell(option.Description),
				Last: index == len(options)-1 && len(shortcuts) == 0, Indent: indent,
				LabelText: func() {
					s.phrase(c, semantic, host, option.Label, theme.TextDefault, semantic.Text.Default)
				},
				DescriptionText: func() {
					if option.Description != "" {
						s.phrase(c, semantic, host, option.Description, theme.TextSmall, semantic.Text.Muted)
					}
				},
				Footer: func() { s.recorder(c, semantic, host, theme.Space16.DIPs(scale)+indent) },
				Control: func() {
					components.Switch{
						Key: option.Selector, Label: label,
						Selected: option.Value, Disabled: !option.Enabled,
						Animate: animate,
						Action: func(value bool) {
							s.apply(appsettings.ToggleBoolean(option.Key, value))
						},
					}.View(c, semantic)
				},
			}.View(c, semantic)
		}
		for index, shortcut := range shortcuts {
			s.shortcutRow(c, semantic, shortcut, index == len(shortcuts)-1, indent)
		}
	})
}
