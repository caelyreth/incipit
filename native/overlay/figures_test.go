package overlay

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestAltTextIsKeptOnReturnOnly(t *testing.T) {
	var dialog AltText
	dialog.Edit(7, "old")
	if _, _, ok := dialog.ConsumeChoice(); ok {
		t.Fatal("words were kept before Return")
	}
	dialog.Value = "A heron at dusk"
	dialog.chosen, dialog.Open = true, false
	node, alt, ok := dialog.ConsumeChoice()
	if !ok || node != 7 || alt != "A heron at dusk" {
		t.Fatalf("kept %v %q %v", node, alt, ok)
	}
	if _, _, ok := dialog.ConsumeChoice(); ok {
		t.Fatal("the words were kept twice")
	}
}

// Zooming keeps the point under the pointer where it is, and a zoomed
// picture never moves past its edges.
func TestTheViewerZoomsTowardThePointer(t *testing.T) {
	var viewer Viewer
	viewer.Show(ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 40, 20))), 40, 20, "", time.Now())
	viewer.zoomAt(2, 300, 100, 400, 200)
	if viewer.zoomTo != 2 || viewer.panToX != -100 || viewer.panToY != 0 {
		t.Fatalf("zoom %v pan %v,%v", viewer.zoomTo, viewer.panToX, viewer.panToY)
	}
	viewer.panBy(-1000, 1000, 400, 200)
	if viewer.panToX != -200 || viewer.panToY != 100 {
		t.Fatalf("panned past the edges to %v,%v", viewer.panToX, viewer.panToY)
	}
	viewer.zoomAt(0.1, 0, 0, 400, 200)
	if viewer.zoomTo != 1 || viewer.panToX != 0 || viewer.panToY != 0 {
		t.Fatalf("zoomed out to %v at %v,%v", viewer.zoomTo, viewer.panToX, viewer.panToY)
	}
	viewer.zoomAt(100, 200, 100, 400, 200)
	if viewer.zoomTo != viewerMost {
		t.Fatalf("zoomed in to %v", viewer.zoomTo)
	}
}
