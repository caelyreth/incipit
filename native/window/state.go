package window

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/app/library"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/notices"
	"github.com/caelyreth/neverland/native/overlay"
	"github.com/caelyreth/neverland/native/settings"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// AppState owns one main-window notebook and its app-wide settings.
type AppState struct {
	Workspace *workspace.Workspace
	Settings  settings.State
	Product   appsettings.State
	Shell     *shell.State
	Palette   *overlay.CommandPalette
	// Notices tells the user what happened apart from what they did, as
	// toasts at the window's foot.
	Notices        *notices.Center
	mainWindow     *mygo.Window
	settingsWindow *mygo.Window
	// menu is the application menu; recording stands its keys down while
	// the settings record a shortcut.
	menu       *mygo.Menu
	systemKeys map[*mygo.MenuItem]string
	recording  bool
	// closeWindow closes a window from its own command route.
	closeWindow func(*mygo.Window)
	// colors carries every window through a theme or accent change at once.
	colors theme.Transition
	// sessions are the pages' sessions by identity, which the notebook
	// saves.
	sessions map[workspace.PageID]*sheetSession
	// notebook keeps the pages on disk; nil holds them in memory alone.
	notebook *notebook
	// library files the pages, as the sidebar shows them.
	library *library.Library
	// nextKey is the notebook key the next new page takes.
	nextKey storage.PageKey
	// nextView numbers the editors' views.
	nextView uint64
	// chrome is the editor state the footer and formatting toolbar last
	// showed.
	chrome chromeKey
	// settingsDirty waits for the next write of the settings record.
	settingsDirty bool
}

// NewAppState is a notebook held in memory alone, as tests use.
func NewAppState() (*AppState, error) {
	return newAppState(nil)
}

// OpenAppState opens the notebook a store holds, and saves it there as it
// changes.
func OpenAppState(store *storage.Store) (*AppState, error) {
	return newAppState(store)
}

func newAppState(store *storage.Store) (*AppState, error) {
	app := &AppState{
		Notices:     &notices.Center{},
		sessions:    map[workspace.PageID]*sheetSession{},
		nextKey:     1,
		closeWindow: func(window *mygo.Window) { window.Close() },
		Product:     appsettings.DefaultState(),
	}
	sessions, layout, err := app.openNotebook(store)
	if err != nil {
		return nil, err
	}
	pages, err := workspace.Restore(sessions, layout, app.createPage)
	if err != nil {
		return nil, err
	}
	app.Workspace = pages
	app.Settings = *settings.New()
	app.Shell = shell.New()
	app.Palette = &overlay.CommandPalette{}
	app.Shell.Colors = &app.colors
	app.Settings.Colors = &app.colors
	if err := app.loadSettings(store); err != nil {
		return nil, err
	}
	app.Shell.Settings = appearanceSettings(app.Product.Appearance)
	app.Shell.Settings.Accent = app.groupAccent()
	app.Shell.Appearance = app.Product.Presentation
	app.BindShell()
	return app, nil
}

// loadSettings restores the saved appearance, writing modes, keymap and
// policy values. A record that cannot be read or compiled is replaced by
// the defaults; the pages are untouched.
func (a *AppState) loadSettings(store *storage.Store) error {
	if store == nil {
		return nil
	}
	data, err := store.LoadSettings()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if err := a.Product.Decode(data); err != nil {
		a.Product = appsettings.DefaultState()
		a.settingsDirty = true
		a.saveSettings()
		a.Notices.Post(notices.Notice{
			ID: "settings.load", Kind: notices.Warning, Title: "Settings were reset",
			Description: "Your notebook is unchanged. The saved preferences could not be read.",
		})
	}
	return nil
}

