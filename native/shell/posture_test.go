package shell

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/stage"
	"github.com/egoist/mygo/ui"
)

func TestPostureTransitionsAndPillInteraction(t *testing.T) {
	state := openState()
	state.Open = []Page{{ID: "1", Title: "First Note"}, {ID: "2", Title: "Second Note"}}
	state.ActivePage = "1"
	state.Sheet = func(c *ui.Context, id string, _ SheetFrame) {
		ui.Text(c, "Content of "+id)
	}
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)

	if state.Posture() != stage.PostureWriting {
		t.Fatalf("Initial posture = %v, want Writing", state.Posture())
	}
	if _, ok := tester.Find("Neverland pill"); ok {
		t.Fatal("Pill should not be in the accessibility tree while Writing")
	}

	// Clicking Toggle field enters Glance when sheets are open.
	if err := tester.Click("Toggle field"); err != nil {
		t.Fatalf("Click Toggle field: %v", err)
	}
	settle(tester)
	if state.Posture() != stage.PostureGlance {
		t.Fatalf("Posture after Toggle field = %v, want Glance", state.Posture())
	}
	pill, ok := tester.Find("Neverland pill")
	if !ok {
		t.Fatal("Pill should be visible in Glance")
	}
	desk, ok := tester.Find("Return to desk")
	if !ok {
		t.Fatal("Leaned desk return shield should be present in Glance")
	}
	if pill.X+pill.W > desk.X+4 {
		t.Fatalf("Pill (%+v) overlaps leaned desk (%+v) in Glance", pill, desk)
	}

	// Typing into the pill's focused search input updates PillQuery.
	if !tester.Focused("Search field") {
		t.Fatal("Pill query field should take focus when entering Glance")
	}
	tester.Type("starfield")
	tester.Frame()
	if state.PillQuery() != "starfield" {
		t.Fatalf("PillQuery = %q, want %q", state.PillQuery(), "starfield")
	}

	// Switching to Explore moves to PostureArrive with a further-leaned desk.
	if err := tester.Click("Explore"); err != nil {
		t.Fatalf("Click Explore: %v", err)
	}
	settle(tester)
	if state.Posture() != stage.PostureArrive {
		t.Fatalf("Posture after Explore = %v, want Arrive", state.Posture())
	}
	arriveDesk, ok := tester.Find("Return to desk")
	if !ok {
		t.Fatal("Leaned desk should remain present in Arrive")
	}
	if arriveDesk.X <= desk.X || arriveDesk.Y <= desk.Y {
		t.Fatalf("Arrive desk (%+v) should lean further right/down than Glance desk (%+v)", arriveDesk, desk)
	}

	// Clicking the leaned desk returns to Writing.
	if err := tester.Click("Return to desk"); err != nil {
		t.Fatalf("Click Return to desk: %v", err)
	}
	settle(tester)
	if state.Posture() != stage.PostureWriting {
		t.Fatalf("Posture after clicking leaned desk = %v, want Writing", state.Posture())
	}
	if _, ok := tester.Find("Neverland pill"); ok {
		t.Fatal("Pill should hide again after returning to Writing")
	}
}

func TestPostureEscapeClearsQueryThenReturnsToWriting(t *testing.T) {
	state := openState()
	state.Open = []Page{{ID: "1", Title: "Draft"}}
	state.ActivePage = "1"
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)

	state.CallField()
	settle(tester)
	tester.Type("aurora")
	tester.Frame()
	if state.PillQuery() != "aurora" {
		t.Fatalf("PillQuery = %q, want aurora", state.PillQuery())
	}

	// First Escape clears the query while staying in the field.
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.PillQuery() != "" {
		t.Fatalf("First Escape should clear PillQuery, got %q", state.PillQuery())
	}
	if state.Posture() != stage.PostureGlance {
		t.Fatalf("First Escape should keep Glance posture, got %v", state.Posture())
	}

	// Second Escape returns to Writing.
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.Posture() != stage.PostureWriting {
		t.Fatalf("Second Escape should return to Writing, got %v", state.Posture())
	}
}

func TestEmptyDeskEntersFullArrivePosture(t *testing.T) {
	state := New()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)

	state.CallField()
	settle(tester)
	if state.Posture() != stage.PostureArrive {
		t.Fatalf("Empty desk CallField posture = %v, want Arrive", state.Posture())
	}
	pill, ok := tester.Find("Neverland pill")
	if !ok {
		t.Fatal("Pill should show in empty-desk Arrive")
	}
	if _, hasDesk := tester.Find("Return to desk"); hasDesk {
		t.Fatal("Empty desk should not render a leaned desk return target")
	}
	if abs(pill.X+pill.W/2-630) > 2 {
		t.Fatalf("Empty-desk Arrive pill should center in window (630), got %+v", pill)
	}
}

func TestPostureMidFlightInterruptReversesSmoothly(t *testing.T) {
	state := openState()
	state.Open = []Page{{ID: "1", Title: "Draft"}}
	state.ActivePage = "1"
	state.Appearance.Motion = true
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()

	state.ToggleField()
	tester.Frame()
	tester.Advance(140 * time.Millisecond)
	tester.Frame()
	midScale := state.StageScene().Desk.Scale
	if !(midScale < 1 && midScale > 0.54) {
		t.Fatalf("Desk scale mid-flight should be between 0.54 and 1, got %v", midScale)
	}

	state.ReturnToDesk()
	tester.Frame()
	tester.Advance(600 * time.Millisecond)
	tester.Frame()
	if got := state.StageScene().Desk.Scale; abs(got-1) > 0.01 {
		t.Fatalf("Interrupted return did not settle back to scale 1, got %v", got)
	}
}
