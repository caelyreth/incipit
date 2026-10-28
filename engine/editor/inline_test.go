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
	settings := MarkerSettings{Enabled: true, AllowStrong: true}
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

func TestMarkerPlanHonorsStrongGateAndOrigin(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "word", model.EmptyUTF16Range(0))
	node := fixture.nodeValue(t)
	active := node.Marks()[0].ID()
	action, err := PlanMarkerTypes(
		node,
		model.EmptyUTF16Range(0),
		"*",
		&active,
		MarkerSettings{Enabled: true, AllowStrong: false},
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !action.Handled || !action.NoOp || action.Exited {
		t.Fatalf("strong gate action=%#v", action)
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
			MarkerSettings{Enabled: true, AllowStrong: true},
			origin,
		)
		if err != nil || action.Handled {
			t.Fatalf("origin %s action=%#v err=%v", origin, action, err)
		}
	}
}

func TestInlineStateUsesNarrowestContainingMark(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "abcdef")
	if _, err := fixture.document.Transact(func(draft *model.DocumentDraft) error {
		if _, err := draft.AddMark(fixture.node, model.NewUTF16Range(0, 5), 1); err != nil {
			return err
		}
		_, err := draft.AddMark(fixture.node, model.NewUTF16Range(1, 4), 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	node := fixture.nodeValue(t)
	var state InlineState
	state.Select(node, fixture.selection(t, 2, 2))
	active, ok := state.Active()
	if !ok || active != node.Marks()[1].ID() {
		t.Fatalf("active=%d,%v marks=%#v", active, ok, node.Marks())
	}
	state.Select(node, fixture.selection(t, 6, 6))
	if _, ok := state.Active(); ok {
		t.Fatal("outside mark remained active")
	}
}

func TestInlineInputStyleAndMutedRuns(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "abc", model.NewUTF16Range(0, 3))
	node := fixture.nodeValue(t)
	active := node.Marks()[0].ID()
	style := InlineInputStyle(node, &active, true)
	if !style.Italic || style.Bold {
		t.Fatalf("input style=%#v", style)
	}
	configuration, err := markupConfiguration(t, true, false, true)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := MarkupProjectDocument(
		fixture.document.Snapshot(),
		fixture.document.Revision(),
		fixture.node,
		0,
		configuration,
	)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := InlineStyleRuns(projection, node, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 || runs[0].Style.Color != surface.ColorMuted ||
		!runs[1].Style.Italic || runs[1].Style.Color != surface.ColorText {
		t.Fatalf("runs=%#v", runs)
	}
}
