// Package lab is the design lab: a window beside a sample notebook in
// which candidates for the shell's look are set on the running app, to be
// judged there in both appearances before one becomes the app's own. It
// holds the candidates and the instruments that read them; the app's
// menus never open it (cmd/neverland-lab does).
package lab

import (
	"fmt"
	"math"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Choice is what the lab has set on the app.
type Choice struct {
	// Scheme indexes Schemes.
	Scheme     int
	Appearance theme.Appearance
	// Accent is the one accent the window wears; with FollowGroup the
	// window wears the open sheet's group's instead, as the app does.
	Accent      theme.Accent
	FollowGroup bool
}

// Accents are the accents the lab offers: the two the remake weighs.
var Accents = [...]theme.Accent{theme.Halo, theme.Unripe}

// Lab is the lab window's state. The app owns what it sets: Apply hands
// it a choice, and Settings and Colors read back what the app paints.
type Lab struct {
	Choice   Choice
	Apply    func(Choice)
	Settings func() theme.Settings
	Colors   *theme.Transition
}

func (l *Lab) choose(change func(*Choice)) {
	change(&l.Choice)
	if l.Apply != nil {
		l.Apply(l.Choice)
	}
}

// View draws the lab in the app's colors as they are now.
func (l *Lab) View(c *ui.Context) {
	settings := theme.DefaultSettings()
	if l.Settings != nil {
		settings = l.Settings()
	}
	semantic, changing := l.Colors.At(settings, c.Now())
	c.SetTheme(settings.MyGoThemeFor(semantic))
	if changing {
		c.AnimationFrame()
	}
	scale := settings.UIFontSize
	gap := theme.Space24.DIPs(scale)
	ui.Column(c).Fill().Background(semantic.Surface.Background).Children(func() {
		// The title bar's height is the window's to drag.
		ui.Box(c).Height(theme.DefaultLayout.Titlebar * scale / theme.DefaultUIFontSize).DragWindow()
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).WidthPercent(100).Gap(gap).
				Padding(0, gap, gap, gap).Children(func() {
				ui.Column(c).Gap(theme.Space04.DIPs(scale)).Children(func() {
					components.Headline(c, "Lab", theme.HeadlineMedium, semantic.Text.Heading).FontWeight(600)
					components.Label(c, "Candidates set on the sample notebook beside this window. Choose here, and judge them there.",
						theme.TextSmall, semantic.Text.Muted)
				})
				l.choices(c, semantic)
				l.ladder(c, semantic, settings)
				l.grounds(c, semantic, settings)
			})
		})
	})
}

func (l *Lab) choices(c *ui.Context, semantic theme.Theme) {
	ui.Column(c).Gap(theme.Space08.DIPs(components.UIFontSize(c))).Children(func() {
		components.SectionHeader{Label: "Set on the app",
			Description: "The scheme, the appearance and the accent the sample notebook paints with."}.View(c, semantic)
		components.SettingGroup{Key: "choices", Label: "Set on the app", Children: func() {
			components.SettingRow{Label: "Scheme",
				Description: "Cosmofi as the app paints today, or a candidate beside it.",
				Control: func() {
					var options []components.SegmentOption
					for i, scheme := range Schemes() {
						options = append(options, components.SegmentOption{
							Key: scheme.Name, Label: scheme.Name, Selected: l.Choice.Scheme == i,
							Action: func() { l.choose(func(ch *Choice) { ch.Scheme = i }) },
						})
					}
					components.Segmented{Key: "scheme", Label: "Scheme", Options: options}.View(c, semantic)
				}}.View(c, semantic)
			components.SettingRow{Label: "Appearance", Control: func() {
				components.Segmented{Key: "appearance", Label: "Appearance", Options: []components.SegmentOption{
					{Key: "starfield", Label: "Starfield", Selected: l.Choice.Appearance != theme.Daybreak,
						Action: func() { l.choose(func(ch *Choice) { ch.Appearance = theme.Starfield }) }},
					{Key: "daybreak", Label: "Daybreak", Selected: l.Choice.Appearance == theme.Daybreak,
						Action: func() { l.choose(func(ch *Choice) { ch.Appearance = theme.Daybreak }) }},
				}}.View(c, semantic)
			}}.View(c, semantic)
			components.SettingRow{Label: "Accent",
				Description: "The one accent of the color budget.",
				Control: func() {
					var options []components.SegmentOption
					for _, accent := range Accents {
						options = append(options, components.SegmentOption{
							Key: accent.Label(), Label: accent.Label(),
							Selected: !l.Choice.FollowGroup && l.Choice.Accent == accent,
							Action: func() {
								l.choose(func(ch *Choice) { ch.Accent, ch.FollowGroup = accent, false })
							},
						})
					}
					components.Segmented{Key: "accent", Label: "Accent", Options: options}.View(c, semantic)
				}}.View(c, semantic)
			components.SettingRow{Label: "Follow the open sheet's group",
				Description: "The window wears the open sheet's group color, as the app does today.",
				Last:        true,
				Control: func() {
					components.Switch{Key: "follow", Label: "Follow the open sheet's group",
						Selected: l.Choice.FollowGroup,
						Action:   func(on bool) { l.choose(func(ch *Choice) { ch.FollowGroup = on }) },
					}.View(c, semantic)
				}}.View(c, semantic)
		}}.View(c, semantic)
	})
}

