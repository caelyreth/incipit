package components

import (
	"testing"
	"time"
)

// A bar's groups hold while what they are offered for keeps changing, as
// a selection being dragged: a new set is taken once it has stood, each
// group shown taking its tools' states as they are meanwhile.
func TestToolGroupsSettleBeforeTheyChange(t *testing.T) {
	marks := ToolGroup{Key: "marks", Tools: []Tool{{ID: "strong"}}}
	kind := ToolGroup{Key: "kind", Tools: []Tool{{ID: "paragraph"}}}
	on := ToolGroup{Key: "marks", Tools: []Tool{{ID: "strong", On: true}}}
	var s ToolSettle
	at := time.Unix(100, 0)
	keys := func(groups []ToolGroup) []string {
		var out []string
		for _, g := range groups {
			out = append(out, g.Key)
		}
		return out
	}

	s.settled([]ToolGroup{kind, marks}, at)
	got, wait := s.settled([]ToolGroup{on}, at.Add(50*time.Millisecond))
	if len(got) != 2 || wait <= 0 || !got[1].Tools[0].On {
		t.Fatalf("A new set at once shows %v (wait %v), want both held, the marks as they are now.", keys(got), wait)
	}
	s.settled([]ToolGroup{kind, marks}, at.Add(100*time.Millisecond))
	got, _ = s.settled([]ToolGroup{on}, at.Add(250*time.Millisecond))
	if len(got) != 2 {
		t.Fatalf("A set that changed back and forth was taken before it stood: %v.", keys(got))
	}
	got, wait = s.settled([]ToolGroup{on}, at.Add(500*time.Millisecond))
	if len(got) != 1 || wait != 0 {
		t.Fatalf("A set that stood is not taken: %v.", keys(got))
	}
}
