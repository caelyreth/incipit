package stage

import (
	"cmp"
	"math"
	"slices"
)

// FieldItem describes one item to pack into the field around the pill.
type FieldItem struct {
	ID       string
	Kind     CardKind
	Size     SizeClass
	Signal   Signal
	Match    float32
	HasQuery bool
}

// FieldSpec configures deterministic 2D packing around the pill.
type FieldSpec struct {
	// Room is the unobstructed area for the field (e.g. Scene.FieldRoom).
	Room Rect
	// PillAnchor is the center of the pill inside Room.
	PillAnchor Point
	// PillSize is the pill's width and height in DIPs, reserved on the grid so
	// no card overlaps it.
	PillSize Size
	// Cell is the unit grid cell size (width and height) in DIPs; Gap is the
	// spacing between adjacent cells.
	Cell float32
	Gap  float32
}

// PlacedCard is one packed card with its grid footprint, world rect in field
// coordinates, nearness score, and query emphasis.
type PlacedCard struct {
	Item      FieldItem
	GridX     int
	GridY     int
	GridW     int
	GridH     int
	Rect      Rect
	Nearness  float64
	Emphasis  EmphasisStyle
	Placement Placement
}

// FieldLayout is the result of packing a set of items around the pill.
type FieldLayout struct {
	Cards      []PlacedCard
	Bounds     Rect
	PillAnchor Point
	Room       Rect
}

type gridCell struct{ x, y int }

type candidateOrigin struct {
	x, y int
	dist float64
}

// PackField deterministically packs items on a unit grid around spec.PillAnchor.
// Items are sorted by Nearness (highest nearest the pill) and placed into the
// closest available cells matching spec.Room's proportions.
func PackField(spec FieldSpec, items []FieldItem) FieldLayout {
	cell := spec.Cell
	if cell <= 0 {
		cell = 148
	}
	gap := max(0, spec.Gap)
	step := cell + gap
	anchor := spec.PillAnchor
	if anchor == (Point{}) && !spec.Room.Empty() {
		anchor = spec.Room.Center()
	}

	occupied := map[gridCell]bool{}
	pillMinX, pillMaxX, pillMinY, pillMaxY := reservePill(spec.PillSize, cell, gap, step, occupied)

	if len(items) == 0 {
		return FieldLayout{
			PillAnchor: anchor,
			Room:       spec.Room,
		}
	}

	sorted := SortByNearness(items)

	// Aspect weighting adapts packing to Room: in a narrower Glance pane (~half
	// window), vertical expansion is preferred slightly over spilling behind
	// the leaned desk; in wide Arrive, expansion stays balanced in 2D.
	roomW := max(step*2, spec.Room.W)
	roomH := max(step*2, spec.Room.H)
	weightX := float64(1)
	weightY := float64(roomW / roomH)
	if weightY < 0.65 {
		weightY = 0.65
	} else if weightY > 1.5 {
		weightY = 1.5
	}

	// Build enough candidate coordinates around the pill to guarantee every card
	// fits even if every card is 2x2.
	needCells := 0
	for _, it := range sorted {
		cw, ch := it.Size.Cells()
		needCells += cw * ch
	}
	pillCells := (pillMaxX - pillMinX + 1) * (pillMaxY - pillMinY + 1)
	radius := int(math.Ceil(math.Sqrt(float64(needCells+pillCells)))) + 4

	candidates := make([]candidateOrigin, 0, (2*radius+1)*(2*radius+1))
	for gy := -radius; gy <= radius; gy++ {
		for gx := -radius; gx <= radius; gx++ {
			dx := float64(gx) * weightX
			dy := float64(gy) * weightY
			// Slight deterministic tie-break prefers top/left balanced rings.
			dist := dx*dx + dy*dy
			candidates = append(candidates, candidateOrigin{x: gx, y: gy, dist: dist})
		}
	}
	slices.SortFunc(candidates, func(a, b candidateOrigin) int {
		if c := cmp.Compare(a.dist, b.dist); c != 0 {
			return c
		}
		if c := cmp.Compare(a.y, b.y); c != 0 {
			return c
		}
		return cmp.Compare(a.x, b.x)
	})

	placed := make([]PlacedCard, 0, len(sorted))
	var minX, minY, maxX, maxY float32

	for i, item := range sorted {
		cw, ch := item.Size.Cells()
		ox, oy, ok := findSpot(candidates, occupied, cw, ch)
		if !ok {
			// Fallback beyond initial radius if heavily fragmented.
			extra := radius + cw + ch + 4
			ox, oy = findFallback(occupied, extra, cw, ch)
		}
		for dy := 0; dy < ch; dy++ {
			for dx := 0; dx < cw; dx++ {
				occupied[gridCell{ox + dx, oy + dy}] = true
			}
		}

		// Map grid cell (0,0) center to PillAnchor so symmetric rings center on
		// the pill.
		rx := anchor.X - cell/2 + float32(ox)*step
		ry := anchor.Y - cell/2 + float32(oy)*step
		rw := float32(cw)*cell + float32(cw-1)*gap
		rh := float32(ch)*cell + float32(ch-1)*gap
		rect := Rect{X: rx, Y: ry, W: rw, H: rh}

		if i == 0 {
			minX, minY, maxX, maxY = rect.X, rect.Y, rect.X+rect.W, rect.Y+rect.H
		} else {
			minX = min(minX, rect.X)
			minY = min(minY, rect.Y)
			maxX = max(maxX, rect.X+rect.W)
			maxY = max(maxY, rect.Y+rect.H)
		}

		emph := Emphasis(item.HasQuery, item.Match)
		placed = append(placed, PlacedCard{
			Item:     item,
			GridX:    ox,
			GridY:    oy,
			GridW:    cw,
			GridH:    ch,
			Rect:     rect,
			Nearness: Nearness(item.Signal),
			Emphasis: emph,
			Placement: Placement{
				ID:          item.ID,
				Kind:        KindCard,
				Rect:        rect,
				Scale:       emph.Scale,
				Opacity:     emph.Opacity,
				Brightness:  emph.Brightness,
				Depth:       emph.Depth,
				Interactive: true,
			},
		})
	}

	return FieldLayout{
		Cards:      placed,
		Bounds:     Rect{X: minX, Y: minY, W: maxX - minX, H: maxY - minY},
		PillAnchor: anchor,
		Room:       spec.Room,
	}
}

