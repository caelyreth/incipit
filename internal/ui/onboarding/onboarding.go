// Package onboarding is Neverland's first launch: a short walkthrough, in
// a window of its own, that sets up where the notebook lives and the few
// choices that change how writing feels, each shown by what it does
// rather than named on a switch. It holds what is chosen until the person
// finishes; the app then opens the notebook there and keeps the choices
// through its settings (Result).
package onboarding

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/caelyreth/neverland/internal/app/fonts"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/shell"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Result is what the walkthrough set up: the notebook's folder, and the
// changes the person chose, in the order chosen, to apply to the
// notebook's settings as it opens. A choice left as it was is not among
// them, so a notebook that is already there keeps its own.
type Result struct {
	Root    string
	Changes []appsettings.Change
}

// Platform is what the walkthrough offers where it runs.
type Platform struct {
	// Mac offers the choices macOS alone has: its own menus or Neverland's.
	Mac bool
}

// Backup describes a verified notebook backup available for recovery.
type Backup struct {
	Path string
	Date string
	Size string
}

// Host is what the walkthrough asks of the app around it.
type Host struct {
	// Holds tells whether a folder holds a notebook already.
	Holds func(root string) bool
	// ChooseFolder asks for another folder, starting at current, and
	// calls chosen with it on the UI thread, or not at all.
	ChooseFolder func(current string, chosen func(string))
	// Backups lists verified backups in root, newest first.
	Backups func(root string) []Backup
	// RestoreBackup confirms and restores backup inside root, calling
	// done on the UI thread with whether restoration succeeded and any error.
	RestoreBackup func(root string, backup Backup, done func(restored bool, err error))
	// SystemDark tells whether the system's appearance is dark, which the
	// System theme follows.
	SystemDark func() bool
	// Appearance moves the window's own chrome to a theme chosen.
	Appearance func(appsettings.ThemeChoice)
	// Finish opens the notebook as set up; it returns why it could not,
	// and the walkthrough stays.
	Finish func(Result) error
}

// page is one page of the walkthrough.
type page uint8

const (
	pageWelcome page = iota
	pageNotebook
	pageWriting
	pageLook
	pageMac
	pageDone
	// pageLost says a notebook recorded is not there, before the cover.
	pageLost
)

// State is the walkthrough as it runs.
type State struct {
	Host     Host
	Platform Platform

	// defaultRoot is where the notebook goes unless another folder is
	// chosen, root the folder chosen.
	defaultRoot, root string
	// values are the settings as chosen so far, which the previews and
	// the window show; changes the choices made, by what each sets.
	values  appsettings.State
	changes []appsettings.Change
	// at is the page shown, from the page before it, turned when.
	at, from page
	turned   time.Time
	started  time.Time
	failure  string
	// clock is the time of the frame last drawn, which actions turn by.
	clock  time.Time
	colors theme.Transition
	// mark is the brand's, as rasterized.
	mark   components.BrandMark
	placed bool
	// lost is the notebook recorded that is not there, if it is not, and
	// lostNote what looking for it last found, with any verified backups
	// found alongside it.
	lost        *Lost
	lostNote    string
	lostBackups []Backup
	finishing   bool
	// trial is the field's card the menus are tried on, made as the Mac's
	// page first shows.
	trial *shell.Trial
}

// New starts the walkthrough with the notebook in defaultRoot.
func New(defaultRoot string, platform Platform, host Host) *State {
	return &State{Host: host, Platform: platform, defaultRoot: defaultRoot, root: defaultRoot, values: appsettings.DefaultState()}
}

// Root is the folder chosen for the notebook.
func (s *State) Root() string { return s.root }

// Result is what has been set up so far.
func (s *State) Result() Result {
	return Result{Root: s.root, Changes: slices.Clone(s.changes)}
}

// pages are the walkthrough's pages here: the lost notebook's first where
// one is, the Mac's only on the Mac.
func (s *State) pages() []page {
	var pages []page
	if s.lost != nil {
		pages = append(pages, pageLost)
	}
	pages = append(pages, pageWelcome, pageNotebook, pageWriting, pageLook)
	if s.Platform.Mac {
		pages = append(pages, pageMac)
	}
	return append(pages, pageDone)
}

// counted are the pages the foot counts: those after the cover.
func (s *State) counted() []page {
	pages := s.pages()
	return pages[s.index(pageWelcome)+1:]
}

// index is p's place among the pages.
func (s *State) index(p page) int { return max(0, slices.Index(s.pages(), p)) }

// turn shows the page step pages on, back for a negative step.
func (s *State) turn(step int, now time.Time) {
	if s.finishing {
		return
	}
	pages := s.pages()
	next := min(max(s.index(s.at)+step, 0), len(pages)-1)
	if pages[next] == s.at {
		return
	}
	s.from, s.at, s.turned, s.failure = s.at, pages[next], now, ""
}

// next goes on, or finishes from the last page.
func (s *State) next(now time.Time) {
	if s.finishing {
		return
	}
	if s.at == pageDone {
		s.finish()
		return
	}
	s.turn(1, now)
}

// finish hands the result to the host; a failure stays on the page.
func (s *State) finish() {
	if s.Host.Finish == nil || s.finishing {
		return
	}
	s.finishing = true
	err := s.Host.Finish(s.Result())
	s.finishing = false
	if err != nil {
		s.failure = err.Error()
	}
}