func (a *AppState) BindShell() {
	a.Shell.Content = a.pageView
	a.Shell.SelectPage = func(id string) {
		page, ok := parsePageID(id)
		if !ok {
			return
		}
		if err := a.Workspace.Open(page); err != nil {
			a.fail("Couldn't open the page", err)
			return
		}
		a.selectMounted()
	}
	a.Shell.Library = a
	// The library opens on the Inbox, where new sheets go.
	a.Shell.StartAt(shell.Place{Kind: shell.PlaceGroup, Group: groupName(library.Inbox)})
	a.Shell.NewPage = func() { a.NewSheet(a.Shell.CurrentGroup(), shell.Last) }
	a.Shell.MoveTab = func(name string, index int) {
		if session, ok := a.sessionOf(name); ok {
			a.Workspace.MoveTab(session.page, index)
			a.refreshShell()
		}
	}
	a.Shell.CloseOtherTabs = func(name string) { a.closeTabsWhere(name, func(at, kept int) bool { return at != kept }) }
	a.Shell.CloseTabsAfter = func(name string) { a.closeTabsWhere(name, func(at, kept int) bool { return at > kept }) }
	a.Shell.ClosePage = func() {
		id, ok := a.Workspace.Active()
		if !ok {
			return
		}
		a.closePageID(fmt.Sprintf("%d", id))
	}
	a.Shell.ClosePageID = a.closePageID
	a.Shell.OpenCommands = func() {
		a.Palette.Open = true
		a.Palette.Reset(a.commandCatalog())
	}
	a.Shell.OpenSettings = a.OpenSettingsWindow
	a.Shell.FindPage = a.findPage
	a.Shell.Typewriter = func(on bool) {
		a.setPresentation(appsettings.PresentationTypewriter, on)
	}
	a.Shell.Zen = func(on bool) {
		a.setPresentation(appsettings.PresentationZen, on)
	}
	a.Shell.Strong = a.formatStrong
	a.Shell.Emphasis = a.formatEmphasis
	a.Shell.Code = a.formatCode
	a.Shell.Strike = a.formatStrike
	a.refreshShell()
}

// pageView is what the open page shows: its title, set as the page's
// heading, over the page's editor.
func (a *AppState) pageView(c *ui.Context) {
	semantic, _ := a.colors.At(a.Shell.Settings, c.Now())
	scale := a.Shell.Settings.UIFontSize
	top := theme.DefaultLayout.ReadingTop * scale / theme.DefaultUIFontSize
	ui.Column(c).Key("page").Role(ui.RoleGroup).Label("Page").
		FillHeight().WidthPercent(100).Padding(top, 0, 0, 0).Children(func() {
		el := a.pageElement()
		if el == nil {
			return
		}
		el.Theme = semantic
		el.Prose = a.Shell.Settings.Prose()
		el.Top, el.Bottom = top/2, top*2
		el.Typewriter = a.Product.Presentation.Typewriter
		el.Keys = a.Product.Keymap
		el.UIFontSize = a.Shell.Settings.UIFontSize
		// A press on what takes no focus, as a pane's background, leaves
		// the focus nowhere: typing belongs to the page then, its caret
		// shown, unless a field of the shell or the palette is taking it.
		if !a.Shell.FocusWithin(c) && !a.Shell.Typing() && (a.Palette == nil || !a.Palette.Open) {
			el.Focus()
		}
		ui.Box(c).Key(fmt.Sprintf("editor-%d", el.Editor.View)).WidthPercent(100).Grow(1).Clip().Children(func() {
			el.View(c)
		})
	})
}

func (a *AppState) refreshToolbarState() {
	if a == nil || a.Shell == nil {
		return
	}
	keys := a.Product.Keymap
	formatting := shell.FormattingState{}
	if session, _, ok := a.activeSession(); ok && session.element != nil {
		formatting = formattingState(session.element.Editor)
	}
	a.Shell.Toolbar = shell.ToolbarState{
		Typewriter:        a.Product.Presentation.Typewriter,
		Zen:               a.Product.Presentation.Zen,
		TypewriterEnabled: true,
		ZenEnabled:        true,
		Formatting:        formatting,
		Bindings:          toolbarBindings(keys),
	}
}

