package stage

import (
	"math"
	"slices"
	"testing"
	"time"
)

func TestPosturesAllocateDeskFieldAndPill(t *testing.T) {
	vp := Rect{W: 1260, H: 850}

	writing := Stage(vp, PostureWriting, true, 40)
	if writing.Desk.Scale != 1 || writing.Desk.Rotate != 0 || !writing.Desk.Interactive {
		t.Fatalf("Writing desk should fill the window interactively, got %+v", writing.Desk)
	}
	if writing.Field.Interactive || writing.Field.Opacity != 0 {
		t.Fatalf("Writing field should stay hidden behind, got %+v", writing.Field)
	}
	if writing.Pill.Rect.X != vp.W/2 || writing.Pill.Rect.Y != 20 {
		t.Fatalf("Writing pill should sit centered on the title line, got %+v", writing.Pill.Rect)
	}

	glance := Stage(vp, PostureGlance, true, 40)
	if !glance.Desk.Interactive || !glance.Field.Interactive {
		t.Fatalf("Glance should keep both desk and field interactive, got desk=%v field=%v", glance.Desk.Interactive, glance.Field.Interactive)
	}
	if glance.FieldRoom.W < vp.W*0.40 || glance.FieldRoom.W > vp.W*0.65 {
		t.Fatalf("Glance should give roughly half the window to the field, got room %+v", glance.FieldRoom)
	}
	if deskLeft := DeskVisualBounds(glance.Desk).X; deskLeft < glance.FieldRoom.W-1 {
		t.Fatalf("Glance field room (%v) overlaps leaned desk left edge (%v)", glance.FieldRoom.W, deskLeft)
	}
	if glance.PillAnchor != glance.FieldRoom.Center() {
		t.Fatalf("Glance pill anchor %+v should center in field room %+v", glance.PillAnchor, glance.FieldRoom)
	}

	arrive := Stage(vp, PostureArrive, true, 40)
	if arrive.FieldRoom.W <= glance.FieldRoom.W {
		t.Fatalf("Arrive should give more room to the field (%v) than Glance (%v)", arrive.FieldRoom.W, glance.FieldRoom.W)
	}
	if arrive.Desk.Scale >= glance.Desk.Scale || arrive.Desk.Rotate <= glance.Desk.Rotate || arrive.Desk.Offset.Y <= glance.Desk.Offset.Y {
		t.Fatalf("Arrive desk should lean further into the bottom-right corner than Glance: arrive=%+v glance=%+v", arrive.Desk, glance.Desk)
	}

	noDesk := Stage(vp, PostureArrive, false, 40)
	if noDesk.Desk.Interactive || noDesk.Desk.Opacity != 0 {
		t.Fatalf("Without open sheets, desk should stay hidden: %+v", noDesk.Desk)
	}
	if noDesk.FieldRoom != vp || noDesk.PillAnchor != vp.Center() {
		t.Fatalf("Without open sheets, field should own the full viewport: room=%+v anchor=%+v", noDesk.FieldRoom, noDesk.PillAnchor)
	}
}

func TestBlendSceneInterpolatesSmoothlyBetweenPostures(t *testing.T) {
	vp := Rect{W: 1260, H: 850}
	start := Stage(vp, PostureWriting, true, 40)
	end := Stage(vp, PostureArrive, true, 40)
	mid := BlendScene(vp, PostureWriting, PostureArrive, 0.5, true, 40)

	if !(mid.Desk.Scale < start.Desk.Scale && mid.Desk.Scale > end.Desk.Scale) {
		t.Fatalf("Mid desk scale %v should lie between %v and %v", mid.Desk.Scale, start.Desk.Scale, end.Desk.Scale)
	}
	if !(mid.Desk.Rotate > start.Desk.Rotate && mid.Desk.Rotate < end.Desk.Rotate) {
		t.Fatalf("Mid desk rotate %v should lie between %v and %v", mid.Desk.Rotate, start.Desk.Rotate, end.Desk.Rotate)
	}
	if !(mid.Field.Opacity > 0 && mid.Field.Opacity < 1) {
		t.Fatalf("Mid field opacity %v should interpolate between 0 and 1", mid.Field.Opacity)
	}
	if !(mid.Pill.Rect.Y > start.Pill.Rect.Y && mid.Pill.Rect.Y < end.Pill.Rect.Y) {
		t.Fatalf("Mid pill Y %v should travel between %v and %v", mid.Pill.Rect.Y, start.Pill.Rect.Y, end.Pill.Rect.Y)
	}
}

