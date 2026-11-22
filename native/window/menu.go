package window

import (
	"strings"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
)

// MainMenu builds the application menu from the keymap's commands: every
// verb the app has stands in it, under its name, with its key, enabled
// only where it acts on what is in front (commandState), so the menu bar
// teaches the app and Help's search finds any verb. Edit's text items
// keep MyGo's roles, so text fields anywhere still cut, copy and paste.
func (a *AppState) MainMenu() *mygo.Menu {
	a.menuItems = map[keymap.CommandID]*mygo.MenuItem{}
	a.menuShown = map[*mygo.MenuItem]menuShown{}
	a.offerItems = nil
	a.undoField = nil
	command := func(id keymap.CommandID) *mygo.MenuItem {
		declared, _ := a.Product.Keymap.Command(id)
		item := &mygo.MenuItem{ID: string(id), Label: declared.Label, Click: a.menuClick(func() {
			if err := a.routeCommand(id); err != nil {
				a.fail(locales.CouldntRunCommand(), err)
			}
		})}
		switch id {
		case keymap.Typewriter, keymap.Motion, keymap.ShowTrash, keymap.ViewList:
			item.Type = mygo.MenuItemCheckbox
		}
		a.menuItems[id] = item
		return item
	}
	ellipsis := func(item *mygo.MenuItem) *mygo.MenuItem {
		item.Label += "…"
		return item
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{
			Role: mygo.RoleAppMenu,
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				mygo.Separator(),
				ellipsis(command(keymap.Settings)),
				mygo.Separator(),
				{Role: mygo.RoleServices},
				mygo.Separator(),
				{Role: mygo.RoleHide},
				{Role: mygo.RoleHideOthers},
				{Role: mygo.RoleUnhide},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			},
		},
		{
			Label: locales.MenuFile(),
			Submenu: []*mygo.MenuItem{
				command(keymap.NewPage),
				command(keymap.NewGroup),
				mygo.Separator(),
				ellipsis(command(keymap.FindPage)),
				mygo.Separator(),
				command(keymap.Duplicate),
				ellipsis(command(keymap.Rename)),
				command(keymap.Pin),
				mygo.Separator(),
				command(keymap.SplitPage),
				mygo.Separator(),
				command(keymap.ClosePage),
				command(keymap.CloseOthers),
				mygo.Separator(),
				command(keymap.Trash),
				ellipsis(command(keymap.EmptyTrash)),
			},
		},
		{
			Label: locales.MenuEdit(),
			Submenu: []*mygo.MenuItem{
				a.fieldUndo(),
				a.fieldRedo(),
				{Role: mygo.RoleUndo},
				{Role: mygo.RoleRedo},
				mygo.Separator(),
				{Role: mygo.RoleCut},
				{Role: mygo.RoleCopy},
				{Role: mygo.RolePaste},
				{Role: mygo.RolePasteAndMatchStyle},
				{Role: mygo.RoleDelete},
				{Role: mygo.RoleSelectAll},
				mygo.Separator(),
				ellipsis(command(keymap.FindInSheet)),
			},
		},
		{
			Label:   locales.MenuFormat(),
			Submenu: append(a.offeredItems(), command(keymap.FormatTools)),
		},
		{
			Label: locales.MenuView(),
			Submenu: []*mygo.MenuItem{
				command(keymap.Field),
				command(keymap.ViewList),
				mygo.Separator(),
				command(keymap.Typewriter),
				command(keymap.Motion),
				mygo.Separator(),
				ellipsis(command(keymap.Commands)),
				mygo.Separator(),
				{Role: mygo.RoleToggleFullScreen},
			},
		},
		{
			Label: locales.MenuGo(),
			Submenu: []*mygo.MenuItem{
				command(keymap.Back),
				command(keymap.Forward),
				mygo.Separator(),
				command(keymap.Home),
				command(keymap.ShowTrash),
				mygo.Separator(),
				command(keymap.NextPage),
				command(keymap.PreviousPage),
			},
		},
		{Role: mygo.RoleWindowMenu},
		{
			Role:    mygo.RoleHelp,
			Label:   locales.MenuHelp(),
			Submenu: []*mygo.MenuItem{command(keymap.Shortcuts)},
		},
	})
	a.menu = menu
	// The system's items keep the keys their roles gave them; they stand
	// down while recording, and come back from here.
	a.systemKeys = make(map[*mygo.MenuItem]string)
	eachMenuItem(menu, func(item *mygo.MenuItem) {
		if item.Accelerator != "" && !a.carries(item) {
			a.systemKeys[item] = item.Accelerator
		}
	})
	a.syncMenu()
	return menu
}

