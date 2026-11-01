package documentview

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPageCoordinatesIncludeInsetAndVisibleOverscroll(t *testing.T) {
	for _, tt := range []struct {
		top, scroll, viewport, content float32
	}{
		{top: 20, scroll: 0, viewport: 30, content: 10},
		{top: 20, scroll: 100, viewport: 30, content: 110},
		{top: 20, scroll: -12, viewport: 30, content: -2},
		{top: 7.5, scroll: 42.25, viewport: 3, content: 37.75},
	} {
		coordinates := pageCoordinates{top: tt.top, scroll: tt.scroll}
		if got := coordinates.contentPoint(ui.Point{X: 73, Y: tt.viewport}); got.X != 73 || got.Y != tt.content {
			t.Errorf("content point = %+v, want (73, %v)", got, tt.content)
		}
		if got := coordinates.viewportY(tt.content); got != tt.viewport {
			t.Errorf("viewport Y = %v, want %v", got, tt.viewport)
		}
	}
}
