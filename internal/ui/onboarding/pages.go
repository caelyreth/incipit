package onboarding

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/app/fonts"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/components"
	uisettings "github.com/caelyreth/neverland/internal/ui/settings"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// metrics are the walkthrough's sizes at its UI size.
type metrics struct {
	scale, k float32
	// top clears the window's controls, pad is the margin, gap the room
	// between the page's columns.
	top, pad, gap float32
	// display is the headline's size, the welcome's larger.
	display, hero float32
	radius        float32
}

func newMetrics(c *ui.Context) metrics {
	scale := components.UIFontSize(c)
	k := scale / theme.DefaultUIFontSize
	return metrics{
		scale: scale, k: k,
		top: 44 * k, pad: 48 * k, gap: 56 * k,
		display: 40 * k, hero: 58 * k,
		radius: theme.DefaultControls.Scaled(scale).PanelRadius,
	}
}

// pageView draws the page shown.
func (s *State) pageView(c *ui.Context, semantic theme.Theme, m metrics, animate bool, now time.Time) {
	switch s.at {
	case pageWelcome:
		s.welcome(c, semantic, m, animate, now)
	case pageNotebook:
		s.spread(c, semantic, m, animate, locales.OnboardingNotebookEyebrow(), locales.OnboardingNotebookHeadline(),
			locales.OnboardingNotebookBody(), func() { s.notebook(c, semantic, m, animate) })
	case pageWriting:
		s.spread(c, semantic, m, animate, locales.OnboardingWritingEyebrow(), locales.OnboardingWritingHeadline(),
			locales.OnboardingWritingBody(), func() { s.writing(c, semantic, m, animate) })
	case pageLook:
		s.spread(c, semantic, m, animate, locales.OnboardingLookEyebrow(), locales.OnboardingLookHeadline(),
			locales.OnboardingLookBody(), func() { s.look(c, semantic, m, animate) })
	case pageMac:
		s.spread(c, semantic, m, animate, locales.OnboardingMacEyebrow(), locales.OnboardingMacHeadline(),
			locales.OnboardingMacBody(), func() { s.mac(c, semantic, m, animate) })
	case pageDone:
		s.done(c, semantic, m, animate, now)
	}
}

// welcome is the first page: the headline set large beside the mark,
// which draws itself as a pen would, a few sparks around it.
func (s *State) welcome(c *ui.Context, semantic theme.Theme, m metrics, animate bool, now time.Time) {
	ui.Row(c).WidthPercent(100).Grow(1).AlignItems(ui.Center).Gap(m.gap).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).MaxWidth(520 * m.k).Gap(18 * m.k).Children(func() {
			s.enter(c, s.eyebrow(c, semantic, m, "", locales.OnboardingWelcomeEyebrow()), 0, animate)
			head := ui.Text(c, locales.OnboardingWelcomeHeadline()).Font(theme.ProseFontFamily).FontSize(m.hero).
				FontWeight(500).LineHeight(1.08).LetterSpacing(-0.02 * m.hero).TextColor(semantic.Text.Heading).
				Role(ui.RoleHeading)
			s.enter(c, head, 1, animate)
			drawn := s.inkAfter(now, 900*time.Millisecond, 700*time.Millisecond, animate)
			ink := semantic.Families[theme.Afterglow].Emphasis
			head.DrawOver(func(p *ui.Painter, r ui.Rect) {
				// A stroke under the last line, as a pen sets the title off.
				line := m.hero * 1.08
				y := r.Y + r.H - line*0.06
				components.InkUnderline(p, r.X+r.W*0.02, r.X+min(r.W, m.hero*6.6), y, ink.Alpha(0.85), 3.2*m.k, 7, drawn)
			})
			s.enter(c, ui.Text(c, locales.OnboardingWelcomeBody()).Font(theme.ProseFontFamily).FontSize(18*m.k).
				LineHeight(1.55).TextColor(semantic.Text.Default).MaxWidth(440*m.k), 2, animate)
			s.enter(c, ui.Row(c).Padding(10*m.k, 0, 0, 0).Children(func() {
				components.Button{Key: "begin", Label: locales.OnboardingBegin(), Style: components.PrimaryButton,
					Animate: animate, Action: func() { s.next(s.clock) }}.View(c, semantic).PaddingX(22 * m.k)
			}), 3, animate)
		})
		mark := ui.Box(c).Key("mark").Grow(1).Shrink(1).MinWidth(0).HeightPercent(80).Role(ui.RoleImage).Label(locales.OnboardingTitle())
		drawn := s.inkAfter(now, 250*time.Millisecond, 2600*time.Millisecond, animate)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			size := min(r.W, r.H*components.BrandAspect())
			box := ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size/components.BrandAspect())/2, W: size, H: size / components.BrandAspect()}
			// A moon of the page's own surface behind it, as the icon's tile.
			disc := min(box.W, box.H) * 1.04
			var moon ui.Path
			moon.Circle(box.X+box.W*0.5, box.Y+box.H*0.52, disc/2)
			p.FillPath(&moon, semantic.Surface.Page.Alpha(0.55*components.Smoothstep(drawn*2)))
			components.DrawBrandMark(p, box, semantic.Text.Heading, drawn)
			glow := float32(1)
			if animate {
				t := now.Sub(s.started).Seconds()
				glow = float32(0.5 + 0.5*math.Sin(t*1.7))
				p.AnimationFrameRate(tideFrameRate)
			}
			for i, at := range [][3]float32{{0.06, 0.18, 18}, {0.9, 0.12, 12}, {0.96, 0.62, 16}, {0.12, 0.86, 10}} {
				g := glow
				if i%2 == 1 {
					g = 1 - glow
				}
				components.InkStar(p, box.X+box.W*at[0], box.Y+box.H*at[1], at[2]*m.k, semantic.Families[theme.Regolith].Foreground.Alpha(drawn), g)
			}
		})
	})
}

