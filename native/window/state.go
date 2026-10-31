package window

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/library"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/documentview"
	"github.com/caelyreth/neverland/native/overlay"
	"github.com/caelyreth/neverland/native/settings"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type clock struct{}

func (clock) Now() time.Time { return time.Now() }

// AppState owns one main-window notebook and its app-wide settings.
type AppState struct {
	Workspace      *workspace.Workspace
	Store          *policyStore
	Settings       settings.State
	Product        appsettings.State
	Shell          *shell.State
	Palette        *overlay.CommandPalette
	Element        *documentview.Element
	Bridge         *documentview.Bridge
	Session        *sheetSession
	Error          string
	mainWindow     *mygo.Window
	settingsWindow *mygo.Window
	// menu is the application menu; recording stands its keys down while
	// the settings record a shortcut.
	menu       *mygo.Menu
	systemKeys map[*mygo.MenuItem]string
	recording  bool
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
	app := &AppState{sessions: map[workspace.PageID]*sheetSession{}, nextKey: 1}
	sessions, layout, err := app.openNotebook(store)
	if err != nil {
		return nil, err
	}
	pages, err := workspace.Restore(sessions, layout, app.createPage)
	if err != nil {
		return nil, err
	}
	var session *sheetSession
	if mounted, ok := pages.Mounted(); ok {
		session = mounted.(*sheetSession)
	}
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		return nil, err
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		return nil, err
	}
	app.Workspace = pages
	app.Store = newPolicyStore(catalog, host.Configuration(), host.PreparedSnapshot(), pages)
	app.Settings = *settings.New()
	app.Product = appsettings.DefaultState()
	app.Shell = shell.New()
	app.Session = session
	app.Palette = &overlay.CommandPalette{}
	app.Element = documentview.NewElement(
		nil,
		theme.DefaultSettings().Semantic(),
		theme.DefaultSettings().Prose(),
		time.Now(),
	)
	app.Element.Label = "Writing document"
	if scroll, ok := pages.ActiveScroll(); ok {
		app.Element.SwitchPage(scroll.Y)
	}
	app.Shell.Colors = &app.colors
	app.Settings.Colors = &app.colors
	app.BindShell()
	return app, nil
}

func (a *AppState) BindShell() {
	a.Shell.Content = func(c *ui.Context) {
		if a.Session == nil || a.Session.Document() == nil {
			ui.Text(c, "No page open")
			return
		}
		if a.Bridge == nil || a.Element.Input != a.Bridge {
			a.mountBridge(c)
		}
		a.Element.Theme, _ = a.colors.At(a.Shell.Settings, c.Now())
		a.Element.Prose = a.Shell.Settings.Prose()
		a.Element.Animate = a.Shell.Appearance.Motion && a.Shell.WindowActive &&
			!c.Preferences().ReduceMotion
		a.Element.Typewriter = a.Shell.Appearance.Typewriter
		// The reading space above the first line scrolls with the page.
		// The last line only needs to clear the floating dock; typewriter
		// mode's centering space is the element's own.
		a.Element.Margin = a.Shell.ColumnMargin()
		a.Element.UIFontSize = a.Shell.Settings.UIFontSize
		a.Element.TopInset = theme.DefaultLayout.ReadingTop *
			a.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
		a.Element.BottomInset = theme.DefaultLayout.DockClearance() *
			a.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
		a.refreshToolbarState()
		a.Element.View(c)
	}
	a.Shell.SelectPage = func(id string) {
		a.parkScroll()
		var page workspace.PageID
		_, _ = fmt.Sscanf(id, "%d", &page)
		if err := a.Workspace.Open(page); err != nil {
			a.Error = err.Error()
			return
		}
		a.selectMounted()
	}
	a.Shell.Library = a
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
		a.Palette.Reset(a.commands())
	}
	a.Shell.OpenSettings = func() {
		a.OpenSettingsWindow()
	}
	a.Shell.FindPage = func() {
		a.findPage()
	}
	a.Shell.FormatItalic = func(bool) {
		a.executeEditorCommand(editor.MarkupEmphasis.ID())
	}
	a.Shell.FormatBold = func(bool) {
		a.executeEditorCommand(editor.MarkupStrong.ID())
	}
	a.Shell.Undo = func() {
		a.executeEditorCommand(editor.HistoryUndo.ID())
	}
	a.Shell.Redo = func() {
		a.executeEditorCommand(editor.HistoryRedo.ID())
	}
	a.Shell.Typewriter = func(on bool) {
		a.setPresentation(appsettings.PresentationTypewriter, on)
	}
	a.Shell.Zen = func(on bool) {
		a.setPresentation(appsettings.PresentationZen, on)
	}
	a.Shell.ToggleCJK = func(on bool) {
		a.setPolicy(editor.CJKPolicyID, on)
	}
	a.refreshShell()
}

