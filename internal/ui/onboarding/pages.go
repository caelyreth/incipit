package onboarding

import (
	"fmt"
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
	// between the words and the choices, measure the page's widest.
	top, pad, gap, measure float32
	// words is the column of a page's words; display its headline's size,
	// wordmark the name's on the welcome.
	words, display, wordmark float32
	radius                   float32
}

func newMetrics(c *ui.Context) metrics {
	scale := components.UIFontSize(c)
	k := scale / theme.DefaultUIFontSize
	return metrics{
		scale: scale, k: k,
		top: 52 * k, pad: 56 * k, gap: 72 * k, measure: 1080 * k,
		words: 260 * k, display: 34 * k, wordmark: 60 * k,
		radius: theme.DefaultControls.Scaled(scale).PanelRadius,
	}
}

// The keys the way on answers to, their caps on its buttons.
var (
	returnKey = components.ButtonKey{Key: ui.KeyEnter, Cap: "↩"}
	escapeKey = components.ButtonKey{Key: ui.KeyEscape, Cap: "esc"}
)

// pageView draws the page shown.
func (s *State) pageView(c *ui.Context, semantic theme.Theme, m metrics, animate bool, now time.Time) {
	switch s.at {
	case pageWelcome:
		s.welcome(c, semantic, m, animate, now)
	case pageNotebook:
		s.spread(c, semantic, m, animate, locales.OnboardingNotebookHeadline(), locales.OnboardingNotebookBody(),
			func() { s.notebook(c, semantic, m, animate) })
	case pageWriting:
		s.spread(c, semantic, m, animate, locales.OnboardingWritingHeadline(), "",
			func() { s.writing(c, semantic, m, animate) })
	case pageLook:
		s.spread(c, semantic, m, animate, locales.OnboardingLookHeadline(), "",
			func() { s.look(c, semantic, m, animate) })
	case pageMac:
		s.spread(c, semantic, m, animate, locales.OnboardingMacHeadline(), locales.OnboardingMacBody(),
			func() { s.mac(c, semantic, m, animate) })
	case pageDone:
		s.spread(c, semantic, m, animate, locales.OnboardingDoneHeadline(), "",
			func() { s.done(c, semantic, m, animate, now) })
	}
}

// welcome is the cover: the mark spreading onto the page, the name under
// it, one line saying what it is, and the way in.
func (s *State) welcome(c *ui.Context, semantic theme.Theme, m metrics, animate bool, now time.Time) {
	ui.Column(c).WidthPercent(100).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(0).Children(func() {
		drawn := s.inkAfter(now, 120*time.Millisecond, 1100*time.Millisecond, animate)
		if drawn < 1 {
			c.AnimationFrame()
		}
		height := markHeight * m.k
		mark := ui.Box(c).Key("mark").Size(height*components.BrandAspect(), height).Role(ui.RoleImage).Label(locales.OnboardingTitle())
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			s.mark.Paint(p, r, semantic.Text.Heading, drawn)
		})
		s.enter(c, ui.Text(c, "Neverland").Font(theme.ProseFontFamily).FontSize(m.wordmark).FontWeight(500).
			LetterSpacing(-0.02*m.wordmark).TextColor(semantic.Text.Heading).Role(ui.RoleHeading).
			Padding(28*m.k, 0, 0, 0), 1, animate)
		line := ui.Text(c, locales.OnboardingWelcomeHeadline()).Font(theme.ProseFontFamily).Italic().FontSize(19*m.k).
			TextColor(semantic.Text.Muted).Padding(6*m.k, 0, 0, 0)
		underline := s.inkAfter(now, 650*time.Millisecond, 500*time.Millisecond, animate)
		if underline < 1 {
			c.AnimationFrame()
		}
		line.DrawOver(func(p *ui.Painter, r ui.Rect) {
			// A stroke under the line, as a pen sets a title off.
			components.InkUnderline(p, r.X+r.W*0.12, r.X+r.W*0.88, r.Y+r.H+4*m.k, semantic.Accent.Foreground, 2.4*m.k, 7, underline)
		})
		s.enter(c, line, 2, animate)
		s.enter(c, ui.Row(c).Padding(40*m.k, 0, 0, 0).Children(func() {
			components.Button{Key: "begin", Label: locales.OnboardingBegin(), Style: components.PrimaryButton,
				Animate: animate, Shortcut: returnKey, Action: func() { s.next(s.clock) }}.View(c, semantic)
		}), 3, animate)
	})
}

