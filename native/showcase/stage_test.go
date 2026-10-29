package showcase

import (
	"image"
	"os"
	"testing"
	"time"

	appshowcase "github.com/caelyreth/neverland/app/showcase"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/testlock"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := testlock.Acquire(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func stageTester(t *testing.T, policy kernel.PolicyID, animate bool) (*ui.Tester, theme.Theme) {
	t.Helper()
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	script, _ := appshowcase.For(policy)
	semantic := theme.DefaultSettings().Semantic()
	stage := Stage{
		Key: "stage", Script: script,
		Configuration: appshowcase.Configuration(policy, host.Configuration()),
		Theme:         semantic, Prose: theme.NewProseStyle(17),
		Height: 132, Padding: 24, Radius: 10, Animate: animate,
	}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(semantic.Surface.Page).Children(func() { stage.View(c) })
	}, 600, 132)
	tester.Frame()
	return tester, semantic
}

// ink is the box of pixels that differ from the page inside the stage's
// border.
func ink(img *image.RGBA, page ui.Color) image.Rectangle {
	box := image.Rectangle{Min: img.Bounds().Max}
	bounds := img.Bounds().Inset(4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R == page.R && c.G == page.G && c.B == page.B {
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

func TestStageTypesTheStoryCenteredOnItsPage(t *testing.T) {
	tester, semantic := stageTester(t, editor.TypographyPolicyID, true)
	empty := ink(tester.Image(), semantic.Surface.Page)
	tester.Advance(5 * time.Second)
	written := ink(tester.Image(), semantic.Surface.Page)
	if written.Empty() || written.Dx() <= empty.Dx() {
		t.Fatalf("the story drew nothing: before %v, after %v", empty, written)
	}
	left, right := written.Min.X, 600-written.Max.X
	if diff := left - right; diff < -4 || diff > 4 {
		t.Fatalf("text spans %v: margins %d and %d are not centered", written, left, right)
	}
	top, bottom := written.Min.Y, 132-written.Max.Y
	if diff := top - bottom; diff < -8 || diff > 8 {
		t.Fatalf("text spans %v: margins %d and %d are not centered", written, top, bottom)
	}
}

func TestStageShowsTheFinishedStoryStillWithoutMotion(t *testing.T) {
	tester, semantic := stageTester(t, editor.TypographyPolicyID, false)
	first := ink(tester.Image(), semantic.Surface.Page)
	if first.Empty() || first.Dx() < 200 {
		t.Fatalf("a still stage shows %v, want the finished sentence", first)
	}
	tester.Advance(10 * time.Second)
	if later := ink(tester.Image(), semantic.Surface.Page); later != first {
		t.Fatalf("a still stage moved from %v to %v", first, later)
	}
}
