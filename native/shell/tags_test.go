package shell

import (
	"fmt"
	"testing"
)

func TestTagsFitTwoLinesAndCountTheRest(t *testing.T) {
	more := func(int) float32 { return 20 }
	cases := []struct {
		widths []float32
		lines  string
		hidden int
	}{
		{[]float32{30, 30}, "[[0 1]]", 0},
		{[]float32{40, 40, 40}, "[[0 1] [2]]", 0},
		{[]float32{40, 40, 40, 40, 40}, "[[0 1] [2]]", 2},
		{[]float32{200, 10}, "[[0] [1]]", 0},
		{[]float32{100, 70, 30}, "[[0] [1]]", 1},
		{[]float32{100, 100, 100}, "[[0] []]", 2},
		{[]float32{100, 100, 100, 100}, "[[0] []]", 3},
	}
	for _, c := range cases {
		lines, hidden := fitTags(c.widths, more, 100, 5, 2)
		if got := fmt.Sprint(lines); got != c.lines || hidden != c.hidden {
			t.Errorf("%v: lines %s hidden %d, want %s hidden %d", c.widths, got, hidden, c.lines, c.hidden)
		}
	}
}