// markHeight is how tall the mark stands on the cover, at the default UI
// size.
const markHeight = 190

// inkAfter is how far a mark has drawn itself, starting after wait from
// the page's turn and taking draw.
func (s *State) inkAfter(now time.Time, wait, draw time.Duration, animate bool) float32 {
	if !animate {
		return 1
	}
	return min(1, max(0, float32(now.Sub(s.turned)-wait)/float32(draw)))
}

// spread is a page of a choice: its headline, and a line more where one
// is needed, on the left, and what it chooses between on the right.
func (s *State) spread(c *ui.Context, semantic theme.Theme, m metrics, animate bool, headline, body string, choices func()) {
	ui.Row(c).WidthPercent(100).Grow(1).MinHeight(0).AlignItems(ui.Center).Gap(m.gap).Children(func() {
		ui.Column(c).Width(m.words).Shrink(0).Gap(12 * m.k).Children(func() {
			s.enter(c, ui.Text(c, headline).Font(theme.ProseFontFamily).FontSize(m.display).FontWeight(500).
				LineHeight(1.15).LetterSpacing(-0.015*m.display).TextColor(semantic.Text.Heading).Role(ui.RoleHeading), 0, animate)
			if body != "" {
				s.enter(c, ui.Text(c, body).Font(theme.ProseFontFamily).FontSize(15*m.k).LineHeight(1.6).
					TextColor(semantic.Text.Muted), 1, animate)
			}
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(14 * m.k).Children(choices)
	})
}

// choiceRow lays a choice's answers side by side, as a radio group, its
// question over them.
func (s *State) choiceRow(c *ui.Context, m metrics, label string, order int, animate bool, answers func()) {
	s.enter(c, ui.Row(c).WidthPercent(100).Gap(16*m.k).AlignItems(ui.Stretch).Role(ui.RoleRadioGroup).Label(label).
		Padding(0, 0, 14*m.k, 0).Children(answers), order, animate)
}

// notebook shows the folder chosen: its name, its path, and
// whether a notebook is there already, with the way to choose another.
func (s *State) notebook(c *ui.Context, semantic theme.Theme, m metrics, animate bool) {
	holds := s.Host.Holds != nil && s.Host.Holds(s.root)
	folder := ui.Column(c).Key("folder").WidthPercent(100).Padding(24*m.k, 28*m.k).Gap(10*m.k).
		Radius(m.radius).Background(semantic.Surface.Page).Border(1, semantic.Border.Variant)
	folder.Children(func() {
		ui.Row(c).Gap(10 * m.k).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, components.IconFolder.SVG()).FontSize(20 * m.k).TextColor(semantic.Accent.Foreground)
			components.Label(c, filepath.Base(s.root), theme.TextLarge, semantic.Text.Heading).FontWeight(600).SingleLine()
		})
		ui.Text(c, shortPath(s.root)).Font(theme.MonoFontFamily).FontSize(13 * m.k).LineHeight(1.5).
			TextColor(semantic.Text.Default).Selectable()
		status, color := locales.OnboardingNotebookNew(), semantic.Text.Muted
		if holds {
			status, color = locales.OnboardingNotebookFound(), semantic.Status.Success.Foreground
		}
		components.Label(c, status, theme.TextSmall, color)
		ui.Row(c).Gap(10*m.k).Padding(10*m.k, 0, 0, 0).Children(func() {
			components.Button{Key: "choose-folder", Label: locales.ChooseFolderEllipsis(), Style: components.OutlinedButton,
				Animate: animate, Action: s.askFolder}.View(c, semantic)
			if s.root != s.defaultRoot {
				components.Button{Key: "default-folder", Label: locales.OnboardingNotebookDefault(), Style: components.SubtleButton,
					Animate: animate, Action: func() { s.chooseRoot(s.defaultRoot) }}.View(c, semantic)
			}
		})
	})
	s.enter(c, folder, 2, animate)
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