// inkAfter is how far a mark has drawn itself, starting after wait from
// the page's turn and taking draw.
func (s *State) inkAfter(now time.Time, wait, draw time.Duration, animate bool) float32 {
	if !animate {
		return 1
	}
	return min(1, max(0, float32(now.Sub(s.turned)-wait)/float32(draw)))
}

// spread is a page of a choice: its words on the left, set as a book's
// opening, and what it chooses between on the right.
func (s *State) spread(c *ui.Context, semantic theme.Theme, m metrics, animate bool, eyebrow, headline, body string, choices func()) {
	ui.Row(c).WidthPercent(100).Grow(1).MinHeight(0).AlignItems(ui.Center).Gap(m.gap).Children(func() {
		ui.Column(c).Width(300 * m.k).Shrink(0).Gap(14 * m.k).Children(func() {
			s.enter(c, s.eyebrow(c, semantic, m, fmt.Sprintf("%02d", s.index(s.at)), eyebrow), 0, animate)
			s.enter(c, ui.Text(c, headline).Font(theme.ProseFontFamily).FontSize(m.display).FontWeight(500).
				LineHeight(1.12).LetterSpacing(-0.015*m.display).TextColor(semantic.Text.Heading).Role(ui.RoleHeading), 1, animate)
			s.enter(c, ui.Text(c, body).Font(theme.ProseFontFamily).FontSize(16*m.k).LineHeight(1.6).
				TextColor(semantic.Text.Default), 2, animate)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(22 * m.k).Children(choices)
	})
}

// eyebrow is the small line above a headline: the page's number in the
// serif's italic, then what the page is about, spaced out.
func (s *State) eyebrow(c *ui.Context, semantic theme.Theme, m metrics, number, words string) ui.Element {
	return ui.Row(c).AlignItems(ui.Baseline).Gap(10 * m.k).Children(func() {
		if number != "" {
			ui.Text(c, number).Font(theme.ProseFontFamily).Italic().FontSize(17 * m.k).
				TextColor(semantic.Families[theme.Afterglow].Foreground)
			ui.Box(c).Size(22*m.k, 1).Background(semantic.Text.Muted.Alpha(0.5)).Translate(0, -4*m.k)
		}
		components.Label(c, words, theme.TextSmall, semantic.Text.Muted).LetterSpacing(0.08 * m.scale).FontWeight(600)
	})
}

// question heads one choice of a page.
func (s *State) question(c *ui.Context, semantic theme.Theme, words string) {
	components.Label(c, words, theme.TextDefault, semantic.Text.Heading).FontWeight(600).Role(ui.RoleHeading)
}

// choiceRow lays a choice's answers side by side, as a radio group.
func (s *State) choiceRow(c *ui.Context, m metrics, label string, order int, animate bool, answers func()) {
	s.enter(c, ui.Row(c).WidthPercent(100).Gap(14*m.k).AlignItems(ui.Stretch).Role(ui.RoleRadioGroup).Label(label).Children(answers), order, animate)
}

// notebook shows the folder chosen as a folder: its tab, its path, and
// whether a notebook is there already, with the way to choose another.
func (s *State) notebook(c *ui.Context, semantic theme.Theme, m metrics, animate bool) {
	holds := s.Host.Holds != nil && s.Host.Holds(s.root)
	tab := 30 * m.k
	folder := ui.Column(c).Key("folder").WidthPercent(100).Padding(tab+18*m.k, 24*m.k, 22*m.k, 24*m.k).Gap(12 * m.k)
	folder.Draw(func(p *ui.Painter, r ui.Rect) {
		paintFolder(p, r, tab, m.radius, semantic)
	})
	folder.Children(func() {
		ui.Row(c).Gap(12 * m.k).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, components.IconFolder.SVG()).FontSize(22 * m.k).TextColor(semantic.Families[theme.Regolith].Foreground)
			components.Label(c, filepath.Base(s.root), theme.TextLarge, semantic.Text.Heading).FontWeight(600).SingleLine()
		})
		ui.Text(c, shortPath(s.root)).Font(theme.MonoFontFamily).FontSize(13 * m.k).LineHeight(1.5).
			TextColor(semantic.Text.Default).Selectable()
		status, color := locales.OnboardingNotebookNew(), semantic.Text.Muted
		if holds {
			status, color = locales.OnboardingNotebookFound(), semantic.Status.Success.Foreground
		}
		components.Label(c, status, theme.TextSmall, color)
		ui.Row(c).Gap(10*m.k).Padding(6*m.k, 0, 0, 0).Children(func() {
			components.Button{Key: "choose-folder", Label: locales.ChooseFolderEllipsis(), Style: components.OutlinedButton,
				Animate: animate, Action: s.askFolder}.View(c, semantic)
			if s.root != s.defaultRoot {
				components.Button{Key: "default-folder", Label: locales.OnboardingNotebookDefault(), Style: components.SubtleButton,
					Animate: animate, Action: func() { s.chooseRoot(s.defaultRoot) }}.View(c, semantic)
			}
		})
	})
	s.enter(c, folder, 3, animate)
}

