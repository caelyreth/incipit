// Package table lays out, animates, paints, and handles pointer controls for
// tables behind a narrow Host and Layouter interface.
package table

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/motion"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Metrics are the page's prose and margin measures needed to lay out a table.
type Metrics struct {
	Body, Spacing, Width, Margin, Reach float32
}

// Line is a single prose line height.
func (m Metrics) Line() float32 { return m.Body * m.Spacing }

// CellLayout is the measured layout of one table cell surface.
type CellLayout struct {
	W, H   float32
	Layout *ui.TextLayout
}

// Surface is the page's view of one laid-out cell surface.
type Surface struct {
	ID        doc.ID
	Index     int
	X, Y      float32
	W, H      float32
	Shown     bool
	Paint     func(p *ui.Painter, sx, sy float32)
	SetPos    func(x, y float32)
	CaretRect func(u int) ui.Rect
}

// Unit is the page's view of a laid-out table unit.
type Unit struct {
	Pres     *present.Presentation
	Box      ui.Rect
	X, Y, H  float32
	Fade     float32
	Frame    float32
	Surfaces []Surface
	Table    *Layout
}

// Bottom returns the unit's bottom edge in page coordinates.
func (u *Unit) Bottom() float32 { return u.Y + u.H }

// Part names which of a table's controls a control is.
type Part string

// ControlKey identifies one table control.
type ControlKey struct {
	Unit doc.ID
	Part Part
	At   int
}

// Control is a pointer target offered by a table.
type Control struct {
	Key    ControlKey
	Rect   ui.Rect
	Cursor ui.Cursor
	Press  func(x, y float32, clicks int, extend bool, at time.Time) Drag
	Menu   func(x, y float32, at time.Time)
}

// Drag is an active pointer drag over a table control.
type Drag interface {
	Key() ControlKey
	Move(x, y float32, at time.Time)
	Finish(canceled bool, at time.Time)
	Cursor() ui.Cursor
	Ask() (doc.ID, In, bool)
}

// ValueKind selects the spring curve for a table spring.
type ValueKind uint8

const (
	ShareValue ValueKind = iota
	RoomValue
	PlaceValue
)

// SpringKey names a spring value owned by a table on the host.
type SpringKey struct {
	Unit doc.ID
	Name string
	At   int
	Of   ValueKind
}

// FlowIn describes one row or column fed into a table's line flow.
type FlowIn struct {
	ID            doc.ID
	Place, Extent float32
	Moves         bool
	Kept          any
	From          float32
}

// FlowExit describes a row or column leaving a table's line flow.
type FlowExit struct {
	After, Cover doc.ID
	Lead         float32
	Extent       float32
	Alpha        float32
	Kept         any
}

// Flow abstracts the host's per-axis line flow animation.
type Flow interface {
	Holds(ids []doc.ID) bool
	Knows(id doc.ID) bool
	Saw(id doc.ID, place float32)
	From(id doc.ID, place float32, now time.Time)
	Tell(in []FlowIn, now time.Time, animate bool) bool
	Step(now time.Time, animate bool) bool
	Drawn(id doc.ID, extent float32) (float32, float32, float32)
	Exits() []FlowExit
}

// Layouter lays out individual cell surfaces for a table.
type Layouter interface {
	LayCell(index int, s present.Surface, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, shape ui.TextShape, keep, shown bool) (Surface, CellLayout)
	ObjectRadius(m Metrics) float32
}

// ControlMetrics are the host's scaled control dimensions.
type ControlMetrics struct {
	Height float32
	Radius float32
}

// Scroll tracks a wide table's horizontal scroll and elastic overscroll.
type Scroll struct {
	X     float32
	Over  motion.Elastic
	Glide motion.Glide
}

// Host is the page surface capability required by table controls, motion, and painting.
type Host interface {
	Theme() theme.Theme
	Prose() theme.ProseStyle
	Metrics() Metrics
	UIFontSize() float32
	Column() float32
	Near() doc.ID
	HotKey() ControlKey
	Pointer() ui.Point
	PointerIn() bool
	Viewport() ui.Rect
	ToPage(x, y float32) (float32, float32)
	Now() time.Time
	Animate() bool
	MarkMoving()
	Snapshot() *doc.Snapshot
	Selection() doc.Selection
	Do(in vocab.Intent, at time.Time) bool
	Unit(id doc.ID) *Unit
	ActiveDrag() Drag
	HasDrag() bool
	Holding() ControlKey
	Lit(key ControlKey) float32
	Scroll(id doc.ID, create bool) *Scroll
	SpringValue(key SpringKey) float32
	SpringFollow(key SpringKey, target, freq float32) float32
	SpringJump(key SpringKey, v float32)
	State(id doc.ID, create bool) *State
	NewFlow() Flow
	ChromeSize() float32
	ChromeScale() float32
	ControlMetrics() ControlMetrics
	ObjectRadius(m Metrics) float32
	PaintChip(p *ui.Painter, text string, place func(ui.Rect) (ui.Rect, bool))
	CursorAt(x, y float32) (ui.Cursor, bool)
	PlaceForMenu(x, y float32, at time.Time)
}

// Kind implements table layout, painting, motion, and controls against a Host.
type Kind struct{}

var (
	chromeFollow = settleFrequency(components.MotionNormal)
	carryPart    = settleFrequency(components.MotionPanel)
	pressFollow  = settleFrequency(components.MotionQuick)
)

func settleFrequency(d time.Duration) float32 { return float32(2 * math.Pi / d.Seconds()) }

func contains(r ui.Rect, x, y float32) bool {
	return x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H
}

func grown(r ui.Rect, dx, dy float32) ui.Rect {
	return ui.Rect{X: r.X - dx, Y: r.Y - dy, W: r.W + 2*dx, H: r.H + 2*dy}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

const controlReach = 28