// choose applies change to the settings shown and keeps it, in place of
// an earlier choice of the same thing.
func (s *State) choose(change appsettings.Change) {
	if _, err := s.values.Apply(change); err != nil {
		return
	}
	id := changeIdentity(change)
	s.changes = slices.DeleteFunc(s.changes, func(c appsettings.Change) bool { return changeIdentity(c) == id })
	s.changes = append(s.changes, change)
	if change.Kind == appsettings.ChangeAppearance && change.Appearance.Kind == appsettings.SetTheme && s.Host.Appearance != nil {
		s.Host.Appearance(appsettings.ThemeChoice(change.Appearance.Theme))
	}
}

// changeIdentity names what a change sets, whatever it sets it to.
func changeIdentity(change appsettings.Change) string {
	switch change.Kind {
	case appsettings.ChangePolicy:
		return fmt.Sprint(change.Kind, change.Option)
	case appsettings.ChangeAppearance:
		return fmt.Sprint(change.Kind, change.Appearance.Kind)
	}
	return fmt.Sprint(change.Kind, change.Presentation)
}

// chooseRoot takes root as the notebook's folder.
func (s *State) chooseRoot(root string) {
	if root == "" {
		return
	}
	s.root, s.failure = filepath.Clean(root), ""
}

// option is the value of a policy option as chosen so far.
func (s *State) option(key string) any {
	if v, ok := s.values.Policy[key]; ok {
		return v
	}
	for _, p := range s.values.Program.Catalog().Policies {
		for _, o := range p.Options {
			if o.Key == key {
				return o.Default
			}
		}
	}
	return nil
}

// with is the policy values chosen so far with key set to value, which a
// preview plays under.
func (s *State) with(key string, value any) policy.Values {
	values := maps.Clone(s.values.Policy)
	if values == nil {
		values = policy.Values{}
	}
	values[key] = value
	return values
}

// preview is the values an answer's page plays under: those chosen so far
// with key set to value, its guides left out, whose keys would only
// crowd a line so short.
func (s *State) preview(key string, value any) policy.Values {
	values := s.with(key, value)
	values[policy.EnabledKey("guides")] = false
	return values
}

// showcase is policy id's demonstration.
func (s *State) showcase(id string) *policy.Showcase {
	for _, p := range s.values.Program.Catalog().Policies {
		if p.ID == id {
			return p.Showcase
		}
	}
	return nil
}

// dark is whether the system is dark, as the System theme reads it.
func (s *State) dark() bool { return s.Host.SystemDark != nil && s.Host.SystemDark() }

// settings is the display the choices so far make.
func (s *State) settings() theme.Settings {
	values := s.values.Appearance
	settings := theme.DefaultSettings()
	settings.Appearance = theme.Appearance(values.ThemeNumber(s.dark()))
	settings.ProseFamily = proseFamily(values.Typeface)
	settings.DrawnMenus = values.DrawnMenus
	return settings
}

// proseFamily is the prose's face in style, as the app ships it.
func proseFamily(style fonts.Style) string {
	if style == fonts.Sans {
		return theme.ProseSansFamily
	}
	return theme.ProseFontFamily
}

// View draws the walkthrough: the page on the window's ground, the way on
// at its foot. Anywhere nothing answers the pointer, it moves the window.
func (s *State) View(c *ui.Context) {
	now := c.Now()
	s.clock = now
	if s.started.IsZero() {
		s.started, s.turned = now, now
	}
	settings := s.settings()
	target := settings.Semantic()
	animate := components.MotionEnabled(c, true)
	if !s.placed || !animate {
		s.colors.Jump(target)
		s.placed = true
	} else {
		s.colors.Retarget(target, now)
	}
	semantic, changing := s.colors.At(settings, now)
	if changing {
		c.AnimationFrame()
	}
	c.SetTheme(settings.MyGoThemeFor(semantic))
	components.Menus(c, semantic, settings.DrawnMenus)
	m := newMetrics(c)

	root := ui.Box(c).Key("onboarding").WidthPercent(100).HeightPercent(100).Clip().
		Background(semantic.Surface.Background).DragWindow()
	root.Children(func() {
		// The page stands in a measure of its own, centered, however wide
		// the window: room around it, never stretched thin.
		ui.Column(c).WidthPercent(100).HeightPercent(100).Padding(m.top, m.pad, m.pad, m.pad).AlignItems(ui.Center).Children(func() {
			page := ui.Column(c).Key("page").WidthPercent(100).MaxWidth(m.measure).Grow(1).MinHeight(0)
			// The way on comes up as the first page counted shows, and goes
			// back to the cover.
			foot := float32(0)
			if s.at != pageWelcome && s.at != pageLost {
				foot = 1
			}
			if animate {
				foot = components.Follow(page, "foot", foot, components.MotionPanel)
			}
			page.Children(func() {
				body := ui.Column(c).Key(int(s.at)).WidthPercent(100).Grow(1).MinHeight(0).Justify(ui.Center)
				body.Children(func() { s.pageView(c, semantic, m, animate, now) })
				s.foot(c, semantic, m, animate, foot)
			})
		})
	})
}

// entering is how far an element of the page shown has come in, from 0
// to 1: each a little after the one before it (order), so the page sets
// itself line by line, as type is laid.
func (s *State) entering(now time.Time, order int, animate bool) float32 {
	if !animate {
		return 1
	}
	since := now.Sub(s.turned) - time.Duration(order)*50*time.Millisecond
	return components.GlideEase(float32(since) / float32(components.MotionScene))
}

// enter has el come in at order: rising and clearing as it arrives, from
// the side the page turned from.
func (s *State) enter(c *ui.Context, el ui.Element, order int, animate bool) ui.Element {
	k := s.entering(c.Now(), order, animate)
	if k < 1 {
		c.AnimationFrame()
		dir := float32(1)
		if s.index(s.at) < s.index(s.from) {
			dir = -1
		}
		el.Opacity(k).Translate(dir*16*(1-k), 0)
	}
	return el
}
