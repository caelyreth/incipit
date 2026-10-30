package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func TestMarkerPlanCreatesUpgradesAndExitsAtActiveEnd(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "")
	settings := testTriggers()
	node := fixture.nodeValue(t)

	action, err := PlanMarkerTypes(
		node,
		model.EmptyUTF16Range(0),
		"*",
		nil,
		settings,
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !action.Handled || action.Kind != MarkerCreate || action.Level != 1 {
		t.Fatalf("create action=%#v", action)
	}
	if _, err := fixture.document.Transact(action.Apply); err != nil {
		t.Fatal(err)
	}

	fixture = newEditingFixture(t, "word", model.NewUTF16Range(0, 4))
	node = fixture.nodeValue(t)
	active := node.Marks()[0].ID()
	action, err = PlanMarkerTypes(
		node,
		model.EmptyUTF16Range(4),
		"*",
		&active,
		settings,
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !action.Handled || action.Kind != MarkerExit || !action.Exited {
		t.Fatalf("exit action=%#v", action)
	}
	before := fixture.document.Snapshot()
	if _, err := fixture.document.Transact(action.Apply); err != nil {
		t.Fatal(err)
	}
	if fixture.document.Snapshot().Revision() != before.Revision() {
		t.Fatal("marker exit published a document change")
	}

	fixture = newEditingFixture(t, "word", model.EmptyUTF16Range(0))
	node = fixture.nodeValue(t)
	active = node.Marks()[0].ID()
	for _, want := range []uint8{2, 0} {
		action, err = PlanMarkerTypes(
			node,
			model.EmptyUTF16Range(0),
			"*",
			&active,
			settings,
			kernel.EventOriginTyping,
		)
		if err != nil {
			t.Fatal(err)
		}
		if action.Kind != MarkerUpgrade || action.Level != want {
			t.Fatalf("upgrade action=%#v want level %d", action, want)
		}
		if _, err := fixture.document.Transact(action.Apply); err != nil {
			t.Fatal(err)
		}
		node = fixture.nodeValue(t)
	}
}

func TestMarkerPlanHonorsTriggersAndOrigin(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "word", model.EmptyUTF16Range(0))
	node := fixture.nodeValue(t)
	active := node.Marks()[0].ID()
	off := testTriggers()
	off.asterisks = false
	action, err := PlanMarkerTypes(
		node,
		model.EmptyUTF16Range(0),
		"*",
		&active,
		off,
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	if action.Handled {
		t.Fatalf("a marker typing may not read was read: %#v", action)
	}
	for _, origin := range []kernel.EventOrigin{
		kernel.EventOriginPaste,
		kernel.EventOriginIME,
		kernel.EventOriginSemantic,
	} {
		action, err = PlanMarkerTypes(
			node,
			model.EmptyUTF16Range(0),
			"*",
			nil,
			testTriggers(),
			origin,
		)
		if err != nil || action.Handled {
			t.Fatalf("origin %s action=%#v err=%v", origin, action, err)
		}
	}
}

func TestCaretMembershipIsTheNarrowestBracketingMark(t *testing.T) {
	t.Parallel()
	session := newMarkedSession(t, "abcdef", markSpec{0, 5, 1}, markSpec{1, 4, 2})
	if text := session.Projection().Text(); text != "*a**bcd**e*f" {
		t.Fatalf("projection = %q", text)
	}
	node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Node())
	outer, inner := node.Marks()[0].ID(), node.Marks()[1].ID()
	cases := []struct {
		at   model.UTF16Offset
		want *model.MarkID
	}{
		{0, nil},     // |*a
		{1, &outer},  // *|a
		{2, &outer},  // *a|**
		{4, &inner},  // **|bcd
		{7, &inner},  // bcd|**
		{9, &outer},  // **|e
		{10, &outer}, // e|*
		{11, nil},    // *|f
	}
	for _, test := range cases {
		if got := caretMembership(node, session.Projection(), test.at); !sameMark(got, test.want) {
			t.Errorf("membership at %d = %v, want %v", test.at, got, test.want)
		}
	}
}

func TestInlineInputStyleAndMutedRuns(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "abc", model.NewUTF16Range(0, 3))
	node := fixture.nodeValue(t)
	active := node.Marks()[0].ID()
	style := InlineInputStyle(node, &active)
	if !style.Italic || style.Bold {
		t.Fatalf("input style=%#v", style)
	}
	configuration, err := markupConfiguration(t, true)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := ProjectPage(
		fixture.document.Snapshot(),
		fixture.document.Revision(),
		0,
		configuration,
	)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := InlineStyleRuns(projection, fixture.document.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 || runs[0].Style.Color != surface.ColorMuted ||
		!runs[1].Style.Italic || runs[1].Style.Color != surface.ColorText {
		t.Fatalf("runs=%#v", runs)
	}
}

// testTriggers lets typing read every marker.
func testTriggers() writingTriggers {
	return triggersFrom(func(_ string, fallback bool) bool { return fallback })
}