// Breaks are where a palette's ladder breaks its order: a hover as loud
// as chosen or louder, or by day a page or a raised surface darker than
// the ground it stands on.
func Breaks(p theme.Palette, appearance theme.Appearance) []string {
	ground := Lightness(p.Background)
	delta := func(color ui.Color) float64 { return (Lightness(color) - ground) * 100 }
	var breaks []string
	if math.Abs(delta(p.Hover)) >= math.Abs(delta(p.Chosen)) {
		breaks = append(breaks, "hover is as loud as chosen, or louder")
	}
	if appearance == theme.Daybreak {
		if delta(p.Page) < 0 {
			breaks = append(breaks, "the page is darker than the ground")
		}
		if delta(p.Raised) < 0 {
			breaks = append(breaks, "what is raised is darker than the ground")
		}
	}
	return breaks
}

// rung is one step of the ladder: a material and its lightness over the
// ground's.
type rung struct {
	label string
	color ui.Color
}

// ladder shows the scheme's materials side by side, each with its
// lightness over the ground's, and says where the order breaks: hover at
// or above chosen, or by day a page or a raised surface darker than the
// ground.
func (l *Lab) ladder(c *ui.Context, semantic theme.Theme, settings theme.Settings) {
	scale := settings.UIFontSize
	palette := settings.Palette()
	rungs := []rung{
		{"Ground", palette.Background}, {"Page", palette.Page}, {"Raised", palette.Raised},
		{"Inset", palette.Inset}, {"Hover", palette.Hover}, {"Chosen", palette.Chosen},
		{"Line", palette.Hairline},
	}
	ground := Lightness(palette.Background)
	delta := func(color ui.Color) float64 { return (Lightness(color) - ground) * 100 }
	breaks := Breaks(palette, settings.Appearance)
	ui.Column(c).Gap(theme.Space08.DIPs(scale)).Children(func() {
		components.SectionHeader{Label: "Ladder",
			Description: "Each material's lightness over the ground's, as OKLab ΔL × 100. About 5 reads at a glance."}.View(c, semantic)
		components.SettingGroup{Key: "ladder", Label: "Ladder", Children: func() {
			ui.Row(c).Wrap().Gap(theme.Space08.DIPs(scale)).Padding(theme.Space16.DIPs(scale)).Children(func() {
				for _, r := range rungs {
					ui.Column(c).Key(r.label).Gap(theme.Space04.DIPs(scale)).Children(func() {
						ui.Box(c).Width(72*scale/theme.DefaultUIFontSize).Height(44*scale/theme.DefaultUIFontSize).
							Radius(theme.DefaultControls.Scaled(scale).Radius).
							Background(r.color).Border(1, palette.Hairline)
						components.Label(c, r.label, theme.TextSmall, semantic.Text.Default)
						components.Label(c, fmt.Sprintf("%+.1f", delta(r.color)), theme.TextXSmall, semantic.Text.Muted).
							FontFeatures("tnum")
					})
				}
			})
			note, color := "The order holds: chosen over hover, and nothing meant to stand out sinks below the ground.", semantic.Text.Muted
			if len(breaks) > 0 {
				note, color = "Inverted: "+strings.Join(breaks, "; ")+".", semantic.Status.Danger.Foreground
			}
			ui.Row(c).Padding(0, theme.Space16.DIPs(scale), theme.Space16.DIPs(scale), theme.Space16.DIPs(scale)).Children(func() {
				components.Label(c, note, theme.TextSmall, color).Role(ui.RoleStatus)
			})
		}}.View(c, semantic)
	})
}