func (a *AppState) refreshShellReadouts() {
	if a == nil || a.Shell == nil || a.Session == nil || a.Session.Document() == nil {
		return
	}
	// The page is named after its first heading, so its title follows
	// what is written.
	if id, ok := a.Workspace.Active(); ok {
		title := a.Workspace.Title()
		for index := range a.Shell.Tabs {
			if a.Shell.Tabs[index].ID == fmt.Sprintf("%d", id) && a.Shell.Tabs[index].Title != title {
				a.refreshShell()
				break
			}
		}
	}
	controller := a.Session.Document()
	if err := controller.LastError(); err != nil {
		a.Shell.Status = err.Error()
	} else if a.Error == "" {
		a.Shell.Status = a.saveStatus()
	}
	metrics, ok := controller.MetricsForCurrentText()
	if !ok {
		metrics = controller.Metrics()
	}
	a.Shell.Statistics = fmt.Sprintf(
		"%d words  ·  %d paragraphs  ·  %d characters",
		metrics.Words,
		metrics.Paragraphs,
		metrics.Characters,
	)
	a.refreshToolbarState()
}

func (a *AppState) refreshToolbarState() {
	if a == nil || a.Shell == nil || a.Session == nil || a.Session.Document() == nil {
		return
	}
	controller := a.Session.Document()
	if controller.Session() != nil {
		toolbar := controller.Session().ToolbarState()
		a.Shell.Toolbar.Italic = toolbar.Emphasis
		a.Shell.Toolbar.Bold = toolbar.Strong
		a.Shell.Toolbar.ItalicEnabled = toolbar.EmphasisEnabled
		a.Shell.Toolbar.BoldEnabled = toolbar.StrongEnabled
		a.Shell.CanUndo = toolbar.CanUndo
		a.Shell.CanRedo = toolbar.CanRedo
	}
	configuration := controller.Runtime().PolicyConfiguration()
	a.Shell.Toolbar.CJK = configuration.IsEnabled(editor.CJKPolicyID)
	prepared := controller.Runtime().Configuration()
	a.Shell.Toolbar.Bindings = shell.ToolbarBindings{
		Bold:   bindingLabel(prepared, editor.MarkupStrong.ID()),
		Italic: bindingLabel(prepared, editor.MarkupEmphasis.ID()),
		Undo:   bindingLabel(prepared, editor.HistoryUndo.ID()),
		Redo:   bindingLabel(prepared, editor.HistoryRedo.ID()),
	}
}

func bindingLabel(configuration *kernel.PreparedPolicyConfiguration, command kernel.CommandID) string {
	if configuration == nil {
		return ""
	}
	bindings := configuration.BindingsFor(command)
	if len(bindings) == 0 {
		return ""
	}
	return keymap.Chord(bindings[0], keymap.PlatformCurrent)
}

func (a *AppState) executeEditorCommand(command kernel.CommandID) {
	if a == nil || a.Bridge == nil {
		return
	}
	action, ok := keymap.ResolveEditorCommand(command)
	if !ok {
		return
	}
	if err := a.Bridge.Execute(action); err != nil {
		a.Error = err.Error()
		a.refreshShell()
	}
}

func (a *AppState) ProductState() appsettings.State { return a.Product }

func (a *AppState) SettingsStore() appsettings.Store { return a.Store }

func (a *AppState) ActiveSession() appsettings.Session {
	if a.Session == nil {
		return nil
	}
	return a.Session
}

func (a *AppState) mountBridge(c *ui.Context) {
	services := mygoServices{services: c.Services()}
	a.Bridge = documentview.NewBridge(a.Session.Document(), services, time.Now)
	a.Bridge.External = a.executeExternalCommand
	a.Element.Input = a.Bridge
}