func TestNearnessAndEmphasis(t *testing.T) {
	active := Nearness(Signal{ActiveOnDesk: true, OnDesk: true, UpdatedAgo: 48 * time.Hour})
	onDesk := Nearness(Signal{OnDesk: true, UpdatedAgo: 48 * time.Hour})
	recent := Nearness(Signal{UpdatedAgo: 15 * time.Minute})
	older := Nearness(Signal{UpdatedAgo: 30 * 24 * time.Hour})

	if !(active > onDesk && onDesk > recent && recent > older) {
		t.Fatalf("Nearness order broken: active=%v onDesk=%v recent=%v older=%v", active, onDesk, recent, older)
	}

	neutral := Emphasis(false, 0)
	if neutral.Scale != 1 || neutral.Opacity != 1 || neutral.Brightness != 1 || neutral.Depth != 0 {
		t.Fatalf("Without query, emphasis should be neutral: %+v", neutral)
	}
	hit := Emphasis(true, 1)
	miss := Emphasis(true, 0)
	if !(hit.Scale > 1 && hit.Brightness > 1 && hit.Depth > 0 && hit.Opacity == 1) {
		t.Fatalf("Matched item should come forward: %+v", hit)
	}
	if !(miss.Scale < 1 && miss.Opacity < 1 && miss.Brightness < 1 && miss.Depth < 0) {
		t.Fatalf("Unmatched item should recede: %+v", miss)
	}
}

func TestResolveFidelityAndCardSizeClass(t *testing.T) {
	if got := ResolveFidelity("", "", 0); got != FidelityOverview {
		t.Fatalf("Default fidelity should be overview, got %v", got)
	}
	if got := ResolveFidelity("novel", "", 0); got != FidelityGroup {
		t.Fatalf("Group scope should resolve to group fidelity, got %v", got)
	}
	if got := ResolveFidelity("novel", "aurora", 0); got != FidelitySheets {
		t.Fatalf("Group + query should resolve to sheets fidelity, got %v", got)
	}
	if got := ResolveFidelity("", "aurora", 0); got != FidelitySheets {
		t.Fatalf("Query from overview should resolve to sheets fidelity, got %v", got)
	}
	if got := ResolveFidelity("", "", 1); got != FidelitySheets {
		t.Fatalf("Manual step should resolve to sheets fidelity, got %v", got)
	}

	if got := CardSizeClass(CardGroup, FidelityOverview, Signal{Words: 5000}, 15); got != Size2x1 {
		t.Fatalf("Substantial group board should use Size2x1, got %v", got)
	}
	if got := CardSizeClass(CardSheet, FidelitySheets, Signal{Words: 200}, 0); got != Size1x1 {
		t.Fatalf("Short sheet should use Size1x1, got %v", got)
	}
	if got := CardSizeClass(CardSheet, FidelitySheets, Signal{OnDesk: true, Words: 200}, 0); got != Size2x1 {
		t.Fatalf("Sheet on desk should use Size2x1, got %v", got)
	}
}

func TestPackFieldDeterministicNonOverlappingAndClearOfPill(t *testing.T) {
	vp := Rect{W: 1260, H: 850}
	scene := Stage(vp, PostureArrive, true, 40)
	spec := FieldSpec{
		Room:       scene.FieldRoom,
		PillAnchor: scene.PillAnchor,
		PillSize:   Size{W: 280, H: 44},
		Cell:       148,
		Gap:        12,
	}
	items := []FieldItem{
		{ID: "desk-active", Kind: CardSheet, Size: Size2x1, Signal: Signal{ActiveOnDesk: true, OnDesk: true, UpdatedAgo: time.Hour}},
		{ID: "g-novel", Kind: CardGroup, Size: Size2x2, Signal: Signal{OnDesk: true, UpdatedAgo: 2 * time.Hour}},
		{ID: "s-1", Kind: CardSheet, Size: Size1x1, Signal: Signal{UpdatedAgo: 3 * time.Hour}, HasQuery: true, Match: 0.9},
		{ID: "s-2", Kind: CardSheet, Size: Size2x1, Signal: Signal{UpdatedAgo: 6 * time.Hour}, HasQuery: true, Match: 0},
		{ID: "s-3", Kind: CardSheet, Size: Size1x1, Signal: Signal{UpdatedAgo: 12 * time.Hour}},
		{ID: "s-4", Kind: CardSheet, Size: Size1x1, Signal: Signal{UpdatedAgo: 24 * time.Hour}},
		{ID: "s-5", Kind: CardSheet, Size: Size2x1, Signal: Signal{UpdatedAgo: 48 * time.Hour}},
		{ID: "s-6", Kind: CardSheet, Size: Size1x1, Signal: Signal{UpdatedAgo: 72 * time.Hour}},
		{ID: "new", Kind: CardNew, Size: Size1x1, Signal: Signal{UpdatedAgo: 96 * time.Hour}},
	}

	first := PackField(spec, items)
	second := PackField(spec, slices.Clone(items))

	if len(first.Cards) != len(items) {
		t.Fatalf("Expected %d placed cards, got %d", len(items), len(first.Cards))
	}
	for i := range first.Cards {
		if first.Cards[i] != second.Cards[i] {
			t.Fatalf("Packing should be deterministic at index %d: %+v vs %+v", i, first.Cards[i], second.Cards[i])
		}
	}

	pillBox := Rect{
		X: spec.PillAnchor.X - spec.PillSize.W/2 - spec.Gap,
		Y: spec.PillAnchor.Y - spec.PillSize.H/2 - spec.Gap,
		W: spec.PillSize.W + 2*spec.Gap,
		H: spec.PillSize.H + 2*spec.Gap,
	}
	for i, a := range first.Cards {
		if a.Rect.Intersects(pillBox) {
			t.Fatalf("Card %s (%+v) overlaps pill box %+v", a.Item.ID, a.Rect, pillBox)
		}
		for j := i + 1; j < len(first.Cards); j++ {
			b := first.Cards[j]
			if a.Rect.Intersects(b.Rect) {
				t.Fatalf("Cards %s (%+v) and %s (%+v) overlap", a.Item.ID, a.Rect, b.Item.ID, b.Rect)
			}
		}
		if a.Item.Size == Size2x1 {
			wantW := 2*spec.Cell + spec.Gap
			if a.Rect.W != wantW || a.Rect.H != spec.Cell {
				t.Fatalf("2x1 card %s should span %vx%v, got %vx%v", a.Item.ID, wantW, spec.Cell, a.Rect.W, a.Rect.H)
			}
		}
	}

	// Highest-nearness card should stand closer to the pill than the lowest-nearness card.
	nearDist := distToCenter(first.Cards[0].Rect, spec.PillAnchor)
	farDist := distToCenter(first.Cards[len(first.Cards)-1].Rect, spec.PillAnchor)
	if nearDist > farDist {
		t.Fatalf("Nearest item (%s, dist=%v) should sit closer to pill than farthest item (%s, dist=%v)",
			first.Cards[0].Item.ID, nearDist, first.Cards[len(first.Cards)-1].Item.ID, farDist)
	}
}