// groundWeeks are the weeks the ground specimens stand for, by how much
// of the band they ink.
var groundWeeks = [...]struct {
	label   string
	density float32
}{{"A quiet week", 0.22}, {"An ordinary week", 0.48}, {"A busy week", 0.82}}

// grounds sets the ground at the library's width, as the foot of the pane
// would hold it, in two inks: the neutral line ink, and the accent.
func (l *Lab) grounds(c *ui.Context, semantic theme.Theme, settings theme.Settings) {
	scale := settings.UIFontSize
	palette := settings.Palette()
	width := theme.DefaultLayout.Sidebar * scale / theme.DefaultUIFontSize
	inks := [...]struct {
		label string
		color ui.Color
	}{{"Neutral ink", palette.Track}, {"Accent ink", semantic.Accent.Foreground.Alpha(0.7)}}
	ui.Column(c).Gap(theme.Space08.DIPs(scale)).Children(func() {
		components.SectionHeader{Label: "Ground",
			Description: "The weight at the library's foot: Trash, the notebook's vital signs, and a horizon in dither as dense as the week's writing."}.View(c, semantic)
		components.SettingGroup{Key: "grounds", Label: "Ground", Children: func() {
			ui.Column(c).Gap(theme.Space16.DIPs(scale)).Padding(theme.Space16.DIPs(scale)).Children(func() {
				for _, ink := range inks {
					ui.Column(c).Key(ink.label).Gap(theme.Space08.DIPs(scale)).Children(func() {
						components.Label(c, ink.label, theme.TextSmall, semantic.Text.Muted)
						ui.Row(c).Wrap().Gap(theme.Space16.DIPs(scale)).Children(func() {
							for i, week := range groundWeeks {
								l.groundSpecimen(c, semantic, palette, width, week.label, Ground{
									Density: week.density, Ink: ink.color, Seed: float32(i) * 1.7,
								})
							}
						})
					})
				}
			})
		}}.View(c, semantic)
	})
}

// groundSpecimen is the foot of a library pane at its width: the vital
// signs over the horizon, and Trash on the ground.
func (l *Lab) groundSpecimen(c *ui.Context, semantic theme.Theme, palette theme.Palette, width float32, label string, g Ground) {
	scale := components.UIFontSize(c)
	pad := theme.Space12.DIPs(scale)
	ui.Column(c).Key(label).Gap(theme.Space04.DIPs(scale)).Children(func() {
		ui.Column(c).Width(width).Background(palette.Background).
			Radius(theme.DefaultControls.Scaled(scale).Radius).Border(1, palette.Hairline).Clip().
			Children(func() {
				ui.Box(c).Height(32 * scale / theme.DefaultUIFontSize)
				ui.Row(c).PaddingX(pad).Children(func() {
					components.Label(c, "25 sheets · 26,770 words · +312 today", theme.TextXSmall, semantic.Text.Muted).
						FontFeatures("tnum").SingleLine()
				})
				ui.Box(c).WidthPercent(100).Height(56 * scale / theme.DefaultUIFontSize).Draw(g.Paint)
				ui.Row(c).WidthPercent(100).Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).
					Padding(theme.Space08.DIPs(scale), pad, theme.Space12.DIPs(scale), pad).Children(func() {
					ui.Icon(c, components.IconTrash.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).
						TextColor(semantic.Icon.Muted)
					components.Label(c, "Trash", theme.TextSmall, semantic.Text.Default)
				})
			})
		components.Label(c, fmt.Sprintf("%s · %.0f%% inked at the foot", label, g.Density*100),
			theme.TextXSmall, semantic.Text.Muted)
	})
}
