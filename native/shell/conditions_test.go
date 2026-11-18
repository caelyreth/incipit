package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/internal/query"
)

// The same conditions come to the same query, whatever order they were
// taken in and whichever way they came in.
func TestTheQueryIsItsConditionsInWhateverOrder(t *testing.T) {
	one, _ := libraryState()
	one.toggleValue(mustKey("updated"), "today")
	one.toggleValue(mustKey("tag"), "work")
	one.toggleValue(mustKey("tag"), "plan")
	other, _ := libraryState()
	other.SetFieldQuery("#work $date:today $tag:plan")
	want := []string{"$tag:plan", "$tag:work", "$updated:today"}
	if !slices.Equal(one.field.conditions, want) || !slices.Equal(other.field.conditions, want) {
		t.Fatalf("Both ways should come to %v: %v and %v.", want, one.field.conditions, other.field.conditions)
	}
	if !sameQuery("$tag:b $tag:a words", "$tag:a  $tag:b words") {
		t.Fatal("Conditions in another order should be the same query.")
	}
}

// A key holding one value takes the new one in its place; a condition
// there already, written again, is not taken twice.
func TestAConditionIsTakenOnceAndOneValueInPlace(t *testing.T) {
	state, _ := libraryState()
	state.toggleValue(mustKey("updated"), "today")
	state.toggleValue(mustKey("tag"), "work")
	state.toggleValue(mustKey("updated"), "yesterday")
	if want := []string{"$tag:work", "$updated:yesterday"}; !slices.Equal(state.field.conditions, want) {
		t.Fatalf("The date should be taken in place of the one held: %v.", state.field.conditions)
	}
	state.takeConditions("$tag:work $updated:today ")
	if want := []string{"$tag:work", "$updated:today"}; !slices.Equal(state.field.conditions, want) {
		t.Fatalf("Written again, a condition should not be taken twice: %v.", state.field.conditions)
	}
	before := state.ribbonID(ribbonPlace{kind: ribbonCondition, text: "$updated:today"})
	if after := state.ribbonID(ribbonPlace{kind: ribbonCondition, text: "$updated:7d"}); before != after {
		t.Fatalf("A date changing should keep its ribbon: %q became %q.", before, after)
	}
}

// A condition taken moves only the ribbons on its own side of the pill.
func TestAConditionTakenLeavesTheOtherSideBe(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	pill, _ := tester.Find("Search")
	group, _ := tester.Find("Group")
	created, _ := tester.Find("Created")
	state.toggleValue(mustKey("tag"), "work")
	state.closeKey()
	state.Search()
	settle(tester)
	nowPill, _ := tester.Find("Search")
	tag, _ := tester.Find("Tag")
	if g, _ := tester.Find("Group"); g.X != group.X || abs((g.Y-nowPill.Y)-(group.Y-pill.Y)) > 0.5 || tag.Y < nowPill.Y {
		t.Fatalf("Group should stay relative to the pill (%v vs %v, now %v vs %v) and Tag under it (%v).", group, pill, g, nowPill, tag)
	}
	if c, _ := tester.Find("Created"); c.X != created.X || abs((c.Y-nowPill.Y)-(created.Y-pill.Y)) > 0.5 {
		t.Fatalf("Created should stay relative to the pill: %v vs %v, now %v vs %v.", created, pill, c, nowPill)
	}
}

// A filter being written never reaches the field, whether or not anything
// is offered for it.
func TestAHalfWrittenFilterNeverReachesTheField(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	tester.Type("$tag:wz")
	settle(tester)
	if q := state.wholeQuery(); q != "" {
		t.Fatalf("The field should answer nothing of the half written filter, but answers %q.", q)
	}
	if slices.ContainsFunc(library.calls, func(c string) bool { return strings.Contains(c, "$tag") }) {
		t.Fatalf("The half written filter should not be searched: %v.", library.calls)
	}
}

// In two groups at once, the field is in neither alone.
func TestTwoGroupsAreNoScope(t *testing.T) {
	state, _ := libraryState()
	for _, text := range []string{"$in:Inbox $in:Novel", "$in:Novel $in:Inbox"} {
		if scope, _, browse := state.browsing(query.Parse(text)); scope != "" || browse {
			t.Fatalf("%q should have no one group and search, but is in %q, browsing %v.", text, scope, browse)
		}
	}
}

// Writing in an open key never asks the library again for what the
// field already shows.
func TestWritingInAKeyLeavesTheResultsBe(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.SetFieldQuery("chapter")
	state.Search()
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	before := len(library.calls)
	tester.Type("we")
	settle(tester)
	if after := library.calls[before:]; slices.ContainsFunc(after, func(c string) bool { return strings.HasPrefix(c, "search ") }) {
		t.Fatalf("The query standing, nothing should be searched again: %v.", after)
	}
}