// menuClick runs fn for a menu item. MyGo runs a click without a frame
// after it, so the window is redrawn here, or the change would wait for
// the next event.
func (a *AppState) menuClick(fn func()) func(*mygo.MenuItem, *mygo.Window) {
	return func(_ *mygo.MenuItem, window *mygo.Window) {
		fn()
		if window != nil && !window.IsDestroyed() {
			window.Invalidate()
		}
	}
}

// offerItem is an item of the Format menu and the command the rules
// offer it for (vocab.SurfaceMenuBar), whose key and state it shows.
type offerItem struct {
	item *mygo.MenuItem
	id   vocab.CommandID
}

// offeredItems are the Format menu's items for what the rules offer in
// the menu bar, a group each, each followed by a separator: built once
// from what the policies declare, each item kept in step with what is
// offered for the page in front (syncOffers).
func (a *AppState) offeredItems() []*mygo.MenuItem {
	labels := map[keymap.CommandID]string{}
	for _, c := range keymap.Declared(a.Product.Program.Catalog()) {
		labels[c.ID] = c.Label
	}
	var out []*mygo.MenuItem
	for _, g := range a.Product.Program.Offered() {
		if g.Group.In&vocab.SurfaceMenuBar == 0 {
			continue
		}
		for _, c := range g.Commands {
			id := c.ID
			item := &mygo.MenuItem{ID: "offer-" + string(id), Label: labels[keymap.CommandID(id)], Click: a.menuClick(func() {
				if el := a.pageElement(); el != nil && a.Shell.OnDesk() {
					el.RunTool(string(id))
					a.refreshEditorChrome()
				}
			})}
			if g.Group.Shape != vocab.OfferActions {
				item.Type = mygo.MenuItemCheckbox
			}
			a.offerItems = append(a.offerItems, offerItem{item: item, id: id})
			out = append(out, item)
		}
		out = append(out, mygo.Separator())
	}
	return out
}

// menuShown is what an item was last given, so syncMenu tells the
// system only of what changed.
type menuShown struct {
	accelerator, label string
	enabled, checked   bool
}

// carries reports whether item runs one of the app's commands, whose key
// is the keymap's.
func (a *AppState) carries(item *mygo.MenuItem) bool {
	for _, held := range a.menuItems {
		if held == item {
			return true
		}
	}
	for _, f := range a.offerItems {
		if f.item == item {
			return true
		}
	}
	return false
}

