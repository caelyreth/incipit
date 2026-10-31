package components

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// pickedRow is a row of the list as Pick saw it: its geometry relative to
// the list, as last laid out, and how far it is picked, 0 to 1.
type pickedRow struct {
	bounds ui.Rect
	amount float32
}

// pickInset is how far in from the sides a row's share of the fill starts
// as it is picked, growing out to its edges.
const pickInset = 6

// Pick records row, in the list's order, as picked by amount: 0 for a row
// not picked, 1 for one picked, between as it changes. Call it for every
// row the list shows, picked or not, so rows picked side by side are known
// to be neighbours: their fill joins across the space between them into
// one, rounded only where the run begins and ends.
func (t *SelectionTrack) Pick(row ui.Element, amount float32) {
	if t == nil {
		return
	}
	bounds := row.LayoutBounds()
	bounds.X -= t.list.X
	bounds.Y -= t.list.Y
	t.picks = append(t.picks, pickedRow{bounds: bounds, amount: max(0, min(amount, 1))})
}

// Beneath fills row in color under the picked rows' fill, as a picked row
// keeps a fill of its own, as its hover: the fill picking it covers it, and
// it is there as that fill goes, so the row goes straight from the one to
// the other. Call it each frame the row has it.
func (t *SelectionTrack) Beneath(row ui.Element, color ui.Color) {
	if t == nil || color.A == 0 {
		return
	}
	bounds := row.LayoutBounds()
	bounds.X -= t.list.X
	bounds.Y -= t.list.Y
	t.beneath = append(t.beneath, beneathRow{bounds: bounds, color: color})
}

// beneathRow is a fill under the picked rows' (Beneath).
type beneathRow struct {
	bounds ui.Rect
	color  ui.Color
}

func (t *SelectionTrack) paintBeneath(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	for _, row := range t.beneath {
		row.bounds.X += rect.X
		row.bounds.Y += rect.Y
		p.Fill(row.bounds, row.color, style.Radius)
	}
}

// Picking reports whether any row is picked, wholly or on its way.
func (t *SelectionTrack) Picking() bool {
	if t == nil {
		return false
	}
	for _, row := range t.picks {
		if row.amount > 0 {
			return true
		}
	}
	return false
}

// paintPicks fills the picked rows as one shape. Each row's share grows
// from its middle as it is picked and reaches halfway across the space to
// a picked neighbour, so a run is one fill: the shares overlap and the
// path fills their union, with no seam where they meet.
func (t *SelectionTrack) paintPicks(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	var path ui.Path
	strongest := float32(0)
	for index, row := range t.picks {
		a := row.amount
		if a <= 0.001 || row.bounds.W <= 0 || row.bounds.H <= 0 {
			continue
		}
		strongest = max(strongest, a)
		above, below := float32(0), float32(0)
		var reachUp, reachDown float32
		if index > 0 {
			if prev := t.picks[index-1]; prev.amount > 0.001 {
				above = min(a, prev.amount)
				reachUp = (row.bounds.Y-(prev.bounds.Y+prev.bounds.H))/2 + 0.5
			}
		}
		if index+1 < len(t.picks) {
			if next := t.picks[index+1]; next.amount > 0.001 {
				below = min(a, next.amount)
				reachDown = (next.bounds.Y-(row.bounds.Y+row.bounds.H))/2 + 0.5
			}
		}
		half := row.bounds.H / 2
		center := rect.Y + row.bounds.Y + half
		inset := pickInset * (1 - a)
		// Toward a picked neighbour the edge goes from where the share has
		// grown to the middle of the space between them, however far it has
		// grown, so the run never opens a seam while it fills.
		upper := center - half*a
		upper += (center - half - max(0, reachUp) - upper) * bridge(above)
		lower := center + half*a
		lower += (center + half + max(0, reachDown) - lower) * bridge(below)
		share := ui.Rect{X: rect.X + row.bounds.X + inset, Y: upper, W: row.bounds.W - 2*inset, H: lower - upper}
		top := style.Radius * (1 - above)
		bottom := style.Radius * (1 - below)
		RoundedRectPath(&path, share, top, top, bottom, bottom)
	}
	if strongest > 0 {
		p.FillPath(&path, style.Picked.Alpha(strongest))
	}
}

// bridge is how far across the space to a neighbour a share reaches as the
// two join by connection: the space closes in the first half, while the
// corners there round off over the whole, so neighbours meet as a seamless
// fill whose waist fills in rather than a hairline closing between two
// squared edges.
func bridge(connection float32) float32 { return min(1, 2*connection) }

// RoundedRectPath adds r to path, its corners rounded clockwise from the
// top left by tl, tr, br and bl, each no more than half the shorter side.
// Every rect it adds winds the same way, so a path of several fills their
// union.
func RoundedRectPath(path *ui.Path, r ui.Rect, tl, tr, br, bl float32) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	limit := min(r.W, r.H) / 2
	tl, tr, br, bl = clampRadius(tl, limit), clampRadius(tr, limit), clampRadius(br, limit), clampRadius(bl, limit)
	right, bottom := r.X+r.W, r.Y+r.H
	path.MoveTo(r.X+tl, r.Y)
	path.LineTo(right-tr, r.Y)
	corner(path, right-tr, r.Y+tr, tr, -math.Pi/2)
	path.LineTo(right, bottom-br)
	corner(path, right-br, bottom-br, br, 0)
	path.LineTo(r.X+bl, bottom)
	corner(path, r.X+bl, bottom-bl, bl, math.Pi/2)
	path.LineTo(r.X, r.Y+tl)
	corner(path, r.X+tl, r.Y+tl, tl, math.Pi)
	path.Close()
}

func clampRadius(radius, limit float32) float32 { return max(0, min(radius, limit)) }

// corner adds a quarter circle of radius about cx, cy from angle from,
// clockwise on the screen, as one cubic.
func corner(path *ui.Path, cx, cy, radius float32, from float64) {
	if radius <= 0 {
		return
	}
	to := from + math.Pi/2
	k := radius * float32(4.0/3.0*math.Tan(math.Pi/8))
	x0, y0 := cx+radius*float32(math.Cos(from)), cy+radius*float32(math.Sin(from))
	x1, y1 := cx+radius*float32(math.Cos(to)), cy+radius*float32(math.Sin(to))
	path.CubeTo(
		x0-k*float32(math.Sin(from)), y0+k*float32(math.Cos(from)),
		x1+k*float32(math.Sin(to)), y1-k*float32(math.Cos(to)),
		x1, y1,
	)
}
