package window

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/documentview"
	"github.com/caelyreth/neverland/native/overlay"
	"github.com/caelyreth/neverland/native/settings"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type clock struct{}

func (clock) Now() time.Time { return time.Now() }

// AppState owns one main-window notebook and its app-wide settings.
type AppState struct {
	Workspace *workspace.Workspace
	Store     *policyStore
	Settings  settings.State
	Product   appsettings.State
	Shell     *shell.State
	Palette   *overlay.CommandPalette
	Element   *documentview.Element
	Bridge    *documentview.Bridge
	Session   *sheetSession
	Error     string
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
	app.Element = documentview.NewElement(
		nil,
		theme.DefaultSettings().Semantic(),
		theme.NewProseStyle(theme.DefaultSettings().ProseFontSize),
		time.Now(),
	)
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
		a.Element.Theme = a.Shell.Settings.Semantic()
		a.Element.Prose = theme.NewProseStyle(a.Shell.Settings.ProseFontSize)
		a.Element.Animate = a.Shell.Appearance.Motion
		a.Element.View(c)
	}
	a.Shell.SelectPage = func(id string) {
		var page workspace.PageID
		_, _ = fmt.Sscanf(id, "%d", &page)
		if err := a.Workspace.Open(page); err != nil {
			a.Error = err.Error()
			return
		}
		a.selectMounted()
	}
	a.Shell.NewPage = func() {
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
		if err := a.Workspace.Close(id); err != nil {
			a.Error = err.Error()
			return
		}
		a.selectMounted()
	}
	a.Shell.OpenCommands = func() {
		a.Palette.Open = true
		a.Palette.Reset(a.commands())
	}
	a.Shell.OpenSettings = func() {
		a.refreshSettingsData()
	}
	a.Shell.ToggleSidebar = func() {
		a.Shell.Appearance.Zen = !a.Shell.Appearance.Zen
	}
	a.refreshShell()
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
	a.Bridge.External = func(command kernel.CommandID) error {
		switch string(command) {
		case "workspace.new":
			_, err := a.Workspace.NewPage()
			return err
		default:
			return nil
		}
	}
	a.Element.Input = a.Bridge
}

func (a *AppState) selectMounted() {
	if mounted, ok := a.Workspace.Mounted(); ok {
		if session, ok := mounted.(*sheetSession); ok {
			a.Session = session
			a.Bridge = nil
		}
	}
	a.refreshShell()
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
	a.Shell.Status = "Ready"
	if a.Error != "" {
		a.Shell.Status = a.Error
	}
	if a.Session != nil && a.Session.Document() != nil {
		metrics := a.Session.Document().Metrics()
		a.Shell.Statistics = fmt.Sprintf("%d words · %d characters", metrics.Words, metrics.Characters)
	}
}

func (a *AppState) refreshSettingsData() {
	if a.Session == nil || a.Session.Document() == nil {
		a.Settings.Data = appsettings.Data{}
		return
	}
	a.Settings.Data = a.Product.Data(a.Store, a.Session)
	a.Settings.Theme = a.Shell.Settings
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

func (a *AppState) applyAppearance() {
	values := a.Product.Appearance
	a.Shell.Settings = appearanceSettings(values)
	a.refreshAppearance()
}

func (a *AppState) refreshAppearance() {
	a.Shell.Appearance = a.Product.Presentation
	a.refreshShell()
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
	if a.Palette != nil {
		a.Palette.View(c)
		if command, ok := a.Palette.ConsumeChoice(); ok && a.Bridge != nil {
			_ = a.Bridge.ExecuteExternal(command)
		}
	}
	a.Shell.View(c)
}

func (a *AppState) commands() []overlay.Command {
	commands := make([]overlay.Command, 0)
	for _, command := range editor.AllTextCommands {
		meta := command.Description()
		commands = append(commands, overlay.Command{ID: meta.ID, Label: meta.Label, Binding: meta.Binding().String()})
	}
	return commands
}
