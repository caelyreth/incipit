package page

import (
	"hash/fnv"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/caelyreth/incipit/internal/ui/page/table"
	"github.com/egoist/mygo/ui"
)

// Metrics are the page's measures, in DIPs.
type Metrics struct {
	// Body is the body text size; Spacing a line's height over it.
	Body, Spacing float32
	// Width is the text column's width.
	Width float32
	// Indent is one quote level's indentation, and a nested list
	// level's; ListIndent the first list level's, short of a full step so
	// a list stands near the text around it (indentOf).
	Indent, ListIndent float32
	// Gap is the room between units.
	Gap float32
	// Reach is how far past the column on each side what is wider than it
	// may show, within the page's margins.
	Reach float32
	// Window is the height the page is read in, which a figure's height
	// is capped by; zero where there is none, as a page laid out still.
	Window float32
	// Family is the prose's family list: the Latin face the app ships,
	// then the face chosen for Chinese and Japanese, once kept (internal/app/fonts).
	Family string
}

// segKind is what a segment of a surface's laid-out text stands for.
type segKind uint8

const (
	segText segKind = iota + 1
	segBreak
	segGhost
	segBox
	segComposition
	segSuggestion
)

// seg maps a piece of a surface's slot to its laid-out text: slot offsets
// [from, to) to UTF-16 indices [u0, u1). Ghosts and the composition span
// no slot offsets.
type seg struct {
	kind     segKind
	from, to int
	u0, u1   int
	text     string
	// stopFrom is the stop before a ghost at its offset, and stops how
	// many it holds: one after each of its glyphs.
	stopFrom, stops int
	// node is a box's inline node, card the surface of its card, and open
	// whether the card is open: the object held.
	node *doc.Node
	card int
	open bool
	// segFace is how a box's or a ghost's text is drawn, nil for text and
	// breaks, which a page holds the most of; groups resolve paint-time ink
	// colors.
	*segFace
	groups []vocab.Group
	// inline is an inline object's renderer; what it keeps here (math)
	// is its own.
	inline inlineKind
	math   mathSet
}

// segFace is how a box's or a ghost's text is drawn, as it was measured,
// and shaped is it shaped.
type segFace struct {
	face   ui.Span
	shaped *ui.TextLayout
}

// decoSpec records a surface decoration range and its highlight groups so
// colors resolve dynamically at paint time without re-shaping on theme changes.
type decoSpec struct {
	rg     ui.TextInputRange
	groups []vocab.Group
	hash   uint64
}