// askFolder asks the host for another folder.
func (s *State) askFolder() {
	if s.Host.ChooseFolder != nil {
		s.Host.ChooseFolder(s.root, s.chooseRoot)
	}
}

// shortPath is path with the home folder as ~, as a person reads it.
func shortPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return path
}

// paintFolder draws a folder behind r: a tab at its top left, its body a
// sheet of the raised surface, a second sheet peeking out behind it.
func paintFolder(p *ui.Painter, r ui.Rect, tab, radius float32, semantic theme.Theme) {
	back := semantic.Families[theme.Regolith].Surface
	body := semantic.Surface.Elevated
	w, h := r.W, r.H
	var behind ui.Path
	tw := min(w*0.36, 220)
	behind.MoveTo(r.X, r.Y+radius)
	behind.QuadTo(r.X, r.Y, r.X+radius, r.Y)
	behind.LineTo(r.X+tw-radius, r.Y)
	behind.QuadTo(r.X+tw, r.Y, r.X+tw+radius*0.8, r.Y+tab*0.6)
	behind.LineTo(r.X+tw+radius*1.4, r.Y+tab*0.9)
	behind.LineTo(r.X+w-radius, r.Y+tab*0.9)
	behind.QuadTo(r.X+w, r.Y+tab*0.9, r.X+w, r.Y+tab*0.9+radius)
	behind.LineTo(r.X+w, r.Y+h-radius)
	behind.QuadTo(r.X+w, r.Y+h, r.X+w-radius, r.Y+h)
	behind.LineTo(r.X+radius, r.Y+h)
	behind.QuadTo(r.X, r.Y+h, r.X, r.Y+h-radius)
	behind.Close()
	p.FillPath(&behind, back)
	// A sheet standing in the folder.
	p.Fill(ui.Rect{X: r.X + 18, Y: r.Y + tab*0.55, W: w - 36, H: h - tab}, semantic.Surface.Page, radius*0.6)
	front := ui.Rect{X: r.X, Y: r.Y + tab*1.15, W: w, H: h - tab*1.15}
	p.Fill(front, body, radius)
	p.Fill(ui.Rect{X: front.X, Y: front.Y, W: front.W, H: 1}, semantic.Text.Default.Alpha(0.08), 0)
}

