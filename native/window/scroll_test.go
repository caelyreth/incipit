package window

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The page's view reaches the header: the reading space above its first
// line scrolls with the text, so text scrolled up leaves at the header and
// no band stands blank under it.
func TestThePageScrollsUpToTheHeader(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for i := range 40 {
		tester.Type(fmt.Sprintf("Line %d of the page", i))
		tester.Key(0, ui.KeyEnter)
	}
	tester.Advance(2 * time.Second)
	header := theme.DefaultLayout.Titlebar * app.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
	shot(t, tester, "scrolled-to-the-header")
	// Text stands in the band under the header, where the page's column
	// is, not the page's background alone.
	img := tester.Image()
	scale := float32(img.Bounds().Dx()) / 1260
	ground := img.At(int(900*scale), int((header+2)*scale))
	inked := 0
	for y := int((header + 4) * scale); y < int((header+60)*scale); y++ {
		for x := int(300 * scale); x < int(960*scale); x++ {
			if img.At(x, y) != ground {
				inked++
			}
		}
	}
	if inked < 50 {
		t.Fatalf("the band under the header stands blank (%d pixels inked)", inked)
	}
}
