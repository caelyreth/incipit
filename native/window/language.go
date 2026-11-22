package window

import "github.com/egoist/mygo"

// chooseLanguage has the app shown in the language of tag, "" for the
// system's: MyGo keeps it for the next launches and tells the app once it
// shows it (localeChanged).
func (a *AppState) chooseLanguage(tag string) {
	a.language = tag
	mygo.App.SetLocale(tag)
	a.refreshSettingsData()
}

// localeChanged has what the app says drawn anew in the language it now
// shows: the menu, built again, the settings and the field. The windows
// draw anew of themselves.
func (a *AppState) localeChanged() {
	a.language = mygo.App.ChosenLocale()
	mygo.App.SetMenu(a.MainMenu())
	a.syncMenu()
	a.refreshSettingsData()
	a.refreshShell()
}