// executeExternalCommand routes policy-owned host commands through the same
// callbacks used by the visible shell controls.
func (a *AppState) executeExternalCommand(command kernel.CommandID) error {
	if a == nil {
		return fmt.Errorf("window: app state is unavailable")
	}
	switch command {
	case "workspace.new", editor.SurfaceNewPageCommandID:
		if a.Shell.NewPage != nil {
			a.Shell.NewPage()
		} else {
			a.newPage()
		}
	case editor.SurfaceNextPageCommandID:
		a.navigatePage(true)
	case editor.SurfacePreviousPageCommandID:
		a.navigatePage(false)
	case editor.SurfaceClosePageCommandID:
		if a.Shell.ClosePage != nil {
			a.Shell.ClosePage()
		} else {
			a.closePage()
		}
	case editor.SurfaceZenCommandID:
		a.togglePresentation(appsettings.PresentationZen, a.Shell.Zen)
	case editor.SurfaceTypewriterCommandID:
		a.togglePresentation(appsettings.PresentationTypewriter, a.Shell.Typewriter)
	case editor.SurfaceMotionCommandID:
		a.setPresentation(
			appsettings.PresentationMotion,
			!a.Product.Presentation.Motion,
		)
	case editor.SurfaceCJKCommandID:
		configuration := a.Session.Document().Runtime().PolicyConfiguration()
		on := !configuration.IsEnabled(editor.CJKPolicyID)
		if a.Shell.ToggleCJK != nil {
			a.Shell.ToggleCJK(on)
		} else {
			a.setPolicy(editor.CJKPolicyID, on)
		}
	case editor.SurfaceCommandsCommandID:
		if a.Shell.OpenCommands != nil {
			a.Shell.OpenCommands()
		} else {
			a.openCommands()
		}
	case editor.SurfaceFindPageCommandID:
		if a.Shell.FindPage != nil {
			a.Shell.FindPage()
		} else {
			a.findPage()
		}
	case editor.SurfaceSidebarCommandID:
		a.toggleSidebar()
	case editor.SurfaceSheetListCommandID:
		a.toggleSheetList()
	case editor.SurfaceLibraryViewCommandID:
		a.showPanes(true, true)
	case editor.SurfaceSheetsViewCommandID:
		a.showPanes(false, true)
	case editor.SurfaceEditorViewCommandID:
		a.showPanes(false, false)
	case editor.SurfaceSettingsCommandID:
		if a.Shell.OpenSettings != nil {
			a.Shell.OpenSettings()
		} else {
			a.OpenSettingsWindow()
		}
	default:
		return nil
	}
	if a.Error != "" {
		return fmt.Errorf("%s", a.Error)
	}
	return nil
}

func (a *AppState) togglePresentation(
	mode appsettings.Presentation,
	callback func(bool),
) {
	on := !a.Product.Presentation.Get(mode)
	if callback != nil {
		callback(on)
		return
	}
	a.setPresentation(mode, on)
}

