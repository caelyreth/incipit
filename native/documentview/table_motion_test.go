package documentview

import (
	"testing"

	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestTableMadeFeedbackRespectsMotionPolicy(t *testing.T) {
	for _, animate := range []bool{false, true} {
		name := "still"
		if animate {
			name = "animated"
		}
		t.Run(name, func(t *testing.T) {
			state := &State{Animate: animate, Theme: theme.DefaultSettings().Semantic()}
			placed := textlayout.PlacedTable{
				Table:   surface.Table{Rows: [][]surface.TableCell{{{Node: 1}}}},
				Rows:    []float32{0},
				Columns: []float32{0},
			}
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) {
					drawMade(p, state, placed, func(int, int) ui.Rect {
						return ui.Rect{X: 5, Y: 5, W: 50, H: 30}
					}, 4)
				})
			}, 100, 100)
			tester.Frame()
			state.TableMade = &tableMade{table: 1, at: tester.Now()}
			tester.Frame()
			if tester.FramePending() != animate {
				t.Fatalf("feedback animation pending = %v, want %v", tester.FramePending(), animate)
			}
			before := tester.Image().RGBAAt(20, 20)
			tester.Advance(madeFade / 2)
			middle := tester.Image().RGBAAt(20, 20)
			if (middle != before) != animate {
				t.Fatalf("feedback changed before its deadline: %v -> %v", before, middle)
			}
			tester.Advance(madeFade)
			if tester.FramePending() || tester.Image().RGBAAt(20, 20) == before {
				t.Fatal("feedback did not clear after its deadline")
			}
		})
	}
}
