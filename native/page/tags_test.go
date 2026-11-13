package page

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

func key(el *Element, k ui.Key) bool {
	ok := el.handle(ui.InputEvent{Kind: ui.InputKeyDown, Key: k, Time: time.Unix(500, 0)}, ui.Services{})
	el.relayout()
	el.syncTags()
	return ok
}

func taggedElement(t *testing.T) *Element {
	el := newElement(t)
	el.Editor.Tags = func(string) []string { return []string{"ocean", "moon", "octopus", "rock"} }
	return el
}

func TestATagBeingWrittenOffersTheNotebooksTags(t *testing.T) {
	el := taggedElement(t)
	typeText(el, "see #oc")
	el.syncTags()
	if got := fmt.Sprint(el.tags.rows); got != "[{ocean false} {octopus false} {rock false} {oc true}]" {
		t.Fatalf("rows %s", got)
	}
	key(el, ui.KeyDown)
	if !key(el, ui.KeyEnter) {
		t.Fatal("Return did not pick")
	}
	if got := firstText(el); got != "see #octopus " {
		t.Fatalf("text %q", got)
	}
	if el.tags.open() {
		t.Fatal("the picker stays open after picking")
	}
}

func TestMovingIntoATagOpensNoPicker(t *testing.T) {
	el := taggedElement(t)
	typeText(el, "#ocean x")
	el.syncTags()
	for range 4 {
		el.Editor.Do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}, time.Unix(600, 0))
	}
	el.relayout()
	el.syncTags()
	if el.tags.open() {
		t.Fatal("moving into a tag opened the picker")
	}
	// Writing in it opens it; Escape closes it until it is written in
	// again.
	typeText(el, "x")
	el.syncTags()
	if !el.tags.open() {
		t.Fatal("writing in a tag opened no picker")
	}
	key(el, ui.KeyEscape)
	if el.tags.open() {
		t.Fatal("Escape left the picker open")
	}
	if key(el, ui.KeyDown); el.tags.open() {
		t.Fatal("the picker opened again without writing")
	}
}

func TestTheTagItIsComesFirstAndMakesNone(t *testing.T) {
	rows := tagRows([]string{"Moon", "moonlight", "honeymoon", "sea"}, "moon")
	if got := fmt.Sprint(rows); got != "[{Moon false} {moonlight false} {honeymoon false}]" {
		t.Fatalf("rows %s", got)
	}
	if got := fmt.Sprint(tagRows(nil, "new")); got != "[{new true}]" {
		t.Fatalf("rows %s", got)
	}
}

func firstText(el *Element) string {
	return el.Editor.Engine.Snapshot().Root().Slot(0).Child(0).Text()
}

// Every tag in a line wears its own background, each fading on its own:
// stretches of one look are told apart by their order.
func TestEveryTagWearsItsBackground(t *testing.T) {
	el := newElement(t)
	tt := ui.NewTester(el.View, 900, 600)
	typeText(el, "#a and #b and #c and #d")
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	shown := 0
	for k, sd := range el.decos {
		if !k.highlight && sd.alpha > 0.99 && len(sd.rows) == 1 {
			shown++
		}
	}
	if shown != 4 {
		t.Fatalf("%d tags wear their background, want 4", shown)
	}
}
