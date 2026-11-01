package documentview

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A figure shows its picture in the room the layout keeps for it, from
// the moment its line does: its place first, as quiet as an inset, and
// the picture fading in over it once decoded. A hairline keeps a picture
// as light as the page from bleeding into it. Selected as a whole, as a
// click on it selects its line, it wears the accent's ring, as the lists'
// menus ring what they act on; inside a larger selection, it takes the
// selection's tint.

const (
	figureFade = float32(14)
	// figureRingOutset and figureRingWidth set the ring off the picture,
	// in DIPs.
	figureRingOutset = 3
	figureRingWidth  = 2
)

// FigureMotion keeps each figure's fade and ring across frames, by its
// picture and its place among the figures showing it.
type FigureMotion struct {
	tracks map[string]*figureTrack
}

type figureTrack struct {
	shown, ring, grips motion.Spring
	seen               bool
}

// Reset forgets every figure's motion, so each shows settled.
func (m *FigureMotion) Reset() {
	if m != nil {
		m.tracks = nil
	}
}

// figureKeys names each figure by its picture and how many before it show
// the same one, which a block's identity changing does not move.
func figureKeys(figures []textlayout.PlacedFigure) []string {
	seen := make(map[string]int, len(figures))
	keys := make([]string, len(figures))
	for index, figure := range figures {
		keys[index] = fmt.Sprintf("%s#%d", figure.Media, seen[figure.Media])
		seen[figure.Media]++
	}
	return keys
}

// figureSelected reports whether the selection is a figure's whole line,
// which a click on it selects.
func figureSelected(figure textlayout.PlacedFigure, selection model.UTF16Range) bool {
	return selection == figure.Line && !selection.Empty()
}

func figureRadius(state *State) float32 {
	return theme.DefaultControls.Scaled(uiFontSize(state)).Radius
}

// uiFontSize is the size the page's chrome is set at, the default when
// none is given.
func uiFontSize(state *State) float32 {
	if state.UIFontSize > 0 {
		return state.UIFontSize
	}
	return theme.DefaultUIFontSize
}

// drawFigures paints the figures in view and reports whether one is still
// moving.
func drawFigures(p *ui.Painter, rect ui.Rect, state *State, options PaintOptions) bool {
	figures := state.Layout.Figures()
	motionState := state.FigureMotion
	if motionState == nil {
		motionState = &FigureMotion{}
	}
	if motionState.tracks == nil {
		motionState.tracks = make(map[string]*figureTrack)
	}
	for _, track := range motionState.tracks {
		track.seen = false
	}
	now := options.Now
	selection := state.Frame.Selection()
	visible := p.Visible()
	radius := figureRadius(state)
	moving := false
	for index, key := range figureKeys(figures) {
		figure := figures[index]
		box := figure.Box
		box.X += rect.X
		box.Y += rect.Y + state.TopInset - state.ScrollY
		bitmap, picture := state.Pictures.Picture(figure.Media, now)
		shown, ring, grips := float32(0), float32(0), float32(0)
		if picture == PictureReady {
			shown = 1
		}
		if figureSelected(figure, selection) && options.Focused {
			ring = 1
		}
		resizing := state.Resizing != nil && state.Resizing.node == figure.Node
		if resizing || ring > 0 || state.HoverFigure.ok && state.HoverFigure.node == figure.Node {
			grips = 1
		}
		track, known := motionState.tracks[key]
		if !known {
			// A figure already decoded shows at once, as when a page
			// opens again; one that has to wait fades in.
			track = &figureTrack{
				shown: motion.NewSpring(shown, now),
				ring:  motion.NewSpring(0, now),
				grips: motion.NewSpring(0, now),
			}
			motionState.tracks[key] = track
		}
		track.seen = true
		if options.Animate {
			track.shown.Step(shown, now, figureFade)
			track.ring.Step(ring, now, figureFade)
			track.grips.Step(grips, now, figureFade)
		} else {
			track.shown, track.ring = motion.NewSpring(shown, now), motion.NewSpring(ring, now)
			track.grips = motion.NewSpring(grips, now)
		}
		moving = moving || track.shown.Moving() || track.ring.Moving() || track.grips.Moving()
		if box.Y+box.H+figureRingOutset < visible.Y || box.Y-figureRingOutset > visible.Y+visible.H {
			continue
		}
		drawFigure(p, box, radius, state, bitmap, picture, track.shown.Value())
		if covers(selection, figure.Line) && !figureSelected(figure, selection) {
			p.Fill(box, state.Theme.Prose.Selection.Alpha(0.5*state.Motion.SelectionOpacity()), radius)
		}
		if alpha := track.ring.Value(); alpha > 0.001 {
			out := ui.Rect{
				X: box.X - figureRingOutset, Y: box.Y - figureRingOutset,
				W: box.W + 2*figureRingOutset, H: box.H + 2*figureRingOutset,
			}
			p.Stroke(out, state.Theme.Accent.Foreground.Alpha(0.85*alpha), radius+figureRingOutset, figureRingWidth)
		}
		if alpha := track.grips.Value(); alpha > 0.001 && picture != PictureMissing {
			drawGrips(p, box, figure, state, alpha)
		}
		if resizing {
			drawResizeLabel(p, box, state, state.Resizing.label())
		}
	}
	for key, track := range motionState.tracks {
		if !track.seen {
			delete(motionState.tracks, key)
		}
	}
	return options.Animate && moving
}

