package window

import (
	"errors"
	"fmt"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/library"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/documentview"
	"github.com/caelyreth/neverland/native/notices"
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
	Workspace *workspace.Workspace
	Store     *appsettings.HostStore
	Settings  settings.State
	Product   appsettings.State
	Shell     *shell.State
	Palette   *overlay.CommandPalette
	// AltText asks for a picture's words, and Viewer shows a picture over
	// the window.
	AltText *overlay.AltText
	Viewer  *overlay.Viewer
	Element *documentview.Element
	Bridge  *documentview.Bridge
	Session *sheetSession
	// altTextTarget binds the open description dialog to the image and page
	// that opened it.
	altTextTarget *altTextTarget
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
	}
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
	policy, err := appsettings.NewHostStoreWithPublisher(catalog, pages.PublishPolicy)
	if err != nil {
		return nil, err
	}
	app.Workspace = pages
	app.Store = policy
	app.Settings = *settings.New()
	app.Product = appsettings.DefaultState()
	app.Shell = shell.New()
	app.Session = session
	app.Palette = &overlay.CommandPalette{}
	app.AltText = &overlay.AltText{}
	app.Viewer = &overlay.Viewer{}
	app.Element = documentview.NewElement(
		nil,
		theme.DefaultSettings().Semantic(),
		theme.DefaultSettings().Prose(),
		time.Now(),
	)
	app.Element.Label = "Writing document"
	app.Element.Pictures = &documentview.Pictures{Read: app.readPicture}
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
		a.Element.Animate = components.MotionEnabled(c, a.Shell.Appearance.Motion, a.Shell.WindowActive)
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
		a.Palette.Reset(a.commands())
	}
	a.Shell.OpenSettings = func() {
		a.OpenSettingsWindow()
	}
	a.Shell.FindPage = func() {
		a.findPage()
	}
	a.Shell.FormatItalic = func(bool) {
		a.executeEditorCommand(editor.EmphasisCommandID)
	}
	a.Shell.FormatBold = func(bool) {
		a.executeEditorCommand(editor.StrongCommandID)
	}
	a.Shell.Undo = func() {
		a.executeEditorCommand(editor.UndoCommandID)
	}
	a.Shell.Redo = func() {
		a.executeEditorCommand(editor.RedoCommandID)
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
	if a == nil || a.Shell == nil || a.Store == nil {
		return
	}
	prepared := a.Store.PreparedConfiguration()
	enabled := func(command kernel.CommandID) bool {
		action, ok := keymap.ResolveCommand(command)
		return ok && a.commandEnabledWith(prepared, action)
	}
	toolbar := shell.ToolbarState{
		Typewriter:        a.Product.Presentation.Typewriter,
		Zen:               a.Product.Presentation.Zen,
		TypewriterEnabled: enabled(editor.SurfaceTypewriterCommandID),
		ZenEnabled:        enabled(editor.SurfaceZenCommandID),
		CJKEnabled:        enabled(editor.SurfaceCJKCommandID),
	}
	toolbar.Bindings = shell.ToolbarBindings{
		Bold:       bindingLabel(prepared, editor.StrongCommandID),
		Italic:     bindingLabel(prepared, editor.EmphasisCommandID),
		Undo:       bindingLabel(prepared, editor.UndoCommandID),
		Redo:       bindingLabel(prepared, editor.RedoCommandID),
		Typewriter: bindingLabel(prepared, editor.SurfaceTypewriterCommandID),
		Zen:        bindingLabel(prepared, editor.SurfaceZenCommandID),
		CJK:        bindingLabel(prepared, editor.SurfaceCJKCommandID),
	}
	a.Shell.CanUndo, a.Shell.CanRedo = false, false
	if a.Session == nil || a.Session.Document() == nil {
		a.Shell.Toolbar = toolbar
		return
	}
	controller := a.Session.Document()
	if controller.Session() != nil {
		state := controller.Session().ToolbarState()
		toolbar.Italic, toolbar.Bold = state.Emphasis, state.Strong
		toolbar.ItalicEnabled, toolbar.BoldEnabled = state.EmphasisEnabled, state.StrongEnabled
		a.Shell.CanUndo, a.Shell.CanRedo = state.CanUndo, state.CanRedo
	}
	configuration := controller.Runtime().PolicyConfiguration()
	toolbar.CJK = configuration.IsEnabled(editor.CJKPolicyID)
	a.Shell.Toolbar = toolbar
}

func bindingLabel(configuration *kernel.PreparedPolicyConfiguration, command kernel.CommandID) string {
	return bindingLabelFor(configuration, command, keymap.PlatformCurrent)
}

func bindingLabelFor(
	configuration *kernel.PreparedPolicyConfiguration,
	command kernel.CommandID,
	platform keymap.Platform,
) string {
	if configuration == nil {
		return ""
	}
	bindings := configuration.BindingsFor(command)
	if len(bindings) == 0 {
		return ""
	}
	return keymap.Chord(bindings[0], platform)
}

