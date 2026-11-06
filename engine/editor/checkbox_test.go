package editor

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func checkboxShape(t *testing.T, session *Session, index int) blockShape {
	t.Helper()
	blocks := DocumentBlocks(session.Runtime().Snapshot().Document())
	if index >= len(blocks) {
		t.Fatalf("page has %d blocks, not block %d", len(blocks), index)
	}
	return shapeOf(blocks[index])
}

func TestCheckboxTriggerUsesPairedBrackets(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		checked           bool
	}{
		{"unchecked", "- [ ", "- [ ] |", false},
		{"checked", "- [x", "- [x] |", true},
		{"uppercase", "+ [X", "- [x] |", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newSessionFixture(t, "", false)
			typeKeys(t, session, test.input)
			expectKinds(t, session, CheckboxNode)
			expectPage(t, session, test.want, "")
			if got := checkboxShape(t, session, 0); got.checked != test.checked {
				t.Fatalf("checked = %v, want %v", got.checked, test.checked)
			}
			typeKeys(t, session, "write")
			expectPage(t, session, test.want[:len(test.want)-1]+"write|", "write")
		})
	}
}

func TestCheckboxTriggerWithoutPairingAndAtStartOnly(t *testing.T) {
	for _, input := range []string{"- [ ] ", "- [x] ", "- [X] "} {
		session, _ := newSessionFixture(t, "", false)
		if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
			configuration.SetEnabled(PairsPolicyID, false)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		typeKeys(t, session, input)
		expectKinds(t, session, CheckboxNode)
		if got := checkboxShape(t, session, 0).checked; got != (input != "- [ ] ") {
			t.Fatalf("%q checked = %v", input, got)
		}
	}
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- word [x")
	expectKinds(t, session, BulletNode)
	expectPage(t, session, "- word [x|]", "word [x]")
}

func TestCheckboxTriggerRespectsListSetting(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- ")
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetSetting(NewWritingSettingKey(listsSetting), kernel.BoolSetting(false))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	typeKeys(t, session, "[x")
	expectKinds(t, session, BulletNode)
	expectPage(t, session, "- [x|]", "[x]")
}

func TestCheckboxToggleHistoryAndWritingPolicy(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- [ task")
	block := DocumentBlocks(session.Runtime().Snapshot().Document())[0]
	before := session.Selection()
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(WritingPolicyID, false)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if outcome, err := setCheckbox(session, block.ID(), true); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("toggle = %v, %v", outcome, err)
	}
	if got := checkboxShape(t, session, 0); !got.checked {
		t.Fatal("toggle did not check item")
	}
	if got := session.Selection(); got.Head().Node() != before.Head().Node() || got.Head().Offset() != before.Head().Offset() {
		t.Fatalf("toggle moved selection from %v to %v", before, got)
	}
	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	if checkboxShape(t, session, 0).checked {
		t.Fatal("undo did not uncheck item")
	}
	if _, err := session.doHistory(historyRedo); err != nil {
		t.Fatal(err)
	}
	if !checkboxShape(t, session, 0).checked {
		t.Fatal("redo did not check item")
	}
	if outcome, err := setCheckbox(session, model.NodeID(9999), true); err != nil || outcome != kernel.OutcomePass {
		t.Fatalf("missing checkbox = %v, %v", outcome, err)
	}
}

func TestCheckboxSplitAndNesting(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- [xfirst\nsecond")
	expectKinds(t, session, CheckboxNode, CheckboxNode)
	if !checkboxShape(t, session, 0).checked || checkboxShape(t, session, 1).checked {
		t.Fatal("a split should leave the original checked and the new item unchecked")
	}
	if _, err := session.doNest(nestDeeper); err != nil {
		t.Fatal(err)
	}
	shape := checkboxShape(t, session, 1)
	if shape.depth != 1 || shape.checked {
		t.Fatalf("nested item = %+v", shape)
	}
	expectPage(t, session, "- [x] first\n  - [ ] second|", "first", "second")
}

func TestCheckboxFlowAndReadableText(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- [xfirst\nsecond")
	if _, err := session.doNest(nestDeeper); err != nil {
		t.Fatal(err)
	}
	flow, err := session.SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	checks := surface.ObjectsOf[surface.Checkbox](flow)
	if len(checks) != 2 || !checks[0].Checked || checks[1].Checked ||
		checks[0].Marker != model.NewUTF16Range(0, 5) || checks[1].Marker != model.NewUTF16Range(14, 19) ||
		checks[0].Label != "first" || checks[1].Label != "second" {
		t.Fatalf("checkbox metadata = %+v", checks)
	}
	text, err := session.ReadableText(model.NewUTF16Range(0, flow.UTF16Len()))
	if err != nil || text != "- [x] first\n  - [ ] second" {
		t.Fatalf("copied text = %q, %v", text, err)
	}
}

func TestCheckboxConversionUndo(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- [")
	typeKeys(t, session, "x")
	expectKinds(t, session, CheckboxNode)
	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	expectKinds(t, session, BulletNode)
	expectPage(t, session, "- [|]", "[]")
	if _, err := session.doHistory(historyRedo); err != nil {
		t.Fatal(err)
	}
	expectPage(t, session, "- [x] |", "")
}

func TestCheckboxTogglePreservesProjectedSelection(t *testing.T) {
	for _, selection := range [][2]model.UTF16Offset{{0, 2}, {8, 6}, {0, 10}} {
		session, _ := newSessionFixture(t, "", false)
		typeKeys(t, session, "- [ task")
		if err := session.SelectProjected(selection[0], selection[1]); err != nil {
			t.Fatal(err)
		}
		before, err := session.ProjectedState()
		if err != nil {
			t.Fatal(err)
		}
		beforeAnchor, beforeHead, err := session.Projection().ResolveSelection(before, session.Runtime().Revision())
		if err != nil {
			t.Fatal(err)
		}
		block := DocumentBlocks(session.Runtime().Snapshot().Document())[0]
		if _, err := setCheckbox(session, block.ID(), true); err != nil {
			t.Fatal(err)
		}
		after, err := session.ProjectedState()
		if err != nil {
			t.Fatal(err)
		}
		afterAnchor, afterHead, err := session.Projection().ResolveSelection(after, session.Runtime().Revision())
		if err != nil || beforeAnchor.RangeUTF16() != afterAnchor.RangeUTF16() || beforeHead.RangeUTF16() != afterHead.RangeUTF16() {
			t.Fatalf("%v: projected selection changed from %+v to %+v, error %v", selection, before, after, err)
		}
	}
}

// setCheckbox checks or unchecks node by a Shape intent, as a click does.
func setCheckbox(session *Session, node model.NodeID, checked bool) (kernel.Outcome, error) {
	shape := BlockShape{Kind: CheckboxNode}
	if block, ok := session.Runtime().Snapshot().Document().Node(node); ok {
		shape = ShapeOf(block)
	}
	shape.Checked = checked
	outcome, err := session.Do(Shape(node, shape), time.Time{})
	return outcome.Outcome, err
}