// drawFigure paints one figure's place and, as far as it has faded in,
// its picture.
func drawFigure(
	p *ui.Painter,
	box ui.Rect,
	radius float32,
	state *State,
	bitmap *ui.Bitmap,
	picture PictureState,
	shown float32,
) {
	if shown < 1 {
		p.Fill(box, state.Theme.Surface.Inset.Alpha(1-shown), radius)
	}
	if picture == PictureMissing {
		size := min(box.W, box.H, 20*uiFontSize(state)/theme.DefaultUIFontSize)
		icon := ui.Rect{X: box.X + (box.W-size)/2, Y: box.Y + (box.H-size)/2, W: size, H: size}
		p.Icon(components.IconImageOff.SVG(), icon, state.Theme.Text.Muted)
		return
	}
	if bitmap == nil || shown <= 0 {
		return
	}
	p.Clip(box, radius, func() {
		p.ImageRegion(bitmap, ui.Rect{}, box, ui.ImageOptions{Tint: ui.Color{R: 255, G: 255, B: 255, A: 255}.Alpha(shown)})
	})
	p.Stroke(box, state.Theme.Prose.Subtle.Alpha(shown), radius, 1)
}

// drawGrips paints the grips on the sides a figure resizes from: both of
// a centered one, the end of one at the start.
func drawGrips(p *ui.Painter, box ui.Rect, figure textlayout.PlacedFigure, state *State, alpha float32) {
	const width, inset = 6, 8
	height := min(36, box.H*0.4)
	if height < 12 || box.W < 3*inset {
		return
	}
	sides := []float32{box.X + box.W - inset - width}
	if figure.Align == surface.FigureCenter {
		sides = append(sides, box.X+inset)
	}
	for _, x := range sides {
		grip := ui.Rect{X: x, Y: box.Y + (box.H-height)/2, W: width, H: height}
		p.Shadow(grip, width/2, 0, 1, 4, 0, ui.Color{A: 255}.Alpha(0.3*alpha))
		p.Fill(grip, state.Theme.Surface.Elevated.Alpha(alpha), width/2)
		p.Stroke(grip, state.Theme.Border.Default.Alpha(alpha), width/2, 1)
	}
}

// drawResizeLabel names the width a figure is being resized to, on a chip
// at the top of the figure.
func drawResizeLabel(p *ui.Painter, box ui.Rect, state *State, label string) {
	size := theme.TextSmall.Size(uiFontSize(state))
	span := ui.Span{Text: label, Size: size, Color: state.Theme.Text.Default}
	width, height := p.MeasureText(0, span)
	padX, padY := size*0.75, size*0.3
	chip := ui.Rect{W: width + 2*padX, H: height + 2*padY}
	chip.X, chip.Y = box.X+(box.W-chip.W)/2, box.Y+padX
	if chip.W > box.W || chip.H*2 > box.H {
		return
	}
	p.Fill(chip, state.Theme.Surface.Elevated.Alpha(0.94), chip.H/2)
	p.RichText(chip.X+padX, chip.Y+padY, 0, span)
}

// covers reports whether a selection takes all of line.
func covers(selection, line model.UTF16Range) bool {
	return selection.Start <= line.Start && selection.End >= line.End && !selection.Empty()
}
