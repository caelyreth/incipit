package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No two things that light under the pointer touch: a card's head stands
// clear of its first row, and its rows of each other.
func TestACardsHeadAndRowsStandApart(t *testing.T) {
	_, library, tester := onField(t, 1260, 850)
	library.extraSheets = 2
	library.rev++
	settle(tester)
	head, ok := tester.Find("Novel")
	if !ok {
		t.Fatal("Novel's block should show.")
	}
	var rows []struct{ y, h float32 }
	for _, name := range []string{"Chapter", "Sheet 1", "Sheet 2"} {
		if r, ok := tester.Find(name); ok && r.X >= head.X-1 && r.X < head.X+head.W {
			rows = append(rows, struct{ y, h float32 }{r.Y, r.H})
		}
	}
	if len(rows) < 2 {
		t.Fatalf("Novel should hold rows to measure: %v.", rows)
	}
	if gap := rows[0].y - (head.Y + head.H); gap < 6 {
		t.Fatalf("The head should stand 6 clear of its first row, not %v.", gap)
	}
	for i := 1; i < len(rows); i++ {
		if gap := rows[i].y - (rows[i-1].y + rows[i-1].h); gap < 4 {
			t.Fatalf("Rows should stand at least 4 apart, not %v: %v.", gap, rows)
		}
	}
}

// The shell names wordless controls with components.Hint, in its own
// material and toward open space: never the system's tooltip, which
// carried instructions and warnings no one saw.
func TestTheShellUsesHintsNotTooltips(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), ".Tooltip(") {
			t.Errorf("%s calls Tooltip; use components.Hint", f)
		}
	}
}
