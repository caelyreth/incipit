package editor

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func TestEditorCommandsKeepCanonicalState(t *testing.T) {
	t.Parallel()
	for seed := int64(1); seed <= 4; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			runEditorStress(t, seed)
		})
	}
}

func runEditorStress(t *testing.T, seed int64) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	session := richPage(t)
	trace := make([]string, 0, 32)
	for step := 0; step < 1000; step++ {
		var (
			label string
			err   error
		)
		switch rng.Intn(9) {
		case 0:
			label = "type"
			replacement := string(rune('a' + rng.Intn(4)))
			label = "type " + replacement
			_, err = session.replaceSelection(replacement, kernel.EventOriginTyping)
		case 1:
			label = "left"
			_, err = session.press(textLeft)
		case 2:
			label = "right"
			_, err = session.press(textRight)
		case 3:
			label = "select-all"
			_, err = session.press(textSelectAll)
		case 4:
			label = "delete"
			_, err = session.press(textBackspace)
		case 5:
			label = "emphasis"
			_, err = session.doFormat(markupEmphasis)
		case 6:
			label = "nest"
			_, err = session.doNest(nestDeeper)
		case 7:
			label = "undo"
			_, err = session.doHistory(historyUndo)
		case 8:
			label = "redo"
			_, err = session.doHistory(historyRedo)
		}
		trace = append(trace, label)
		if len(trace) > 32 {
			trace = trace[1:]
		}
		if err != nil {
			t.Fatalf("step %d %s after %q: %v", step, label, trace, err)
		}
		snapshot := session.Runtime().Snapshot()
		if err := session.Selection().ValidateDocument(snapshot.Document()); err != nil {
			t.Fatalf("step %d %s selection: %v\ntrace=%q", step, label, err, trace)
		}
		if err := session.Projection().ValidateSource(snapshot.Revision(), snapshot.Document()); err != nil {
			t.Fatalf("step %d %s projection: %v\ntrace=%q", step, label, err, trace)
		}
		if err := session.Projection().ValidateTarget(session.Caret()); err != nil {
			t.Fatalf("step %d %s target: %v\ntrace=%q", step, label, err, trace)
		}
		if _, err := session.ProjectedState(); err != nil {
			t.Fatalf("step %d %s projected state: %v\ntrace=%q", step, label, err, trace)
		}
		if _, err := session.VisualSelection(); err != nil {
			t.Fatalf("step %d %s visual selection: %v\ntrace=%q", step, label, err, trace)
		}
	}
}

func randomValidProjectedSelection(
	t *testing.T,
	session *Session,
	rng *rand.Rand,
) (model.UTF16Offset, model.UTF16Offset, bool) {
	t.Helper()
	length := session.Projection().UTF16Len()
	if length == 0 {
		return 0, 0, true
	}
	for range 20 {
		anchor := model.UTF16Offset(rng.Intn(int(length) + 1))
		head := model.UTF16Offset(rng.Intn(int(length) + 1))
		if _, err := session.Projection().Target(model.EmptyUTF16Range(anchor)); err != nil {
			continue
		}
		if _, err := session.Projection().Target(model.EmptyUTF16Range(head)); err != nil {
			continue
		}
		return anchor, head, true
	}
	return 0, 0, false
}

func TestRandomProjectedSelectionDoesNotCorruptState(t *testing.T) {
	t.Parallel()
	for seed := int64(2); seed <= 5; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			session := richPage(t)
			for step := 0; step < 500; step++ {
				anchor, head, ok := randomValidProjectedSelection(t, session, rng)
				if !ok {
					continue
				}
				if err := session.SelectProjected(anchor, head); err != nil {
					t.Fatalf("step %d select %d..%d: %v", step, anchor, head, err)
				}
				if _, err := session.ProjectedState(); err != nil {
					t.Fatalf("step %d state after select %d..%d: %v", step, anchor, head, err)
				}
				if _, err := session.VisualSelection(); err != nil {
					t.Fatalf("step %d visual after select %d..%d: %v", step, anchor, head, err)
				}
				if step%7 == 0 {
					if _, err := session.replaceSelection("", kernel.EventOriginSemantic); err != nil {
						t.Fatalf("step %d delete after select %d..%d: %v", step, anchor, head, err)
					}
				}
			}
		})
	}
}

func TestReplacingASelectionWithTheSameTextIsANoOp(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "c", false)
	if err := session.SelectProjected(0, 1); err != nil {
		t.Fatal(err)
	}
	outcome, err := session.replaceSelection("c", kernel.EventOriginTyping)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Outcome != kernel.OutcomeHandled {
		t.Fatalf("same-text replacement outcome = %v", outcome.Outcome)
	}
	node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Node())
	if got := node.Text().String(); got != "c" {
		t.Fatalf("same-text replacement changed text to %q", got)
	}
	if visual, err := session.VisualSelection(); err != nil ||
		visual != model.NewUTF16Range(1, 1) {
		t.Fatalf("same-text replacement visual = %#v, %v", visual, err)
	}
}