func TestPackFieldAdaptsToGlanceRoom(t *testing.T) {
	vp := Rect{W: 1260, H: 850}
	glance := Stage(vp, PostureGlance, true, 40)
	spec := FieldSpec{
		Room:       glance.FieldRoom,
		PillAnchor: glance.PillAnchor,
		PillSize:   Size{W: 260, H: 44},
		Cell:       140,
		Gap:        12,
	}
	var items []FieldItem
	for i := range 12 {
		items = append(items, FieldItem{
			ID:     string(rune('a' + i)),
			Kind:   CardSheet,
			Size:   Size1x1,
			Signal: Signal{UpdatedAgo: time.Duration(i+1) * time.Hour},
		})
	}
	layout := PackField(spec, items)
	// Top cards should sit inside the Glance room without all hiding under the leaned desk.
	inside := 0
	for _, c := range layout.Cards {
		if c.Rect.X >= 0 && c.Rect.X+c.Rect.W <= glance.FieldRoom.W+spec.Cell/2 {
			inside++
		}
	}
	if inside < 6 {
		t.Fatalf("Expected at least 6 cards clearly inside Glance room (width=%v), got %d", glance.FieldRoom.W, inside)
	}
}

func TestVisibleCardsAndClampPan(t *testing.T) {
	spec := FieldSpec{
		Room:       Rect{W: 800, H: 600},
		PillAnchor: Point{X: 400, Y: 300},
		PillSize:   Size{W: 240, H: 40},
		Cell:       140,
		Gap:        12,
	}
	var items []FieldItem
	for i := range 24 {
		items = append(items, FieldItem{
			ID:     string(rune('a' + i)),
			Kind:   CardSheet,
			Size:   Size1x1,
			Signal: Signal{UpdatedAgo: time.Duration(i+1) * time.Hour},
		})
	}
	layout := PackField(spec, items)

	initial := VisibleCards(layout, spec.Room, Point{})
	if len(initial) == 0 || len(initial) > len(layout.Cards) {
		t.Fatalf("Unexpected initial visible count: %d", len(initial))
	}
	farAway := VisibleCards(layout, spec.Room, Point{X: 5000, Y: 5000})
	if len(farAway) != 0 {
		t.Fatalf("Expected 0 visible cards far past field bounds, got %d", len(farAway))
	}

	clamped := ClampPan(layout, Point{X: 5000, Y: -5000}, 40)
	if clamped.X >= 5000 || clamped.Y <= -5000 {
		t.Fatalf("ClampPan should bound extreme pan offsets, got %+v", clamped)
	}
	if empty := ClampPan(FieldLayout{}, Point{X: 100, Y: 100}, 40); empty != (Point{}) {
		t.Fatalf("Empty layout should clamp pan to zero, got %+v", empty)
	}
}

func distToCenter(r Rect, p Point) float64 {
	c := r.Center()
	dx, dy := float64(c.X-p.X), float64(c.Y-p.Y)
	return math.Hypot(dx, dy)
}
