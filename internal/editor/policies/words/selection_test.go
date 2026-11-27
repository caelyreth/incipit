package words

import (
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func TestDoubleClickSpacePolicy(test *testing.T) {
	for _, testCase := range []struct {
		name   string
		values policy.Values
		want   [2]int
	}{
		{"default", nil, [2]int{3, 6}},
		{"nearby", policy.Values{"words.nearbyContent": true}, [2]int{6, 9}},
		{"whitespace", policy.Values{"words.nearbyContent": false}, [2]int{3, 6}},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			harness := newHarnessWith(test, testCase.values)
			harness.typeText("one   two")
			place := vocab.Place{Stop: harness.caret(5), Over: vocab.Backward, Clicks: 2}
			preview, err := harness.e.Preview(1, vocab.Input{Intent: place, At: harness.now})
			if err != nil {
				test.Fatal(err)
			}
			from, to := preview.Selection.Ordered(harness.e.Snapshot())
			if got := [2]int{from.Pos.Offset, to.Pos.Offset}; got != testCase.want {
				test.Fatalf("preview selected %v, want %v", got, testCase.want)
			}
			harness.do(place)
			view, _ := harness.e.View(1)
			if view.Selection != preview.Selection {
				test.Fatalf("committed selection differs from preview: %+v versus %+v", view.Selection, preview.Selection)
			}
			for _, press := range []struct {
				clicks int
				want   [2]int
			}{{1, [2]int{5, 5}}, {3, [2]int{0, 9}}, {4, testCase.want}} {
				place.Clicks = press.clicks
				harness.do(place)
				view, _ = harness.e.View(1)
				from, to = view.Selection.Ordered(harness.e.Snapshot())
				if got := [2]int{from.Pos.Offset, to.Pos.Offset}; got != press.want {
					test.Fatalf("press %d selected %v, want %v", press.clicks, got, press.want)
				}
			}
		})
	}
}

func TestNearbyContentSelection(test *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		off  int
		over vocab.Direction
		want [2]int
	}{
		{"left edge", "one   two", 3, vocab.Forward, [2]int{0, 3}},
		{"left half", "one   two", 4, vocab.Forward, [2]int{0, 3}},
		{"right half", "one   two", 5, vocab.Backward, [2]int{6, 9}},
		{"right edge", "one   two", 6, vocab.Backward, [2]int{6, 9}},
		{"tie before", "one    two", 5, vocab.Backward, [2]int{0, 3}},
		{"tie after", "one    two", 5, vocab.Forward, [2]int{7, 10}},
		{"tie without side", "one    two", 5, 0, [2]int{0, 3}},
		{"leading", "   one", 1, vocab.Forward, [2]int{3, 6}},
		{"trailing", "one   ", 5, vocab.Backward, [2]int{0, 3}},
		{"outside text", "one   ", 6, 0, [2]int{0, 3}},
		{"tab", "one\t two", 4, vocab.Forward, [2]int{5, 8}},
		{"unicode spaces", "one\u2003   two", 7, vocab.Backward, [2]int{0, 3}},
		{"unicode spaces after", "one\u2003   two", 7, vocab.Forward, [2]int{9, 12}},
		{"CJK", "你好   world", 7, vocab.Forward, [2]int{0, 6}},
		{"emoji", "a  👩‍💻", 3, vocab.Backward, [2]int{3, len("a  👩‍💻")}},
		{"punctuation nearby", "one ,two", 4, vocab.Backward, [2]int{4, 5}},
		{"punctuation hit", "one ,two", 4, vocab.Forward, [2]int{4, 5}},
		{"word hit", "one two", 4, vocab.Forward, [2]int{4, 7}},
		{"before line break", "one \n two", 4, vocab.Backward, [2]int{0, 3}},
		{"after line break", "one \n two", 5, vocab.Forward, [2]int{6, 9}},
		{"line break", "one \n two", 4, vocab.Forward, [2]int{4, 5}},
		{"blank line", "one\n   \ntwo", 5, vocab.Forward, [2]int{4, 7}},
		{"blank paragraph", "   ", 1, vocab.Forward, [2]int{0, 3}},
		{"empty paragraph", "", 0, 0, [2]int{0, 0}},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			content := doc.NewInline(doc.TextItem(testCase.text, nil))
			from, to := segmentAt(content, testCase.off, testCase.over, true)
			if got := [2]int{from, to}; got != testCase.want {
				test.Fatalf("selected %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestDoubleClickSpacesBesideInlineObjects(test *testing.T) {
	formula := doc.NewNode(profile.Formula, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x", nil)))
	content := doc.NewInline(doc.TextItem("a  ", nil), doc.NodeItem(formula), doc.TextItem("  b", nil))
	for _, testCase := range []struct {
		off    int
		over   vocab.Direction
		nearby bool
		want   [2]int
	}{
		{3, vocab.Backward, false, [2]int{1, 3}},
		{3, vocab.Forward, false, [2]int{3, 4}},
		{4, vocab.Forward, false, [2]int{4, 6}},
		{3, vocab.Backward, true, [2]int{3, 4}},
		{3, vocab.Forward, true, [2]int{3, 4}},
		{4, vocab.Forward, true, [2]int{3, 4}},
		{1, vocab.Forward, true, [2]int{0, 1}},
		{6, vocab.Backward, true, [2]int{6, 7}},
	} {
		from, to := segmentAt(content, testCase.off, testCase.over, testCase.nearby)
		if got := [2]int{from, to}; got != testCase.want {
			test.Errorf("offset %d, side %v, nearby %v selected %v, want %v", testCase.off, testCase.over, testCase.nearby, got, testCase.want)
		}
	}
}
