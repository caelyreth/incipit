package settings

import (
	"fmt"
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	appshowcase "github.com/caelyreth/neverland/app/showcase"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/showcase"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// editorPage draws one card per policy. A policy is a master switch with
// tweaks under it, so the card's first line is the policy and its switch;
// its story plays beneath, and the tweaks nest under that, each with the
// keys it answers to, folding away while the policy is off.
func (s *State) editorPage(c *ui.Context, semantic theme.Theme) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	for _, card := range s.Data.Policies {
		if query != "" && !s.policyMatches(card, query) {
			continue
		}
		options, shortcuts := card.Options, card.Shortcuts
		if query != "" && !containsFold(card.Title+" "+s.spellWords(card.Description), query) {
			options = s.matchingOptions(options, query)
			shortcuts = matchingShortcuts(shortcuts, query)
		}
		s.policyCard(c, semantic, card, options, shortcuts, query != "")
	}
}

func (s *State) policyMatches(card appsettings.Policy, query string) bool {
	if containsFold(card.Title+" "+s.spellWords(card.Description), query) {
		return true
	}
	return len(s.matchingOptions(card.Options, query))+len(matchingShortcuts(card.Shortcuts, query)) > 0
}

func (s *State) matchingOptions(options []appsettings.Option, query string) []appsettings.Option {
	matches := make([]appsettings.Option, 0, len(options))
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
	card appsettings.Policy,
	options []appsettings.Option,
	shortcuts []appsettings.Shortcut,
	searching bool,
) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	s.anchored(c, card.Title, func() {
		components.SettingGroup{
			Key: "settings-group-" + card.Title, Label: card.Title,
			Children: func() {
				s.policyHeader(c, semantic, card, len(options)+len(shortcuts))
				s.policyShowcase(c, semantic, card)
				if len(options)+len(shortcuts) == 0 {
					return
				}
				s.policyOptions(c, semantic, card.Title, options, shortcuts, card.Enabled || searching, animate, scale)
			},
		}.View(c, semantic)
	})
}

// policyHeader is the card's first line: the policy and its master switch.
// Its description names the policy's keys live; one pressed records there,
// under the header, which stays open while the policy is off.
func (s *State) policyHeader(c *ui.Context, semantic theme.Theme, card appsettings.Policy, optionCount int) {
	scale := s.Theme.UIFontSize
	inset := theme.Space16.DIPs(scale)
	host := "policy-" + card.ID
	defer s.recorder(c, semantic, host, inset)
	ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).
		Gap(theme.Space16.DIPs(scale)).
		Padding(theme.Space16.DIPs(scale), inset, theme.Space16.DIPs(scale), inset).
		Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
				components.Label(c, card.Title, theme.TextDefault, semantic.Text.Heading).
					FontWeight(600)
				if card.Description != "" {
					s.phrase(c, semantic, host, card.Description, theme.TextSmall, semantic.Text.Muted)
				}
				if !card.Enabled && optionCount > 0 {
					noun := "options"
					if optionCount == 1 {
						noun = "option"
					}
					components.Label(c, fmt.Sprintf("Turn on to adjust %d %s.", optionCount, noun),
						theme.TextXSmall, semantic.Text.Muted)
				}
			})
			if card.Switch == "" {
				return
			}
			components.Switch{
				Key: "policy-switch-" + card.ID, Label: card.Title,
				Selected: card.Enabled, Animate: s.motion(c),
				Action: func(value bool) { s.apply(appsettings.SetOption(card.Switch, value)) },
			}.View(c, semantic)
		})
}

// policyShowcase opens the card with the policy's story on a small page:
// the engine typing under the person's own options, centered as it grows.
// A policy that is off still shows its story, dimmed, as what turning it
// on would do.
func (s *State) policyShowcase(c *ui.Context, semantic theme.Theme, card appsettings.Policy) {
	script, ok := appshowcase.For(card.ID)
	if !ok {
		return
	}
	scale := s.Theme.UIFontSize
	inset := theme.Space16.DIPs(scale)
	ui.Row(c).WidthPercent(100).Padding(0, inset, inset, inset).Children(func() {
		showcase.Stage{
			Key:     "showcase-" + card.ID,
			Script:  script,
			Policy:  card.ID,
			Values:  s.Data.PolicyValues,
			Theme:   semantic,
			Prose:   theme.NewProseStyle(s.Theme.ProseFontSize * 0.9).WithSpacing(s.Theme.LineSpacing),
			Height:  132 * scale / theme.DefaultUIFontSize,
			Padding: theme.Space24.DIPs(scale),
			Radius:  10 * scale / theme.DefaultUIFontSize,
			Off:     !card.Enabled,
			Animate: s.motion(c),
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
	options []appsettings.Option,
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
			host := "option-" + option.Key
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
				Footer:  func() { s.recorder(c, semantic, host, theme.Space16.DIPs(scale)+indent) },
				Control: func() { s.optionControl(c, semantic, option, label, animate) },
			}.View(c, semantic)
		}
		for index, shortcut := range shortcuts {
			s.shortcutRow(c, semantic, shortcut, index == len(shortcuts)-1, indent)
		}
	})
}

// optionControl is an option's control by its kind: a switch, a choice of
// segments or a stepped number.
func (s *State) optionControl(c *ui.Context, semantic theme.Theme, option appsettings.Option, label string, animate bool) {
	key := "option-control-" + option.Key
	switch option.Kind {
	case policy.OptionBool:
		on, _ := option.Value.(bool)
		components.Switch{
			Key: key, Label: label, Selected: on, Disabled: !option.Applies, Animate: animate,
			Action: func(value bool) { s.apply(appsettings.SetOption(option.Key, value)) },
		}.View(c, semantic)
	case policy.OptionEnum:
		segments := make([]components.SegmentOption, 0, len(option.Choices))
		for _, choice := range option.Choices {
			segments = append(segments, components.SegmentOption{
				Key: fmt.Sprint(key, "-", choice.Value), Label: choice.Label, Selected: choice.Value == option.Value,
				Action: func() { s.apply(appsettings.SetOption(option.Key, choice.Value)) },
			})
		}
		components.Segmented{Key: key, Label: label, Animate: animate, Options: segments}.View(c, semantic)
	case policy.OptionInt:
		value, _ := option.Value.(int)
		step := max(1, (option.Max-option.Min)/100)
		components.NumberField{
			Key: key, Label: label, Value: fmt.Sprint(value), Animate: animate,
			CanDecrease: value > option.Min, CanIncrease: value < option.Max,
			Decrease: func() { s.apply(appsettings.SetOption(option.Key, max(option.Min, value-step))) },
			Increase: func() { s.apply(appsettings.SetOption(option.Key, min(option.Max, value+step))) },
		}.View(c, semantic)
	}
}
