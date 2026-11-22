package window

import (
	"fmt"
	"slices"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/library"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/native/components"
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
	Notices *NoticeCenter
	// sessionsRev moves on whenever a session comes, goes or is summed up
	// anew, and as the shell is refreshed; projected keeps the shell's view
	// of the library while it and the library are unchanged. Sessions come
	// and go through putSession and dropSession.
	sessionsRev uint64
	projected   *projections
	mainWindow  *mygo.Window
	// menu is the application menu; recording stands its keys down while
	// the settings record a shortcut.
	menu       *mygo.Menu
	systemKeys map[*mygo.MenuItem]string
	// menuItems are the menu's items by the command each runs,
	// formatItems the Format menu's, and menuShown what each was last
	// given (syncMenu).
	menuItems         map[keymap.CommandID]*mygo.MenuItem
	formatItems       []formatItem
	insertPictureItem *mygo.MenuItem
	menuShown         map[*mygo.MenuItem]menuShown
	// fresh are the sheets made this run, which go if closed empty.
	fresh map[workspace.PageID]bool
	// undoItem and redoItem are Edit's Undo and Redo for the field.
	undoItem, redoItem *mygo.MenuItem
	// undoField is whether Undo and Redo were last the field's.
	undoField *bool
	recording bool
	// colors carries every window through a theme or accent change at once.
	colors theme.Transition
	// fonts are the faces of other scripts downloaded.
	fonts *fontState
	// sessions are the pages' sessions by identity, which the notebook
	// saves.
	sessions map[workspace.PageID]*sheetSession
	// notebook keeps the pages on disk; nil holds them in memory alone.
	notebook *notebook
	// library files the pages as the field shows them.
	library *library.Library
	// nextKey is the notebook key the next new page takes.
	nextKey storage.PageKey
	// written is the words written each day.
	written written
	// nextView numbers the editors' views.
	nextView uint64
	// chrome is the editor state the footer and formatting toolbar last
	// showed.
	chrome chromeKey
	// settingsDirty waits for the next write of the settings record.
	settingsDirty bool
	// shownPictures decodes the notebook's pictures for every page, and
	// cardPictures them small for the cards in the field.
	shownPictures, cardPictures *page.Pictures
	// finder keeps the search index in step and answers searches.
	finder *finder
	// sought is the column a sheet's find last ran in, while seeking.
	sought  workspace.ColumnID
	seeking bool
	// shortcutMap caches globalShortcut resolution while Product.Keymap is unchanged.
	shortcutMap *keymap.Keymap
	shortcuts   []globalShortcut
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
		Notices:  &NoticeCenter{},
		sessions: map[workspace.PageID]*sheetSession{},
		nextKey:  1,
		Product:  appsettings.DefaultState(),
		fonts:    openFonts(""),
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
	if err := app.loadSettings(store); err != nil {
		return nil, err
	}
	app.Shell.Settings = app.appearanceSettings()
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
		a.Notices.Post(Notice{
			ID: "settings.load", Kind: NoticeWarning, Title: "Settings were reset",
			Description: "Your notebook is unchanged. The saved preferences could not be read.",
		})
	}
	return nil
}

func (a *AppState) BindShell() {
	a.Shell.Sheet = a.sheetView
	a.Shell.SelectColumn = func(name string) {
		col, ok := columnOf(name)
		if !ok {
			return
		}
		if err := a.Workspace.Focus(col); err != nil {
			a.fail("Couldn't go to the sheet", err)
			return
		}
		a.selectMounted()
	}
	a.Shell.Library = a
	a.Shell.Thumbnail = a.thumbnail
	a.Shell.CloseColumnID = a.closeColumn
	a.Shell.SplitColumnID = a.splitColumn
	a.Shell.Commands = a.commandCatalog
	a.Shell.Binding = func(id string) string { return bindingLabel(a.Product.Keymap, keymap.CommandID(id)) }
	a.Shell.Wrote = func() time.Time {
		var last time.Time
		for _, session := range a.sessions {
			for _, el := range session.columns {
				if at := el.Editor.Wrote(); at.After(last) {
					last = at
				}
			}
		}
		return last
	}
	a.Shell.WrittenToday = func() int { return a.written.on(time.Now()) }
	a.Shell.KeyOf = func(id string) (ui.Modifiers, ui.Key, bool) {
		chord, ok := a.Product.Keymap.ChordFor(keymap.CommandID(id))
		if !ok {
			return 0, 0, false
		}
		return keymap.Native(chord, keymap.PlatformCurrent)
	}
	a.Shell.RunCommand = func(id string) {
		if err := a.routeCommand(keymap.CommandID(id)); err != nil {
			a.fail("Couldn't run the command", err)
		}
	}
	a.Shell.AppSettings = fieldSettings{a}
	a.Settings.Find = a.Shell.FindSettings
	a.refreshSettingsData()
	a.Shell.Find = a.findQuery
	a.Shell.FindStep = a.findStep
	a.Shell.EndFind = a.endFind
	a.Shell.Typewriter = func(on bool) {
		a.setPresentation(appsettings.PresentationTypewriter, on)
	}
	a.refreshShell()
}

