package window

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
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
	PageTitle      *overlay.PageTitle
	Element        *documentview.Element
	Bridge         *documentview.Bridge
	Session        *sheetSession
	Error          string
	mainWindow     *mygo.Window
	settingsWindow *mygo.Window
	// colors carries every window through a theme or accent change at once.
	colors theme.Transition
}

func NewAppState() (*AppState, error) {
	initial, err := controller.NewTextDocument("", clock{})
	if err != nil {
		return nil, err
	}
	session := &sheetSession{
		page:       0,
		controller: initial,
		title:      workspace.DefaultTitle,
	}
	pages, err := workspace.New(session, func(
		id workspace.PageID,
		configuration kernel.PolicyConfiguration,
	) (workspace.Session, error) {
		document, err := controller.NewTextDocument("", clock{})
		if err != nil {
			return nil, err
		}
		if err := document.Adopt(configuration); err != nil {
			return nil, err
		}
		return &sheetSession{
			page:       id,
			controller: document,
			title:      workspace.DefaultTitle,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		return nil, err
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		return nil, err
	}
	app := &AppState{
		Workspace: pages,
		Store:     newPolicyStore(catalog, host.Configuration(), host.PreparedSnapshot(), pages),
		Settings:  *settings.New(),
		Product:   appsettings.DefaultState(),
		Shell:     shell.New(),
		Session:   session,
	}
	app.Palette = &overlay.CommandPalette{}
	app.PageTitle = &overlay.PageTitle{
		Confirm: func(id, title string) {
			page, ok := parsePageID(id)
			if !ok {
				return
			}
			app.Workspace.Rename(page, title)
			app.refreshShell()
			app.refreshSettingsData()
		},
	}
	app.Element = documentview.NewElement(
		nil,
		theme.DefaultSettings().Semantic(),
		theme.NewProseStyle(theme.DefaultSettings().ProseFontSize),
		time.Now(),
	)
	app.Element.Label = "Writing document"
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
		a.refreshShellReadouts()
		a.Element.Theme, _ = a.colors.At(a.Shell.Settings, c.Now())
		a.Element.Prose = theme.NewProseStyle(a.Shell.Settings.ProseFontSize)
		a.Element.Animate = a.Shell.Appearance.Motion && a.Shell.WindowActive &&
			!c.Preferences().ReduceMotion
		a.Element.Typewriter = a.Shell.Appearance.Typewriter
		a.Element.Title = a.Workspace.Title()
		a.Element.UIFontSize = a.Shell.Settings.UIFontSize
		// The last line only needs to clear the floating dock; typewriter
		// mode's centering space is the element's own.
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
	a.Shell.NewPage = func() {
		a.parkScroll()
		if _, err := a.Workspace.NewPage(); err != nil {
			a.Error = err.Error()
			return
		}
		a.selectMounted()
	}
	a.Shell.ClosePage = func() {
		id, ok := a.Workspace.Active()
		if !ok {
			return
		}
		a.closePageID(fmt.Sprintf("%d", id))
	}
	a.Shell.ClosePageID = a.closePageID
	a.Shell.RenamePage = func(id string) {
		page, ok := parsePageID(id)
		if !ok {
			return
		}
		value, ok := a.Workspace.Page(page)
		if !ok {
			return
		}
		a.PageTitle.Cancel = func(string) {
			a.refreshShell()
		}
		a.PageTitle.Begin(id, value.Title)
	}
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
	controller := a.Session.Document()
	if err := controller.LastError(); err != nil {
		a.Shell.Status = err.Error()
	} else if a.Error == "" {
		a.Shell.Status = ""
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
	case editor.SurfaceRenamePageCommandID:
		a.renameActivePage()
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

func (a *AppState) renameActivePage() {
	id, ok := a.Workspace.Active()
	if !ok || a.Shell.RenamePage == nil {
		return
	}
	a.Shell.RenamePage(fmt.Sprintf("%d", id))
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
	a.Shell.FindPageRequested = true
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
	if a.PageTitle != nil {
		a.PageTitle.Open = false
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
	a.Shell.Pages = nil
	for _, page := range a.Workspace.Pages() {
		a.Shell.Pages = append(a.Shell.Pages, shell.Page{
			ID:    fmt.Sprintf("%d", page.ID),
			Title: page.Title,
		})
	}
	if id, ok := a.Workspace.Active(); ok {
		a.Shell.ActivePage = fmt.Sprintf("%d", id)
	} else {
		a.Shell.ActivePage = ""
	}
	a.Shell.Status = ""
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
	a.Settings.Apply = func(change appsettings.Change) {
		outcome, err := a.Product.Apply(change, a.Store, a.Session)
		if err != nil {
			a.Error = err.Error()
			return
		}
		if outcome.PolicyChanged {
			a.Bridge = nil
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
	a.Shell.View(c)
	if a.PageTitle != nil {
		a.PageTitle.View(c, semantic, animate)
	}
}

func (a *AppState) commands() []overlay.Command {
	return commandCatalog()
}