// done sums up what was chosen, each line checked, and opens the notebook.
func (s *State) done(c *ui.Context, semantic theme.Theme, m metrics, animate bool, now time.Time) {
	if s.failure != "" {
		ui.Column(c).Gap(4*m.k).Padding(12*m.k).Radius(m.radius).Background(semantic.Status.Danger.Background).
			Border(1, semantic.Status.Danger.Border).Role(ui.RoleStatus).Children(func() {
			components.Label(c, locales.OnboardingNotebookFailed(), theme.TextSmall, semantic.Status.Danger.Foreground).FontWeight(600)
			components.Label(c, s.failure, theme.TextSmall, semantic.Text.Default)
		})
	}
	lines := s.summary()
	s.enter(c, ui.Column(c).WidthPercent(100).Padding(10*m.k, 28*m.k).Radius(m.radius).
		Background(semantic.Surface.Page).Border(1, semantic.Border.Variant).Children(func() {
		for i, line := range lines {
			s.summaryLine(c, semantic, m, line, i == len(lines)-1)
		}
	}), 1, animate)
}

// summaryLine is one line of the summary: what was set, and to what.
func (s *State) summaryLine(c *ui.Context, semantic theme.Theme, m metrics, line [2]string, last bool) {
	ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(16*m.k).Padding(12*m.k, 0).Children(func() {
		components.Label(c, line[0], theme.TextDefault, semantic.Text.Muted).Width(170 * m.k).Shrink(0).SingleLine()
		ui.Text(c, line[1]).Font(theme.ProseFontFamily).FontSize(15 * m.k).TextColor(semantic.Text.Heading).
			Grow(1).MinWidth(0).MaxLines(2)
	})
	if !last {
		components.HorizontalDivider(c, semantic, components.FadedDivider)
	}
}

// foot is the way on: the pages as dots along a line, the one shown
// ringed, and Back and Continue, Escape and Return.
func (s *State) foot(c *ui.Context, semantic theme.Theme, m metrics, animate bool) {
	if s.at == pageWelcome {
		return
	}
	// The cover is not counted: the count starts at the first choice.
	pages := s.pages()[1:]
	at := s.index(s.at) - 1
	count := locales.OnboardingStep(fmt.Sprint(at+1), fmt.Sprint(len(pages)))
	ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(16*m.k).Padding(20*m.k, 0, 0, 0).Children(func() {
		step := 22 * m.k
		track := ui.Box(c).Key("track").Size(float32(len(pages)-1)*step+16*m.k, 24*m.k).Shrink(0).
			Role(ui.RoleImage).Label(count)
		ring := float32(at)
		if animate {
			ring = components.Follow(track, "ring", ring, components.MotionScene)
		}
		track.Draw(func(p *ui.Painter, r ui.Rect) {
			cy := r.Y + r.H/2
			x0 := r.X + 8*m.k
			p.Fill(ui.Rect{X: x0, Y: cy - 0.5, W: step * float32(len(pages)-1), H: 1}, semantic.Text.Muted.Alpha(0.3), 0)
			for i := range pages {
				ink, d := semantic.Text.Muted.Alpha(0.5), 5*m.k
				if i <= at {
					ink, d = semantic.Text.Heading, 6*m.k
				}
				var dot ui.Path
				dot.Circle(x0+step*float32(i), cy, d/2)
				p.FillPath(&dot, ink)
			}
			x := x0 + step*ring
			components.InkRing(p, ui.Rect{X: x - 6*m.k, Y: cy - 6*m.k, W: 12 * m.k, H: 12 * m.k},
				semantic.Accent.Foreground, 1.6*m.k, float32(at), 1)
		})
		components.Label(c, count, theme.TextSmall, semantic.Text.Muted).FontFeatures("tnum").AccessHidden(true)
		ui.Box(c).Grow(1)
		components.Button{Key: "back", Label: locales.Back(), Style: components.SubtleButton, Animate: animate,
			Shortcut: escapeKey, Action: func() { s.turn(-1, s.clock) }}.View(c, semantic)
		label := locales.OnboardingContinue()
		if s.at == pageDone {
			label = locales.OnboardingStart()
		}
		components.Button{Key: "continue", Label: label, Style: components.PrimaryButton, Animate: animate,
			Shortcut: returnKey, Action: func() { s.next(s.clock) }}.View(c, semantic)
	})
}
