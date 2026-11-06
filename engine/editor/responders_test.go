package editor

import (
	"github.com/caelyreth/neverland/engine/model"
	"strings"
	"testing"
)

// Two responders that can answer one intent in one kind of place must be
// declared in order, as the kernel requires of its rules: nothing may
// depend on their names' order.
func TestRespondersMeetingOnAnIntentAreOrdered(t *testing.T) {
	ids := make([]string, 0, len(responders))
	byID := make(map[string]*responder)
	for i := range responders {
		r := &responders[i]
		if byID[r.id] != nil {
			t.Fatalf("two responders are %q", r.id)
		}
		byID[r.id] = r
		ids = append(ids, r.id)
	}
	ranking, _, err := orderedResponders(ids, byID)
	if err != nil {
		t.Fatal(err)
	}
	samples := []Intent{
		Insert("a", 0), InsertAt(model.UTF16Range{}, "a", 0),
		Dismiss, Break, SoftBreak, Leave, Return, Undo, Redo, Format(1), Nest(Forward), Nest(Backward),
	}
	for _, by := range []Reach{ByGrapheme, ByWord, BySentence, ByParagraph, ByLine, ByRow, ByScope, ByPage} {
		for _, dir := range []Direction{Backward, Forward} {
			samples = append(samples, Delete(by, dir), Move(by, dir, false), Move(by, dir, true))
		}
		samples = append(samples, SelectUnit(by))
	}
	kinds := []PlaceKind{PlaceFollowUp, PlaceCells, PlaceObject, PlacePair, PlaceEmphasis, PlaceFormula,
		PlaceBlock, PlaceCell, PlaceGroup, PlaceTable, PlacePage}
	for i, a := range responderOrder {
		for _, b := range responderOrder[i+1:] {
			for _, kind := range kinds {
				if !a.in(kind) || !b.in(kind) {
					continue
				}
				for _, intent := range samples {
					if a.answers(intent) && b.answers(intent) && !ranking.Related(a.id, b.id) {
						t.Errorf("%s and %s both answer %v in place %d with neither declared first",
							a.id, b.id, intent.Kind, kind)
					}
				}
			}
		}
	}
}

func TestEveryResponderAnswersSomehow(t *testing.T) {
	for _, r := range responders {
		if (r.step == nil) == (r.respond == nil) {
			t.Errorf("%s must either step or respond", r.id)
		}
		if strings.TrimSpace(r.id) == "" || r.answers == nil {
			t.Errorf("responder %q declares no intents", r.id)
		}
	}
}