// writing chooses how the marks behave: each answer a live page typing
// its line under that answer.
func (s *State) writing(c *ui.Context, semantic theme.Theme, m metrics, animate bool) {
	type answer struct {
		key            string
		title, caption string
		value          any
	}
	choices := []struct {
		label, policy, option string
		answers               [2]answer
	}{
		{locales.OnboardingMarks(), "notation", "notation.reveal", [2]answer{
			{"marks-shown", locales.OnboardingMarksShown(), locales.OnboardingMarksShownCaption(), "shown"},
			{"marks-hidden", locales.OnboardingMarksHidden(), locales.OnboardingMarksHiddenCaption(), "hidden"},
		}},
		{locales.OnboardingPairs(), "pairs", "pairs.autoClose", [2]answer{
			{"pairs-closed", locales.OnboardingPairsClosed(), locales.OnboardingPairsClosedCaption(), true},
			{"pairs-open", locales.OnboardingPairsOpen(), locales.OnboardingPairsOpenCaption(), false},
		}},
		{locales.OnboardingSymbols(), "typography", "typography.symbols", [2]answer{
			{"symbols-on", locales.OnboardingSymbolsOn(), locales.OnboardingSymbolsOnCaption(), true},
			{"symbols-off", locales.OnboardingSymbolsOff(), locales.OnboardingSymbolsOffCaption(), false},
		}},
	}
	prose := theme.NewProseStyle(14.5 * m.k).WithSpacing(1.6)
	prose.Family = s.settings().Prose().Family
	for i, choice := range choices {
		script := s.showcase(choice.policy)
		s.enter(c, components.Label(c, choice.label, theme.TextDefault, semantic.Text.Heading).FontWeight(600), 3+2*i, animate)
		s.choiceRow(c, m, choice.label, 4+2*i, animate, func() {
			for j, a := range choice.answers {
				components.Specimen{
					Key: a.key, Title: a.title, Caption: a.caption, Animate: animate, Seed: float32(i*2 + j),
					Selected: s.option(choice.option) == a.value,
					Action:   func() { s.choose(appsettings.SetOption(choice.option, a.value)) },
				}.View(c, semantic, func() {
					if script == nil {
						return
					}
					uisettings.ShowcaseStage{
						Key: "stage-" + a.key, Script: *script, Policy: choice.policy,
						Values: s.preview(choice.option, a.value), Theme: semantic, Prose: prose,
						Height: 58 * m.k, Padding: 14 * m.k, Radius: 8 * m.k, Animate: animate,
					}.View(c)
				})
			}
		})
	}
}

// look chooses the theme and the typeface, each answer a small page set
// as it would be; choosing turns the window too.
func (s *State) look(c *ui.Context, semantic theme.Theme, m metrics, animate bool) {
	s.enter(c, components.Label(c, locales.Theme(), theme.TextDefault, semantic.Text.Heading).FontWeight(600), 3, animate)
	family := s.settings().Prose().Family
	s.choiceRow(c, m, locales.Theme(), 4, animate, func() {
		themes := []struct {
			key            string
			title, caption string
			choice         appsettings.ThemeChoice
		}{
			{"theme-starfield", locales.ThemeStarfield(), locales.OnboardingStarfieldCaption(), appsettings.Starfield},
			{"theme-daybreak", locales.ThemeDaybreak(), locales.OnboardingDaybreakCaption(), appsettings.Daybreak},
			{"theme-system", locales.ThemeSystem(), locales.OnboardingSystemCaption(), appsettings.System},
		}
		for i, t := range themes {
			components.Specimen{
				Key: t.key, Title: t.title, Caption: t.caption, Animate: animate, Seed: float32(10 + i),
				Selected: s.values.Appearance.Appearance == t.choice,
				Action: func() {
					s.choose(appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetTheme, Theme: uint8(t.choice)}))
				},
			}.View(c, semantic, func() {
				themePreview(c, m, t.choice, family)
			})
		}
	})
	s.enter(c, components.Label(c, locales.Typeface(), theme.TextDefault, semantic.Text.Heading).FontWeight(600).Padding(6*m.k, 0, 0, 0), 5, animate)
	s.choiceRow(c, m, locales.Typeface(), 6, animate, func() {
		faces := []struct {
			key            string
			title, caption string
			style          fonts.Style
		}{
			{"face-serif", locales.StyleSerif(), locales.OnboardingSerifCaption(), fonts.Serif},
			{"face-sans", locales.StyleSans(), locales.OnboardingSansCaption(), fonts.Sans},
		}
		for i, f := range faces {
			components.Specimen{
				Key: f.key, Title: f.title, Caption: f.caption, Animate: animate, Seed: float32(20 + i),
				Selected: s.values.Appearance.Typeface == f.style,
				Action: func() {
					s.choose(appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetTypeface, Typeface: f.style}))
				},
			}.View(c, semantic, func() {
				facePreview(c, m, semantic, proseFamily(f.style))
			})
		}
	})
}