// syncMenu gives each item its command's key, name and state for the
// scene in front: the library's keys only while the field is, so the
// page hears its own on the desk, and the Format menu's only on the
// desk. While a shortcut is being recorded every item lets go of its
// key, the system's too, so the recorder hears each chord as keys: a
// disabled item would not do, as macOS validates the edit items enabled
// and still lets them take their keys.
func (a *AppState) syncMenu() {
	if a == nil || a.menu == nil || a.Shell == nil {
		return
	}
	keys := a.Product.Keymap
	desk := a.Shell.OnDesk()
	for id, item := range a.menuItems {
		declared, _ := keys.Command(id)
		state := a.commandState(id, declared.Label)
		accelerator := ""
		if chord, ok := keys.ChordFor(id); ok && !a.recording && (declared.Scope != keymap.ScopeLibrary || !desk) {
			accelerator, _ = keymap.Accelerator(chord, keymap.PlatformCurrent)
		}
		label := state.Label
		if strings.HasSuffix(item.Label, "…") && !strings.HasSuffix(label, "…") {
			label += "…"
		}
		a.showItem(item, menuShown{accelerator: accelerator, label: label, enabled: state.Enabled, checked: state.Checked})
	}
	mounted := false
	if a.Workspace != nil {
		_, mounted = a.Workspace.Mounted()
	}
	hasPage := desk && mounted
	// The offered items are as the rules offer them for the page in
	// front: enabled where they can run, checked where they are on.
	offers := map[vocab.CommandID]engine.Offered{}
	if el := a.pageElement(); hasPage && el != nil {
		for _, g := range el.Editor.Engine.Offers(el.Editor.View, vocab.SurfaceMenuBar) {
			for _, o := range g.Offers {
				offers[o.Command] = o
			}
		}
	}
	for _, f := range a.offerItems {
		accelerator := ""
		if chord, ok := keys.ChordFor(keymap.CommandID(f.id)); ok && hasPage && !a.recording {
			accelerator, _ = keymap.Accelerator(chord, keymap.PlatformCurrent)
		}
		o := offers[f.id]
		a.showItem(f.item, menuShown{accelerator: accelerator, label: f.item.Label, enabled: o.Available, checked: o.On})
	}
	// While the field is in front, Undo and Redo are its own, named for
	// what they take back; elsewhere, the system's, which the page and
	// text fields answer.
	field := a.Shell.FieldUndoes()
	for _, u := range [...]struct {
		item  *mygo.MenuItem
		verb  string
		named func(string) string
		can   func() (string, bool)
		key   string
	}{
		{a.undoItem, locales.Undo(), locales.UndoAction, a.Shell.CanUndo, "CmdOrCtrl+Z"},
		{a.redoItem, locales.Redo(), locales.RedoAction, a.Shell.CanRedo, "CmdOrCtrl+Shift+Z"},
	} {
		if u.item == nil {
			continue
		}
		what, ok := u.can()
		label := u.verb
		if ok {
			label = u.named(what)
		}
		accelerator := u.key
		if !field || a.recording {
			accelerator = ""
		}
		a.showItem(u.item, menuShown{accelerator: accelerator, label: label, enabled: ok && field})
	}
	if a.undoField == nil || *a.undoField != field {
		a.undoField = &field
		for _, item := range []*mygo.MenuItem{a.undoItem, a.redoItem} {
			if item != nil {
				item.SetVisible(field)
			}
		}
		for item := range a.systemKeys {
			if item.Role == mygo.RoleUndo || item.Role == mygo.RoleRedo {
				item.SetVisible(!field)
			}
		}
	}
	for item, accelerator := range a.systemKeys {
		if a.recording || field && (item.Role == mygo.RoleUndo || item.Role == mygo.RoleRedo) {
			accelerator = ""
		}
		setAccelerator(item, accelerator)
	}
}

// fieldUndo is Edit's Undo for the field's changes.
func (a *AppState) fieldUndo() *mygo.MenuItem {
	a.undoItem = &mygo.MenuItem{ID: "field-undo", Label: locales.Undo(), Click: a.menuClick(func() { a.Shell.Undo() })}
	return a.undoItem
}

// fieldRedo is Edit's Redo for the field's changes.
func (a *AppState) fieldRedo() *mygo.MenuItem {
	a.redoItem = &mygo.MenuItem{ID: "field-redo", Label: locales.Redo(), Click: a.menuClick(func() { a.Shell.Redo() })}
	return a.redoItem
}

// showItem gives item what shown says, telling the system only what
// changed.
func (a *AppState) showItem(item *mygo.MenuItem, shown menuShown) {
	was, ok := a.menuShown[item]
	if ok && was == shown {
		return
	}
	a.menuShown[item] = shown
	setAccelerator(item, shown.accelerator)
	if !ok || was.label != shown.label {
		item.SetLabel(shown.label)
	}
	if !ok || was.enabled != shown.enabled {
		item.SetEnabled(shown.enabled)
	}
	if item.Type == mygo.MenuItemCheckbox && (!ok || was.checked != shown.checked) {
		item.SetChecked(shown.checked)
	}
}

func setAccelerator(item *mygo.MenuItem, accelerator string) {
	if item.Accelerator != accelerator {
		item.SetAccelerator(accelerator)
	}
}

// setRecording stands the menu's keys down while a shortcut is recorded.
func (a *AppState) setRecording(recording bool) {
	if a.recording != recording {
		a.recording = recording
		a.syncMenu()
	}
}

// reservedKey names the menu item that keeps chord for itself: a key of the
// system's items, as Quit's, which no keymap command can be given, as the
// menu would always take it first.
func (a *AppState) reservedKey(chord keymap.Chord) (string, bool) {
	if a == nil {
		return "", false
	}
	for item, accelerator := range a.systemKeys {
		if held, ok := keymap.ParseAccelerator(accelerator, keymap.PlatformCurrent); ok && held == chord {
			return item.Label, true
		}
	}
	return "", false
}

func eachMenuItem(menu *mygo.Menu, visit func(*mygo.MenuItem)) {
	eachItem(menu.Items(), visit)
}

func eachItem(items []*mygo.MenuItem, visit func(*mygo.MenuItem)) {
	for _, item := range items {
		visit(item)
		eachItem(item.Submenu, visit)
	}
}