// sheetView is an open column's page where the shell sets it: its editor
// in the column, set as frame says. A column whose page takes the
// keyboard comes to the front, as clicking into a sheet open beside
// others on the desk does.
func (a *AppState) sheetView(c *ui.Context, open shell.Page, frame shell.SheetFrame) {
	session, ok := a.sessionOf(open.ID)
	col, isColumn := columnOf(open.Column)
	if !ok || !isColumn {
		return
	}
	semantic, moving := a.sheetTheme(session, open.ID, c.Now())
	if moving {
		c.AnimationFrame()
	}
	box := ui.Column(c).Key("page-" + open.Column).Role(ui.RoleGroup).Label("Page").FillHeight().WidthPercent(100)
	box.Children(func() {
		el := a.columnElement(session, col)
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
	if front, ok := a.Workspace.Front(); box.FocusWithin() && (!ok || front != col) {
		if err := a.Workspace.Focus(col); err != nil {
			a.fail("Couldn't go to the sheet", err)
			return
		}
		a.selectMounted()
	}
}

func (a *AppState) refreshToolbarState() {
	if a == nil || a.Shell == nil {
		return
	}
	a.Shell.Toolbar = shell.ToolbarState{
		Typewriter:        a.Product.Presentation.Typewriter,
		TypewriterEnabled: true,
		Bindings:          shell.ToolbarBindings{Typewriter: bindingLabel(a.Product.Keymap, keymap.Typewriter)},
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
	if _, ok := a.Workspace.Navigate(forward); ok {
		a.selectMounted()
	}
}

func (a *AppState) findPage() {
	if a == nil || a.Shell == nil {
		return
	}
	a.Shell.Search()
}

func (a *AppState) selectMounted() {
	a.captureViews()
	a.refreshShell()
	a.syncMenu()
}

// newSheet makes a sheet in the group the library shows, in front.
func (a *AppState) newSheet() { a.NewSheet(a.Shell.CurrentGroup(), shell.Last) }

// closeActive closes the column in front, as the shell closes one: the
// one beside it comes in front.
func (a *AppState) closeActive() {
	if col, ok := a.Workspace.Front(); ok {
		a.Shell.CloseColumn(columnName(col))
	}
}

// splitActive opens the sheet in front again beside it.
func (a *AppState) splitActive() {
	if col, ok := a.Workspace.Front(); ok {
		a.splitColumn(columnName(col))
	}
}

// closeColumn closes a column, and its sheet with its last.
func (a *AppState) closeColumn(name string) {
	col, ok := columnOf(name)
	if !ok {
		return
	}
	page, ok := a.Workspace.ColumnPage(col)
	if !ok {
		return
	}
	a.captureViews()
	if err := a.Workspace.Close(col); err != nil {
		a.fail("Couldn't close the sheet", err)
		return
	}
	if !slices.Contains(a.Workspace.Opened(), page) {
		// A sheet made this run and closed with nothing in it was never
		// written: it goes, as a new note left empty does, rather than
		// piling up as Untitled.
		discard := a.fresh[page] && a.emptyPage(page)
		delete(a.fresh, page)
		if session, ok := a.sessions[page]; discard && ok && a.library.TrashSheet(library.SheetID(session.key), time.Now()) == nil {
			a.DeleteSheet(sheetName(page))
		}
	}
	a.selectMounted()
}

// splitColumn opens a column's sheet again beside it, where it stands in
// the column, and brings it in front.
func (a *AppState) splitColumn(name string) {
	col, ok := columnOf(name)
	if !ok {
		return
	}
	a.captureViews()
	if _, err := a.Workspace.Split(col); err != nil {
		a.fail("Couldn't split the sheet", err)
		return
	}
	a.selectMounted()
}

// emptyPage reports whether the page holds nothing: one empty block.
func (a *AppState) emptyPage(page workspace.PageID) bool {
	session, ok := a.sessions[page]
	if !ok {
		return false
	}
	snap := session.text()
	return snap != nil && snap.Len() <= 2 && storage.Summarize(snap.Root()).Words == 0
}

func (a *AppState) refreshShell() {
	a.sessionsRev++
	a.Shell.Open = nil
	for _, col := range a.Workspace.Columns() {
		a.Shell.Open = append(a.Shell.Open, shell.Page{
			ID:     sheetName(col.Page.ID),
			Column: columnName(col.ID),
			Title:  col.Page.Title,
		})
	}
	a.Shell.ActivePage, a.Shell.ActiveColumn = "", ""
	if col, ok := a.Workspace.Front(); ok {
		page, _ := a.Workspace.ColumnPage(col)
		a.Shell.ActivePage, a.Shell.ActiveColumn = sheetName(page), columnName(col)
	}
	a.closeColumns()
	a.refreshEditorChrome()
	a.refreshToolbarState()
}

func (a *AppState) refreshSettingsData() {
	a.Settings.Data = a.Product.Data()
	a.Settings.Theme = a.Shell.Settings
	a.Settings.Animate = a.Product.Presentation.Motion
	a.Settings.Recording = a.setRecording
	a.Settings.Reserved = a.reservedKey
	a.Settings.Fonts, a.Settings.HanFace, a.Settings.Font = a.fonts.rows(), a.Product.Appearance.HanFace, a.fontAct
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
	a.Shell.Settings = a.appearanceSettings()
	target := a.Shell.Settings.Semantic()
	if a.Product.Presentation.Motion {
		a.colors.Retarget(target, time.Now())
	} else {
		a.colors.Jump(target)
	}
	mygo.Theme.SetSource(themeSource(a.Shell.Settings))
	if window := a.mainWindow; window != nil && !window.IsDestroyed() {
		_ = window.SetBackgroundColor(colorHex(target.Surface.Background))
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

// appearanceSettings is the display the settings choose, the prose in the
// faces kept.
func (a *AppState) appearanceSettings() theme.Settings {
	values := a.Product.Appearance
	settings := theme.DefaultSettings()
	settings.ProseFamily = a.fonts.family(values.HanFace)
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
	animate := components.MotionEnabled(c, a.Shell.Appearance.Motion)
	a.pressedCommands(c)
	a.Shell.View(c)
	// The menu follows the scene in front and what it holds.
	a.syncMenu()
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