// mac chooses the menus: the system's, or the app's own drawn to match.
func (s *State) mac(c *ui.Context, semantic theme.Theme, m metrics, animate bool) {
	s.enter(c, components.Label(c, locales.Menus(), theme.TextDefault, semantic.Text.Heading).FontWeight(600), 3, animate)
	s.choiceRow(c, m, locales.Menus(), 4, animate, func() {
		kinds := []struct {
			key            string
			title, caption string
			drawn          bool
		}{
			{"menus-native", locales.MenusNative(), locales.OnboardingMenusNativeCaption(), false},
			{"menus-neverland", locales.MenusNeverland(), locales.OnboardingMenusNeverlandCaption(), true},
		}
		for i, k := range kinds {
			components.Specimen{
				Key: k.key, Title: k.title, Caption: k.caption, Animate: animate, Seed: float32(30 + i),
				Selected: s.values.Appearance.DrawnMenus == k.drawn,
				Action: func() {
					s.choose(appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetDrawnMenus, Drawn: k.drawn}))
				},
			}.View(c, semantic, func() {
				menuPreview(c, m, semantic, k.drawn)
			})
		}
	})
}

// done sums up what was chosen, each line checked off in ink in turn,
// beside the mark, and opens the notebook.
func (s *State) done(c *ui.Context, semantic theme.Theme, m metrics, animate bool, now time.Time) {
	ui.Row(c).WidthPercent(100).Grow(1).AlignItems(ui.Center).Gap(m.gap).Children(func() {
		ui.Column(c).Width(300 * m.k).Shrink(0).Gap(14 * m.k).Children(func() {
			s.enter(c, s.eyebrow(c, semantic, m, "", locales.OnboardingDoneEyebrow()), 0, animate)
			s.enter(c, ui.Text(c, locales.OnboardingDoneHeadline()).Font(theme.ProseFontFamily).FontSize(m.display).FontWeight(500).
				LineHeight(1.12).LetterSpacing(-0.015*m.display).TextColor(semantic.Text.Heading).Role(ui.RoleHeading), 1, animate)
			s.enter(c, ui.Text(c, locales.OnboardingDoneBody()).Font(theme.ProseFontFamily).FontSize(16*m.k).LineHeight(1.6).
				TextColor(semantic.Text.Default), 2, animate)
			if s.failure != "" {
				ui.Column(c).Gap(4*m.k).Padding(12*m.k).Radius(m.radius).Background(semantic.Status.Danger.Background).
					Border(1, semantic.Status.Danger.Border).Role(ui.RoleStatus).Children(func() {
					components.Label(c, locales.OnboardingNotebookFailed(), theme.TextSmall, semantic.Status.Danger.Foreground).FontWeight(600)
					components.Label(c, s.failure, theme.TextSmall, semantic.Text.Default)
				})
			}
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Padding(26*m.k, 30*m.k).Radius(m.radius).
			Background(semantic.Surface.Page.Alpha(0.82)).Border(1, semantic.Border.Variant).Gap(0).Children(func() {
			for i, line := range s.summary() {
				s.summaryLine(c, semantic, m, line, i, i == len(s.summary())-1, animate, now)
			}
		})
	})
}

// summaryLine is one line of the summary: what was set and to what, a
// check drawn beside it after the line before it.
func (s *State) summaryLine(c *ui.Context, semantic theme.Theme, m metrics, line [2]string, i int, last, animate bool, now time.Time) {
	row := ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(14*m.k).Padding(11*m.k, 0)
	drawn := s.inkAfter(now, time.Duration(450+160*i)*time.Millisecond, 420*time.Millisecond, animate)
	if drawn < 1 && animate {
		c.AnimationFrame()
	}
	row.Children(func() {
		mark := 18 * m.k
		ui.Box(c).Size(mark, mark).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
			components.InkCheck(p, r, semantic.Families[theme.Life].Emphasis, 2.4*m.k, drawn)
		})
		components.Label(c, line[0], theme.TextDefault, semantic.Text.Muted).Width(160 * m.k).Shrink(0).SingleLine()
		ui.Text(c, line[1]).Font(theme.ProseFontFamily).FontSize(15 * m.k).TextColor(semantic.Text.Heading).
			Grow(1).MinWidth(0).MaxLines(2)
	})
	if !last {
		components.HorizontalDivider(c, semantic, components.FadedDivider)
	}
	s.enter(c, row, 2+i, animate)
}

