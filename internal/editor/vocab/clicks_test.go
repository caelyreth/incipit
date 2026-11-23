package vocab

import "testing"

// Past its third press a run of clicks goes on in turn: a double click on
// every even press, a triple on every odd one.
func TestARunOfClicksStepsInTurn(t *testing.T) {
	want := []ClickStep{SingleClick, SingleClick, DoubleClick, TripleClick, DoubleClick, TripleClick, DoubleClick}
	for clicks, step := range want {
		if got := StepOf(clicks); got != step {
			t.Errorf("press %d is step %v, want %v", clicks, got, step)
		}
	}
}