func reservePill(pill Size, cell, gap, step float32, occupied map[gridCell]bool) (minX, maxX, minY, maxY int) {
	halfX := max(0, pill.W/2) + gap
	halfY := max(0, pill.H/2) + gap
	reachX := max(0, int(math.Ceil(float64((cell/2+halfX)/step)))-1)
	reachY := max(0, int(math.Ceil(float64((cell/2+halfY)/step)))-1)
	minX, maxX = -reachX, reachX
	minY, maxY = -reachY, reachY
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			occupied[gridCell{x, y}] = true
		}
	}
	return minX, maxX, minY, maxY
}

func findSpot(candidates []candidateOrigin, occupied map[gridCell]bool, w, h int) (int, int, bool) {
	for _, cand := range candidates {
		if fitsAt(occupied, cand.x, cand.y, w, h) {
			return cand.x, cand.y, true
		}
	}
	return 0, 0, false
}

func findFallback(occupied map[gridCell]bool, r, w, h int) (int, int) {
	for ring := r; ; ring++ {
		for gy := -ring; gy <= ring; gy++ {
			for gx := -ring; gx <= ring; gx++ {
				if fitsAt(occupied, gx, gy, w, h) {
					return gx, gy
				}
			}
		}
	}
}

func fitsAt(occupied map[gridCell]bool, ox, oy, w, h int) bool {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			if occupied[gridCell{ox + dx, oy + dy}] {
				return false
			}
		}
	}
	return true
}

// VisibleCards returns the cards intersecting view after panning the field by
// pan (a positive pan shifts cards right/down, or view left/up).
func VisibleCards(layout FieldLayout, view Rect, pan Point) []PlacedCard {
	if view.Empty() || len(layout.Cards) == 0 {
		return nil
	}
	// Translate view into un-panned field coordinates.
	shifted := Rect{
		X: view.X - pan.X,
		Y: view.Y - pan.Y,
		W: view.W,
		H: view.H,
	}
	var out []PlacedCard
	for _, card := range layout.Cards {
		if card.Rect.Intersects(shifted) {
			out = append(out, card)
		}
	}
	return out
}

// ClampPan restricts a field pan offset so the packed bounds cannot be panned
// completely outside the visible room plus margin.
func ClampPan(layout FieldLayout, pan Point, margin float32) Point {
	if layout.Bounds.Empty() || layout.Room.Empty() {
		return Point{}
	}
	// Allow panning until the far edge of Bounds reaches the center + margin of
	// Room, keeping the field reachable without getting lost.
	minPanX := (layout.Room.X + layout.Room.W/2) - (layout.Bounds.X + layout.Bounds.W) - margin
	maxPanX := (layout.Room.X + layout.Room.W/2) - layout.Bounds.X + margin
	minPanY := (layout.Room.Y + layout.Room.H/2) - (layout.Bounds.Y + layout.Bounds.H) - margin
	maxPanY := (layout.Room.Y + layout.Room.H/2) - layout.Bounds.Y + margin
	if minPanX > maxPanX {
		minPanX, maxPanX = 0, 0
	}
	if minPanY > maxPanY {
		minPanY, maxPanY = 0, 0
	}
	return Point{
		X: max(minPanX, min(pan.X, maxPanX)),
		Y: max(minPanY, min(pan.Y, maxPanY)),
	}
}
