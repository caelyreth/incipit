package shell

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func BenchmarkShellFrames(b *testing.B) {
	for _, scenario := range []string{"rebuild", "hover", "field"} {
		b.Run(scenario, func(b *testing.B) {
			state, library := libraryState()
			library.extraSheets = 30
			inFront(state, "7")
			state.ToggleField()
			tester := ui.NewTester(state.View, 1260, 850)
			settle(tester)
			first, _ := tester.Find("Inbox")
			second, _ := tester.Find("Novel")
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
				case "field":
					if frame%30 == 0 {
						state.ToggleField()
					}
					tester.Advance(16 * time.Millisecond)
				}
				tester.Frame()
				frame++
			}
		})
	}
}

// BenchmarkShellTransitions measures Field <-> Desk spring transitions across
// a 30-sheet Field and an 8-sheet Desk, exercising leanDesk CacheScaled and
// Field card Cache layer lifecycle transitions.
func BenchmarkShellTransitions(b *testing.B) {
	setup := func(onDesk bool) (*State, *ui.Tester) {
		state, library := libraryState()
		library.extraSheets = 30
		library.desk = state
		state.Sheet = func(c *ui.Context, page Page, _ SheetFrame) {
			ui.Text(c, "Body of "+page.Title)
		}
		state.Open = nil
		for i := 1; i <= 8; i++ {
			id := fmt.Sprint(i)
			state.Open = append(state.Open, sheetPage(id, fmt.Sprintf("Sheet %d", i)))
		}
		inFront(state, "1")
		if onDesk {
			state.ShowDesk()
		} else {
			state.ShowField()
		}
		tester := ui.NewTester(state.View, 1260, 850)
		settle(tester)
		return state, tester
	}

	b.Run("desk_to_field", func(b *testing.B) {
		state, tester := setup(true)
		b.ReportAllocs()
		for b.Loop() {
			state.ShowField()
			settle(tester)
			b.StopTimer()
			state.ShowDesk()
			settle(tester)
			b.StartTimer()
		}
	})

	b.Run("field_to_desk", func(b *testing.B) {
		state, tester := setup(false)
		b.ReportAllocs()
		for b.Loop() {
			state.ShowDesk()
			settle(tester)
			b.StopTimer()
			state.ShowField()
			settle(tester)
			b.StartTimer()
		}
	})
}
