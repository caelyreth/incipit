package components

import "github.com/egoist/mygo/ui"

// ProgressRing is a ring size across that fills clockwise from the top as
// far as done says, from 0 to 1: its track in track, what is done in ink,
// each width wide, told to the screen reader as label. Full, it holds a
// check in ink, so done reads as done and not as nearly. It takes no press
// of its own, which goes to what holds it, but the pointer rests on it,
// for a hint to tell what it counts.
func ProgressRing(c *ui.Context, key string, size, width, done float32, track, ink ui.Color, label string) ui.Element {
	done = max(0, min(done, 1))
	return ui.Box(c.Key(key)).Size(size, size).Shrink(0).Role(ui.RoleImage).Label(label).
		Draw(func(p *ui.Painter, r ui.Rect) {
			cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, (min(r.W, r.H)-width)/2
			var ring ui.Path
			p.StrokePath(topCircle(&ring, cx, cy, rad), width, track)
			if done > 0 {
				var arc ui.Path
				p.StrokePath(topCircle(&arc, cx, cy, rad).Cap(ui.RoundCap).Trim(0, done), width, ink)
			}
			if done >= 1 {
				var check ui.Path
				in := rad - width
				check.MoveTo(cx-in*0.55, cy+in*0.02)
				check.LineTo(cx-in*0.12, cy+in*0.45)
				check.LineTo(cx+in*0.6, cy-in*0.4)
				p.StrokePath(&check, width*0.85, ink)
			}
		})
}

// topCircle adds to path a circle that starts at its top and goes
// clockwise, as a ring fills.
func topCircle(path *ui.Path, cx, cy, r float32) *ui.Path {
	const k = 0.5522847498
	path.MoveTo(cx, cy-r)
	path.CubeTo(cx+r*k, cy-r, cx+r, cy-r*k, cx+r, cy)
	path.CubeTo(cx+r, cy+r*k, cx+r*k, cy+r, cx, cy+r)
	path.CubeTo(cx-r*k, cy+r, cx-r, cy+r*k, cx-r, cy)
	path.CubeTo(cx-r, cy-r*k, cx-r*k, cy-r, cx, cy-r)
	return path
}
