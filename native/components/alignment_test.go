package components

import (
	"image"
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Alignment is checked in device pixels at 2x, where half a DIP shows.

type rgb [3]uint8

func pixel(img *image.RGBA, x, y int) rgb {
	c := img.RGBAAt(x, y)
	return rgb{c.R, c.G, c.B}
}

func (a rgb) near(b rgb, tolerance int) bool {
	distance := 0
	for index := range a {
		delta := int(a[index]) - int(b[index])
		if delta < 0 {
			delta = -delta
		}
		distance += delta
	}
	return distance <= tolerance
}

// region bounds the pixels of r that keep accepts.
func region(img *image.RGBA, r image.Rectangle, keep func(x, y int, c rgb) bool) image.Rectangle {
	out := image.Rectangle{Min: image.Pt(1<<30, 1<<30), Max: image.Pt(-1, -1)}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if keep(x, y, pixel(img, x, y)) {
				out.Min.X, out.Min.Y = min(out.Min.X, x), min(out.Min.Y, y)
				out.Max.X, out.Max.Y = max(out.Max.X, x+1), max(out.Max.Y, y+1)
			}
		}
	}
	return out
}

// dominant is the most common color in r.
func dominant(img *image.RGBA, r image.Rectangle) rgb {
	counts := map[rgb]int{}
	best, color := 0, rgb{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := pixel(img, x, y)
			counts[c]++
			if counts[c] > best {
				best, color = counts[c], c
			}
		}
	}
	return color
}

// renderOnBlack draws view at 2x over black and returns the frame and the
// bounds of everything view painted.
func renderOnBlack(view func(c *ui.Context)) (*image.RGBA, image.Rectangle) {
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, 1)).Padding(20, 20, 20, 20).Children(func() { view(c) })
	}, 260, 80)
	tester.SetScale(2)
	tester.Frame()
	img := tester.Image()
	black := rgb{}
	return img, region(img, img.Bounds(), func(_, _ int, c rgb) bool { return !c.near(black, 6) })
}

// inkWithin bounds what differs from box's dominant fill, away from its
// rounded corners and edges.
func inkWithin(img *image.RGBA, box image.Rectangle, corner int) image.Rectangle {
	fill := dominant(img, box)
	black := rgb{}
	return region(img, box, func(x, y int, c rgb) bool {
		if x < box.Min.X+4 || x >= box.Max.X-4 || y < box.Min.Y+4 || y >= box.Max.Y-4 {
			return false
		}
		if (x < box.Min.X+corner || x >= box.Max.X-corner) &&
			(y < box.Min.Y+corner || y >= box.Max.Y-corner) {
			return false
		}
		return !c.near(fill, 60) && !c.near(black, 30)
	})
}

// thumbProfile finds the thumb along the track's center row, then down the
// column through the thumb's center, so the track's rounded ends, whose
// anti-aliasing differs from its fill, never count as thumb.
func thumbProfile(img *image.RGBA, track image.Rectangle) image.Rectangle {
	fill := dominant(img, track)
	differs := func(x, y int) bool { return !pixel(img, x, y).near(fill, 60) }
	row := (track.Min.Y + track.Max.Y) / 2
	left, right := -1, -1
	for x := track.Min.X + 2; x < track.Max.X-2; x++ {
		if differs(x, row) && !differs(x-1, row) && left < 0 && x > track.Min.X+2 {
			left = x
		}
		if differs(x, row) {
			right = x + 1
		}
	}
	if left < 0 {
		left = track.Min.X + 2
	}
	column := (left + right) / 2
	top, bottom := -1, -1
	for y := track.Min.Y + 1; y < track.Max.Y-1; y++ {
		if differs(column, y) {
			if top < 0 {
				top = y
			}
			bottom = y + 1
		}
	}
	return image.Rect(left, top, right, bottom)
}

func TestSwitchThumbKeepsOneMarginInBothStates(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	gaps := map[bool][4]int{}
	for _, on := range []bool{false, true} {
		img, track := renderOnBlack(func(c *ui.Context) {
			Switch{Key: "s", Label: "S", Selected: on}.View(c, semantic)
		})
		thumb := thumbProfile(img, track)
		gap := [4]int{
			thumb.Min.X - track.Min.X, track.Max.X - thumb.Max.X,
			thumb.Min.Y - track.Min.Y, track.Max.Y - thumb.Max.Y,
		}
		if gap[2] != gap[3] {
			t.Fatalf("on=%v thumb is not vertically centered: top=%d bottom=%d", on, gap[2], gap[3])
		}
		gaps[on] = gap
	}
	off, on := gaps[false], gaps[true]
	if off[0] != off[2] || on[1] != on[2] || off[0] != on[1] {
		t.Fatalf("thumb margins differ: off left=%d top=%d, on right=%d top=%d",
			off[0], off[2], on[1], on[2])
	}
}

func TestIconButtonsCenterTheirGlyph(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	for name, button := range map[string]IconButton{
		"outlined": {Key: "a", Label: "A", Icon: IconKeyboard.SVG(), Style: OutlinedButton},
		"selected": {Key: "a", Label: "A", Icon: IconSearch.SVG(), Style: SubtleButton, Selected: true},
		"compact":  {Key: "a", Label: "A", Icon: IconClose.SVG(), Style: SubtleButton, Size: CompactButton, Selected: true},
	} {
		img, box := renderOnBlack(func(c *ui.Context) { button.View(c, semantic) })
		glyph := inkWithin(img, box, 18)
		left, right := glyph.Min.X-box.Min.X, box.Max.X-glyph.Max.X
		top, bottom := glyph.Min.Y-box.Min.Y, box.Max.Y-glyph.Max.Y
		if left != right || top != bottom {
			t.Fatalf("%s glyph off center: left=%d right=%d top=%d bottom=%d", name, left, right, top, bottom)
		}
	}
}