// bindingKeys is the caps of the command's first binding.
func bindingKeys(configuration *kernel.PreparedPolicyConfiguration, command kernel.CommandID) []string {
	if configuration == nil {
		return nil
	}
	bindings := configuration.BindingsFor(command)
	if len(bindings) == 0 {
		return nil
	}
	return keymap.Keycaps(bindings[0], keymap.PlatformCurrent)
}

func (a *AppState) executeEditorCommand(command kernel.CommandID) {
	if a == nil {
		return
	}
	if err := a.routeWindowCommand(a.mainWindow, command); err != nil {
		a.fail("Couldn't run the command", err)
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
	session := a.Session
	a.Bridge = documentview.NewBridge(a.Session.Document(), services, time.Now)
	a.Bridge.Mounted = func() bool { return a.sessionMounted(session) }
	a.Bridge.External = a.executeExternalCommand
	a.Bridge.Images = pictureIntake{app: a}
	a.Bridge.ImageError = func(err error) { a.fail("Couldn't add the picture", err) }
	a.Bridge.Figures = figureHost{app: a}
	a.Element.Input = a.Bridge
}

func (a *AppState) sessionMounted(session *sheetSession) bool {
	if a == nil || a.Workspace == nil || session == nil ||
		a.Session != session || a.sessions[session.page] != session {
		return false
	}
	mounted, ok := a.Workspace.Mounted()
	return ok && mounted == session
}

// executeExternalCommand runs the host-owned commands documentview's surface
// boundary delegates back to the window.
func (a *AppState) executeExternalCommand(command kernel.CommandID) error {
	return a.executeHostCommand(command)
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
	var session *sheetSession
	if mounted, ok := a.Workspace.Mounted(); ok {
		session, _ = mounted.(*sheetSession)
	}
	if a.Session != session {
		a.Session = session
		a.Bridge = nil
		if a.Element != nil {
			var offset float32
			if scroll, ok := a.Workspace.ActiveScroll(); ok {
				offset = scroll.Y
			}
			a.Element.SwitchPage(offset)
		}
	}
	a.refreshShell()
	a.syncMenu()
}

func (a *AppState) closePageID(id string) {
	page, ok := parsePageID(id)
	if !ok {
		return
	}
	a.parkScroll()
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
	if a.Session != nil && a.Session.Document() != nil {
		controller := a.Session.Document()
		a.refreshShellReadouts()
		a.Shell.CanUndo = controller.Session().History().UndoDepth() > 0
		a.Shell.CanRedo = controller.Session().History().RedoDepth() > 0
	} else {
		a.Shell.Statistics = "No open document"
		a.refreshToolbarState()
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
			a.fail("Couldn't change the setting", err)
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
	change := appsettings.ToggleBoolean(appsettings.PresentationBoolean(mode), on)
	outcome, err := a.Product.Apply(change, a.Store, a.Session)
	if err != nil {
		a.fail("Couldn't change the setting", err)
		a.refreshAppearance()
		return
	}
	if !outcome.Applied {
		return
	}
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
		a.fail("Couldn't change the setting", err)
		a.refreshShell()
		return
	}
	if outcome.PolicyChanged {
		a.Bridge = nil
	}
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

// View composes the live shell, command overlay, and settings diagnostics.
func (a *AppState) View(c *ui.Context) {
	if a == nil || a.Shell == nil {
		return
	}
	a.followAccent(c.Now())
	semantic, _ := a.colors.At(a.Shell.Settings, c.Now())
	animate := components.MotionEnabled(c, a.Shell.Appearance.Motion, a.Shell.WindowActive)
	if a.Palette != nil {
		a.Palette.View(c, semantic, animate)
		if command, ok := a.Palette.ConsumeChoice(); ok {
			if err := a.routeWindowCommand(a.mainWindow, command); err != nil &&
				!errors.Is(err, keymap.ErrCommandUnavailable) {
				a.fail("Couldn't run the command", err)
			}
		}
	}
	a.AltText.View(c, semantic, animate)
	if node, alt, ok := a.AltText.ConsumeChoice(); ok {
		target := a.altTextTarget
		a.altTextTarget = nil
		if target == nil || target.node != node {
			a.fail("Couldn't change the image", documentview.ErrStaleImageOperation)
		} else if err := target.operation.DescribeImage(alt); err != nil {
			a.fail("Couldn't change the image", err)
		}
	} else if !a.AltText.Open {
		a.altTextTarget = nil
	}
	a.Viewer.View(c, semantic, animate)
	// What the shell reads off the page, its title and counts, is read
	// before the shell is built, so header, sidebar and counter show this
	// frame's page.
	a.refreshShellReadouts()
	a.Shell.View(c)
	// What this frame's input changed goes to the notebook.
	a.save()
	// The toasts stand above the foot's counter, and over all else.
	a.reportEditing()
	scale := a.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
	a.Notices.View(c, semantic, animate, theme.DefaultLayout.Status*scale)
}

func (a *AppState) commands() []overlay.Command {
	return a.commandCatalog()
}