func groupsHash(groups []vocab.Group) uint64 {
	h := fnv.New64a()
	var buf [4]byte
	for _, g := range groups {
		v := uint32(g)
		buf[0], buf[1], buf[2], buf[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
		_, _ = h.Write(buf[:])
	}
	return h.Sum64()
}

// deco is a stretch of a surface the page draws itself, beneath its
// text: a background, a line, or both. Consecutive segments wearing the
// same are one run, signs and text alike, so nothing seams where spans
// meet; a highlight's range is one too.
type deco struct {
	rg   ui.TextInputRange
	bg   ui.Color
	line present.LineKind
	// ink is the line's color; zero is the text's.
	ink ui.Color
	// outline is drawn around an object the decoration holds whole.
	outline ui.Color
}

// decoOf is what a style asks the page to draw for a range: its
// background and its line, in the line's color, or else the text's.
func decoOf(rg ui.TextInputRange, s present.Style) deco {
	d := deco{rg: rg, line: s.Line}
	if s.Set&present.SetBackground != 0 {
		d.bg = unpack(s.Background)
	}
	d.outline = d.bg
	if s.Set&present.SetOutline != 0 {
		d.outline = unpack(s.Outline)
	}
	switch {
	case d.line == present.LineNone:
	case s.Set&present.SetLineColor != 0:
		d.ink = unpack(s.LineColor)
	case s.Set&present.SetColor != 0:
		d.ink = unpack(s.Color)
	}
	return d
}

func (d deco) drawn() bool {
	return d.bg != (ui.Color{}) || d.line != present.LineNone || d.outline != (ui.Color{})
}

func (d deco) same(o deco) bool {
	return d.bg == o.bg && d.line == o.line && d.ink == o.ink && d.outline == o.outline
}

// invisible paints nothing; a zero color would take the layout's default.
var invisible = ui.Color{R: 1}

// stopAt is a stop, the i-th of its presentation's, and the UTF-16
// index it stands at in its surface. It names the stop rather than copying
// its caret: a page holds one per grapheme.
type stopAt struct {
	pres *present.Presentation
	i    int32
	u    int
}

// caret is the stop's caret.
func (s stopAt) caret() doc.Caret { return s.pres.Stops[s.i].Caret }

// surfaceLayout is one laid-out surface of a unit.
type surfaceLayout struct {
	index int
	// card is an inline object's card; panel a surface its object stands
	// on an inset panel, as a math block's source.
	card   bool
	panel  bool
	unit   doc.ID
	anchor doc.ID
	slot   doc.SlotRef
	// cardFormula and cardDialect record the anchored Formula node and its
	// active dialect when this surface is an inline formula's card.
	cardFormula doc.ID
	cardDialect profile.MathDialect
	layout *ui.TextLayout
	decos  []decoSpec
	text   string
	segs   []seg
	stops  []stopAt
	// x and y place the surface in its unit; w and h are its extent,
	// fontSize its resolved base font size in DIPs, and reach how far right
	// its lines' text goes, kept as it is laid out so what measures the page
	// never shapes a layout let go (release.go).
	x, y, w, h, fontSize, reach float32
	// open is the presentation's: a main flow, or an object's own slot
	// while the object is held. shown is whether the surface is drawn and
	// takes room, which its object's renderer decides from open: a closed
	// card is not, a caption with text is. A range over a surface not
	// shown is painted on what stands for it (standIn).
	open, shown bool
	// marked is the composition's UTF-16 range, when it is in this
	// surface.
	marked    ui.TextInputRange
	hasMarked bool
	compSel   int
	suggEnd   int
	hasSugg   bool
}

// unitLayout is one laid-out unit.
type unitLayout struct {
	key       layoutKey
	pres      *present.Presentation
	accessKey string
	// at is where each of the unit's stops in its slots is, made when
	// first looked up (place).
	at map[doc.Caret]place
	// since is when the layout was first painted.
	since time.Time
	// kind is an object's renderer, nil for text; what it keeps here
	// (math, figure) is its own.
	kind     objectKind
	surfaces []*surfaceLayout
	// x is the unit's indent from the column's start; y its top in the
	// page as drawn, top its top with every room above settled, and h its
	// height.
	x, y, top, h float32
	// sketch is set for a unit placed far from the window without its
	// text, at its height as last laid out or an estimate, and shape what
	// it held open then, -1 where it never was laid out (window.go).
	sketch bool
	shape  int
	// gen is the build that last kept the layout (layoutCache.gen), and
	// placedAt, for a sketch, what it was placed at.
	gen      uint64
	placedAt sketchedAt
	// gapFrom are the presentations the gap above it was resolved
	// between: the one above and its own.
	gapFrom [2]*present.Presentation
	// far is whether its text's shaping was let go, far out of view, and
	// held whether it was read there since, so it stays shaped
	// (release.go).
	far, held bool
	// gaps are an object's gap stops before and after it, nil on a side
	// where text stands beside it (present.UnitRef.Gaps); box is what it
	// draws, in the unit, and the text it holds (a figure's caption) is its
	// surfaces, under the box.
	gaps [2]*stopAt
	box  ui.Rect
	// drawn is all an object draws, which an outline goes around held
	// whole: its box, or more where its renderer draws more around it.
	// frame, when set, is the corner radius of a frame the object draws on
	// drawn's edge, as an open math block's: an outline then lies on that
	// frame, the frame taking the paint's look (paints.go).
	drawn ui.Rect
	frame float32
	// math is a math block's display over its source panel.
	math *mathLayout
	// figure is a figure's shape, as it is laid out, and crop the share of
	// its picture's height its box shows.
	figure *profile.Shape
	crop   float32
	// table is a table's grid.
	table *table.Layout
	// view, when set, is what the unit shows of what it lays out, in the
	// unit, as a table scrolled across: whatever is past it, its surfaces'
	// text and what covers them, is clipped, and a point past it is read
	// at its edge.
	view ui.Rect
	// shown is the height the unit is drawn at: its height, or on its way
	// there as a surface in it opens or closes; fade how strongly it shows,
	// short of whole while it comes in.
	shown, fade float32
	// size is a text unit's size over the body, its first surface's, and
	// gap the room above it, in gaps; what it is drawn at while its text
	// resizes is asked of its layout (sizes.go).
	size, gap float32
}

// bottom is the unit's lowest extent as drawn.
func (u *unitLayout) bottom() float32 { return u.y + u.shown }

// TODO(paint): layout and paint are one layer here. Colours are baked into
// shaped spans and decorations, and the key carries a theme id, so a theme or
// accent change reshapes every unit. Separate paint from layout (§13.7).
type layoutKey struct {
	pres    *present.Presentation
	metrics Metrics
	theme   uintptr
	comp    string
	sugg    string
	// in is what the page asks of an object's layout besides its
	// presentation.
	in objectIn
}

// objectIn is what the page asks of a unit's layout besides its
// presentation, as a gesture previews it or its motion sets it: a figure's
// span as a resize sets it, a table's room an area opens and a column's
// width its border's drag sets, a text's size on its way to its own.
type objectIn struct {
	// span is a figure's span, or -1.
	span  int
	table table.In
	// size is the size a text unit is laid out at, over the body, and gap
	// the room above it, in gaps, as its text resizes; 0 where it stands at
	// its own (sizes.go).
	size, gap float32
}

// pageLayout is the page's units laid out from top to bottom.
type pageLayout struct {
	units []*unitLayout
	// height is the page's height as drawn, rooms on their way; rest is
	// its height with every room settled.
	height, rest float32
	// slots finds the unit a caret's slot is laid out in, and gaps the
	// place of each gap stop, beside an object rather than in a slot; a
	// unit finds its own stops (unitLayout.place).
	slots map[doc.SlotRef]*unitLayout
	gaps  map[doc.Caret]place
	// unsketched are the units laid out that the layout before had as
	// sketches: they come into the page's flow quietly (settleRooms).
	unsketched map[doc.ID]bool
	// first and last are the page's first and last stops.
	first, last *stopAt
	// moving are the units a paint steps (Element.step): those of an
	// object kind or holding inline objects, found once a layout.
	moving      []*unitLayout
	movingFound bool
}

// steppers are the layout's units that step as it is painted: an
// object's, or one holding inline objects. The others hold nothing that
// moves.
func (pl *pageLayout) steppers() []*unitLayout {
	if !pl.movingFound {
		pl.movingFound = true
		for _, u := range pl.units {
			if u.kind != nil || holdsInline(u) {
				pl.moving = append(pl.moving, u)
			}
		}
	}
	return pl.moving
}

// holdsInline reports whether a unit lays out an inline object.
func holdsInline(u *unitLayout) bool {
	for _, s := range u.surfaces {
		for _, sg := range s.segs {
			if sg.inline != nil {
				return true
			}
		}
	}
	return false
}

type place struct {
	unit    *unitLayout
	surface *surfaceLayout // nil for a gap stop
	stop    *stopAt
}

// layoutCache keeps units' layouts between frames, by unit.
type layoutCache struct {
	units map[doc.ID]*unitLayout
	// sketches are the layouts of units sketched far from the window;
	// measured is how each unit was last laid out, and estimates the
	// heights of those never laid out as they are (window.go).
	sketches  map[doc.ID]*unitLayout
	measured  map[doc.ID]measured
	estimates map[doc.ID]estimated
	// gen counts the builds, which stamp the layouts they keep;
	// sketchedAt is what the sketches were placed at, and gapOf the gaps
	// between looks resolved there.
	gen        uint64
	sketchedAt sketchedAt
	gapOf      map[gapKey]float32
	// resized is where each unit whose text changed size was drawn as it
	// changed, until a paint takes it up (sizes.go).
	resized map[doc.ID]resize
	// good is each formula's last source that set soundly.
	good    map[lastGood]string
	sugg    *policy.Suggestion
	themeID uintptr
}

func newLayoutCache() *layoutCache {
	return &layoutCache{units: map[doc.ID]*unitLayout{}, good: map[lastGood]string{},
		sketches: map[doc.ID]*unitLayout{}, measured: map[doc.ID]measured{}, estimates: map[doc.ID]estimated{}, gapOf: map[gapKey]float32{}}
}

// keyMetrics is what of m p's layout is keyed on: the margins only for
// what reads them, so a page widened past its measure lays out again only
// the objects that show into its margins, not every line of its text.