func (a *AppState) togglePresentation(mode appsettings.Presentation, callback func(bool)) {
	on := !a.Product.Presentation.Get(mode)
	if callback != nil {
		callback(on)
		return
	}
	a.setPresentation(mode, on)
}

func (a *AppState) navigatePage(forward bool) {
	if _, ok, err := a.Workspace.Navigate(forward); err != nil {
		a.fail("Couldn't go to the page", err)
		return
	} else if !ok {
		return
	}
	a.selectMounted()
}

func (a *AppState) findPage() {
	if a == nil || a.Shell == nil {
		return
	}
	a.setPresentation(appsettings.PresentationZen, false)
	visible := true
	a.Shell.SidebarOverride = &visible
	a.Shell.Search()
}

func (a *AppState) selectMounted() {
	a.captureViews()
	a.refreshShell()
	a.syncMenu()
}

func (a *AppState) closePageID(id string) {
	page, ok := parsePageID(id)
	if !ok {
		return
	}
	a.captureViews()
	if err := a.Workspace.Close(page); err != nil {
		a.fail("Couldn't close the page", err)
		return
	}
	a.selectMounted()
}

func parsePageID(id string) (workspace.PageID, bool) {
	var page workspace.PageID
	if _, err := fmt.Sscanf(id, "%d", &page); err != nil {
		return 0, false
	}
	return page, true
}

func (a *AppState) refreshShell() {
	a.Shell.Tabs = nil
	for _, page := range a.Workspace.TabPages() {
		a.Shell.Tabs = append(a.Shell.Tabs, shell.Page{
			ID:    fmt.Sprintf("%d", page.ID),
			Title: page.Title,
		})
	}
	a.Shell.ActivePage = ""
	if id, ok := a.Workspace.Active(); ok {
		a.Shell.ActivePage = fmt.Sprintf("%d", id)
	}
	a.refreshEditorChrome()
	a.refreshToolbarState()
}

func (a *AppState) refreshSettingsData() {
	a.Settings.Data = a.Product.Data()
	a.Settings.Theme = a.Shell.Settings
	a.Settings.Animate = a.Product.Presentation.Motion
	a.Settings.Recording = a.setRecording
	a.Settings.Reserved = a.reservedKey
	a.Settings.Apply = func(change appsettings.Change) {
		outcome, err := a.Product.Apply(change)
		if err != nil {
			a.fail("Couldn't change the setting", err)
			return
		}
		if outcome.PolicyChanged {
			a.applyPolicy()
		}
		if outcome.KeymapChanged {
			a.syncMenu()
			a.refreshToolbarState()
		}
		switch change.Kind {
		case appsettings.ChangeAppearance:
			a.applyAppearance()
		case appsettings.ChangePresentation:
			a.refreshAppearance()
		}
		a.settingsChanged()
		a.refreshSettingsData()
	}
}

// settingsChanged queues the settings record; the saver writes it after
// its settle delay, beside the workspace layout.
func (a *AppState) settingsChanged() {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return
	}
	a.settingsDirty = true
	a.saveSettings()
}

func (a *AppState) saveSettings() {
	if a == nil || !a.settingsDirty || a.notebook == nil || a.notebook.closed {
		return
	}
	data, err := a.Product.Encode()
	if err != nil {
		a.fail("Couldn't save settings", err)
		return
	}
	a.notebook.saver.Settings(data)
	a.settingsDirty = false
}

