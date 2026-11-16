package window

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/library"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/notices"
	"github.com/caelyreth/neverland/native/page"
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
	// Notices tells the user what happened apart from what they did, as
	// toasts at the window's foot.
	Notices *notices.Center
	// sessionsRev moves on whenever a session comes, goes or is summed up
	// anew, and as the shell is refreshed; projected keeps the shell's view
	// of the library while it and the library are unchanged. Sessions come
	// and go through putSession and dropSession.
	sessionsRev    uint64
	projected      *projections
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
	// shownPictures decodes the notebook's pictures for every page.
	shownPictures *page.Pictures
	// finder keeps the search index in step and answers searches.
	finder *finder
	// soughtPage is the page a sheet's find last ran on, 0 for none.
	soughtPage workspace.PageID
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
	if err := app.openFinder(store); err != nil {
		return nil, err
	}
	app.Settings = *settings.New()
	app.Shell = shell.New()
	app.Shell.Colors = &app.colors
	app.Settings.Colors = &app.colors
	if err := app.loadSettings(store); err != nil {
		return nil, err
	}
	app.Shell.Settings = appearanceSettings(app.Product.Appearance)
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
	a.Shell.Sheet = a.sheetView
	a.Shell.SelectPage = func(id string) {
		page, ok := pageOf(id)
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
	a.Shell.ClosePageID = a.closePageID
	a.Shell.Commands = a.commandCatalog
	a.Shell.RunCommand = func(id string) {
		if err := a.routeWindowCommand(a.mainWindow, keymap.CommandID(id)); err != nil {
			a.fail("Couldn't run the command", err)
		}
	}
	a.Shell.OpenSettings = a.OpenSettingsWindow
	a.Shell.Find = a.findQuery
	a.Shell.FindStep = a.findStep
	a.Shell.EndFind = a.endFind
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

// sheetView is an open sheet's page where the shell sets it: its editor,
// set as frame says. A page that takes the keyboard comes to the front,
// as clicking into a sheet open beside others on the desk does.
func (a *AppState) sheetView(c *ui.Context, id string, frame shell.SheetFrame) {
	session, ok := a.sessionOf(id)
	if !ok {
		return
	}
	semantic, _ := a.colors.At(a.Shell.Settings, c.Now())
	box := ui.Column(c).Key("page-" + id).Role(ui.RoleGroup).Label("Page").FillHeight().WidthPercent(100)
	box.Children(func() {
		el := a.sessionElement(session)
		if el == nil {
			return
		}
		el.Theme = semantic
		el.Prose = frame.Prose
		el.Top, el.Bottom, el.Dock = frame.Top, frame.Bottom, frame.Dock
		el.LayoutWidth, el.Inset = frame.LayoutWidth, frame.Inset
		el.Typewriter = a.Product.Presentation.Typewriter
		el.Keys = a.Product.Keymap
		el.UIFontSize = frame.UIFontSize
		// A press on what takes no focus, as a pane's background, leaves
		// the focus nowhere: typing belongs to the page then, its caret
		// shown, unless a field of the shell is taking it.
		if frame.Claim || frame.Focus && !a.Shell.FocusWithin(c) && !a.Shell.Typing() {
			el.Focus()
		}
		ui.Box(c).Key(fmt.Sprintf("editor-%d", el.Editor.View)).WidthPercent(100).Grow(1).Clip().Children(func() {
			el.View(c)
		})
	})
	if active, ok := a.Workspace.Active(); box.FocusWithin() && (!ok || active != session.page) {
		if err := a.Workspace.Open(session.page); err != nil {
			a.fail("Couldn't open the page", err)
			return
		}
		a.selectMounted()
	}
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
		Bindings:          toolbarBindings(keys, a.Product.Program.Catalog()),
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
	a.Shell.Search()
}

func (a *AppState) selectMounted() {
	a.captureViews()
	a.refreshShell()
	a.syncMenu()
}

// newSheet makes a sheet in the group the library shows, in front.
func (a *AppState) newSheet() { a.NewSheet(a.Shell.CurrentGroup(), shell.Last) }

// closeActive closes the sheet in front, as the shell closes one: the one
// beside it comes in front.
func (a *AppState) closeActive() {
	if id, ok := a.Workspace.Active(); ok {
		a.Shell.CloseSheet(sheetName(id))
	}
}

func (a *AppState) closePageID(id string) {
	page, ok := pageOf(id)
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

func (a *AppState) refreshShell() {
	a.sessionsRev++
	a.Shell.Open = nil
	for _, page := range a.Workspace.OpenedPages() {
		a.Shell.Open = append(a.Shell.Open, shell.Page{
			ID:    sheetName(page.ID),
			Title: page.Title,
		})
	}
	a.Shell.ActivePage = ""
	if id, ok := a.Workspace.Active(); ok {
		a.Shell.ActivePage = sheetName(id)
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

// View composes the shell and the notices.
func (a *AppState) View(c *ui.Context) {
	if a == nil || a.Shell == nil {
		return
	}
	a.refreshEditorChrome()
	semantic, _ := a.colors.At(a.Shell.Settings, c.Now())
	animate := components.MotionEnabled(c, a.Shell.Appearance.Motion, a.Shell.WindowActive)
	a.pressedCommands(c)
	a.Shell.View(c)
	// An open find follows the page in front.
	a.followFind()
	// What this frame changed names its page and goes to the notebook.
	a.summarize()
	a.save()
	a.reindex()
	// The toasts stand above the foot, and over all else.
	scale := a.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
	a.Notices.View(c, semantic, animate, theme.DefaultLayout.Status*scale)
}