func (a *AppState) navigatePage(forward bool) {
	a.parkScroll()
	if _, ok, err := a.Workspace.Navigate(forward); err != nil {
		a.Error = err.Error()
		a.refreshShell()
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
	if mounted, ok := a.Workspace.Mounted(); ok {
		if session, ok := mounted.(*sheetSession); ok {
			a.Session = session
			a.Bridge = nil
			if scroll, ok := a.Workspace.ActiveScroll(); ok && a.Element != nil {
				a.Element.SwitchPage(scroll.Y)
			}
		}
	} else {
		a.Session = nil
		a.Bridge = nil
		if a.Element != nil {
			a.Element.Input = nil
		}
	}
	a.refreshShell()
}

func (a *AppState) closePageID(id string) {
	page, ok := parsePageID(id)
	if !ok {
		return
	}
	a.parkScroll()
	if err := a.Workspace.Close(page); err != nil {
		a.Error = err.Error()
		a.refreshShell()
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

func (a *AppState) parkScroll() {
	if a == nil || a.Workspace == nil || a.Element == nil {
		return
	}
	scroll, _ := a.Workspace.ActiveScroll()
	scroll.Y = a.Element.ScrollY()
	a.Workspace.SetActiveScroll(scroll)
}

func (a *AppState) refreshShell() {
	a.Shell.Tabs = nil
	for _, page := range a.Workspace.TabPages() {
		a.Shell.Tabs = append(a.Shell.Tabs, shell.Page{
			ID:    fmt.Sprintf("%d", page.ID),
			Title: page.Title,
		})
	}
	if id, ok := a.Workspace.Active(); ok {
		a.Shell.ActivePage = fmt.Sprintf("%d", id)
	} else {
		a.Shell.ActivePage = ""
	}
	a.Shell.Status = a.saveStatus()
	if a.Error != "" {
		a.Shell.Status = a.Error
	}
	if a.Session != nil && a.Session.Document() != nil {
		controller := a.Session.Document()
		a.refreshShellReadouts()
		a.Shell.CanUndo = controller.Session().History().UndoDepth() > 0
		a.Shell.CanRedo = controller.Session().History().RedoDepth() > 0
	} else {
		a.Shell.Statistics = "No open document"
	}
}

func (a *AppState) refreshSettingsData() {
	if a.Session == nil || a.Session.Document() == nil {
		a.Settings.Data = appsettings.Data{}
		return
	}
	a.Settings.Data = a.Product.Data(a.Store, a.Session)
	a.Settings.Theme = a.Shell.Settings
	a.Settings.Animate = a.Product.Presentation.Motion
	a.Settings.Recording = a.setRecording
	a.Settings.Reserved = a.reservedKey
	a.Settings.Apply = func(change appsettings.Change) {
		outcome, err := a.Product.Apply(change, a.Store, a.Session)
		if err != nil {
			a.Error = err.Error()
			return
		}
		if outcome.PolicyChanged {
			a.Bridge = nil
			a.syncMenu()
		}
		if change.Kind == appsettings.ChangeAppearance {
			a.applyAppearance()
		}
		if change.Boolean.Kind == appsettings.BooleanPresentation {
			a.refreshAppearance()
		}
		a.refreshSettingsData()
	}
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
	change := appsettings.ToggleBoolean(appsettings.PresentationBoolean(mode), on)
	outcome, err := a.Product.Apply(change, a.Store, a.Session)
	if err != nil {
		a.Error = err.Error()
		a.refreshAppearance()
		return
	}
	if !outcome.Applied {
		return
	}
	a.Error = ""
	a.refreshAppearance()
	a.refreshSettingsData()
}

func (a *AppState) setPolicy(policy kernel.PolicyID, on bool) {
	if a == nil {
		return
	}
	change := appsettings.ToggleBoolean(appsettings.PolicyBoolean(policy), on)
	outcome, err := a.Product.Apply(change, a.Store, a.Session)
	if err != nil {
		a.Error = err.Error()
		a.refreshShell()
		return
	}
	if outcome.PolicyChanged {
		a.Bridge = nil
	}
	a.Error = ""
	a.refreshShell()
	a.refreshSettingsData()
}

func (a *AppState) toggleZen() {
	if a == nil {
		return
	}
	a.setPresentation(appsettings.PresentationZen, !a.Product.Presentation.Zen)
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
	settings.Accent = theme.Accent(values.AccentNumber() - 1)
	return settings
}

// View composes the live shell, command overlay, and settings diagnostics.
func (a *AppState) View(c *ui.Context) {
	if a == nil || a.Shell == nil {
		return
	}
	semantic, _ := a.colors.At(a.Shell.Settings, c.Now())
	animate := a.Shell.Appearance.Motion && a.Shell.WindowActive &&
		!c.Preferences().ReduceMotion
	if a.Palette != nil {
		a.Palette.View(c, semantic, animate)
		if command, ok := a.Palette.ConsumeChoice(); ok && a.Bridge != nil {
			if action, known := keymap.ResolveEditorCommand(command); known {
				_ = a.Bridge.Execute(action)
			} else {
				_ = a.Bridge.ExecuteExternal(command)
			}
		}
	}
	// What the shell reads off the page, its title and counts, is read
	// before the shell is built, so header, sidebar and status show this
	// frame's page.
	a.refreshShellReadouts()
	a.Shell.View(c)
	// What this frame's input changed goes to the notebook.
	a.save()
}

func (a *AppState) commands() []overlay.Command {
	return a.commandCatalog()
}
