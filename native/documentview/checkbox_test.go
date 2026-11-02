package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func checkboxPage(t *testing.T, input string) (*controller.DocumentController, *Element, *ui.Tester) {
	t.Helper()
	document, err := controller.NewTextDocument("", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range input {
		if _, err := document.ReplaceSelection(string(r), kernel.EventOriginTyping); err != nil {
			t.Fatal(err)
		}
	}
	settings := theme.DefaultSettings()
	element := NewElement(NewBridge(document, nil, nil), settings.Semantic(), settings.Prose(), time.Now())
	element.Animate = true
	tester := ui.NewTester(element.View, 600, 400)
	tester.Frame()
	tester.Frame()
	return document, element, tester
}

func checkboxes(t *testing.T, document *controller.DocumentController) []model.NodeID {
	t.Helper()
	flow, err := document.Session().SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	var nodes []model.NodeID
	for _, check := range flow.Checkboxes() {
		nodes = append(nodes, check.Node)
	}
	return nodes
}

func checkboxChecked(t *testing.T, document *controller.DocumentController) bool {
	t.Helper()
	flow, err := document.Session().SurfaceFlow()
	if err != nil || len(flow.Checkboxes()) != 1 {
		t.Fatalf("checkbox flow = %v, %v", flow, err)
	}
	return flow.Checkboxes()[0].Checked
}

func TestCheckboxClickKeyboardAndUndo(t *testing.T) {
	document, element, tester := checkboxPage(t, "- [ task")
	before := document.Selection()
	box, ok := tester.Find("task")
	if !ok || box.W <= 0 || box.H <= 0 {
		t.Fatalf("checkbox not found: %+v, %v", box, ok)
	}
	tester.Move(box.X+box.W/2, box.Y+box.H/2)
	if tester.Cursor() != ui.CursorPointer {
		t.Fatalf("checkbox cursor = %v", tester.Cursor())
	}
	if err := tester.Click("task"); err != nil {
		t.Fatal(err)
	}
	if !checkboxChecked(t, document) {
		t.Fatal("click did not check item")
	}
	if got := document.Selection(); got.Range != before.Range || got.Reversed != before.Reversed {
		t.Fatalf("checkbox click moved selection from %+v to %+v", before, got)
	}
	tester.Advance(40 * time.Millisecond)
	tester.Frame()
	spring := element.checkboxes.tracks[checkboxes(t, document)[0]]
	if spring == nil || spring.Value() <= 0 || spring.Value() >= 1 {
		t.Fatalf("check did not animate: %+v", spring)
	}
	tester.Key(0, ui.KeySpace)
	if checkboxChecked(t, document) {
		t.Fatal("Space did not uncheck focused item")
	}
	if _, err := document.Execute(controller.Command{ID: string(editor.HistoryUndo.ID())}); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !checkboxChecked(t, document) {
		t.Fatal("undo did not restore the checked item")
	}
}

func TestEmptyCheckboxHasAnAccessibleControl(t *testing.T) {
	document, _, tester := checkboxPage(t, "- [ ")
	if err := tester.Click("Untitled item"); err != nil {
		t.Fatal(err)
	}
	if !checkboxChecked(t, document) {
		t.Fatal("empty checkbox did not toggle")
	}
}

func TestCheckboxReducedMotionAndPageSwitch(t *testing.T) {
	document, element, tester := checkboxPage(t, "- [ task")
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()
	if err := tester.Click("task"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	node := checkboxes(t, document)[0]
	if spring := element.checkboxes.tracks[node]; spring == nil || spring.Value() != 1 || spring.Moving() {
		t.Fatalf("reduced motion did not settle: %+v", spring)
	}
	element.SwitchPage(0)
	if len(element.checkboxes.tracks) != 0 {
		t.Fatal("page switch kept checkbox animation")
	}
}

func TestCheckboxAppearsWhileTypingAndFollowsScrolledLayout(t *testing.T) {
	document, element, tester := checkboxPage(t, "")
	for _, r := range "- [ task" {
		tester.Type(string(r))
	}
	tester.Frame()
	if _, ok := tester.Find("task"); !ok {
		t.Fatal("typed checkbox did not appear as an interactive control")
	}
	before, ok := tester.Find("task")
	if !ok {
		t.Fatal("checkbox not found")
	}
	element.SetScrollY(10)
	tester.Frame()
	after, ok := tester.Find("task")
	if !ok || after.Y >= before.Y {
		t.Fatalf("checkbox did not follow scroll: before %+v, after %+v", before, after)
	}
	if err := tester.Click("task"); err != nil || !checkboxChecked(t, document) {
		t.Fatalf("scrolled checkbox did not toggle: %v", err)
	}
}

func TestCheckboxControlTracksMovedAndRemovedItems(t *testing.T) {
	document, element, tester := checkboxPage(t, "before\n- [ task")
	items := checkboxes(t, document)
	if len(items) != 1 {
		t.Fatalf("checkboxes = %v", items)
	}
	first := editor.DocumentBlocks(document.Session().Runtime().Snapshot().Document())[0].ID()
	if _, err := document.MoveBlock(items[0], first); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if _, ok := tester.Find("task"); !ok {
		t.Fatal("moved checkbox lost its control")
	}
	if err := tester.Click("task"); err != nil || !checkboxChecked(t, document) {
		t.Fatalf("moved checkbox did not toggle: %v", err)
	}
	if err := document.SelectProjected(0, int(document.Session().Projection().UTF16Len())); err != nil {
		t.Fatal(err)
	}
	if _, err := document.Execute(controller.Command{ID: string(editor.TextBackspace.ID())}); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if _, ok := tester.Find("task"); ok || len(element.checkboxes.tracks) != 0 {
		t.Fatal("removed checkbox left a control or animation")
	}
}

func TestCheckboxTextStillAcceptsPointerSelectionAndTyping(t *testing.T) {
	document, element, tester := checkboxPage(t, "- [ task")
	caret := element.current.Caret(8)
	tester.ClickAt(caret.X, caret.Y+caret.H/2)
	tester.Type("X")
	if got := document.Text(); got != "- [ ] taXsk" {
		t.Fatalf("typed at checkbox text: %q", got)
	}
}

func TestCheckedCheckboxPaintsADiagonalTick(t *testing.T) {
	_, element, tester := checkboxPage(t, "- [ task")
	box, ok := tester.Find("task")
	if !ok {
		t.Fatal("checkbox not found")
	}
	if err := tester.Click("task"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(time.Second)
	tester.Frame()
	accent := element.Theme.Accent.Foreground
	minX, maxX, minY, maxY := int(box.W), 0, int(box.H), 0
	for y := 2; y < int(box.H)-2; y++ {
		for x := 2; x < int(box.W)-2; x++ {
			color := pixel(tester, box.X+float32(x), box.Y+float32(y))
			difference := abs(float32(color.R)-float32(accent.R)) + abs(float32(color.G)-float32(accent.G)) + abs(float32(color.B)-float32(accent.B))
			if difference > 90 {
				minX, maxX = min(minX, x), max(maxX, x)
				minY, maxY = min(minY, y), max(maxY, y)
			}
		}
	}
	if maxX-minX < 4 || maxY-minY < 3 {
		t.Fatalf("tick did not span diagonally: x=%d..%d, y=%d..%d", minX, maxX, minY, maxY)
	}
}

func TestCheckboxRemainsInPlaceDuringIMEPreview(t *testing.T) {
	document, element, tester := checkboxPage(t, "- [ task")
	client := element.Input.Client()
	client.SetMarkedText(nil, "日", ui.TextInputRange{Start: 1, End: 1})
	tester.Frame()
	checks := element.current.Flow().Checkboxes()
	if len(checks) != 1 || checks[0].Marker != model.NewUTF16Range(0, 5) {
		t.Fatalf("composition lost checkbox: %+v", checks)
	}
	if err := tester.Click("task"); err != nil {
		t.Fatal(err)
	}
	if checkboxChecked(t, document) {
		t.Fatal("checkbox changed during composition")
	}
	client.UnmarkText()
	tester.Frame()
	if got := document.Text(); got != "- [ ] task日" {
		t.Fatalf("composition committed as %q", got)
	}
}

func TestCheckboxPressDoesNotCrossPageSwitch(t *testing.T) {
	first, element, tester := checkboxPage(t, "- [ task")
	box, ok := tester.Find("task")
	if !ok {
		t.Fatal("checkbox not found")
	}
	tester.Press(box.X+box.W/2, box.Y+box.H/2)
	second, _, _ := checkboxPage(t, "- [ task")
	element.SwitchPage(0)
	element.Input = NewBridge(second, nil, nil)
	tester.Frame()
	tester.Release(box.X+box.W/2, box.Y+box.H/2)
	tester.Frame()
	if checkboxChecked(t, first) || checkboxChecked(t, second) {
		t.Fatal("a press on the old page changed a checkbox after the switch")
	}
}
