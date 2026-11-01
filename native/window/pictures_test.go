package window

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/documentview"
	"github.com/egoist/mygo/ui"
)

type delayedPictures struct {
	place func([]editor.ImageShape)
}

func (d *delayedPictures) Take(_ []documentview.Incoming, place func([]editor.ImageShape)) {
	d.place = place
}

func insertPicture(t *testing.T, document *controller.DocumentController, media, alt string) model.NodeID {
	t.Helper()
	_, err := document.InsertImage(editor.ImageShape{
		Media: media, Width: 4, Height: 3, Alt: alt,
	})
	if err != nil {
		t.Fatal(err)
	}
	document.Refresh()
	for _, node := range document.Runtime().Snapshot().Document().Nodes() {
		if shape, ok := editor.ImageShapeOf(node); ok && shape.Media == media {
			return node.ID()
		}
	}
	t.Fatalf("picture %q was not inserted", media)
	return 0
}

func pictureShape(t *testing.T, document *controller.DocumentController, node model.NodeID) editor.ImageShape {
	t.Helper()
	current, ok := document.Runtime().Snapshot().Document().Node(node)
	if !ok {
		t.Fatalf("image node %d is missing", node)
	}
	shape, ok := editor.ImageShapeOf(current)
	if !ok {
		t.Fatalf("node %d is not an image", node)
	}
	return shape
}

func TestImageOperationKeepsTheDialogInsertionPoint(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "one\ntwo")
	if err := app.Session.Document().SetSelection(controller.UTF16Range{Start: 1, End: 1}); err != nil {
		t.Fatal(err)
	}
	operation, ok := app.Bridge.BeginImageOperation()
	if !ok {
		t.Fatal("the insertion operation did not begin")
	}
	if err := app.Session.Document().SetSelection(controller.UTF16Range{Start: 4, End: 4}); err != nil {
		t.Fatal(err)
	}
	if err := operation.PlaceImages([]editor.ImageShape{{Media: "a", Width: 4, Height: 3}}); err != nil {
		t.Fatal(err)
	}
	if got := app.Session.Document().Text(); got != "one\n\uFFFC\ntwo" {
		t.Fatalf("the moved caret changed the insertion point: %q", got)
	}
}

func TestDelayedIntakeCannotWriteIntoAnotherPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "page one")
	first := app.Session
	secondID, err := app.Workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	typeText(t, app, "page two")
	second := app.Session
	if err := app.Workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()

	pictures := &delayedPictures{}
	app.Bridge.Images = pictures
	operation, ok := app.Bridge.BeginImageOperation()
	if !ok {
		t.Fatal("the intake operation did not begin")
	}
	var placeErr error
	if !operation.Take([]documentview.Incoming{{Path: "/tmp/one.png"}}, func(shapes []editor.ImageShape) {
		placeErr = operation.PlaceImages(shapes)
	}) {
		t.Fatal("the intake did not start")
	}
	if err := app.Workspace.Open(secondID); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()
	if pictures.place == nil {
		t.Fatal("intake did not keep its completion")
	}
	pictures.place([]editor.ImageShape{{Media: "a", Width: 4, Height: 3}})
	if !errors.Is(placeErr, documentview.ErrStaleImageOperation) {
		t.Fatalf("late intake error = %v", placeErr)
	}
	if got := first.Document().Text(); got != "page one" {
		t.Fatalf("first page changed after switching away: %q", got)
	}
	if got := second.Document().Text(); got != "page two" {
		t.Fatalf("late intake changed the mounted page: %q", got)
	}
}

func TestImageOperationDoesNotReviveAfterReturningToPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	first := app.Session
	operation, ok := app.Bridge.BeginImageOperation()
	if !ok {
		t.Fatal("the intake operation did not begin")
	}
	if _, err := app.Workspace.NewPage(); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()
	if err := app.Workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()
	if err := operation.PlaceImages([]editor.ImageShape{{Media: "a", Width: 4, Height: 3}}); !errors.Is(err, documentview.ErrStaleImageOperation) {
		t.Fatalf("old operation on remounted page error = %v", err)
	}
	if got := first.Document().Text(); got != "" {
		t.Fatalf("old operation changed the remounted page: %q", got)
	}
}