// applyAppearance moves every window to the chosen theme together: the
// views blend their colors over ThemeTransition, and the native chrome,
// window backgrounds and traffic lights follow the new scheme.
func (a *AppState) applyAppearance() {
	values := a.Product.Appearance
	a.Shell.Settings = appearanceSettings(values)
	a.Shell.Settings.Accent = a.groupAccent()
	target := a.Shell.Settings.Semantic()
	if a.Product.Presentation.Motion {
		a.colors.Retarget(target, time.Now())
	} else {
		a.colors.Jump(target)
	}
	mygo.Theme.SetSource(themeSource(a.Shell.Settings))
	background := colorHex(target.Surface.Background)
	for _, window := range []*mygo.Window{a.mainWindow, a.settingsWindow} {
		if window == nil || window.IsDestroyed() {
			continue
		}
		_ = window.SetBackgroundColor(background)
		window.Invalidate()
	}
	a.refreshAppearance()
}

func (a *AppState) refreshAppearance() {
	a.Shell.Appearance = a.Product.Presentation
	a.refreshShell()
}

func (a *AppState) setPresentation(mode appsettings.Presentation, on bool) {
	if a == nil {
		return
	}
	outcome, err := a.Product.Apply(appsettings.SetPresentation(mode, on))
	if err != nil {
		a.fail("Couldn't change the setting", err)
		return
	}
	if outcome.Applied {
		a.settingsChanged()
	}
	a.refreshAppearance()
	a.refreshSettingsData()
}

// themeSource is the native appearance matching settings, so traffic
// lights, menus and window chrome agree with the painted theme.
func themeSource(settings theme.Settings) mygo.ThemeSource {
	if settings.Appearance == theme.Daybreak {
		return mygo.ThemeLight
	}
	return mygo.ThemeDark
}

func appearanceSettings(values appsettings.AppearanceSettings) theme.Settings {
	settings := theme.DefaultSettings()
	settings.UIFontSize = values.UIFontSize
	settings.ProseFontSize = values.ProseFontSize
	settings.LineSpacing = values.LineSpacing
	settings.Appearance = theme.Appearance(values.ThemeNumber())
	return settings
}

// groupAccent is the accent the window wears: the color of the open
// sheet's group, Halo for a group in Moonveil, the Inbox, or no sheet.
func (a *AppState) groupAccent() theme.Accent {
	if a.library == nil || a.Shell == nil {
		return theme.Halo
	}
	group, ok := a.Group(a.SheetGroup(a.Shell.ActivePage))
	if !ok || group.Color < shell.TintHalo || group.Color >= shell.TintHalo+len(theme.AllAccents) {
		return theme.Halo
	}
	return theme.AllAccents[group.Color-shell.TintHalo]
}

// followAccent turns the window to its group's accent when another
// sheet comes to the front or its group's color changes, blending as a
// theme change does.
func (a *AppState) followAccent(now time.Time) {
	accent := a.groupAccent()
	if accent == a.Shell.Settings.Accent {
		return
	}
	a.Shell.Settings.Accent = accent
	target := a.Shell.Settings.Semantic()
	if a.Product.Presentation.Motion {
		a.colors.Retarget(target, now)
	} else {
		a.colors.Jump(target)
	}
}

// View composes the shell, the command palette and the notices.
func (a *AppState) View(c *ui.Context) {
	if a == nil || a.Shell == nil {
		return
	}
	a.followAccent(c.Now())
	a.refreshEditorChrome()
	semantic, _ := a.colors.At(a.Shell.Settings, c.Now())
	animate := components.MotionEnabled(c, a.Shell.Appearance.Motion, a.Shell.WindowActive)
	a.pressedCommands(c)
	if a.Palette != nil {
		a.Palette.View(c, semantic, animate)
		if command, ok := a.Palette.ConsumeChoice(); ok {
			if err := a.routeWindowCommand(a.mainWindow, command); err != nil {
				a.fail("Couldn't run the command", err)
			}
		}
	}
	a.Shell.View(c)
	// What this frame changed names its page and goes to the notebook.
	a.summarize()
	a.save()
	// The toasts stand above the foot, and over all else.
	scale := a.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
	a.Notices.View(c, semantic, animate, theme.DefaultLayout.Status*scale)
}