// summary is what was set, line by line: the notebook and every choice of
// the pages, as chosen or as it stays.
func (s *State) summary() [][2]string {
	pick := func(on bool, yes, no string) string {
		if on {
			return yes
		}
		return no
	}
	themeName := map[appsettings.ThemeChoice]string{appsettings.Starfield: locales.ThemeStarfield(),
		appsettings.Daybreak: locales.ThemeDaybreak(), appsettings.System: locales.ThemeSystem()}
	lines := [][2]string{
		{locales.Notebook(), shortPath(s.root)},
		{locales.OnboardingMarks(), pick(s.option("notation.reveal") == "shown", locales.OnboardingMarksShown(), locales.OnboardingMarksHidden())},
		{locales.OnboardingPairs(), pick(s.option("pairs.autoClose") == true, locales.OnboardingPairsClosed(), locales.OnboardingPairsOpen())},
		{locales.OnboardingSymbols(), pick(s.option("typography.symbols") == true, locales.OnboardingSymbolsOn(), locales.OnboardingSymbolsOff())},
		{locales.Theme(), themeName[s.values.Appearance.Appearance]},
		{locales.Typeface(), pick(s.values.Appearance.Typeface == fonts.Sans, locales.StyleSans(), locales.StyleSerif())},
	}
	if s.Platform.Mac {
		lines = append(lines, [2]string{locales.Menus(), pick(s.values.Appearance.DrawnMenus, locales.MenusNeverland(), locales.MenusNative())})
	}
	return lines
}

// foot is the way on: the pages as marks along a line, the one shown
// ringed, and Back and Continue.
func (s *State) foot(c *ui.Context, semantic theme.Theme, m metrics, animate bool, now time.Time) {
	if s.at == pageWelcome {
		return
	}
	// The welcome is the cover; the count starts at the first choice.
	pages := s.pages()[1:]
	at := s.index(s.at) - 1
	ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(16*m.k).Padding(16*m.k, 0, 0, 0).Children(func() {
		track := ui.Box(c).Key("track").Size(float32(len(pages)-1)*26*m.k+12*m.k, 24*m.k).Shrink(0).
			Role(ui.RoleImage).Label(locales.OnboardingStep(fmt.Sprint(at+1), fmt.Sprint(len(pages))))
		ring := float32(at)
		if animate {
			ring = components.Follow(track, "ring", ring, components.MotionScene)
		}
		track.Draw(func(p *ui.Painter, r ui.Rect) {
			cy := r.Y + r.H/2
			step := 26 * m.k
			x0 := r.X + 6*m.k
			p.Fill(ui.Rect{X: x0, Y: cy - 0.5, W: step * float32(len(pages)-1), H: 1}, semantic.Text.Muted.Alpha(0.35), 0)
			for i := range pages {
				x := x0 + step*float32(i)
				ink := semantic.Text.Muted.Alpha(0.55)
				d := 5 * m.k
				if i <= at {
					ink, d = semantic.Text.Heading, 7*m.k
				}
				var dot ui.Path
				dot.Circle(x, cy, d/2)
				p.FillPath(&dot, ink)
			}
			x := x0 + step*ring
			components.InkRing(p, ui.Rect{X: x - 7*m.k, Y: cy - 7*m.k, W: 14 * m.k, H: 14 * m.k},
				semantic.Families[theme.Afterglow].Emphasis, 1.8*m.k, float32(at), 1)
		})
		components.Label(c, locales.OnboardingStep(fmt.Sprint(at+1), fmt.Sprint(len(pages))), theme.TextSmall, semantic.Text.Muted).
			FontFeatures("tnum").AccessHidden(true)
		ui.Box(c).Grow(1)
		components.Button{Key: "back", Label: locales.Back(), Style: components.SubtleButton, Animate: animate,
			Action: func() { s.turn(-1, s.clock) }}.View(c, semantic)
		label := locales.OnboardingContinue()
		if s.at == pageDone {
			label = locales.OnboardingStart()
		}
		components.Button{Key: "continue", Label: label, Style: components.PrimaryButton, Animate: animate,
			Action: func() { s.next(s.clock) }}.View(c, semantic).PaddingX(20 * m.k)
	})
}
