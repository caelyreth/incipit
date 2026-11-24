package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// BenchmarkMenuSync is the menu's share of a frame the window draws anew:
// syncMenu, run by every View, against the frame itself, on the desk with
// a page in front.
func BenchmarkMenuSync(b *testing.B) {
	app, err := NewAppState()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { app.Close() })
	app.MainMenu()
	// A running app reads its locale from what it shows, kept once worked
	// out; a test's MyGo, which never runs, works it out at every read.
	locale := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return locale })
	tester := ui.NewTester(app.View, 1200, 800)
	tester.Frame()
	app.selectMounted()
	app.Shell.ShowDesk()
	tester.Advance(time.Second) // the desk comes to rest
	if !app.Shell.OnDesk() || app.pageElement() == nil {
		b.Fatal("no page in front on the desk")
	}
	b.Run("sync", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			app.syncMenu()
		}
	})
	b.Run("frame", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			tester.Frame()
		}
	})
}
