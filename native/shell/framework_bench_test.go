package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func BenchmarkShellFrames(b *testing.B) {
	for _, scenario := range []string{"rebuild", "hover", "panes"} {
		b.Run(scenario, func(b *testing.B) {
			state, library := libraryState()
			library.extraSheets = 30
			state.ActivePage = "7"
			state.Go(Place{Kind: PlaceGroup, Group: "2"})
			tester := ui.NewTester(state.View, 1260, 850)
			settle(tester)
			first, _ := tester.Find("Chapter")
			second, _ := tester.Find("Sheet 1")
			frame := 0
			b.ReportAllocs()
			for b.Loop() {
				switch scenario {
				case "hover":
					row := first
					if frame%2 != 0 {
						row = second
					}
					tester.Move(row.X+row.W/2, row.Y+row.H/2)
				case "panes":
					if frame%30 == 0 {
						state.ToggleSidebar()
					}
					tester.Advance(16 * time.Millisecond)
				}
				tester.Frame()
				frame++
			}
		})
	}
}