func TestAStaleDialogOperationIsReportedAndLeavesHistoryAlone(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "first page")
	first := app.Session
	operation, ok := app.Bridge.BeginImageOperation()
	if !ok {
		t.Fatal("the insertion operation did not begin")
	}
	history := first.Document().Session().History().UndoDepth()
	secondID, err := app.Workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()

	app.takePictures("Couldn't add the picture", operation,
		[]documentview.Incoming{{Path: "/tmp/one.png"}})
	tester.Frame()
	if !tester.HasText("Couldn't add the picture") {
		t.Fatalf("the stale operation was not told: %v", tester.Texts())
	}
	if got := first.Document().Text(); got != "first page" {
		t.Fatalf("the parked page changed: %q", got)
	}
	if got := first.Document().Session().History().UndoDepth(); got != history {
		t.Fatalf("the stale operation entered history: %d -> %d", history, got)
	}
	if active, _ := app.Workspace.Active(); active != secondID {
		t.Fatalf("the stale operation changed the active page to %d", active)
	}
}

func TestReplacementUsesTheCapturedPageWhenNodeIDsCollide(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	defer app.Close()
	first := app.Session
	firstNode := insertPicture(t, first.Document(), "first-media", "First")
	app.save()
	firstRevision := first.Document().Runtime().Revision()
	if first.saved != firstRevision {
		t.Fatalf("first page saved revision = %d, want %d", first.saved, firstRevision)
	}

	secondID, err := app.Workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	second := app.Session
	secondNode := insertPicture(t, second.Document(), "second-media", "Second")
	if firstNode != secondNode {
		t.Fatalf("fixture image nodes do not collide: %d and %d", firstNode, secondNode)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	if err := app.Workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()
	operation, ok := app.Bridge.BeginImageTarget(firstNode)
	if !ok {
		t.Fatal("the replacement operation did not begin")
	}

	if err := app.Workspace.Open(secondID); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()
	secondRevision := second.Document().Runtime().Revision()
	firstHistory := first.Document().Session().History().UndoDepth()
	secondHistory := second.Document().Session().History().UndoDepth()
	err = operation.ReplaceImage(editor.ImageShape{Media: "replacement", Width: 8, Height: 6})
	if !errors.Is(err, documentview.ErrStaleImageOperation) {
		t.Fatalf("replacement on another page error = %v", err)
	}
	if shape := pictureShape(t, first.Document(), firstNode); shape.Media != "first-media" {
		t.Fatalf("parked page picture changed to %q", shape.Media)
	}
	if shape := pictureShape(t, second.Document(), secondNode); shape.Media != "second-media" {
		t.Fatalf("mounted page's colliding picture changed to %q", shape.Media)
	}
	if second.Document().Runtime().Revision() != secondRevision {
		t.Fatal("a rejected replacement changed the mounted page revision")
	}
	if first.Document().Session().History().UndoDepth() != firstHistory ||
		second.Document().Session().History().UndoDepth() != secondHistory {
		t.Fatal("a rejected replacement entered history")
	}
	if first.Document().Runtime().Revision() != firstRevision || first.saved != firstRevision {
		t.Fatal("a rejected replacement made the parked page unsaved")
	}
}

func TestReplacementDoesNotApplyOverANewerRevision(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	node := insertPicture(t, app.Session.Document(), "first-media", "First")
	operation, ok := app.Bridge.BeginImageTarget(node)
	if !ok {
		t.Fatal("the replacement operation did not begin")
	}
	typeText(t, app, "changed")
	err = operation.ReplaceImage(editor.ImageShape{Media: "replacement", Width: 8, Height: 6})
	if !errors.Is(err, documentview.ErrStaleImageOperation) {
		t.Fatalf("replacement over a newer revision error = %v", err)
	}
	if shape := pictureShape(t, app.Session.Document(), node); shape.Media != "first-media" {
		t.Fatalf("a stale replacement changed the picture to %q", shape.Media)
	}
}

func TestAltTextUsesTheCapturedPageWhenNodeIDsCollide(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	first := app.Session
	firstNode := insertPicture(t, first.Document(), "first-media", "First")
	secondID, err := app.Workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	second := app.Session
	secondNode := insertPicture(t, second.Document(), "second-media", "Second")
	if firstNode != secondNode {
		t.Fatalf("fixture image nodes do not collide: %d and %d", firstNode, secondNode)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	if err := app.Workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()
	figureHost{app: app}.EditAlt(firstNode, "First")
	tester.Frame()

	if err := app.Workspace.Open(secondID); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	tester.Frame()
	app.AltText.Value = "Changed"
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	if !tester.HasText("Couldn't change the image") {
		t.Fatalf("the stale alt text was not reported: %v", tester.Texts())
	}
	if shape := pictureShape(t, first.Document(), firstNode); shape.Alt != "First" {
		t.Fatalf("alt text changed the parked page: %q", shape.Alt)
	}
	if shape := pictureShape(t, second.Document(), secondNode); shape.Alt != "Second" {
		t.Fatalf("alt text changed the mounted page's colliding image: %q", shape.Alt)
	}
}
