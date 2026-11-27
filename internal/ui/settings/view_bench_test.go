package settings

import (
	"testing"

	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	"github.com/egoist/mygo/ui"
)

// BenchmarkSettingsFrame draws every section of the settings, one card
// under another, frame after frame: each section finds itself, and only
// itself, as the field draws it.
func BenchmarkSettingsFrame(b *testing.B) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Apply = func(appsettings.Change) {}
	tester := ui.NewTester(state.cards(), 600, 4000)
	tester.Frame()
	b.ReportAllocs()
	for b.Loop() {
		tester.Frame()
	}
}
